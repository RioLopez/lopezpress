// Package router assembles the HTTP routes and middleware chain for the
// application using the standard library net/http (Go 1.22+ ServeMux
// method/wildcard patterns).
package router

import (
	"net/http"

	"lopezpress/internal/config"
	"lopezpress/internal/handlers"
	"lopezpress/internal/middleware"
	"lopezpress/internal/models"
)

// authMode declares how a route authenticates its caller.
type authMode int

const (
	// authPublic: no JWT required (anonymous readers / public assets).
	authPublic authMode = iota
	// authRequired: a valid JWT is mandatory; missing/invalid -> 401.
	authRequired
	// authOptional: browse anonymously, but honor a valid bearer token.
	authOptional
)

// route is the single declaration point for one HTTP endpoint: its method,
// path, authentication mode, allowed roles, and handler. Roles is the
// route-level privilege guard (P4): the router enforces it via
// handlers.RequireRole, so handlers must NOT re-check roles themselves.
type route struct {
	method string
	path   string
	auth   authMode
	roles  []string // nil = any caller; else the exact allowed role list
	// maxBody overrides the default request-body ceiling for this route. Zero
	// means "use defaultBodyLimit". Only the multipart upload needs more.
	//
	// It lives in the table, not in the handler, because the alternative has
	// already failed once: R3-05 capped the login body and R3-07 capped the
	// post body, and every other write handler kept decoding an unbounded
	// `r.Body` — 18 of 21 decode sites had no ceiling at all. Putting the
	// number next to the route makes "a new endpoint forgot its cap" a
	// declaration error rather than an omission nobody notices.
	maxBody int64
	handler http.HandlerFunc
}

// Role lists used by the route table (see models.Role*).
var (
	adminRoles = []string{models.RoleAdmin, models.RoleRoot}
	rootRoles  = []string{models.RoleRoot}
)

