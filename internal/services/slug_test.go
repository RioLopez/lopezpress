package services

// Permanent links (slug).

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Hello World":               "hello-world",
		"  Go 的  并发模型  ":            "go-的-并发模型",
		"Post #1: Getting Started!": "post-1-getting-started",
		"":                          "",
		"!!!":                       "",
		"a_b.c":                     "a-b-c",
		"UPPER 123":                 "upper-123",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Fatalf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCreatePostAllocatesUniqueSlug(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")

	first, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "Hello World", Body: "b1"})
	if err != nil {
		t.Fatalf("create 1: %v", err)
	}
	if first.Slug != "hello-world" {
		t.Fatalf("expected slug hello-world, got %q", first.Slug)
	}

	// Same title, second post: the slug must not collide.
	second, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "Hello World", Body: "b2"})
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if second.Slug != "hello-world-2" {
		t.Fatalf("expected hello-world-2, got %q", second.Slug)
	}

	// A title with nothing slug-able falls back to the id (never empty).
	third, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "!!!", Body: "b3"})
	if err != nil {
		t.Fatalf("create 3: %v", err)
	}
	if third.Slug != third.ID {
		t.Fatalf("expected slug fallback to the id, got %q vs %q", third.Slug, third.ID)
	}

	// An explicit slug wins over the title.
	fourth, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "Whatever", Slug: "Custom Link", Body: "b4"})
	if err != nil {
		t.Fatalf("create 4: %v", err)
	}
	if fourth.Slug != "custom-link" {
		t.Fatalf("expected custom-link, got %q", fourth.Slug)
	}
}

func TestGetPostBySlugRespectsVisibility(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	alice := makeUser(t, db, "alice", "alice@example.com", "user")

	pub := makePostNoTime(t, db, alice, "published", "b", "Public Post")
	priv := makePostNoTime(t, db, alice, "private", "b", "Private Post")
	if err := db.Model(&models.Post{}).Where("id = ?", pub.ID).Update("slug", "public-post").Error; err != nil {
		t.Fatalf("set slug: %v", err)
	}
	if err := db.Model(&models.Post{}).Where("id = ?", priv.ID).Update("slug", "private-post").Error; err != nil {
		t.Fatalf("set slug: %v", err)
	}

	got, err := svc.GetPostBySlug(ctx, anonymous(), "public-post")
	if err != nil {
		t.Fatalf("anon by slug: %v", err)
	}
	if got.ID != pub.ID {
		t.Fatalf("resolved the wrong post")
	}

	// Restricted content answers 404, not 403 — same rule as the id route.
	if _, err := svc.GetPostBySlug(ctx, anonymous(), "private-post"); StatusOf(err) != 404 {
		t.Fatalf("private slug must 404 for anonymous, got %v", err)
	}
	// The author may read their own private post by slug.
	if _, err := svc.GetPostBySlug(ctx, alice, "private-post"); err != nil {
		t.Fatalf("author should resolve their own private slug: %v", err)
	}
	// Unknown slug.
	if _, err := svc.GetPostBySlug(ctx, root, "nope"); StatusOf(err) != 404 {
		t.Fatalf("unknown slug must 404, got %v", err)
	}
}

func TestUpdatePostCanChangeSlug(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	post, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "Original Title", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	original := post.Slug

	// A title-only edit must NOT move the permanent link.
	updated, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Title: "Renamed Title"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if updated.Slug != original {
		t.Fatalf("renaming must keep the slug: %q -> %q", original, updated.Slug)
	}

	// An explicit slug changes it.
	moved, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Slug: "Brand New Link"})
	if err != nil {
		t.Fatalf("slug change: %v", err)
	}
	if moved.Slug != "brand-new-link" {
		t.Fatalf("expected brand-new-link, got %q", moved.Slug)
	}
}
