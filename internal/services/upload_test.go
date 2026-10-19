package services

import (
	"context"
	"testing"
)

// TestUploadToSmmsHonorsContext is the regression test for as-built audit
// finding Q11: uploadToSmms used http.DefaultClient with no timeout, so a hung
// SM.MS remote would pin a goroutine (and the enclosing UploadImage handler)
// indefinitely on a low-core VPS. The fix binds the call to the request
// context and gives it a hard 10s client timeout. This test proves the context
// is wired through: a cancelled context must fail fast (no network attempt,
// no hang) rather than dialing sm.ms.
func TestUploadToSmmsHonorsContext(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call is made

	// The call must return immediately with an error — it must NOT attempt to
	// reach sm.ms (which would hang or fail non-deterministically).
	if _, err := svc.uploadToSmms(ctx, "x.png", "image/png", []byte("data"), "tok"); err == nil {
		t.Fatal("expected error from cancelled context, got nil (context not wired through)")
	}
}
