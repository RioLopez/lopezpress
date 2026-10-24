package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lopezpress/internal/models"
)

// TestSignAndVerifyToken exercises the JWT round trip: a signed token must
// verify and resolve to the originating user, both in "Bearer <token>" and
// bare-token forms.
func TestSignAndVerifyToken(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")

	token, _, err := svc.SignToken(alice.ID, "user", 0)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}

	// Bare token form.
	user, err := svc.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("Authenticate(bare): %v", err)
	}
	if user.ID != alice.ID {
		t.Fatalf("resolved wrong user: %+v", user)
	}

	// Bearer-prefixed form.
	if _, err := svc.Authenticate(ctx, "Bearer "+token); err != nil {
		t.Fatalf("Authenticate(bearer): %v", err)
	}
}

// TestAuthenticateRejectsTamperedToken verifies that a token whose signature
// has been altered (or a garbage string) is rejected with 401.
func TestAuthenticateRejectsTamperedToken(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// A completely invalid token.
	if _, err := svc.Authenticate(ctx, "not-a-jwt"); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 for garbage token, got %v (err=%v)", StatusOf(err), err)
	}

	// Tamper the signature portion (last segment) to break it. Flip the
	// first byte's bits so the change is guaranteed even on base64url edge
	// cases (e.g. when the trailing character is already 'x').
	bob := makeUser(t, db, "bob", "bob@example.com", "user")
	token, _, err := svc.SignToken(bob.ID, "user", 0)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	parts := strings.Split(token, ".")
	sig := []byte(parts[2])
	sig[0] ^= 0xFF // invert every bit of the first signature byte
	parts[2] = string(sig)
	tampered := strings.Join(parts, ".")
	if _, err := svc.Authenticate(ctx, tampered); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered token, got %v (err=%v)", StatusOf(err), err)
	}
}

// TestListPostsStatusFilter verifies that the optional status query narrows
// the result set. Under the three-tier model, a root creates posts that are
// published by default; filtering by status isolates that subset.
func TestListPostsStatusFilter(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")

	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "pub", Body: "x"}); err != nil {
		t.Fatalf("create published: %v", err)
	}
	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "priv", Body: "x", Status: "private"}); err != nil {
		t.Fatalf("create private: %v", err)
	}

	published, err := svc.ListPosts(ctx, root, "published")
	if err != nil {
		t.Fatalf("ListPosts(published): %v", err)
	}
	if len(published) != 1 || published[0].Status != models.PostPublished {
		t.Fatalf("expected 1 published post, got %d: %+v", len(published), published)
	}

	privates, err := svc.ListPosts(ctx, root, "private")
	if err != nil {
		t.Fatalf("ListPosts(private): %v", err)
	}
	if len(privates) != 1 || privates[0].Status != models.PostPrivate {
		t.Fatalf("expected 1 private post, got %d: %+v", len(privates), privates)
	}

	// No filter -> both.
	all, err := svc.ListPosts(ctx, root, "")
	if err != nil {
		t.Fatalf("ListPosts(all): %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 posts unfiltered, got %d", len(all))
	}
}

// TestGetPostNotFound verifies the 404 path for a missing post id.
func TestGetPostNotFound(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	if _, err := svc.GetPost(ctx, models.User{}, "does-not-exist"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected 404, got %v (err=%v)", StatusOf(err), err)
	}
}

