package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/config"
	"lopezpress/internal/handlers"
	"lopezpress/internal/middleware"
	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// These tests live in the router package on purpose. The R1-01 defect was not
// inside any handler — it was in the route table: four read endpoints were
// registered as authPublic, so they never reached WithOptionalAuth, the single
// place that consults the anonymous-read switch. Testing the handlers directly
// would have passed before the fix and after it, because the handlers were
// never the problem. Only walking the real router exercises what broke.

// buildTestRouter wires the full router over a throwaway database and returns
// it together with the service (for seeding) and a sign-in helper.
func buildTestRouter(t *testing.T) (http.Handler, *services.Service, *gorm.DB) {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath+"?_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := db.Where("id = ?", 1).First(&models.SiteConfig{}).Error; err != nil {
		_ = db.Create(&models.SiteConfig{
			ID: 1, SiteName: "Echo", SiteTitle: "Echo", ImgHostMode: "local",
			AllowRegister: true, PageSize: 10, PublicRead: true,
		}).Error
	}

	cfg := config.Load()
	cfg.JWTSecret = "router-test-secret"
	cfg.JWTExpiryHours = 1
	// Keep every write inside the temp dir: the default UploadDir/BackupDir
	// point at the repository's data/, and a test must not leave files there.
	cfg.UploadDir = filepath.Join(dir, "uploads")
	cfg.BackupDir = filepath.Join(dir, "backups")
	_ = os.MkdirAll(cfg.UploadDir, 0o755)
	_ = os.MkdirAll(cfg.BackupDir, 0o755)

	svc := services.New(db, cfg)
	handlers.Init(db, cfg, svc)

	h := New(cfg, middleware.NewLimiter(middleware.LimiterConfig{}), middleware.NewLoginLimiter())
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return h, svc, db
}

func seedPublishedPost(t *testing.T, db *gorm.DB, authorID, body string) models.Post {
	t.Helper()
	p := models.Post{
		ID:       "pub-" + body,
		AuthorID: authorID,
		Title:    "Published " + body,
		Body:     body,
		Status:   "published",
		Slug:     body, // the permalink endpoint resolves by slug, not id
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}
	return p
}

func seedUser(t *testing.T, db *gorm.DB, username, email, role string) models.User {
	t.Helper()
	u := models.User{Username: username, Email: email, Role: role}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return u
}

func tokenFor(t *testing.T, svc *services.Service, username, email string) string {
	t.Helper()
	if _, err := svc.Register(context.Background(), services.RegisterInput{
		Username: username, Email: email, Password: "secret123",
	}); err != nil {
		t.Fatalf("register %s: %v", username, err)
	}
	res, err := svc.Login(context.Background(),
		services.LoginInput{Email: username, Password: "secret123"}, services.LoginMeta{})
	if err != nil {
		t.Fatalf("login %s: %v", username, err)
	}
	return res.Token
}

