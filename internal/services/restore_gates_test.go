package services

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/models"
)

// writeFakeSnapshot builds a snapshot-shaped SQLite file in the service's backup
// directory. `users` and `posts` decide what it holds, which is the whole input
// to the restore gates.
func writeFakeSnapshot(t *testing.T, dir, name string, users, posts int) {
	t.Helper()
	path := filepath.Join(dir, name)
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate snapshot: %v", err)
	}
	for i := 0; i < users; i++ {
		u := models.User{
			ID:           "snap-user-" + string(rune('a'+i)),
			Username:     "snap" + string(rune('a'+i)),
			Email:        "snap" + string(rune('a'+i)) + "@example.com",
			Role:         models.RoleRoot,
			PasswordHash: "x",
		}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed snapshot user: %v", err)
		}
	}
	for i := 0; i < posts; i++ {
		p := models.Post{
			ID:       "snap-post-" + string(rune('a'+i)),
			AuthorID: "snap-user-a",
			Title:    "snap",
			Body:     "snap",
			Status:   models.PostPublished,
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("seed snapshot post: %v", err)
		}
	}
	if sqlDB, e := db.DB(); e == nil {
		_ = sqlDB.Close()
	}
}

// R7-04: restore-snapshot was the most destructive request in the app and had
// the fewest gates. ImportBackup required a confirmation phrase and offered a
// dry run; the snapshot restore — which replaces the content AND the account
// table — required neither, so one POST naming a stale or empty snapshot turned
// a populated site into an empty one.
func TestSnapshotRestoreNeedsConfirmation(t *testing.T) {
	svc, db := newFileTestService(t)
	svc.cfg.BackupDir = t.TempDir()
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)
	if _, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Live", Body: "b"}); err != nil {
		t.Fatalf("seed post: %v", err)
	}
	writeFakeSnapshot(t, svc.cfg.BackupDir, "snapshot-20260101-000000.db", 1, 0)

	// No phrase → refused, and nothing changes.
	if _, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000000.db", RestoreSnapshotOptions{}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("a restore with no confirm phrase must be refused with 400, got %v (err=%v)", StatusOf(err), err)
	}
	// The wrong phrase is refused too — including the phrase the IMPORT flow
	// uses, so a backup file's confirmation cannot stand in for this one.
	if _, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000000.db",
		RestoreSnapshotOptions{Confirm: ImportConfirmPhrase}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("the import phrase must not authorise a snapshot restore, got %v", StatusOf(err))
	}
	var posts int64
	db.Model(&models.Post{}).Count(&posts)
	if posts != 1 {
		t.Fatalf("a refused restore still changed the data (posts=%d)", posts)
	}

	// A dry run reports the contents and changes nothing.
	before := posts
	stats, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000000.db",
		RestoreSnapshotOptions{Confirm: RestoreConfirmPhrase, DryRun: true})
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if stats.Users != 1 || stats.Posts != 0 {
		t.Fatalf("dry run should report the snapshot's contents, got %+v", stats)
	}
	db.Model(&models.Post{}).Count(&posts)
	if posts != before {
		t.Fatalf("a dry run changed the data (posts %d → %d)", before, posts)
	}
}

// A snapshot with no accounts cannot describe a usable site: restoring it
// leaves nobody able to log in, including the root who would fix it.
func TestSnapshotRestoreRefusesAnEmptyUsersTable(t *testing.T) {
	svc, db := newFileTestService(t)
	svc.cfg.BackupDir = t.TempDir()
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)

	writeFakeSnapshot(t, svc.cfg.BackupDir, "snapshot-20260101-000001.db", 0, 3)

	_, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000001.db",
		RestoreSnapshotOptions{Confirm: RestoreConfirmPhrase, AllowDataLoss: true})
	if StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("a snapshot with no users must be refused with 400 even when data loss "+
			"is allowed, got %v (err=%v)", StatusOf(err), err)
	}
	// The live root is still there.
	var n int64
	db.Model(&models.User{}).Count(&n)
	if n == 0 {
		t.Fatal("the refused restore wiped the accounts anyway")
	}
}

