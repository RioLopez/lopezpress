package captcha

import "time"

// This file holds the policy half of the store: how often a client has failed
// and therefore which challenge it deserves. It is split from the state machine
// in captcha.go because the two answer different questions — "what is a valid
// answer" versus "how hard should the next question be" — and because keeping
// them together pushed one file past the repository's size ceiling.

// failureRetention bounds how long a failure streak colours the policy.
const failureRetention = 30 * time.Minute

// KindFor reports which challenge this client must solve next, based on how
// often it has failed recently.
func (s *Store) KindFor(client string) Kind {
	if s == nil {
		return KindImage
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.fails[client]; ok && f.count >= s.slideAfter {
		return KindSlide
	}
	return KindImage
}

// NoteFailure records one failed login for the client and returns the new
// count. It is called for both a wrong password and a wrong captcha.
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

// reapLocked drops expired challenges and stale failure counters. Caller holds
// s.mu. It runs opportunistically on every issue: there is no background
// goroutine for this by design (the thumbnail worker is the only one the site
// runs), and issue is the only operation that grows the map.
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
}
