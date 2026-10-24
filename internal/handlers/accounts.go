package handlers

import (
	"encoding/json"
	"net/http"

	"lopezpress/internal/services"
)

// Audit rows are written by the service layer, next to the write they describe
// (internal/services/audit.go). Handlers deliberately have no audit API: that
// is what makes "every write path is audited" a property of the code rather
// than a checklist item a new endpoint can forget.

// --- site config (spec 05) ---

// GetSiteConfig returns the PUBLIC subset of the site config (no token, no
// allow_register flag). Used by the frontend to render the brand, title and
// footer on every page.
// GET /api/site-config (public)
func GetSiteConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := svc.GetSiteConfig(r.Context(), false)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"site_name":    cfg.SiteName,
		"site_title":   cfg.SiteTitle,
		"site_logo":    cfg.SiteLogo,
		"site_favicon": cfg.SiteFavicon,
		"site_url":     cfg.SiteURL,
		"footer_text":  cfg.FooterText,
		"icp_no":       cfg.IcpNo,
		"friend_links": cfg.FriendLinks,
		// Spec 24: the public list needs the server-decided page size so it
		// can render ?page=N without ever sending `size` itself.
		"page_size": cfg.PageSize,
		// Spec 27: the SPA's route guard needs to know whether anonymous
		// reading is allowed before it can decide to bounce to /login.
		"public_read": cfg.PublicRead,
	})
}

// GetAdminSiteConfig returns the FULL site config for the settings form.
// GET /api/admin/site-config (auth, admin)
//
// Secrets are masked for a plain admin: smtp_pass and img_host_token are
// credentials (the SMTP password is very often reused for the root login), and
// reading them from a settings screen is a privilege escalation, not a feature.
// The admin form still works because UpdateSiteConfig re-inherits the stored
// values for any field the caller may not change.
func GetAdminSiteConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := svc.GetSiteConfig(r.Context(), true)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if !services.IsRoot(userFromContext(r)) {
		cfg.SmtpPass = ""
		cfg.ImgHostToken = ""
	}
	writeJSON(w, http.StatusOK, cfg)
}

// UpdateSiteConfig persists the full site config.
// PUT /api/admin/site-config (auth, admin)
func UpdateSiteConfig(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	// Map the JSON body onto the model. friend_links arrives as a raw JSON
	// string (the client serialises the array itself).
	cfg := services.SiteConfigFromMap(in)
	// Root-only settings: the daily-backup policy, the registration gate and
	// the mail/image-host credentials. A non-root admin saving the settings
	// form must not be able to switch any of them off (or blank a secret it
	// cannot read) just because their payload omitted or masked it, so their
	// submission inherits the stored values. Field-level authorisation, not a
	// route guard: an admin keeps full control of the content fields.
	if !services.IsRoot(userFromContext(r)) {
		if cur, cerr := svc.GetSiteConfig(r.Context(), true); cerr == nil {
			cfg.AutoBackup = cur.AutoBackup
			cfg.BackupKeep = cur.BackupKeep
			cfg.AllowRegister = cur.AllowRegister
			cfg.SmtpHost = cur.SmtpHost
			cfg.SmtpPort = cur.SmtpPort
			cfg.SmtpUser = cur.SmtpUser
			cfg.SmtpPass = cur.SmtpPass
			cfg.ImgHostToken = cur.ImgHostToken
		}
	}
	out, err := svc.UpdateSiteConfig(r.Context(), cfg)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Never echo a secret back, whatever the caller's role.
	out.SmtpPass = ""
	out.ImgHostToken = ""
	writeJSON(w, http.StatusOK, out)
}

// --- registration gate (spec 16) ---

// RegistrationOpen reports whether public registration is enabled.
// GET /api/auth/register/open (public)
func RegistrationOpen(w http.ResponseWriter, r *http.Request) {
	cfg, err := svc.GetSiteConfig(r.Context(), false)
	open := false
	if err == nil {
		open = cfg.AllowRegister
	}
	writeJSON(w, http.StatusOK, map[string]bool{"open": open})
}

// SendTestMail synchronously sends a test e-mail to the admin's own address to
// verify the SMTP settings. Unlike the throttled notification sends (which run
// in detached goroutines and swallow errors), this returns the actual error so
// the operator can fix a misconfigured SMTP server.
// POST /api/admin/mail/test (auth, admin)
func SendTestMail(w http.ResponseWriter, r *http.Request) {
	actor := userFromContext(r)
	err := svc.SendTestMail(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

// --- user management (admin, spec 09) ---

// CreateUser provisions a new account on behalf of an admin.
// POST /api/users (auth, admin)
func CreateUser(w http.ResponseWriter, r *http.Request) {
	var in services.CreateUserInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	u, err := svc.CreateUser(r.Context(), userFromContext(r), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserRow(u))
}

// UpdateUser changes role/disabled for a user.
// PUT /api/users/{id} (auth, admin)
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in services.UpdateUserInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	u, err := svc.UpdateUser(r.Context(), userFromContext(r), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserRow(u))
}

// ResetPassword sets a new password for a user.
// POST /api/users/{id}/reset-password (auth, admin)
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	if err := svc.ResetPassword(r.Context(), userFromContext(r), id, in.Password); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

// DeleteUser removes a user (self + last-admin + posts guards in service).
// DELETE /api/users/{id} (auth, admin)
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := svc.DeleteUser(r.Context(), userFromContext(r), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListUserLogins returns a user's recent login history.
// GET /api/users/{id}/logins?limit=50 (auth, admin)
func ListUserLogins(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := parseLimit(r.URL.Query(), "limit", 50, 100)
	logs, err := svc.ListUserLogins(r.Context(), userFromContext(r), id, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// --- profile / author (spec 15) ---

// UpdateMe edits the caller's own profile.
// PUT /api/auth/me (auth)
func UpdateMe(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	var in services.UpdateMeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	u, err := svc.UpdateMe(r.Context(), me.ID, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, services.UserPublic{
		ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role,
		DisplayName: u.DisplayName, Bio: u.Bio, Avatar: u.Avatar, CreatedAt: u.CreatedAt,
	})
}

// GetAuthor returns a public author profile + published posts.
// GET /api/authors/{username} (public)
func GetAuthor(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	view, err := svc.GetAuthor(r.Context(), username)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// --- dashboard (spec 06) ---

// GetDashboard aggregates admin dashboard widgets.
// GET /api/admin/dashboard (auth, admin)
func GetDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := svc.GetDashboard(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}
