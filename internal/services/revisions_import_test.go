package services

// Spec 31 (revisions) + spec 32 (markdown import).

import (
	"context"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// --- revisions ---

func TestUpdateSnapshotsPreviousVersion(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	post, err := svc.CreatePost(ctx, root, CreatePostInput{Title: "v1", Body: "first body"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Title: "v2", Body: "second body"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	revs, err := svc.ListRevisions(ctx, root, post.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(revs) != 1 {
		t.Fatalf("expected 1 revision, got %d", len(revs))
	}
	if revs[0].Body != "first body" || revs[0].Title != "v1" {
		t.Fatalf("revision should hold the PREVIOUS state, got %q / %q", revs[0].Title, revs[0].Body)
	}
}

func TestRevisionsAreCappedAt20(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	post, _ := svc.CreatePost(ctx, root, CreatePostInput{Title: "cap", Body: "b0"})
	for i := 1; i <= 25; i++ {
		if _, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Body: "b" + itoa(i)}); err != nil {
			t.Fatalf("update %d: %v", i, err)
		}
	}
	var n int64
	if err := db.Model(&models.PostRevision{}).Where("post_id = ?", post.ID).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != maxRevisionsPerPost {
		t.Fatalf("expected %d revisions, got %d", maxRevisionsPerPost, n)
	}
}

func TestRestoreRevisionKeepsStatus(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "admin", "admin@example.com", "root")
	post, _ := svc.CreatePost(ctx, root, CreatePostInput{Title: "orig", Body: "good body", Status: "published"})
	// Break it.
	if _, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Body: "bad body", Status: "private"}); err != nil {
		t.Fatalf("break: %v", err)
	}

	revs, _ := svc.ListRevisions(ctx, root, post.ID)
	if len(revs) == 0 {
		t.Fatalf("no revisions to restore")
	}
	restored, err := svc.RestoreRevision(ctx, root, post.ID, revs[0].ID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Body != "good body" {
		t.Fatalf("body should be restored, got %q", restored.Body)
	}
	if restored.Status != models.PostPrivate {
		t.Fatalf("status must NOT roll back, got %q", restored.Status)
	}
	// The restore itself must be undoable.
	after, _ := svc.ListRevisions(ctx, root, post.ID)
	if len(after) != 2 {
		t.Fatalf("restore should add a snapshot, got %d revisions", len(after))
	}
}

