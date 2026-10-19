package handlers

import (
	"encoding/json"
	"net/http"

	"lopezpress/internal/services"
)

// recordAudit is a thin wrapper that appends an action_log row. It pulls the
// actor from the request context (set by WithAuth) and the client IP from the
// request, so every admin handler can call it on success without threading
// those through service signatures.
func recordAudit(r *http.Request, action, target, detail string) {
	svc.RecordAction(r.Context(), services.ActionInput{
		Actor:  userFromContext(r),
		Action: action,
		Target: target,
		Detail: detail,
		IP:     clientIP(r),
	})
}

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
	})
}

// GetAdminSiteConfig returns the FULL site config (incl. token + flags).
// GET /api/admin/site-config (auth, admin)
func GetAdminSiteConfig(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	cfg, err := svc.GetSiteConfig(r.Context(), true)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// UpdateSiteConfig persists the full site config.
// PUT /api/admin/site-config (auth, admin)
func UpdateSiteConfig(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var in map[string]any
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	// Map the JSON body onto the model. friend_links arrives as a raw JSON
	// string (the client serialises the array itself).
	cfg := services.SiteConfigFromMap(in)
	out, err := svc.UpdateSiteConfig(r.Context(), cfg)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "update_site_config", "site_config", "")
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
	if !requireAdmin(w, r) {
		return
	}
	actor := userFromContext(r)
	err := svc.SendTestMail(r.Context(), actor)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "send_test_mail", "mail:"+actor.Email, "")
	writeJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

// --- user management (admin, spec 09) ---

// CreateUser provisions a new account on behalf of an admin.
// POST /api/users (auth, admin)
func CreateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
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
	recordAudit(r, "create_user", "user:"+u.ID, "username="+in.Username+"; role="+u.Role)
	writeJSON(w, http.StatusCreated, toUserRow(u))
}

// UpdateUser changes role/disabled for a user.
// PUT /api/users/{id} (auth, admin)
func UpdateUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
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
	recordAudit(r, "update_user", "user:"+id, "role="+in.Role)
	writeJSON(w, http.StatusOK, toUserRow(u))
}

// ResetPassword sets a new password for a user.
// POST /api/users/{id}/reset-password (auth, admin)
func ResetPassword(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
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
	recordAudit(r, "reset_password", "user:"+id, "")
	writeJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

// DeleteUser removes a user (self + last-admin + posts guards in service).
// DELETE /api/users/{id} (auth, admin)
func DeleteUser(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if err := svc.DeleteUser(r.Context(), userFromContext(r), id); err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "delete_user", "user:"+id, "")
	w.WriteHeader(http.StatusNoContent)
}

// ListUserLogins returns a user's recent login history.
// GET /api/users/{id}/logins?limit=50 (auth, admin)
func ListUserLogins(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	limit := parseLimit(r.URL.Query(), "limit", 50, 100)
	logs, err := svc.ListUserLogins(r.Context(), id, limit)
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
	if !requireAdmin(w, r) {
		return
	}
	data, err := svc.GetDashboard(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, data)
}
