package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// TestFeedRSSHandler verifies the public RSS feed: it is served as
// application/rss+xml, lists only published posts, escapes markup in titles,
// and never leaks a private post.
func TestFeedRSSHandler(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, services.RegisterInput{
		Username: "author", Email: "author@example.com", Password: "secret123",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Plain users are forced into the pending review queue under the
	// three-tier model; promote the author to root so the post below lands in
	// "published" and the public feed (published-only) has something to show.
	if err := db.Model(&models.User{}).Where("email = ?", "author@example.com").
		Update("role", models.RoleRoot).Error; err != nil {
		t.Fatalf("promote author to root: %v", err)
	}
	login, err := svc.Login(ctx, services.LoginInput{Email: "author@example.com", Password: "secret123"}, services.LoginMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	user := models.User{ID: login.User.ID, Role: models.RoleRoot}

	if _, err := svc.CreatePost(ctx, user, services.CreatePostInput{
		Title: "Hello <World>", Body: "# Hi\n\nThis is **bold** text.", Status: "published",
	}); err != nil {
		t.Fatalf("create published: %v", err)
	}
	if _, err := svc.CreatePost(ctx, user, services.CreatePostInput{
		Title: "Secret Draft", Body: "x", Status: "private",
	}); err != nil {
		t.Fatalf("create private: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/feed", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	FeedRSS(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/rss+xml") {
		t.Fatalf("unexpected content-type %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<rss") {
		t.Fatalf("feed missing <rss> envelope: %s", body)
	}
	if !strings.Contains(body, "Hello") {
		t.Fatalf("feed missing published post: %s", body)
	}
	if strings.Contains(body, "Secret Draft") {
		t.Fatalf("private post leaked into feed: %s", body)
	}
	if strings.Contains(body, "<World>") {
		t.Fatalf("title markup not escaped (XSS risk): %s", body)
	}
}

// TestSitemapHandler verifies the public sitemap lists the home/archive pages
// and every published post with an absolute /read/ link.
func TestSitemapHandler(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	if _, err := svc.Register(ctx, services.RegisterInput{
		Username: "author2", Email: "author2@example.com", Password: "secret123",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Promote the author to root so the published post reaches the sitemap
	// (a plain user's post is forced into the pending review queue).
	if err := db.Model(&models.User{}).Where("email = ?", "author2@example.com").
		Update("role", models.RoleRoot).Error; err != nil {
		t.Fatalf("promote author to root: %v", err)
	}
	login, err := svc.Login(ctx, services.LoginInput{Email: "author2@example.com", Password: "secret123"}, services.LoginMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.CreatePost(ctx, models.User{ID: login.User.ID, Role: models.RoleRoot}, services.CreatePostInput{
		Title: "Map Me", Body: "y", Status: "published",
	}); err != nil {
		t.Fatalf("create published: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	Sitemap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/xml") {
		t.Fatalf("unexpected content-type %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<urlset") {
		t.Fatalf("sitemap missing <urlset>: %s", body)
	}
	// Spec 29: canonical post links use the slug when the post has one.
	if !strings.Contains(body, "/p/map-me") {
		t.Fatalf("sitemap should advertise the slug permalink: %s", body)
	}
}

// Spec 29: a row created before slugs existed has an empty slug and must keep
// being advertised through the UUID route rather than disappearing.
func TestSitemapFallsBackToIDWithoutSlug(t *testing.T) {
	_, db := setup(t)
	author := models.User{Username: "legacy", Email: "legacy@example.com", Role: models.RoleUser}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("create author: %v", err)
	}
	legacy := models.Post{Title: "Old Post", Body: "b", Status: "published", AuthorID: author.ID, Slug: ""}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	Sitemap(rec, req)

	if body := rec.Body.String(); !strings.Contains(body, "/read/"+legacy.ID) {
		t.Fatalf("sitemap should fall back to /read/{id} without a slug: %s", body)
	}
}

// A blog that allows Chinese titles has Chinese slugs. The feed and sitemap
// must percent-encode them so the advertised URL is legal under RFC 3986 —
// a raw Chinese byte in <loc>/<link>/<guid> is rejected by Google and drops
// the item in several RSS readers.
func TestSitemapEncodesChineseSlug(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()
	if _, err := svc.Register(ctx, services.RegisterInput{
		Username: "zha", Email: "zha@example.com", Password: "secret123",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := db.Model(&models.User{}).Where("email = ?", "zha@example.com").
		Update("role", models.RoleRoot).Error; err != nil {
		t.Fatalf("promote: %v", err)
	}
	login, err := svc.Login(ctx, services.LoginInput{Email: "zha@example.com", Password: "secret123"}, services.LoginMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if _, err := svc.CreatePost(ctx, models.User{ID: login.User.ID, Role: models.RoleRoot}, services.CreatePostInput{
		Title: "前后端分离架构", Body: "x", Status: "published", Slug: "前后端分离架构",
	}); err != nil {
		t.Fatalf("create post: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "example.com"
	rec := httptest.NewRecorder()
	Sitemap(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "/p/前后端分离架构") {
		t.Fatalf("sitemap advertised a raw Chinese slug (illegal URL): %s", body)
	}
	// The percent-encoded form must appear inside a <loc>.
	if !strings.Contains(body, "/p/%E5%89%8D%E5%90%8E%E7%AB%AF%E5%88%86%E7%A6%BB%E6%9E%B6%E6%9E%84") {
		t.Fatalf("sitemap slug not percent-encoded as expected: %s", body)
	}

	// The RSS feed must encode identically.
	feedReq := httptest.NewRequest(http.MethodGet, "/feed", nil)
	feedReq.Host = "example.com"
	feedRec := httptest.NewRecorder()
	FeedRSS(feedRec, feedReq)
	fb := feedRec.Body.String()
	if strings.Contains(fb, "/p/前后端分离架构") {
		t.Fatalf("feed advertised a raw Chinese slug: %s", fb)
	}
	if !strings.Contains(fb, "/p/%E5%89%8D%E5%90%8E%E7%AB%AF%E5%88%86%E7%A6%BB%E6%9E%B6%E6%9E%84") {
		t.Fatalf("feed slug not percent-encoded as expected: %s", fb)
	}
}
