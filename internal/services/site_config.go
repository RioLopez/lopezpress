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
	// Validate friend_links: well-formed JSON array, and every url must be an
	// absolute http(s) link. The footer renders these into <a href> verbatim,
	// so a "javascript:" entry is a stored-XSS primitive that fires on click.
	if strings.TrimSpace(in.FriendLinks) != "" {
		var links []map[string]string
		if err := json.Unmarshal([]byte(in.FriendLinks), &links); err != nil {
			return models.SiteConfig{}, newErr(http.StatusBadRequest, "friend links must be a JSON array")
		}
		if len(links) > 50 {
			return models.SiteConfig{}, newErr(http.StatusBadRequest, "too many friend links (max 50)")
		}
		for _, l := range links {
			u := strings.TrimSpace(l["url"])
			low := strings.ToLower(u)
			if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
				return models.SiteConfig{}, newErr(http.StatusBadRequest, "friend link url must start with http:// or https://")
			}
			if len([]rune(u)) > 500 {
				return models.SiteConfig{}, newErr(http.StatusBadRequest, "friend link url too long")
			}
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
		SiteName:      getStr("site_name"),
		SiteTitle:     getStr("site_title"),
		SiteLogo:      getStr("site_logo"),
		SiteFavicon:   getStr("site_favicon"),
		SiteURL:       getStr("site_url"),
		FooterText:    getStr("footer_text"),
		IcpNo:         getStr("icp_no"),
		FriendLinks:   getStr("friend_links"),
		ImgHostMode:   mode,
		ImgHostToken:  getStr("img_host_token"),
		AllowRegister: getBool("allow_register"),
		// Spec 24: absent/invalid page_size -> 0, which UpdateSiteConfig
		// clamps back to the default.
		PageSize:   getInt("page_size"),
		AutoBackup: getBool("auto_backup"),
		BackupKeep: getInt("backup_keep"),
		// Spec 27: absent -> false. The admin form always sends the current
		// value, so a missing field means a hand-built payload; false is the
		// safe (locked-down) reading in that case.
		PublicRead: getBool("public_read"),
		SmtpHost:   getStr("smtp_host"),
		SmtpPort:   getInt("smtp_port"),
		SmtpUser:   getStr("smtp_user"),
		SmtpPass:   getStr("smtp_pass"),
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
