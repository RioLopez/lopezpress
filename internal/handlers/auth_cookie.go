package handlers

import (
	"net/http"
	"strings"
	"time"
)

// authCookieName carries the JWT as an HttpOnly cookie so the SPA never has to
// hold the token in JS-readable storage (localStorage), which is what made a
// single XSS a full session compromise. See VERIFY_04 X1 / ATTACK_SURFACE_REPORT.
const authCookieName = "lp_token"

// isTLS reports whether the request arrived over HTTPS, directly or behind a
// trusted reverse proxy that sets X-Forwarded-Proto (the nginx A/B chain does).
func isTLS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// setAuthCookie writes the JWT as an HttpOnly, SameSite=Lax cookie. Secure is set
// only over TLS so a plain-HTTP dev deployment still works. SameSite=Lax blocks
// the cookie from cross-site POSTs (CSRF) while still sending it on same-site
// navigations and the SPA's same-origin fetch calls.
func setAuthCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	maxAge := int(time.Until(expires).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	c := &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	}
	if isTLS(r) {
		c.Secure = true
	}
	http.SetCookie(w, c)
}

// clearAuthCookie expires the session cookie immediately.
func clearAuthCookie(w http.ResponseWriter, r *http.Request) {
	c := &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	}
	if isTLS(r) {
		c.Secure = true
	}
	http.SetCookie(w, c)
}

// extractToken returns the bearer token from the request, preferring the
// HttpOnly auth cookie (XSS-resistant) and falling back to the Authorization
// header (kept for API clients and backward compatibility). Either is accepted.
func extractToken(r *http.Request) string {
	if c, err := r.Cookie(authCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}
