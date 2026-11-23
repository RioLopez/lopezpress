package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"lopezpress/internal/services"
)

// Self-update surface (root-only; the router table enforces that, handlers do
// not re-check roles — see the P4 note in router.go).
//
// The three endpoints deliberately mirror the backup surface's shape:
//   - status is a poll target (the UI refreshes it every couple of seconds
//     while a job runs), so it never talks to GitHub synchronously on the hot
//     path beyond the 60-second cache inside the service;
//   - apply returns 202 immediately and the work continues in a background
//     goroutine — nginx's 60 s proxy_read_timeout would kill a synchronous
//     download-and-swap, so "fire + poll" is the only honest shape;
//   - rollback is local file renames and genuinely finishes fast.

// GetUpdateStatus reports the running build, the repo's latest release
// (cached), rollback options on disk, the current job state and history.
// GET /api/admin/update/status (auth, root)
func GetUpdateStatus(w http.ResponseWriter, r *http.Request) {
	dto, err := svc.UpdateStatus(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

// ApplyUpdate starts an update job toward the latest release (empty body /
// empty version) or a pinned tag ("1.0.32" / "v1.0.32").
// POST /api/admin/update/apply  body: { "version": "" } (auth, root)
func ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version string `json:"version"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			// An empty body is a valid "update to latest"; anything unparsable
			// that is not empty is a client bug.
			if r.ContentLength != 0 {
				writeError(w, http.StatusBadRequest, "请求体不是合法 JSON")
				return
			}
		}
	}
	if err := svc.StartUpdate(in.Version); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"started": true})
}

// RollbackUpdate swaps the running build for a kept lopezpress.v<ver> +
// web.v<ver> pair, then restarts. Returns before the restart fires so the
// client can render the "restarting" state.
// POST /api/admin/update/rollback  body: { "version": "1.0.30" } (auth, root)
func RollbackUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "请求体不是合法 JSON")
		return
	}
	if err := svc.RollbackUpdate(in.Version); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"restarting": true})
}

// UploadUpdate accepts a release asset uploaded by the operator and runs the
// same verify → snapshot → swap pipeline as the online update. This is the
// offline path: servers that cannot reach GitHub (or a mirror) can still be
// updated, because the operator downloads the asset anywhere and hands it in
// through this endpoint.
//
// The stream is written to the staging dir by the service; only then does the
// job start (202). All integrity gates still apply: optional client sha256,
// the -version smoke test on the extracted binary, the DB snapshot and the
// rollback backups.
// POST /api/admin/update/upload  multipart: file=<asset.tar.gz>, sha256=<optional> (auth, root)
func UploadUpdate(w http.ResponseWriter, r *http.Request) {
	// Belt-and-suspenders cap: the router table already limits the body; this
	// also guards direct exposure without nginx.
	r.Body = http.MaxBytesReader(w, r.Body, services.MaxUpdateUploadBytes+(1<<20))
	const maxFormBytes = 1 << 20
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "上传文件过大（上限 64MB）")
			return
		}
		writeError(w, http.StatusBadRequest, "无效的 multipart 表单")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "缺少 file 字段")
		return
	}
	defer file.Close()
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".tar.gz") {
		writeError(w, http.StatusBadRequest, "请上传 .tar.gz 更新包（Release 页下载的 lopezpress-<ver>-linux-amd64.tar.gz）")
		return
	}

	assetPath, err := svc.StageUpdateUpload(file, header.Size)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	sha := strings.TrimSpace(r.FormValue("sha256"))
	if err := svc.StartUpdateFromUpload(assetPath, sha); err != nil {
		svc.DiscardStagedUpload(assetPath)
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"started": true})
}
