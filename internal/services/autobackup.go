package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// Auto-backup.
//
// The rule that shaped this file: a personal blog on a 1 vCPU box should not
// need cron, systemd timers, or an external agent to keep its data safe. One
// VACUUM INTO per interval is cheap (SQLite copies the file atomically), and
// the decision "has the next snapshot run yet" is a filename lookup, not a
// scheduler.
//
// Consequence worth stating: the check is lazy. If the process is not running
// at the moment the interval flips, the snapshot happens the next time it is —
// the guarantee is "at most one per interval", not "exactly one at 03:00".
//
// What the database snapshot does NOT contain is the upload directory: article
// images and PDF attachments live as files on disk. BackupIncludeUploads adds
// a point-in-time copy of that directory (see snapshotUploads), which uses hard
// links for unchanged files so a week of snapshots does not multiply the disk
// usage.

const (
	// autoBackupPrefix marks snapshots created by this job. It is deliberately
	// different from the pre-import prefix ("snapshot-") so that an import at
	// 10:00 cannot make the daily job think the day is already covered.
	autoBackupPrefix = "auto-"
	// uploadsSnapshotPrefix marks the file-level companion of one database
	// snapshot. The date in both names is the same, so the pair can be found
	// and pruned together.
	uploadsSnapshotPrefix = "uploads-"
	// minBackupKeep / maxBackupKeep bound the retention the operator may set.
	minBackupKeep = 1
	maxBackupKeep = 90
	// minBackupInterval / maxBackupInterval bound how often the job may run.
	// Daily is the default; monthly is the slowest useful cadence.
	minBackupInterval = 1
	maxBackupInterval = 30
)

// MaybeAutoBackup takes the next snapshot when the feature is on and the
// interval is due. Safe to call on every boot and on a coarse timer: the happy
// path is a few os.Stat calls.
func (s *Service) MaybeAutoBackup(ctx context.Context) (string, error) {
	return s.autoBackup(ctx, false)
}

// MaybeAutoBackupNow is the "snapshot now" button: it skips the interval and
// hour gates but still refuses to overwrite a file it already wrote today.
func (s *Service) MaybeAutoBackupNow(ctx context.Context) (string, error) {
	return s.autoBackup(ctx, true)
}

func (s *Service) autoBackup(ctx context.Context, force bool) (string, error) {
	cfg, err := s.GetSiteConfig(ctx, true)
	if err != nil {
		return "", err
	}
	if !cfg.AutoBackup {
		return "", nil
	}
	now := time.Now()
	if !force {
		// 0 = no preferred hour, i.e. run as soon as the interval is due.
		if hour := ClampBackupHour(cfg.BackupHour); hour > 0 && now.Hour() < hour {
			return "", nil
		}
	}
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	name := autoBackupPrefix + now.UTC().Format("20060102") + ".db"
	target := filepath.Join(dir, name)
	if _, statErr := os.Stat(target); statErr == nil {
		return "", nil // today is already covered
	}
	if !force && !autoBackupDue(dir, now, ClampBackupInterval(cfg.BackupIntervalDays)) {
		return "", nil
	}

	// VACUUM INTO fails if the target exists, which is exactly the guard we
	// want against two workers racing on the same file.
	if err := s.db.WithContext(ctx).Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", fmt.Errorf("vacuum into %s: %w", name, err)
	}
	keep := ClampBackupKeep(cfg.BackupKeep)
	s.pruneAutoBackups(dir, keep)
	// The file-level companion is best-effort: a failed copy must not turn a
	// successful database snapshot into an error the scheduler retries forever.
	if cfg.BackupIncludeUploads {
		if _, copyErr := s.snapshotUploads(dir, now); copyErr == nil {
			s.pruneUploadSnapshots(dir, keep)
		}
	}
	// Attribute the row to the caller when there is one (the "run now" button
	// in the admin UI), otherwise to the scheduler. A daily snapshot that
	// nobody requested still deserves a line in the audit trail.
	actor := ActorFromContext(ctx)
	if actor.ID == "" {
		actor = SystemActor()
	}
	s.audit(ctx, actor, "auto_backup", "backup", "file="+name)
	return name, nil
}

// pruneAutoBackups deletes the oldest auto-*.db files beyond keep. Only files
// this job created are touched — a pre-import snapshot is never removed here.
func (s *Service) pruneAutoBackups(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, autoBackupPrefix) && strings.HasSuffix(n, ".db") {
			names = append(names, n)
		}
	}
	if len(names) <= keep {
		return
	}
	// auto-YYYYMMDD.db sorts chronologically as a plain string.
	slices.Sort(names)
	for _, old := range names[:len(names)-keep] {
		_ = os.Remove(filepath.Join(dir, old))
	}
}