// The exact scenario that happened: a small, empty snapshot replaced a
// populated site. Deleting every article is allowed, but it has to be said out
// loud — an operator restoring yesterday's snapshot expects fewer posts, not
// zero.
func TestSnapshotRestoreRefusesToDeleteEveryPostSilently(t *testing.T) {
	svc, db := newFileTestService(t)
	svc.cfg.BackupDir = t.TempDir()
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)
	for i := 0; i < 3; i++ {
		if _, err := svc.CreatePost(ctx, author, CreatePostInput{
			Title: "Live " + string(rune('a'+i)), Body: "b",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	writeFakeSnapshot(t, svc.cfg.BackupDir, "snapshot-20260101-000002.db", 1, 0)

	_, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000002.db",
		RestoreSnapshotOptions{Confirm: RestoreConfirmPhrase})
	if StatusOf(err) != http.StatusConflict {
		t.Fatalf("restoring an empty snapshot over 3 posts must be refused with 409, got %v (err=%v)", StatusOf(err), err)
	}
	var posts int64
	db.Model(&models.Post{}).Count(&posts)
	if posts != 3 {
		t.Fatalf("the refused restore deleted posts anyway (%d left)", posts)
	}

	// Saying so explicitly is enough.
	if _, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000002.db",
		RestoreSnapshotOptions{Confirm: RestoreConfirmPhrase, AllowDataLoss: true}); err != nil {
		t.Fatalf("with allow_data_loss the restore must proceed: %v", err)
	}
	db.Model(&models.Post{}).Count(&posts)
	if posts != 0 {
		t.Fatalf("the restore should have replaced the content, %d posts left", posts)
	}
}

// A snapshot that holds posts is restorable without any extra flag, even when
// it is smaller than the live site — restoring an older backup is a normal
// operation and must not need an override.
func TestSnapshotRestoreOfAnOlderButRealSnapshotNeedsNoOverride(t *testing.T) {
	svc, db := newFileTestService(t)
	svc.cfg.BackupDir = t.TempDir()
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)
	for i := 0; i < 5; i++ {
		if _, err := svc.CreatePost(ctx, author, CreatePostInput{
			Title: "Live " + string(rune('a'+i)), Body: "b",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	writeFakeSnapshot(t, svc.cfg.BackupDir, "snapshot-20260101-000003.db", 1, 2)

	stats, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000003.db",
		RestoreSnapshotOptions{Confirm: RestoreConfirmPhrase})
	if err != nil {
		t.Fatalf("a snapshot with content must restore without an override: %v", err)
	}
	if stats.Posts != 2 {
		t.Fatalf("reported %d posts, expected 2", stats.Posts)
	}
	var posts int64
	db.Model(&models.Post{}).Count(&posts)
	if posts != 2 {
		t.Fatalf("expected the 2 posts from the snapshot, found %d", posts)
	}
}

// A dry run is a preview. It must work WITHOUT the confirmation phrase (the
// phrase protects the destructive apply, not the look), and it must report even
// the dangerous snapshots — an operator cannot say "no" to a snapshot they were
// never allowed to see.
func TestSnapshotRestoreDryRunNeedsNoConfirmation(t *testing.T) {
	svc, db := newFileTestService(t)
	svc.cfg.BackupDir = t.TempDir()
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)

	// A snapshot with users but no posts, previewed with no confirm phrase.
	writeFakeSnapshot(t, svc.cfg.BackupDir, "snapshot-20260101-000010.db", 1, 0)
	stats, err := svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000010.db",
		RestoreSnapshotOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry run must not require the confirm phrase: %v", err)
	}
	if stats.Users != 1 || stats.Posts != 0 {
		t.Fatalf("dry run reported wrong stats: %+v", stats)
	}

	// An empty-users snapshot must still be previewable — the apply refuses it
	// (nothing could log in afterwards), but the preview reports it honestly.
	writeFakeSnapshot(t, svc.cfg.BackupDir, "snapshot-20260101-000011.db", 0, 0)
	stats, err = svc.RestoreSnapshot(ctx, root, "snapshot-20260101-000011.db",
		RestoreSnapshotOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry run must preview even an empty snapshot: %v", err)
	}
	if stats.Users != 0 {
		t.Fatalf("dry run should honestly report 0 users, got %d", stats.Users)
	}
}
