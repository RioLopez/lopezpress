package services

import (
	"context"
	"net/http"
	"strings"
	"unicode/utf8"
	"sync"
	"testing"

	"lopezpress/internal/models"
)

// R3-07: unbounded content fields. CreatePost only checked "not empty", so one
// pending post with a 1 MB title and a 5 MB body produced a 6.29 MB create
// response, a 9.30 MB admin list, an 18.88 MB revision history and a 4.20 MB
// audit page — and grew the database by 27 MB. Nothing rejected any of it.
func TestCreatePostRejectsOversizedContent(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	u := makeUser(t, db, "bigwriter", "big@example.com", models.RoleRoot)

	// Title past the ceiling.
	long := strings.Repeat("T", MaxTitleLen+1)
	if _, err := svc.CreatePost(ctx, u, CreatePostInput{Title: long, Body: "fine"}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long title: expected 400, got %v (err=%v)", StatusOf(err), err)
	}

	// Body past the ceiling (256 KiB).
	huge := strings.Repeat("x", MaxBodyLen+1)
	if _, err := svc.CreatePost(ctx, u, CreatePostInput{Title: "ok", Body: huge}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long body: expected 400, got %v (err=%v)", StatusOf(err), err)
	}

	// Exactly at the ceiling is fine.
	atLimit := strings.Repeat("y", MaxTitleLen)
	if _, err := svc.CreatePost(ctx, u, CreatePostInput{Title: atLimit, Body: "fine"}); err != nil {
		t.Fatalf("a title at the limit must be accepted: %v", err)
	}
}