// routes is the complete route table, grouped for readability. Each privileged
// endpoint declares its roles here rather than in the handler (P4).
var routes = []route{
	// ---- Public: auth & read-only surface ----
	{method: "POST", path: "/api/auth/register", auth: authPublic, handler: handlers.Register},
	{method: "GET", path: "/api/auth/register/open", auth: authPublic, handler: handlers.RegistrationOpen},
	// Login challenge. Public like register/login: it is part of the
	// pre-authentication flow, and the kind it returns is decided by the
	// caller's failure history, not by anything in the request.
	{method: "GET", path: "/api/auth/captcha", auth: authPublic, handler: handlers.IssueCaptcha},
	// Email verification (experimental, off by default). Both answer
	// identically whether or not the address has an account.
	{method: "POST", path: "/api/auth/email-code", auth: authPublic, handler: handlers.RequestEmailCode},
	{method: "POST", path: "/api/auth/password/reset", auth: authPublic, handler: handlers.ResetPasswordWithCode},
	// Author profiles and single-post reads are OPTIONAL-auth, not public: the
	// anonymous-read switch has to reach them. As authPublic they sat outside
	// WithOptionalAuth — the only place AllowAnonymous is consulted — so a site
	// set to public_read=false still answered these four endpoints with full
	// bodies, RSS items and author post lists to anonymous callers (R1-01).
	{method: "GET", path: "/api/authors/{username}", auth: authOptional, handler: handlers.GetAuthor},
	{method: "GET", path: "/api/site-config", auth: authPublic, handler: handlers.GetSiteConfig},
	// Read a published article without authentication. Only returns posts with
	// status "published"; no longer increments view_count (counting
	// moved to POST /api/posts/{id}/view).
	{method: "GET", path: "/api/posts/{id}/public", auth: authOptional, handler: handlers.GetPostPublic},

	// ---- Public: static assets & SEO ----
	// /uploads and /robots.txt stay authPublic on purpose: images are embedded
	// by URL (a browser sends no Authorization header, so gating them would
	// break covers for every signed-in reader too), and robots.txt must be
	// readable by a crawler to be told to stay away.
	{method: "GET", path: "/uploads/", auth: authPublic, handler: handlers.ServeUploads},
	{method: "GET", path: "/feed", auth: authOptional, handler: handlers.FeedRSS},
	{method: "GET", path: "/sitemap.xml", auth: authOptional, handler: handlers.Sitemap},
	{method: "GET", path: "/robots.txt", auth: authPublic, handler: handlers.Robots},

	// ---- Authenticated: current-user session (JWT) ----
	{method: "GET", path: "/api/auth/me", auth: authRequired, handler: handlers.Me},
	{method: "PUT", path: "/api/auth/me", auth: authRequired, handler: handlers.UpdateMe},
	{method: "POST", path: "/api/auth/logout", auth: authRequired, handler: handlers.Logout},
	// Change-own-password (first-boot admin flow).
	{method: "POST", path: "/api/auth/change-password", auth: authRequired, handler: handlers.ChangePassword},
	// Personal-center metrics + security center.
	{method: "GET", path: "/api/me/stats", auth: authRequired, handler: handlers.MeStats},
	{method: "GET", path: "/api/me/login-history", auth: authRequired, handler: handlers.MeLoginHistory},
	{method: "POST", path: "/api/auth/logout-all", auth: authRequired, handler: handlers.LogoutAll},
	// Verified email change: the code goes to the *new* address, and
	// only an authenticated session may request one.
	{method: "POST", path: "/api/me/email-change/request", auth: authRequired, handler: handlers.RequestEmailChange},
	{method: "POST", path: "/api/me/email-change/confirm", auth: authRequired, handler: handlers.ConfirmEmailChange},

	// ---- Read-only surface: optional auth ----
	{method: "GET", path: "/api/posts", auth: authOptional, handler: handlers.ListPosts},
	{method: "GET", path: "/api/posts/recent", auth: authOptional, handler: handlers.RecentPosts},
	{method: "GET", path: "/api/posts/archive", auth: authOptional, handler: handlers.ListArchive},
	{method: "GET", path: "/api/posts/archive/paged", auth: authOptional, handler: handlers.ListArchivePaged},
	{method: "GET", path: "/api/posts/{id}", auth: authOptional, handler: handlers.GetPost},
	{method: "GET", path: "/api/posts/{id}/nav", auth: authOptional, handler: handlers.GetPostPrevNext},
	{method: "GET", path: "/api/posts/{id}/related", auth: authOptional, handler: handlers.GetRelatedPosts},
	// Permanent-link resolution lives OUTSIDE /api/posts/ to avoid ambiguity
	// with /api/posts/{id}/public (Go ServeMux panics on equal specificity).
	{method: "GET", path: "/api/permalinks/{slug}", auth: authOptional, handler: handlers.GetPostBySlug},
	{method: "GET", path: "/api/categories", auth: authOptional, handler: handlers.ListCategories},
	{method: "GET", path: "/api/tags", auth: authOptional, handler: handlers.ListTags},

	// ---- Authenticated: post writes (strict JWT, any user) ----
	{method: "POST", path: "/api/posts", auth: authRequired, handler: handlers.CreatePost},
	// View report: optional auth so anonymous readers count too.
	{method: "POST", path: "/api/posts/{id}/view", auth: authOptional, handler: handlers.ReportView},
	{method: "PUT", path: "/api/posts/{id}", auth: authRequired, handler: handlers.UpdatePost},
	{method: "DELETE", path: "/api/posts/{id}", auth: authRequired, handler: handlers.DeletePost},
	{method: "GET", path: "/api/posts/{id}/revisions", auth: authRequired, handler: handlers.ListRevisions},
	{method: "POST", path: "/api/posts/{id}/revisions/{revID}/restore", auth: authRequired, handler: handlers.RestoreRevision},

	// ---- Authenticated: image upload ----
	// Any signed-in author may upload (an article needs pictures). The
	// LIBRARY, however, is not theirs: see the admin block below.
	{method: "POST", path: "/api/upload", auth: authRequired, maxBody: maxUploadBodyLimit, handler: handlers.UploadImage},
	// Personal view: only the caller's own uploads. This exists so "withdraw
	// an image I uploaded" has somewhere to live without handing out a
	// site-wide directory.
	{method: "GET", path: "/api/me/uploads", auth: authRequired, handler: handlers.ListMyUploads},
	{method: "DELETE", path: "/api/me/uploads/{name}", auth: authRequired, handler: handlers.DeleteMyUpload},

	// ---- Admin (admin|root): categories & tags ----
	{method: "GET", path: "/api/categories/with-count", auth: authRequired, roles: adminRoles, handler: handlers.ListCategoriesWithCount},
	{method: "POST", path: "/api/admin/categories/preset", auth: authRequired, roles: adminRoles, handler: handlers.ApplyPresetCategories},
	{method: "POST", path: "/api/categories", auth: authRequired, roles: adminRoles, handler: handlers.CreateCategory},
	{method: "PUT", path: "/api/categories/{id}", auth: authRequired, roles: adminRoles, handler: handlers.UpdateCategory},
	{method: "DELETE", path: "/api/categories/{id}", auth: authRequired, roles: adminRoles, handler: handlers.DeleteCategory},
	{method: "GET", path: "/api/tags/with-count", auth: authRequired, roles: adminRoles, handler: handlers.ListTagsWithCount},
	{method: "PUT", path: "/api/tags/{id}", auth: authRequired, roles: adminRoles, handler: handlers.UpdateTag},
	{method: "DELETE", path: "/api/tags/{id}", auth: authRequired, roles: adminRoles, handler: handlers.DeleteTag},

	// ---- Admin (admin|root): user management ----
	{method: "GET", path: "/api/users", auth: authRequired, roles: adminRoles, handler: handlers.ListUsers},
	{method: "POST", path: "/api/users", auth: authRequired, roles: adminRoles, handler: handlers.CreateUser},
	{method: "PUT", path: "/api/users/{id}", auth: authRequired, roles: adminRoles, handler: handlers.UpdateUser},
	{method: "PUT", path: "/api/users/{id}/disabled", auth: authRequired, roles: adminRoles, handler: handlers.SetUserDisabled},
	{method: "POST", path: "/api/users/{id}/reset-password", auth: authRequired, roles: adminRoles, handler: handlers.ResetPassword},
	{method: "DELETE", path: "/api/users/{id}", auth: authRequired, roles: adminRoles, handler: handlers.DeleteUser},
	{method: "GET", path: "/api/users/{id}/logins", auth: authRequired, roles: adminRoles, handler: handlers.ListUserLogins},

	// ---- Admin (admin|root): site config, dashboard, audit, mail ----
	{method: "GET", path: "/api/admin/site-config", auth: authRequired, roles: adminRoles, handler: handlers.GetAdminSiteConfig},
	{method: "PUT", path: "/api/admin/site-config", auth: authRequired, roles: adminRoles, handler: handlers.UpdateSiteConfig},
	{method: "GET", path: "/api/admin/dashboard", auth: authRequired, roles: adminRoles, handler: handlers.GetDashboard},
	{method: "GET", path: "/api/admin/actions", auth: authRequired, roles: adminRoles, handler: handlers.ListActions},
	{method: "POST", path: "/api/admin/mail/test", auth: authRequired, roles: adminRoles, handler: handlers.SendTestMail},

	// ---- Admin (admin|root): content moderation queue ----
	{method: "GET", path: "/api/admin/pending", auth: authRequired, roles: adminRoles, handler: handlers.ListPending},
	{method: "POST", path: "/api/admin/posts/batch", auth: authRequired, roles: adminRoles, handler: handlers.BatchUpdatePosts},
	{method: "POST", path: "/api/admin/review/{id}", auth: authRequired, roles: adminRoles, handler: handlers.ReviewPost},

	// ---- Root-only: banned vault ----
	{method: "POST", path: "/api/admin/ban/{id}", auth: authRequired, roles: rootRoles, handler: handlers.BanPost},
	{method: "GET", path: "/api/admin/banned", auth: authRequired, roles: rootRoles, handler: handlers.ListBanned},
	{method: "POST", path: "/api/admin/banned/{id}/send-back", auth: authRequired, roles: rootRoles, handler: handlers.SendBackPost},
	{method: "DELETE", path: "/api/admin/banned/{id}", auth: authRequired, roles: rootRoles, handler: handlers.DeleteBannedPermanently},

	// ---- Admin (admin|root): Markdown import + trash ----
	{method: "POST", path: "/api/admin/import/markdown", auth: authRequired, roles: adminRoles, maxBody: maxMarkdownImportLimit, handler: handlers.ImportMarkdown},
	{method: "GET", path: "/api/trash", auth: authRequired, roles: adminRoles, handler: handlers.ListTrash},
	{method: "POST", path: "/api/trash/empty", auth: authRequired, roles: adminRoles, handler: handlers.EmptyTrash},
	{method: "POST", path: "/api/trash/{id}/restore", auth: authRequired, roles: adminRoles, handler: handlers.RestoreTrash},

	// ---- Root-only: backup export / download / import / snapshot restore ----
	// The export JSON embeds bcrypt password hashes and the snapshots are raw
	// .db copies of the users table, so the whole backup surface is root-only.
	// An admin could previously list-restrict snapshots (403) yet still pull
	// any file by name through the download endpoint, which made the export
	// and download paths an unguarded hash-disclosure channel.
	{method: "POST", path: "/api/backup", auth: authRequired, roles: rootRoles, handler: handlers.ExportBackup},
	{method: "GET", path: "/api/backup/download", auth: authRequired, roles: rootRoles, handler: handlers.DownloadBackup},
	{method: "POST", path: "/api/backup/import", auth: authRequired, roles: rootRoles, maxBody: maxBackupImportLimit, handler: handlers.ImportBackup},
	{method: "POST", path: "/api/backup/restore-snapshot", auth: authRequired, roles: rootRoles, handler: handlers.RestoreSnapshot},
	{method: "GET", path: "/api/backup/snapshots", auth: authRequired, roles: rootRoles, handler: handlers.ListSnapshots},
	{method: "GET", path: "/api/admin/backup/auto", auth: authRequired, roles: rootRoles, handler: handlers.GetAutoBackupStatus},
	{method: "POST", path: "/api/admin/backup/auto", auth: authRequired, roles: rootRoles, handler: handlers.RunAutoBackupNow},

	// ---- Admin (admin|root): image library ----
	// Site-wide asset view: who uploaded what, when, from where, and what still
	// references it. Plain admins get it too — they moderate content, and the
	// gallery is where a bad image is actually dealt with.
	{method: "GET", path: "/api/uploads", auth: authRequired, roles: adminRoles, handler: handlers.ListUploads},
	{method: "DELETE", path: "/api/uploads/{name}", auth: authRequired, roles: adminRoles, handler: handlers.DeleteUpload},
	{method: "POST", path: "/api/uploads/{name}/restore", auth: authRequired, roles: adminRoles, handler: handlers.RestoreUpload},
}

