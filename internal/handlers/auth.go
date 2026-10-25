package handlers

import (
	"encoding/json"
	"net/http"

	"gorm.io/gorm"

	"lopezpress/internal/config"
	"lopezpress/internal/middleware"
	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// The authenticated user and the originating client IP live on the request
// context under a key owned by the services package
// (services.WithActor / services.ActorFromContext). Identity is a domain
// concept — the service layer needs "who is acting, from where" to write an
// audit row — so the key is not redeclared here.
//
// cfg is injectable: cmd/server wires it via Init. upload.go reads it for the
// upload size cap; all business logic lives in the package-level svc.
var cfg *config.Config

// svc is the shared services.Service. Set once by Init before the router is
// wired up; both auth.go and posts.go call into it. Storing it at package
// level keeps WithAuth's signature minimal (no need to re-plumb db/cfg
// through the router).
var svc *services.Service

// Init injects the shared GORM handle, configuration and services.Service
// into the handlers package. Called once from cmd/server before the
// router is wired up. The gorm handle is accepted for API stability but is
// no longer stored at package level (business logic lives on svc).
func Init(_ *gorm.DB, c *config.Config, s *services.Service) {
	cfg = c
	svc = s
}

// --- JSON response helpers (echo-free) ---

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// writeInvalidBody is the repeated shorthand for "the JSON request body could
// not be decoded". Centralised so the message stays consistent (P2-2 dedup of
// the 16 call sites that previously inlined the identical string).
func writeInvalidBody(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, "invalid request body")
}

// writeServiceError maps a services.ServiceError (or any error) to an HTTP
// response using the status + message the service attached.
func writeServiceError(w http.ResponseWriter, err error) {
	writeError(w, services.StatusOf(err), services.MsgOf(err))
}

// --- auth endpoints ---

// Register creates a new user account and returns a signed JWT so the new
// user is logged in immediately (spec 04 AC-2.2).
// POST /api/auth/register
func Register(w http.ResponseWriter, r *http.Request) {
	var in services.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}

	res, err := svc.Register(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, res)
}

// Login verifies credentials and returns a JWT.
// POST /api/auth/login
func Login(w http.ResponseWriter, r *http.Request) {
	var in services.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}

	meta := services.LoginMeta{IP: clientIP(r), UA: r.UserAgent()}
	res, err := svc.Login(r.Context(), in, meta)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

// Me returns the public view of the currently authenticated user.
// GET /api/auth/me  (requires JWT)
func Me(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	writeJSON(w, http.StatusOK, services.UserPublic{
		ID: me.ID, Username: me.Username, Email: me.Email, Role: me.Role,
		DisplayName: me.DisplayName, Bio: me.Bio, Avatar: me.Avatar, CreatedAt: me.CreatedAt,
	})
}

// Logout is a client-side signal only. JWT is stateless, so the server does
// not truly revoke; the client should discard its stored token.
// POST /api/auth/logout  (requires JWT)
func Logout(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// ChangePassword lets the authenticated user change their own password and,
// as a side effect, clears the must_change_password flag (first-boot admin
// flow, spec #2). Requires the caller's current password.
// POST /api/auth/change-password  (requires JWT)
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	var in services.ChangePasswordInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	token, expires, err := svc.ChangePassword(r.Context(), me, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// A fresh token is returned because the change revoked the old one (token
	// version bumped). The client swaps it in to stay signed in.
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "password changed",
		"token":   token,
		"expires": expires,
	})
}

