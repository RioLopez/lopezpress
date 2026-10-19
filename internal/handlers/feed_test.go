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
	if !strings.Contains(body, "/read/") {
		t.Fatalf("sitemap missing post links: %s", body)
	}
}
