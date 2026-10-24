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
	method  string
	path    string
	auth    authMode
	roles   []string // nil = any caller; else the exact allowed role list
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
	{method: "GET", path: "/api/authors/{username}", auth: authPublic, handler: handlers.GetAuthor},
	{method: "GET", path: "/api/site-config", auth: authPublic, handler: handlers.GetSiteConfig},
	// Read a published article without authentication. Only returns posts with
	// status "published"; no longer increments view_count (spec 23 — counting
	// moved to POST /api/posts/{id}/view).
	{method: "GET", path: "/api/posts/{id}/public", auth: authPublic, handler: handlers.GetPostPublic},

	// ---- Public: static assets & SEO ----
	{method: "GET", path: "/uploads/", auth: authPublic, handler: handlers.ServeUploads},
	{method: "GET", path: "/feed", auth: authPublic, handler: handlers.FeedRSS},
	{method: "GET", path: "/sitemap.xml", auth: authPublic, handler: handlers.Sitemap},
	{method: "GET", path: "/robots.txt", auth: authPublic, handler: handlers.Robots},

	// ---- Authenticated: current-user session (JWT) ----
	{method: "GET", path: "/api/auth/me", auth: authRequired, handler: handlers.Me},
	{method: "PUT", path: "/api/auth/me", auth: authRequired, handler: handlers.UpdateMe},
	{method: "POST", path: "/api/auth/logout", auth: authRequired, handler: handlers.Logout},
	// Change-own-password (first-boot admin flow, spec #2).
	{method: "POST", path: "/api/auth/change-password", auth: authRequired, handler: handlers.ChangePassword},
	// Personal-center metrics (spec 20) + security center.
	{method: "GET", path: "/api/me/stats", auth: authRequired, handler: handlers.MeStats},
	{method: "GET", path: "/api/me/login-history", auth: authRequired, handler: handlers.MeLoginHistory},
	{method: "POST", path: "/api/auth/logout-all", auth: authRequired, handler: handlers.LogoutAll},

	// ---- Read-only surface: optional auth (spec 22) ----
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
	// View report (spec 23): optional auth so anonymous readers count too.
	{method: "POST", path: "/api/posts/{id}/view", auth: authOptional, handler: handlers.ReportView},
	{method: "PUT", path: "/api/posts/{id}", auth: authRequired, handler: handlers.UpdatePost},
	{method: "DELETE", path: "/api/posts/{id}", auth: authRequired, handler: handlers.DeletePost},
	{method: "GET", path: "/api/posts/{id}/revisions", auth: authRequired, handler: handlers.ListRevisions},
	{method: "POST", path: "/api/posts/{id}/revisions/{revID}/restore", auth: authRequired, handler: handlers.RestoreRevision},

	// ---- Authenticated: image upload / library (any user) ----
	{method: "POST", path: "/api/upload", auth: authRequired, handler: handlers.UploadImage},
	{method: "GET", path: "/api/uploads", auth: authRequired, handler: handlers.ListUploads},

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
	{method: "POST", path: "/api/admin/import/markdown", auth: authRequired, roles: adminRoles, handler: handlers.ImportMarkdown},
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
	{method: "POST", path: "/api/backup/import", auth: authRequired, roles: rootRoles, handler: handlers.ImportBackup},
	{method: "POST", path: "/api/backup/restore-snapshot", auth: authRequired, roles: rootRoles, handler: handlers.RestoreSnapshot},
	{method: "GET", path: "/api/backup/snapshots", auth: authRequired, roles: rootRoles, handler: handlers.ListSnapshots},
	{method: "GET", path: "/api/admin/backup/auto", auth: authRequired, roles: rootRoles, handler: handlers.GetAutoBackupStatus},
	{method: "POST", path: "/api/admin/backup/auto", auth: authRequired, roles: rootRoles, handler: handlers.RunAutoBackupNow},

	// ---- Admin (admin|root): image deletion ----
	{method: "DELETE", path: "/api/uploads/{name}", auth: authRequired, roles: adminRoles, handler: handlers.DeleteUpload},
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
// loginLimiter (spec #3) wraps ONLY POST /api/auth/login with a stricter,
// login-specific limiter (IP + account dual bucket + 5-failure lockout).
// The general limiter still applies to every other route.
func New(cfg *config.Config, limiter *middleware.Limiter, loginLimiter *middleware.LoginLimiter) http.Handler {
	mux := http.NewServeMux()

	// Special cases that don't fit the table: the exact-root index, the health
	// probe, and the login endpoint (which carries its own dedicated limiter).
	mux.HandleFunc("GET /", rootIndex)
	mux.HandleFunc("GET /health", health)
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
		pattern := rt.method + " " + rt.path
		switch rt.auth {
		case authPublic:
			mux.HandleFunc(pattern, h)
		case authRequired:
			mux.Handle(pattern, withAuth(h))
		case authOptional:
			mux.Handle(pattern, withOptionalAuth(h))
		}
	}
}

// rootIndex serves the API index. The "/" pattern is a subtree prefix that
// would swallow EVERY unmatched path (incl. /favicon.ico -> 200 JSON noise),
// so it is restricted to the exact root; unknown paths fall through to 404.
func rootIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"service":"lopezpress","endpoints":["/health","/api/auth/register","/api/auth/login","/api/posts","/api/posts/{id}","/feed","/sitemap.xml"]}`))
}

// health is the liveness probe.
func health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

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
