package services

// View de-duplication + tag rename.

import (
	"context"
	"testing"
	"time"

	"lopezpress/internal/models"
)

func viewMeta(ip, ua string, now time.Time) ViewMeta {
	return ViewMeta{IP: ip, UA: ua, Now: now}
}

// AC-1.1 / AC-1.4: same visitor + post + day counts once; the next day counts
// again.
func TestReportViewDedupesPerVisitorPerDay(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	post := makePostNoTime(t, db, alice, "published", "body", "Hello")

	day1 := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Firefox/131.0"

	// First read of the day -> counted.
	res, err := svc.ReportView(ctx, anonymous(), post.ID, viewMeta("1.2.3.4", ua, day1))
	if err != nil {
		t.Fatalf("first report: %v", err)
	}
	if !res.Counted || res.ViewCount != 1 {
		t.Fatalf("first read should count to 1, got %+v", res)
	}

	// Same visitor, same day, later hour -> duplicate, count unchanged.
	later := day1.Add(3 * time.Hour)
	res, err = svc.ReportView(ctx, anonymous(), post.ID, viewMeta("1.2.3.4", ua, later))
	if err != nil {
		t.Fatalf("duplicate report: %v", err)
	}
	if res.Counted || res.Reason != "duplicate" || res.ViewCount != 1 {
		t.Fatalf("duplicate read must not count, got %+v", res)
	}

	// Different visitor -> counts.
	res, err = svc.ReportView(ctx, anonymous(), post.ID, viewMeta("5.6.7.8", ua, later))
	if err != nil {
		t.Fatalf("second visitor: %v", err)
	}
	if !res.Counted || res.ViewCount != 2 {
		t.Fatalf("a different visitor should count to 2, got %+v", res)
	}

	// Next day (UTC) -> counts again.
	day2 := day1.AddDate(0, 0, 1)
	res, err = svc.ReportView(ctx, anonymous(), post.ID, viewMeta("1.2.3.4", ua, day2))
	if err != nil {
		t.Fatalf("next-day report: %v", err)
	}
	if !res.Counted || res.ViewCount != 3 {
		t.Fatalf("next day should count to 3, got %+v", res)
	}
}

// AC-1.2: the author reading their own post is not traffic.
func TestReportViewSkipsAuthor(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	post := makePostNoTime(t, db, alice, "published", "body", "Mine")

	res, err := svc.ReportView(ctx, alice, post.ID, viewMeta("1.2.3.4", "Mozilla/5.0 Firefox/131", time.Now()))
	if err != nil {
		t.Fatalf("author report: %v", err)
	}
	if res.Counted || res.Reason != "author" {
		t.Fatalf("author read must not count, got %+v", res)
	}

	var stored models.Post
	if err := db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload post: %v", err)
	}
	if stored.ViewCount != 0 {
		t.Fatalf("view_count must stay 0, got %d", stored.ViewCount)
	}
}

// AC-1.3: crawler / CLI user agents are not people.
func TestReportViewSkipsBots(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	post := makePostNoTime(t, db, alice, "published", "body", "Bot bait")

	for _, ua := range []string{
		"curl/8.4.0",
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"python-requests/2.31.0",
		"Go-http-client/1.1",
	} {
		res, err := svc.ReportView(ctx, anonymous(), post.ID, viewMeta("9.9.9.9", ua, time.Now()))
		if err != nil {
			t.Fatalf("bot report (%s): %v", ua, err)
		}
		if res.Counted || res.Reason != "bot" {
			t.Fatalf("UA %q must not count, got %+v", ua, res)
		}
	}
}

// Only published posts are countable; restricted statuses answer 404 so the
// endpoint cannot be used to probe for private/pending/banned content.
func TestReportViewRejectsNonPublished(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	for _, status := range []string{"private", "pending", "rejected", "banned"} {
		p := makePostNoTime(t, db, alice, status, "body", status+" post")
		if _, err := svc.ReportView(ctx, anonymous(), p.ID, viewMeta("1.2.3.4", "Mozilla/5.0", time.Now())); err == nil {
			t.Fatalf("status %q must not be countable", status)
		} else if StatusOf(err) != 404 {
			t.Fatalf("status %q should 404, got %d", status, StatusOf(err))
		}
	}
	if _, err := svc.ReportView(ctx, anonymous(), "does-not-exist", viewMeta("1.2.3.4", "Mozilla/5.0", time.Now())); StatusOf(err) != 404 {
		t.Fatalf("missing post should 404, got %v", err)
	}
}

