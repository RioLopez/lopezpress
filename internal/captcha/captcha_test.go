package captcha

import (
	"strings"
	"testing"
	"time"
)

func testStore() *Store { return New(2*time.Minute, 3) }

// answerOf reads the private answer of a live challenge. Tests live in the
// same package precisely so they can do this: there is no public API for
// "what was the answer", and adding one for testing would be a real leak.
func answerOf(t *testing.T, s *Store, id string) *entry {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.items[id]
	if !ok {
		t.Fatalf("challenge %q not found in store", id)
	}
	return e
}

func TestImageChallengeIsSolvableOnce(t *testing.T) {
	s := testStore()
	ch, err := s.Issue("203.0.113.1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ch.Kind != KindImage {
		t.Fatalf("a clean client must get the image kind, got %q", ch.Kind)
	}
	if !strings.HasPrefix(ch.Image, "data:image/png;base64,") {
		t.Fatalf("image must be a png data URI, got %.40q", ch.Image)
	}
	// The answer must never travel to the browser.
	if ch.Tile != "" || ch.Bg != "" {
		t.Fatalf("image challenge leaked slide fields: %+v", ch)
	}

	code := answerOf(t, s, ch.ID).answer
	if err := s.Verify(ch.ID, Answer{Code: code}); err != nil {
		t.Fatalf("correct answer rejected: %v", err)
	}

	// Replaying the same answer must fail: a challenge is consumed on success,
	// otherwise a single solved picture authorises unlimited logins.
	if err := s.Verify(ch.ID, Answer{Code: code}); err != ErrNotFound {
		t.Fatalf("replay must fail with ErrNotFound, got %v", err)
	}
}

func TestImageAnswerIgnoresCase(t *testing.T) {
	s := testStore()
	ch, err := s.Issue("203.0.113.2")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	code := answerOf(t, s, ch.ID).answer

	// Users type whatever case they see; failing them for that is hostile.
	if err := s.Verify(ch.ID, Answer{Code: strings.ToLower(code)}); err != nil {
		t.Fatalf("lower-cased answer rejected: %v", err)
	}
}

func TestWrongAnswerIsCountedAndCapped(t *testing.T) {
	s := testStore()
	ch, err := s.Issue("203.0.113.3")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// maxTries wrong guesses are tolerated (each one is a real attempt by a
	// human who misread the picture), after which the challenge dies so the
	// four characters cannot be brute-forced on one image.
	for i := 0; i < s.maxTries; i++ {
		if err := s.Verify(ch.ID, Answer{Code: "ZZZZ"}); err != ErrWrong {
			t.Fatalf("attempt %d: want ErrWrong, got %v", i+1, err)
		}
	}
	if err := s.Verify(ch.ID, Answer{Code: "ZZZZ"}); err != ErrNotFound {
		t.Fatalf("challenge must be dropped after maxTries, got %v", err)
	}
}

func TestExpiredChallengeFails(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	ch, err := s.Issue("203.0.113.4")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	code := answerOf(t, s, ch.ID).answer

	s.Reset(func() time.Time { return base.Add(3 * time.Minute) })
	if err := s.Verify(ch.ID, Answer{Code: code}); err != ErrExpired {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

func TestPolicyEscalatesToSlide(t *testing.T) {
	s := testStore()
	const ip = "203.0.113.5"

	// Below the threshold the cheap challenge is used.
	for i := 0; i < 3; i++ {
		ch, err := s.Issue(ip)
		if err != nil {
			t.Fatalf("issue: %v", err)
		}
		if ch.Kind != KindImage {
			t.Fatalf("after %d failures want image, got %q", i, ch.Kind)
		}
		s.NoteFailure(ip)
	}

	// At the threshold it escalates.
	ch, err := s.Issue(ip)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ch.Kind != KindSlide {
		t.Fatalf("after 3 failures want slide, got %q", ch.Kind)
	}

	// A successful login resets the streak, so one solved slide is enough to
	// get back to the easy path.
	s.NoteSuccess(ip)
	back, err := s.Issue(ip)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if back.Kind != KindImage {
		t.Fatalf("after success want image again, got %q", back.Kind)
	}
}

func TestSlideAcceptsNearMissAndRejectsFarDrop(t *testing.T) {
	s := testStore()
	const ip = "203.0.113.6"
	for i := 0; i < 3; i++ {
		s.NoteFailure(ip)
	}

	ch, err := s.Issue(ip)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ch.Kind != KindSlide {
		t.Fatalf("want slide, got %q", ch.Kind)
	}
	if ch.Bg == "" || ch.Tile == "" {
		t.Fatalf("slide must carry both images: %+v", ch)
	}
	// The notch's x is the answer and must not be in the payload.
	if ch.TileSize == 0 || ch.TileY == 0 {
		t.Fatalf("slide must report tile geometry: %+v", ch)
	}

	e := answerOf(t, s, ch.ID)
	target := e.x * slideXScale / slideWidth

	// A drop far from the notch fails.
	if err := s.Verify(ch.ID, Answer{X: clampMilli(target + 120)}); err != ErrWrong {
		t.Fatalf("far drop must fail, got %v", err)
	}
	// A drop within tolerance passes.
	if err := s.Verify(ch.ID, Answer{X: target}); err != nil {
		t.Fatalf("on-target drop rejected: %v", err)
	}
}

func TestSlideRejectsOutOfRangePosition(t *testing.T) {
	s := testStore()
	const ip = "203.0.113.7"
	for i := 0; i < 3; i++ {
		s.NoteFailure(ip)
	}
	ch, err := s.Issue(ip)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// Zero and out-of-range values are what a script sends when it does not
	// understand the widget; they must not accidentally land inside tolerance.
	for _, x := range []int{0, -5, slideXScale + 1, 99999} {
		if err := s.Verify(ch.ID, Answer{X: x}); err != ErrWrong {
			t.Fatalf("x=%d must be rejected, got %v", x, err)
		}
	}
}

func TestNilStoreIsSafe(t *testing.T) {
	var s *Store
	if k := s.KindFor("x"); k != KindImage {
		t.Fatalf("nil store must degrade to image, got %q", k)
	}
	if s.NoteFailure("x") != 0 {
		t.Fatal("nil store must not count")
	}
	s.NoteSuccess("x") // must not panic
	if _, err := s.Issue("x"); err != ErrNotReady {
		t.Fatalf("nil store must report ErrNotReady, got %v", err)
	}
	if err := s.Verify("id", Answer{}); err != ErrNotReady {
		t.Fatalf("nil store verify must report ErrNotReady, got %v", err)
	}
}

func TestEmptyIDIsNotFound(t *testing.T) {
	s := testStore()
	if err := s.Verify("", Answer{Code: "AAAA"}); err != ErrNotFound {
		t.Fatalf("empty id must be ErrNotFound, got %v", err)
	}
}

func TestReapDropsExpiredChallenges(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	if _, err := s.Issue("203.0.113.8"); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if s.Len() != 1 {
		t.Fatalf("want 1 live challenge, got %d", s.Len())
	}

	// Issuing later reaps on the way through; there is no background goroutine
	// for this by design.
	s.Reset(func() time.Time { return base.Add(5 * time.Minute) })
	if _, err := s.Issue("203.0.113.8"); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if s.Len() != 1 {
		t.Fatalf("expired challenge should have been reaped, got %d", s.Len())
	}
}

// clampMilli keeps a per-mille value inside the range the frontend can send.
func clampMilli(v int) int {
	if v < 1 {
		return 1
	}
	if v > slideXScale {
		return slideXScale
	}
	return v
}
