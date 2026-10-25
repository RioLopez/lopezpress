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
				ID: 1, SiteName: "LopezPress", SiteTitle: "LopezPress",
				ImgHostMode: "local", AllowRegister: false, BackupKeep: 7,
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
	if err := validateSiteConfig(in); err != nil {
		return models.SiteConfig{}, err
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
	// Spec 24: page size is a closed set (5/10/20/30). Clamping here — rather
	// than rejecting — keeps an API client that sends 25 from bricking the
	// save while still guaranteeing the public list never queries a huge page.
	existing.PageSize = ClampPageSize(in.PageSize)
	// Spec 34: 1..90 days of retention. A zero/negative value would mean
	// "delete every snapshot", which is never what an operator means.
	existing.AutoBackup = in.AutoBackup
	existing.BackupKeep = ClampBackupKeep(in.BackupKeep)
	// Spec 27: anonymous-read switch.
	existing.PublicRead = in.PublicRead
	applyStoragePolicy(&existing, in)
	applyEmailVerifyPolicy(&existing, in)
	existing.UpdatedAt = time.Now()
	if err := s.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return models.SiteConfig{}, newErr(http.StatusInternalServerError, "failed to save site config")
	}
	// The gate caches this flag; a toggle must take effect on the next
	// request without a restart.
	s.invalidatePublicRead()
	s.audit(ctx, models.User{}, "update_site_config", "site_config", "")
	return existing, nil
}

// clampInt64 keeps n inside [lo, hi]. Used for the storage-policy numbers,
// where rejection would break an otherwise valid settings save but an
// out-of-range value could disable the protection entirely.
func clampInt64(n, lo, hi int64) int64 {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
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
	mode := cfgStr(m, "img_host_mode")
	if mode != "smms" {
		mode = "local"
	}
	return models.SiteConfig{
		SiteName:      cfgStr(m, "site_name"),
		SiteTitle:     cfgStr(m, "site_title"),
		SiteLogo:      cfgStr(m, "site_logo"),
		SiteFavicon:   cfgStr(m, "site_favicon"),
		SiteURL:       cfgStr(m, "site_url"),
		FooterText:    cfgStr(m, "footer_text"),
		IcpNo:         cfgStr(m, "icp_no"),
		FriendLinks:   cfgStr(m, "friend_links"),
		ImgHostMode:   mode,
		ImgHostToken:  cfgStr(m, "img_host_token"),
		AllowRegister: cfgBool(m, "allow_register"),
		// Spec 24: absent/invalid page_size -> 0, which UpdateSiteConfig
		// clamps back to the default.
		PageSize:   cfgInt(m, "page_size"),
		AutoBackup: cfgBool(m, "auto_backup"),
		BackupKeep: cfgInt(m, "backup_keep"),
		// Spec 27: absent -> false. The admin form always sends the current
		// value, so a missing field means a hand-built payload; false is the
		// safe (locked-down) reading in that case.
		PublicRead: cfgBool(m, "public_read"),
		SmtpHost:   cfgStr(m, "smtp_host"),
		SmtpPort:   cfgInt(m, "smtp_port"),
		SmtpUser:   cfgStr(m, "smtp_user"),
		SmtpPass:   cfgStr(m, "smtp_pass"),
		// Spec 36: byte quotas. They need their own getter because the fields
		// are int64 — reusing getInt would truncate a 500 MB quota on a 32-bit
		// build and, worse, made the whole policy unreachable (the field was
		// simply never decoded).
		UploadMaxImage:        cfgInt64(m, "upload_max_image"),
		UploadMaxAvatar:       cfgInt64(m, "upload_max_avatar"),
		UserQuotaBytes:        cfgInt64(m, "user_quota_bytes"),
		SiteQuotaBytes:        cfgInt64(m, "site_quota_bytes"),
		AllowUserDeleteUpload: cfgBool(m, "allow_user_delete_upload"),
		AllowUserChangeAvatar: cfgBool(m, "allow_user_change_avatar"),
		AllowExternalAvatar:   cfgBool(m, "allow_external_avatar"),
		// Spec 37: email verification. The flag itself is a bool that defaults
		// to false (= the feature is off), which is also what "the client did
		// not send it" decodes to — deliberately the safe direction here.
		EmailVerifyEnabled: cfgBool(m, "email_verify_enabled"),
		EmailCodeTTLMin:    cfgInt(m, "email_code_ttl_min"),
		EmailCodeCooldown:  cfgInt(m, "email_code_cooldown"),
	}
}

