package httpx

import (
	"net/http"
	"strings"
)

// AuthCookieName is the single source of truth for the HttpOnly session
// cookie. The auth handler writes it; the auth middleware and the general
// rate limiter both read it. Keeping the literal in one place is what stops
// a rename from silently turning every signed-in browser request into an
// anonymous one in one of the two readers.
const AuthCookieName = "lp_token"

// ExtractToken returns the session JWT carried by the request. The cookie
// wins because that is all the SPA sends since X1; the Authorization header
// remains as a fallback for API clients and older integrations.
func ExtractToken(r *http.Request) string {
	if c, err := r.Cookie(AuthCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}