func TestRevisionPermissionsAndScope(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	bob := makeUser(t, db, "bob", "bob@example.com", "user")

	post, _ := svc.CreatePost(ctx, alice, CreatePostInput{Title: "alice", Body: "a"})
	if _, err := svc.UpdatePost(ctx, alice, post.ID, UpdatePostInput{Body: "a2"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	revs, _ := svc.ListRevisions(ctx, alice, post.ID)
	if len(revs) != 1 {
		t.Fatalf("expected 1 revision")
	}

	if _, err := svc.ListRevisions(ctx, bob, post.ID); StatusOf(err) != 403 {
		t.Fatalf("bob must be 403, got %v", err)
	}
	if _, err := svc.ListRevisions(ctx, admin, post.ID); err != nil {
		t.Fatalf("admin should read a plain user's history: %v", err)
	}

	// A revision id from another post must not be usable.
	other, _ := svc.CreatePost(ctx, alice, CreatePostInput{Title: "other", Body: "o"})
	svc.UpdatePost(ctx, alice, other.ID, UpdatePostInput{Body: "o2"})
	otherRevs, _ := svc.ListRevisions(ctx, alice, other.ID)
	if _, err := svc.RestoreRevision(ctx, alice, post.ID, otherRevs[0].ID); StatusOf(err) != 404 {
		t.Fatalf("cross-post revision must 404, got %v", err)
	}
}

// --- markdown import ---

func TestImportMarkdownFrontmatter(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	content := `---
title: 从 WordPress 迁移
date: 2020-05-06
category: 技术笔记
tags: [Go, 迁移]
---

正文第一段。
`
	res, err := svc.ImportMarkdown(ctx, root, []MarkdownFile{{Name: "migrate.md", Body: []byte(content)}})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created != 1 || len(res.Skipped) != 0 {
		t.Fatalf("expected 1 created / 0 skipped, got %+v", res)
	}

	var post models.Post
	if err := db.Where("title = ?", "从 WordPress 迁移").First(&post).Error; err != nil {
		t.Fatalf("post not created: %v", err)
	}
	if post.Status != models.PostPublished {
		t.Fatalf("root import should publish, got %q", post.Status)
	}
	if post.Slug == "" {
		t.Fatalf("imported post must get a slug")
	}
	want := time.Date(2020, 5, 6, 0, 0, 0, 0, time.UTC)
	if !post.CreatedAt.UTC().Equal(want) {
		t.Fatalf("created_at should keep the frontmatter date, got %v", post.CreatedAt.UTC())
	}
	var cat models.Category
	if err := db.Where("id = ?", *post.CategoryID).First(&cat).Error; err != nil || cat.Name != "技术笔记" {
		t.Fatalf("category not applied: %v / %v", err, cat.Name)
	}
	var links int64
	db.Model(&models.PostTag{}).Where("post_id = ?", post.ID).Count(&links)
	if links != 2 {
		t.Fatalf("expected 2 tag links, got %d", links)
	}
}

func TestImportMarkdownTitleFallbacks(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	res, err := svc.ImportMarkdown(ctx, root, []MarkdownFile{
		{Name: "heading.md", Body: []byte("# 标题来自正文\n\n内容")},
		{Name: "my-first-post.md", Body: []byte("没有 frontmatter 也没有标题")},
		{Name: "empty.md", Body: []byte("---\ntitle: 空\n---\n\n")},
		{Name: "note.txt", Body: []byte("x")},
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created != 2 {
		t.Fatalf("expected 2 created, got %d (%+v)", res.Created, res.Skipped)
	}
	if len(res.Skipped) != 2 {
		t.Fatalf("expected 2 skipped, got %+v", res.Skipped)
	}
	var h models.Post
	if err := db.Where("title = ?", "标题来自正文").First(&h).Error; err != nil {
		t.Fatalf("heading title not used: %v", err)
	}
	if h.Body[:1] == "#" {
		t.Fatalf("the leading heading line should be removed from the body")
	}
	var f models.Post
	if err := db.Where("title = ?", "my first post").First(&f).Error; err != nil {
		t.Fatalf("filename fallback failed: %v", err)
	}
}

func TestImportMarkdownAdminGoesToReviewAndReusesTaxonomy(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "boss", "boss@example.com", "admin")
	doc := []byte("---\ntitle: T\ncategory: 共享分类\ntags: [复用]\n---\n\nbody\n")

	if _, err := svc.ImportMarkdown(ctx, admin, []MarkdownFile{{Name: "a.md", Body: doc}}); err != nil {
		t.Fatalf("import 1: %v", err)
	}
	if _, err := svc.ImportMarkdown(ctx, admin, []MarkdownFile{{Name: "b.md", Body: doc}}); err != nil {
		t.Fatalf("import 2: %v", err)
	}

	var post models.Post
	if err := db.Where("title = ?", "T").First(&post).Error; err != nil {
		t.Fatalf("post missing: %v", err)
	}
	// admin import must respect the review queue
	if post.Status != models.PostPending {
		t.Fatalf("admin import should be pending, got %q", post.Status)
	}
	var cats int64
	db.Model(&models.Category{}).Where("name = ?", "共享分类").Count(&cats)
	if cats != 1 {
		t.Fatalf("category should be reused, found %d rows", cats)
	}
	var tags int64
	db.Model(&models.Tag{}).Where("name = ?", "复用").Count(&tags)
	if tags != 1 {
		t.Fatalf("tag should be reused, found %d rows", tags)
	}
}

func TestImportMarkdownValidation(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	root := makeUser(t, db, "admin", "admin@example.com", "root")

	if _, err := svc.ImportMarkdown(ctx, root, nil); StatusOf(err) != 400 {
		t.Fatalf("empty upload should be 400, got %v", err)
	}
	tooMany := make([]MarkdownFile, 201)
	for i := range tooMany {
		tooMany[i] = MarkdownFile{Name: "a.md", Body: []byte("x")}
	}
	if _, err := svc.ImportMarkdown(ctx, root, tooMany); StatusOf(err) != 400 {
		t.Fatalf("201 files should be 400, got %v", err)
	}
}
