package imghost

import (
	"context"
	"testing"
)

// TestUploadHonorsContext is the regression test for as-built audit finding
// Q11: the SM.MS proxy used http.DefaultClient with no timeout, so a hung
// remote would pin a goroutine (and the enclosing upload handler)
// indefinitely on a low-core VPS. The fix binds the call to the request
// context and gives it a hard 10s client timeout. This test proves the context
// is wired through: a cancelled context must fail fast (no network attempt,
// no hang) rather than dialing sm.ms.
//
// Moved here from internal/services along with the code it covers (P1 of the
// 2026-10-23 architecture review: outbound HTTP is an adapter, not domain
// logic). The assertion is unchanged.
func TestUploadHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call is made

	if _, err := Upload(ctx, "x.png", []byte("data"), "tok"); err == nil {
		t.Fatal("expected error from cancelled context, got nil (context not wired through)")
	}
}

// TestUploadTimeoutIsBounded guards the anti-hang property that Q11 was about:
// the package must never fall back to a client without a deadline.
func TestUploadTimeoutIsBounded(t *testing.T) {
	if UploadTimeout <= 0 {
		t.Fatalf("UploadTimeout must be positive, got %v", UploadTimeout)
	}
	if client.Timeout != UploadTimeout {
		t.Fatalf("client.Timeout (%v) must equal UploadTimeout (%v)", client.Timeout, UploadTimeout)
	}
}
