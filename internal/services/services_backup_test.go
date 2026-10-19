package services

import (
	"context"
	"net/http"
	"testing"
	"time"

	"echo-content-api/internal/models"
)

// TestImportBackupDryRun verifies that a dry-run import validates the payload,
// returns the expected stats, and leaves the database untouched.
func TestImportBackupDryRun(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// Seed the live DB so we can assert it is NOT modified by a dry run.
	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")
	_ = makePostNoTime(t, db, admin, "published", "live body", "Live Post")
	var liveCount int64
	db.Model(&models.Post{}).Count(&liveCount)

	// Build an import payload that would wipe + replace the DB.
	in := ImportData{
		SchemaVersion: BackupSchemaVersion,
		AppVersion:    BackupAppVersion,
		ExportedAt:    time.Now().UTC().Format(time.RFC3339),
		Users:         []models.User{makeUser(t, db, "newadmin", "na@example.com", "admin")},
	}
	res, err := svc.ImportBackup(ctx, root, in, true)
	if err != nil {
		t.Fatalf("dry-run import: %v", err)
	}
	if !res.DryRun || res.Applied {
		t.Fatalf("expected dry-run (not applied), got %+v", res)
	}
	if res.SnapshotFile != "" {
		t.Fatalf("dry-run must not create a snapshot, got %q", res.SnapshotFile)
	}
	// Live DB must be completely unchanged.
	var after int64
	db.Model(&models.Post{}).Count(&after)
	if after != liveCount {
		t.Fatalf("dry-run modified the DB: before=%d after=%d", liveCount, after)
	}
}

// TestImportBackupSchemaValidation verifies that a mismatched schema_version
// or app_version is rejected with 400 before any write.
func TestImportBackupSchemaValidation(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", "root")

	// Wrong schema version.
	bad := ImportData{SchemaVersion: 99, Users: []models.User{}}
	if _, err := svc.ImportBackup(ctx, root, bad, false); err == nil {
		t.Fatalf("expected schema_version mismatch to be rejected")
	} else if StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (err=%v)", StatusOf(err), err)
	}

	// Wrong app version.
	bad2 := ImportData{AppVersion: "some-other-app"}
	if _, err := svc.ImportBackup(ctx, root, bad2, false); err == nil {
		t.Fatalf("expected app_version mismatch to be rejected")
	} else if StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (err=%v)", StatusOf(err), err)
	}

	// Legacy payload (no metadata at all) must still be accepted so older
	// backups keep working.
	legacy := ImportData{Users: []models.User{makeUser(t, db, "legacy", "l@example.com", "user")}}
	res, err := svc.ImportBackup(ctx, root, legacy, true)
	if err != nil {
		t.Fatalf("legacy payload should pass validation: %v", err)
	}
	if !res.DryRun {
		t.Fatalf("expected dry-run")
	}
}

// TestImportBackupAppliedTakesSnapshot verifies that a real (non-dry-run)
// import actually replaces the data AND returns the auto-snapshot file name.
func TestImportBackupAppliedTakesSnapshot(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// Seed live data that the import will replace.
	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")
	_ = makePostNoTime(t, db, admin, "published", "old", "Old Post")

	in := ImportData{
		SchemaVersion: BackupSchemaVersion,
		AppVersion:    BackupAppVersion,
		Users:         []models.User{makeUser(t, db, "fresh", "f@example.com", "admin")},
		Posts:         []models.Post{makePostNoTime(t, db, admin, "published", "new", "New Post")},
	}
	res, err := svc.ImportBackup(ctx, root, in, false)
	if err != nil {
		t.Fatalf("applied import: %v", err)
	}
	if res.DryRun || !res.Applied {
		t.Fatalf("expected applied import, got %+v", res)
	}
	if res.SnapshotFile == "" {
		t.Fatalf("applied import must create + return a snapshot file")
	}
	// The live DB should now reflect the imported user, and the OLD user gone.
	var got int64
	db.Model(&models.User{}).Where("username = ?", "boss").Count(&got)
	if got != 0 {
		t.Fatalf("old user 'boss' should have been replaced, still present")
	}
	var fresh int64
	db.Model(&models.User{}).Where("username = ?", "fresh").Count(&fresh)
	if fresh != 1 {
		t.Fatalf("imported user 'fresh' should be present")
	}
}
