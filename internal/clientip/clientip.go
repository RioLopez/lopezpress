// Package clientip resolves the originating client IP of an HTTP request
// (spec 23 / TODO P2-6).
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
//   - If remoteAddr's IP is in the trusted set, the first entry of
//     X-Forwarded-For is the real client; otherwise the header is ignored.
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
// Precedence: X-Forwarded-For (first hop) > CF-Connecting-IP > RemoteAddr,
// but the proxy headers are only consulted when RemoteAddr is a trusted
// proxy. A forged header from an untrusted peer is therefore ignored.
func Client(r *http.Request) string {
	remote := hostOnly(r.RemoteAddr)
	if !IsTrustedProxy(remote) {
		return remote
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.IndexByte(xff, ','); idx >= 0 {
			if ip := strings.TrimSpace(xff[:idx]); ip != "" {
				return ip
			}
		} else if ip := strings.TrimSpace(xff); ip != "" {
			return ip
		}
	}
	if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
		return cf
	}
	return remote
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
