package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// The "cleared" half of the update protocol.
//
// Both fields below described the same user-visible defect shape: a control
// that answers 200 and keeps the old value. Cover was a plain string, so "" was
// indistinguishable from "the client did not mention a cover" and 移除封面
// wrote the old image straight back; category_id had no encoding for 不分类 at
// all. A control that silently does nothing is worse than one that refuses —
// the user has no error to react to, so they retry, see the same result, and
// conclude the site is broken.

func TestClearingTheCoverRemovesIt(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "cover-clear", "cover-clear@example.com", models.RoleUser)

	// A remote cover: this test is about the field, not the image ledger, and
	// an /uploads/ name with no row behind it would fail the ledger's FK.
	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "with cover", Body: "body", Cover: "https://example.com/c.png",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if post.Cover == "" {
		t.Fatalf("fixture: the post should have started with a cover")
	}

	out, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Cover: strPtr("")})
	if err != nil {
		t.Fatalf("clearing the cover must not fail: %v", err)
	}
	if out.Cover != "" {
		t.Fatalf("cover = %q after clearing, want empty", out.Cover)
	}
	// The response is re-read from a fresh row, so asserting on it also proves
	// the write reached the database rather than only the in-memory copy.
	var stored models.Post
	if err := db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Cover != "" {
		t.Fatalf("stored cover = %q, want empty", stored.Cover)
	}
}

// TestAbsentCoverLeavesItAlone is the other half of the same distinction: a
// client that never mentions the cover (the batch editor, a title-only API
// call) must not wipe it. Pointerising the field is only safe if nil keeps
// meaning "unchanged".
func TestAbsentCoverLeavesItAlone(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "cover-keep", "cover-keep@example.com", models.RoleUser)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "keeps its cover", Body: "body", Cover: "https://example.com/k.png",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	out, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Body: "a new body"})
	if err != nil {
		t.Fatalf("title-only edit failed: %v", err)
	}
	if out.Cover != "https://example.com/k.png" {
		t.Fatalf("cover = %q, want it untouched", out.Cover)
	}
}

// TestClearingTheCoverReopensReview: the cover is the face of the article in
// every listing, so removing it changes what a reader sees — the same re-review
// rule that replacing it already triggers.
func TestClearingTheCoverReopensReview(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "cover-rr", "cover-rr@example.com", models.RoleUser)
	admin := makeUser(t, db, "cover-rr-admin", "cover-rr-admin@example.com", models.RoleAdmin)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "cover goes back for review", Body: "body", Cover: "https://example.com/r.png",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.ReviewPost(ctx, admin, post.ID, ReviewPostInput{Action: "approve"}); err != nil {
		t.Fatalf("approve: %v", err)
	}

	out, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Cover: strPtr("")})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if out.Status != models.PostPending {
		t.Fatalf("removing the cover of a live post must queue it for review, got %q", out.Status)
	}
}

// TestZeroCategoryMeansNoCategory pins the encoding: nil is taken by
// "unchanged", so 不分类 needs its own value and 0 is the one
// UpdateCategoryInput.ParentID already uses for "top level".
func TestZeroCategoryMeansNoCategory(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "cat-clear", "cat-clear@example.com", models.RoleUser)

	cat := models.Category{Name: "News"}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "filed under News", Body: "body", CategoryID: &cat.ID,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if post.CategoryID == nil || *post.CategoryID != cat.ID {
		t.Fatalf("fixture: post should start in category %d, got %v", cat.ID, post.CategoryID)
	}

	zero := int64(0)
	out, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{CategoryID: &zero})
	if err != nil {
		t.Fatalf("clearing the category must not fail: %v", err)
	}
	if out.CategoryID != nil {
		t.Fatalf("category_id = %v after 不分类, want nil", *out.CategoryID)
	}
	// A nil *int64 in the update map has to become SQL NULL, not be skipped —
	// a skipped entry leaves the old category in the row while the response
	// (built from the in-memory copy) claims otherwise.
	var stored models.Post
	if err := db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.CategoryID != nil {
		t.Fatalf("stored category_id = %v, want NULL", *stored.CategoryID)
	}
}

// TestAdminCannotClearSomeoneElsesCover: clearing a cover is a content edit, so
// the reviewer-impersonation guard has to treat a present-but-empty cover the
// same way it treats a new one. The gate used to test `in.Cover != ""`, which
// an empty string passed.
func TestAdminCannotClearSomeoneElsesCover(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "cover-owned", "cover-owned@example.com", models.RoleUser)
	admin := makeUser(t, db, "cover-admin", "cover-admin@example.com", models.RoleAdmin)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "not the admin's post", Body: "body", Cover: "https://example.com/o.png",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.UpdatePost(ctx, admin, post.ID, UpdatePostInput{Cover: strPtr("")}); StatusOf(err) != 403 {
		t.Fatalf("admin clearing another author's cover = %v, want 403", StatusOf(err))
	}
}