// R1-01: with the site locked to signed-in readers, every endpoint that serves
// article content must refuse an anonymous caller. Before the fix, four of them
// answered with full bodies because they sat outside the one gate that checks.
func TestLockedSiteRefusesAnonymousOnEveryContentEndpoint(t *testing.T) {
	h, svc, db := buildTestRouter(t)
	ctx := context.Background()

	author := seedUser(t, db, "writer", "writer@example.com", models.RoleUser)
	post := seedPublishedPost(t, db, author.ID, "locked-body")
	reviewer := seedUser(t, db, "mod", "mod@example.com", models.RoleAdmin)
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).
		Update("reviewed_by", reviewer.ID).Error; err != nil {
		t.Fatalf("record reviewer: %v", err)
	}

	// Lock the site.
	if err := db.Model(&models.SiteConfig{}).Where("id = ?", 1).
		Update("public_read", false).Error; err != nil {
		t.Fatalf("lock site: %v", err)
	}

	paths := []struct{ name, path string }{
		{"post list", "/api/posts"},
		{"single post", "/api/posts/" + post.ID},
		{"post public", "/api/posts/" + post.ID + "/public"},
		{"rss feed", "/feed"},
		{"sitemap", "/sitemap.xml"},
		{"author profile", "/api/authors/writer"},
		{"categories", "/api/categories"},
	}
	for _, c := range paths {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: anonymous read on a locked site must be 401, got %d (body=%s)",
				c.name, rec.Code, rec.Body.String())
		}
	}

	// A signed-in reader still gets through: the switch gates ANONYMOUS
	// access, not access.
	token := tokenFor(t, svc, "reader", "reader@example.com")
	for _, c := range paths {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: signed-in read must succeed, got %d (body=%s)",
				c.name, rec.Code, rec.Body.String())
		}
	}

	// Sanity: the switch itself is what we flipped, not the whole app.
	var cfg models.SiteConfig
	if err := db.Where("id = ?", 1).First(&cfg).Error; err != nil || cfg.PublicRead {
		t.Fatalf("site should be locked, got public_read=%v err=%v", cfg.PublicRead, err)
	}
	_ = ctx
}

// R1-04 / R2-07: the reviewer's user id rode along in post payloads served to
// anonymous readers and to the post's own author. It is an admin account id,
// nothing in the client renders it, and it is a stable identifier for
// admin-targeted attacks.
func TestPublicPayloadsHideReviewerIdentity(t *testing.T) {
	h, _, db := buildTestRouter(t)

	author := seedUser(t, db, "writer2", "writer2@example.com", models.RoleUser)
	post := seedPublishedPost(t, db, author.ID, "review-body")
	reviewer := seedUser(t, db, "mod2", "mod2@example.com", models.RoleAdmin)
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).
		Updates(map[string]any{"reviewed_by": reviewer.ID, "review_note": "ok"}).Error; err != nil {
		t.Fatalf("record review: %v", err)
	}

	paths := []string{
		"/api/posts/" + post.ID + "/public",
		"/api/posts/" + post.ID,
		"/api/permalinks/review-body",
		"/api/authors/writer2",
	}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d (body=%s)", p, rec.Code, rec.Body.String())
		}
		if body := rec.Body.String(); containsID(body, reviewer.ID) {
			t.Errorf("%s: payload must not contain the reviewer id %q", p, reviewer.ID)
		}
	}

	// The list endpoint too — it is the one that serialises a page of posts.
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", rec.Code)
	}
	// The endpoint answers either the paged shape ({items:[...]}) or the
	// legacy bare array, so accept both rather than assuming one.
	var items []map[string]any
	var paged struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &paged); err == nil && len(paged.Items) > 0 {
		items = paged.Items
	} else if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, item := range items {
		if v, ok := item["reviewed_by"]; ok && v != "" {
			t.Errorf("list item must not carry reviewed_by, got %v", v)
		}
	}

	// The database still knows who reviewed it — this is an outbound rule.
	var stored models.Post
	if err := db.Where("id = ?", post.ID).First(&stored).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.ReviewedBy != reviewer.ID {
		t.Errorf("stored reviewer must be preserved for the audit trail, got %q", stored.ReviewedBy)
	}
}

// containsID reports whether the JSON blob carries the id as a value. Compared
// as a quoted string so a coincidental substring inside another field cannot
// produce a false positive.
func containsID(body, id string) bool {
	return id != "" && (jsonHasStringValue(body, id))
}

func jsonHasStringValue(body, want string) bool {
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		// Non-JSON (RSS/sitemap XML): fall back to a plain substring check,
		// which is what the attacker would use anyway.
		return indexOf(body, want) >= 0
	}
	return walkForValue(v, want)
}

func walkForValue(v any, want string) bool {
	switch t := v.(type) {
	case map[string]any:
		for _, val := range t {
			if walkForValue(val, want) {
				return true
			}
		}
	case []any:
		for _, val := range t {
			if walkForValue(val, want) {
				return true
			}
		}
	case string:
		return t == want
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
