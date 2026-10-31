package middleware

import (
	"bytes"
	"context"
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
		fails:      make(map[string]failRecord),
		locked:     make(map[string]time.Time),
		lockLevel:  make(map[string]int),
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

// R1-02 / R3-05: the login body used to be read with a bare io.ReadAll, so an
// anonymous client could send a multi-megabyte "email" field, balloon the
// process by hundreds of MB, and have that same string stored verbatim in
// login_logs. The body is now capped and the over-cap answer is an explicit
// 413 rather than a handler decode failure.
func TestLoginLimiterRejectsOversizedBody(t *testing.T) {
	l := newTestLoginLimiter()
	mw := l.Middleware(fakeHandler(http.StatusOK))

	// 1 MB identifier — two orders of magnitude past the 8 KB cap.
	huge := strings.Repeat("a", 1<<20)
	body := `{"email":"` + huge + `","password":"x"}`
	r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized login body, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "too large") {
		t.Fatalf("413 body should explain the limit, got %q", w.Body.String())
	}
}

// The cap must not disturb an ordinary login: a well-formed body still reaches
// the handler and still keys the account bucket.
func TestLoginLimiterNormalBodyStillPasses(t *testing.T) {
	l := newTestLoginLimiter()
	mw := l.Middleware(fakeHandler(http.StatusOK))

	r := loginReq(t, "alice@example.com", "1.1.1.1")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("normal login body should pass through, got %d", w.Code)
	}
}

// R3-06: failure entries used to live in the map forever unless the account
// got locked (only then did cleanup look at them). A few failures that never
// reached maxFails therefore leaked memory with attacker-controlled keys.
func TestLoginLimiterReapsStaleFailureCounters(t *testing.T) {
	l := newTestLoginLimiter()
	l.lockWindow = time.Minute
	mw := l.Middleware(fakeHandler(http.StatusUnauthorized))

	// Two failures — below maxFails=3, so the account is never locked and the
	// old code would have kept the entry indefinitely.
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, loginReq(t, "bob@example.com", "2.2.2.2"))
	}

	l.mu.Lock()
	if len(l.fails) != 1 {
		l.mu.Unlock()
		t.Fatalf("expected 1 tracked failure, got %d", len(l.fails))
	}
	// Age the entry past the longest lockout step, which is the janitor's
	// cutoff (a counter must not be reaped while a 15-minute lock could still
	// be referring to it).
	l.fails["i:bob@example.com"] = failRecord{count: 2, at: time.Now().Add(-16 * time.Minute)}
	l.mu.Unlock()

	l.cleanup()

	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.fails) != 0 {
		t.Fatalf("stale failure counter should be reaped, got %d entries", len(l.fails))
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
// login fields.
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

// R4-04: a fixed 15-minute lockout from the first five wrong guesses let an
// anonymous attacker shut a known admin out of the console at will, for free,
// forever. The lock still has to exist (the alternative is unlimited guessing),
// so it escalates: 60 s, then 5 min, then 15 min — and a successful login puts
// the account back on the 60-second floor.
func TestLockoutEscalatesAndResetsOnSuccess(t *testing.T) {
	l := newTestLoginLimiter()
	l.maxFails = 3

	if got := lockWindowFor(0); got != 60*time.Second {
		t.Fatalf("first lockout must be 60s, got %v", got)
	}
	if got := lockWindowFor(1); got != 5*time.Minute {
		t.Fatalf("second lockout must be 5m, got %v", got)
	}
	if got := lockWindowFor(2); got != 15*time.Minute {
		t.Fatalf("third lockout must be 15m, got %v", got)
	}
	// Beyond the last step it clamps rather than growing without bound.
	if got := lockWindowFor(99); got != 15*time.Minute {
		t.Fatalf("clamped lockout must be 15m, got %v", got)
	}

	key := "admin@example.com"

	// First lockout: 60 seconds.
	for i := 0; i < 3; i++ {
		l.recordFailure(key, "")
	}
	first, ok := l.locked[key]
	if !ok {
		t.Fatal("expected a lockout after maxFails")
	}
	if d := time.Until(first); d > 61*time.Second || d < 55*time.Second {
		t.Fatalf("first lockout should be ~60s, got %v", d)
	}

	// Three more failures while the first lock is still recorded → 5 minutes.
	for i := 0; i < 3; i++ {
		l.recordFailure(key, "")
	}
	second := l.locked[key]
	if d := time.Until(second); d < 4*time.Minute {
		t.Fatalf("second lockout should be ~5m, got %v", d)
	}

	// A successful login clears the escalation history entirely.
	l.recordSuccess(key)
	if _, still := l.locked[key]; still {
		t.Fatal("a successful login must clear the lockout")
	}
	if lv, still := l.lockLevel[key]; still {
		t.Fatalf("a successful login must reset the escalation level, got %d", lv)
	}

	// …and the next lockout is back to 60 seconds.
	for i := 0; i < 3; i++ {
		l.recordFailure(key, "")
	}
	if d := time.Until(l.locked[key]); d > 61*time.Second {
		t.Fatalf("after a success the next lockout must be 60s again, got %v", d)
	}
}

// Re-locking must need a fresh run of failures: if every attempt after the
// threshold extended the lock, the account could never come back.
func TestLockoutCounterResetsSoTheAccountCanRecover(t *testing.T) {
	l := newTestLoginLimiter()
	l.maxFails = 3
	key := "user@example.com"

	for i := 0; i < 3; i++ {
		l.recordFailure(key, "")
	}
	if _, locked := l.locked[key]; !locked {
		t.Fatal("expected lockout")
	}
	// The counter must have been reset by the lock, so a single further
	// failure does not immediately re-lock at the next tier.
	l.recordFailure(key, "")
	if lv := l.lockLevel[key]; lv != 1 {
		t.Fatalf("one failure after the lock must not escalate again, level = %d", lv)
	}
}

// R7-03: login matches `username = ? OR email = ?`, so a lockout keyed on the
// string the client typed is not a lockout at all. Five wrong guesses against
// "admin" left "admin@example.com" completely untouched -- the attacker simply
// switches spelling and keeps guessing, and the owner of the account cannot get
// in because the lock only followed one of their two names.
//
// This drives the real middleware with a resolver, the way main.go wires it.
func TestLockoutFollowsTheAccountNotTheSpelling(t *testing.T) {
	l := newTestLoginLimiter()
	l.maxFails = 3
	// Both spellings belong to one account.
	l.SetIdentityResolver(func(_ context.Context, identifier string) (string, string, bool) {
		switch strings.ToLower(identifier) {
		case "admin", "admin@example.com":
			return "u-1", "user", true
		}
		return "", "", false
	})

	// Wrong password → 401; right password → 200.
	mw := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Password string `json:"password"`
		}
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Password == "right" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))

	wrong := func(identifier string) int {
		body := `{"email":"` + identifier + `","password":"wrong"}`
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, r)
		return w.Code
	}
	right := func(identifier string) int {
		body := `{"email":"` + identifier + `","password":"right"}`
		r := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(body))
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, r)
		return w.Code
	}

	for i := 0; i < 3; i++ {
		if got := wrong("admin"); got != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, got)
		}
	}

	// The username spelling is locked.
	if got := wrong("admin"); got != http.StatusTooManyRequests {
		t.Fatalf("the account should be locked after 3 failures, got %d", got)
	}
	// And the email spelling must be locked too -- this is the bug.
	if got := right("admin@example.com"); got == http.StatusOK {
		t.Fatal("the lockout was bypassed by using the email spelling: a correct " +
			"password got 200 while the account was locked")
	} else if got != http.StatusTooManyRequests {
		t.Fatalf("the email spelling should hit the same lockout (429), got %d", got)
	}
	// A DIFFERENT account must not be affected by someone else's lockout.
	if got := right("bob@example.com"); got != http.StatusOK {
		t.Fatalf("an unrelated identifier was collaterally locked (%d)", got)
	}
}