// MeStats returns the current user's post metrics (count + total views) for
// the personal center banner. GET /api/me/stats  (requires JWT)
func MeStats(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	stats, err := svc.UserStats(r.Context(), me)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// MeLoginHistory returns the current user's own recent login attempts
// (success + failure) for the security center. GET /api/me/login-history
// (requires JWT). Distinct from the admin endpoint which can read any user.
func MeLoginHistory(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	limit := parseLimit(r.URL.Query(), "limit", 20, 100)
	logs, err := svc.ListUserLogins(r.Context(), me, me.ID, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// LogoutAll revokes every JWT the caller holds by bumping their token version
// (the "log out all devices" action). The caller's own token is invalidated
// too, so the client must discard it and return to login.
// POST /api/auth/logout-all  (requires JWT)
func LogoutAll(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	if err := svc.RevokeAllSessions(r.Context(), me); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "all sessions revoked"})
}

// WithAuth wraps next with JWT verification: it parses the
// "Authorization: Bearer <token>" header, validates the token, loads the
// user from the DB (via svc.Authenticate) and stores it on the request
// context under UserContextKey. It also wraps the request context via
// middleware.WithUserID so the rate limiter keys the per-user bucket by
// user ID. On failure it writes 401 JSON responses.
//
// The signature no longer needs db/cfg — it relies on the package-level svc
// set by Init. The router simply passes through the handler.
func WithAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		user, err := svc.Authenticate(r.Context(), h)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		ctx := services.WithActor(r.Context(), user, clientIP(r))
		// Wrap so the limiter (which runs outermost) sees the user ID.
		ctx = middleware.WithUserID(ctx, user.ID)

		// First-boot flow: a user flagged must_change_password (e.g. the
		// seeded admin with the one-time password) may only change their
		// password, read their own session, or log out — every other
		// authenticated action is blocked until the password is changed.
		if user.MustChangePassword && !isPasswordChangeWhitelisted(r.URL.Path) {
			writeError(w, http.StatusForbidden, "password change required")
			return
		}

		next(w, r.WithContext(ctx))
	}
}

// isPasswordChangeWhitelisted reports whether the path is allowed while the
// caller still has must_change_password set (spec #2 first-boot flow).
func isPasswordChangeWhitelisted(path string) bool {
	switch path {
	case "/api/auth/me", "/api/auth/change-password", "/api/auth/logout", "/api/auth/logout-all":
		return true
	}
	return false
}

// userFromContext retrieves the authenticated user stored by WithAuth. An
// anonymous request (or one that never went through the auth middleware)
// yields the zero user, which every caller already treats as "not logged in".
func userFromContext(r *http.Request) models.User {
	return services.ActorFromContext(r.Context())
}

// RequireRole guards next so only a caller whose role appears in roles may
// proceed. It is the single route-level privilege check (P4): the router's
// route table declares each endpoint's allowed roles, and this middleware
// enforces them — handlers no longer carry their own requireAdmin/requireRoot
// checks.
//
// roles uses the models.Role* strings ("root", "admin", "user"); the role is
// read from the authenticated user placed on the request context by WithAuth /
// WithOptionalAuth. An anonymous request yields the zero user (Role ""), which
// matches no entry and is rejected 403.
func RequireRole(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u := userFromContext(r)
			for _, role := range roles {
				if u.Role == role {
					next(w, r)
					return
				}
			}
			// Preserve the pre-P4 messages: root-only endpoints reported
			// "root only", every other privileged endpoint "admin only".
			msg := "admin only"
			if len(roles) == 1 && roles[0] == models.RoleRoot {
				msg = "root only"
			}
			writeError(w, http.StatusForbidden, msg)
		}
	}
}

// WithOptionalAuth wraps next with "optional" JWT verification: if an
// Authorization header is present and valid, the user is resolved and stored
// on the context exactly like WithAuth; if the header is absent, the request
// is allowed to proceed with an anonymous (zero) user. This is used for the
// read-only surface (post lists, archive, sidebar categories/tags, recent
// posts) which must be browsable by logged-out visitors, while still giving
// signed-in users their personal scopes (e.g. their own private posts).
//
// A malformed or expired token still yields 401 — an explicit credential is
// not silently dropped, it is rejected. Only the *absence* of a credential
// is treated as anonymous.
func WithOptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if h == "" {
			// Spec 27: when the site is locked to signed-in readers, an
			// anonymous read is a 401 — the "login required" promise must be
			// enforced on the API too, not only in the SPA's route guard.
			if !svc.AllowAnonymous(r.Context()) {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			next(w, r)
			return
		}

		user, err := svc.Authenticate(r.Context(), h)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		// First-boot lock: a user flagged must_change_password may only change
		// their password / read their own session / log out. On the *optional*
		// auth surface there is no change-password route (read-only only), so
		// such a token is downgraded to an anonymous (zero) user — the caller
		// still gets the public read scope, but never their private/personal
		// posts. This mirrors WithAuth's lock (P1-5) so a pre-change-password
		// token can't keep browsing personal data.
		if user.MustChangePassword {
			user = models.User{}
			// Spec 27: the downgrade produces an anonymous caller, so the same
			// locked-site rule applies.
			if !svc.AllowAnonymous(r.Context()) {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
		}

		ctx := services.WithActor(r.Context(), user, clientIP(r))
		ctx = middleware.WithUserID(ctx, user.ID)
		next(w, r.WithContext(ctx))
	}
}
