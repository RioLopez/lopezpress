package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"

	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// withUser injects a user into the request context the same way WithAuth does,
// so handlers can be exercised without a real JWT round trip.
func withUser(r *http.Request, u models.User) *http.Request {
	return r.WithContext(services.WithActor(r.Context(), u, ""))
}

// seedModerationFixtures creates a root, an admin and two plain users, plus a
// pending post from a plain user and a pending post from the admin.
//
// Register always yields a plain "user" (the root role can never be assigned
// through the API), so the fixture promotes the two privileged accounts
// straight in the DB — exactly what the 0003_root_migration does on boot.
func seedModerationFixtures(t *testing.T, svc *services.Service, db *gorm.DB) (root, admin, author models.User) {
	t.Helper()
	ctx := context.Background()

	rootRes, err := svc.Register(ctx, services.RegisterInput{
		Username: "root", Email: "root@example.com", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register root: %v", err)
	}
	adminRes, err := svc.Register(ctx, services.RegisterInput{
		Username: "admin", Email: "admin@example.com", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	authorRes, err := svc.Register(ctx, services.RegisterInput{
		Username: "author", Email: "author@example.com", Password: "secret123",
	})
	if err != nil {
		t.Fatalf("register author: %v", err)
	}

	if err := db.Model(&models.User{}).Where("id = ?", rootRes.User.ID).
		Update("role", models.RoleRoot).Error; err != nil {
		t.Fatalf("promote root: %v", err)
	}
	if err := db.Model(&models.User{}).Where("id = ?", adminRes.User.ID).
		Update("role", models.RoleAdmin).Error; err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	root = models.User{ID: rootRes.User.ID, Role: models.RoleRoot}
	admin = models.User{ID: adminRes.User.ID, Role: models.RoleAdmin}
	author = models.User{ID: authorRes.User.ID, Role: models.RoleUser}
	return root, admin, author
}

// TestListPendingRequiresAdminOrRoot verifies the HTTP-level guard: anonymous
// callers are rejected before any service work happens.
func TestListPendingRequiresAdminOrRoot(t *testing.T) {
	_, _ = setup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/pending", nil)
	rec := httptest.NewRecorder()
	ListPending(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for anonymous caller, got %d", rec.Code)
	}
}

// TestListPendingAdminCannotSeeRootPosts checks the queue scoping: an admin
// only sees pending posts authored by plain users, never the admin's own (or
// another admin's) pending posts.
func TestListPendingAdminCannotSeeAdminPosts(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	// The fixture already promotes the second account to admin (root is the
	// only account that can do that, and the API path is exercised by the
	// services tests); here we only need the resulting identities.
	_, adminUser, author := seedModerationFixtures(t, svc, db)

	// A plain user's post and an admin's post both enter the pending queue.
	if _, err := svc.CreatePost(ctx, author, services.CreatePostInput{Title: "from user", Body: "x"}); err != nil {
		t.Fatalf("create user post: %v", err)
	}
	if _, err := svc.CreatePost(ctx, adminUser, services.CreatePostInput{Title: "from admin", Body: "x"}); err != nil {
		t.Fatalf("create admin post: %v", err)
	}

	req := withUser(httptest.NewRequest(http.MethodGet, "/api/admin/pending", nil), adminUser)
	rec := httptest.NewRecorder()
	ListPending(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var posts []models.Post
	if err := decodeJSON(rec, &posts); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, p := range posts {
		if p.Title == "from admin" {
			t.Fatalf("admin queue leaked an admin-authored post: %+v", posts)
		}
	}
	if len(posts) != 1 || posts[0].Title != "from user" {
		t.Fatalf("expected only the user's post, got %+v", posts)
	}
}

// TestReviewPostRejectRequiresNote verifies the service rule is reachable
// through the HTTP handler: rejecting without a note is a 400.
func TestReviewPostRejectRequiresNote(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	root, admin, author := seedModerationFixtures(t, svc, db)
	_ = root
	p, err := svc.CreatePost(ctx, author, services.CreatePostInput{Title: "to reject", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	body := `{"decision":"reject"}`
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/admin/review/"+p.ID, strings.NewReader(body)), admin)
	req.SetPathValue("id", p.ID)
	rec := httptest.NewRecorder()
	ReviewPost(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a note-less reject, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestBanPostRequiresRoot verifies the banned-vault endpoints are root-only at
// the HTTP layer: an admin gets 403 even with a valid session.
func TestBanPostRequiresRoot(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	root, admin, author := seedModerationFixtures(t, svc, db)
	_ = root
	p, err := svc.CreatePost(ctx, author, services.CreatePostInput{Title: "ban me", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/admin/ban/"+p.ID, nil), admin)
	req.SetPathValue("id", p.ID)
	rec := httptest.NewRecorder()
	BanPost(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for admin banning, got %d", rec.Code)
	}
}

// TestListBannedHiddenFromAdmin verifies the vault is invisible to admins:
// root sees the banned post, the admin does not.
func TestListBannedHiddenFromAdmin(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	root, admin, author := seedModerationFixtures(t, svc, db)
	p, err := svc.CreatePost(ctx, author, services.CreatePostInput{Title: "hidden", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := svc.ReviewPost(ctx, root, p.ID, services.ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.BanPost(ctx, root, p.ID); err != nil {
		t.Fatalf("ban: %v", err)
	}

	// Root sees exactly one entry.
	req := withUser(httptest.NewRequest(http.MethodGet, "/api/admin/banned", nil), root)
	rec := httptest.NewRecorder()
	ListBanned(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("root list: expected 200, got %d", rec.Code)
	}
	var rootView []models.Post
	if err := decodeJSON(rec, &rootView); err != nil {
		t.Fatalf("decode root view: %v", err)
	}
	if len(rootView) != 1 || rootView[0].ID != p.ID {
		t.Fatalf("expected the banned post for root, got %+v", rootView)
	}

	// The admin is refused outright.
	req = withUser(httptest.NewRequest(http.MethodGet, "/api/admin/banned", nil), admin)
	rec = httptest.NewRecorder()
	ListBanned(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin list: expected 403, got %d", rec.Code)
	}
}

// TestSendBackPostMovesOutOfVault verifies the send-back endpoint: a banned
// post returns to the author as "rejected" carrying the reviewer's note, and
// disappears from the vault.
func TestSendBackPostMovesOutOfVault(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	root, _, author := seedModerationFixtures(t, svc, db)
	p, err := svc.CreatePost(ctx, author, services.CreatePostInput{Title: "send me back", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := svc.ReviewPost(ctx, root, p.ID, services.ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.BanPost(ctx, root, p.ID); err != nil {
		t.Fatalf("ban: %v", err)
	}

	body := `{"note":"please fix the intro"}`
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/admin/banned/"+p.ID+"/send-back", strings.NewReader(body)), root)
	req.SetPathValue("id", p.ID)
	rec := httptest.NewRecorder()
	SendBackPost(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var out models.Post
	if err := decodeJSON(rec, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Status != models.PostRejected {
		t.Fatalf("expected rejected, got %q", out.Status)
	}
	if out.ReviewNote != "please fix the intro" {
		t.Fatalf("note not carried through: %q", out.ReviewNote)
	}

	// The vault must now be empty.
	banned, err := svc.ListBanned(ctx, root)
	if err != nil {
		t.Fatalf("ListBanned: %v", err)
	}
	if len(banned) != 0 {
		t.Fatalf("expected an empty vault, got %+v", banned)
	}
}

// TestDeleteBannedPermanentlyRemovesPost verifies the hard delete actually
// removes the row (Unscoped), not just a soft delete.
func TestDeleteBannedPermanentlyRemovesPost(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	root, _, author := seedModerationFixtures(t, svc, db)
	p, err := svc.CreatePost(ctx, author, services.CreatePostInput{Title: "delete me", Body: "x"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := svc.ReviewPost(ctx, root, p.ID, services.ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.BanPost(ctx, root, p.ID); err != nil {
		t.Fatalf("ban: %v", err)
	}

	req := withUser(httptest.NewRequest(http.MethodDelete, "/api/admin/banned/"+p.ID, nil), root)
	req.SetPathValue("id", p.ID)
	rec := httptest.NewRecorder()
	DeleteBannedPermanently(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", rec.Code, rec.Body.String())
	}
	// A direct lookup must now fail — the row is gone, not soft-deleted.
	if _, err := svc.ReviewPost(ctx, root, p.ID, services.ReviewPostInput{Action: "approve"}); err == nil {
		t.Fatalf("expected the post to be gone, but a review call still succeeded")
	}
}

// TestResetPasswordForbidsAdminResettingRoot is the regression test for the
// P0-1 privilege-escalation: an admin must not be able to reset the unique
// root's password (which would let them log in as root and take over the
// site). The handler only checks requireAdmin, so the hierarchical guard lives
// in the service via canManageUser.
func TestResetPasswordForbidsAdminResettingRoot(t *testing.T) {
	svc, db := setup(t)

	root, admin, _ := seedModerationFixtures(t, svc, db)

	body := `{"password":"newpassword123"}`
	req := withUser(
		httptest.NewRequest(http.MethodPost, "/api/users/"+root.ID+"/reset-password", strings.NewReader(body)),
		admin,
	)
	req.SetPathValue("id", root.ID)
	rec := httptest.NewRecorder()
	ResetPassword(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when an admin resets root's password, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// TestResetPasswordAllowsAdminResettingUser confirms the legitimate path still
// works: an admin may reset a plain user's password.
func TestResetPasswordAllowsAdminResettingUser(t *testing.T) {
	svc, db := setup(t)

	_, admin, author := seedModerationFixtures(t, svc, db)

	body := `{"password":"newpassword123"}`
	req := withUser(
		httptest.NewRequest(http.MethodPost, "/api/users/"+author.ID+"/reset-password", strings.NewReader(body)),
		admin,
	)
	req.SetPathValue("id", author.ID)
	rec := httptest.NewRecorder()
	ResetPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when an admin resets a user's password, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// --- helpers ---

// decodeJSON decodes a recorder body into v.
func decodeJSON(rec *httptest.ResponseRecorder, v any) error {
	return json.Unmarshal(rec.Body.Bytes(), v)
}
