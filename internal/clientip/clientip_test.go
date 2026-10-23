package clientip

// Spec 23 / TODO P2-6: X-Forwarded-For is only believed when the request
// actually arrived from a configured proxy.

import (
	"net/http"
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

	r := withRemote(t, "127.0.0.1:40000")
	r.Header.Set("X-Forwarded-For", "198.51.100.7, 10.0.0.5")
	if got := Client(r); got != "198.51.100.7" {
		t.Fatalf("first XFF hop expected, got %q", got)
	}

	v6 := withRemote(t, "[::1]:40000")
	v6.Header.Set("X-Forwarded-For", "198.51.100.8")
	if got := Client(v6); got != "198.51.100.8" {
		t.Fatalf("ipv6 loopback proxy should be trusted, got %q", got)
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
