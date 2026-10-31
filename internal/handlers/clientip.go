package handlers

import (
	"net/http"

	"lopezpress/internal/clientip"
	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// --- request identity on the public surface --------------------------------
//
// The service layer reads "who is acting, and from where" off the request
// context (services.WithActor / ActorFromContext / ClientIPFromContext), and
// that context used to be populated only by WithAuth / WithOptionalAuth —
// i.e. only on routes that carry a JWT. The public routes therefore looked
// anonymous AND origin-less to the service layer, and any budget keyed by IP
// was quietly disabled on exactly the surface an anonymous attacker uses.

// clientIP extracts the best-effort client IP. The trust rules (proxy headers
// are only honoured when they arrive from a configured proxy) live in
// internal/clientip.
func clientIP(r *http.Request) string {
	return clientip.Client(r)
}

// WithClientIP attaches the originating client IP to the request context for
// routes that have no authenticated identity.
//
// The concrete case that forced this: /api/auth/email-code is a public route
// whose only abuse control is a per-IP hourly budget, and allowCodeFromIP used
// to be handed an empty string on every request. It answered "" with "no
// origin information, allow it", so twelve addresses from one client produced
// twelve mails and not one 429 (R3-03 / R4-06), and 40 of the 46 rows stored
// at the time had an empty ip column that made the trail useless afterwards.
//
// Wrapping the whole public surface — not just that one endpoint — is what
// keeps the hole closed for handlers added later.
//
// The user attached is the zero user, which is exactly what ActorFromContext
// already returns for an anonymous request, so no handler's behaviour changes:
// a public route that never looked at the IP sees no difference.
func WithClientIP(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := services.WithActor(r.Context(), models.User{}, clientIP(r))
		next(w, r.WithContext(ctx))
	}
}