// snapshotUploads writes a point-in-time copy of the upload directory next to
// the database snapshot.
//
// Unchanged files are HARDLINKED to the previous snapshot instead of copied.
// That is what makes "also back up images and PDFs" affordable on a small VPS:
// a week of daily snapshots costs roughly one copy of the files plus the new
// bytes of each day, not seven full copies. If the filesystem refuses a hard
// link (different volume, exotic FS, Windows without privilege), it falls back
// to a regular copy, so correctness never depends on the optimisation.
func (s *Service) snapshotUploads(dir string, now time.Time) (string, error) {
	name := uploadsSnapshotPrefix + now.UTC().Format("20060102")
	dest := filepath.Join(dir, name)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return name, nil // already captured today
	}
	src := s.uploadDir()
	if info, err := os.Stat(src); err != nil || !info.IsDir() {
		return "", nil // nothing uploaded yet
	}
	prev := latestUploadsSnapshot(dir, name)
	if err := copyTreeHardlinked(src, dest, prev); err != nil {
		return "", err
	}
	return name, nil
}

// latestUploadsSnapshot returns the newest uploads-YYYYMMDD directory other
// than `exclude`.
func latestUploadsSnapshot(dir, exclude string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	best := ""
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() || n == exclude ||
			!strings.HasPrefix(n, uploadsSnapshotPrefix) {
			continue
		}
		if n > best { // uploads-YYYYMMDD sorts chronologically
			best = n
		}
	}
	if best == "" {
		return ""
	}
	return filepath.Join(dir, best)
}

// copyTreeHardlinked copies src into dst, reusing hard links from prev when the
// file is unchanged (same size and modification time). prev may be empty.
func copyTreeHardlinked(src, dst, prev string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil // skip sockets, devices, symlinks
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if prev != "" {
			if old, err := os.Stat(filepath.Join(prev, rel)); err == nil &&
				old.Mode().IsRegular() && old.Size() == info.Size() &&
				old.ModTime().Equal(info.ModTime()) {
				if os.Link(filepath.Join(prev, rel), target) == nil {
					return nil
				}
			}
		}
		return copyFilePreservingTime(path, target, info)
	})
}

// copyFilePreservingTime copies the bytes and copies the source mtime onto the
// destination. The mtime is the whole basis of the hard-link reuse check: a
// copied file that got "now" as its timestamp would never compare equal to the
// original on the next run, so every later snapshot would copy it again and the
// incremental design would silently become a full copy.
func copyFilePreservingTime(src, dst string, info os.FileInfo) error {
	if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
		return err
	}
	_ = os.Chtimes(dst, info.ModTime(), info.ModTime())
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// pruneUploadSnapshots keeps the newest `keep` uploads-* directories. Deleting
// a directory drops only that snapshot's hard links; files that another
// snapshot still references stay on disk.
func (s *Service) pruneUploadSnapshots(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), uploadsSnapshotPrefix) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return
	}
	slices.Sort(names)
	for _, old := range names[:len(names)-keep] {
		_ = os.RemoveAll(filepath.Join(dir, old))
	}
}

// AutoBackupReport describes the current state for the settings/backup pages.
func (s *Service) AutoBackupReport(ctx context.Context, actor models.User) (AutoBackupStatus, error) {
	if !IsRoot(actor) {
		return AutoBackupStatus{}, newErr(http.StatusForbidden, "only the root can inspect backups")
	}
	cfg, err := s.GetSiteConfig(ctx, true)
	if err != nil {
		return AutoBackupStatus{}, err
	}
	st := AutoBackupStatus{
		Enabled:        cfg.AutoBackup,
		Keep:           ClampBackupKeep(cfg.BackupKeep),
		IntervalDays:   ClampBackupInterval(cfg.BackupIntervalDays),
		Hour:           ClampBackupHour(cfg.BackupHour),
		IncludeUploads: cfg.BackupIncludeUploads,
		Subdir:         cfg.BackupSubdir,
		Dir:            s.backupDir(),
		UploadsDir:     s.uploadDir(),
	}
	// The hour field is interpreted on the server clock, so the page must say
	// what that clock reads. Without it, an operator in UTC+8 reads "04:00" as
	// their own morning while the job runs at noon.
	now := time.Now()
	st.ServerNow = now.Format("2006-01-02 15:04")
	st.ServerZone = now.Format("MST (-07:00)")
	if abs, absErr := filepath.Abs(st.Dir); absErr == nil {
		st.DirAbs = abs
	}
	if abs, absErr := filepath.Abs(st.UploadsDir); absErr == nil {
		st.UploadsDirAbs = abs
	}
	fillBackupDirStats(&st)
	return st, nil
}
