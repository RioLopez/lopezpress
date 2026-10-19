package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Account role names. Three-level hierarchy (spec: role model):
//   - RootRole ("root"): single super-admin. There is exactly one in the whole
//     database (guaranteed by the 0003 migration + enforcement in the account
//     services). root can promote a user to admin, can ban any user's post, and
//     can see the banned vault. root cannot be created, demoted, disabled, or
//     deleted by any operation path.
//   - AdminRole ("admin"): ordinary admin. Can manage users, categories, tags,
//     and reviews; may review posts whose author is a plain user, but cannot
//     ban any post (banning is root-only). Posts they author go into the
//     pending queue for root review.
//   - UserRole ("user"): regular member. Posts they author go into the pending
//     queue for admin/root review.
//
// Role strings are stored verbatim in users.role (VARCHAR 16); the constants
// keep every comparison in one place so the "admin" literal stops drifting.
const (
	RoleRoot  = "root"
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// Post status names. The visibility axis expanded from public/private to a
// review workflow (spec: content moderation + banned vault):
//   - Published: public, visible to everyone (default outcome of approval).
//   - Private: author + admin/root only.
//   - Pending: in the review queue. Visible to the author and to admin/root.
//   - Rejected: sent back to the author for revision, with a reviewer note.
//     Visible to the author + admin/root; carries ReviewNote.
//   - Banned: hard-moderated by root. Visible to root ONLY (banned vault);
//     hidden from the author and other admins.
const (
	PostPublished = "published"
	PostPrivate   = "private"
	PostPending   = "pending"
	PostRejected  = "rejected"
	PostBanned    = "banned"
)

// User represents an account that can authenticate and author posts.
type User struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Email        string    `gorm:"uniqueIndex;size:128;not null" json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `gorm:"size:16;default:'user'" json:"role"`
	Disabled     bool      `gorm:"default:false" json:"disabled"`
	// Profile fields (round 19): display name, bio, avatar, last-login audit.
	DisplayName   string     `gorm:"size:128" json:"display_name"`
	Bio           string     `gorm:"type:text" json:"bio"`
	Avatar        string     `gorm:"size:512" json:"avatar"`
	LastLoginIP   string     `gorm:"size:64" json:"last_login_ip"`
	LastLoginAt   *time.Time `json:"last_login_at"`
	// MustChangePassword forces the account to change its password on next
	// login. The first-boot admin (seeded with a random console-banner
	// password) is created with this flag set; it is cleared by the
	// "change own password" endpoint once the user picks their own.
	MustChangePassword bool      `gorm:"default:false" json:"must_change_password"`
	// TokenVersion invalidates all previously issued JWTs when bumped, giving
	// the otherwise-stateless tokens a revocation primitive (e.g. "log out all
	// devices"). The version is embedded in every token's "ver" claim and
	// checked on authenticate; a mismatch → 401.
	TokenVersion int64 `gorm:"default:0" json:"-"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Posts         []Post     `gorm:"foreignKey:AuthorID" json:"-"`
}

// BeforeCreate sets the primary key and defaults.
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	if u.Role == "" {
		u.Role = "user"
	}
	return nil
}

