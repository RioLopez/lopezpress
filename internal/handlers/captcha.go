package handlers

import (
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

// IssueCaptcha hands out a login challenge.
// GET /api/auth/captcha (public)
//
// The caller does not choose the kind. A client with a clean record gets the
// four-character image; one that has already failed repeatedly gets the
// sliding puzzle. Letting the request ask for the easy one would make the
// escalation decorative.
func IssueCaptcha(w http.ResponseWriter, r *http.Request) {
	if !captchaOn() {
		writeJSON(w, http.StatusOK, captchaIssued{Enabled: false})
		return
	}

	ch, err := captcha.Default().Issue(clientIP(r))
	if err != nil {
		// Never fall through to "no captcha": a failure to draw the challenge
		// must not become a failure to require it. Refusing the login is the
		// fail-closed direction.
		writeError(w, http.StatusServiceUnavailable, "captcha unavailable, try again")
		return
	}
	writeJSON(w, http.StatusOK, captchaIssued{Enabled: true, Captcha: &ch})
}

// checkCaptcha consumes the challenge named by id and returns nil when the
// answer matches. The challenge is single-use either way.
func checkCaptcha(st *captcha.Store, id, code string, x int) error {
	if id == "" {
		return captcha.ErrNotFound
	}
	return st.Verify(id, captcha.Answer{Code: code, X: x})
}

// rejectCaptcha answers a login whose challenge was missing or wrong.
//
// Two things matter here. First, the response carries a *fresh* challenge, so
// a human who misread four letters does not have to retype their password
// just to get another picture. Second, the status is 400, not 401: the login
// limiters key their account lockout off 401/403, and letting a wrong captcha
// count toward that would hand an attacker a cheaper way to lock the owner
// out — five blank captchas instead of five wrong passwords.
//
// The failure still counts toward the captcha policy, which is exactly what
// escalates the next challenge to a slide.
func rejectCaptcha(w http.ResponseWriter, st *captcha.Store, client string, reason string) {
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
func captchaReason(err error) string {
	switch err {
	case captcha.ErrExpired:
		return "captcha expired"
	case captcha.ErrNotFound:
		return "captcha missing or already used"
	case captcha.ErrTooMany:
		return "too many captcha attempts"
	case captcha.ErrNotReady:
		return "captcha unavailable"
	default:
		return "captcha incorrect"
	}
}
