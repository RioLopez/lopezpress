package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/config"
	"lopezpress/internal/models"
)

// newTestService opens an in-memory SQLite DB, runs migrations, and returns a
// *Service wired to it. The DB handle is closed on test completion so
// Windows can release the underlying file-mutex without error.
func newTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	// Use :memory: so the glebarez driver never acquires a Windows file
	// handle on a temp-file DB, which would block TempDir cleanup.
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Post{}, &models.Category{}, &models.Tag{}, &models.PostTag{}, &models.SiteConfig{}, &models.LoginLog{}, &models.PostView{}, &models.PostRevision{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Tests predating the registration gate rely on open registration; enable
	// it for the test DB so Register works as before.
	if err := db.Where("id = ?", 1).First(&models.SiteConfig{}).Error; err != nil {
		_ = db.Create(&models.SiteConfig{ID: 1, SiteName: "Echo", SiteTitle: "Echo", ImgHostMode: "local", AllowRegister: true, PageSize: 10, PublicRead: true}).Error
	}

	cfg := config.Load()
	// Ensure a non-empty JWT secret/expiry so sign/verify is deterministic.
	cfg.JWTSecret = "test-secret"
	cfg.JWTExpiryHours = 1

	svc := New(db, cfg)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return svc, db
}

// makeUser creates a user directly in the DB with the given role and returns it.
func makeUser(t *testing.T, db *gorm.DB, username, email, role string) models.User {
	t.Helper()
	hash, err := models.HashPassword("secret123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u := models.User{Username: username, Email: email, Role: role, PasswordHash: hash}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

func TestRegisterAndLoginRoundTrip(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// Register a fresh user.
	created, err := svc.Register(ctx, RegisterInput{Username: "alice", Email: "alice@example.com", Password: "secret123"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Register now signs the new user in, so the result carries a usable JWT.
	if created.Token == "" || created.User.ID == "" || created.User.Role != "user" {
		t.Fatalf("unexpected register result: %+v", created)
	}

	// Duplicate register -> 409.
	_, err = svc.Register(ctx, RegisterInput{Username: "alice", Email: "alice@example.com", Password: "secret123"})
	if StatusOf(err) != http.StatusConflict {
		t.Fatalf("expected 409, got %v (err=%v)", StatusOf(err), err)
	}

	// Missing field -> 400.
	_, err = svc.Register(ctx, RegisterInput{Username: "bob", Email: "", Password: "x"})
	if StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v (err=%v)", StatusOf(err), err)
	}

	// Login with the username in the email field (existing behaviour).
	res, err := svc.Login(ctx, LoginInput{Email: "alice", Password: "secret123"}, LoginMeta{})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if res.Token == "" || res.User.ID != created.User.ID || res.User.Username != "alice" {
		t.Fatalf("unexpected login result: %+v", res)
	}

	// Authenticate the token -> loads the user.
	user, err := svc.Authenticate(ctx, "Bearer "+res.Token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if user.ID != created.User.ID {
		t.Fatalf("authenticate returned wrong user: %+v", user)
	}

	// Login with wrong password -> 401.
	_, err = svc.Login(ctx, LoginInput{Email: "alice", Password: "wrong"}, LoginMeta{})
	if StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad password, got %v (err=%v)", StatusOf(err), err)
	}
}

func TestCreatePostPopulatesAuthor(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "carol", "carol@example.com", "user")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Hello", Body: "World"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if post.AuthorID != author.ID {
		t.Fatalf("AuthorID mismatch: %q vs %q", post.AuthorID, author.ID)
	}
	// The bug-fix: Author must be a populated pointer (the previous value type
	// would have serialized as an empty {id:""} object).
	if post.Author == nil {
		t.Fatalf("expected Author to be populated after create, got nil")
	}
	if post.Author.ID != author.ID {
		t.Fatalf("Author.ID mismatch: %+v", post.Author)
	}

	// The author must actually appear in the JSON.
	b, _ := json.Marshal(post)
	if !contains(string(b), `"author":{`) {
		t.Fatalf("expected author object in JSON, got %s", string(b))
	}

	// CreatePost with empty title -> 400.
	_, err = svc.CreatePost(ctx, author, CreatePostInput{Title: ""})
	if StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty title, got %v (err=%v)", StatusOf(err), err)
	}
}