// TestGetPostPrivateVisibility is the regression test for the visibility
// guard: a private post is readable only by its author — not by another
// user, not by an admin, not by root (strict "仅自己可见"). Anyone else gets
// 404 so the post's existence is not leaked. Pending/rejected posts stay
// readable by admin/root for the review flow. A banned post is visible to
// root only (the banned vault), so even the author and a plain admin are
// blocked from it.
func TestGetPostPrivateVisibility(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "author", "author@example.com", "user")
	other := makeUser(t, db, "other", "other@example.com", "user")
	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// Posts are inserted straight into the DB (makePostNoTime) rather than
	// through CreatePost: CreatePost forces plain users' submissions to
	// "pending", so it cannot produce a genuinely private post for a
	// non-root author.
	priv := makePostNoTime(t, db, author, "private", "x", "secret")
	pub, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "public", Body: "y"})
	if err != nil {
		t.Fatalf("CreatePost(published): %v", err)
	}
	// A pending post stays readable by admin/root: the review flow needs it.
	pend, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "queued", Body: "z"})
	if err != nil {
		t.Fatalf("CreatePost(pending): %v", err)
	}
	// A banned post, placed into the vault by root, is invisible to everyone
	// except root.
	banned := makePostNoTime(t, db, author, "private", "x", "nasty")
	if _, err := svc.BanPost(ctx, root, banned.ID); err != nil {
		t.Fatalf("BanPost: %v", err)
	}

	cases := []struct {
		name   string
		user   models.User
		id     string
		wantOK bool
	}{
		{"author reads own private", author, priv.ID, true},
		{"root blocked from other's private", root, priv.ID, false},
		{"admin blocked from other's private", admin, priv.ID, false},
		{"other user blocked from private", other, priv.ID, false},
		{"anonymous blocked from private", models.User{}, priv.ID, false},
		{"anonymous reads published", models.User{}, pub.ID, true},
		{"author reads own pending", author, pend.ID, true},
		{"admin reads pending (review flow)", admin, pend.ID, true},
		{"root reads pending (review flow)", root, pend.ID, true},
		{"other user blocked from pending", other, pend.ID, false},
		{"anonymous blocked from pending", models.User{}, pend.ID, false},
		{"root reads banned (vault)", root, banned.ID, true},
		{"author blocked from own banned", author, banned.ID, false},
		{"admin blocked from banned", admin, banned.ID, false},
	}
	for _, c := range cases {
		_, err := svc.GetPost(ctx, c.user, c.id)
		if c.wantOK && err != nil {
			t.Errorf("%s: expected success, got %v", c.name, err)
		}
		if !c.wantOK && StatusOf(err) != http.StatusNotFound {
			t.Errorf("%s: expected 404, got %v (err=%v)", c.name, StatusOf(err), err)
		}
	}
}

// TestMsgOfBehavior verifies the two branches of MsgOf: a *ServiceError
// surfaces its own client message, while any other error falls back to the
// generic "internal server error" string so internals are not leaked.
func TestMsgOfBehavior(t *testing.T) {
	// A ServiceError surfaces its own message.
	se := newErr(http.StatusForbidden, "denied")
	if got := MsgOf(se); got != "denied" {
		t.Fatalf("MsgOf(ServiceError) = %q, want %q", got, "denied")
	}

	// A non-ServiceError must NOT leak its underlying message.
	if got := MsgOf(errors.New("secret internal detail")); got != "internal server error" {
		t.Fatalf("MsgOf(raw error) leaked: %q, want generic", got)
	}
}

// TestCreatePostDefaultStatus verifies the role-based initial status: a plain
// user's new post enters the review queue (pending), while a root's post
// publishes directly.
func TestCreatePostDefaultStatus(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	post, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "t", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if post.Status != models.PostPending {
		t.Fatalf("expected user post to enter the queue (pending), got %q", post.Status)
	}

	// A root's post publishes immediately.
	root := makeUser(t, db, "root", "root@example.com", "root")
	rpost, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "t", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost(root): %v", err)
	}
	if rpost.Status != models.PostPublished {
		t.Fatalf("expected root post to be published, got %q", rpost.Status)
	}
}

// TestListPostsEmptyReturnsEmptySlice verifies an empty result yields a
// non-nil empty slice (so JSON marshals to [] not null).
func TestListPostsEmptyReturnsEmptySlice(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	ghost := makeUser(t, db, "ghost", "ghost@example.com", "user")
	posts, err := svc.ListPosts(ctx, ghost, "")
	if err != nil {
		t.Fatalf("ListPosts: %v", err)
	}
	if posts == nil {
		t.Fatalf("expected non-nil empty slice, got nil")
	}
	if len(posts) != 0 {
		t.Fatalf("expected 0 posts, got %d", len(posts))
	}
}

// TestAuthenticateRejectsExpiredToken verifies a token signed with an expiry
// in the past is rejected with 401.
func TestAuthenticateRejectsExpiredToken(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	tok := signExpiredToken(alice.ID)
	if _, err := svc.Authenticate(ctx, tok); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %v (err=%v)", StatusOf(err), err)
	}
}

// signExpiredToken issues an HS256 JWT with a past expiry, using the same
// "test-secret" that newTestService configures.
func signExpiredToken(userID string) string {
	exp := time.Now().Add(-2 * time.Hour)
	claims := jwt.MapClaims{
		"sub": userID, "role": "user", "exp": exp.Unix(), "iat": time.Now().Unix(),
	}
	tk, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test-secret"))
	return tk
}

