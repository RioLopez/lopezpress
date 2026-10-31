package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestListSnapshotsRootOnly covers the snapshot listing: the endpoint
// behind the new restore UI is root-only (a snapshot holds every account and
// password hash — same gate as RestoreSnapshot), returns
// an empty list (not an error) when the directory is absent, and filters to
// snapshot-*.db files only.
func TestListSnapshotsRootOnly(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// 1. A plain admin cannot list snapshots.
	if _, err := svc.ListSnapshots(ctx, admin); StatusOf(err) != http.StatusForbidden {
		t.Errorf("admin ListSnapshots: expected 403, got %v (err=%v)", StatusOf(err), err)
	}

	// 2. Root with no backup directory yet -> empty list, no error.
	svc.cfg.BackupDir = filepath.Join(t.TempDir(), "does-not-exist")
	snaps, err := svc.ListSnapshots(ctx, root)
	if err != nil {
		t.Fatalf("root ListSnapshots (missing dir): %v", err)
	}
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots for a missing dir, got %d", len(snaps))
	}

	// 3. Root with a directory holding two snapshots + one non-snapshot file:
	//    only the snapshots are listed, with their sizes.
	dir := t.TempDir()
	svc.cfg.BackupDir = dir
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("snapshot-20260101-000000.db", "a")
	write("snapshot-20260102-000000.db", "bb")
	write("backup-20260101-000000.json", "not a snapshot")

	snaps, err = svc.ListSnapshots(ctx, root)
	if err != nil {
		t.Fatalf("root ListSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots (non-snapshot filtered), got %d: %+v", len(snaps), snaps)
	}
	sizes := map[string]int64{}
	for _, s := range snaps {
		sizes[s.Name] = s.Size
	}
	if sizes["snapshot-20260101-000000.db"] != 1 || sizes["snapshot-20260102-000000.db"] != 2 {
		t.Errorf("unexpected snapshot sizes: %+v", sizes)
	}
}
