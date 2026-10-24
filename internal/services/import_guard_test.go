package services

import (
	"context"
	"net/http"
	"testing"

	"lopezpress/internal/models"
)

// TestImportBackupRejectsEmptyPayload pins the fix for the 2026-10-24
// wipe incident: `POST /api/backup/import {}` used to pass validation (every
// check was "if the field is present, it must match") and then delete every
// user, post, category and tag in one transaction. The payload is now
// rejected both in preview and in apply mode, and — the part that matters —
// the database is untouched.
func TestImportBackupRejectsEmptyPayload(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")
	_ = makePostNoTime(t, db, root, models.PostPublished, "body", "live post")

	var usersBefore, postsBefore int64
	db.Model(&models.User{}).Count(&usersBefore)
	db.Model(&models.Post{}).Count(&postsBefore)

	for _, dry := range []bool{true, false} {
		if _, err := svc.ImportBackup(ctx, root, ImportData{}, dry); StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("dry_run=%v: expected 400 for an empty payload, got %v (err=%v)", dry, StatusOf(err), err)
		}
	}

	var usersAfter, postsAfter int64
	db.Model(&models.User{}).Count(&usersAfter)
	db.Model(&models.Post{}).Count(&postsAfter)
	if usersAfter != usersBefore || postsAfter != postsBefore {
		t.Fatalf("rejected import still modified data: users %d->%d, posts %d->%d",
			usersBefore, usersAfter, postsBefore, postsAfter)
	}
}

// TestImportBackupRequiresConfirm verifies that a structurally valid payload
// still cannot replace the database unless it carries the confirmation
// phrase, and that an applied import revokes every pre-existing session
// (restoring a dump must not resurrect previously revoked tokens).
func TestImportBackupRequiresConfirm(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")
	makeUser(t, db, "boss", "boss@example.com", "admin")

	in := ImportData{Users: []models.User{
		makeUser(t, db, "kept", "kept@example.com", "user"),
		makeUser(t, db, "keptroot", "keptroot@example.com", "root"),
	}}
	if _, err := svc.ImportBackup(ctx, root, in, false); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 without confirmation, got %v (err=%v)", StatusOf(err), err)
	}
	// The unconfirmed attempt must not have touched anything.
	var boss int64
	db.Model(&models.User{}).Where("username = ?", "boss").Count(&boss)
	if boss != 1 {
		t.Fatalf("unconfirmed import modified the database")
	}

	in.Confirm = ImportConfirmPhrase
	if _, err := svc.ImportBackup(ctx, root, in, false); err != nil {
		t.Fatalf("confirmed import failed: %v", err)
	}
	// Every restored account starts at token_version >= 1, so any JWT signed
	// before the restore (version 0) is rejected on its next use.
	var kept models.User
	if err := db.Where("username = ?", "kept").First(&kept).Error; err != nil {
		t.Fatalf("imported user missing: %v", err)
	}
	if kept.TokenVersion < 1 {
		t.Fatalf("restore must bump token_version, got %d", kept.TokenVersion)
	}
	// The dump itself carries token_version 0, so a token minted from the
	// restored row before the bump would still be version 0. Authenticate must
	// reject it, which only happens because the restore bumped the column.
	stale, _, terr := svc.SignToken(kept.ID, kept.Role, 0)
	if terr != nil {
		t.Fatalf("sign stale token: %v", terr)
	}
	if _, err := svc.Authenticate(ctx, stale); err == nil {
		t.Fatalf("a version-0 token must not authenticate after a restore")
	}
}

// TestUserCannotPinPost locks the pinned fix: pinning floats a post to the
// top of every public list, so a plain member may not do it at creation, and
// an approval drops any pin a member set beforehand.
func TestUserCannotPinPost(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "writer", "w@example.com", "user")
	admin := makeUser(t, db, "boss", "boss@example.com", "admin")

	yes := true
	if _, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "t", Body: "b", Pinned: &yes}); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("expected 403 when a plain user pins a post, got %v (err=%v)", StatusOf(err), err)
	}

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "t", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Simulate a legacy row that was pinned before this guard existed.
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).Update("pinned", true).Error; err != nil {
		t.Fatalf("seed legacy pin: %v", err)
	}
	approved, err := svc.ReviewPost(ctx, admin, post.ID, ReviewPostInput{Action: "approve"})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if approved.Pinned {
		t.Fatalf("approving a plain user's post must clear the self-set pin")
	}
}

// TestUsernameValidation covers the impersonation surface: Unicode homoglyphs
// (Cyrillic "а" in "аdmin") and case-only variants ("Admin" vs "admin") must
// both be refused no matter which endpoint creates the account.
func TestUsernameValidation(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", "root")

	cases := []string{"аdmin", "ad min", "ad!min", "ab", "admin@"}
	for _, name := range cases {
		_, err := svc.CreateUser(ctx, root, CreateUserInput{
			Username: name, Email: "x" + "@example.com", Password: "password1",
		})
		if StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("username %q: expected 400, got %v (err=%v)", name, StatusOf(err), err)
		}
	}

	if _, err := svc.CreateUser(ctx, root, CreateUserInput{Username: "Admin", Email: "a@example.com", Password: "password1"}); err != nil {
		t.Fatalf("create Admin: %v", err)
	}
	if _, err := svc.CreateUser(ctx, root, CreateUserInput{Username: "admin", Email: "b@example.com", Password: "password1"}); StatusOf(err) != http.StatusConflict {
		t.Fatalf("case-only duplicate: expected 409, got %v (err=%v)", StatusOf(err), err)
	}
	if _, err := svc.CreateUser(ctx, root, CreateUserInput{Username: "other", Email: "A@EXAMPLE.COM", Password: "password1"}); StatusOf(err) != http.StatusConflict {
		t.Fatalf("case-only duplicate email: expected 409, got %v (err=%v)", StatusOf(err), err)
	}
}
