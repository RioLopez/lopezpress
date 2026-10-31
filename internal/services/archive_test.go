package services

// Paginated archive.

import (
	"context"
	"testing"
)

func TestListArchivePagePaginates(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	for i := 0; i < 5; i++ {
		makePostNoTime(t, db, alice, "published", "body "+string(rune('A'+i)), "pub "+string(rune('A'+i)))
	}
	// Non-published posts must never leak into the archive.
	makePostNoTime(t, db, alice, "private", "secret", "private one")

	first, err := svc.ListArchivePage(ctx, 1, 2)
	if err != nil {
		t.Fatalf("archive page 1: %v", err)
	}
	if len(first.Items) != 2 || first.Total != 5 || first.PageSize != 2 || first.Page != 1 {
		t.Fatalf("page 1: items=%d total=%d size=%d page=%d", len(first.Items), first.Total, first.PageSize, first.Page)
	}
	for _, p := range first.Items {
		if p.Body != "" {
			t.Fatalf("archive items must have their body stripped")
		}
		if p.Status != "published" {
			t.Fatalf("archive leaked a %q post", p.Status)
		}
	}

	third, err := svc.ListArchivePage(ctx, 3, 2)
	if err != nil {
		t.Fatalf("archive page 3: %v", err)
	}
	if len(third.Items) != 1 {
		t.Fatalf("page 3 should hold the last post, got %d", len(third.Items))
	}

	// Out of range: empty slice, real total, no error.
	beyond, err := svc.ListArchivePage(ctx, 99, 2)
	if err != nil {
		t.Fatalf("archive beyond: %v", err)
	}
	if len(beyond.Items) != 0 || beyond.Total != 5 {
		t.Fatalf("beyond range: items=%d total=%d", len(beyond.Items), beyond.Total)
	}

	// Defaults: page 1, size 50.
	def, err := svc.ListArchivePage(ctx, 0, 0)
	if err != nil {
		t.Fatalf("archive defaults: %v", err)
	}
	if def.Page != 1 || def.PageSize != 50 {
		t.Fatalf("defaults should be page 1 / size 50, got %d / %d", def.Page, def.PageSize)
	}

	// Cap: size above 200 is clamped.
	capped, err := svc.ListArchivePage(ctx, 1, 5000)
	if err != nil {
		t.Fatalf("archive cap: %v", err)
	}
	if capped.PageSize != 200 {
		t.Fatalf("size should clamp to 200, got %d", capped.PageSize)
	}
}
