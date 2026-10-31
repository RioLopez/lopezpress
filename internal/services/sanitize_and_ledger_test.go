package services

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// Two classes of bug live here, both of the same shape: a rule that exists on
// most paths but not all of them.
//
//  1. Outbound sanitisation. Every payload that carries a post must drop the
//     reviewer's identity. Two endpoints did not: /api/posts/recent (which is
//     authOptional, so anonymous) and the PUT response.
//  2. The image ledger must be released by every operation that removes
//     content for good. Three did not: purging a banned post, deleting a user,
//     and emptying the trash. Each one leaves a file pinned at ref_count > 0 by
//     a row that no longer exists, and the gallery then refuses to ever offer
//     that file for deletion — there is no endpoint that recomputes the count.

// reviewerIdentity asserts that a post payload leaving the service carries no
// moderation identity. ReviewNote is deliberately NOT part of this: a rejected
// author needs to know why.
func reviewerIdentity(t *testing.T, label string, p models.Post) {
	t.Helper()
	if p.ReviewedBy != "" {
		t.Fatalf("%s leaks reviewed_by = %q", label, p.ReviewedBy)
	}
	if p.ReviewedAt != nil {
		t.Fatalf("%s leaks reviewed_at = %v", label, p.ReviewedAt)
	}
}

func TestRecentPostsHideReviewerIdentity(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "recent-author", "recent-author@example.com", models.RoleUser)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Live post", Body: "text"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	// A row published long ago, before the moderation metadata was cleared on
	// approval — the shape that actually exists in an upgraded database.
	reviewed := time.Now().Add(-48 * time.Hour)
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).Updates(map[string]any{
		"status":      models.PostPublished,
		"reviewed_by": "moderator-account-id",
		"reviewed_at": reviewed,
	}).Error; err != nil {
		t.Fatalf("seed review metadata: %v", err)
	}

	posts, err := svc.GetRecentPosts(ctx)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("recent returned %d posts, want 1", len(posts))
	}
	reviewerIdentity(t, "GET /api/posts/recent", posts[0])
}

func TestUpdatePostResponseHidesReviewerIdentity(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "edit-author", "edit-author@example.com", models.RoleUser)
	root := makeUser(t, db, "edit-root", "edit-root@example.com", models.RoleRoot)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Old title", Body: "text"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).Updates(map[string]any{
		"status":      models.PostPublished,
		"reviewed_by": "moderator-account-id",
		"reviewed_at": time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed review metadata: %v", err)
	}

	// Root edits someone else's post: content changes, but the re-review rule
	// is author-only, so the status (and the stored metadata) stays put and the
	// response is the only place the sanitisation can happen.
	updated, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Title: "New title"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	reviewerIdentity(t, "PUT /api/posts/{id}", updated)
}

func TestPurgingABannedPostReleasesItsImageEdges(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "purge-author", "purge-author@example.com", models.RoleUser)
	root := makeUser(t, db, "purge-root", "purge-root@example.com", models.RoleRoot)
	pic := makeLibraryImage(t, db, "up-purge", "purged.png", author.ID, "post")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Bannable", Body: "bad ![](/uploads/purged.png)",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if got := refCountOf(t, db, pic.ID); got != 1 {
		t.Fatalf("setup: ref_count = %d, want 1", got)
	}

	if _, err := svc.BanPost(ctx, root, post.ID); err != nil {
		t.Fatalf("ban: %v", err)
	}
	if err := svc.DeleteBannedPermanently(ctx, root, post.ID); err != nil {
		t.Fatalf("purge: %v", err)
	}

	if got := refCountOf(t, db, pic.ID); got != 0 {
		t.Fatalf("ref_count = %d after the post was purged, want 0 — the file is now permanently undeletable", got)
	}
	var edges int64
	if err := db.Model(&models.UploadRef{}).Where("post_id = ?", post.ID).Count(&edges).Error; err != nil {
		t.Fatalf("count edges: %v", err)
	}
	if edges != 0 {
		t.Fatalf("%d ledger edges survived the purge", edges)
	}
}

func TestDeletingAUserReleasesTheAvatarEdge(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	root := makeUser(t, db, "del-root", "del-root@example.com", models.RoleRoot)
	leaver := makeUser(t, db, "del-user", "del-user@example.com", models.RoleUser)
	avatar := makeLibraryImage(t, db, "up-avatar-del", "avatar-del.png", leaver.ID, "avatar")

	// The state a profile save leaves behind: the user points at the file and
	// the ledger has the edge.
	if err := db.Model(&models.User{}).Where("id = ?", leaver.ID).
		Update("avatar", "/uploads/avatar-del.png").Error; err != nil {
		t.Fatalf("set avatar: %v", err)
	}
	if err := setSlotUploadRefTx(db, "avatar", leaver.ID, "/uploads/avatar-del.png"); err != nil {
		t.Fatalf("set avatar edge: %v", err)
	}
	if got := refCountOf(t, db, avatar.ID); got != 1 {
		t.Fatalf("setup: avatar ref_count = %d, want 1", got)
	}

	if err := svc.DeleteUser(ctx, root, leaver.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if got := refCountOf(t, db, avatar.ID); got != 0 {
		t.Fatalf("avatar ref_count = %d after the account was deleted, want 0", got)
	}
	var edges int64
	if err := db.Model(&models.UploadRef{}).Where("user_id = ? AND post_id = ''", leaver.ID).
		Count(&edges).Error; err != nil {
		t.Fatalf("count edges: %v", err)
	}
	if edges != 0 {
		t.Fatalf("%d user-level ledger edges survived the delete", edges)
	}
}

func TestEmptyTrashReleasesImageEdges(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "empty-author", "empty-author@example.com", models.RoleUser)
	pic := makeLibraryImage(t, db, "up-empty", "trashed.png", author.ID, "post")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Trashed", Body: "look ![](/uploads/trashed.png)",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := svc.DeletePost(ctx, author, post.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Simulate the residue this fix is about: a trashed post whose edges are
	// still in the ledger (DeletePost has since learned to release them, but an
	// existing database can carry rows written before that).
	if err := syncPostUploadRefsTx(db, post.ID, author.ID, post.Body, post.Cover); err != nil {
		t.Fatalf("seed edges: %v", err)
	}
	if got := refCountOf(t, db, pic.ID); got != 1 {
		t.Fatalf("setup: ref_count = %d, want 1", got)
	}

	n, err := svc.EmptyTrash(ctx, author)
	if err != nil {
		t.Fatalf("empty trash: %v", err)
	}
	if n != 1 {
		t.Fatalf("empty trash removed %d posts, want 1", n)
	}
	if got := refCountOf(t, db, pic.ID); got != 0 {
		t.Fatalf("ref_count = %d after the trash was emptied, want 0", got)
	}
	var edges int64
	if err := db.Model(&models.UploadRef{}).Where("post_id = ?", post.ID).Count(&edges).Error; err != nil {
		t.Fatalf("count edges: %v", err)
	}
	if edges != 0 {
		t.Fatalf("%d ledger edges survived emptying the trash", edges)
	}
}

