package services

import (
	"context"
	"net/http"
	"testing"

	"lopezpress/internal/models"
)

// TestAuthenticateRejectsDisabledUser is the regression test for the as-built
// audit finding Q6: a disabled account must lose API access immediately, not
// only at the next login attempt. Without the gate in Authenticate, a
// stateless JWT (valid up to 24h) kept every existing session of a disabled
// account fully usable — including write endpoints.
//
// The re-enable half of this test used to assert the opposite of what we now
// want: it asserted that the SAME token works again after a re-enable. That
// is exactly the R6-01 hole — a suspension that is not actually a suspension,
// because nothing bumped token_version. The state is still written directly
// here (this test is about the Authenticate gate, not the toggle), so the
// token stays valid across a raw column flip; the toggle's own revocation
// behaviour is covered by TestDisableRevokesSessions.
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

	// Re-enabling restores access for a token that was never revoked (the raw
	// column flip does not touch token_version).
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

// R6-01: disabling an account has to end its sessions. The endpoint used to
// write only the `disabled` column, so a stolen token kept working for the
// whole suspension and came back to life the moment the account was
// re-enabled — the measured sequence was disable → 403 → re-enable → 200 with
// the same token. Every other revocation primitive (change password, admin
// reset, logout-all, password reset) bumps token_version; this one now does
// too, so the suspension is real and the revival cannot happen.
func TestDisableRevokesSessions(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "rooty", "rooty@example.com", "root")
	target := makeUser(t, db, "suspendme", "suspend@example.com", "user")

	// A live session, as if the user had logged in before the suspension.
	tok, _, err := svc.SignToken(target.ID, target.Role, target.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+tok); err != nil {
		t.Fatalf("token should be valid before suspension: %v", err)
	}

	// Suspend through the real endpoint (not a raw column write) so the
	// token_version bump is exercised.
	if _, err := svc.SetUserDisabled(ctx, admin, target.ID, true); err != nil {
		t.Fatalf("SetUserDisabled(true): %v", err)
	}

	// The same token must now be rejected. Authenticate checks the version
	// before the disabled flag, so a bumped account reports 401 "token
	// revoked" — which is the more accurate of the two refusals.
	if _, err := svc.Authenticate(ctx, "Bearer "+tok); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("suspended user's token: expected 401, got %v (err=%v)", StatusOf(err), err)
	}

	// Lift the suspension. The old token must NOT come back: this is the
	// revival R6-01 reported.
	if _, err := svc.SetUserDisabled(ctx, admin, target.ID, false); err != nil {
		t.Fatalf("SetUserDisabled(false): %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+tok); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("re-enabled account must not accept the pre-suspension token, got %v (err=%v)", StatusOf(err), err)
	}

	// A fresh sign-in at the current version works again. Reload the row: the
	// service bumped the stored version, so the in-memory copy is stale.
	var fresh models.User
	if err := db.Where("id = ?", target.ID).First(&fresh).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if fresh.Disabled {
		t.Fatalf("account should be re-enabled")
	}
	tok2, _, err := svc.SignToken(fresh.ID, fresh.Role, fresh.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken after re-enable: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+tok2); err != nil {
		t.Fatalf("re-enabled account should accept a fresh token: %v", err)
	}
}

// Re-disabling must bump again, and a no-op toggle must not: flipping the flag
// to the value it already holds would otherwise revoke a session for nothing.
func TestDisableToggleIsIdempotentForTokens(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	admin := makeUser(t, db, "rooty2", "rooty2@example.com", "root")
	u := makeUser(t, db, "steady", "steady@example.com", "user")

	tok, _, err := svc.SignToken(u.ID, u.Role, u.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	// Disable once (bumps), then disable again (no-op, must not bump).
	if _, err := svc.SetUserDisabled(ctx, admin, u.ID, true); err != nil {
		t.Fatalf("first disable: %v", err)
	}
	afterFirst, err := svc.SetUserDisabled(ctx, admin, u.ID, true)
	if err != nil {
		t.Fatalf("second disable: %v", err)
	}
	if afterFirst.TokenVersion != u.TokenVersion+1 {
		t.Fatalf("a repeated disable must not bump again: got %d, want %d",
			afterFirst.TokenVersion, u.TokenVersion+1)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+tok); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("token should be rejected after the real disable, got %v", err)
	}

	// Re-enable does not bump either: the account is simply usable again.
	back, err := svc.SetUserDisabled(ctx, admin, u.ID, false)
	if err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	if back.TokenVersion != u.TokenVersion+1 {
		t.Fatalf("re-enable must not bump token_version: got %d, want %d",
			back.TokenVersion, u.TokenVersion+1)
	}
}
