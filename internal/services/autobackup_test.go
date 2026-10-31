package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// autoBackupTestService points the backup dir at a temp folder so the test
// never writes into the project's data/ directory.
func autoBackupTestService(t *testing.T) (*Service, func(enabled bool, keep int)) {
	t.Helper()
	svc, db := newTestService(t)
	dir := t.TempDir()
	svc.cfg.BackupDir = dir

	set := func(enabled bool, keep int) {
		if err := db.Model(&models.SiteConfig{}).Where("id = ?", 1).
			Updates(map[string]any{"auto_backup": enabled, "backup_keep": keep}).Error; err != nil {
			t.Fatalf("set backup settings: %v", err)
		}
	}
	return svc, set
}

// Off by default means the job must not write anything.
func TestAutoBackupOffWritesNothing(t *testing.T) {
	svc, set := autoBackupTestService(t)
	set(false, 7)

	name, err := svc.MaybeAutoBackup(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "" {
		t.Fatalf("expected no snapshot when disabled, got %q", name)
	}
	entries, _ := os.ReadDir(svc.cfg.BackupDir)
	if len(entries) != 0 {
		t.Fatalf("expected an empty backup dir, found %d entries", len(entries))
	}
}

// At most one snapshot per UTC day; the second call is a no-op.
func TestAutoBackupOncePerDay(t *testing.T) {
	svc, set := autoBackupTestService(t)
	set(true, 7)

	name, err := svc.MaybeAutoBackup(context.Background())
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	want := "auto-" + time.Now().UTC().Format("20060102") + ".db"
	if name != want {
		t.Fatalf("expected %q, got %q", want, name)
	}
	if _, err := os.Stat(filepath.Join(svc.cfg.BackupDir, name)); err != nil {
		t.Fatalf("snapshot file missing: %v", err)
	}

	again, err := svc.MaybeAutoBackup(context.Background())
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if again != "" {
		t.Fatalf("expected the second run to be a no-op, got %q", again)
	}
}

// Retention keeps the newest `keep` files and deletes the rest.
func TestAutoBackupPrunesOldSnapshots(t *testing.T) {
	svc, set := autoBackupTestService(t)
	set(true, 3)

	// Seed five older days. Names sort chronologically, matching the prune
	// assumption documented in the service.
	for _, d := range []string{"20250101", "20250102", "20250103", "20250104", "20250105"} {
		f, err := os.Create(filepath.Join(svc.cfg.BackupDir, "auto-"+d+".db"))
		if err != nil {
			t.Fatalf("seed %s: %v", d, err)
		}
		_ = f.Close()
	}

	if _, err := svc.MaybeAutoBackup(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	var names []string
	entries, _ := os.ReadDir(svc.cfg.BackupDir)
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 files after prune, got %d: %v", len(names), names)
	}
	today := "auto-" + time.Now().UTC().Format("20060102") + ".db"
	found := false
	for _, n := range names {
		if n == today {
			found = true
		}
		if n == "auto-20250101.db" || n == "auto-20250102.db" || n == "auto-20250103.db" {
			t.Fatalf("oldest snapshots should have been pruned, %s survived", n)
		}
	}
	if !found {
		t.Fatalf("today's snapshot should survive pruning, got %v", names)
	}
}

// A pre-import snapshot is never deleted by the auto prune —
// they are separate retention policies.
func TestAutoBackupPruneIgnoresImportSnapshots(t *testing.T) {
	svc, set := autoBackupTestService(t)
	set(true, 1)

	importSnap := filepath.Join(svc.cfg.BackupDir, "snapshot-20240101-120000.db")
	if err := os.WriteFile(importSnap, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed import snapshot: %v", err)
	}
	for _, d := range []string{"20250101", "20250102"} {
		if err := os.WriteFile(filepath.Join(svc.cfg.BackupDir, "auto-"+d+".db"), []byte("x"), 0o644); err != nil {
			t.Fatalf("seed auto %s: %v", d, err)
		}
	}
	if _, err := svc.MaybeAutoBackup(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := os.Stat(importSnap); err != nil {
		t.Fatalf("pre-import snapshot must survive the auto prune: %v", err)
	}
}

// ClampBackupKeep must never return 0 (which would delete every snapshot).
func TestClampBackupKeep(t *testing.T) {
	cases := map[int]int{-5: 7, 0: 7, 1: 1, 7: 7, 90: 90, 500: 90}
	for in, want := range cases {
		if got := ClampBackupKeep(in); got != want {
			t.Fatalf("ClampBackupKeep(%d) = %d, want %d", in, got, want)
		}
	}
}