// TestEscapeLike verifies SQL LIKE wildcards and the escape character itself
// are escaped so untrusted search input is treated as literal text.
func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"hello":         "hello",
		"100%":          "100\\%",
		"a_b":           "a\\_b",
		`path\to`:       `path\\to`,
		"%_\\":          "\\%\\_\\\\",
	}
	for in, want := range cases {
		if got := escapeLike(in); got != want {
			t.Errorf("escapeLike(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSignTokenFallsBackToDevSecret verifies signing works even when cfg is
// nil (uses the documented dev fallback secret) and returns a future expiry.
func TestSignTokenFallsBackToDevSecret(t *testing.T) {
	svc := New(nil, nil) // nil cfg -> dev fallback secret
	tok, exp, err := svc.SignToken("user-1", "user", 0)
	if err != nil {
		t.Fatalf("SignToken with nil cfg: %v", err)
	}
	if tok == "" {
		t.Fatalf("expected non-empty token")
	}
	if exp.IsZero() {
		t.Fatalf("expected a non-zero expiry, got zero time")
	}
}

// TestRegisterDuplicateByEmail verifies a second user reusing the SAME email
// (different username) is rejected with 409.
func TestRegisterDuplicateByEmail(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	if _, err := svc.Register(ctx, RegisterInput{Username: "u1", Email: "same@example.com", Password: "password1"}); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if _, err := svc.Register(ctx, RegisterInput{Username: "u2", Email: "same@example.com", Password: "password2"}); StatusOf(err) != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate email, got %v (err=%v)", StatusOf(err), err)
	}
}

// TestLoginMissingFields verifies empty login input yields 400.
func TestLoginMissingFields(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	if _, err := svc.Login(ctx, LoginInput{}, LoginMeta{}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400, got %v (err=%v)", StatusOf(err), err)
	}
}

// --- role-based visibility + review workflow ---

// TestCreatePostVisibilityWhitelist verifies role-gated initial status: a
// plain user always enters the review queue (pending) regardless of the
// requested value; only a root can publish or keep private. Stale client
// values cannot bypass the queue.
func TestCreatePostVisibilityWhitelist(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// A plain user is forced into the queue no matter what the client asks.
	for _, status := range []string{"", "published", "draft", "scheduled", "banned"} {
		p, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "t", Body: "x", Status: status})
		if err != nil {
			t.Fatalf("CreatePost(status=%q): %v", status, err)
		}
		if p.Status != models.PostPending {
			t.Fatalf("user status %q -> %q, want pending", status, p.Status)
		}
	}

	// A root's explicit private is preserved.
	rr, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "t", Body: "x", Status: "private"})
	if err != nil {
		t.Fatalf("CreatePost(root private): %v", err)
	}
	if rr.Status != models.PostPrivate {
		t.Fatalf("root private -> %q, want private", rr.Status)
	}
	// A root with no status publishes.
	rp, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "t", Body: "x"})
	if err != nil {
		t.Fatalf("CreatePost(root default): %v", err)
	}
	if rp.Status != models.PostPublished {
		t.Fatalf("root default -> %q, want published", rp.Status)
	}
}

