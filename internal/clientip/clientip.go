// Package clientip resolves the originating client IP of an HTTP request
//
// The naive approach — "always trust X-Forwarded-For" — lets any client forge
// its IP by sending the header itself, which defeats every IP-based decision
// (rate limiting, login lockout, view de-duplication). This package only
// honours the header when the request actually came from a configured proxy.
//
// Trust rules:
//   - TRUSTED_PROXIES holds a comma-separated list of IPs / CIDRs.
//   - When the list is empty we trust loopback only (127.0.0.1/32, ::1/128),
//     which covers the most common deployment: nginx on the same host.
//   - If remoteAddr's IP is in the trusted set, the X-Forwarded-For chain is
//     walked from the RIGHT and the first address that is not itself a
//     trusted proxy is the real client; otherwise the header is ignored.
//
// Configuration is process-global and set once at startup (main -> Init).
// Tests that need a different trust set call Init explicitly.
package clientip

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
)

var (
	mu      sync.RWMutex
	trusted []*net.IPNet
)

// DefaultTrustedProxySpec is used when TRUSTED_PROXIES is unset/empty.
const DefaultTrustedProxySpec = "127.0.0.1/32,::1/128"

// Init parses a comma-separated list of IPs/CIDRs into the trusted set.
// An empty spec falls back to DefaultTrustedProxySpec. It returns an error
// for an unparsable entry so a typo fails fast at startup instead of
// silently trusting nothing (which would break IP logging in production).
func Init(spec string) error {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = DefaultTrustedProxySpec
	}
	nets := make([]*net.IPNet, 0, 4)
	for _, raw := range strings.Split(spec, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			if ip := net.ParseIP(entry); ip != nil {
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				entry = fmt.Sprintf("%s/%d", ip.String(), bits)
			}
		}
		_, ipnet, err := net.ParseCIDR(entry)
		if err != nil {
			return fmt.Errorf("invalid trusted proxy %q: %w", raw, err)
		}
		nets = append(nets, ipnet)
	}
	mu.Lock()
	trusted = nets
	mu.Unlock()
	return nil
}

// IsTrustedProxy reports whether the given IP belongs to the trusted set.
func IsTrustedProxy(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, n := range trusted {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// Client returns the best-effort originating client IP for r.
//
// Precedence: the right-most untrusted X-Forwarded-For hop > CF-Connecting-IP
// > RemoteAddr, but the proxy headers are only consulted when RemoteAddr is a
// trusted proxy. A forged header from an untrusted peer is therefore ignored.
//
// The chain is walked from the RIGHT, not the left. The usual nginx spelling
// — proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for — APPENDS the
// address nginx actually saw, so the real client ends up last while the
// left-most entry is whatever the client itself chose to send. Reading the
// first hop therefore let anyone name their own IP, which defeated every
// IP-keyed decision in the process: the general rate limiter, the login IP
// bucket, and the IP recorded in the audit trail (R3-04 demonstrated 520
// login attempts in 4.4 s by rotating the forged prefix). Taking the
// right-most address that is not one of our own proxies makes a forged prefix
// inert — an attacker can only push values further left, never into the slot
// we read.
func Client(r *http.Request) string {
	return canonical(ClientRaw(r))
}

// ClientRaw is Client without the canonicalisation. Only useful for callers
// that want to echo the address exactly as presented; every limiter and audit
// row must use Client.
func ClientRaw(r *http.Request) string {
	remote := hostOnly(r.RemoteAddr)
	if !IsTrustedProxy(remote) {
		return remote
	}
	xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if xff == "" {
		if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
			return cf
		}
		return remote
	}
	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if IsTrustedProxy(ip) {
			continue // one of our proxies appended itself; keep walking left
		}
		return ip
	}
	// Every hop was a trusted proxy (or the list was malformed): fall back to
	// the peer we actually talked to.
	return remote
}

// canonical normalises an address to one spelling, so every rate-limit bucket
// and audit row agrees on who a client is.
//
// Without this, the SAME client presents as several different keys depending on
// how the address is written, and each spelling gets its own bucket:
//
//	127.0.0.1          loopback over IPv4
//	::ffff:127.0.0.1   the same peer seen by a dual-stack listener
//	::1                loopback over IPv6
//	2001:0DB8::1  vs  2001:db8::1   the same host, different case
//
// A dual-stack `:8080` listener is the default on this stack, so an attacker can
// alternate between v4 and v4-mapped-v6 for free and get twice the attempts —
// and rotating the hex case of an IPv6 address multiplies that further. The
// fix is one line, and it is the difference between "the limiter counts per
// client" and "the limiter counts per spelling".
//
// A value that does not parse is returned trimmed but unchanged: it can only
// come from a header we chose to trust, and mangling it would put a different
// lie in the audit trail than the one the peer told.
func canonical(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	// Strip an IPv6 zone ("fe80::1%eth0") before parsing: ParseIP rejects it,
	// and the zone is not part of the client's identity anyway.
	if i := strings.IndexByte(addr, '%'); i > 0 {
		addr = addr[:i]
	}
	// Accept a bracketed form too ("[::1]"), which some proxies emit.
	addr = strings.TrimSuffix(strings.TrimPrefix(addr, "["), "]")
	if ip := net.ParseIP(addr); ip != nil {
		// String() renders an IPv4-mapped IPv6 address as plain dotted-quad,
		// which is exactly the collapse we want; it also lower-cases and
		// compresses IPv6 per RFC 5952.
		return ip.String()
	}
	return addr
}

// hostOnly strips the port from a net/http RemoteAddr value.
func hostOnly(addr string) string {
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	// Bare IP without a port (also the case for unbracketed IPv6).
	return strings.Trim(addr, "[]")
}