// The same ceilings apply on edit — and, importantly, an over-long write must
// not consume a revision slot. Every rejected attempt leaving a snapshot behind
// would slowly evict the real history (only 20 revisions survive per post).
func TestUpdatePostRejectsOversizedContent(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	u := makeUser(t, db, "editor1", "editor1@example.com", models.RoleRoot)

	post, err := svc.CreatePost(ctx, u, CreatePostInput{Title: "Original", Body: "original body"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	huge := strings.Repeat("z", MaxBodyLen+1)
	if _, err := svc.UpdatePost(ctx, u, post.ID, UpdatePostInput{Body: huge}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long body on edit: expected 400, got %v (err=%v)", StatusOf(err), err)
	}

	// No revision was written for the refused attempt.
	var revs int64
	db.Model(&models.PostRevision{}).Where("post_id = ?", post.ID).Count(&revs)
	if revs != 0 {
		t.Fatalf("a refused edit must not snapshot a revision, got %d", revs)
	}

	// The stored body is untouched.
	var stored models.Post
	if err := db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.Body != "original body" {
		t.Fatalf("refused edit must not modify the post, got %q", stored.Body)
	}
}

// R6-07 / R3-07 (audit half): the detail string embeds the post title, and
// action_log.detail declares size:512 — which SQLite does not enforce. A 1 MB
// title therefore produced a 1 MB audit row, and the admin audit page (which
// reads 200 rows at a time) reached 4.26 MB. The detail is now clamped.
func TestAuditDetailIsClamped(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	u := makeUser(t, db, "audited", "audited@example.com", models.RoleRoot)

	long := strings.Repeat("A", MaxTitleLen)
	if _, err := svc.CreatePost(ctx, u, CreatePostInput{Title: long, Body: "body"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	var rows []models.ActionLog
	if err := db.Where("action = ?", "create_post").Find(&rows).Error; err != nil {
		t.Fatalf("load audit: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected a create_post audit row")
	}
	for _, r := range rows {
		if len(r.Detail) > MaxAuditDetailLen+len("…[truncated]") {
			t.Fatalf("audit detail must be clamped, got %d bytes", len(r.Detail))
		}
		if !strings.HasSuffix(r.Detail, "…[truncated]") {
			t.Fatalf("a clamped detail must say so, got %q", r.Detail[len(r.Detail)-40:])
		}
	}
}

// R4-10: the Markdown importer had no name ceiling at all, while the tag
// endpoint did. A ~1 MB category name imported fine and then the ANONYMOUS
// /api/categories endpoint served 1.05 MB to every visitor.
func TestImportTaxonomyNamesAreBounded(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	tooLong := strings.Repeat("c", MaxCategoryNameLen+1)
	if _, err := svc.findOrCreateCategoryByName(ctx, tooLong); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long category name: expected 400, got %v (err=%v)", StatusOf(err), err)
	}

	// A tag name over the ceiling is skipped rather than failing the import:
	// one bad tag in a file should not cost the reader the whole file.
	svc.attachTagsByName(ctx, "no-such-post", []string{strings.Repeat("t", MaxTagNameLen+1), "goodtag"})
	var n int64
	db.Model(&models.Tag{}).Where("name = ?", "goodtag").Count(&n)
	if n != 1 {
		t.Fatalf("a valid tag alongside an over-long one must still be created, got %d rows", n)
	}
	var bad int64
	db.Model(&models.Tag{}).Where("length(name) > ?", MaxTagNameLen).Count(&bad)
	if bad != 0 {
		t.Fatalf("no tag may exceed the ceiling, found %d", bad)
	}
}

// The clamp must not corrupt multi-byte content: a byte-level cut lands inside
// a multi-byte rune and leaves an invalid UTF-8 fragment in the audit row,
// which then renders as a replacement character for every admin who reads it.
// This test caught exactly that in the first version of clampDetail.
func TestClampDetailKeepsValidUTF8(t *testing.T) {
	long := strings.Repeat("中", MaxAuditDetailLen) // 3 bytes per rune
	got := clampDetail(long)
	for _, r := range got {
		if r == '\uFFFD' {
			t.Fatal("clamped detail contains a replacement character (cut landed mid-rune)")
		}
	}
	if !utf8.ValidString(got) {
		t.Fatal("clamped detail is not valid UTF-8")
	}
	if !strings.HasSuffix(got, "…[truncated]") {
		t.Fatalf("clamped detail must say so, got %q", got[max(0, len(got)-40):])
	}
}

// R6-02: two concurrent creates of the same title both saw "slug is free" and
// both inserted, because the uniqueness check was a COUNT and not a lock —
// twenty concurrent creates produced four posts sharing one permanent link.
// Migration 0014 puts a UNIQUE index on posts.slug, so the loser's INSERT now
// fails and CreatePost retries with a suffix.
func TestConcurrentCreatesGetDistinctSlugs(t *testing.T) {
	// A file-backed DB: :memory: gives each pooled connection its own store, so
	// concurrent racers would not even share the index.
	svc, db := newFileTestService(t)

	u := makeUser(t, db, "slugracer", "slugracer@example.com", models.RoleRoot)

	const racers = 6
	slugs := make([]string, racers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			p, err := svc.CreatePost(context.Background(), u, CreatePostInput{
				Title: "Same Title", Body: "body " + string(rune('a'+i)), Status: models.PostPublished,
			})
			if err != nil {
				t.Errorf("racer %d: %v", i, err)
				return
			}
			slugs[i] = p.Slug
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[string]bool{}
	for i, s := range slugs {
		if s == "" {
			t.Fatalf("racer %d produced no slug", i)
		}
		if seen[s] {
			t.Fatalf("slug %q was handed to more than one post — the unique index did not hold", s)
		}
		seen[s] = true
	}
	// And the database agrees.
	var rows []models.Post
	if err := db.Where("author_id = ?", u.ID).Find(&rows).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(rows) != racers {
		t.Fatalf("expected %d posts, got %d", racers, len(rows))
	}
	dbSlugs := map[string]bool{}
	for _, r := range rows {
		if dbSlugs[r.Slug] {
			t.Fatalf("duplicate slug %q in the database", r.Slug)
		}
		dbSlugs[r.Slug] = true
	}
}

// R6-06: (page-1)*pageSize overflows int64 for a large enough page, goes
// negative, and SQLite reads a negative OFFSET as 0 — so an absurd page number
// answered with the FIRST page's data while echoing the impossible page back.
func TestPageNumberOverflowIsRefused(t *testing.T) {
	svc, db := newTestService(t)
	u := makeUser(t, db, "pager", "pager@example.com", models.RoleRoot)
	for i := 0; i < 3; i++ {
		if _, err := svc.CreatePost(context.Background(), u, CreatePostInput{
			Title: "P" + string(rune('0'+i)), Body: "b", Status: models.PostPublished,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// A page number whose (page-1)*10 overflows int64.
	if _, err := svc.ListPostsPage(context.Background(), u, ListPostsQuery{
		Page: 922337203685477580, PageSize: 10,
	}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("an overflowing page number must be refused with 400, got %v (err=%v)", StatusOf(err), err)
	}

	// A merely large-but-in-range page is fine and returns an empty page.
	page, err := svc.ListPostsPage(context.Background(), u, ListPostsQuery{Page: maxPageNumber, PageSize: 10})
	if err != nil {
		t.Fatalf("a page within the ceiling must work: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("a page past the end must be empty, got %d items", len(page.Items))
	}
}

// R1-05: the operator could switch email verification ON with no SMTP host
// configured. The save returned 200 and the feature then failed at runtime on
// every request (503 "SMTP is not configured"), which with registration open
// took the whole sign-up flow down. The two settings only make sense together.
func TestEmailVerifyRequiresSmtpHost(t *testing.T) {
	_, db := newTestService(t)
	var cfg models.SiteConfig
	if err := db.Where("id = ?", 1).First(&cfg).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.EmailVerifyEnabled = true
	cfg.SmtpHost = ""
	if err := validateSiteConfig(cfg); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("enabling verification without SMTP must be refused, got %v (err=%v)", StatusOf(err), err)
	}
	// With a host configured the same config is accepted.
	cfg.SmtpHost = "smtp.example.com"
	if err := validateSiteConfig(cfg); err != nil {
		t.Fatalf("a configured SMTP host must be accepted: %v", err)
	}
	// And leaving verification off with no host is still fine.
	cfg.EmailVerifyEnabled = false
	cfg.SmtpHost = ""
	if err := validateSiteConfig(cfg); err != nil {
		t.Fatalf("verification off with no SMTP must be accepted: %v", err)
	}
}

// R6-09: a friend-link label had no length limit, and the site-config response
// is public — so one long name inflated the payload every anonymous reader
// downloaded (measured ~5 KB of footer text from a single 5000-character label).
func TestFriendLinkNameIsBounded(t *testing.T) {
	long := strings.Repeat("N", maxFriendLinkNameLen+1)
	links := `[{"name":"` + long + `","url":"https://example.com/"}]`
	cfg := models.SiteConfig{SiteName: "Echo", ImgHostMode: "local", FriendLinks: links}
	if err := validateSiteConfig(cfg); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("an over-long link name must be refused, got %v (err=%v)", StatusOf(err), err)
	}

	// A name at the limit, and a normal one, are fine.
	ok := `[{"name":"` + strings.Repeat("N", maxFriendLinkNameLen) + `","url":"https://example.com/"}]`
	cfg.FriendLinks = ok
	if err := validateSiteConfig(cfg); err != nil {
		t.Fatalf("a name at the limit must be accepted: %v", err)
	}
}

// --- second sweep: the fields the first pass did not look at ---------------
//
// R3-07's report named the title and the body, so those were the two that got
// ceilings. Auditing every user-writable string afterwards found five more with
// none at all, all of them the same bug: a value that ends up in a response
// other people download.

// A plain author could attach a megabyte-long tag through POST /api/posts. The
// tag was created, and from then on the ANONYMOUS /api/tags endpoint served that
// megabyte to every visitor — one request, no privileges required. Fixing only
// the importer (which is how R4-10 was reported) left this path open.
func TestTagsHaveACeilingOnEveryPath(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "tagger", "tagger@example.com", models.RoleUser)

	tooLong := strings.Repeat("t", MaxTagNameLen+1)
	if _, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Tags", Body: "b", Tags: []string{tooLong, "ok-tag"},
	}); err != nil {
		t.Fatalf("an over-long tag must be skipped, not fail the save: %v", err)
	}
	var bad int64
	db.Model(&models.Tag{}).Where("length(name) > ?", MaxTagNameLen).Count(&bad)
	if bad != 0 {
		t.Fatalf("%d tag(s) exceeded the ceiling", bad)
	}
	var good int64
	db.Model(&models.Tag{}).Where("name = ?", "ok-tag").Count(&good)
	if good != 1 {
		t.Fatal("the valid tag alongside it must still be created")
	}

	// And the count of tags per post is bounded too: the per-name ceiling is
	// useless against ten thousand legal names.
	many := make([]string, MaxTagsPerPost+1)
	for i := range many {
		many[i] = "tag" + string(rune('a'+i%26)) + itoa(i)
	}
	if _, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Too many tags", Body: "b", Tags: many,
	}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("more than %d tags must be refused, got %v (err=%v)", MaxTagsPerPost, StatusOf(err), err)
	}
}

// The category CRUD endpoints had no ceiling, only the importer did. Category
// names are served by the anonymous /api/categories endpoint.
func TestCategoryNamesHaveACeiling(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	long := strings.Repeat("c", MaxCategoryNameLen+1)

	if _, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: long}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("CreateCategory with an over-long name: expected 400, got %v (err=%v)", StatusOf(err), err)
	}
	cat, err := svc.CreateCategory(ctx, CreateCategoryInput{Name: "Fine"})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := svc.UpdateCategory(ctx, cat.ID, UpdateCategoryInput{Name: long}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("UpdateCategory with an over-long name: expected 400, got %v (err=%v)", StatusOf(err), err)
	}
	// A non-ASCII name at the ceiling is legal (the old byte-based test in
	// UpdateTag refused these).
	cjk := strings.Repeat("中", MaxTagNameLen)
	tag := models.Tag{Name: "seed"}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := svc.UpdateTag(ctx, tag.ID, UpdateTagInput{Name: cjk}); err != nil {
		t.Fatalf("a CJK name at the rune ceiling must be accepted: %v", err)
	}
}

