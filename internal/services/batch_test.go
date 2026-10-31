package services

// Bulk actions on posts delegate to the single-item service methods,
// so the role matrix and status machine are inherited rather than re-written.

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

func TestBatchPublishRootOnly(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	p1 := makePostNoTime(t, db, root, "private", "b1", "Private one")
	p2 := makePostNoTime(t, db, root, "private", "b2", "Private two")

	// admin may not publish — the whole batch is refused up front.
	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{
		IDs: []string{p1.ID}, Action: BatchPublish,
	}); err != nil {
		// root is allowed: this branch must NOT trigger.
		t.Fatalf("root publish should succeed, got %v", err)
	}

	res, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{
		IDs: []string{p2.ID}, Action: BatchPublish,
	})
	if err != nil || res.Affected != 1 {
		t.Fatalf("root publish: affected=%d err=%v", res.Affected, err)
	}

	var reloaded models.Post
	if err := db.Where("id = ?", p2.ID).First(&reloaded).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Status != models.PostPublished {
		t.Fatalf("status should be published, got %q", reloaded.Status)
	}
}

func TestBatchPublishRejectedForAdmin(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	p := makePostNoTime(t, db, admin, "private", "x", "x")

	if _, err := svc.BatchUpdatePosts(ctx, admin, BatchPostInput{IDs: []string{p.ID}, Action: BatchPublish}); StatusOf(err) != 403 {
		t.Fatalf("admin publish must be 403, got %v", err)
	}
}

// A batch must not fail wholesale when one row is off-limits: that row is
// reported as skipped, the rest still apply.
func TestBatchPartialPermission(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	plain := makeUser(t, db, "alice", "alice@example.com", "user")

	plainPost := makePostNoTime(t, db, plain, "published", "p", "Plain post")
	rootPost := makePostNoTime(t, db, root, "published", "r", "Root post")

	res, err := svc.BatchUpdatePosts(ctx, admin, BatchPostInput{
		IDs: []string{plainPost.ID, rootPost.ID}, Action: BatchPrivate,
	})
	if err != nil {
		t.Fatalf("batch private: %v", err)
	}
	if res.Affected != 1 {
		t.Fatalf("expected 1 affected, got %d (%+v)", res.Affected, res.Skipped)
	}
	if len(res.Skipped) != 1 || res.Skipped[0].ID != rootPost.ID {
		t.Fatalf("root's post should be skipped, got %+v", res.Skipped)
	}
	if res.Skipped[0].Reason == "" {
		t.Fatalf("skipped rows must carry a reason")
	}
}

func TestBatchAddTagsIsIdempotent(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	post := makePostNoTime(t, db, root, "published", "t", "Tagged post")

	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{
		IDs: []string{post.ID}, Action: BatchAddTags, Tags: []string{"Go"},
	}); err != nil {
		t.Fatalf("first add: %v", err)
	}
	// Adding an existing tag plus a new one must not duplicate "Go".
	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{
		IDs: []string{post.ID}, Action: BatchAddTags, Tags: []string{"Go", "SQLite"},
	}); err != nil {
		t.Fatalf("second add: %v", err)
	}

	var links []models.PostTag
	if err := db.Where("post_id = ?", post.ID).Find(&links).Error; err != nil {
		t.Fatalf("load links: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected exactly 2 tag links, got %d", len(links))
	}

	// Remove drops only the named tag.
	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{
		IDs: []string{post.ID}, Action: BatchRemoveTags, Tags: []string{"go"},
	}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := db.Where("post_id = ?", post.ID).Find(&links).Error; err != nil {
		t.Fatalf("reload links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 tag link after removal, got %d", len(links))
	}
}

func TestBatchValidation(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{Action: BatchDelete}); StatusOf(err) != 400 {
		t.Fatalf("empty ids should be 400, got %v", err)
	}
	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{IDs: []string{"x"}, Action: "nuke"}); StatusOf(err) != 400 {
		t.Fatalf("unknown action should be 400, got %v", err)
	}
	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{IDs: []string{"x"}, Action: BatchSetCategory}); StatusOf(err) != 400 {
		t.Fatalf("missing category_id should be 400, got %v", err)
	}

	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = "id"
	}
	if _, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{IDs: tooMany, Action: BatchDelete}); StatusOf(err) != 400 {
		t.Fatalf("101 ids should be 400, got %v", err)
	}

	// Missing posts are skipped, not fatal.
	res, err := svc.BatchUpdatePosts(ctx, root, BatchPostInput{IDs: []string{"nope"}, Action: BatchDelete})
	if err != nil {
		t.Fatalf("missing post should not fail the batch: %v", err)
	}
	if res.Affected != 0 || len(res.Skipped) != 1 {
		t.Fatalf("expected 0 affected / 1 skipped, got %+v", res)
	}
}
