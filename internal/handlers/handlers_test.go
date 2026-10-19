package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"echo-content-api/internal/config"
	"echo-content-api/internal/middleware"
	"echo-content-api/internal/models"
	"echo-content-api/internal/services"
)

// setup wires a real *Service backed by a temp-file SQLite DB into the
// handlers package globals (Init) and returns a login helper. Each test
// gets an isolated DB.
func setup(t *testing.T) (*services.Service, *gorm.DB) {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "hdl-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	f.Close()

	db, err := gorm.Open(sqlite.Open(f.Name()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Post{}, &models.Category{}, &models.Tag{}, &models.PostTag{}, &models.SiteConfig{}, &models.LoginLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Tests rely on open registration; enable it for the test DB.
	if err := db.Where("id = ?", 1).First(&models.SiteConfig{}).Error; err != nil {
		_ = db.Create(&models.SiteConfig{ID: 1, SiteName: "Echo", SiteTitle: "Echo", ImgHostMode: "local", AllowRegister: true}).Error
	}

	cfg := config.Load()
	cfg.JWTSecret = "test-secret"
	cfg.JWTExpiryHours = 1

	s := services.New(db, cfg)
	Init(db, cfg, s)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return s, db
}

// mustRegisterAndLogin creates a user via the service and returns a valid
// Bearer token for it (Register + Login).
func mustToken(t *testing.T, svc *services.Service, username, email string) string {
	t.Helper()
	if _, err := svc.Register(context.Background(), services.RegisterInput{
		Username: username, Email: email, Password: "secret123",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	res, err := svc.Login(context.Background(), services.LoginInput{Email: username, Password: "secret123"}, services.LoginMeta{})
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	return res.Token
}

func TestRegisterHandler(t *testing.T) {
	svc, _ := setup(t)
	_ = svc

	body := `{"username":"newuser","email":"new@example.com","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(body))
	rec := httptest.NewRecorder()
	Register(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestRegisterHandlerInvalidBody(t *testing.T) {
	svc, _ := setup(t)
	_ = svc

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader("{not-json"))
	rec := httptest.NewRecorder()
	Register(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestLoginHandler(t *testing.T) {
	svc, _ := setup(t)
	// Seed a user.
	if _, err := svc.Register(context.Background(), services.RegisterInput{
		Username: "alice", Email: "alice@example.com", Password: "secret123",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	body := `{"email":"alice","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	rec := httptest.NewRecorder()
	Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	var out services.AuthResult
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Token == "" {
		t.Fatalf("expected a token in the response")
	}
}

func TestMeHandlerRequiresAuth(t *testing.T) {
	svc, _ := setup(t)
	_ = svc

	// No Authorization header -> WithAuth rejects with 401.
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	WithAuth(Me)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d", rec.Code)
	}
}

func TestMeHandlerWithAuth(t *testing.T) {
	svc, _ := setup(t)
	tok := mustToken(t, svc, "alice", "alice@example.com")

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	WithAuth(Me)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	var u services.UserPublic
	if err := json.NewDecoder(rec.Body).Decode(&u); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if u.Username != "alice" {
		t.Fatalf("expected username alice, got %q", u.Username)
	}
}

func TestMeHandlerBadToken(t *testing.T) {
	svc, _ := setup(t)
	_ = svc

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()
	WithAuth(Me)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for bad token, got %d", rec.Code)
	}
}

func TestCRUDHandlersRoundTrip(t *testing.T) {
	svc, _ := setup(t)
	tok := mustToken(t, svc, "alice", "alice@example.com")

	// CREATE
	createBody := `{"title":"Hello","body":"World"}`
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewBufferString(createBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	WithAuth(CreatePost)(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	postID, _ := created["id"].(string)
	if postID == "" {
		t.Fatalf("no id in created post: %v", created)
	}

	// LIST — router registers withOptionalAuth(ListPosts); mirror that here
	// so the test exercises the same wrapper as production (P2-10).
	req = httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	WithOptionalAuth(ListPosts)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var list []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 post in list, got %d", len(list))
	}

	// GET
	req = httptest.NewRequest(http.MethodGet, "/api/posts/"+postID, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	// Provide PathValue via a ServeMux-matched request is complex; instead use
	// a mux with a wildcard so PathValue("id") resolves.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		WithAuth(GetPost)(w, r)
	})
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	// UPDATE
	updateBody := `{"title":"Edited"}`
	req = httptest.NewRequest(http.MethodPut, "/api/posts/"+postID, bytes.NewBufferString(updateBody))
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.HandleFunc("PUT /api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		WithAuth(UpdatePost)(w, r)
	})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update: expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	// DELETE
	req = httptest.NewRequest(http.MethodDelete, "/api/posts/"+postID, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	mux.HandleFunc("DELETE /api/posts/{id}", func(w http.ResponseWriter, r *http.Request) {
		WithAuth(DeletePost)(w, r)
	})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d (%s)", rr.Code, rr.Body.String())
	}
}

// TestMiddlewareLoggerDoesNotPanic verifies Logger + Recover chain through
// a real handler without crashing.
func TestRecoverAndLoggerChain(t *testing.T) {
	svc, _ := setup(t)
	_ = svc

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := middleware.Chain(inner, middleware.Recover, middleware.Logger)
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// TestOptionalAuthDowngradesMustChangePassword locks in P1-5: a user still
// flagged must_change_password hitting an optional-auth read endpoint is
// downgraded to an anonymous user — they may only see public data, never
// their own private posts. Mirrors WithAuth's first-boot lock so a
// pre-change-password token can't keep browsing personal data.
func TestOptionalAuthDowngradesMustChangePassword(t *testing.T) {
	svc, db := setup(t)
	tok := mustToken(t, svc, "alice", "alice@example.com")

	// Flag the user as needing a password change, as the first-boot seed does.
	if err := db.Model(&models.User{}).Where("email = ?", "alice@example.com").
		Update("must_change_password", true).Error; err != nil {
		t.Fatalf("flag must_change_password: %v", err)
	}

	// Optional-auth LIST with the flagged token: must succeed (200) but the
	// caller is downgraded to anonymous, so they must NOT see alice's own
	// private posts. ListPosts writes a bare array (posts.Items).
	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	WithOptionalAuth(ListPosts)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("optional-auth list: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var items []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode optional-auth list: %v", err)
	}
	for _, p := range items {
		if p.Status == "private" {
			t.Fatalf("flagged user saw private posts via optional auth: %+v", items)
		}
	}
}

// TestListPostsRejectsBadPageParam guards P2-7: an explicitly-supplied but
// non-numeric ?page= (e.g. "abc") must be rejected with 400 instead of
// silently degrading to the legacy bare-array branch.
func TestListPostsRejectsBadPageParam(t *testing.T) {
	svc, _ := setup(t)
	tok := mustToken(t, svc, "u", "u@example.com")

	for _, q := range []string{
		"/api/posts?page=abc",
		"/api/posts?size=abc",
		"/api/posts?category=abc",
		"/api/posts?page=1&size=abc",
	} {
		req := httptest.NewRequest(http.MethodGet, q, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		WithOptionalAuth(ListPosts)(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("GET %s: expected 400 for a bad int param, got %d (%s)", q, rec.Code, rec.Body.String())
		}
	}

	// A valid page param must still work (no false positive).
	req := httptest.NewRequest(http.MethodGet, "/api/posts?page=1&size=10", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	WithOptionalAuth(ListPosts)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/posts?page=1&size=10: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
}
