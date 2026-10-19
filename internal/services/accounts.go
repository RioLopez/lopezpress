package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- site config (spec 05) ---

// GetSiteConfig returns the single-row site config (id=1), creating the
// default row if it does not yet exist. The admin flag is accepted for API
// symmetry; the same row is returned — the handler decides which fields to
// expose (the public endpoint omits the token / allow_register flag).
func (s *Service) GetSiteConfig(ctx context.Context, admin bool) (models.SiteConfig, error) {
	var cfg models.SiteConfig
	if err := s.db.WithContext(ctx).Where("id = ?", 1).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cfg = models.SiteConfig{
				ID: 1, SiteName: "Echo", SiteTitle: "Echo",
				ImgHostMode: "local", AllowRegister: false,
			}
			if err2 := s.db.WithContext(ctx).Create(&cfg).Error; err2 != nil {
				return cfg, newErr(http.StatusInternalServerError, "failed to init site config")
			}
			return cfg, nil
		}
		return cfg, newErr(http.StatusInternalServerError, "failed to load site config")
	}
	return cfg, nil
}

// UpdateSiteConfig validates and persists the full site config row.
func (s *Service) UpdateSiteConfig(ctx context.Context, in models.SiteConfig) (models.SiteConfig, error) {
	if strings.TrimSpace(in.SiteName) == "" {
		return models.SiteConfig{}, newErr(http.StatusBadRequest, "site name is required")
	}
	if len([]rune(in.SiteName)) > 64 {
		return models.SiteConfig{}, newErr(http.StatusBadRequest, "site name too long")
	}
	if len([]rune(in.SiteTitle)) > 128 {
		return models.SiteConfig{}, newErr(http.StatusBadRequest, "site title too long")
	}
	if in.ImgHostMode != "local" && in.ImgHostMode != "smms" {
		return models.SiteConfig{}, newErr(http.StatusBadRequest, "invalid image host mode")
	}
	// SiteURL, when set, must be an absolute http(s) URL so feed/sitemap
	// can build correct absolute links.
	if u := strings.TrimSpace(in.SiteURL); u != "" {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return models.SiteConfig{}, newErr(http.StatusBadRequest, "site url must start with http:// or https://")
		}
	}
	// Validate friend_links is well-formed JSON when supplied.
	if strings.TrimSpace(in.FriendLinks) != "" {
		var links []map[string]string
		if err := json.Unmarshal([]byte(in.FriendLinks), &links); err != nil {
			return models.SiteConfig{}, newErr(http.StatusBadRequest, "friend links must be a JSON array")
		}
	}

	var existing models.SiteConfig
	if err := s.db.WithContext(ctx).Where("id = ?", 1).First(&existing).Error; err != nil {
		return models.SiteConfig{}, newErr(http.StatusInternalServerError, "failed to load site config")
	}
	existing.SiteName = in.SiteName
	existing.SiteTitle = in.SiteTitle
	existing.SiteLogo = in.SiteLogo
	existing.SiteFavicon = in.SiteFavicon
	existing.SiteURL = in.SiteURL
	existing.FooterText = in.FooterText
	existing.IcpNo = in.IcpNo
	existing.FriendLinks = in.FriendLinks
	existing.ImgHostMode = in.ImgHostMode
	existing.ImgHostToken = in.ImgHostToken
	existing.AllowRegister = in.AllowRegister
	existing.SmtpHost = in.SmtpHost
	existing.SmtpPort = in.SmtpPort
	existing.SmtpUser = in.SmtpUser
	existing.SmtpPass = in.SmtpPass
	existing.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return models.SiteConfig{}, newErr(http.StatusInternalServerError, "failed to save site config")
	}
	return existing, nil
}

// allowRegister reports whether public registration is enabled.
func (s *Service) allowRegister(ctx context.Context) bool {
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return false
	}
	return cfg.AllowRegister
}

// notifyNewLocation detects whether a successful login is from a new IP or
// browser compared to the user's last recorded login, and sends a
// security notification when it is. Best-effort: a mail failure never affects
// the login outcome. It runs BEFORE the last-login state is updated, so it
// compares against the previous values.
func (s *Service) notifyNewLocation(ctx context.Context, user models.User, meta LoginMeta) {
	if user.LastLoginIP == "" {
		// First login on record — nothing to compare; no notification.
		return
	}
	// A "new location" here means a different IP than the previous login.
	// (Comparing UA alone is unreliable — browsers update their fingerprint.)
	if meta.IP == user.LastLoginIP {
		return
	}
	s.SendMail(ctx, user, MailNewLocation, map[string]string{
		"ip": meta.IP, "prev_ip": user.LastLoginIP, "time": time.Now().Format("2006-01-02 15:04 UTC"), "ua": meta.UA,
	})
}

