package captcha

import "time"

// This file holds the policy half of the store: how often one client may ask
// for a picture, and how long a failure streak colours that allowance. It is
// split from the state machine in captcha.go because the two answer different
// questions — "what is a valid answer" versus "how many pictures does this
// client deserve" — and because keeping them together pushed one file past the
// repository's size ceiling.

const (
	// issueWindowLen is the sliding window used for issuing.
	issueWindowLen = time.Minute
	// issueBurst is how many pictures one client may ask for per window. A
	// human needs one or two; this leaves room for a shared office address
	// without giving an attacker enough frames to train an OCR model.
	issueBurst = 30
	// issueTight is the allowance once this client has failed to log in
	// recently. It throttles the guessing loop directly: at ten pictures a
	// minute an attacker gains nothing over just guessing passwords, which the
	// account lockout already caps at five.
	issueTight = 10
	// tightAfter is the failure count that switches to the tight allowance.
	tightAfter = 3
	// maxTries is how many wrong answers one picture tolerates. Each miss is a
	// real attempt by a human who misread the picture, but three is already
	// generous for four characters, and every extra try is brute force budget.
	maxTries = 3
	// failureRetention bounds how long a failure streak colours the allowance.
	//
	// It used to be thirty minutes, which punished the wrong person. Reading
	// four characters off a picture and getting them wrong is a normal human
	// error, and the counter cannot tell that apart from someone guessing
	// passwords — so one mistyped captcha and two fat-fingered passwords cost
	// a real visitor half an hour of being throttled to ten pictures a minute,
	// which reads to them as "the captcha is broken". Five minutes is still
	// far longer than any guessing loop needs to be useful: an attacker who
	// keeps failing keeps refreshing f.at and stays throttled, while someone
	// who simply typed it wrong is back to normal before they finish reading
	// the error message twice.
	failureRetention = 5 * time.Minute
)

// issueWindow counts challenges handed to one client inside a window.
type issueWindow struct {
	start time.Time
	count int
}

// allowIssueLocked reports whether client may be given another picture, and
// records it if so. Caller holds s.mu.
func (s *Store) allowIssueLocked(client string) bool {
	now := s.now()

	limit := issueBurst
	if f, ok := s.fails[client]; ok && f.count >= tightAfter {
		limit = issueTight
	}

	w, ok := s.issues[client]
	if !ok || now.Sub(w.start) >= issueWindowLen {
		s.issues[client] = &issueWindow{start: now, count: 1}
		return true
	}
	if w.count >= limit {
		return false
	}
	w.count++
	return true
}

// NoteFailure records one failed login attempt for the client and returns the
// new count. It is called for both a wrong password and a wrong captcha.
func (s *Store) NoteFailure(client string) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.fails[client]
	if f == nil {
		f = &failure{}
		s.fails[client] = f
	}
	f.count++
	f.at = s.now()
	return f.count
}

// NoteSuccess wipes the client's failure history: a human who eventually
// remembers the password should not keep paying for yesterday's typos.
func (s *Store) NoteSuccess(client string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.fails, client)
}

// FailureCount reports the current streak, for tests and diagnostics.
func (s *Store) FailureCount(client string) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.fails[client]; ok {
		return f.count
	}
	return 0
}

// Reset replaces the clock, for tests that need to advance time.
func (s *Store) Reset(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.now = now
}

// Len reports how many live challenges are held, for tests and metrics.
func (s *Store) Len() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// MaxTries reports the per-challenge attempt budget, for tests.
func (s *Store) MaxTries() int { return maxTries }

// reapLocked drops expired challenges and stale bookkeeping. Caller holds s.mu.
// It runs opportunistically on every issue: there is no background goroutine
// for this by design (the thumbnail worker is the only one the site runs), and
// issue is the only operation that grows the maps.
func (s *Store) reapLocked() {
	now := s.now()
	for id, e := range s.items {
		if now.After(e.expires) {
			delete(s.items, id)
		}
	}
	// A failure history older than the escalation horizon is meaningless: the
	// attacker has stopped, or the human has moved on.
	cutoff := now.Add(-failureRetention)
	for k, f := range s.fails {
		if f.at.Before(cutoff) {
			delete(s.fails, k)
		}
	}
	// Windows are per-minute bookkeeping; drop the ones that no longer apply so
	// a long-running process does not accumulate one entry per address seen.
	for k, w := range s.issues {
		if now.Sub(w.start) >= 2*issueWindowLen {
			delete(s.issues, k)
		}
	}
}
