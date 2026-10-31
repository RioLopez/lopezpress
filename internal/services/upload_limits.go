package services

import (
	"time"

	"golang.org/x/time/rate"
)

// Per-account upload rate limiting.
//
// The site-wide limiter allows 20 requests per second to a signed-in identity,
// which is the right shape for page loads and no limit at all for uploads: a
// script with one session could push twenty images a second into a queue whose
// real cost is decoding 24 megapixels each. The storage quotas do stop the
// flood eventually, but "eventually" on a 1 vCPU box means the machine is
// already busy decoding.
//
// Two buckets, because they answer different questions. The minute bucket stops
// a burst (say, pasting a gallery into the editor — which the composer uploads
// serially, so a human never approaches it). The hour bucket stops a sustained
// drip that stays under the minute rate forever.
//
// The values are deliberately generous against humans: pasting twenty images in
// one go is a legitimate editing session and lands exactly at the minute limit.
const (
	uploadPerMinute = 20
	uploadPerHour   = 200
	// uploadBurst allows a short spike above the sustained rate so a paste of a
	// few images does not have to wait out the refill.
	uploadBurst = 5
	// uploadGates bounds the map. Accounts are finite, but a long-lived process
	// should not keep a bucket for every account that ever uploaded.
	uploadGates = 1024
)

// uploadBuckets is one account's pair of token buckets.
type uploadBuckets struct {
	minute   *rate.Limiter
	hour     *rate.Limiter
	lastUsed time.Time
}

// AllowUpload reports whether userID may upload right now, and if not, how long
// to wait. It is enforced before the body is parsed, so a refused request never
// costs a decode.
func (s *Service) AllowUpload(userID string) (time.Duration, bool) {
	if userID == "" {
		return 0, true
	}
	s.uploadMu.Lock()
	defer s.uploadMu.Unlock()
	if s.uploadGates == nil {
		s.uploadGates = make(map[string]*uploadBuckets)
	}
	now := time.Now()
	b := s.uploadGates[userID]
	if b == nil {
		if len(s.uploadGates) >= uploadGates {
			s.sweepUploadGatesLocked(now)
		}
		b = &uploadBuckets{
			minute: rate.NewLimiter(rate.Every(time.Minute/uploadPerMinute), uploadBurst),
			hour:   rate.NewLimiter(rate.Every(time.Hour/uploadPerHour), uploadBurst),
		}
		s.uploadGates[userID] = b
	}
	b.lastUsed = now

	// ReserveN reserves a token now and reports how late it would be. Zero delay
	// means the token was available; anything else is put back so a refused
	// attempt does not consume the caller's allowance.
	minuteRes := b.minute.ReserveN(now, 1)
	if delay := minuteRes.Delay(); delay > 0 {
		minuteRes.Cancel()
		return delay, false
	}
	hourRes := b.hour.ReserveN(now, 1)
	if delay := hourRes.Delay(); delay > 0 {
		hourRes.Cancel()
		minuteRes.Cancel()
		return delay, false
	}
	return 0, true
}

// sweepUploadGatesLocked drops buckets idle for over an hour. The caller holds
// uploadMu. An idle bucket is indistinguishable from a fresh one — the tokens
// refill either way — so dropping it costs nothing.
func (s *Service) sweepUploadGatesLocked(now time.Time) {
	cutoff := now.Add(-time.Hour)
	for id, b := range s.uploadGates {
		if b.lastUsed.Before(cutoff) {
			delete(s.uploadGates, id)
		}
	}
}
