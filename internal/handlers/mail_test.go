package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"echo-content-api/internal/models"
)

func TestSendTestMailRequiresAdmin(t *testing.T) {
	_, _ = setup(t)

	// No Authorization header -> withAuth rejects with 401.
	req := httptest.NewRequest(http.MethodPost, "/api/admin/mail/test", nil)
	w := httptest.NewRecorder()
	WithAuth(SendTestMail)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", w.Code)
	}
}

func TestSendTestMailNotConfigured(t *testing.T) {
	svc, _ := setup(t)
	_ = svc

	// Inject an admin user into the context so requireAdmin passes, then the
	// "SMTP not configured" branch (400) is what we actually exercise.
	admin := models.User{ID: "adm", Username: "admin", Email: "admin@example.com", Role: "admin"}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/mail/test", nil)
	req = req.WithContext(context.WithValue(req.Context(), UserContextKey, admin))
	w := httptest.NewRecorder()
	SendTestMail(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when SMTP not configured, got %d (body=%s)", w.Code, w.Body.String())
	}
}
