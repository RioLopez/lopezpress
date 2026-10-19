package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// newTestLoginLimiter builds a limiter with fast, test-friendly knobs:
// high RPS so the bucket rarely trips in a tight loop, but a very small
// maxFails/lockWindow so the lockout path is reachable without waiting.
func newTestLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		ipBkts:     make(map[string]*rate.Limiter),
		acctBkts:   make(map[string]*rate.Limiter),
		fails:      make(map[string]int),
		locked:     make(map[string]time.Time),
		ipRPS:      100,
		ipBurst:    1000,
		acctRPS:    100,
		acctBurst:  1000,
		maxFails:   3,
		lockWindow: time.Minute,
	}
}

func loginReq(t *testing.T, identifier, ip string) *http.Request {
	t.Helper()
	body := `{"email":"` + identifier + `","password":"x"}`
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	if ip != "" {
		r.Header.Set("X-Forwarded-For", ip)
	}
	return r
}

// fakeHandler returns the configured status to simulate the login outcome.
func fakeHandler(status int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	})
}

func TestLoginLimiterLockoutAfterMaxFails(t *testing.T) {
	l := newTestLoginLimiter()
	mw := l.Middleware(fakeHandler(http.StatusUnauthorized))

	// 3 consecutive failures for the same identifier engage lockout (maxFails=3).
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, loginReq(t, "alice@example.com", "1.1.1.1"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401 from fake handler, got %d", i+1, w.Code)
		}
	}

	// 4th attempt now hits the lockout branch -> 429 + Retry-After.
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, loginReq(t, "alice@example.com", "2.2.2.2"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 lockout, got %d", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra == "" {
		t.Fatalf("lockout response must set Retry-After")
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["retry_after_s"]; !ok {
		t.Fatalf("lockout body should include retry_after_s, got %v", body)
	}
}

func TestLoginLimiterSuccessResetsCounter(t *testing.T) {
	l := newTestLoginLimiter()
	// Controllable status: first N calls fail, then succeed.
	var failFor int = 2
	var calls int
	mw := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= failFor {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))

	// 2 failures (matches maxFails=3, so no lock yet).
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, loginReq(t, "bob@example.com", "1.1.1.1"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("fail #%d expected 401, got %d", i+1, w.Code)
		}
	}
	// Success resets the counter.
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, loginReq(t, "bob@example.com", "1.1.1.1"))
	if w.Code != http.StatusOK {
		t.Fatalf("expected success 200, got %d", w.Code)
	}
	// After reset, 3 MORE consecutive failures are required to lock
	// (not just 1 more). So the next 3 attempts should all pass through to
	// the handler (which now returns 401 again) and the 4th triggers lock.
	failFor = 1000 // keep failing
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, loginReq(t, "bob@example.com", "1.1.1.1"))
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("failure %d should not have locked yet (reset verified by maxFails=3)", i+1)
		}
	}
	// The 4th consecutive failure engages lockout.
	w = httptest.NewRecorder()
	mw.ServeHTTP(w, loginReq(t, "bob@example.com", "1.1.1.1"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected lockout 429 after 4 consecutive failures, got %d", w.Code)
	}
}

func TestLoginLimiterIPBucketDistinctFromAccount(t *testing.T) {
	l := newTestLoginLimiter()
	// Tighten the IP bucket to see 429 on a 4th burst from the same IP.
	l.ipRPS = 100
	l.ipBurst = 3
	mw := l.Middleware(fakeHandler(http.StatusOK))

	// 3 different accounts from the same IP: the IP bucket (burst 3) should
	// allow the first 3 and reject the 4th even though each account is fresh.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, loginReq(t, "acct"+string(rune('a'+i))+"@x.com", "9.9.9.9"))
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("IP burst attempt %d should pass", i+1)
		}
	}
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, loginReq(t, "acctd@x.com", "9.9.9.9"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request from same IP should 429 (IP bucket), got %d", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra == "" {
		t.Fatalf("IP 429 must set Retry-After")
	}
}

func TestLoginLimiterCaseInsensitiveAccount(t *testing.T) {
	l := newTestLoginLimiter()
	l.maxFails = 2
	// Make the fake handler fail twice then succeed so we exercise the
	// per-account bucket across case variants of the same identifier.
	var calls int
	mw := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls <= 2 {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))

	// Two failures under "Alice@X.COM".
	mw.ServeHTTP(httptest.NewRecorder(), loginReq(t, "Alice@X.COM", "1.1.1.1"))
	mw.ServeHTTP(httptest.NewRecorder(), loginReq(t, "alice@x.com", "1.1.1.1"))
	// Now the lockout should be keyed on the lowercased form, so the next
	// attempt with any casing hits the lockout branch (429).
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, loginReq(t, "ALICE@X.COM", "1.1.1.1"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected case-insensitive lockout 429, got %d", w.Code)
	}
}

func TestLoginLimiterMalformedBodyPassesThrough(t *testing.T) {
	l := newTestLoginLimiter()
	mw := l.Middleware(fakeHandler(http.StatusOK))
	// A non-JSON body: identifier extraction fails (identifier=""), so the
	// limiter must still let the request reach the handler (no 429/401 here
	// — the fake returns 200 regardless).
	r := httptest.NewRequest("POST", "/api/auth/login", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("malformed body should pass through to handler, got %d", w.Code)
	}
}

// loginReqUsername sends a login body with only a "username" field (no
// "email"), exercising the identifier fallback path.
func loginReqUsername(t *testing.T, username, ip string) *http.Request {
	t.Helper()
	body := `{"username":"` + username + `","password":"x"}`
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	if ip != "" {
		r.Header.Set("X-Forwarded-For", ip)
	}
	return r
}

// TestLoginLimiterUsernameFieldFallback verifies the defensive identifier
// extraction: when the login body carries only a "username" field (no
// "email"), the account-level lockout still keys on that username. Today the
// client sends the identifier in the "email" field, but this guards the
// lockout against a future revision that introduces separate username/email
// login fields (HANDOVER §7.2).
func TestLoginLimiterUsernameFieldFallback(t *testing.T) {
	l := newTestLoginLimiter()
	mw := l.Middleware(fakeHandler(http.StatusUnauthorized))

	// maxFails=3 consecutive failures keyed on the username.
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, loginReqUsername(t, "carol", "3.3.3.3"))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, w.Code)
		}
	}
	// 4th attempt is locked even though the body carries no email field.
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, loginReqUsername(t, "carol", "4.4.4.4"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("username-only login should lock after maxFails, got %d", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra == "" {
		t.Fatalf("lockout must set Retry-After")
	}
}
