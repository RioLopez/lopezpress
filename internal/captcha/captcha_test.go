package captcha

import (
	"strings"
	"testing"
	"time"
)

func testStore() *Store { return New(2 * time.Minute) }

const testClient = "203.0.113.1"

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

func TestChallengeIsSolvableOnce(t *testing.T) {
	s := testStore()
	ch, err := s.Issue(testClient)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if ch.Kind != KindImage {
		t.Fatalf("want the image kind, got %q", ch.Kind)
	}
	if !strings.HasPrefix(ch.Image, "data:image/png;base64,") {
		t.Fatalf("image must be a png data URI, got %.40q", ch.Image)
	}

	code := answerOf(t, s, ch.ID).answer
	if err := s.Verify(testClient, ch.ID, Answer{Code: code}); err != nil {
		t.Fatalf("correct answer rejected: %v", err)
	}

	// Replaying the same answer must fail: a challenge is consumed on success,
	// otherwise one solved picture authorises unlimited logins.
	if err := s.Verify(testClient, ch.ID, Answer{Code: code}); err != ErrNotFound {
		t.Fatalf("replay must fail with ErrNotFound, got %v", err)
	}
}

func TestAnswerIgnoresCase(t *testing.T) {
	s := testStore()
	ch, err := s.Issue(testClient)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	code := answerOf(t, s, ch.ID).answer

	// Users type whatever case they see; failing them for that is hostile.
	if err := s.Verify(testClient, ch.ID, Answer{Code: strings.ToLower(code)}); err != nil {
		t.Fatalf("lower-cased answer rejected: %v", err)
	}
}

// TestChallengeIsBoundToClient: a solved picture is worth nothing from another
// address. This is what makes a captcha farm useless — the answer has to be
// produced by the same client that is guessing.
func TestChallengeIsBoundToClient(t *testing.T) {
	s := testStore()
	ch, err := s.Issue("203.0.113.10")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	code := answerOf(t, s, ch.ID).answer

	if err := s.Verify("198.51.100.7", ch.ID, Answer{Code: code}); err != ErrWrong {
		t.Fatalf("answer from another client must be refused, got %v", err)
	}
	// The stranger's attempts must not have burnt the owner's budget.
	if err := s.Verify("203.0.113.10", ch.ID, Answer{Code: code}); err != nil {
		t.Fatalf("owner must still be able to solve it, got %v", err)
	}
}

func TestWrongAnswerIsCountedAndCapped(t *testing.T) {
	s := testStore()
	ch, err := s.Issue(testClient)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// A few wrong guesses are tolerated (each one is a real attempt by a human
	// who misread the picture), after which the challenge dies so the four
	// characters cannot be brute-forced on one image.
	for i := 0; i < s.MaxTries(); i++ {
		if err := s.Verify(testClient, ch.ID, Answer{Code: "ZZZZ"}); err != ErrWrong {
			t.Fatalf("attempt %d: want ErrWrong, got %v", i+1, err)
		}
	}
	if err := s.Verify(testClient, ch.ID, Answer{Code: "ZZZZ"}); err != ErrNotFound {
		t.Fatalf("challenge must be dropped after MaxTries, got %v", err)
	}
}

func TestEmptyAnswerIsRejectedWithoutConsumingATry(t *testing.T) {
	s := testStore()
	ch, err := s.Issue(testClient)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if err := s.Verify(testClient, ch.ID, Answer{}); err != ErrBadKind {
		t.Fatalf("empty answer must be ErrBadKind, got %v", err)
	}
	// An empty field is what the browser sends before the user types; it must
	// not eat the attempt budget.
	if got := answerOf(t, s, ch.ID).tries; got != 0 {
		t.Fatalf("empty answer must not consume a try, got %d", got)
	}
}

func TestExpiredChallengeFails(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	ch, err := s.Issue(testClient)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	code := answerOf(t, s, ch.ID).answer

	s.Reset(func() time.Time { return base.Add(3 * time.Minute) })
	if err := s.Verify(testClient, ch.ID, Answer{Code: code}); err != ErrExpired {
		t.Fatalf("want ErrExpired, got %v", err)
	}
}

