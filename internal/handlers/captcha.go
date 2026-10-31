package handlers

import (
	"errors"
	"net/http"

	"lopezpress/internal/captcha"
)

// captchaIssued is the response of GET /api/auth/captcha.
//
// The challenge is nested under `captcha`, which is the same shape a rejected
// login uses in rejectCaptcha. One shape means the frontend has one code path
// for "here is a new puzzle", whichever way it arrived.
//
// The `enabled` flag is always present so a frontend built against this version
// keeps working against a server that has the feature switched off — it renders
// no widget instead of an empty box nobody can solve.
type captchaIssued struct {
	Enabled bool               `json:"enabled"`
	Captcha *captcha.Challenge `json:"captcha,omitempty"`
}

// captchaOn reports whether the login challenge is in force.
func captchaOn() bool {
	return cfg != nil && cfg.CaptchaEnabled && captcha.Default() != nil
}

// noStoreCaptcha marks a response as uncacheable.
//
// Every captcha response carries a live challenge id and its picture. Letting
// any cache — browser, proxy, or the operator's CDN — keep a copy would serve
// one user's challenge to the next, whose answer the first user has already
// seen.
func noStoreCaptcha(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
}

// IssueCaptcha hands out a login challenge.
// GET /api/auth/captcha (public)
func IssueCaptcha(w http.ResponseWriter, r *http.Request) {
	noStoreCaptcha(w)

	if !captchaOn() {
		writeJSON(w, http.StatusOK, captchaIssued{Enabled: false})
		return
	}

	ch, err := captcha.Default().Issue(clientIP(r))
	if err == nil {
		writeJSON(w, http.StatusOK, captchaIssued{Enabled: true, Captcha: &ch})
		return
	}

	if errors.Is(err, captcha.ErrRateLimited) {
		// Refusing is the point: an attacker who wants frames to train an OCR
		// model has to wait for them at the same pace a human does.
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "too many captcha requests, slow down")
		return
	}
	// Never fall through to "no captcha": a failure to draw the challenge must
	// not become a failure to require it. Refusing the login is the fail-closed
	// direction.
	writeError(w, http.StatusServiceUnavailable, "captcha unavailable, try again")
}

// checkCaptcha consumes the challenge named by id and returns nil when the
// answer matches. The challenge is single-use either way, and only the client
// it was issued to can spend it.
func checkCaptcha(st *captcha.Store, client, id, code string) error {
	if id == "" {
		return captcha.ErrNotFound
	}
	return st.Verify(client, id, captcha.Answer{Code: code})
}

// rejectCaptcha answers a login whose challenge was missing or wrong.
//
// Two things matter here. First, the response carries a *fresh* challenge, so
// a human who misread four letters does not have to retype their password just
// to get another picture. Second, the status is 400, not 401: the login
// limiters key their account lockout off 401/403, and letting a wrong captcha
// count toward that would hand an attacker a cheaper way to lock the owner
// out — five blank captchas instead of five wrong passwords.
func rejectCaptcha(w http.ResponseWriter, st *captcha.Store, client string, reason string) {
	noStoreCaptcha(w)
	st.NoteFailure(client)
	resp := map[string]any{
		"error":            reason,
		"captcha_required": true,
	}
	if ch, err := st.Issue(client); err == nil {
		resp["captcha"] = ch
	}
	writeJSON(w, http.StatusBadRequest, resp)
}

// captchaReason turns a verification error into a message the UI can act on.
//
// The strings are deliberately coarse. Telling a caller "expired" versus
// "wrong" is useful to a human and costs nothing: neither reveals whether the
// four characters were close, and the picture is already spent.
func captchaReason(err error) string {
	switch {
	case errors.Is(err, captcha.ErrExpired):
		return "captcha expired"
	case errors.Is(err, captcha.ErrNotFound):
		return "captcha missing or already used"
	case errors.Is(err, captcha.ErrTooMany):
		return "too many captcha attempts"
	case errors.Is(err, captcha.ErrRateLimited):
		return "too many captcha requests"
	case errors.Is(err, captcha.ErrNotReady):
		return "captcha unavailable"
	default:
		return "captcha incorrect"
	}
}
