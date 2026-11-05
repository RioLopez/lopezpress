package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// mintToken signs a minimal HS256 token with the given secret/subject so tests
// can exercise the limiter's verified-token path without a database.
func mintToken(t *testing.T, secret, sub string) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": sub, "exp": time.Now().Add(time.Hour).Unix()}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return tok
}

// TestLimiterGlobalAnonymousBucketCapsRotatingIPs proves D5: an attacker
// spreading load across many source IPs can no longer dilute the per-IP rate
// limit, because a single global anonymous bucket caps all unauthenticated
// traffic combined.
//
// The per-identity buckets are given a generous allowance here so ONLY the
// global anonymous bucket can trip; if it did not exist, 500 distinct-IP
// requests would all pass.
func TestLimiterGlobalAnonymousBucketCapsRotatingIPs(t *testing.T) {
	lim := NewLimiter(LimiterConfig{RPS: 1000, Burst: 1000})
	h := newLimiterHandler(lim)

	const total = 500
	statuses := map[int]int{}
	for i := 0; i < total; i++ {
		// Distinct RemoteAddr each time so the per-IP bucket can never catch
		// them — exactly the attack the global bucket is meant to stop.
		req := httptest.NewRequest("GET", "/x", nil)
		req.RemoteAddr = fmt.Sprintf("1.2.3.%d:1234", i%256)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		statuses[rec.Code]++
	}

	if statuses[http.StatusTooManyRequests] == 0 {
		t.Fatalf("global anonymous bucket should throttle rotating-IP traffic; got 0 x429 (statuses=%v)", statuses)
	}
	if got := statuses[http.StatusOK]; got > anonDefaultBurst+5 {
		t.Fatalf("expected at most the anon burst (%d) to pass, got %d OK", anonDefaultBurst, got)
	}
}

// TestLimiterAuthenticatedSkipsGlobalAnonBucket proves that signed-in callers
// are NOT charged against the anonymous bucket, so a logged-in admin is never
// throttled by anonymous traffic pressure.
func TestLimiterAuthenticatedSkipsGlobalAnonBucket(t *testing.T) {
	lim := NewLimiter(LimiterConfig{RPS: 1000, Burst: 1000})
	lim.SetTokenSecret("test-secret-for-limiter-only")
	h := newLimiterHandler(lim)

	tok := mintToken(t, "test-secret-for-limiter-only", "user-1")

	// Far more than the anon burst, all authenticated as the same user. The
	// per-user bucket (1000 burst) absorbs them; the anon bucket must not.
	for i := 0; i < 500; i++ {
		req := httptest.NewRequest("GET", "/x", nil)
		req.RemoteAddr = fmt.Sprintf("9.9.9.%d:1234", i%256)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("authenticated request %d unexpectedly got %d (anon bucket leaking onto authed traffic?)", i, rec.Code)
		}
	}
}
