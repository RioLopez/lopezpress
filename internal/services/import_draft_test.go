package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// A "draft: true" front matter used to force the imported post to the private
// status, because the build has no draft state and the importer special-cased
// it. That special case is gone: the post now follows the normal rule (root
// publishes directly, everyone else lands in the review queue). This test pins
// the behaviour so a future reader cannot reintroduce a silent "draft → private"
// mapping.
func TestImportMarkdownIgnoresDraft(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "imp-root", "imp-root@example.com", "root")

	body := "---\ntitle: Draft Post\ndraft: true\n---\n\nBody text here.\n"
	if _, err := svc.ImportMarkdown(ctx, root, []MarkdownFile{{Name: "draft.md", Body: []byte(body)}}); err != nil {
		t.Fatalf("import: %v", err)
	}

	var posts []models.Post
	if err := db.Where("title = ?", "Draft Post").Find(&posts).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("expected exactly one imported post, got %d", len(posts))
	}
	if posts[0].Status == models.PostPrivate {
		t.Errorf("draft:true must not force private status; got %q", posts[0].Status)
	}
	if posts[0].Status != models.PostPublished {
		t.Errorf("root import should publish directly, got %q", posts[0].Status)
	}
}
