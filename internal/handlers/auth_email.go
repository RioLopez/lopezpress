package handlers

import (
	"encoding/json"
	"net/http"

	"lopezpress/internal/services"
)

// --- email verification endpoints (experimental) ---
//
// Four endpoints, split by who may call them:
//
//	public    POST /api/auth/email-code        request a code
//	public    POST /api/auth/password/reset    finish a self-service reset
//	signed-in POST /api/me/email-change/request
//	signed-in POST /api/me/email-change/confirm
//
// The anonymous pair answers identically whether or not the address belongs to
// an account; see the enumeration note in services/auth_email.go. That is why
// the handlers never inspect the result beyond "error or not".

// RequestEmailCode mails a one-time code. Always answers 200 on success so a
// caller cannot probe which addresses are registered.
func RequestEmailCode(w http.ResponseWriter, r *http.Request) {
	var in services.EmailCodeRequestInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	if err := svc.RequestEmailCode(r.Context(), in); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "if this address can receive mail, a code is on its way",
	})
}

// ResetPasswordWithCode completes a password reset. Named differently from the
// admin-side ResetPassword (accounts.go) which sets somebody else's password.
func ResetPasswordWithCode(w http.ResponseWriter, r *http.Request) {
	var in services.PasswordResetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	if err := svc.ResetPasswordByCode(r.Context(), in); err != nil {
		writeServiceError(w, err)
		return
	}
	// Every session was revoked by the reset, so the client must log in again.
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "password reset; sign in with the new password",
	})
}

// RequestEmailChange mails a code to the requested new address.
func RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	var in services.EmailChangeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	if err := svc.RequestEmailChange(r.Context(), me, in.Email); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "code sent to the new address",
	})
}

// ConfirmEmailChange applies the new address after the code check and returns
// the refreshed account so the UI can update without a second request.
func ConfirmEmailChange(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	var in services.EmailChangeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	user, err := svc.ConfirmEmailChange(r.Context(), me, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services.PublicUser(user))
}
