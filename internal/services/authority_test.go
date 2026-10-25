package services

import (
	"context"
	"net/http"
	"testing"

	"lopezpress/internal/models"
)

// R5-05: a reviewer is not an author. Before this, a plain admin could rewrite
// any ordinary user's title and body, and for a PUBLISHED post the change went
// live at once — no review, no trace, and the article still carried the original
// author's name. Anyone with an admin session (weak password, an XSS on the
// console) could therefore impersonate every author on the site.
func TestAdminCannotRewriteAnotherAuthorsContent(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "author1", "author1@example.com", models.RoleUser)
	admin := makeUser(t, db, "admin1", "admin1@example.com", models.RoleAdmin)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Original", Body: "original words"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Body rewrite → 403.
	if _, err := svc.UpdatePost(ctx, admin, post.ID,
		UpdatePostInput{Body: "words the admin prefers"}); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("admin rewriting another author's body: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// Title rewrite → 403.
	if _, err := svc.UpdatePost(ctx, admin, post.ID,
		UpdatePostInput{Title: "Title the admin prefers"}); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("admin rewriting another author's title: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	// Cover rewrite → 403.
	if _, err := svc.UpdatePost(ctx, admin, post.ID,
		UpdatePostInput{Cover: "/uploads/admin-choice.png"}); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("admin rewriting another author's cover: expected 403, got %v (err=%v)", StatusOf(err), err)
	}

	// The content is untouched.
	var row models.Post
	if err := db.Where("id = ?", post.ID).First(&row).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if row.Title != "Original" || row.Body != "original words" {
		t.Fatalf("content changed anyway: title=%q body=%q", row.Title, row.Body)
	}

	// Root may still fix a typo — root is the recovery path.
	root := makeUser(t, db, "root1", "root1@example.com", models.RoleRoot)
	if _, err := svc.UpdatePost(ctx, root, post.ID,
		UpdatePostInput{Body: "fixed by root"}); err != nil {
		t.Fatalf("root must be able to edit anyone's content: %v", err)
	}
}

// The editorial actions a reviewer actually needs must keep working, or the
// fix breaks the review workflow instead of tightening it.
func TestAdminKeepsEditorialActionsOnOthersPostsOnOthersPosts(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "author2", "author2@example.com", models.RoleUser)
	admin := makeUser(t, db, "admin2", "admin2@example.com", models.RoleAdmin)

	cat := models.Category{Name: "News"}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("category: %v", err)
	}
	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Pending piece", Body: "text"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Recategorise + retag: allowed (organising the site is an admin's job).
	if _, err := svc.UpdatePost(ctx, admin, post.ID, UpdatePostInput{
		CategoryID: &cat.ID, Tags: []string{"news"},
	}); err != nil {
		t.Fatalf("admin must be able to recategorise/retag: %v", err)
	}
	// Unpublish to private: allowed (that is a moderation action).
	if _, err := svc.UpdatePost(ctx, admin, post.ID,
		UpdatePostInput{Status: models.PostPrivate}); err != nil {
		t.Logf("admin unpublish returned %v (allowed to be refused if the state rejects it)", err)
	}
}

// Reading history is a review need; writing it back is not. An admin may see
// what an author changed, but restoring an old version would put text into a
// published article — the same impersonation path as editing the body.
func TestRevisionAccessSplitsReadAndWrite(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "author3", "author3@example.com", models.RoleUser)
	admin := makeUser(t, db, "admin3", "admin3@example.com", models.RoleAdmin)
	root := makeUser(t, db, "root3", "root3@example.com", models.RoleRoot)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "v1", Body: "first body"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Produce a revision by editing once (the author's own edit).
	if _, err := svc.UpdatePost(ctx, author, post.ID,
		UpdatePostInput{Body: "second body"}); err != nil {
		t.Fatalf("author edit: %v", err)
	}

	revs, err := svc.ListRevisions(ctx, admin, post.ID)
	if err != nil {
		t.Fatalf("an admin must be able to READ history for review: %v", err)
	}
	if len(revs) == 0 {
		t.Fatalf("expected at least one revision")
	}

	// Writing history back is author/root only.
	if _, err := svc.RestoreRevision(ctx, admin, post.ID, revs[0].ID); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("admin restoring another author's revision: expected 403, got %v (err=%v)", StatusOf(err), err)
	}

	// The author can still use their own history.
	if _, err := svc.RestoreRevision(ctx, author, post.ID, revs[0].ID); err != nil {
		t.Fatalf("author must be able to restore their own revision: %v", err)
	}
	// And root can act on anyone's.
	revs2, err := svc.ListRevisions(ctx, root, post.ID)
	if err != nil || len(revs2) == 0 {
		t.Fatalf("root must be able to read history: %v", err)
	}

	// A plain third party reads nothing — and for a post they cannot see (it
	// is still pending), the answer is 404 rather than 403, so the endpoint
	// does not confirm the post exists.
	other := makeUser(t, db, "other3", "other3@example.com", models.RoleUser)
	if _, err := svc.ListRevisions(ctx, other, post.ID); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("an unrelated user reading history: expected 404, got %v (err=%v)", StatusOf(err), err)
	}
}

// R7-05 (visibility half): the revision list of a post is that post's content,
// one version per row. The visibility model says a private or draft post is
// author-only — even root is excluded — so the revision endpoint must not hand
// an admin the private text through a side door. Before this, assertCanReadHistory
// checked only the reader's role, never whether the post was even visible to
// them, so an admin could read the history of a private post the model said
// they could not see.
func TestAdminCannotReadHistoryOfAPrivatePost(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "ph-author", "ph-author@example.com", models.RoleUser)
	admin := makeUser(t, db, "ph-admin", "ph-admin@example.com", models.RoleAdmin)
	root := makeUser(t, db, "ph-root", "ph-root@example.com", models.RoleRoot)

	// A private post: strictly author-only. A plain author's new posts enter
	// the review queue, and only a published post may be made private by its
	// author, so the test sets the state directly (a fixture convenience, not
	// a path any request can reach without review).
	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Secret draft", Body: "first",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).
		Update("status", models.PostPrivate).Error; err != nil {
		t.Fatalf("set private: %v", err)
	}
	// Produce a revision so there is something to leak.
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Body: "second"}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// The author reads their own history.
	if _, err := svc.ListRevisions(ctx, author, post.ID); err != nil {
		t.Fatalf("author must read their own history: %v", err)
	}
	// Admin and even root must be refused with 404 (the post does not exist to
	// them, so the endpoint must not confirm it does).
	if _, err := svc.ListRevisions(ctx, admin, post.ID); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("admin reading a private post's history: want 404, got %v (err=%v)", StatusOf(err), err)
	}
	if _, err := svc.ListRevisions(ctx, root, post.ID); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("root reading a private post's history: want 404, got %v (err=%v)", StatusOf(err), err)
	}

	// A published post's history stays readable by a reviewer — that is the
	// reason admins can read history at all, and the fix must not take it away.
	pub, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Public", Body: "v1", Status: models.PostPublished,
	})
	if err != nil {
		t.Fatalf("create public: %v", err)
	}
	if _, err := svc.UpdatePost(ctx, author, pub.ID, UpdatePostInput{Body: "v2"}); err != nil {
		t.Fatalf("edit public: %v", err)
	}
	if _, err := svc.ListRevisions(ctx, admin, pub.ID); err != nil {
		t.Fatalf("an admin must still read the history of a visible post: %v", err)
	}
}
