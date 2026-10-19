package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// TestCreatePostSavesTagsAtomically verifies that CreatePost persists the post
// row and its tag associations together (P3-1): a post created with tags must
// carry exactly those tags, with blank and duplicate names collapsed. This
// guards the transaction refactor of CreatePost.
func TestCreatePostSavesTagsAtomically(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	author := makeUser(t, db, "tagger", "tagger@example.com", "user")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "With Tags",
		Body:  "hello",
		Tags:  []string{"Go", "Web", "  ", "Go"}, // blank + duplicate
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}

	var ptCount int64
	if err := db.Model(&models.PostTag{}).Where("post_id = ?", post.ID).Count(&ptCount).Error; err != nil {
		t.Fatalf("count post_tags: %v", err)
	}
	if ptCount != 2 {
		t.Fatalf("expected 2 tag associations (blank dropped, dup collapsed), got %d", ptCount)
	}

	var tagCount int64
	if err := db.Model(&models.Tag{}).Count(&tagCount).Error; err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tagCount != 2 {
		t.Fatalf("expected 2 distinct tags, got %d", tagCount)
	}
}

// TestUpdatePostReplacesTagsInTransaction verifies that UpdatePost's tag
// handling (Clear + Append inside a transaction, P3-1) actually replaces the
// association set rather than appending: an update with a new tag list drops
// the old tags, and an empty list clears them.
func TestUpdatePostReplacesTagsInTransaction(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	author := makeUser(t, db, "editor", "editor@example.com", "user")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "T", Body: "x", Tags: []string{"a", "b"}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Tags: []string{"c"}})
	if err != nil {
		t.Fatalf("update replace: %v", err)
	}
	var ptCount int64
	if err := db.Model(&models.PostTag{}).Where("post_id = ?", post.ID).Count(&ptCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if ptCount != 1 {
		t.Fatalf("expected 1 tag after replace, got %d", ptCount)
	}
	if len(updated.Tags) != 1 || updated.Tags[0].Name != "c" {
		t.Fatalf("expected returned post to carry tag 'c', got %+v", updated.Tags)
	}

	// Empty list must clear all tags.
	updated2, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Tags: []string{}})
	if err != nil {
		t.Fatalf("update clear: %v", err)
	}
	if err := db.Model(&models.PostTag{}).Where("post_id = ?", post.ID).Count(&ptCount).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if ptCount != 0 {
		t.Fatalf("expected 0 tags after clearing, got %d", ptCount)
	}
	if len(updated2.Tags) != 0 {
		t.Fatalf("expected no tags, got %+v", updated2.Tags)
	}
}