// TestListPostsNonAdminFilter verifies the three-tier visibility scoping:
// a signed-in plain user sees ALL public posts plus their own restricted
// posts (private / pending / rejected — never the banned vault, and never
// anyone else's restricted content). A root sees everything.
func TestListPostsNonAdminFilter(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	bob := makeUser(t, db, "bob", "bob@example.com", "user")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// Give each plain user a restricted post (private) + a pending post.
	// Plain users cannot publish directly, so their "public" content arrives
	// via root's published posts instead.
	if _, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "alice priv", Body: "x", Status: "private"}); err != nil {
		t.Fatalf("create alice private: %v", err)
	}
	if _, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "alice pend", Body: "x"}); err != nil {
		t.Fatalf("create alice pending: %v", err)
	}
	if _, err := svc.CreatePost(ctx, bob, CreatePostInput{Title: "bob priv", Body: "x", Status: "private"}); err != nil {
		t.Fatalf("create bob private: %v", err)
	}
	if _, err := svc.CreatePost(ctx, bob, CreatePostInput{Title: "bob pend", Body: "x"}); err != nil {
		t.Fatalf("create bob pending: %v", err)
	}

	// root publishes two posts directly.
	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "root pub1", Body: "x"}); err != nil {
		t.Fatalf("create root pub1: %v", err)
	}
	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "root pub2", Body: "x"}); err != nil {
		t.Fatalf("create root pub2: %v", err)
	}

	// alice: 2 public (root pub1+pub2) + her own 2 restricted (priv+pend)
	// = 4. She must NOT see bob's restricted posts.
	alicePosts, err := svc.ListPosts(ctx, alice, "")
	if err != nil {
		t.Fatalf("ListPosts(alice): %v", err)
	}
	if len(alicePosts) != 4 {
		t.Fatalf("alice should see 4 posts (2 public + her 2 restricted), got %d: %+v", len(alicePosts), alicePosts)
	}
	for _, p := range alicePosts {
		if p.AuthorID == bob.ID && p.Status != models.PostPublished {
			t.Fatalf("alice must not see bob's non-public posts, got %q (%s)", p.Title, p.Status)
		}
	}

	// bob: 2 public + his own 2 restricted = 4.
	bobPosts, err := svc.ListPosts(ctx, bob, "")
	if err != nil {
		t.Fatalf("ListPosts(bob): %v", err)
	}
	if len(bobPosts) != 4 {
		t.Fatalf("bob should see 4 posts (2 public + his 2 restricted), got %d", len(bobPosts))
	}
	for _, p := range bobPosts {
		if p.AuthorID == alice.ID && p.Status != models.PostPublished {
			t.Fatalf("bob must not see alice's non-public posts, got %q (%s)", p.Title, p.Status)
		}
	}

	// root sees every post (2 public + 4 restricted = 6).
	allPosts, err := svc.ListPosts(ctx, root, "")
	if err != nil {
		t.Fatalf("ListPosts(root): %v", err)
	}
	if len(allPosts) != 6 {
		t.Fatalf("root should see 6 posts, got %d", len(allPosts))
	}
}

func TestUpdateAndDeleteForbiddenForNonAuthor(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	mallory := makeUser(t, db, "mallory", "mallory@example.com", "user")

	post, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "mine", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	// Mallory (plain user, non-author) is forbidden from updating -> 403.
	_, err = svc.UpdatePost(ctx, mallory, post.ID, UpdatePostInput{Title: "hijacked"})
	if StatusOf(err) != http.StatusForbidden {
		t.Fatalf("expected 403 for non-author update, got %v (err=%v)", StatusOf(err), err)
	}

	// Mallory is also forbidden from deleting -> 403.
	if err := svc.DeletePost(ctx, mallory, post.ID); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("expected 403 for non-author delete, got %v (err=%v)", StatusOf(err), err)
	}

	// Unknown post id -> 404.
	_, err = svc.UpdatePost(ctx, mallory, "does-not-exist", UpdatePostInput{Title: "x"})
	if StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected 404, got %v (err=%v)", StatusOf(err), err)
	}

	// The author can update her own post.
	updated, err := svc.UpdatePost(ctx, alice, post.ID, UpdatePostInput{Title: "updated"})
	if err != nil {
		t.Fatalf("author update: %v", err)
	}
	if updated.Title != "updated" || updated.Author == nil || updated.Author.ID != alice.ID {
		t.Fatalf("unexpected updated post: %+v", updated)
	}

	// The author can delete her own post.
	if err := svc.DeletePost(ctx, alice, post.ID); err != nil {
		t.Fatalf("author delete: %v", err)
	}
}