// The reviewer's note is returned to the author inside the post payload, so an
// admin could hand a third party a megabyte of response with one reject.
func TestReviewNoteIsBounded(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "noted", "noted@example.com", models.RoleUser)
	admin := makeUser(t, db, "noter", "noter@example.com", models.RoleAdmin)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "T", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	huge := strings.Repeat("n", MaxReviewNoteLen+1)
	if _, err := svc.ReviewPost(ctx, admin, post.ID,
		ReviewPostInput{Action: "reject", Note: huge}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long reject note: expected 400, got %v (err=%v)", StatusOf(err), err)
	}
	// A normal note still works.
	if _, err := svc.ReviewPost(ctx, admin, post.ID,
		ReviewPostInput{Action: "reject", Note: "please fix the intro"}); err != nil {
		t.Fatalf("a normal note must be accepted: %v", err)
	}
}

// A cover URL is rendered into <img src> by the front end and consumed by RSS
// and the sitemap. It had no validation at all.
func TestCoverURLIsValidated(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "covered", "covered@example.com", models.RoleUser)

	for _, bad := range []string{
		"javascript:alert(1)",
		"data:image/svg+xml,<svg onload=alert(1)>",
		"//evil.example/x.png",
		"vbscript:msgbox",
		strings.Repeat("x", MaxCoverURLLen+1),
	} {
		if _, err := svc.CreatePost(ctx, author, CreatePostInput{
			Title: "Cover", Body: "b", Cover: bad,
		}); StatusOf(err) != http.StatusBadRequest {
			t.Fatalf("cover %q must be refused, got %v (err=%v)", bad[:min(len(bad), 30)], StatusOf(err), err)
		}
	}
	// The two accepted shapes.
	for _, good := range []string{"", "https://example.com/a.png", "/uploads/abc.png"} {
		if _, err := svc.CreatePost(ctx, author, CreatePostInput{
			Title: "Cover ok " + good, Body: "b", Cover: good,
		}); err != nil {
			t.Fatalf("cover %q must be accepted: %v", good, err)
		}
	}
}

