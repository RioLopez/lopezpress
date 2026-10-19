package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// LoginLimiter is a purpose-built limiter for the /api/auth/login endpoint
// (spec #3). Unlike the general per-identity Limiter, it enforces two
// independent rate limits and a failure-based lockout:
//
//   - IP bucket: 0.5 rps / burst 3 — stops an attacker from blasting
//     logins from one address.
//   - Account bucket: 0.5 rps / burst 3 per login identifier — stops
//     brute-forcing a single account even from many IPs.
//   - Failure lockout: after 5 consecutive failed logins for a given
//     identifier the identifier is locked for 15 minutes; during the lockout
//     the response carries a Retry-After header.
//
// A successful login resets that identifier's failure counter.
type LoginLimiter struct {
	mu       sync.Mutex
	ipBkts   map[string]*rate.Limiter
	acctBkts map[string]*rate.Limiter
	fails    map[string]int
	locked   map[string]time.Time
	// knobs (kept here so tests can override)
	ipRPS      float64
	ipBurst    int
	acctRPS    float64
	acctBurst  int
	maxFails   int
	lockWindow time.Duration
	cleanupEvery time.Duration
}

// NewLoginLimiter builds a LoginLimiter with the documented production
// defaults (0.5 rps / burst 3 per bucket, 5 failures, 15 min lock).
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		ipBkts:     make(map[string]*rate.Limiter),
		acctBkts:   make(map[string]*rate.Limiter),
		fails:      make(map[string]int),
		locked:     make(map[string]time.Time),
		ipRPS:      0.5,
		ipBurst:    3,
		acctRPS:    0.5,
		acctBurst:  3,
		maxFails:   5,
		lockWindow: 15 * time.Minute,
	}
}

// Start launches the background janitor that reaps stale failure/lock
// entries every minute. Safe to call multiple times; subsequent calls
// return immediately. The janitor stops when ctx is cancelled.
func (l *LoginLimiter) Start(ctx context.Context) {
	if l.cleanupEvery <= 0 {
		l.cleanupEvery = time.Minute
	}
	go func() {
		ticker := time.NewTicker(l.cleanupEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.cleanup()
			}
		}
	}()
}

// Middleware returns an http middleware enforcing the login limits. It must
// be wrapped around ONLY the /api/auth/login route: it reads the JSON body's
// identifier (the "email" field, or an explicit "username" field as a
// fallback), so applying it to other endpoints would consume their bodies.
func (l *LoginLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)

		// 1) IP rate limit.
		if !l.ipAllow(ip) {
			l.write429(w, "ip rate limit exceeded", l.ipRPS)
			return
		}

		// 2) Read the JSON body ONCE, extract the identifier, and keep the
		//    raw bytes so we can re-inject a fresh body for the handler.
		raw, err := io.ReadAll(r.Body)
		if err == nil {
			r.Body = io.NopCloser(strings.NewReader(string(raw)))
		}
		identifier := ""
		if err == nil {
			var in struct {
				Email    string `json:"email"`
				Username string `json:"username"`
			}
			if json.Unmarshal(raw, &in) == nil {
				// The login identifier travels in the "email" field today
				// (it may carry a username or an email — see services.LoginInput,
				// which matches `username = ? OR email = ?` against that one
				// field). We also honour an explicit "username" field so the
				// account-level lockout keeps working if a future revision
				// introduces separate username/email login fields.
				identifier = in.Email
				if identifier == "" {
					identifier = in.Username
				}
			}
		}

		if key := strings.ToLower(strings.TrimSpace(identifier)); key != "" {
			// 3) Account rate limit.
			if !l.acctAllow(key) {
				l.write429(w, "too many attempts for this account", l.acctRPS)
				return
			}
			// 4) Failure lockout.
			if until, lockedNow := l.lockedUntil(key); lockedNow {
				retrySecs := int64(time.Until(until).Seconds())
				if retrySecs < 1 {
					retrySecs = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySecs))
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"error":         "too many failed attempts, account temporarily locked",
					"retry_after_s": retrySecs,
				})
				return
			}
		}

		// 5) Let the handler run; record success/failure by its status.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if key := strings.ToLower(strings.TrimSpace(identifier)); key != "" {
			switch rec.status {
			case http.StatusOK:
				l.recordSuccess(key)
			case http.StatusUnauthorized, http.StatusForbidden:
				l.recordFailure(key)
			}
		}
	})
}

// write429 emits a 429 with a Retry-After estimated from the bucket RPS.
// At 0.5 rps a token refills every 2 s.
func (l *LoginLimiter) write429(w http.ResponseWriter, msg string, rps float64) {
	retry := int64(1)
	if rps > 0 {
		retry = int64(1.0 / rps)
		if retry < 1 {
			retry = 1
		}
	}
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retry))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// --- internals (all take l.mu) ---

func (l *LoginLimiter) ipAllow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.ipBkts[ip]
	if !ok {
		l.evictLocked(l.ipBkts)
		lim = rate.NewLimiter(rate.Limit(l.ipRPS), l.ipBurst)
		l.ipBkts[ip] = lim
	}
	return lim.Allow()
}

func (l *LoginLimiter) acctAllow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.acctBkts[key]
	if !ok {
		l.evictLocked(l.acctBkts)
		lim = rate.NewLimiter(rate.Limit(l.acctRPS), l.acctBurst)
		l.acctBkts[key] = lim
	}
	return lim.Allow()
}

// evictLocked removes an arbitrary bucket entry when the cap is reached.
// Must be called with l.mu held.
func (l *LoginLimiter) evictLocked(m map[string]*rate.Limiter) {
	if len(m) >= maxBuckets {
		for k := range m {
			delete(m, k)
			break
		}
	}
}

// lockedUntil reports whether the identifier is inside its lockout window.
func (l *LoginLimiter) lockedUntil(key string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	until, ok := l.locked[key]
	if !ok {
		return time.Time{}, false
	}
	if time.Now().Before(until) {
		return until, true
	}
	delete(l.locked, key)
	return time.Time{}, false
}

func (l *LoginLimiter) recordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[key]++
	if l.fails[key] >= l.maxFails {
		l.locked[key] = time.Now().Add(l.lockWindow)
		slog.Warn("login lockout engaged", "identifier", key, "window", l.lockWindow.String())
	}
}

func (l *LoginLimiter) recordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.locked, key)
}

// cleanup reaps expired locks and (optionally) stale failure counters so
// the maps stay bounded on a long-lived process.
func (l *LoginLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.lockWindow)
	for k, until := range l.locked {
		if until.Before(cutoff) {
			delete(l.locked, k)
			delete(l.fails, k)
		}
	}
}