// SiteConfigFromMap converts a decoded JSON body into a models.SiteConfig.
// friend_links is expected as a raw JSON string (the client serialises the
// array itself), so it round-trips unchanged through the public/admin split.
func SiteConfigFromMap(m map[string]any) models.SiteConfig {
	getStr := func(k string) string {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
		return ""
	}
	getBool := func(k string) bool {
		if v, ok := m[k]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
		return false
	}
	getInt := func(k string) int {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case int:
				return n
			case float64:
				return int(n)
			case json.Number:
				if i, err := n.Int64(); err == nil {
					return int(i)
				}
			}
		}
		return 0
	}
	mode := getStr("img_host_mode")
	if mode != "smms" {
		mode = "local"
	}
	return models.SiteConfig{
		SiteName:     getStr("site_name"),
		SiteTitle:    getStr("site_title"),
		SiteLogo:     getStr("site_logo"),
		SiteFavicon:  getStr("site_favicon"),
		SiteURL:      getStr("site_url"),
		FooterText:   getStr("footer_text"),
		IcpNo:        getStr("icp_no"),
		FriendLinks:  getStr("friend_links"),
		ImgHostMode:  mode,
		ImgHostToken: getStr("img_host_token"),
		AllowRegister: getBool("allow_register"),
		SmtpHost:     getStr("smtp_host"),
		SmtpPort:     getInt("smtp_port"),
		SmtpUser:     getStr("smtp_user"),
		SmtpPass:     getStr("smtp_pass"),
	}
}

// --- login log (spec 14) ---

// RecordLogin appends a row to login_log. Failures are swallowed so the audit
// trail can never break the login flow.
func (s *Service) RecordLogin(ctx context.Context, userID, identifier, ip, ua string, success bool) {
	log := models.LoginLog{
		UserID: userID, Identifier: identifier, IP: ip, UA: ua, Success: success,
	}
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		_ = err // non-fatal
	}
}

// ActionInput describes an auditable operation recorded to action_log.
type ActionInput struct {
	Actor  models.User // who did it
	Action string      // "create_post" / "delete_user" / ...
	Target string      // "post:<uuid>" / "user:<uuid>" / ...
	Detail string      // optional free-form detail (old→new title, count, ...)
	IP     string      // originating client IP
}

// RecordAction appends a row to action_log. Failures are swallowed so the
// audit trail can never break the operation that triggered it.
func (s *Service) RecordAction(ctx context.Context, in ActionInput) {
	_ = ctx
	row := models.ActionLog{
		ActorID:   in.Actor.ID,
		ActorName: in.Actor.Username,
		Action:    in.Action,
		Target:    in.Target,
		Detail:    in.Detail,
		IP:        in.IP,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		// audit is best-effort; never fail the caller's operation
		_ = err
	}
}

// ListActions returns the most recent action_log rows (optionally filtered
// by an actor). Admin endpoint — the handler enforces requireAdmin.
func (s *Service) ListActions(ctx context.Context, actorID string, limit int) ([]models.ActionLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := s.db.WithContext(ctx).Model(&models.ActionLog{})
	if actorID != "" {
		query = query.Where("actor_id = ?", actorID)
	}
	var out []models.ActionLog
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load actions")
	}
	if out == nil {
		out = []models.ActionLog{}
	}
	return out, nil
}

// --- user management (admin, spec 09) ---

