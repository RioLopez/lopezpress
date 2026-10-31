package handlers

import (
	"encoding/json"
	"net/http"

	"lopezpress/internal/captcha"
	"lopezpress/internal/services"
)

// Login verifies credentials and returns a JWT.
// POST /api/auth/login
//
// A correct captcha is a precondition, not a fallback: without one the request
// is refused before the password is ever compared, so guess volume is bounded
// by how fast an attacker can solve pictures rather than by how fast they can
// POST.
//
// This lives in its own file because it is the only auth endpoint whose body is
// shaped differently from the service DTO it feeds. Keeping that divergence in
// one place stops it from spreading into services.LoginInput.
func Login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)

	// The captcha fields are decoded here instead of being added to
	// services.LoginInput: the service layer authenticates a password and has
	// no business knowing about puzzles. Anonymous embedding keeps the JSON
	// shape flat, so the wire format did not change for email/username/password.
	var in struct {
		services.LoginInput
		CaptchaID   string `json:"captcha_id"`
		CaptchaCode string `json:"captcha_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}

	if captchaOn() {
		st := captcha.Default()
		if err := checkCaptcha(st, ip, in.CaptchaID, in.CaptchaCode); err != nil {
			rejectCaptcha(w, st, ip, captchaReason(err))
			return
		}
	}

	meta := services.LoginMeta{IP: ip, UA: r.UserAgent()}
	res, err := svc.Login(r.Context(), in.LoginInput, meta)
	if err != nil {
		// A wrong password tightens this client's picture allowance. Note it is
		// recorded here, not by the middleware: the login limiter counts 401s
		// for the account lockout, and doubling up would let one wrong password
		// cost the account two strikes.
		captcha.Default().NoteFailure(ip)
		writeServiceError(w, err)
		return
	}

	// Getting in clears the streak: a human who eventually types the right
	// password should not keep paying for yesterday's typos.
	captcha.Default().NoteSuccess(ip)
	writeJSON(w, http.StatusOK, res)
}
