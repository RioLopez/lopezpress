package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// Spec 35: related posts = same category OR shared tag, published only.
func TestRelatedPostsByCategoryAndTag(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	cat, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Go"})
	if err != nil {
		t.Fatalf("category: %v", err)
	}
	other, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Life"})
	if err != nil {
		t.Fatalf("category: %v", err)
	}

	mk := func(title string, catID int64, tags []string) models.Post {
		p, err := svc.CreatePost(ctx, root, CreatePostInput{
			Title: title, Body: "body", CategoryID: &catID, Tags: tags,
		})
		if err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
		return p
	}

	target := mk("target", cat.ID, []string{"golang"})
	sameCat := mk("same category", cat.ID, nil)
	sameTag := mk("same tag", other.ID, []string{"golang"})
	unrelated := mk("unrelated", other.ID, []string{"cooking"})

	got, err := svc.RelatedPosts(ctx, target.ID, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	ids := map[string]bool{}
	for _, p := range got {
		ids[p.ID] = true
	}
	if !ids[sameCat.ID] {
		t.Fatalf("a post in the same category should be related")
	}
	if !ids[sameTag.ID] {
		t.Fatalf("a post sharing a tag should be related")
	}
	if ids[unrelated.ID] {
		t.Fatalf("an unrelated post must not appear")
	}
	if ids[target.ID] {
		t.Fatalf("the post itself must not appear in its own related list")
	}
}

// A banned/pending post must never surface as "related": the reader would see
// a link that answers 404.
func TestRelatedPostsExcludesUnpublished(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	cat, _ := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Go"})
	target, _ := svc.CreatePost(ctx, root, CreatePostInput{Title: "target", Body: "b", CategoryID: &cat.ID})
	hidden, _ := svc.CreatePost(ctx, root, CreatePostInput{Title: "hidden", Body: "b", CategoryID: &cat.ID})
	if err := db.Model(&models.Post{}).Where("id = ?", hidden.ID).Update("status", "banned").Error; err != nil {
		t.Fatalf("hide post: %v", err)
	}

	got, err := svc.RelatedPosts(ctx, target.ID, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	for _, p := range got {
		if p.ID == hidden.ID {
			t.Fatalf("a banned post must not be offered as related")
		}
	}
}

// No category + no tags is a normal state, not an error: the reader page simply
// hides the block.
func TestRelatedPostsEmptyForIsolatedPost(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	isolated, _ := svc.CreatePost(ctx, root, CreatePostInput{Title: "lonely", Body: "b"})
	other, _ := svc.CreatePost(ctx, root, CreatePostInput{Title: "other", Body: "b"})

	got, err := svc.RelatedPosts(ctx, isolated.ID, 5)
	if err != nil {
		t.Fatalf("related: %v", err)
	}
	if got == nil {
		t.Fatalf("expected an empty slice, not nil (the client expects JSON [])")
	}
	if len(got) != 0 {
		t.Fatalf("expected no related posts, got %d (other=%s)", len(got), other.ID)
	}
}

// An unknown id is a 404, not an empty list: the caller asked about a post
// that does not exist.
func TestRelatedPostsUnknownPost(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	if _, err := svc.RelatedPosts(context.Background(), "does-not-exist", 5); err == nil {
		t.Fatalf("expected a not-found error")
	} else if StatusOf(err) != 404 {
		t.Fatalf("expected 404, got %d", StatusOf(err))
	}
}
