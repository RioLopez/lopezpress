// Package router assembles the HTTP routes and middleware chain for the
// application using the standard library net/http (Go 1.22+ ServeMux
// method/wildcard patterns).
package router

import (
	"net/http"

	"echo-content-api/internal/config"
	"echo-content-api/internal/handlers"
	"echo-content-api/internal/middleware"
)

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
// is the innermost wrapper so it still guards the actual handlers. Each
// protected posts route is additionally wrapped individually with
// handlers.WithAuth (JWT).
//
// db is not a parameter: handlers.WithAuth now resolves its dependencies via
// the package-level services.Service installed by handlers.Init (called from
// cmd/server before the router is built).
// New builds the composed http.Handler: ...
//
// loginLimiter (spec #3) wraps ONLY POST /api/auth/login with a stricter,
// login-specific limiter (IP + account dual bucket + 5-failure lockout).
// The general limiter still applies to every other route.
func New(cfg *config.Config, limiter *middleware.Limiter, loginLimiter *middleware.LoginLimiter) http.Handler {
	mux := http.NewServeMux()

	// Public: root index (exact path only).
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// In Go 1.22 ServeMux the "/" pattern is a subtree prefix that would
		// swallow EVERY unmatched path (incl. /favicon.ico -> 200 JSON noise).
		// Restrict to the exact root; unknown paths fall through to the 404
		// handler registered just below.
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"service":"echo-content-api","endpoints":["/health","/api/auth/register","/api/auth/login","/api/posts","/api/posts/{id}","/feed","/sitemap.xml"]}`))
	})

	// Public: health check.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Public: auth.
	mux.HandleFunc("POST /api/auth/register", handlers.Register)
	// Login gets the dedicated login limiter (IP + account buckets + failure
	// lockout, spec #3) wrapped around the handler, before the general
	// limiter's route. The general per-identity limiter still guards every
	// other route; the login limiter is a stricter, login-only policy.
	mux.Handle("POST /api/auth/login", loginLimiter.Middleware(http.HandlerFunc(handlers.Login)))
	// Public: probe whether open registration is enabled (spec 16) + public
	// author profile (spec 15).
	mux.HandleFunc("GET /api/auth/register/open", handlers.RegistrationOpen)
	mux.HandleFunc("GET /api/authors/{username}", handlers.GetAuthor)
	// Public: read-only site config (brand/title/footer) — no secrets.
	mux.HandleFunc("GET /api/site-config", handlers.GetSiteConfig)

	// Public: read a published article without authentication.
	// Only returns posts with status "published"; increments view_count.
	mux.HandleFunc("GET /api/posts/{id}/public", handlers.GetPostPublic)

	// Protected: current-user session (JWT via WithAuth).
	mux.Handle("GET /api/auth/me", withAuth(handlers.Me))
	mux.Handle("PUT /api/auth/me", withAuth(handlers.UpdateMe))
	mux.Handle("POST /api/auth/logout", withAuth(handlers.Logout))
	// Change-own-password (first-boot admin flow, spec #2): verifies the
	// current password and clears must_change_password in one call.
	mux.Handle("POST /api/auth/change-password", withAuth(handlers.ChangePassword))
	// Personal-center metrics (spec 20): the caller's post count + total views.
	mux.Handle("GET /api/me/stats", withAuth(handlers.MeStats))
	// Security center (spec 20): self-service login history + revoke-all.
	mux.Handle("GET /api/me/login-history", withAuth(handlers.MeLoginHistory))
	mux.Handle("POST /api/auth/logout-all", withAuth(handlers.LogoutAll))

	// READ-ONLY surface: optional auth. Logged-out visitors see all public
	// content; signed-in users additionally see their own private posts.
	// (Spec 22: public/private visibility.) Bad/expired token still 401.
	mux.Handle("GET /api/posts", withOptionalAuth(handlers.ListPosts))
	mux.Handle("GET /api/posts/recent", withOptionalAuth(handlers.RecentPosts))
	mux.Handle("GET /api/posts/archive", withOptionalAuth(handlers.ListArchive))
	mux.Handle("GET /api/posts/{id}", withOptionalAuth(handlers.GetPost))
	mux.Handle("GET /api/posts/{id}/nav", withOptionalAuth(handlers.GetPostPrevNext))
	mux.Handle("GET /api/categories", withOptionalAuth(handlers.ListCategories))
	mux.Handle("GET /api/tags", withOptionalAuth(handlers.ListTags))

	// Writes / mutations still require a valid JWT (strict).
	mux.Handle("POST /api/posts", withAuth(handlers.CreatePost))
	mux.Handle("PUT /api/posts/{id}", withAuth(handlers.UpdatePost))
	mux.Handle("DELETE /api/posts/{id}", withAuth(handlers.DeletePost))

	// Write/admin: categories and tags.
	mux.Handle("GET /api/categories/with-count", withAuth(handlers.ListCategoriesWithCount))
	mux.Handle("POST /api/categories", withAuth(handlers.CreateCategory))
	mux.Handle("PUT /api/categories/{id}", withAuth(handlers.UpdateCategory))
	mux.Handle("DELETE /api/categories/{id}", withAuth(handlers.DeleteCategory))
	mux.Handle("GET /api/tags/with-count", withAuth(handlers.ListTagsWithCount))
	mux.Handle("DELETE /api/tags/{id}", withAuth(handlers.DeleteTag))

	// Protected: image upload (multipart). Returns { "url": "/uploads/<file>" }.
	mux.Handle("POST /api/upload", withAuth(handlers.UploadImage))
	// Image library: browse + delete locally-stored uploads.
	mux.Handle("GET /api/uploads", withAuth(handlers.ListUploads))
	mux.Handle("DELETE /api/uploads/{name}", withAuth(handlers.DeleteUpload))

	// Protected: admin user management (JWT + admin role enforced in handler).
	mux.Handle("GET /api/users", withAuth(handlers.ListUsers))
	mux.Handle("POST /api/users", withAuth(handlers.CreateUser))
	mux.Handle("PUT /api/users/{id}", withAuth(handlers.UpdateUser))
	mux.Handle("PUT /api/users/{id}/disabled", withAuth(handlers.SetUserDisabled))
	mux.Handle("POST /api/users/{id}/reset-password", withAuth(handlers.ResetPassword))
	mux.Handle("DELETE /api/users/{id}", withAuth(handlers.DeleteUser))
	mux.Handle("GET /api/users/{id}/logins", withAuth(handlers.ListUserLogins))

	// Protected: admin site config + dashboard (JWT + admin role enforced).
	mux.Handle("GET /api/admin/site-config", withAuth(handlers.GetAdminSiteConfig))
	mux.Handle("PUT /api/admin/site-config", withAuth(handlers.UpdateSiteConfig))
	mux.Handle("GET /api/admin/dashboard", withAuth(handlers.GetDashboard))
	// Admin: operation audit trail (content + admin mutations).
	mux.Handle("GET /api/admin/actions", withAuth(handlers.ListActions))
	// Admin: verify SMTP settings with a synchronous test e-mail.
	mux.Handle("POST /api/admin/mail/test", withAuth(handlers.SendTestMail))

	// Protected: content moderation + banned vault (JWT + role enforced in
	// handler). The review queue is admin|root; the banned vault endpoints
	// are root-only (the service re-checks every privilege).
	mux.Handle("GET /api/admin/pending", withAuth(handlers.ListPending))
	mux.Handle("POST /api/admin/review/{id}", withAuth(handlers.ReviewPost))
	mux.Handle("POST /api/admin/ban/{id}", withAuth(handlers.BanPost))
	mux.Handle("GET /api/admin/banned", withAuth(handlers.ListBanned))
	mux.Handle("POST /api/admin/banned/{id}/send-back", withAuth(handlers.SendBackPost))
	mux.Handle("DELETE /api/admin/banned/{id}", withAuth(handlers.DeleteBannedPermanently))

	// Protected: admin data backup (JWT + admin role enforced in handler).
	mux.Handle("POST /api/backup", withAuth(handlers.ExportBackup))
	mux.Handle("POST /api/backup/import", withAuth(handlers.ImportBackup))
	// Spec #6: restore a previously created VACUUM INTO snapshot file.
	mux.Handle("POST /api/backup/restore-snapshot", withAuth(handlers.RestoreSnapshot))
	// Spec #6: list available snapshots for the restore UI (root-only, P2-14).
	mux.Handle("GET /api/backup/snapshots", withAuth(handlers.ListSnapshots))
	// Download a previously generated backup file (name validated server-side).
	mux.Handle("GET /api/backup/download", withAuth(handlers.DownloadBackup))

	// Protected: admin trash / recovery (JWT + admin role enforced in handler).
	mux.Handle("GET /api/trash", withAuth(handlers.ListTrash))
	mux.Handle("POST /api/trash/empty", withAuth(handlers.EmptyTrash))
	mux.Handle("POST /api/trash/{id}/restore", withAuth(handlers.RestoreTrash))

	// Public: serve uploaded images. Static binary assets are not
	// authentication-sensitive; the path-traversal check lives in the
	// service layer.
	mux.HandleFunc("GET /uploads/", handlers.ServeUploads)

	// Public: RSS 2.0 feed + sitemap for SEO and subscriptions (spec: blog
	// essentials). Both derive absolute links from site_url (or the request
	// host) and list only published posts.
	mux.HandleFunc("GET /feed", handlers.FeedRSS)
	mux.HandleFunc("GET /sitemap.xml", handlers.Sitemap)

	// Compose the middleware chain:
	//   limiter (outer) -> CORS -> Logger -> Recover (inner)
	// Chain applies mws in reverse, so the FIRST listed middleware is the
	// outermost. limiter.Middleware(next) already returns an http.Handler.
	//
	// CORS_ORIGINS is read from config (default "*" in dev; an explicit
	// origin list in prod). The CORS middleware never sets
	// Access-Control-Allow-Credentials together with "*", so there is no
	// "wildcard origin + credentials" footgun (see docs/decisions/0004-cors).
	return middleware.Chain(mux,
		func(h http.Handler) http.Handler { return limiter.Middleware(h) },
		middleware.CORS(cfg.CORSOrigins),
		middleware.Logger,
		middleware.Recover,
	)
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
