package clientip

// Spec 23 / TODO P2-6: X-Forwarded-For is only believed when the request
// actually arrived from a configured proxy.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func withRemote(t *testing.T, remote string) *http.Request {
	t.Helper()
	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	r.RemoteAddr = remote
	return r
}

func TestForgedHeaderFromUntrustedPeerIsIgnored(t *testing.T) {
	if err := Init(""); err != nil {
		t.Fatalf("init defaults: %v", err)
	}
	t.Cleanup(func() { _ = Init("") })

	r := withRemote(t, "203.0.113.9:54321")
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if got := Client(r); got != "203.0.113.9" {
		t.Fatalf("untrusted peer must be measured by RemoteAddr, got %q", got)
	}

	// Same for the Cloudflare header — it is also forgeable.
	r2 := withRemote(t, "203.0.113.9:54321")
	r2.Header.Set("CF-Connecting-IP", "2.2.2.2")
	if got := Client(r2); got != "203.0.113.9" {
		t.Fatalf("untrusted peer must ignore CF-Connecting-IP, got %q", got)
	}
}

func TestHeaderFromTrustedLoopbackProxyIsBelieved(t *testing.T) {
	if err := Init(""); err != nil { // empty -> loopback only
		t.Fatalf("init defaults: %v", err)
	}
	t.Cleanup(func() { _ = Init("") })

	// Single hop: the only XFF entry is the client.
	r := withRemote(t, "127.0.0.1:40000")
	r.Header.Set("X-Forwarded-For", "198.51.100.7")
	if got := Client(r); got != "198.51.100.7" {
		t.Fatalf("single XFF hop expected, got %q", got)
	}

	v6 := withRemote(t, "[::1]:40000")
	v6.Header.Set("X-Forwarded-For", "198.51.100.8")
	if got := Client(v6); got != "198.51.100.8" {
		t.Fatalf("ipv6 loopback proxy should be trusted, got %q", got)
	}

	// Multiple hops with no trusted proxy in the chain: the RIGHT-most entry
	// is the one our proxy actually saw, so that is the client.
	multi := withRemote(t, "127.0.0.1:40000")
	multi.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.5")
	if got := Client(multi); got != "10.0.0.5" {
		t.Fatalf("right-most XFF hop expected, got %q", got)
	}
}

// R3-04: with the first-hop rule, a client that sent its own
// "X-Forwarded-For: 1.2.3.4" had that value believed when nginx appended the
// real address after it ($proxy_add_x_forwarded_for). Every IP-keyed decision
// — rate limiting, login lockout, the audited IP — could then be aimed at
// somebody else's address. The chain is now read right-to-left.
func TestForgedPrefixCannotSteerTheMeasuredIP(t *testing.T) {
	if err := Init(""); err != nil {
		t.Fatalf("init defaults: %v", err)
	}
	t.Cleanup(func() { _ = Init("") })

	r := withRemote(t, "127.0.0.1:40000")
	// Attacker picks a victim address; nginx appends the true peer after it.
	r.Header.Set("X-Forwarded-For", "203.0.113.77, 198.51.100.23")
	if got := Client(r); got != "198.51.100.23" {
		t.Fatalf("forged prefix must be ignored, got %q", got)
	}

	// Same shape with several forged hops in front.
	r2 := withRemote(t, "127.0.0.1:40000")
	r2.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2, 3.3.3.3, 198.51.100.24")
	if got := Client(r2); got != "198.51.100.24" {
		t.Fatalf("right-most hop must win over forged prefixes, got %q", got)
	}
}

// A trusted proxy that appends itself stays in the chain: the walk skips it
// and reports the address it was forwarding for.
func TestTrustedProxyHopsAreSkipped(t *testing.T) {
	if err := Init("10.0.0.0/8"); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Cleanup(func() { _ = Init("") })

	// client -> 10.0.0.5 (trusted CDN) -> 10.0.0.9 (trusted nginx) -> us
	r := withRemote(t, "10.0.0.9:40000")
	r.Header.Set("X-Forwarded-For", "198.51.100.30, 10.0.0.5")
	if got := Client(r); got != "198.51.100.30" {
		t.Fatalf("trusted hops should be skipped, got %q", got)
	}

	// When the CDN is NOT in the trusted set we stop there rather than
	// trusting a value we cannot verify.
	if err := Init("10.0.0.9/32"); err != nil {
		t.Fatalf("init: %v", err)
	}
	r2 := withRemote(t, "10.0.0.9:40000")
	r2.Header.Set("X-Forwarded-For", "198.51.100.31, 10.0.0.5")
	if got := Client(r2); got != "10.0.0.5" {
		t.Fatalf("untrusted hop should stop the walk, got %q", got)
	}
}

