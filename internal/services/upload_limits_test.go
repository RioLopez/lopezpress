package services

import (
	"testing"
)

// Upload rate limiting: a burst is allowed, the request past it is refused with
// a wait, and one account's burst does not touch another's.
//
// The site-wide limiter allows 20 requests/second per identity, which is no
// limit at all for uploads; these buckets are the thing that actually keeps one
// session from feeding the decoder continuously.
func TestAllowUploadBurstThenRefuse(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)

	for i := 0; i < uploadBurst; i++ {
		if wait, ok := svc.AllowUpload("user-1"); !ok {
			t.Fatalf("upload %d of the burst should be allowed (wait=%v)", i+1, wait)
		}
	}
	wait, ok := svc.AllowUpload("user-1")
	if ok {
		t.Fatalf("the upload past the burst must be refused")
	}
	if wait <= 0 {
		t.Fatalf("a refusal must say how long to wait, got %v", wait)
	}

	// Another account starts with its own full buckets.
	if _, ok := svc.AllowUpload("user-2"); !ok {
		t.Fatalf("one account's burst must not throttle another")
	}
	// Anonymous callers are not bucketed here — the route already requires a
	// session, and bucketing "" would put every unauthenticated caller in one
	// shared bucket.
	if _, ok := svc.AllowUpload(""); !ok {
		t.Fatalf("an empty id must not be bucketed")
	}
}

// A refusal must not spend the caller's allowance: hammering the endpoint while
// throttled should not push the recovery time further out.
func TestRefusedUploadsDoNotConsumeTokens(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)

	for i := 0; i < uploadBurst; i++ {
		svc.AllowUpload("hammer")
	}
	first, _ := svc.AllowUpload("hammer")
	for i := 0; i < 20; i++ {
		svc.AllowUpload("hammer")
	}
	last, ok := svc.AllowUpload("hammer")
	if ok {
		t.Fatalf("the bucket must still be empty after repeated refusals")
	}
	// The wait is bounded by one refill interval; repeated refusals must not
	// compound it into minutes.
	if last > first*2 {
		t.Fatalf("refusals must not consume tokens: first wait %v, after 20 more %v", first, last)
	}
}