// TestAdminCanEditOthersPost verifies an admin may update/delete a plain
// user's post (but not another admin's or root's).
func TestAdminCanEditOthersPost(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	admin := makeUser(t, db, "root", "root@example.com", "admin")

	post, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "alice", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	if _, err := svc.UpdatePost(ctx, admin, post.ID, UpdatePostInput{Title: "admin edited"}); err != nil {
		t.Fatalf("admin update: %v", err)
	}
	if err := svc.DeletePost(ctx, admin, post.ID); err != nil {
		t.Fatalf("admin delete: %v", err)
	}
}

func TestListArchive(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	bob := makeUser(t, db, "bob", "bob@example.com", "user")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// root posts (published directly), alice and bob posts (go to pending).
	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "root pub", Body: "hello"}); err != nil {
		t.Fatalf("create root pub: %v", err)
	}
	if _, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "alice pend", Body: "x"}); err != nil {
		t.Fatalf("create alice pending: %v", err)
	}
	if _, err := svc.CreatePost(ctx, bob, CreatePostInput{Title: "bob pend", Body: "x"}); err != nil {
		t.Fatalf("create bob pending: %v", err)
	}

	// The archive is publicly browsable and only ever lists "published" posts.
	// Every caller — anonymous, signed-in, and root — sees the same full
	// public set; pending / private / rejected / banned posts never appear.
	anonArch, err := svc.ListArchive(ctx, models.User{})
	if err != nil {
		t.Fatalf("ListArchive(anon): %v", err)
	}
	if len(anonArch) != 1 { // only root's published post
		t.Fatalf("anon archive should list the 1 public post, got %d", len(anonArch))
	}
	for _, p := range anonArch {
		if p.Status != models.PostPublished {
			t.Fatalf("archive must only include published posts, got %q (%s)", p.Title, p.Status)
		}
		if p.Body != "" {
			t.Fatalf("expected body stripped from archive, got %q", p.Body)
		}
	}

	// A signed-in user sees the identical public set.
	aliceArch, err := svc.ListArchive(ctx, alice)
	if err != nil {
		t.Fatalf("ListArchive(alice): %v", err)
	}
	if len(aliceArch) != 1 {
		t.Fatalf("signed-in archive should list the 1 public post, got %d", len(aliceArch))
	}
}

// closeDB releases the underlying sql.DB so shared-cache in-memory DBs do not
// leak between tests.
func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- audit fixes (2026-10-15): privilege hierarchy + review-loop closure ---

// TestResetPasswordHierarchy is the service-level regression for P0-1: the
// hierarchical guard (via canManageUser) must reject an admin resetting the
// unique root, while legitimate resets (root→admin, admin→user) succeed.
func TestResetPasswordHierarchy(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	admin := makeUser(t, db, "admin", "admin@example.com", models.RoleAdmin)
	author := makeUser(t, db, "author", "author@example.com", models.RoleUser)

	// Admin MUST NOT be able to reset the root's password (would be takeover).
	if err := svc.ResetPassword(ctx, admin, root.ID, "newpassword123"); err == nil {
		t.Fatal("expected admin→root reset to be forbidden, got nil")
	}
	// Root may reset an admin's password.
	if err := svc.ResetPassword(ctx, root, admin.ID, "newpassword123"); err != nil {
		t.Fatalf("root resetting admin should succeed, got %v", err)
	}
	// Admin may reset a plain user's password.
	if err := svc.ResetPassword(ctx, admin, author.ID, "newpassword123"); err != nil {
		t.Fatalf("admin resetting user should succeed, got %v", err)
	}
	// Password too short is still rejected (independent of hierarchy).
	if StatusOf(svc.ResetPassword(ctx, admin, author.ID, "short")) != http.StatusBadRequest {
		t.Fatal("expected 400 for a too-short password")
	}
}