// EmptyTrash must not strip the edges of a post that merely left its scope. The
// ids are resolved before the delete and the scope is re-applied on it, so the
// two sets can differ by one role change; cleaning up "the ids we read" would
// then release edges belonging to a post that is still in the trash.
func TestEmptyTrashKeepsEdgesOfPostsItDidNotDelete(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	admin := makeUser(t, db, "scope-admin", "scope-admin@example.com", models.RoleAdmin)
	peer := makeUser(t, db, "scope-peer", "scope-peer@example.com", models.RoleAdmin)
	pic := makeLibraryImage(t, db, "up-scope", "kept.png", peer.ID, "post")

	post, err := svc.CreatePost(ctx, peer, CreatePostInput{
		Title: "Peer post", Body: "look ![](/uploads/kept.png)",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := svc.DeletePost(ctx, peer, post.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// Put the edge back so its survival is observable.
	if err := syncPostUploadRefsTx(db, post.ID, peer.ID, post.Body, post.Cover); err != nil {
		t.Fatalf("seed edges: %v", err)
	}

	// A plain admin may not manage another admin's trashed post, so it is
	// listed for nobody here — EmptyTrash sees zero rows and must change nothing.
	n, err := svc.EmptyTrash(ctx, admin)
	if err != nil {
		t.Fatalf("empty trash: %v", err)
	}
	if n != 0 {
		t.Fatalf("empty trash removed %d posts, want 0", n)
	}
	if got := refCountOf(t, db, pic.ID); got != 1 {
		t.Fatalf("ref_count = %d for a post outside the caller's scope, want 1", got)
	}
}

// The two wholesale-replace paths share one content-table list, because the
// copies drifted: the import cleared post_revisions and the snapshot restore did
// not. This pins the contract both callers rely on — everything that describes
// CONTENT goes, and the instance state (library rows, site config, accounts)
// is left for the caller to decide on.
func TestClearContentTablesKeepsInstanceState(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	root := makeUser(t, db, "reset-root", "reset-root@example.com", models.RoleRoot)
	pic := makeLibraryImage(t, db, "up-reset", "kept.png", root.ID, "post")

	cat, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Reset cat"})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	post, err := svc.CreatePost(ctx, root, CreatePostInput{
		Title: "Versioned", Body: "look ![](/uploads/kept.png)", CategoryID: &cat.ID,
		Tags: []string{"reset-tag"},
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if _, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Title: "Second", Status: models.PostPublished}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	if err := clearContentTablesTx(db); err != nil {
		t.Fatalf("clear: %v", err)
	}

	for label, count := range map[string]func(*testing.T) int64{
		"posts":          func(t *testing.T) int64 { return countOf(t, db, &models.Post{}, true) },
		"post_tags":      func(t *testing.T) int64 { return countOf(t, db, &models.PostTag{}, true) },
		"post_revisions": func(t *testing.T) int64 { return countOf(t, db, &models.PostRevision{}, true) },
		"tags":           func(t *testing.T) int64 { return countOf(t, db, &models.Tag{}, true) },
		"categories":     func(t *testing.T) int64 { return countOf(t, db, &models.Category{}, true) },
	} {
		if got := count(t); got != 0 {
			t.Fatalf("%d rows of %s survived the content clear", got, label)
		}
	}

	// Instance state, which each caller decides on separately.
	if got := countOf(t, db, &models.UploadFile{}, true); got != 1 {
		t.Fatalf("upload_files count = %d, want 1 (the library is not content)", got)
	}
	if got := refCountOf(t, db, pic.ID); got != 0 {
		t.Fatalf("ref_count = %d, want 0 — the edges must be rebuilt by the caller, not kept", got)
	}
	if got := countOf(t, db, &models.User{}, false); got != 1 {
		t.Fatalf("users count = %d, want 1 (accounts are replaced separately)", got)
	}
}

func countOf(t *testing.T, db *gorm.DB, model any, unscoped bool) int64 {
	t.Helper()
	q := db
	if unscoped {
		q = db.Unscoped()
	}
	var n int64
	if err := q.Model(model).Count(&n).Error; err != nil {
		t.Fatalf("count %T: %v", model, err)
	}
	return n
}