// HashPassword returns the bcrypt hash of the given plain password.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// VerifyPassword reports whether plain matches the stored hash.
func VerifyPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Post represents a single content article.
type Post struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	Title      string    `gorm:"size:200;not null" json:"title"`
	Body       string    `json:"body"`
	Status     string    `gorm:"size:16;default:'published'" json:"status"`
	AuthorID   string    `gorm:"size:36;index" json:"author_id"`
	CategoryID *int64    `json:"category_id"`
	// belongs-to: posts.category_id references categories.id. The previous
	// tag `foreignKey:ID` made GORM resolve the relation backwards, which
	// migrated `categories` with a text primary key plus an FK pointing at
	// posts.id, leaving every category id NULL. Do not revert to that form.
	Category   *Category `gorm:"foreignKey:CategoryID;references:ID" json:"category,omitempty"`
	Author     *User     `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
	Tags       []Tag     `gorm:"many2many:post_tags" json:"tags"`
	Pinned     bool      `gorm:"default:false" json:"pinned"`
	Cover      string    `gorm:"size:512" json:"cover,omitempty"`
	ViewCount  int64     `gorm:"default:0" json:"view_count"`
	// Review metadata (spec: content moderation + banned vault). ReviewNote
	// is the reviewer's free-text comment, set when a post is rejected
	// (sent back for revision) or re-approved. ReviewedBy / ReviewedAt track
	// which admin/root acted and when. Populated for statuses
	// "rejected" and "banned"; cleared on approval to "published".
	ReviewNote string     `gorm:"type:text" json:"review_note,omitempty"`
	ReviewedBy string    `gorm:"size:36" json:"reviewed_by,omitempty"`
	ReviewedAt *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	// DeletedAt implements GORM soft-delete. When set, the post is excluded
	// from normal queries (gorm adds WHERE deleted_at IS NULL) and is
	// retrievable only with Unscoped(). Populated within 7 days for recovery.
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate sets the primary key and defaults for posts.
func (p *Post) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.Status == "" {
		p.Status = "published"
	}
	return nil
}

// Category is a named grouping for posts.
type Category struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:64;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Tag is a free-form label attached to posts via the post_tags join table.
type Tag struct {
	ID    int64     `gorm:"primaryKey" json:"id"`
	Name  string    `gorm:"uniqueIndex;size:64;not null" json:"name"`
	Posts []Post    `gorm:"many2many:post_tags" json:"-"`
}

// PostTag is the join table between Post and Tag (many-to-many).
type PostTag struct {
	PostID string `gorm:"primaryKey" json:"post_id"`
	TagID  int64  `gorm:"primaryKey" json:"tag_id"`
}

// SiteConfig is a single-row table (id is always 1) holding site-level
// settings: branding, footer/compliance text, image-host config, and the
// public-registration flag. The friend_links field stores a JSON array of
// {name,url} objects as text (see services for marshal/unmarshal helpers).
// Round 19 replaces the old browser-localStorage SM.MS token with the
// server-held img_host_token below.
type SiteConfig struct {
	ID           int64  `gorm:"primaryKey" json:"id"`
	SiteName     string `gorm:"size:128" json:"site_name"`
	SiteTitle    string `gorm:"size:128" json:"site_title"`
	SiteLogo     string `gorm:"size:512" json:"site_logo"`
	SiteFavicon  string `gorm:"size:512" json:"site_favicon"`
	SiteURL      string `gorm:"size:512" json:"site_url"`
	FooterText   string `gorm:"type:text" json:"footer_text"`
	IcpNo        string `gorm:"size:128" json:"icp_no"`
	FriendLinks  string `gorm:"type:text" json:"friend_links"`
	ImgHostMode  string `gorm:"size:16;default:'local'" json:"img_host_mode"`
	ImgHostToken string `gorm:"type:text" json:"img_host_token"`
	AllowRegister bool  `gorm:"default:false" json:"allow_register"`
	// SMTP settings for security notifications (login-failure / password
	// change / new-location login). Empty host disables the feature. The
	// password is stored in the DB (admin-configurable) — acceptable because
	// the row is only readable by admins; the public endpoint omits it.
	SmtpHost string `gorm:"size:128" json:"smtp_host"`
	SmtpPort int    `json:"smtp_port"`
	SmtpUser string `gorm:"size:128" json:"smtp_user"`
	SmtpPass string `gorm:"type:text" json:"smtp_pass"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// LoginLog records every login attempt (success or failure) for security
// auditing. A successful attempt carries user_id; a failed attempt carries the
// attempted identifier (username/email) with a nullable user_id. See spec 14.
type LoginLog struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	UserID     string    `gorm:"index" json:"user_id"`
	Identifier string    `gorm:"size:128" json:"identifier"`
	IP         string    `gorm:"size:64" json:"ip"`
	UA         string    `gorm:"type:text" json:"ua"`
	Success    bool      `gorm:"index" json:"success"`
	CreatedAt  time.Time `json:"created_at"`
}

// ActionLog records who did what on the site (content + admin operations).
// The LoginLog table handles authentication; this one handles post-auth
// mutations: create/delete/edit posts, manage users, restore/delete backups,
// etc. ActorID is the user who performed the action; Action is a short verb
// ("create_post" / "delete_user" / ...); Target is the affected resource
// ("post:<uuid>" / "user:<uuid>" / "backup:<file>" / ...); Detail holds an
// optional free-form summary (e.g. old→new title, post count).
type ActionLog struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	ActorID   string    `gorm:"size:36;index" json:"actor_id"`
	ActorName string    `gorm:"size:64" json:"actor_name"` // denormalized for display
	Action    string    `gorm:"size:40;index" json:"action"`
	Target    string    `gorm:"size:200" json:"target"`
	Detail    string    `gorm:"size:512" json:"detail"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// SchemaMigration tracks which one-shot manual migrations have been applied.
// AutoMigrate does not drop columns, so any column removal (e.g. the
// deprecated posts.scheduled_at, spec #8) is handled by a hand-written
// migration gated by a row in this table.
type SchemaMigration struct {
	Version   string    `gorm:"primaryKey;size:64" json:"version"`
	AppliedAt time.Time `json:"applied_at"`
}
