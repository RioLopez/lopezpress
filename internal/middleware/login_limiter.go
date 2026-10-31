package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IdentityResolver maps a login identifier to the account behind it, without
// verifying any credential.
//
// The lockout has to be keyed by ACCOUNT, not by the string the client typed.
// Login accepts either a username or an email, so a string-keyed lockout can be
// walked around by simply using the other spelling: lock "admin" with five wrong
// guesses, then log in as "admin@example.com" and the counter is back to zero —
// the lockout was never a lockout, it was a per-spelling speed bump. The reverse
// is equally wrong: an attacker who knows both spellings can alternate between
// them for double the attempts, and can lock one while the other still answers.
//
// Resolving the identifier first collapses every spelling of the same account
// onto one key.
//
// A nil resolver is legal: the limiter then keys on the typed string, which
// still throttles someone enumerating names that do not exist.
type IdentityResolver func(ctx context.Context, identifier string) (userID, role string, ok bool)

// maxLoginBodyBytes caps the login request body. A login carries two short
// strings, so 8 KB is generous. The cap is not cosmetic: the handler reads
// the whole body into memory to inspect the identifier, so without it an
// anonymous client that sends a multi-megabyte "email" field both balloons
// the process (R1-02 measured +284 MB RSS from two 25 MB requests) and gets
// that same string written verbatim into login_logs, where SQLite stores it
// even though the column declares size:128 (R3-05 measured a +53 MB database
// from the same two requests).
const maxLoginBodyBytes = 8 << 10

// maxIdentifierKeyLen bounds the identifier before it becomes a map key or a
// log field. Map keys are attacker-controlled strings, so an unbounded key is
// an unbounded memory sink: R3-06 filled the failure map with 50 KB keys that
// nothing ever reclaimed.
const maxIdentifierKeyLen = 254

// clampKey normalises an identifier for use as a map key or a log field.
func clampKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) > maxIdentifierKeyLen {
		s = s[:maxIdentifierKeyLen]
	}
	return s
}

// failRecord is one identifier's failure counter plus the time of its last
// failure. The timestamp is what makes the map reapable (R3-06).
type failRecord struct {
	count int
	at    time.Time
}

// LoginLimiter is a purpose-built limiter for the /api/auth/login endpoint.
// Unlike the general per-identity Limiter, it enforces two independent rate
// limits and a failure-based lockout:
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
	fails    map[string]failRecord
	locked   map[string]time.Time
	// lockLevel counts how many times an identifier has been locked without a
	// successful login in between; it selects the escalation step (R4-04).
	lockLevel map[string]int
	// resolveIdentity maps an identifier to the account behind it, so the
	// account bucket and the lockout follow the ACCOUNT rather than the
	// spelling the client typed. Nil is legal (see IdentityResolver).
	resolveIdentity IdentityResolver
	// knobs (kept here so tests can override)
	ipRPS        float64
	ipBurst      int
	acctRPS      float64
	acctBurst    int
	maxFails     int
	lockWindow   time.Duration
	cleanupEvery time.Duration
}

// SetIdentityResolver installs the identifier -> account mapping. Call it once
// at startup, before the server accepts traffic.
func (l *LoginLimiter) SetIdentityResolver(fn IdentityResolver) {
	l.mu.Lock()
	l.resolveIdentity = fn
	l.mu.Unlock()
}