// TestAuthorResubmitsRejectedPost is the service-level regression for P0-4:
// the author of a rejected post may move it back to "pending" (clearing the
// stale reviewer note), closing the "rejected → fix → resubmit" loop. A
// different user cannot, and a non-rejected post's pending transition is
// ignored (only root may publish).
func TestAuthorResubmitsRejectedPost(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "author", "author@example.com", models.RoleUser)
	other := makeUser(t, db, "other", "other@example.com", models.RoleUser)

	rejected := makePostNoTime(t, db, author, models.PostRejected, "body", "Rejected post")

	// Author resubmits: status becomes pending and the note is cleared.
	out, err := svc.UpdatePost(ctx, author, rejected.ID, UpdatePostInput{Status: models.PostPending})
	if err != nil {
		t.Fatalf("author resubmit failed: %v", err)
	}
	if out.Status != models.PostPending {
		t.Fatalf("expected pending after resubmit, got %q", out.Status)
	}
	if out.ReviewNote != "" {
		t.Fatalf("expected review note cleared on resubmit, got %q", out.ReviewNote)
	}

	// A different user may NOT resubmit someone else's rejected post.
	if _, err := svc.UpdatePost(ctx, other, rejected.ID, UpdatePostInput{Status: models.PostPending}); StatusOf(err) != http.StatusForbidden {
		t.Fatal("expected 403 for a non-author resubmit")
	}

	// A published post's pending transition is ignored (only root publishes).
	pub := makePostNoTime(t, db, author, models.PostPublished, "body", "Published post")
	out2, err := svc.UpdatePost(ctx, author, pub.ID, UpdatePostInput{Status: models.PostPending})
	if err != nil {
		t.Fatalf("update published failed: %v", err)
	}
	if out2.Status != models.PostPublished {
		t.Fatalf("expected published to stay published, got %q", out2.Status)
	}
}

// TestAuthorCannotEscapeQueueViaPrivate locks in P1-14: an author may make
// their own *published* post private, but cannot use "save as private" to
// pull a pending or rejected post out of the review queue.
func TestAuthorCannotEscapeQueueViaPrivate(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "author", "author@example.com", models.RoleUser)

	// published -> private is a legal unpublish.
	published := makePostNoTime(t, db, author, models.PostPublished, "body", "pub")
	out, err := svc.UpdatePost(ctx, author, published.ID, UpdatePostInput{Status: models.PostPrivate})
	if err != nil {
		t.Fatalf("published->private failed: %v", err)
	}
	if out.Status != models.PostPrivate {
		t.Fatalf("expected private, got %q", out.Status)
	}

	// pending -> private must be ignored: the post stays in the queue.
	pending := makePostNoTime(t, db, author, models.PostPending, "body", "pending")
	out2, err := svc.UpdatePost(ctx, author, pending.ID, UpdatePostInput{Status: models.PostPrivate})
	if err != nil {
		t.Fatalf("pending->private call failed: %v", err)
	}
	if out2.Status != models.PostPending {
		t.Fatalf("author escaped the queue: expected pending, got %q", out2.Status)
	}

	// rejected -> private must also be ignored.
	rejected := makePostNoTime(t, db, author, models.PostRejected, "body", "rejected")
	out3, err := svc.UpdatePost(ctx, author, rejected.ID, UpdatePostInput{Status: models.PostPrivate})
	if err != nil {
		t.Fatalf("rejected->private call failed: %v", err)
	}
	if out3.Status != models.PostRejected {
		t.Fatalf("author escaped rejected lane: expected rejected, got %q", out3.Status)
	}
}

// TestDeleteLocalUploadPathTraversal guards the defence-in-depth check added
// after the P2 path-safety audit: names that could resolve outside the upload
// directory are rejected, while a plain file name inside the directory is
// allowed to proceed to the actual os.Remove.
func TestDeleteLocalUploadPathTraversal(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// Traversal / malformed names must be rejected before touching the disk.
	for _, bad := range []string{
		"",
		"../secrets",
		"..\\secrets",
		"a/b.png",
		"a\\b.png",
		"snapshot/../data/uploads/secret.png",
	} {
		err := svc.DeleteLocalUpload(ctx, bad)
		if err == nil {
			t.Fatalf("DeleteLocalUpload(%q) should reject, got nil", bad)
		}
	}

	// A well-formed name inside the directory is allowed; the file does not
	// exist yet, so we expect "file not found" (404) NOT "invalid file name".
	err := svc.DeleteLocalUpload(ctx, "a-safe-name.png")
	if err == nil {
		t.Fatalf("DeleteLocalUpload(safe name) should fail on missing file, got nil")
	}
	var se *ServiceError
	if errors.As(err, &se) {
		if se.Status == http.StatusNotFound {
			// correct: name passed validation, os.Remove reported not-found.
			return
		}
		if se.Status == http.StatusBadRequest {
			t.Fatalf("safe name should not be rejected as invalid: %v", err)
		}
	}
	t.Fatalf("unexpected error class for safe name: %v", err)
}
