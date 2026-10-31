package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"lopezpress/internal/models"
)

// First boot (and `-reset-root-password`) sets must_change_password on the
// account, and the server then refuses every endpoint except the
// password-change one until it is cleared.
//
// GET /api/auth/me hand-assembled its UserPublic and dropped the flag. After one
// reload the change-password screen read the stale view, said "nothing to
// change", and offered a "continue" button into a site where every action
// answered 403 — with no route back to the form except logging out. /me is the
// only source the screen can trust after a reload, so the flag has to survive
// the round trip.
func TestMeReportsMustChangePassword(t *testing.T) {
	h, svc, db := buildTestRouter(t)
	token := tokenFor(t, svc, "firstboot", "firstboot@example.com")

	if err := db.Model(&models.User{}).
		Where("username = ?", "firstboot").
		Update("must_change_password", true).Error; err != nil {
		t.Fatalf("set flag: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("me: expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if flag, _ := got["must_change_password"].(bool); !flag {
		t.Fatalf("must_change_password missing from /api/auth/me: %s", rec.Body.String())
	}
}