// New builds the composed http.Handler:
//
//   - a *http.ServeMux using Go 1.22 method + wildcard patterns
//   - a middleware chain wrapped around the mux
//
// Chain order (outermost -> innermost):
//
//	limiter.Middleware -> CORS -> Logger -> Recover
//
// so the limiter sees every request first (before logging/cors), and Recover
// is the innermost wrapper so it still guards the actual handlers.
//
// db is not a parameter: handlers.WithAuth now resolves its dependencies via
// the package-level services.Service installed by handlers.Init (called from
// cmd/server before the router is built).
//
// loginLimiter wraps ONLY POST /api/auth/login with a stricter,
// login-specific limiter (IP + account dual bucket + 5-failure lockout).
// The general limiter still applies to every other route.
func New(cfg *config.Config, limiter *middleware.Limiter, loginLimiter *middleware.LoginLimiter) http.Handler {
	mux := http.NewServeMux()

	// Special cases that don't fit the table: the catch-all, the health probes,
	// and the login endpoint (which carries its own dedicated limiter).
	//
	// "/" is the catch-all, not an API index. It used to answer
	// {"service":"lopezpress","endpoints":[…]} — a free route map naming the
	// project, which is exactly the fingerprint a self-hosted box should not
	// hand out. In production nginx serves the SPA at "/" and never forwards
	// here, so the index was invisible to normal readers and visible only to
	// whoever was already probing the Go port directly — the one audience that
	// should learn nothing. A method-agnostic catch-all also replaces the
	// text/plain "404 page not found" the stdlib would emit, which is itself a
	// Go signature.
	mux.HandleFunc("/", notFoundJSON)
	// Two probes, two questions. /health is liveness (process up, no DB touch —
	// a supervisor must not restart-loop over a disk problem); /health/ready is
	// readiness (can this instance serve, which requires the database to answer).
	mux.HandleFunc("GET /health", handlers.Healthz)
	mux.HandleFunc("GET /health/ready", handlers.Ready)
	mux.Handle("POST /api/auth/login", loginLimiter.Middleware(http.HandlerFunc(handlers.Login)))

	register(mux, routes)

	// Compose the middleware chain:
	//   limiter (outer) -> CORS -> SecurityHeaders -> Logger -> Recover (inner)
	// Chain applies mws in reverse, so the FIRST listed middleware is the
	// outermost. limiter.Middleware(next) already returns an http.Handler.
	// SecurityHeaders sits inside CORS so the CORS preflight short-circuit
	// still carries the hardening headers.
	return middleware.Chain(mux,
		func(h http.Handler) http.Handler { return limiter.Middleware(h) },
		middleware.CORS(cfg.CORSOrigins),
		middleware.SecurityHeaders,
		middleware.Logger,
		middleware.Recover,
	)
}

