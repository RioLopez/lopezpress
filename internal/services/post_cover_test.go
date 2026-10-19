package services

import (
	"context"
	"testing"

	"echo-content-api/internal/models"
)

// TestCreatePostKeepsCover is the regression test for the cross-stack
// contract drift found in the round-33 audit: the editor already sent
// `cover` on create (CreatePage -> createPost), but CreatePostInput had no
// Cover field, so Go's JSON decoder silently dropped it and a cover chosen
// at creation time vanished (only UpdatePost carried it). Assert the field
// round-trips both in the returned post and in the database row.
func TestCreatePostKeepsCover(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")
	const cover = "https://example.com/cover.png"

	p, err := svc.CreatePost(ctx, root, CreatePostInput{
		Title: "with cover",
		Body:  "body",
		Cover: cover,
	})
	if err != nil {
		t.Fatalf("CreatePost: %v", err)
	}
	if p.Cover != cover {
		t.Errorf("returned post cover = %q, want %q", p.Cover, cover)
	}

	var stored models.Post
	if err := db.Where("id = ?", p.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if stored.Cover != cover {
		t.Errorf("stored post cover = %q, want %q", stored.Cover, cover)
	}

	// An empty cover must stay empty (no phantom value).
	p2, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "no cover", Body: "body"})
	if err != nil {
		t.Fatalf("CreatePost (no cover): %v", err)
	}
	if p2.Cover != "" {
		t.Errorf("expected empty cover, got %q", p2.Cover)
	}
}
