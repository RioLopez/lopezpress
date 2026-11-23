package services

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"lopezpress/internal/version"
)

// Self-update via GitHub Releases.
//
// Flow: root asks for status (current build vs the repo's latest release),
// starts an apply job, and the server downloads the release asset — a tar.gz
// holding the linux-amd64 binary and the SPA dist TOGETHER, so the two halves
// of the site can never drift — verifies its sha256 against the release's
// checksum asset, snapshots the database, swaps the binary and the web
// directory (keeping the previous versions as rollback copies), and then
// exits. systemd (Restart=always) brings the new binary up within seconds.
//
// Why exit instead of exec: the process runs under a systemd sandbox
// (NoNewPrivileges, ProtectSystem=strict minus the ReadWritePaths carve-out).
// It has no sudo, no systemctl, no shell — but it CAN replace its own files
// and it CAN exit. Restart=always turns that exit into the restart, which
// keeps the sandbox fully intact. `systemctl stop` still stops the service
// for good: explicit stops never count as a restart trigger.
//
// Rollback is the mirror operation over the kept lopezpress.v<ver> files and
// web.v<ver> directories: swap back, exit, systemd reaps the new process.

// Keep this many previous versions on disk for rollback. Each binary is
// ~15 MB and each web dir ~1 MB, so three copies stay well inside the 5 GB
// disk budget while still allowing "step back twice".
const updateKeepBackups = 3

// Phases reported through the job state (UI strings live in the frontend).
const (
	updatePhaseIdle        = "idle"
	updatePhaseChecking    = "checking"
	updatePhaseDownloading = "downloading"
	updatePhaseVerifying   = "verifying"
	updatePhaseSnapshot    = "snapshotting"
	updatePhaseSwapping    = "swapping"
	updatePhaseRestarting  = "restarting"
	updatePhaseDone        = "done"
	updatePhaseFailed      = "failed"
)

// statePendingRestart is the phase persisted to disk right before the process
// exits. On the next boot it is what tells the new binary "you are the result
// of a planned update" so the history entry can be closed as a success.
const statePendingRestart = "restarting"

// versionNameRe is the only shape accepted as a rollback target or embedded
// in a backup file name. Everything else is refused before it can touch a
// path: a version never becomes a traversal vector.
var versionNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// --- DTOs (also the HTTP JSON shape) ----------------------------------------

type UpdateCurrentInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Built   string `json:"built"`
	Dirty   bool   `json:"dirty"`
}