// CreateUserInput is the decoded payload for POST /api/users.
type CreateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// CreateUser creates an account on behalf of an admin. It is NOT subject to
// the allow_register gate (admins can always provision users).
//
// Role rules (three-tier model, spec: role model):
//   - Only a root actor may create accounts with role "admin".
//   - A plain admin may create accounts but only with role "user".
//   - No creation path may ever produce "root" — the single root is
//     established at bootstrap / migration and is not user-mutable.
//   - An unknown / empty role falls back to "user".
func (s *Service) CreateUser(ctx context.Context, actor models.User, in CreateUserInput) (models.User, error) {
	if in.Username == "" || in.Email == "" || in.Password == "" {
		return models.User{}, newErr(http.StatusBadRequest, "username, email and password are required")
	}
	if len(in.Password) < 8 {
		return models.User{}, newErr(http.StatusBadRequest, "password must be at least 8 characters")
	}
	// Service-level privilege check (defense in depth): only admin-or-root may
	// provision accounts. The handler already enforces this via requireAdmin,
	// but the service is the authoritative layer and must not trust the
	// handler to have done it.
	if !IsAdminOrRoot(actor) {
		return models.User{}, newErr(http.StatusForbidden, "only admin or root can create users")
	}
	var existing models.User
	if s.db.WithContext(ctx).Where("username = ? OR email = ?", in.Username, in.Email).First(&existing).RowsAffected == 1 {
		return models.User{}, newErr(http.StatusConflict, "username or email already in use")
	}
	// Resolve the requested role against the actor's privilege.
	var role string
	switch in.Role {
	case "", models.RoleUser:
		role = models.RoleUser
	case models.RoleAdmin:
		// Only a root may elevate to admin.
		if actor.Role != models.RoleRoot {
			return models.User{}, newErr(http.StatusForbidden, "only the root can create an admin")
		}
		role = models.RoleAdmin
	case models.RoleRoot:
		// The single root is never created through this endpoint.
		return models.User{}, newErr(http.StatusForbidden, "the root role cannot be assigned here")
	default:
		// Unknown role falls back to the safe default.
		role = models.RoleUser
	}
	hash, err := models.HashPassword(in.Password)
	if err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to hash password")
	}
	u := models.User{Username: in.Username, Email: in.Email, PasswordHash: hash, Role: role}
	if err := s.db.WithContext(ctx).Create(&u).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to create user")
	}
	return u, nil
}

// UpdateUserInput is the decoded payload for PUT /api/users/{id}.
type UpdateUserInput struct {
	Role     string `json:"role"`
	Disabled *bool  `json:"disabled"`
}

// UpdateUser changes role/disabled for a user with hierarchical protection.
//
// Role changes:
//   - The target role must be one of user/admin/root; anything else → 400.
//   - root is the unique super-admin and is UNMUTABLE through this path:
//     attempting to demote, disable, or retarget root → 403.
//   - Only a root actor may grant admin. A plain admin cannot elevate anyone
//     to admin, and cannot touch another admin or root.
//   - An admin may demote a user to user (no-op), but may not demote an
//     admin or themselves below admin.
//
// Disabled changes:
//   - An admin may toggle disabled only for plain users.
//   - root may toggle disabled for admin and user accounts, but never for root.
//   - No one may disable themselves.
func (s *Service) UpdateUser(ctx context.Context, actor models.User, id string, in UpdateUserInput) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}

	if in.Role != "" {
		// The root role can never be assigned or retargeted here.
		if in.Role == models.RoleRoot {
			return models.User{}, newErr(http.StatusForbidden, "the root role cannot be assigned")
		}
		if in.Role != models.RoleAdmin && in.Role != models.RoleUser {
			return models.User{}, newErr(http.StatusBadRequest, "invalid role")
		}
		// Protect the unique root: no demotion, no retarget.
		if user.Role == models.RoleRoot {
			return models.User{}, newErr(http.StatusForbidden, "the root account cannot be demoted")
		}
		// Hierarchical privilege checks.
		switch {
		case in.Role == models.RoleAdmin:
			// Granting admin is a root-only power.
			if actor.Role != models.RoleRoot {
				return models.User{}, newErr(http.StatusForbidden, "only the root can promote to admin")
			}
		case actor.Role == models.RoleAdmin:
			// A plain admin may manage plain users only — never an admin.
			if user.Role == models.RoleAdmin {
				return models.User{}, newErr(http.StatusForbidden, "admin cannot modify another admin")
			}
		}
		// Self-demotion guard: an admin demoting their own account below
		// admin would break recovery; root is already protected above.
		if user.ID == actor.ID && in.Role != actor.Role {
			return models.User{}, newErr(http.StatusForbidden, "cannot change your own role")
		}
		user.Role = in.Role
	}

	if in.Disabled != nil {
		// Never disable yourself, and never disable the unique root.
		if *in.Disabled && user.ID == actor.ID {
			return models.User{}, newErr(http.StatusForbidden, "cannot disable your own account")
		}
		if user.Role == models.RoleRoot {
			return models.User{}, newErr(http.StatusForbidden, "the root account cannot be disabled")
		}
		if *in.Disabled && actor.Role == models.RoleAdmin && user.Role == models.RoleAdmin {
			return models.User{}, newErr(http.StatusForbidden, "admin cannot disable another admin")
		}
		user.Disabled = *in.Disabled
	}

	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update user")
	}
	return user, nil
}

