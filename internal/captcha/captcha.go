// Package captcha issues and verifies the login challenge.
//
// One kind only: a four-character picture. An earlier version escalated to a
// sliding puzzle after repeated failures; it measured well on paper and badly
// in practice — the widget was unfamiliar and hard to operate, and a captcha
// nobody can solve protects nothing, because the operator switches it off.
// The defences that actually matter here are cheap and invisible: the answer
// is never sent, each picture authorises exactly one attempt, a challenge is
// bound to the client that asked for it, and issuing is rate limited.
//
// State is in-process only. The site runs one binary against SQLite and has no
// Redis; a captcha lives for two minutes, so a restart merely costs the user one
// extra challenge. Nothing here is worth a table.
package captcha

import (
	"errors"
	"sync"
	"time"
)

// Kind names the interaction the frontend must render. There is exactly one
// value now; it is kept in the payload so an older frontend keeps working.
type Kind string

const (
	// KindImage is a four-character picture: letters and digits, no look-alikes
	// (0/O, 1/I/l) so a human does not fail on fonts.
	KindImage Kind = "image"
)

// Verification errors. Callers map these to a message and, importantly, treat
// every one of them as a login failure — a wrong captcha must cost the attacker
// the same thing a wrong password does, otherwise the captcha is a free oracle
// for testing passwords.
var (
	ErrNotFound = errors.New("captcha: challenge not found or already used")
	ErrExpired  = errors.New("captcha: challenge expired")
	ErrWrong    = errors.New("captcha: answer incorrect")
	ErrTooMany  = errors.New("captcha: too many attempts on this challenge")
	ErrBadKind  = errors.New("captcha: empty answer")
	ErrNotReady = errors.New("captcha: store not initialised")
	// ErrRateLimited means this client has asked for more pictures than the
	// window allows. It is the only error a caller should surface as 429.
	ErrRateLimited = errors.New("captcha: issuing too fast")
)

// Challenge is what the browser receives. The answer never appears here — only
// the rendered picture — and there is no field an attacker could use to derive
// it: the id is 128 random bits and refers to state held on the server.
type Challenge struct {
	ID        string `json:"id"`
	Kind      Kind   `json:"kind"`
	Image     string `json:"image"`
	ExpiresIn int    `json:"expires_in"`
}

// Answer is what the browser sends back alongside the credentials.
type Answer struct {
	// Code is the four characters the user typed.
	Code string
}

// entry is one outstanding challenge.
//
// client is the address the challenge was issued to. Binding it costs nothing
// and closes the resale path: an answer solved on a clean machine cannot be
// spent from the attacker's, so a captcha farm has to feed the same address
// that is doing the guessing. tries is per-challenge, so a wrong answer
// invalidates the value of the picture rather than the picture itself.
type entry struct {
	client  string
	answer  string
	expires time.Time
	tries   int
}

// failure counts stale login failures per client. It no longer selects a
// harder challenge — it tightens the issuing window instead.
type failure struct {
	count int
	at    time.Time
}

// Store holds outstanding challenges and the per-client bookkeeping.
type Store struct {
	mu     sync.Mutex
	items  map[string]*entry
	fails  map[string]*failure
	issues map[string]*issueWindow
	ttl    time.Duration
	maxTry int
	now    func() time.Time
}

// New returns a Store. ttl bounds how long a challenge stays solvable.
func New(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &Store{
		items:  make(map[string]*entry),
		fails:  make(map[string]*failure),
		issues: make(map[string]*issueWindow),
		ttl:    ttl,
		maxTry: maxTries,
		now:    time.Now,
	}
}

// defaultStore is the process-wide instance wired by Init.
var (
	defaultMu    sync.RWMutex
	defaultStore *Store
)

// Init installs the process-wide store. Called once from cmd/server.
func Init(s *Store) {
	defaultMu.Lock()
	defaultStore = s
	defaultMu.Unlock()
}

// Default returns the process-wide store (nil before Init).
func Default() *Store {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return defaultStore
}

// Issue creates a fresh challenge for this client.
func (s *Store) Issue(client string) (Challenge, error) {
	return s.issue(client, "")
}

// IssueWithAnswerForTest creates a challenge whose answer the caller chooses,
// so tests in other packages can solve it.
//
// It is not a hole: nothing in the router can reach it, and it grants the
// ability to *set* an answer, never to read one. The honest alternatives were
// worse — exporting a reader would leak live answers to any future caller, and
// a package-internal test cannot exercise the handler that lives in
// internal/handlers. The name carries ForTest for the next reader.
func (s *Store) IssueWithAnswerForTest(client, answer string) (Challenge, error) {
	if answer == "" {
		return Challenge{}, ErrWrong
	}
	return s.issue(client, answer)
}

func (s *Store) issue(client, fixedAnswer string) (Challenge, error) {
	if s == nil {
		return Challenge{}, ErrNotReady
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.allowIssueLocked(client) {
		return Challenge{}, ErrRateLimited
	}

	id, err := newID()
	if err != nil {
		return Challenge{}, err
	}

	var b64, code string
	if fixedAnswer != "" {
		code = fixedAnswer
		b64, err = newImageWith(code)
	} else {
		b64, code, err = newImage()
	}
	if err != nil {
		return Challenge{}, err
	}

	s.items[id] = &entry{
		client:  client,
		answer:  code,
		expires: s.now().Add(s.ttl),
	}
	s.reapLocked()

	return Challenge{
		ID:        id,
		Kind:      KindImage,
		Image:     b64,
		ExpiresIn: int(s.ttl.Seconds()),
	}, nil
}

// Verify consumes the challenge and reports whether the answer matches.
//
// A challenge is single-use whether it succeeds or runs out of attempts: on
// success because the answer must not be replayable, on failure because leaving
// it alive lets one picture be guessed indefinitely.
//
// client must be the address the challenge was issued to. A mismatch is
// rejected without touching tries: otherwise anyone who guessed an id could
// burn a stranger's attempts and lock them out of their own login.
func (s *Store) Verify(client, id string, a Answer) error {
	if s == nil {
		return ErrNotReady
	}
	if id == "" {
		return ErrNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}
	if s.now().After(e.expires) {
		delete(s.items, id)
		return ErrExpired
	}
	if e.client != client {
		return ErrWrong
	}
	// An empty field is what the browser sends when someone presses Sign in
	// before typing. It is not a guess, so it must not eat the attempt budget —
	// but it must not be treated as a correct answer either.
	if a.Code == "" {
		return ErrBadKind
	}

	e.tries++
	if e.tries > s.maxTry {
		delete(s.items, id)
		return ErrTooMany
	}
	if !equalFoldASCII(a.Code, e.answer) {
		if e.tries >= s.maxTry {
			delete(s.items, id)
		}
		return ErrWrong
	}

	delete(s.items, id)
	return nil
}