type UpdateReleaseInfo struct {
	Tag         string `json:"tag"`
	Version     string `json:"version"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
	Notes       string `json:"notes"`
}

type UpdateRollbackOption struct {
	Version string `json:"version"`
	Binary  bool   `json:"binary"`
	Web     bool   `json:"web"`
}

type UpdateJobState struct {
	Phase     string `json:"phase"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
	Target    string `json:"target,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type UpdateHistoryEntry struct {
	Action string `json:"action"` // "update" | "rollback"
	From   string `json:"from"`
	To     string `json:"to"`
	At     string `json:"at"`
	Result string `json:"result"` // "pending-restart" | "success" | "failed"
	Note   string `json:"note,omitempty"`
}

type UpdateStatusDTO struct {
	Current         UpdateCurrentInfo      `json:"current"`
	Latest          *UpdateReleaseInfo     `json:"latest"`
	UpdateAvailable bool                   `json:"update_available"`
	Rollback        []UpdateRollbackOption `json:"rollback"`
	Job             UpdateJobState         `json:"job"`
	History         []UpdateHistoryEntry   `json:"history"`
}

// --- GitHub API shapes (only the fields we read) -----------------------------

type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	PublishedAt string        `json:"published_at"`
	HTMLURL     string        `json:"html_url"`
	Body        string        `json:"body"`
	Assets      []githubAsset `json:"assets"`
}

// updaterState is the Service-hung state: one job at a time, one restart
// hook, and a short-lived cache of the "latest release" answer (a status
// poll every few seconds must not turn into a GitHub API call every few
// seconds — anonymous quota is 60/h per IP).
type updaterState struct {
	mu      sync.Mutex
	running bool
	job     UpdateJobState
	hook    func() // set by main.go; cancels the shutdown context -> graceful exit 0 -> systemd restart

	cacheMu     sync.Mutex
	cacheLatest *githubRelease
	cacheAt     time.Time
}

func newUpdaterState() *updaterState { return &updaterState{} }

// SetRestartHook registers the shutdown trigger. main.go passes the cancel
// function of its signal context: calling it is equivalent to receiving
// SIGTERM, so the normal graceful path (flush views, drain HTTP) runs before
// the process exits 0 and systemd restarts it into the new binary.
func (s *Service) SetRestartHook(fn func()) {
	s.upd.mu.Lock()
	defer s.upd.mu.Unlock()
	s.upd.hook = fn
}

// TriggerRestartAfterDelay gives the HTTP response time to reach the client
// (and the UI a moment to show the final phase) before the graceful exit.
func (s *Service) TriggerRestartAfterDelay(delay time.Duration) {
	s.upd.mu.Lock()
	hook := s.upd.hook
	s.upd.mu.Unlock()
	if hook == nil {
		return
	}
	go func() {
		time.Sleep(delay)
		hook()
	}()
}

// --- paths -------------------------------------------------------------------

// updateAppDir is the directory the running binary lives in. On the systemd
// deployment that is /opt/lopezpress, which is exactly the directory the
// service unit's ReadWritePaths carve-out allows us to write.
func (s *Service) updateAppDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

func (s *Service) updateDir() string { return s.cfg.UpdateDir }

func (s *Service) stagingDir() string { return filepath.Join(s.cfg.UpdateDir, "staging") }

func stateFile(s *Service) string { return filepath.Join(s.cfg.UpdateDir, "state.json") }

func historyFile(s *Service) string { return filepath.Join(s.cfg.UpdateDir, "history.json") }

// --- job state ---------------------------------------------------------------

func (st *updaterState) setJob(phase, message string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	st.job.Phase = phase
	st.job.Message = message
	st.job.UpdatedAt = now
	if st.job.StartedAt == "" {
		st.job.StartedAt = now
	}
}

func (st *updaterState) failJob(err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	st.job.Phase = updatePhaseFailed
	st.job.Error = err.Error()
	st.job.UpdatedAt = now
	st.running = false
}

func (st *updaterState) snapshot() UpdateJobState {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.job
}

func (st *updaterState) isRunning() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.running
}

// --- release lookup ----------------------------------------------------------

func (s *Service) githubClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func (s *Service) githubRequest(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if tok := s.cfg.GitHubToken; tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	return s.githubClient(20 * time.Second).Do(req)
}

// latestRelease fetches the repo's newest release with a 60-second cache so
// the status endpoint can be polled freely.
func (s *Service) latestRelease(ctx context.Context) (*githubRelease, error) {
	s.upd.cacheMu.Lock()
	cached, at := s.upd.cacheLatest, s.upd.cacheAt
	s.upd.cacheMu.Unlock()
	if cached != nil && time.Since(at) < 60*time.Second {
		return cached, nil
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", s.cfg.GitHubRepo)
	resp, err := s.githubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusNotFound:
		// A repo with no releases yet answers 404 — that is "nothing newer",
		// not an error.
		return nil, nil
	default:
		return nil, newErr(502, fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, newErr(502, "GitHub API response unreadable")
	}
	s.upd.cacheMu.Lock()
	s.upd.cacheLatest = &rel
	s.upd.cacheAt = time.Now()
	s.upd.cacheMu.Unlock()
	return &rel, nil
}

// releaseByTag resolves a specific tag (used when the operator pins a
// version instead of taking the latest).
func (s *Service) releaseByTag(ctx context.Context, tag string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", s.cfg.GitHubRepo, tag)
	resp, err := s.githubRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, newErr(404, "找不到该版本的 Release："+tag)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, newErr(502, fmt.Sprintf("GitHub API returned %d", resp.StatusCode))
	}
	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, newErr(502, "GitHub API response unreadable")
	}
	return &rel, nil
}

// --- status ------------------------------------------------------------------

func (s *Service) UpdateStatus(ctx context.Context) (*UpdateStatusDTO, error) {
	latest, err := s.latestRelease(ctx)
	if err != nil {
		// A GitHub outage must not blind the status page: surface the current
		// build and local rollback options, leave `latest` empty.
		latest = nil
	}

	dto := &UpdateStatusDTO{
		Current: UpdateCurrentInfo{
			Version: version.Version,
			Commit:  version.Commit,
			Built:   version.BuildTime,
			Dirty:   version.Dirty == "true",
		},
		Job: s.upd.snapshot(),
	}
	if latest != nil {
		ver := strings.TrimPrefix(latest.TagName, "v")
		dto.Latest = &UpdateReleaseInfo{
			Tag:         latest.TagName,
			Version:     ver,
			Name:        latest.Name,
			PublishedAt: latest.PublishedAt,
			URL:         latest.HTMLURL,
			Notes:       latest.Body,
		}
		dto.UpdateAvailable = semverLess(version.Version, ver)
	}
	dto.Rollback = s.listRollbackOptions()
	dto.History = s.readHistory()
	if dto.History == nil {
		dto.History = []UpdateHistoryEntry{}
	}
	return dto, nil
}

// listRollbackOptions scans the app dir for lopezpress.v<ver> / web.v<ver>
// pairs — the copies an apply (or rollback) leaves behind.
func (s *Service) listRollbackOptions() []UpdateRollbackOption {
	appDir, err := s.updateAppDir()
	if err != nil {
		return []UpdateRollbackOption{}
	}
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return []UpdateRollbackOption{}
	}
	opts := []UpdateRollbackOption{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "lopezpress.v") {
			continue
		}
		ver := strings.TrimPrefix(name, "lopezpress.v")
		if !versionNameRe.MatchString(ver) {
			continue
		}
		webOK := false
		if info, err := os.Stat(filepath.Join(appDir, s.cfg.WebDir+".v"+ver)); err == nil && info.IsDir() {
			webOK = true
		}
		opts = append(opts, UpdateRollbackOption{Version: ver, Binary: true, Web: webOK})
	}
	sort.Slice(opts, func(i, j int) bool { return semverLess(opts[j].Version, opts[i].Version) })
	return opts
}

// --- apply -------------------------------------------------------------------

// StartUpdate validates the request and spawns the background job. The HTTP
// response returns immediately: nginx's 60 s read timeout makes a synchronous
// update impossible, so the UI polls the status endpoint instead.
func (s *Service) StartUpdate(target string) error {
	if version.Version == "0.0.0-dev" {
		return newErr(409, "当前是未烙印版本号的开发构建（go run），请在生产二进制上执行更新")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return newErr(409, "Release 资产只发布 linux-amd64，当前平台不支持在线更新")
	}
	if s.upd.isRunning() {
		return newErr(409, "已有更新任务正在进行")
	}
	target = strings.TrimSpace(target)
	if target != "" {
		target = strings.TrimPrefix(target, "v")
		if !versionNameRe.MatchString(target) {
			return newErr(400, "版本号格式不合法")
		}
	}
	if err := os.MkdirAll(s.stagingDir(), 0o755); err != nil {
		return newErr(500, "更新目录不可写："+err.Error())
	}

	s.upd.mu.Lock()
	if s.upd.running {
		s.upd.mu.Unlock()
		return newErr(409, "已有更新任务正在进行")
	}
	s.upd.running = true
	s.upd.job = UpdateJobState{Phase: updatePhaseChecking, Target: target, StartedAt: time.Now().UTC().Format(time.RFC3339)}
	s.upd.mu.Unlock()

	go s.runUpdate(target)
	return nil
}

func (s *Service) runUpdate(targetVersion string) {
	ctx := context.Background()
	defer func() {
		if r := recover(); r != nil {
			s.upd.failJob(fmt.Errorf("panic: %v", r))
		}
	}()

	// 1. Resolve the release.
	var rel *githubRelease
	var err error
	if targetVersion == "" {
		s.upd.setJob(updatePhaseChecking, "查询最新 Release…")
		rel, err = s.latestRelease(ctx)
		if rel == nil && err == nil {
			err = fmt.Errorf("仓库还没有任何 Release，先推送一个 v* tag")
		}
	} else {
		s.upd.setJob(updatePhaseChecking, "解析版本 v"+targetVersion+"…")
		rel, err = s.releaseByTag(ctx, "v"+targetVersion)
	}
	if err != nil {
		s.upd.failJob(err)
		return
	}
	if rel == nil {
		s.upd.failJob(fmt.Errorf("未找到 Release"))
		return
	}
	assetVersion := strings.TrimPrefix(rel.TagName, "v")
	assetName := fmt.Sprintf("lopezpress-%s-linux-amd64.tar.gz", assetVersion)
	var asset *githubAsset
	for i := range rel.Assets {
		if rel.Assets[i].Name == assetName {
			asset = &rel.Assets[i]
			break
		}
	}
	if asset == nil {
		s.upd.failJob(fmt.Errorf("Release %s 缺少资产 %s", rel.TagName, assetName))
		return
	}

	// 2. Download the asset into the staging dir.
	if err := os.MkdirAll(s.stagingDir(), 0o755); err != nil {
		s.upd.failJob(err)
		return
	}
	assetPath := filepath.Join(s.stagingDir(), assetName)
	s.upd.setJob(updatePhaseDownloading, fmt.Sprintf("下载 %s（%.1f MB）…", assetName, float64(asset.Size)/1024/1024))
	if err := s.downloadAsset(ctx, asset.BrowserDownloadURL, assetPath, asset.Size); err != nil {
		s.upd.failJob(err)
		return
	}

	// 3. Verify sha256 against the release's checksum asset.
	s.upd.setJob(updatePhaseVerifying, "校验 sha256…")
	if err := s.verifyChecksum(ctx, rel, assetName, assetPath); err != nil {
		s.upd.failJob(err)
		return
	}

	// 4. Database snapshot first: a bad update is recoverable code, lost data
	// is not. Reuses the backup-dir snapshot machinery (keep 3).
	s.upd.setJob(updatePhaseSnapshot, "拍摄数据库快照…")
	if _, err := s.snapshotBeforeImport(ctx); err != nil {
		s.upd.failJob(fmt.Errorf("数据库快照失败: %w", err))
		return
	}

	// 5. Swap binary + web dir, then schedule the restart.
	s.upd.setJob(updatePhaseSwapping, "解包并替换文件…")
	if err := s.swapIn(assetPath, assetVersion, "update"); err != nil {
		s.upd.failJob(err)
		return
	}

	s.upd.setJob(updatePhaseRestarting, "文件已就位，服务即将重启以加载新版本…")
	s.TriggerRestartAfterDelay(3 * time.Second)
}

func (s *Service) downloadAsset(ctx context.Context, url, dest string, size int64) error {
	const maxAssetBytes = 200 << 20 // sanity ceiling; a real asset is ~20 MB
	if size > maxAssetBytes {
		return newErr(502, "Release 资产超出预期大小，拒绝下载")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if tok := s.cfg.GitHubToken; tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := s.githubClient(10 * time.Minute).Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return newErr(502, fmt.Sprintf("资产下载返回 %d", resp.StatusCode))
	}
	f, err := os.Create(dest)
	if err != nil {
		return newErr(500, "无法写入暂存文件："+err.Error())
	}
	defer f.Close()
	// LimitReader, not MaxBytesReader: the latter needs an http.ResponseWriter
	// (nil here would panic on overflow) and we already capped `size` above.
	n, err := io.Copy(f, io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return fmt.Errorf("下载中断: %w", err)
	}
	if n > maxAssetBytes {
		return fmt.Errorf("资产超出大小上限 %d 字节，拒绝", maxAssetBytes)
	}
	if size > 0 && n != size {
		return fmt.Errorf("下载不完整: got %d want %d bytes", n, size)
	}
	return nil
}

// verifyChecksum downloads the release's sha256sums.txt and checks the asset
// against it. Missing checksum asset = refuse; the whole design leans on the
// HTTPS transport AND a second integrity signal.
func (s *Service) verifyChecksum(ctx context.Context, rel *githubRelease, assetName, assetPath string) error {
	var sumsURL string
	for _, a := range rel.Assets {
		if a.Name == "sha256sums.txt" {
			sumsURL = a.BrowserDownloadURL
			break
		}
	}
	if sumsURL == "" {
		return fmt.Errorf("Release 缺少 sha256sums.txt，拒绝应用（完整性无法确认）")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sumsURL, nil)
	if err != nil {
		return err
	}
	resp, err := s.githubClient(30 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("校验文件下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("校验文件下载返回 %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && (fields[1] == assetName || strings.TrimPrefix(fields[1], "*") == assetName) {
			want = strings.ToLower(fields[0])
			break
		}
	}
	if len(want) != 64 {
		return fmt.Errorf("sha256sums.txt 中没有 %s 的记录", assetName)
	}
	f, err := os.Open(assetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return fmt.Errorf("sha256 不匹配（got %s… want %s…），文件可能损坏或被篡改", got[:12], want[:12])
	}
	return nil
}

// swapIn unpacks the verified tar.gz, smoke-tests the new binary, backs up
// the current binary + web dir and moves the new ones into place. On any
// error before the first rename, nothing on disk has changed.
func (s *Service) swapIn(assetPath, newVersion, action string) error {
	appDir, err := s.updateAppDir()
	if err != nil {
		return err
	}
	binPath := filepath.Join(appDir, "lopezpress")
	webPath := filepath.Join(appDir, s.cfg.WebDir)

	// The updater assumes the standard deployment layout: the web dir next to
	// the binary. Anything else means this install is laid out differently
	// and the operator should update it by hand instead of half-updating it.
	if info, err := os.Stat(webPath); err != nil || !info.IsDir() {
		return newErr(409, fmt.Sprintf("前端目录 %s 不存在，布局与预期不符；请用 deploy/ 手动更新", webPath))
	}

	extractDir := filepath.Join(s.stagingDir(), "extract-"+time.Now().UTC().Format("20060102-150405"))
	if err := extractTarGz(assetPath, extractDir); err != nil {
		return fmt.Errorf("解包失败: %w", err)
	}
	newBin := filepath.Join(extractDir, "lopezpress")
	newWeb := filepath.Join(extractDir, "web")
	if info, err := os.Stat(newBin); err != nil || info.IsDir() {
		return fmt.Errorf("解包产物缺少 lopezpress 二进制")
	}
	if info, err := os.Stat(newWeb); err != nil || !info.IsDir() {
		return fmt.Errorf("解包产物缺少 web 目录")
	}
	// tar does not always preserve the exec bit; make sure.
	if err := os.Chmod(newBin, 0o755); err != nil {
		return err
	}

	// Smoke test: the new binary must at least identify itself. A build from
	// a broken tag dies HERE instead of taking the site down at boot.
	smokeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(smokeCtx, newBin, "-version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("新二进制自检失败: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), newVersion) {
		return fmt.Errorf("新二进制自报版本 %q 与 Release %s 不符", strings.TrimSpace(string(out)), newVersion)
	}

	cur := sanitizeVersion(version.Version)

	// Point of no return: everything below are renames inside one directory
	// (atomic per file on Linux) and the failure mode afterwards is "old
	// files still on disk as .v backups", which rollback + SSH can fix.
	backupBin := filepath.Join(appDir, "lopezpress.v"+cur)
	backupWeb := filepath.Join(appDir, s.cfg.WebDir+".v"+cur)
	_ = os.RemoveAll(backupBin) // a stale backup of the same version is worthless
	_ = os.RemoveAll(backupWeb)
	if err := os.Rename(binPath, backupBin); err != nil {
		return fmt.Errorf("备份当前二进制失败: %w", err)
	}
	if err := os.Rename(webPath, backupWeb); err != nil {
		_ = os.Rename(backupBin, binPath) // best-effort un-do
		return fmt.Errorf("备份当前前端目录失败: %w", err)
	}
	if err := os.Rename(newBin, binPath); err != nil {
		_ = os.Rename(backupBin, binPath)
		_ = os.Rename(backupWeb, webPath)
		return fmt.Errorf("安装新二进制失败: %w", err)
	}
	if err := os.Rename(newWeb, webPath); err != nil {
		_ = os.Rename(backupWeb, webPath)
		return fmt.Errorf("安装新前端目录失败: %w", err)
	}

	s.pruneUpdateBackups(appDir)
	s.appendHistory(UpdateHistoryEntry{
		Action: action,
		From:   version.Version,
		To:     newVersion,
		At:     time.Now().UTC().Format(time.RFC3339),
		Result: "pending-restart",
	})
	_ = os.MkdirAll(s.updateDir(), 0o755)
	_ = os.WriteFile(stateFile(s), []byte(fmt.Sprintf(
		`{"phase":%q,"target":%q,"from":%q,"to":%q,"at":%q}`,
		statePendingRestart, newVersion, version.Version, newVersion, time.Now().UTC().Format(time.RFC3339))), 0o644)

	// Clean the staging copy; keep the extracted dir for forensics until the
	// next run overwrites it.
	_ = os.Remove(assetPath)
	return nil
}

// RollbackUpdate swaps the running version for a kept lopezpress.v<ver> +
// web.v<ver> pair and schedules the restart. Local renames only: this is a
// second-scale operation and can stay synchronous.
func (s *Service) RollbackUpdate(target string) error {
	target = strings.TrimPrefix(strings.TrimSpace(target), "v")
	if !versionNameRe.MatchString(target) {
		return newErr(400, "版本号格式不合法")
	}
	if version.Version == "0.0.0-dev" {
		return newErr(409, "当前是开发构建，没有可回滚的版本烙印")
	}
	if s.upd.isRunning() {
		return newErr(409, "更新任务正在进行，无法回滚")
	}
	appDir, err := s.updateAppDir()
	if err != nil {
		return err
	}
	binPath := filepath.Join(appDir, "lopezpress")
	webPath := filepath.Join(appDir, s.cfg.WebDir)
	oldBin := filepath.Join(appDir, "lopezpress.v"+target)
	oldWeb := filepath.Join(appDir, s.cfg.WebDir+".v"+target)
	if info, err := os.Stat(oldBin); err != nil || info.IsDir() {
		return newErr(404, "找不到该版本的二进制备份："+target)
	}
	if info, err := os.Stat(oldWeb); err != nil || !info.IsDir() {
		return newErr(409, "该版本有二进制备份但没有配套的前端目录备份，拒绝半量回滚："+target)
	}

	cur := sanitizeVersion(version.Version)
	if cur != target {
		// The current version goes where the target's backup was; clear any
		// stale copies first. If the renames below fail midway, the backups
		// still exist under their own names — SSH can always finish the job.
		_ = os.RemoveAll(filepath.Join(appDir, "lopezpress.v"+cur))
		_ = os.RemoveAll(filepath.Join(appDir, s.cfg.WebDir+".v"+cur))
		if err := os.Rename(binPath, filepath.Join(appDir, "lopezpress.v"+cur)); err != nil {
			return fmt.Errorf("备份当前二进制失败: %w", err)
		}
		if err := os.Rename(webPath, filepath.Join(appDir, s.cfg.WebDir+".v"+cur)); err != nil {
			_ = os.Rename(filepath.Join(appDir, "lopezpress.v"+cur), binPath)
			return fmt.Errorf("备份当前前端目录失败: %w", err)
		}
	} else {
		// Re-applying the same version (repair): drop the duplicate backups
		// so the renames below have somewhere to land.
		_ = os.RemoveAll(oldBin)
		_ = os.RemoveAll(oldWeb)
	}
	if err := os.Rename(oldBin, binPath); err != nil {
		return fmt.Errorf("恢复旧二进制失败: %w", err)
	}
	if err := os.Rename(oldWeb, webPath); err != nil {
		return fmt.Errorf("恢复旧前端目录失败: %w", err)
	}

	s.appendHistory(UpdateHistoryEntry{
		Action: "rollback",
		From:   version.Version,
		To:     target,
		At:     time.Now().UTC().Format(time.RFC3339),
		Result: "pending-restart",
	})
	_ = os.MkdirAll(s.updateDir(), 0o755)
	_ = os.WriteFile(stateFile(s), []byte(fmt.Sprintf(
		`{"phase":%q,"target":%q,"from":%q,"to":%q,"at":%q}`,
		statePendingRestart, target, version.Version, target, time.Now().UTC().Format(time.RFC3339))), 0o644)

	s.TriggerRestartAfterDelay(1 * time.Second)
	return nil
}

// FinishPendingUpdate runs once on boot. If the previous process exited for
// a planned update, close the history entry as a success; a stale
// pending-restart entry with no state file means the previous process died
// before or without a restart, so mark it honestly as unconfirmed.
func (s *Service) FinishPendingUpdate() {
	data, err := os.ReadFile(stateFile(s))
	if err != nil {
		return
	}
	_ = os.Remove(stateFile(s))
	var st struct {
		Phase string `json:"phase"`
		To    string `json:"to"`
		At    string `json:"at"`
	}
	if json.Unmarshal(data, &st) != nil || st.Phase != statePendingRestart {
		return
	}
	if version.Version == st.To || st.To == "" {
		s.appendHistory(UpdateHistoryEntry{
			Action: "note",
			From:   st.To,
			To:     version.Version,
			At:     time.Now().UTC().Format(time.RFC3339),
			Result: "success",
			Note:   "新版本启动成功，更新完成",
		})
	} else {
		s.appendHistory(UpdateHistoryEntry{
			Action: "note",
			From:   st.To,
			To:     version.Version,
			At:     time.Now().UTC().Format(time.RFC3339),
			Result: "failed",
			Note:   fmt.Sprintf("计划更新到 %s，但本次启动运行的是 %s", st.To, version.Version),
		})
	}
}

// --- history -----------------------------------------------------------------

func (s *Service) readHistory() []UpdateHistoryEntry {
	data, err := os.ReadFile(historyFile(s))
	if err != nil {
		return nil
	}
	var entries []UpdateHistoryEntry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	// Newest first.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	if len(entries) > 20 {
		entries = entries[:20]
	}
	return entries
}

func (s *Service) appendHistory(e UpdateHistoryEntry) {
	entries := []UpdateHistoryEntry{}
	if data, err := os.ReadFile(historyFile(s)); err == nil {
		_ = json.Unmarshal(data, &entries)
	}
	entries = append(entries, e)
	if len(entries) > 50 {
		entries = entries[len(entries)-50:]
	}
	if data, err := json.Marshal(entries); err == nil {
		_ = os.MkdirAll(s.updateDir(), 0o755)
		_ = os.WriteFile(historyFile(s), data, 0o644)
	}
}

// pruneUpdateBackups keeps the newest updateKeepBackups version pairs.
func (s *Service) pruneUpdateBackups(appDir string) {
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return
	}
	var vers []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if v, ok := strings.CutPrefix(e.Name(), "lopezpress.v"); ok && versionNameRe.MatchString(v) {
			vers = append(vers, v)
		}
	}
	if len(vers) <= updateKeepBackups {
		return
	}
	sort.Slice(vers, func(i, j int) bool { return semverLess(vers[j], vers[i]) })
	for _, v := range vers[updateKeepBackups:] {
		_ = os.Remove(filepath.Join(appDir, "lopezpress.v"+v))
		_ = os.RemoveAll(filepath.Join(appDir, s.cfg.WebDir+".v"+v))
	}
}

// --- tar / version helpers ----------------------------------------------------

// extractTarGz unpacks a flat layout (lopezpress + web/) into dest. Every
// member name is validated against zip-slip before any byte is written.
func extractTarGz(assetPath, dest string) error {
	f, err := os.Open(assetPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := hdr.Name
		if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
			return fmt.Errorf("tar 成员名不安全: %q", name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode & 0o777)
			if mode == 0 {
				mode = 0o644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			// tar entries carry their declared size; a truncated stream ends
			// with an EOF error from Copy which we surface.
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			// Symlinks and friends are never part of our release asset; skip
			// rather than honor them.
			continue
		}
	}
	return nil
}

func sanitizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	if versionNameRe.MatchString(v) {
		return v
	}
	return "unknown"
}

// semverLess compares dotted numeric versions ("1.0.31" < "1.0.32"). Unknown
// shapes fall back to a plain string compare; "0.0.0-dev" is always the
// smallest because its last segment is not numeric.
func semverLess(a, b string) bool {
	if a == b {
		return false
	}
	as := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bs := strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		ai, aerr := strconv.Atoi(as[i])
		bi, berr := strconv.Atoi(bs[i])
		switch {
		case aerr == nil && berr == nil:
			if ai != bi {
				return ai < bi
			}
		default:
			if as[i] != bs[i] {
				return as[i] < bs[i]
			}
		}
	}
	return len(as) < len(bs)
}