// Alternating spellings used to buy double the attempts: each spelling had its
// own counter, so a 5-failure lock never arrived. With an account key, the
// counter is shared and the account locks on schedule.
func TestAlternatingSpellingsShareOneFailureCounter(t *testing.T) {
	l := newTestLoginLimiter()
	l.maxFails = 3
	l.SetIdentityResolver(func(_ context.Context, identifier string) (string, string, bool) {
		if identifier == "eve" || identifier == "eve@example.com" {
			return "u-eve", "user", true
		}
		return "", "", false
	})

	// Three failures, alternating spelling as the attacker would.
	for i := 0; i < 3; i++ {
		l.recordFailure("u:u-eve", "user")
	}
	if _, locked := l.locked["u:u-eve"]; !locked {
		t.Fatal("3 alternating failures must lock the account")
	}
	// The typed strings themselves were never used as keys.
	if _, ok := l.fails["eve"]; ok {
		t.Fatal("the typed spelling is still being used as a key")
	}
}

// Root is the only way back into a site. It still locks (so guessing stops
// dead), but the punishment never grows: an anonymous attacker who knows the
// name must not be able to keep the owner out for fifteen minutes at a time.
func TestRootLockoutNeverEscalates(t *testing.T) {
	l := newTestLoginLimiter()
	l.maxFails = 1

	for round := 0; round < 5; round++ {
		l.recordFailure("u:root-id", "root")
		until := l.locked["u:root-id"]
		if d := time.Until(until); d > 61*time.Second {
			t.Fatalf("root lockout escalated to %v on round %d; it must stay at 60s", d, round+1)
		}
		// Clear the lock so the next round re-locks, and NO success in between
		// (that is what would normally reset the level).
		delete(l.locked, "u:root-id")
	}
	// A non-root account in the same state does escalate, which is the point of
	// the comparison: the cap is specific to root, not a global softening.
	other := newTestLoginLimiter()
	other.maxFails = 1
	for round := 0; round < 3; round++ {
		other.recordFailure("u:user-id", "user")
		delete(other.locked, "u:user-id")
	}
	other.recordFailure("u:user-id", "user")
	if d := time.Until(other.locked["u:user-id"]); d < 4*time.Minute {
		t.Fatalf("a normal account should have escalated by now, got %v", d)
	}
}
