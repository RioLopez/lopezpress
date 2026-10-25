package middleware

import (
	"log/slog"
	"time"

	"lopezpress/internal/models"
)

// --- failure lockout --------------------------------------------------------
//
// Split out of login_limiter.go: the rate buckets answer "how fast may this
// client try", while the lockout answers "has this account been attacked" and
// keeps its own state and its own escalation policy. They share only the maps.

// lockWindow is one step of the escalating lockout. Asking for an account to be
// unreachable for a fixed 15 minutes from the first five wrong guesses is a
// weapon: an anonymous attacker who knows an admin's username can shut that
// admin out of the console at will, repeatedly, at no cost (R4-04). The lock
// still has to exist — the alternative is unlimited password guessing — so it
// escalates instead of starting at the top:
//
//	1st lockout   60 s   a typo costs a minute, an admin is not locked out
//	2nd lockout    5 min someone still guessing
//	3rd +          15 min a real attack, and it now costs the attacker time
//
// The level resets on a successful login, so a legitimate user who eventually
// remembers their password goes back to the 60-second floor.
var lockEscalation = []time.Duration{60 * time.Second, 5 * time.Minute, 15 * time.Minute}

// lockWindowFor returns the lockout duration for the given level (0-based,
// clamped to the last step).
func lockWindowFor(level int) time.Duration {
	if level < 0 {
		level = 0
	}
	if level >= len(lockEscalation) {
		return lockEscalation[len(lockEscalation)-1]
	}
	return lockEscalation[level]
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

// recordFailure bumps the failure counter for an account and, once the
// threshold is crossed, locks it.
//
// `role` is the account's role as resolved for this attempt ("" when the
// identifier belongs to no account, i.e. the key is a typed string). It exists
// for one decision only: a root lockout must not escalate. Root is the only way
// to recover a site, and an anonymous attacker who knows the name could
// otherwise keep the owner locked out for fifteen minutes at a time, for free,
// for as long as they cared to keep going. Root still locks for the first tier,
// so guessing against it is still stopped dead while the lock holds — the
// difference is that the punishment never grows past 60 seconds.
func (l *LoginLimiter) recordFailure(key, role string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.fails[key]
	f.count++
	f.at = time.Now()
	l.fails[key] = f
	if f.count >= l.maxFails {
		// R4-04: escalate instead of jumping straight to the 15-minute ceiling.
		// The first lockout after five wrong guesses is the most likely to be a
		// human typo (or an anonymous attacker aiming at a known admin name),
		// and 60 seconds is enough to stop a script while keeping the account
		// usable. Repeated lockouts without a success in between climb to the
		// full window.
		if l.lockLevel == nil { // a zero-value limiter in a test
			l.lockLevel = make(map[string]int)
		}
		level := l.lockLevel[key]
		window := lockWindowFor(level)
		if role == models.RoleRoot {
			// Never escalate the recovery account; see the doc comment.
			window = lockWindowFor(0)
		}
		l.lockLevel[key] = level + 1
		l.locked[key] = time.Now().Add(window)
		// Reset the counter so the NEXT five guesses are needed to re-lock:
		// otherwise every single attempt after the threshold would extend the
		// lock and the account could never come back.
		l.fails[key] = failRecord{count: 0, at: time.Now()}
		slog.Warn("login lockout engaged", "identifier", key, "window", window.String(), "level", level+1)
	}
}

func (l *LoginLimiter) recordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, key)
	delete(l.locked, key)
	// A successful login is the only thing that clears the escalation history:
	// a legitimate user who eventually gets in should not stay on the 15-minute
	// tier because of yesterday's typos.
	delete(l.lockLevel, key)
}

// cleanup reaps expired locks and stale failure counters so the maps stay
// bounded on a long-lived process.
func (l *LoginLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	// The longest step, not the configured window: an entry must not be reaped
	// while its lock (or its escalation history) is still meaningful.
	maxWindow := lockWindowFor(len(lockEscalation))
	if l.lockWindow > maxWindow {
		maxWindow = l.lockWindow
	}
	cutoff := time.Now().Add(-maxWindow)
	for k, until := range l.locked {
		if until.Before(cutoff) {
			delete(l.locked, k)
			delete(l.fails, k)
		}
	}
	// R3-06: an identifier that failed a few times but was never locked used
	// to stay in `fails` for the life of the process — nothing ever looked at
	// it again, so the map only grew (and its keys were attacker-controlled
	// strings). Each entry now carries the time of its last failure and is
	// reaped on the same schedule as the locks.
	for k, f := range l.fails {
		if f.at.Before(cutoff) {
			delete(l.fails, k)
		}
	}
	// Escalation history expires on the same schedule: an identifier that has
	// not been locked for the longest window starts fresh at the 60-second step
	// rather than staying on the 15-minute tier forever.
	for k := range l.lockLevel {
		if _, stillLocked := l.locked[k]; stillLocked {
			continue
		}
		if _, stillFailing := l.fails[k]; !stillFailing {
			delete(l.lockLevel, k)
		}
	}
}