// NewLoginLimiter builds a LoginLimiter with the documented production
// defaults: 0.5 rps per bucket, burst 10 on the IP bucket and 3 on the account
// bucket, 5 failures, escalating lock (60 s → 5 min → 15 min).
//
// The IP burst is deliberately looser than the account burst. Password guessing
// is bounded by the ACCOUNT bucket plus the lockout; the IP bucket only exists
// to stop one host from flooding the endpoint, and at burst 3 it also throttled
// every colleague behind the same NAT whenever one of them mistyped their
// password three times (R4-04's "collateral lockout" note).
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{
		ipBkts:     make(map[string]*rate.Limiter),
		acctBkts:   make(map[string]*rate.Limiter),
		fails:      make(map[string]failRecord),
		locked:     make(map[string]time.Time),
		lockLevel:  make(map[string]int),
		ipRPS:      0.5,
		ipBurst:    10,
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

		// Read once under the mutex: SetIdentityResolver is called at startup,
		// but taking the lock here keeps `go test -race` honest.
		l.mu.Lock()
		resolve := l.resolveIdentity
		l.mu.Unlock()

		// 1) IP rate limit.
		if !l.ipAllow(ip) {
			l.write429(w, "ip rate limit exceeded", l.ipRPS)
			return
		}

		// 2) Read the JSON body ONCE, extract the identifier, and keep the
		//    raw bytes so we can re-inject a fresh body for the handler.
		//    The body is capped first: an uncapped read is a memory and disk
		//    amplification primitive (R1-02 / R3-05).
		identifier, ok := l.readIdentifier(w, r)
		if !ok {
			return // already answered (413)
		}

		typed := clampKey(identifier)

		// 3–5) Account gate: resolve the account, then rate-limit and lock it.
		acctKey, role, ok := l.accountGate(w, r, identifier, typed, resolve)
		if !ok {
			return // already answered (429)
		}

		// 6) Let the handler run; record success/failure by its status.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		if acctKey != "" {
			switch rec.status {
			case http.StatusOK:
				l.recordSuccess(acctKey)
			case http.StatusUnauthorized, http.StatusForbidden:
				l.recordFailure(acctKey, role)
			}
		}
	})
}

// readIdentifier consumes the request body (under the size cap), replays it for
// the handler, and returns the login identifier it carried. The bool is false
// when the body was over the cap and a 413 has already been written.
//
// Split out of Middleware because the body handling — cap, read, replay, JSON
// decode — is a self-contained concern with its own failure mode, and because
// it is the part an attacker aims at: everything else here is bookkeeping.
func (l *LoginLimiter) readIdentifier(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		l.write413(w)
		return "", false
	}
	// Replay without the second copy the old code made: strings.NewReader
	// string(raw) doubled a body that could already be 8 KB.
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var in struct {
		Email    string `json:"email"`
		Username string `json:"username"`
	}
	if json.Unmarshal(raw, &in) != nil {
		return "", true // not JSON: no identifier, but let the handler answer
	}
	// The login identifier travels in the "email" field today (it may carry a
	// username or an email — see services.LoginInput, which matches
	// `username = ? OR email = ?` against that one field). We also honour an
	// explicit "username" field so the account-level lockout keeps working if
	// a future revision introduces separate username/email login fields.
	if in.Email != "" {
		return in.Email, true
	}
	return in.Username, true
}

// write413 answers a body that exceeded maxLoginBodyBytes.
func (l *LoginLimiter) write413(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusRequestEntityTooLarge)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "request body too large",
		"limit": fmt.Sprintf("%d bytes", maxLoginBodyBytes),
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

// accountGate resolves the account behind an identifier and applies the account
// rate limit and the failure lockout. It returns the key to record under ("" if
// the identifier was empty) and whether the request may proceed.
//
// Everything here is keyed by ACCOUNT, not by the string the client typed, so
// the username and the email spelling of one account share a bucket and a
// lockout — the fix for R7-03, where locking "admin" left "admin@example.com"
// wide open. The per-IP limit has already run by the time this is called, so
// the identity lookup cannot be used to flood the database.
func (l *LoginLimiter) accountGate(w http.ResponseWriter, r *http.Request, identifier, typed string, resolve IdentityResolver) (acctKey, role string, ok bool) {
	if typed == "" {
		return "", "", true
	}
	acctKey, role = "i:"+typed, ""
	if resolve != nil {
		if uid, rr, found := resolve(r.Context(), identifier); found {
			acctKey, role = "u:"+uid, rr
		}
	}
	if !l.acctAllow(acctKey) {
		l.write429(w, "too many attempts for this account", l.acctRPS)
		return "", "", false
	}
	if until, lockedNow := l.lockedUntil(acctKey); lockedNow {
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
		return "", "", false
	}
	return acctKey, role, true
}