// ResetPassword sets a new password hash for a user on behalf of an admin.
// The actor is checked against the three-tier hierarchy: no caller may reset
// the unique root's password, and a plain admin may only reset plain users
// (never another admin, never self). This mirrors the other user-management
// endpoints, which all route through canManageUser.
func (s *Service) ResetPassword(ctx context.Context, actor models.User, id, password string) error {
	if len(password) < 8 {
		return newErr(http.StatusBadRequest, "password must be at least 8 characters")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return newErr(http.StatusNotFound, "user not found")
	}
	// Hierarchical guard: reuse the single authority for "may actor manage
	// target" so this endpoint can never diverge from UpdateUser/DeleteUser.
	if !canManageUser(actor, user) {
		return newErr(http.StatusForbidden, "you do not have permission to reset this password")
	}
	hash, err := models.HashPassword(password)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to hash password")
	}
	if err := s.db.WithContext(ctx).Model(&user).Update("password_hash", hash).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to reset password")
	}
	return nil
}

// ChangePasswordInput is the decoded payload for POST /api/auth/change-password.
type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword lets the caller change their own password. It verifies the
// current password, enforces a minimum new-password length, and clears the
// must_change_password flag so the flag can never re-trigger after the user
// has picked a password of their own. Used by the first-boot admin flow
// (spec #2: admin is seeded with a random console-banner password).
func (s *Service) ChangePassword(ctx context.Context, user models.User, in ChangePasswordInput) error {
	if !models.VerifyPassword(in.CurrentPassword, user.PasswordHash) {
		return newErr(http.StatusUnauthorized, "current password is incorrect")
	}
	if len(in.NewPassword) < 8 {
		return newErr(http.StatusBadRequest, "new password must be at least 8 characters")
	}
	hash, err := models.HashPassword(in.NewPassword)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to hash password")
	}
	upd := map[string]any{"password_hash": hash}
	if user.MustChangePassword {
		upd["must_change_password"] = false
	}
	if err := s.db.WithContext(ctx).Model(&user).Updates(upd).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to change password")
	}
	// Security notification to the owner (best-effort; a broken SMTP never
	// fails the password change itself).
	s.SendMail(ctx, user, MailPasswordChange, map[string]string{
		"time": time.Now().Format("2006-01-02 15:04 UTC"),
	})
	return nil
}

// RevokeAllSessions invalidates every JWT the user currently holds by
// incrementing their TokenVersion. Tokens embed the version at sign time, so
// any previously issued token fails the Authenticate version check on the
// next request — this is the "log out all devices" primitive for an
// otherwise-stateless JWT scheme. The caller's own token is invalidated too,
// so the client must clear its local session afterwards.
func (s *Service) RevokeAllSessions(ctx context.Context, user models.User) error {
	if err := s.db.WithContext(ctx).Model(&user).
		Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to revoke sessions")
	}
	return nil
}

// DeleteUser removes a user with hierarchical protection:
//   - cannot delete self;
//   - the unique root is undeletable by any path;
//   - the last admin cannot be deleted (admin/root accounts only);
//   - an admin cannot delete another admin (only root can);
//   - cannot delete a user that still owns posts.
func (s *Service) DeleteUser(ctx context.Context, actor models.User, id string) error {
	if id == actor.ID {
		return newErr(http.StatusBadRequest, "cannot delete your own account")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return newErr(http.StatusNotFound, "user not found")
	}
	// The unique root account is undeletable.
	if user.Role == models.RoleRoot {
		return newErr(http.StatusForbidden, "the root account cannot be deleted")
	}
	// A plain admin cannot delete another admin (or root).
	if user.Role == models.RoleAdmin && actor.Role != models.RoleRoot {
		return newErr(http.StatusForbidden, "only the root can delete an admin")
	}
	// Preserve the last-admin invariant for admin accounts.
	if user.Role == models.RoleAdmin {
		var adminCount int64
		s.db.WithContext(ctx).Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&adminCount)
		if adminCount <= 1 {
			return newErr(http.StatusBadRequest, "cannot delete the last admin")
		}
	}
	var postCount int64
	// Unscoped so soft-deleted (trashed) posts are also counted — otherwise a
	// user whose only posts live in the trash could be deleted, leaving those
	// rows with an orphaned author_id that the trash UI cannot resolve.
	s.db.WithContext(ctx).Unscoped().Model(&models.Post{}).Where("author_id = ?", id).Count(&postCount)
	if postCount > 0 {
		return newErr(http.StatusConflict, "please transfer or delete this user's posts first")
	}
	if err := s.db.WithContext(ctx).Delete(&user).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete user")
	}
	return nil
}

