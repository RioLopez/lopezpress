package services

import (
	"context"
	"net/http"
	"testing"

	"echo-content-api/internal/models"
)

// TestAuthenticateRejectsDisabledUser is the regression test for the as-built
// audit finding Q6: a disabled account must lose API access immediately, not
// only at the next login attempt. Without the gate in Authenticate, a
// stateless JWT (valid up to 24h) kept every existing session of a disabled
// account fully usable — including write endpoints.
func TestAuthenticateRejectsDisabledUser(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	// A disabled account that still holds a freshly signed token (as if it
	// had logged in before being disabled).
	u := makeUser(t, db, "banned", "banned@example.com", "user")
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).Update("disabled", true).Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	token, _, err := svc.SignToken(u.ID, u.Role, u.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	// The token is cryptographically valid, but the account is disabled.
	if _, err := svc.Authenticate(ctx, token); StatusOf(err) != http.StatusForbidden {
		t.Errorf("disabled user token: expected 403, got %v (err=%v)", StatusOf(err), err)
	}

	// Re-enabling restores access with the same token (no re-login needed).
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).Update("disabled", false).Error; err != nil {
		t.Fatalf("re-enable user: %v", err)
	}
	got, err := svc.Authenticate(ctx, token)
	if err != nil {
		t.Errorf("re-enabled user token: expected success, got %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("re-enabled user: expected id %q, got %q", u.ID, got.ID)
	}
}
