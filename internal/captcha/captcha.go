// Package captcha issues and verifies the login challenge.
//
// Two kinds exist, and which one a caller gets is not their choice: it is
// decided by how many times that client has already failed to log in. A
// first-time visitor solves a four-character image, an attacker who keeps
// guessing gets the sliding puzzle. Making the kind depend on failure count
// (rather than on a fixed step number) is deliberate — a step counter is
// defeated by rotating usernames, because every fresh account restarts at step
// one and never escalates.
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

// Kind names the interaction the frontend must render.
type Kind string

const (
	// KindImage is a four-character picture: letters and digits, no look-alikes
	// (0/O, 1/I/l) so a human does not fail on fonts.
	KindImage Kind = "image"
	// KindSlide is a puzzle piece the user drags into a gap.
	KindSlide Kind = "slide"
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
	ErrBadKind  = errors.New("captcha: answer does not match the challenge kind")
	ErrNotReady = errors.New("captcha: store not initialised")

	defaultSlide = 3
)

// Challenge is what the browser receives. The answer never appears here: for
// the image kind only the rendered picture is sent, and for the slide kind only
// the piece's vertical offset — the horizontal target is exactly what the user
// has to work out.
type Challenge struct {
	ID        string `json:"id"`
	Kind      Kind   `json:"kind"`
	Image     string `json:"image,omitempty"`
	Bg        string `json:"bg,omitempty"`
	Tile      string `json:"tile,omitempty"`
	TileY     int    `json:"tile_y,omitempty"`
	TileSize  int    `json:"tile_size,omitempty"`
	ExpiresIn int    `json:"expires_in"`
}

// Answer is what the browser sends back alongside the credentials.
type Answer struct {
	// Code is the four characters the user typed (image kind).
	Code string
	// X is where the user dropped the piece, in per-mille of the image width
	// (0..1000), not in CSS pixels. The form is responsive and the picture is
	// scaled on narrow screens, so a pixel offset measured in the browser would
	// disagree with the server by the scale factor.
	X int
}

// entry is one outstanding challenge. tries is per-challenge, so guessing the
// four characters costs a fresh challenge each time and the stored answer is
// never worth brute-forcing at 32^4 with a handful of attempts.
type entry struct {
	kind    Kind
	answer  string
	x       int
	y       int
	size    int
	expires time.Time
	tries   int
}

// failure counts stale login failures per client so the policy can escalate.
type failure struct {
	count int
	at    time.Time
}

// Store holds outstanding challenges and the per-client failure history.
type Store struct {
	mu         sync.Mutex
	items      map[string]*entry
	fails      map[string]*failure
	ttl        time.Duration
	slideAfter int
	maxTries   int
	now        func() time.Time
}

// New returns a Store. ttl bounds how long a challenge stays solvable;
// slideAfter is the failure count at which the slide replaces the image.
func New(ttl time.Duration, slideAfter int) *Store {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	if slideAfter <= 0 {
		slideAfter = defaultSlide
	}
	return &Store{
		items:      make(map[string]*entry),
		fails:      make(map[string]*failure),
		ttl:        ttl,
		slideAfter: slideAfter,
		maxTries:   5,
		now:        time.Now,
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

// Issue creates a fresh challenge of the kind this client currently deserves.
func (s *Store) Issue(client string) (Challenge, error) {
	return s.issue(client, "")
}

// IssueWithAnswerForTest creates an image challenge whose answer the caller
// chooses, so tests in other packages can solve it.
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
	kind := s.KindFor(client)
	if fixedAnswer != "" {
		// A fixed answer only makes sense for the picture kind.
		kind = KindImage
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := newID()
	if err != nil {
		return Challenge{}, err
	}
	e := &entry{kind: kind, expires: s.now().Add(s.ttl)}
	c := Challenge{ID: id, Kind: kind, ExpiresIn: int(s.ttl.Seconds())}

	switch kind {
	case KindSlide:
		ch, x, y, size, err := newSlide()
		if err != nil {
			return Challenge{}, err
		}
		e.x, e.y, e.size = x, y, size
		c.Bg, c.Tile, c.TileY, c.TileSize = ch.bg, ch.tile, y, size
	default:
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
		e.answer = code
		c.Image = b64
	}

	s.items[id] = e
	s.reapLocked()
	return c, nil
}

// Verify consumes the challenge and reports whether the answer matches.
//
// A challenge is single-use whether it succeeds or runs out of attempts: on
// success because the answer must not be replayable, on failure because leaving
// it alive lets one challenge be guessed indefinitely.
func (s *Store) Verify(id string, a Answer) error {
	if s == nil {
		return ErrNotReady
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
	e.tries++
	if e.tries > s.maxTries {
		delete(s.items, id)
		return ErrTooMany
	}

	switch e.kind {
	case KindSlide:
		if a.X <= 0 || a.X > slideXScale {
			return ErrWrong
		}
		// Convert the reported per-mille into the image's own coordinates, then
		// compare against the notch with a tolerance. The tolerance is what
		// makes the puzzle solvable by a human: nobody lands within one pixel of
		// a target they only saw dimmed behind noise.
		px := a.X * slideWidth / slideXScale
		if abs(px-e.x) > slideTolerance {
			if e.tries >= s.maxTries {
				delete(s.items, id)
			}
			return ErrWrong
		}
	case KindImage:
		if a.Code == "" {
			return ErrBadKind
		}
		if !equalFoldASCII(a.Code, e.answer) {
			if e.tries >= s.maxTries {
				delete(s.items, id)
			}
			return ErrWrong
		}
	default:
		delete(s.items, id)
		return ErrBadKind
	}

	delete(s.items, id)
	return nil
}
