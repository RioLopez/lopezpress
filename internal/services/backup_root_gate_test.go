package services

import (
	"context"
	"net/http"
	"testing"

	"echo-content-api/internal/models"
)

// TestImportAndRestoreAreRootOnly is the regression test for the as-built
// audit finding Q7: a backup payload is attacker-controllable JSON that can
// carry arbitrary user rows (including role "root" and foreign password
// hashes), and RestoreSnapshot replaces the whole database too. Before the
// gate, any admin could import a hand-crafted backup and mint/overwrite a
// root account — a full takeover path around the "root is never created via
// API" invariant. Both operations are now root-only at the service layer.
func TestImportAndRestoreAreRootOnly(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// An import payload crafted to mint a second root. The user row is a
	// hand-built struct (never written to the live DB by the fixture) so it
	// only exists inside the payload — exactly like an attacker's JSON.
	evil := ImportData{
		SchemaVersion: BackupSchemaVersion,
		AppVersion:    BackupAppVersion,
		Users: []models.User{
			{ID: "evil-root-id", Username: "evil", Email: "evil@example.com", Role: models.RoleRoot, PasswordHash: "x"},
		},
	}

	// 1. A plain admin cannot import — not even a dry run (the dry run would
	//    otherwise leak the ability to probe, and the gate is cheapest here).
	if _, err := svc.ImportBackup(ctx, admin, evil, true); StatusOf(err) != http.StatusForbidden {
		t.Errorf("admin ImportBackup: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// 2. An admin cannot restore a snapshot either.
	if _, err := svc.RestoreSnapshot(ctx, admin, "snapshot-20260101-000000.db"); StatusOf(err) != http.StatusForbidden {
		t.Errorf("admin RestoreSnapshot: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// 3. The root passes the gate (dry run here so nothing is destroyed;
	//    the schema-version gate still applies afterwards).
	if _, err := svc.ImportBackup(ctx, root, evil, true); err != nil {
		t.Errorf("root ImportBackup dry-run: expected success, got %v", err)
	}
	// 4. The crafted root account was never created by the rejected import.
	var rootCount int64
	db.Model(&models.User{}).Where("role = ?", "root").Count(&rootCount)
	if rootCount != 1 {
		t.Errorf("expected exactly one root after rejected imports, got %d", rootCount)
	}
}
