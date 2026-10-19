package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// newLimiterHandler wires a *Limiter in front of a trivial 200 handler so we
// can drive real HTTP requests through it via httptest.
func newLimiterHandler(l *Limiter) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return l.Middleware(inner)
}

// TestLimiterAllowsUnderBurst verifies that requests within the burst size
// pass through with 200.
func TestLimiterAllowsUnderBurst(t *testing.T) {
	lim := NewLimiter(LimiterConfig{RPS: 1000, Burst: 5})
	h := newLimiterHandler(lim)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// Force every request to share the same bucket (same IP).
		req.RemoteAddr = "1.2.3.4:5"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

// TestLimiterReturns429AfterBurst verifies that once an identity exceeds its
// burst allowance it is throttled with 429 + Retry-After.
func TestLimiterReturns429AfterBurst(t *testing.T) {
	// Very low burst + RPS so the bucket cannot refill mid-test.
	lim := NewLimiter(LimiterConfig{RPS: 0.01, Burst: 2})
	h := newLimiterHandler(lim)

	const identity = "10.0.0.1:99"

	// Burn through the burst.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = identity
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// The next request must be throttled.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = identity
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after exceeding burst, got %d", rec.Code)
	}
	if ra := rec.Header().Get("Retry-After"); ra == "" {
		t.Fatalf("expected Retry-After header on 429, got none")
	}
}

// TestLimiterKeysBucketsPerUserID verifies that the user id (stamped via
// WithUserID on the request context) keys a separate bucket from the client
// IP. Each of two distinct users gets its own allowance, so neither is
// throttled by the other's traffic.
func TestLimiterKeysBucketsPerUserID(t *testing.T) {
	// RPS 0.01 / Burst 1: each identity gets exactly one free request.
	lim := NewLimiter(LimiterConfig{RPS: 0.01, Burst: 1})
	h := newLimiterHandler(lim)

	// Both users share the same client IP but have distinct user ids, so
	// they must land in different buckets. Each should get a 200 (their
	// first and only allowed request).
	for _, uid := range []string{"user-A", "user-B"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "9.9.9.9:1"
		req = req.WithContext(WithUserID(req.Context(), uid))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("user %s: expected 200, got %d", uid, rec.Code)
		}
	}
}

// TestLimiterLRUEviction verifies that when the bucket cache is full the
// least-recently-used identity is evicted — not an arbitrary one. This guards
// against the previous map-iteration-order bug where eviction was random.
func TestLimiterLRUEviction(t *testing.T) {
	lim := NewLimiter(LimiterConfig{RPS: 1000, Burst: 1000})
	lim.maxBuckets = 2 // shrink for the test

	// Fill the cache with "a" and "b".
	_ = lim.limiterFor("a")
	_ = lim.limiterFor("b")
	if lim.lru.Len() != 2 {
		t.Fatalf("expected 2 buckets, got %d", lim.lru.Len())
	}

	// Touch "a" so it becomes most-recently-used; "b" is now LRU.
	_ = lim.limiterFor("a")

	// Insert "c" -> should evict "b" (the least-recently-used).
	_ = lim.limiterFor("c")

	if lim.lru.Len() != 2 {
		t.Fatalf("expected cache capped at 2, got %d", lim.lru.Len())
	}
	if _, ok := lim.buckets["a"]; !ok {
		t.Fatal("expected 'a' (most-recently-used) to survive eviction")
	}
	if _, ok := lim.buckets["c"]; !ok {
		t.Fatal("expected 'c' (just inserted) to be present")
	}
	if _, ok := lim.buckets["b"]; ok {
		t.Fatal("expected 'b' (least-recently-used) to be evicted")
	}
}

// TestLimiterStaysBoundedUnderChurn drives more distinct identities than
// maxBuckets through the middleware and asserts the cache never grows past
// its cap — i.e. no slow memory leak on a long-running low-spec server.
func TestLimiterStaysBoundedUnderChurn(t *testing.T) {
	lim := NewLimiter(LimiterConfig{RPS: 1000, Burst: 1000})
	lim.maxBuckets = 8

	h := newLimiterHandler(lim)
	for i := 0; i < 500; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.1:" + itoa(i) // distinct IP per request
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
	if lim.lru.Len() > lim.maxBuckets {
		t.Fatalf("cache grew past cap: %d > %d", lim.lru.Len(), lim.maxBuckets)
	}
}

// itoa is a tiny local helper to avoid importing strconv in the test for a
// one-off integer-to-string conversion.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
