package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// newPost helper: create a live post in the service's DB with the given title/body.
func newPost(t *testing.T, svc *Service, author models.User, title, body, status string) models.Post {
	t.Helper()
	in := CreatePostInput{Title: title, Body: body, Status: status}
	p, err := svc.CreatePost(context.Background(), author, in)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	return p
}

// TestSearchHitByTitleBodyAndAuthor covers three cases:
//   - searching by a word in body
//   - searching by a word in title
//   - searching by author username
//
// All go through the single ListPostsPage.Search path; the driver (pure-Go
// SQLite, no FTS5) performs these via LIKE.
func TestSearchHitByTitleBodyAndAuthor(t *testing.T) {
	svc, db := newTestService(t)
	author := makeUser(t, db, "bob_dev", "bob@example.com", "admin")

	p1 := newPost(t, svc, author, "Quantum Computing Primer", "Qubits, superposition, and entanglement", "published")
	p2 := newPost(t, svc, author, "Gardening Tips", "Prune your tomato plants weekly", "published")

	ctx := context.Background()

	// 1. Body word hit.
	page, err := svc.ListPostsPage(ctx, author, ListPostsQuery{Search: "quantum", PageSize: 20})
	if err != nil {
		t.Fatalf("search 'quantum': %v", err)
	}
	if page.Total != 1 || page.Items[0].ID != p1.ID {
		t.Fatalf("expected p1 for 'quantum', got total=%d", page.Total)
	}

	// 2. Title word hit.
	page, err = svc.ListPostsPage(ctx, author, ListPostsQuery{Search: "gardening", PageSize: 20})
	if err != nil {
		t.Fatalf("search 'gardening': %v", err)
	}
	if page.Total != 1 || page.Items[0].ID != p2.ID {
		t.Fatalf("expected p2 for 'gardening', got total=%d", page.Total)
	}

	// 3. Author username hit (same username as the author's).
	page, err = svc.ListPostsPage(ctx, author, ListPostsQuery{Search: "bob_dev", PageSize: 20})
	if err != nil {
		t.Fatalf("search 'bob_dev': %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("expected both of author's posts for 'bob_dev', got %d", page.Total)
	}
}

// TestSearchSoftDeleteIsolation verifies that a soft-deleted post is NOT
// returned by keyword search, and that restoring it puts it back.
func TestSearchSoftDeleteIsolation(t *testing.T) {
	svc, db := newTestService(t)
	author := makeUser(t, db, "carol", "carol@example.com", "admin")

	p := newPost(t, svc, author, "Zebra pattern", "Zebra pattern is beautiful", "published")
	ctx := context.Background()

	// Baseline: search hits the live post.
	page, err := svc.ListPostsPage(ctx, author, ListPostsQuery{Search: "zebra", PageSize: 20})
	if err != nil {
		t.Fatalf("baseline search: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("baseline expected 1, got %d", page.Total)
	}

	// Soft delete: search no longer hits the post.
	if err := svc.DeletePost(ctx, author, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	page, err = svc.ListPostsPage(ctx, author, ListPostsQuery{Search: "zebra", PageSize: 20})
	if err != nil {
		t.Fatalf("search after delete: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("expected 0 after soft-delete, got %d", page.Total)
	}

	// Restore: search hits the post again.
	if err := svc.RestoreTrash(ctx, author, p.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	page, err = svc.ListPostsPage(ctx, author, ListPostsQuery{Search: "zebra", PageSize: 20})
	if err != nil {
		t.Fatalf("search after restore: %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("expected 1 after restore, got %d", page.Total)
	}
}