// The page size comes from site config and is clamped to the offered
// set.
func TestListPageSizeFromConfig(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	if got := svc.ListPageSize(ctx); got != DefaultPageSize {
		t.Fatalf("default page size should be %d, got %d", DefaultPageSize, got)
	}

	if err := db.Model(&models.SiteConfig{}).Where("id = 1").Update("page_size", 30).Error; err != nil {
		t.Fatalf("update page size: %v", err)
	}
	if got := svc.ListPageSize(ctx); got != 30 {
		t.Fatalf("page size should follow config (30), got %d", got)
	}

	// A hand-edited / out-of-set value falls back to the default rather than
	// producing a huge page on a low-spec box.
	for _, n := range []int{0, -5, 7, 1000} {
		if got := ClampPageSize(n); got != DefaultPageSize {
			t.Fatalf("ClampPageSize(%d) should be %d, got %d", n, DefaultPageSize, got)
		}
	}
	for _, n := range allowedPageSizes {
		if got := ClampPageSize(n); got != n {
			t.Fatalf("ClampPageSize(%d) should be %d, got %d", n, n, got)
		}
	}
}

// When the caller omits size, ListPostsPage uses the configured size.
func TestListPostsPageUsesConfiguredSize(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	for i := 0; i < 7; i++ {
		makePostNoTime(t, db, alice, "published", "body", "post "+string(rune('A'+i)))
	}
	if err := db.Model(&models.SiteConfig{}).Where("id = 1").Update("page_size", 5).Error; err != nil {
		t.Fatalf("update page size: %v", err)
	}

	page, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{Page: 1})
	if err != nil {
		t.Fatalf("list page: %v", err)
	}
	if page.PageSize != 5 {
		t.Fatalf("PageSize should follow config (5), got %d", page.PageSize)
	}
	if len(page.Items) != 5 || page.Total != 7 {
		t.Fatalf("expected 5 of 7 items, got %d of %d", len(page.Items), page.Total)
	}

	// Page 2 holds the remaining two and does not repeat page 1.
	page2, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{Page: 2})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2.Items) != 2 {
		t.Fatalf("page 2 should hold 2 items, got %d", len(page2.Items))
	}
	seen := map[string]bool{}
	for _, p := range page2.Items {
		seen[p.ID] = true
	}
	for _, p := range page.Items {
		if seen[p.ID] {
			t.Fatalf("post %s appears on both pages", p.ID)
		}
	}

	// An out-of-range page returns an empty slice with the real total.
	page9, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{Page: 9})
	if err != nil {
		t.Fatalf("out-of-range page: %v", err)
	}
	if len(page9.Items) != 0 || page9.Total != 7 {
		t.Fatalf("out-of-range page should be empty with total 7, got %d items / total %d", len(page9.Items), page9.Total)
	}
}

// Renaming a tag keeps every post association.
func TestUpdateTagKeepsAssociations(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	alice := makeUser(t, db, "alice", "alice@example.com", "user")
	tag := models.Tag{Name: "golang"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	for i := 0; i < 3; i++ {
		p := makePostNoTime(t, db, alice, "published", "body", "tagged "+string(rune('A'+i)))
		if err := db.Create(&models.PostTag{PostID: p.ID, TagID: tag.ID}).Error; err != nil {
			t.Fatalf("link tag: %v", err)
		}
	}

	updated, err := svc.UpdateTag(ctx, tag.ID, UpdateTagInput{Name: "  Go  "})
	if err != nil {
		t.Fatalf("rename tag: %v", err)
	}
	if updated.Name != "Go" {
		t.Fatalf("expected trimmed name %q, got %q", "Go", updated.Name)
	}

	var links int64
	if err := db.Model(&models.PostTag{}).Where("tag_id = ?", tag.ID).Count(&links).Error; err != nil {
		t.Fatalf("count links: %v", err)
	}
	if links != 3 {
		t.Fatalf("rename must not touch associations: expected 3, got %d", links)
	}
}

func TestUpdateTagValidation(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	first := models.Tag{Name: "go"}
	second := models.Tag{Name: "rust"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create tag: %v", err)
	}

	// Empty / whitespace-only name -> 400.
	if _, err := svc.UpdateTag(ctx, first.ID, UpdateTagInput{Name: "   "}); StatusOf(err) != 400 {
		t.Fatalf("blank name should be 400, got %v", err)
	}
	// Duplicate of another tag -> 409.
	if _, err := svc.UpdateTag(ctx, first.ID, UpdateTagInput{Name: "rust"}); StatusOf(err) != 409 {
		t.Fatalf("duplicate name should be 409, got %v", err)
	}
	// Missing tag -> 404.
	if _, err := svc.UpdateTag(ctx, 99999, UpdateTagInput{Name: "whatever"}); StatusOf(err) != 404 {
		t.Fatalf("missing tag should be 404, got %v", err)
	}
	// Pure case change of itself -> allowed (dup check excludes self).
	if _, err := svc.UpdateTag(ctx, first.ID, UpdateTagInput{Name: "Go"}); err != nil {
		t.Fatalf("case-only rename should succeed, got %v", err)
	}
}
