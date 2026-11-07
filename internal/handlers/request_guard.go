package handlers

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// isJSONContentType reports whether the request body is declared as JSON.
//
// Login is the only write endpoint that accepted any Content-Type: the JSON
// decoder reads the bytes, not the header, so a cross-site page could POST a
// body with `Content-Type: text/plain` — one of the three CORS-safelisted
// values, which therefore needs no preflight — and a browser would send it
// without asking this server anything. Requiring application/json means a
// cross-origin form or fetch either fails the preflight or is refused here
// (F-01 / X-01).
func isJSONContentType(r *http.Request) bool {
	ct := strings.TrimSpace(r.Header.Get("Content-Type"))
	if ct == "" {
		return false
	}
	mt, _, err := mime.ParseMediaType(ct)
	return err == nil && mt == "application/json"
}

// sameOrigin reports whether a browser-supplied Origin belongs to this site.
//
// No Origin is allowed: curl, cron jobs and same-origin GETs do not send one,
// and "absent" is not evidence of a cross-site caller. When it is present the
// host must match the request's Host. The one deliberate exception is the
// loopback pair, because `scripts/dev.sh` serves the SPA on 127.0.0.1:5173 and
// proxies /api to 127.0.0.1:8080 with Host rewritten — same machine, different
// port. Any other host is refused before the password is read (F-01 / X-01).
func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		// Includes the opaque "null" origin sent by sandboxed frames/files.
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	return isLoopbackHost(u.Hostname()) && isLoopbackHost(bareHost(r.Host))
}

// isLoopbackHost matches localhost and any loopback literal (127.0.0.0/8, ::1).
func isLoopbackHost(h string) bool {
	h = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(strings.Trim(h, "[]"))
	return ip != nil && ip.IsLoopback()
}

// bareHost strips a port from Host/Hostname-style values.
func bareHost(hostport string) string {
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		return h
	}
	return strings.Trim(hostport, "[]")
}
