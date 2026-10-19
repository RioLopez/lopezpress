package services

import (
	"context"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// signRaw signs an arbitrary claim map with the service's (possibly dev
// fallback) secret, for tests that need a token without a version claim.
func signRaw(svc *Service, claims map[string]any) string {
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(claims))
	tok, err := t.SignedString([]byte(svc.signingSecret()))
	if err != nil {
		panic(err)
	}
	return tok
}

// TestRevokeAllSessionsInvalidatesToken verifies the JWT revocation primitive
// behind "log out all devices": a token issued at version N is accepted until
// RevokeAllSessions bumps the user's TokenVersion, after which the same token
// is rejected with 401.
func TestRevokeAllSessionsInvalidatesToken(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	user := makeUser(t, db, "revuser", "rev@example.com", "user")

	// Sign a token carrying the current version (0).
	tok, _, err := svc.SignToken(user.ID, user.Role, user.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}

	// Valid before revocation.
	if _, err := svc.Authenticate(ctx, "Bearer "+tok); err != nil {
		t.Fatalf("expected token valid before revoke, got %v", err)
	}

	// Revoke every session for this user.
	if err := svc.RevokeAllSessions(ctx, user); err != nil {
		t.Fatalf("RevokeAllSessions: %v", err)
	}

	// The same token must now be rejected.
	if _, err := svc.Authenticate(ctx, "Bearer "+tok); StatusOf(err) != http.StatusUnauthorized {
		t.Fatalf("expected 401 after revoke, got %v (err=%v)", StatusOf(err), err)
	}
}

// TestLegacyTokenAcceptedWhileVersionIsZero guards backward compatibility: a
// token without a "ver" claim is still accepted as long as the stored version
// is 0 (the state on first deploy, before any revocation).
func TestLegacyTokenAcceptedWhileVersionIsZero(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	user := makeUser(t, db, "leguser", "leg@example.com", "user")

	// Build a token manually with no "ver" claim.
	claims := map[string]any{"sub": user.ID, "role": user.Role, "exp": 9_999_999_999, "iat": 1}
	tok := signRaw(svc, claims)

	if _, err := svc.Authenticate(ctx, "Bearer "+tok); err != nil {
		t.Fatalf("expected legacy token accepted at version 0, got %v", err)
	}
}