// DisplayName and Bio appear in the author byline of every post that user
// wrote, so an unbounded one multiplies across the whole site.
func TestProfileTextIsBounded(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	u := makeUser(t, db, "profile", "profile@example.com", models.RoleUser)

	huge := strings.Repeat("b", MaxBioLen+1)
	if _, err := svc.UpdateMe(ctx, u.ID, UpdateMeInput{Bio: &huge}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long bio: expected 400, got %v (err=%v)", StatusOf(err), err)
	}
	longName := strings.Repeat("d", MaxDisplayNameLen+1)
	if _, err := svc.UpdateMe(ctx, u.ID, UpdateMeInput{DisplayName: &longName}); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("over-long display name: expected 400, got %v (err=%v)", StatusOf(err), err)
	}
	okBio := "I write about Go."
	if _, err := svc.UpdateMe(ctx, u.ID, UpdateMeInput{Bio: &okBio}); err != nil {
		t.Fatalf("a normal bio must be accepted: %v", err)
	}
}

// A reviewer could rewrite ANOTHER author's permanent link. The first version of
// the R5-05 guard listed title/body/cover and forgot slug — a URL is as much the
// author's as the text is, and re-pointing it silently breaks every reference
// that already exists.
func TestAdminCannotChangeAnotherAuthorsSlug(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "slugowner", "slugowner@example.com", models.RoleUser)
	admin := makeUser(t, db, "slugadmin", "slugadmin@example.com", models.RoleAdmin)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Stable Link", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	before := post.Slug
	if _, err := svc.UpdatePost(ctx, admin, post.ID,
		UpdatePostInput{Slug: "taken-over"}); StatusOf(err) != http.StatusForbidden {
		t.Fatalf("admin changing another author's slug: expected 403, got %v (err=%v)", StatusOf(err), err)
	}
	var row models.Post
	db.Where("id = ?", post.ID).First(&row)
	if row.Slug != before {
		t.Fatalf("slug changed anyway: %q -> %q", before, row.Slug)
	}
	// Root may still fix it, and the author may rename their own.
	root := makeUser(t, db, "slugroot", "slugroot@example.com", models.RoleRoot)
	if _, err := svc.UpdatePost(ctx, root, post.ID, UpdatePostInput{Slug: "root-fixed"}); err != nil {
		t.Fatalf("root must be able to change a slug: %v", err)
	}
}