// TestListPostsHidesOtherUsersRestricted verifies that a plain user never
// sees another user's restricted posts (pending/rejected/private) or the
// banned vault, while their own restricted posts remain visible. Anonymous
// visitors see only public posts.
func TestListPostsHidesOtherUsersRestricted(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	bob := makeUser(t, db, "bob", "bob@example.com", "user")
	root := makeUser(t, db, "root", "root@example.com", "root")

	// A root publishes two public posts; alice and bob each author a
	// restricted post (private).
	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "pub1", Body: "x"}); err != nil {
		t.Fatalf("create pub1: %v", err)
	}
	if _, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "pub2", Body: "x"}); err != nil {
		t.Fatalf("create pub2: %v", err)
	}
	if _, err := svc.CreatePost(ctx, alice, CreatePostInput{Title: "a-private", Body: "x", Status: "private"}); err != nil {
		t.Fatalf("create alice private: %v", err)
	}
	if _, err := svc.CreatePost(ctx, bob, CreatePostInput{Title: "b-private", Body: "x", Status: "private"}); err != nil {
		t.Fatalf("create bob private: %v", err)
	}

	// Anonymous visitor: sees the 2 public posts, never any private.
	anon := models.User{} // zero user = logged-out
	anonPosts, err := svc.ListPosts(ctx, anon, "")
	if err != nil {
		t.Fatalf("ListPosts(anon): %v", err)
	}
	anonPublic, anonPrivate := 0, 0
	for _, p := range anonPosts {
		switch p.Status {
		case models.PostPublished:
			anonPublic++
		case models.PostPrivate:
			anonPrivate++
		}
	}
	if anonPublic != 2 { // pub1 + pub2
		t.Fatalf("anon should see 2 public posts, got %d", anonPublic)
	}
	if anonPrivate != 0 {
		t.Fatalf("anon must never see private posts, got %d", anonPrivate)
	}

	// Signed-in bob: sees the 2 public + his own private, NOT alice's.
	bobPosts, err := svc.ListPosts(ctx, bob, "")
	if err != nil {
		t.Fatalf("ListPosts(bob): %v", err)
	}
	leaked := 0
	for _, p := range bobPosts {
		if p.AuthorID == alice.ID && p.Status == models.PostPrivate {
			leaked++
		}
	}
	if leaked != 0 {
		t.Fatalf("bob leaked %d of alice's private posts", leaked)
	}

	// alice: 2 public + her own private = 3.
	alicePosts, err := svc.ListPosts(ctx, alice, "")
	if err != nil {
		t.Fatalf("ListPosts(alice): %v", err)
	}
	if len(alicePosts) != 3 {
		t.Fatalf("expected alice to see 3 posts (2 public + her 1 private), got %d", len(alicePosts))
	}
}

// TestUpdatePostVisibilityToggle verifies root can flip a post between
// published and private; a plain user can only move toward private (the
// restricted direction) — they cannot publish their own content.
func TestUpdatePostVisibilityToggle(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")
	p, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "t", Body: "x"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// root flips to private.
	priv, err := svc.UpdatePost(ctx, root, p.ID, UpdatePostInput{Status: "private"})
	if err != nil {
		t.Fatalf("update private: %v", err)
	}
	if priv.Status != models.PostPrivate {
		t.Fatalf("expected private, got %q", priv.Status)
	}

	// root flips back to published.
	pub, err := svc.UpdatePost(ctx, root, p.ID, UpdatePostInput{Status: "published"})
	if err != nil {
		t.Fatalf("update public: %v", err)
	}
	if pub.Status != models.PostPublished {
		t.Fatalf("expected published, got %q", pub.Status)
	}
}

// TestUpdatePostIgnoresStaleStatus verifies that a partial edit without a
// valid status leaves the stored status untouched.
func TestUpdatePostIgnoresStaleStatus(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")
	p, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "t", Body: "x", Status: "private"})
	if err != nil {
		t.Fatalf("create private: %v", err)
	}
	// Title-only edit must keep the post private.
	updated, err := svc.UpdatePost(ctx, root, p.ID, UpdatePostInput{Title: "renamed"})
	if err != nil {
		t.Fatalf("title-only edit of private post: %v", err)
	}
	if updated.Status != models.PostPrivate {
		t.Fatalf("expected private to stay private on partial edit, got %q", updated.Status)
	}
}

// TestRegisterIssuesUsableToken covers spec 04 AC-2.2: registering must log the
// user straight in, meaning the returned JWT actually authenticates as the new
// user without a separate login call.
func TestRegisterIssuesUsableToken(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	res, err := svc.Register(ctx, RegisterInput{Username: "carol", Email: "carol@example.com", Password: "secret123"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Token == "" {
		t.Fatalf("expected register to return a JWT, got empty token")
	}
	if res.User.ID == "" || res.User.Username != "carol" {
		t.Fatalf("unexpected user in register result: %+v", res.User)
	}

	// The token must be accepted by Authenticate and resolve to the new user.
	me, err := svc.Authenticate(ctx, res.Token)
	if err != nil {
		t.Fatalf("Authenticate(register token): %v", err)
	}
	if me.ID != res.User.ID {
		t.Fatalf("token resolved to %s, want %s", me.ID, res.User.ID)
	}

	// Duplicates still conflict, even with the token-issuing flow.
	if _, err := svc.Register(ctx, RegisterInput{Username: "carol", Email: "carol@example.com", Password: "secret123"}); StatusOf(err) != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate, got %v (err=%v)", StatusOf(err), err)
	}
}