// register wires each route into mux, composing (from inside out):
// handler -> RequireRole (if roles declared) -> auth middleware.
func register(mux *http.ServeMux, routes []route) {
	for _, rt := range routes {
		h := rt.handler
		if len(rt.roles) > 0 {
			h = handlers.RequireRole(rt.roles...)(h)
		}
		// Body cap first (outermost of the handler composition): it must apply
		// before anything reads the body, including the auth middleware's own
		// parsing on optional-auth routes.
		limit := rt.maxBody
		if limit <= 0 {
			limit = defaultBodyLimit
		}
		h = limitBody(limit, h)
		pattern := rt.method + " " + rt.path
		switch rt.auth {
		case authPublic:
			// Every route carries the client IP, even the anonymous ones: the
			// service layer's abuse budgets (email-code per-IP, audit rows)
			// read it off the context, and an unannotated public route used to
			// look like "no origin" to them (R3-03 / R4-06).
			mux.HandleFunc(pattern, handlers.WithClientIP(h))
		case authRequired:
			mux.Handle(pattern, withAuth(h))
		case authOptional:
			mux.Handle(pattern, withOptionalAuth(h))
		}
	}
}

// notFoundJSON answers every path no route claimed. It is the catch-all for
// "/", so it must never describe what the application is: the same body and
// the same status for a missing page, a missing API, and a probe for /.git —
// nothing to distinguish one guess from another.
//
// The body is JSON so it matches every other error the API can produce; the
// stdlib default (text/plain "404 page not found") leaks the language's
// standard library string and stands out against real responses.
func notFoundJSON(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"error":"not found"}`))
}

// health is the liveness probe.

// withAuth wraps an http.HandlerFunc with JWT authentication. It relies on
// the package-level services.Service installed via handlers.Init, so the
// router does not need to re-plumb db/cfg.
func withAuth(h http.HandlerFunc) http.Handler {
	return handlers.WithAuth(h)
}

// withOptionalAuth allows read endpoints to be browsed anonymously while
// still honoring a valid bearer token when one is supplied.
func withOptionalAuth(h http.HandlerFunc) http.Handler {
	return handlers.WithOptionalAuth(h)
}