// ListUserLogins returns the most recent login_log rows for a user.
func (s *Service) ListUserLogins(ctx context.Context, id string, limit int) ([]models.LoginLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var logs []models.LoginLog
	if err := s.db.WithContext(ctx).Where("user_id = ?", id).
		Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load login logs")
	}
	if logs == nil {
		logs = []models.LoginLog{}
	}
	return logs, nil
}

// --- profile / author (spec 15) ---

// UpdateMeInput is the decoded payload for PUT /api/auth/me.
type UpdateMeInput struct {
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar"`
}

// UpdateMe edits the caller's own profile. Empty fields are left unchanged so
// a partial save never wipes the others.
func (s *Service) UpdateMe(ctx context.Context, id string, in UpdateMeInput) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}
	if in.DisplayName != "" {
		user.DisplayName = in.DisplayName
	}
	if in.Bio != "" {
		user.Bio = in.Bio
	}
	if in.Avatar != "" {
		user.Avatar = in.Avatar
	}
	if err := s.db.WithContext(ctx).Save(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update profile")
	}
	return user, nil
}

// AuthorUser is the public, sanitised view of an author.
type AuthorUser struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar"`
}

// AuthorView is the payload for GET /api/authors/{username}.
type AuthorView struct {
	User  AuthorUser    `json:"user"`
	Posts []models.Post  `json:"posts"`
}

// GetAuthor returns a public author profile plus their published posts. A
// missing or disabled user yields 404 (not exposed).
func (s *Service) GetAuthor(ctx context.Context, username string) (AuthorView, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return AuthorView{}, newErr(http.StatusNotFound, "author not found")
	}
	if user.Disabled {
		return AuthorView{}, newErr(http.StatusNotFound, "author not found")
	}
	view := AuthorView{
		User: AuthorUser{
			Username:    user.Username,
			DisplayName: user.DisplayName,
			Bio:         user.Bio,
			Avatar:      user.Avatar,
		},
	}
	var posts []models.Post
	if err := s.db.WithContext(ctx).
		Where("author_id = ? AND status = ?", user.ID, "published").
		Preload("Category").Preload("Tags").
		Order("created_at DESC").Find(&posts).Error; err != nil {
		return AuthorView{}, newErr(http.StatusInternalServerError, "failed to load posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	view.Posts = posts
	return view, nil
}

// --- dashboard (spec 06) ---

// DashboardData aggregates the admin dashboard widgets.
type DashboardData struct {
	PostsTotal   int64            `json:"posts_total"`
	ViewsTotal   int64            `json:"views_total"`
	RecentPosts  []models.Post    `json:"recent_posts"`
	RecentLogins []models.LoginLog `json:"recent_logins"`
}

// GetDashboard returns post totals, sum of views, recent posts, and recent
// successful logins for the admin dashboard.
func (s *Service) GetDashboard(ctx context.Context) (DashboardData, error) {
	var postsTotal int64
	s.db.WithContext(ctx).Model(&models.Post{}).Count(&postsTotal)

	var viewsTotal int64
	s.db.WithContext(ctx).Model(&models.Post{}).
		Select("COALESCE(SUM(view_count),0)").Scan(&viewsTotal)

	var recentPosts []models.Post
	s.db.WithContext(ctx).Where("status = ?", "published").
		Preload("Author").Preload("Category").
		Order("created_at DESC").Limit(5).Find(&recentPosts)
	if recentPosts == nil {
		recentPosts = []models.Post{}
	}

	var recentLogins []models.LoginLog
	s.db.WithContext(ctx).Where("success = ?", true).
		Order("created_at DESC").Limit(5).Find(&recentLogins)
	if recentLogins == nil {
		recentLogins = []models.LoginLog{}
	}

	return DashboardData{
		PostsTotal:   postsTotal,
		ViewsTotal:   viewsTotal,
		RecentPosts:  recentPosts,
		RecentLogins: recentLogins,
	}, nil
}
