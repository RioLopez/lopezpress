package handlers

import "lopezpress/internal/models"

// inheritAbsentSiteConfig copies the stored value for any key the payload
// omitted. Several fields have a zero value that is itself a meaningful setting
// (boolean switches default to off, branding slots to cleared), so "absent" must
// mean "unchanged", never "reset". The caller fetches the stored row once and
// passes it in; this only ever reads it.
func inheritAbsentSiteConfig(in map[string]any, cur models.SiteConfig, cfg *models.SiteConfig) {
	inheritSiteConfigSwitches(in, cur, cfg)
	inheritSiteConfigBranding(in, cur, cfg)
	inheritSiteConfigRootOnly(in, cur, cfg)
}

// inheritSiteConfigSwitches keeps the availability/behaviour switches at their
// stored values when the payload omits them.
func inheritSiteConfigSwitches(in map[string]any, cur models.SiteConfig, cfg *models.SiteConfig) {
	if _, ok := in["allow_user_delete_upload"]; !ok {
		cfg.AllowUserDeleteUpload = cur.AllowUserDeleteUpload
	}
	if _, ok := in["allow_user_change_avatar"]; !ok {
		cfg.AllowUserChangeAvatar = cur.AllowUserChangeAvatar
	}
	if _, ok := in["allow_external_avatar"]; !ok {
		cfg.AllowExternalAvatar = cur.AllowExternalAvatar
	}
	if _, ok := in["email_verify_enabled"]; !ok {
		cfg.EmailVerifyEnabled = cur.EmailVerifyEnabled
	}
	if _, ok := in["email_code_ttl_min"]; !ok {
		cfg.EmailCodeTTLMin = cur.EmailCodeTTLMin
	}
	if _, ok := in["email_code_cooldown"]; !ok {
		cfg.EmailCodeCooldown = cur.EmailCodeCooldown
	}
	if _, ok := in["public_read"]; !ok {
		cfg.PublicRead = cur.PublicRead
	}
	if _, ok := in["auto_backup"]; !ok {
		cfg.AutoBackup = cur.AutoBackup
	}
	if _, ok := in["backup_keep"]; !ok {
		cfg.BackupKeep = cur.BackupKeep
	}
	if _, ok := in["page_size"]; !ok {
		cfg.PageSize = cur.PageSize
	}
}

// inheritSiteConfigBranding keeps the presentation/branding fields at their
// stored values when the payload omits them. SiteConfigFromMap decodes a missing
// key as "", so without this a partial PUT would zero them.
func inheritSiteConfigBranding(in map[string]any, cur models.SiteConfig, cfg *models.SiteConfig) {
	if _, ok := in["site_name"]; !ok {
		cfg.SiteName = cur.SiteName
	}
	if _, ok := in["site_title"]; !ok {
		cfg.SiteTitle = cur.SiteTitle
	}
	if _, ok := in["site_logo"]; !ok {
		cfg.SiteLogo = cur.SiteLogo
	}
	if _, ok := in["site_favicon"]; !ok {
		cfg.SiteFavicon = cur.SiteFavicon
	}
	if _, ok := in["site_url"]; !ok {
		cfg.SiteURL = cur.SiteURL
	}
	if _, ok := in["site_description"]; !ok {
		cfg.SiteDescription = cur.SiteDescription
	}
	if _, ok := in["footer_text"]; !ok {
		cfg.FooterText = cur.FooterText
	}
	if _, ok := in["icp_no"]; !ok {
		cfg.IcpNo = cur.IcpNo
	}
	if _, ok := in["friend_links"]; !ok {
		cfg.FriendLinks = cur.FriendLinks
	}
	if _, ok := in["img_host_mode"]; !ok {
		cfg.ImgHostMode = cur.ImgHostMode
	}
}

// inheritSiteConfigRootOnly keeps root-only fields at their stored values when
// the payload omits them. SiteConfigFromMap decodes an absent key as the zero
// value, and the field-level authorisation in accounts.go only protects
// non-root admins. Before this, a root PUT carrying just {"page_size":10}
// silently switched allow_register off and blanked the SMTP host/port/user —
// the defect the 2026-11-06 external review recorded as F-09. Explicitly
// sending the key (even with a zero value) still applies it, so root can
// deliberately close registration or clear a setting. Storage quotas are
// deliberately absent here: services.applyStoragePolicy already inherits a
// non-positive value, so duplicating that rule would be dead code.
func inheritSiteConfigRootOnly(in map[string]any, cur models.SiteConfig, cfg *models.SiteConfig) {
	if _, ok := in["allow_register"]; !ok {
		cfg.AllowRegister = cur.AllowRegister
	}
	if _, ok := in["smtp_host"]; !ok {
		cfg.SmtpHost = cur.SmtpHost
	}
	if _, ok := in["smtp_port"]; !ok {
		cfg.SmtpPort = cur.SmtpPort
	}
	if _, ok := in["smtp_user"]; !ok {
		cfg.SmtpUser = cur.SmtpUser
	}
	if _, ok := in["smtp_pass"]; !ok {
		cfg.SmtpPass = cur.SmtpPass
	}
	if _, ok := in["img_host_token"]; !ok {
		cfg.ImgHostToken = cur.ImgHostToken
	}
}