// cfgStr/cfgBool/cfgInt/cfgInt64 read one optional key of a decoded JSON body.
// Split out of SiteConfigFromMap so that function stays a single readable
// literal list — it drifted twice before (smtp_pass, then the spec-36 quotas)
// precisely because adding a field meant editing three places in one long body.
func cfgStr(m map[string]any, k string) string {
	if s, ok := m[k].(string); ok {
		return s
	}
	return ""
}

func cfgBool(m map[string]any, k string) bool {
	if b, ok := m[k].(bool); ok {
		return b
	}
	return false
}

func cfgInt(m map[string]any, k string) int {
	return int(cfgInt64(m, k))
}

func cfgInt64(m map[string]any, k string) int64 {
	switch n := m[k].(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i
		}
	}
	return 0
}

// maxLoginIdentifierLen bounds the identifier written to login_logs. The
// column declares size:128 but SQLite does not enforce it, so without this
// clamp a login attempt carrying a multi-megabyte "email" field stores the
// whole string: R3-05 measured two such requests growing the database by
// 53 MB and leaving a 26,214,407-byte row that made every login-history
// query pull it back.
const maxLoginIdentifierLen = 254

// clampIdentifier trims an identifier to the log limit. Kept as a helper so
// the login path and any future caller share one bound.
func clampIdentifier(s string) string {
	if len(s) <= maxLoginIdentifierLen {
		return s
	}
	return s[:maxLoginIdentifierLen]
}

func (s *Service) RecordLogin(ctx context.Context, userID, identifier, ip, ua string, success bool) {
	log := models.LoginLog{
		UserID: userID, Identifier: clampIdentifier(identifier), IP: clampIdentifier(ip), UA: clampIdentifier(ua), Success: success,
	}
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		_ = err // non-fatal
	}
}

// ListActions returns the most recent action_log rows (optionally filtered
// by an actor). Admin endpoint — the route table enforces the role (P4).
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

// validateSiteConfig runs every field check that does not need the stored row.
// Split out so UpdateSiteConfig reads as "validate, merge, save" instead of a
// hundred lines of guards with the actual work buried at the end.
func validateSiteConfig(in models.SiteConfig) error {
	if strings.TrimSpace(in.SiteName) == "" {
		return newErr(http.StatusBadRequest, "site name is required")
	}
	if len([]rune(in.SiteName)) > 64 {
		return newErr(http.StatusBadRequest, "site name too long")
	}
	if len([]rune(in.SiteTitle)) > 128 {
		return newErr(http.StatusBadRequest, "site title too long")
	}
	if in.ImgHostMode != "local" && in.ImgHostMode != "smms" {
		return newErr(http.StatusBadRequest, "invalid image host mode")
	}
	// R1-05: turning email verification on with no mail server configured was
	// accepted with a 200 and then broke the feature at runtime — every
	// /api/auth/email-code call answered 503 "SMTP is not configured", and with
	// registration open the whole sign-up flow was unusable. The two settings
	// are only meaningful together, so say so at save time rather than letting
	// the operator discover it from a user's failed sign-up.
	if in.EmailVerifyEnabled && strings.TrimSpace(in.SmtpHost) == "" {
		return newErr(http.StatusBadRequest,
			"email verification needs an SMTP host; configure the mail server first")
	}
	// SiteURL, when set, must be an absolute http(s) URL so feed/sitemap
	// can build correct absolute links.
	if u := strings.TrimSpace(in.SiteURL); u != "" {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			return newErr(http.StatusBadRequest, "site url must start with http:// or https://")
		}
	}
	// Validate friend_links: well-formed JSON array, and every url must be an
	// absolute http(s) link. The footer renders these into <a href> verbatim,
	// so a "javascript:" entry is a stored-XSS primitive that fires on click.
	if err := validateFriendLinks(in.FriendLinks); err != nil {
		return err
	}
	return nil
}

