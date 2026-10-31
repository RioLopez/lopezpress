package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// TestCreatePostSavesTagsAtomically verifies that CreatePost persists the post
// row and its tag associations together: a post created with tags must
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

// TestTagNamesAreCaseInsensitive pins the fix for a real authoring papercut:
// typing "go" when the site already had "Go" silently created a second tag,
// so the same concept split into go / Go / GO and every one of them filtered
// to a different, near-empty list. Tag resolution now compares names without
// regard to case and collapses interior whitespace, and the ?tag= filter
// matches the same way.
func TestTagNamesAreCaseInsensitive(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", "root")

	first, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "A", Body: "x", Tags: []string{"Go"}})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	// Different spellings of the same tag, plus one with doubled whitespace.
	for _, spelling := range []string{"go", "GO", "  go  "} {
		if _, err := svc.CreatePost(ctx, root, CreatePostInput{
			Title: "B-" + spelling, Body: "x", Tags: []string{spelling},
		}); err != nil {
			t.Fatalf("create with %q: %v", spelling, err)
		}
	}

	var tagCount int64
	if err := db.Model(&models.Tag{}).Count(&tagCount).Error; err != nil {
		t.Fatalf("count tags: %v", err)
	}
	if tagCount != 1 {
		t.Fatalf("case variants must collapse into one tag, got %d rows", tagCount)
	}

	// The stored spelling is the one that was created first.
	var only models.Tag
	if err := db.First(&only).Error; err != nil {
		t.Fatalf("load tag: %v", err)
	}
	if only.Name != "Go" {
		t.Fatalf("expected the original spelling to win, got %q", only.Name)
	}

	// The filter is case-insensitive too, so ?tag=go finds the "Go" posts.
	page, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{Page: 1, PageSize: 50, Tag: "go"})
	if err != nil {
		t.Fatalf("filter by tag: %v", err)
	}
	if page.Total != 4 {
		t.Fatalf("expected 4 posts under tag go/Go, got %d", page.Total)
	}
	if len(first.Tags) != 1 || first.Tags[0].Name != "Go" {
		t.Fatalf("expected the first post to carry tag Go, got %+v", first.Tags)
	}
}

// TestUpdatePostReplacesTagsInTransaction verifies that UpdatePost's tag
// handling (Clear + Append inside a transaction) actually replaces the
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
