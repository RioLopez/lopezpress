package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The HttpOnly cookie must take precedence over the Authorization header, so an
// XSS that manages to read a leaked header token can't override the cookie path.
func TestExtractTokenCookiePrecedence(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: authCookieName, Value: "cookie-token"})
	r.Header.Set("Authorization", "Bearer header-token")
	if got := extractToken(r); got != "cookie-token" {
		t.Fatalf("cookie should win, got %q", got)
	}
}

func TestExtractTokenHeaderFallback(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer header-token")
	if got := extractToken(r); got != "header-token" {
		t.Fatalf("header fallback failed, got %q", got)
	}

	// A bare token (no Bearer prefix) is still accepted for API clients.
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	r2.Header.Set("Authorization", "bare-token")
	if got := extractToken(r2); got != "bare-token" {
		t.Fatalf("bare token fallback failed, got %q", got)
	}
}

func TestExtractTokenAbsent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := extractToken(r); got != "" {
		t.Fatalf("expected empty token, got %q", got)
	}
}

func TestSetAndClearAuthCookie(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	setAuthCookie(w, r, "tok", time.Now().Add(time.Hour))
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if !c.HttpOnly {
		t.Error("expected HttpOnly")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSite=Lax, got %v", c.SameSite)
	}
	if c.Secure {
		t.Error("expected Secure=false on a plain-HTTP request")
	}

	w2 := httptest.NewRecorder()
	clearAuthCookie(w2, r)
	cc := w2.Result().Cookies()
	if len(cc) != 1 || cc[0].MaxAge != -1 {
		t.Errorf("expected a clearing cookie with MaxAge=-1, got %+v", cc)
	}
}
