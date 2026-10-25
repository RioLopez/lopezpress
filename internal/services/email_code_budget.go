package services

import (
	"time"
)

// --- per-IP budget ---------------------------------------------------------
//
// The database cooldown is keyed by address, so it does nothing against a
// client that walks a list of addresses: each one looks new. This is the other
// half of the limit — how many messages one origin may trigger per hour.
//
// Split out of email_codes.go because the budget is a self-contained concern
// (an in-memory fixed window) that has nothing to do with minting, mailing or
// verifying a code — and because it was the piece that was silently disabled:
// the public routes never put an IP on the request context, so this function
// was called with "" and answered "allow" (R3-03 / R4-06).

const (
	// emailCodeIPLimit is deliberately small. A human needs two codes at most
	// (one typo, one resend); ten leaves room for a shared office NAT while
	// keeping a script from turning the VPS into a mail relay.
	emailCodeIPLimit = 10
	emailCodeIPWindow = time.Hour
)

// ipCodeWindow is a fixed-window counter. A sliding window would be more
// accurate, but the extra bookkeeping buys nothing here: the limit exists to
// stop bulk abuse, not to be fair to the last second of an hour.
type ipCodeWindow struct {
	start time.Time
	count int
}

// allowCodeFromIP consumes one unit of the caller's hourly budget.
//
// An empty IP is refused rather than waved through. It used to return true on
// the theory that "the database cooldown still applies", which was true and
// useless: the cooldown is keyed by address, so a client that walks a list of
// addresses looks new every time. R3-03/R4-06 exploited exactly that — 12
// different addresses, 12 mails sent, not one 429 — and the reason it worked
// is that the public routes never put an IP on the context at all, so this
// function was being called with "" every single time. The routes now inject
// the client IP (see handlers.WithClientIP); an empty value here therefore
// means something is genuinely wrong, and the safe answer is to refuse.
func (s *Service) allowCodeFromIP(ip string) bool {
	if ip == "" {
		return false
	}
	now := time.Now()
	s.codeIPMu.Lock()
	defer s.codeIPMu.Unlock()
	if s.codeIP == nil {
		s.codeIP = make(map[string]*ipCodeWindow)
	}
	w, ok := s.codeIP[ip]
	if !ok || now.Sub(w.start) >= emailCodeIPWindow {
		s.codeIP[ip] = &ipCodeWindow{start: now, count: 1}
		// Opportunistic sweep: the map only ever holds origins from the last
		// hour, so it cannot grow without bound on a long-running process.
		for k, v := range s.codeIP {
			if now.Sub(v.start) >= emailCodeIPWindow {
				delete(s.codeIP, k)
			}
		}
		return true
	}
	if w.count >= emailCodeIPLimit {
		return false
	}
	w.count++
	return true
}
