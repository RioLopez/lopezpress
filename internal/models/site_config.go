package models

// SiteConfig lives in its own file: it is the widest entity in the schema and
// every spec that adds a switch (page size, public read, backups, storage,
// email verification) lands here. models.go stays readable because of it.

import "time"

// SiteConfig is a single-row table (id is always 1) holding site-level
// settings: branding, footer/compliance text, image-host config, and the
// public-registration flag. The friend_links field stores a JSON array of
// {name,url} objects as text (see services for marshal/unmarshal helpers).
// Round 19 replaces the old browser-localStorage SM.MS token with the
// server-held img_host_token below.
type SiteConfig struct {
	ID            int64  `gorm:"primaryKey" json:"id"`
	SiteName      string `gorm:"size:128" json:"site_name"`
	SiteTitle     string `gorm:"size:128" json:"site_title"`
	SiteLogo      string `gorm:"size:512" json:"site_logo"`
	SiteFavicon   string `gorm:"size:512" json:"site_favicon"`
	SiteURL       string `gorm:"size:512" json:"site_url"`
	FooterText    string `gorm:"type:text" json:"footer_text"`
	IcpNo         string `gorm:"size:128" json:"icp_no"`
	FriendLinks   string `gorm:"type:text" json:"friend_links"`
	ImgHostMode   string `gorm:"size:16;default:'local'" json:"img_host_mode"`
	ImgHostToken  string `gorm:"type:text" json:"img_host_token"`
	AllowRegister bool   `gorm:"default:false" json:"allow_register"`
	// PageSize is the number of posts per page on the public list page
	// (spec 24). Admin-configurable (5/10/20/30, default 10). The server —
	// not the client — decides the page size so that ?page=N means the same
	// thing for every visitor and stays shareable.
	PageSize int `gorm:"default:10" json:"page_size"`
	// PublicRead decides whether anonymous visitors may read the site
	// (spec 27). true = public blog; false = everything behind the login.
	// Enforced in BOTH layers: WithOptionalAuth rejects anonymous reads when
	// it is false, and the SPA bounces to /login. Defaulting to true keeps the
	// "public blog" positioning; an admin can lock the site down in one click.
	PublicRead bool `gorm:"default:true" json:"public_read"`
	// AutoBackup / BackupKeep drive the daily snapshot (spec 34). Off by
	// default: an unrequested background job writing files is a surprise, and
	// the operator may keep backups at the filesystem level instead.
	AutoBackup bool `gorm:"default:false" json:"auto_backup"`
	BackupKeep int  `gorm:"default:7" json:"backup_keep"`
	// SMTP settings for security notifications (login-failure / password
	// change / new-location login). Empty host disables the feature. The
	// password is stored in the DB (admin-configurable) — acceptable because
	// the row is only readable by admins; the public endpoint omits it.
	SmtpHost  string    `gorm:"size:128" json:"smtp_host"`
	SmtpPort  int       `json:"smtp_port"`
	SmtpUser  string    `gorm:"size:128" json:"smtp_user"`
	SmtpPass  string    `gorm:"type:text" json:"smtp_pass"`
	// --- image storage policy (spec 36) ---
	//
	// Three ceilings, from narrow to wide: one file, one user, the whole site.
	// Without them a single account can fill the VPS disk, and a full disk on
	// a 1 GB-RAM box takes the database down with it.
	UploadMaxImage  int64 `gorm:"default:5242880" json:"upload_max_image"`   // 5 MB, article images
	UploadMaxAvatar int64 `gorm:"default:524288" json:"upload_max_avatar"`   // 512 KB, avatars
	UserQuotaBytes  int64 `gorm:"default:20971520" json:"user_quota_bytes"`  // 20 MB per account
	SiteQuotaBytes  int64 `gorm:"default:524288000" json:"site_quota_bytes"` // 500 MB total
	// AllowUserDeleteUpload lets an author withdraw their own uploads. The
	// delete is soft: the file stays on disk so published articles that embed
	// it keep rendering, but it disappears from every list and can never be
	// re-used by somebody else (dedup skips it). Admins can still hard-delete.
	AllowUserDeleteUpload bool `gorm:"default:true" json:"allow_user_delete_upload"`
	// AllowUserChangeAvatar / AllowExternalAvatar are profile-field gates:
	// the first hides the avatar picker from ordinary users, the second
	// decides whether a plain https URL is an accepted avatar value or only
	// something uploaded through this site (an external avatar is a tracking
	// pixel with a friendly name).
	AllowUserChangeAvatar bool      `gorm:"default:true" json:"allow_user_change_avatar"`
	AllowExternalAvatar   bool      `gorm:"default:false" json:"allow_external_avatar"`
	// --- email verification (spec 37) ---
	//
	// EMAIL VERIFICATION IS A TEST FEATURE: it is off by default, the admin UI
	// labels it as experimental, and nothing else in the product depends on it.
	// The reason it must default to off is that switching it on cannot be
	// undone by the app — every existing account has no verified address, so a
	// hard requirement would lock the whole site out. Keeping it a soft,
	// reversible toggle means the worst case is "registration asks for a code
	// again".
	//
	// It is also root-only (like allow_register and the SMTP credentials): the
	// switch decides whether anonymous visitors can make this server send
	// mail, which is an abuse surface, not a presentation preference.
	EmailVerifyEnabled bool `gorm:"default:false" json:"email_verify_enabled"`
	// EmailCodeTTLMin / EmailCodeCooldown are the two timings an operator may
	// want to tune: how long a code lives, and how often one address may ask
	// for a new one. Both are clamped on save so a typo cannot produce a
	// one-second TTL (nobody can type that fast) or a cooldown longer than the
	// TTL (the code would expire before another could be requested).
	EmailCodeTTLMin   int       `gorm:"default:10" json:"email_code_ttl_min"`
	EmailCodeCooldown int       `gorm:"default:60" json:"email_code_cooldown"`
	UpdatedAt         time.Time `json:"updated_at"`
}