func TestCustomTrustedCIDR(t *testing.T) {
	if err := Init("10.0.0.0/8, 192.168.1.1"); err != nil {
		t.Fatalf("init custom: %v", err)
	}
	t.Cleanup(func() { _ = Init("") })

	r := withRemote(t, "10.1.2.3:1234")
	r.Header.Set("X-Forwarded-For", "198.51.100.20")
	if got := Client(r); got != "198.51.100.20" {
		t.Fatalf("10/8 proxy should be trusted, got %q", got)
	}

	// A single-IP entry (no /prefix) is normalised to a /32.
	one := withRemote(t, "192.168.1.1:1234")
	one.Header.Set("X-Forwarded-For", "198.51.100.21")
	if got := Client(one); got != "198.51.100.21" {
		t.Fatalf("exact-IP entry should be trusted, got %q", got)
	}

	// Loopback is NOT in this explicit list, so it must not be trusted.
	loop := withRemote(t, "127.0.0.1:1234")
	loop.Header.Set("X-Forwarded-For", "198.51.100.22")
	if got := Client(loop); got != "127.0.0.1" {
		t.Fatalf("unlisted loopback must not be trusted, got %q", got)
	}
}

func TestInitRejectsGarbage(t *testing.T) {
	t.Cleanup(func() { _ = Init("") })
	if err := Init("not-an-ip"); err == nil {
		t.Fatalf("expected an error for an unparsable entry")
	}
}

func TestHostOnly(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4:5678":    "1.2.3.4",
		"[2001:db8::1]:4": "2001:db8::1",
		"1.2.3.4":         "1.2.3.4",
		"":                "",
	}
	for in, want := range cases {
		if got := hostOnly(in); got != want {
			t.Fatalf("hostOnly(%q) = %q, want %q", in, got, want)
		}
	}
}

// The limiter keys its buckets on the string this function returns, so two
// spellings of one address are two buckets. A dual-stack listener (the default
// on this stack) hands the same peer to us as 127.0.0.1 on one connection and
// ::ffff:127.0.0.1 on the next, and an attacker can alternate between them for
// twice the attempts at no cost. IPv6 case and compression add more spellings
// on top. Everything below must collapse to a single key.
func TestClientCanonicalisesAddressSpellings(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1":          "127.0.0.1",
		"  127.0.0.1  ":      "127.0.0.1",
		"::ffff:127.0.0.1":   "127.0.0.1", // IPv4-mapped over a dual-stack socket
		"::FFFF:127.0.0.1":   "127.0.0.1",
		"::1":                "::1",
		"0:0:0:0:0:0:0:1":    "::1",
		"2001:0DB8::1":       "2001:db8::1",
		"2001:db8::1":        "2001:db8::1",
		"2001:0db8:0000::1":  "2001:db8::1",
		"fe80::1%eth0":       "fe80::1", // zone is not identity
		"[::1]":              "::1",
		"203.0.113.7":        "203.0.113.7",
	}
	for in, want := range cases {
		if got := canonical(in); got != want {
			t.Errorf("canonical(%q) = %q, want %q", in, got, want)
		}
	}

	// An unparseable value survives (trimmed) rather than becoming a new
	// identity: mangling it would put a different lie in the audit trail.
	if got := canonical("not-an-ip"); got != "not-an-ip" {
		t.Errorf("unparseable address should pass through, got %q", got)
	}
	if got := canonical(""); got != "" {
		t.Errorf("empty address should stay empty, got %q", got)
	}
}

// End to end: the same peer reached over two socket families must produce one
// key, not two.
func TestClientIsStableAcrossSocketFamilies(t *testing.T) {
	mk := func(remote string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		return r
	}
	v4 := Client(mk("127.0.0.1:5001"))
	v4mapped := Client(mk("[::ffff:127.0.0.1]:5002"))
	v6 := Client(mk("[::1]:5003"))

	if v4 != v4mapped {
		t.Errorf("IPv4 and IPv4-mapped IPv6 must be one bucket: %q vs %q", v4, v4mapped)
	}
	if v4 == v6 {
		t.Errorf("IPv4 and IPv6 loopback are different peers and must not collide")
	}
}