// TestIssueIsRateLimited: the endpoint is anonymous, so without a ceiling it
// is also a free source of labelled training data for an OCR model.
func TestIssueIsRateLimited(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	for i := 0; i < issueBurst; i++ {
		if _, err := s.Issue(testClient); err != nil {
			t.Fatalf("issue %d of %d: %v", i+1, issueBurst, err)
		}
	}
	if _, err := s.Issue(testClient); err != ErrRateLimited {
		t.Fatalf("over-burst issue must be refused, got %v", err)
	}

	// Another address has its own bucket.
	if _, err := s.Issue("198.51.100.9"); err != nil {
		t.Fatalf("a different client must not be affected: %v", err)
	}

	// The window slides: one minute later the same client may ask again.
	s.Reset(func() time.Time { return base.Add(issueWindowLen + time.Second) })
	if _, err := s.Issue(testClient); err != nil {
		t.Fatalf("after the window the client must be served again: %v", err)
	}
}

// TestFailuresTightenTheAllowance: an address that is failing logins is by
// definition guessing, so it gets fewer pictures to work with.
func TestFailuresTightenTheAllowance(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	for i := 0; i < tightAfter; i++ {
		s.NoteFailure(testClient)
	}
	for i := 0; i < issueTight; i++ {
		if _, err := s.Issue(testClient); err != nil {
			t.Fatalf("issue %d under the tight allowance: %v", i+1, err)
		}
	}
	if _, err := s.Issue(testClient); err != ErrRateLimited {
		t.Fatalf("tight allowance must cut in, got %v", err)
	}

	// A successful login restores the generous allowance.
	s.NoteSuccess(testClient)
	if _, err := s.Issue(testClient); err != nil {
		t.Fatalf("after success the client must be served again: %v", err)
	}
}

// TestFailureRetentionLapses pins the other half of the tightening rule: it is
// temporary. A visitor who mistyped four characters must not stay throttled
// for the rest of their session — that is what turned "I read it wrong" into
// "the captcha is broken" and sent people into a retry loop.
func TestFailureRetentionLapses(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	for i := 0; i < tightAfter; i++ {
		s.NoteFailure(testClient)
	}
	// Confirm the tight allowance really is in force first.
	for i := 0; i < issueTight; i++ {
		if _, err := s.Issue(testClient); err != nil {
			t.Fatalf("issue %d under the tight allowance: %v", i+1, err)
		}
	}
	if _, err := s.Issue(testClient); err != ErrRateLimited {
		t.Fatalf("expected the tight allowance to cut in, got %v", err)
	}

	// Once the retention horizon passes, the streak is stale bookkeeping and
	// the generous allowance returns without a successful login.
	s.Reset(func() time.Time { return base.Add(failureRetention + time.Second) })
	for i := 0; i < issueBurst; i++ {
		if _, err := s.Issue(testClient); err != nil {
			t.Fatalf("issue %d after the retention lapsed: %v", i+1, err)
		}
	}
}

func TestNilStoreIsSafe(t *testing.T) {
	var s *Store
	if s.NoteFailure("x") != 0 {
		t.Fatal("nil store must not count")
	}
	s.NoteSuccess("x") // must not panic
	if _, err := s.Issue("x"); err != ErrNotReady {
		t.Fatalf("nil store must report ErrNotReady, got %v", err)
	}
	if err := s.Verify("x", "id", Answer{}); err != ErrNotReady {
		t.Fatalf("nil store verify must report ErrNotReady, got %v", err)
	}
}

func TestEmptyIDIsNotFound(t *testing.T) {
	s := testStore()
	if err := s.Verify(testClient, "", Answer{Code: "AAAA"}); err != ErrNotFound {
		t.Fatalf("empty id must be ErrNotFound, got %v", err)
	}
}

func TestReapDropsExpiredChallenges(t *testing.T) {
	s := testStore()
	base := time.Now()
	s.Reset(func() time.Time { return base })

	if _, err := s.Issue(testClient); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if s.Len() != 1 {
		t.Fatalf("want 1 live challenge, got %d", s.Len())
	}

	// Issuing later reaps on the way through; there is no background goroutine
	// for this by design.
	s.Reset(func() time.Time { return base.Add(5 * time.Minute) })
	if _, err := s.Issue(testClient); err != nil {
		t.Fatalf("issue: %v", err)
	}
	if s.Len() != 1 {
		t.Fatalf("expired challenge should have been reaped, got %d", s.Len())
	}
}
