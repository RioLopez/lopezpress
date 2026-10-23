package services

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// Auto-backup (spec 34).
//
// The rule that shaped this file: a personal blog on a 1 vCPU box should not
// need cron, systemd timers, or an external agent to keep its data safe. One
// VACUUM INTO per day is cheap (SQLite copies the file atomically), and the
// decision "has today's backup run yet" is a filename lookup, not a scheduler.
//
// Consequence worth stating: the check is lazy. If the process is not running
// at the moment the day flips, the snapshot happens the next time it is — the
// guarantee is "at most one per UTC day", not "exactly one at 03:00".

const (
	// autoBackupPrefix marks snapshots created by this job. It is deliberately
	// different from the pre-import prefix ("snapshot-") so that an import at
	// 10:00 cannot make the daily job think the day is already covered.
	autoBackupPrefix = "auto-"
	// minBackupKeep / maxBackupKeep bound the retention the operator may set.
	minBackupKeep = 1
	maxBackupKeep = 90
)

// ClampBackupKeep keeps the retention inside a sane window: at least one copy,
// at most 90 (a quarter). Values outside the range silently fall back to a
// week instead of being stored as typed, so the prune step can never be told
// to delete everything.
func ClampBackupKeep(n int) int {
	if n < minBackupKeep {
		return 7
	}
	if n > maxBackupKeep {
		return maxBackupKeep
	}
	return n
}

// AutoBackupStatus is what the settings page shows about the daily snapshot.
type AutoBackupStatus struct {
	Enabled bool   `json:"enabled"`
	Keep    int    `json:"keep"`
	LastAt  string `json:"last_at"` // RFC3339, empty when never run
	Files   int    `json:"files"`
}

// MaybeAutoBackup takes today's snapshot when the feature is on and today's
// file does not exist yet. It is safe to call on every boot and on a coarse
// timer: the happy path is one os.Stat.
//
// Returns the created file name ("" when nothing was due).
func (s *Service) MaybeAutoBackup(ctx context.Context) (string, error) {
	cfg, err := s.GetSiteConfig(ctx, true)
	if err != nil {
		return "", err
	}
	if !cfg.AutoBackup {
		return "", nil
	}
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}
	name := autoBackupPrefix + time.Now().UTC().Format("20060102") + ".db"
	target := filepath.Join(dir, name)
	if _, statErr := os.Stat(target); statErr == nil {
		return "", nil // today is already covered
	}

	// VACUUM INTO fails if the target exists, which is exactly the guard we
	// want against two workers racing on the same file.
	if err := s.db.WithContext(ctx).Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", fmt.Errorf("vacuum into %s: %w", name, err)
	}
	s.pruneAutoBackups(dir, ClampBackupKeep(cfg.BackupKeep))
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

// AutoBackupReport describes the current state for the settings/backup pages.
func (s *Service) AutoBackupReport(ctx context.Context, actor models.User) (AutoBackupStatus, error) {
	if !IsRoot(actor) {
		return AutoBackupStatus{}, newErr(http.StatusForbidden, "only the root can inspect backups")
	}
	cfg, err := s.GetSiteConfig(ctx, true)
	if err != nil {
		return AutoBackupStatus{}, err
	}
	st := AutoBackupStatus{Enabled: cfg.AutoBackup, Keep: ClampBackupKeep(cfg.BackupKeep)}
	entries, err := os.ReadDir(s.backupDir())
	if err != nil {
		return st, nil // no directory yet: everything stays zero-valued
	}
	var newest time.Time
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, autoBackupPrefix) || !strings.HasSuffix(n, ".db") {
			continue
		}
		st.Files++
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	if !newest.IsZero() {
		st.LastAt = newest.UTC().Format(time.RFC3339)
	}
	return st, nil
}
