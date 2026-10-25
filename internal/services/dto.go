package services

import (
	"time"

	"lopezpress/internal/models"
)

// --- input / output DTOs ---

// RegisterInput is the decoded payload for POST /api/auth/register.
type RegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	// Code is the emailed verification code. Required only while the site has
	// email verification switched on (spec 37, experimental); ignored
	// otherwise so existing clients keep working.
	Code string `json:"code"`
}

// EmailCodeRequestInput asks the server to mail a one-time code.
// POST /api/auth/email-code (anonymous).
type EmailCodeRequestInput struct {
	Email   string `json:"email"`
	Purpose string `json:"purpose"` // register | reset_password
}

// PasswordResetInput completes a self-service reset. The code replaces the old
// password as proof of ownership.
// POST /api/auth/password/reset (anonymous).
type PasswordResetInput struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

// EmailChangeInput serves both halves of the verified email change:
// Code is empty for the request step and required for the confirm step.
// POST /api/me/email-change/{request,confirm}.
type EmailChangeInput struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// EmailCodePolicy is the effective (already clamped) timing policy for
// verification codes. A struct rather than three return values so a caller
// cannot swap the TTL and the cooldown at the call site.
type EmailCodePolicy struct {
	Enabled  bool
	TTL      time.Duration
	Cooldown time.Duration
}

// LoginInput is the decoded payload for POST /api/auth/login. The Email
// field may carry either the username or the email (per existing behaviour),
// and an explicit "username" key is also accepted as a fallback identifier —
// so a client can post the same field names it registered with, and the login
// limiter (which already reads both keys) never disagrees with the handler.
type LoginInput struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// CreatePostInput is the decoded payload for POST /api/posts.
//
// Spec 22: posts have two visibilities — "published" (public) and "private"
// (author-only). Scheduled publishing and drafts are no longer supported, so
// no scheduled_at field is accepted here.
type CreatePostInput struct {
	Title string `json:"title"`
	// Slug lets the client pin the permanent link; empty = derive from title.
	Slug       string   `json:"slug"`
	Body       string   `json:"body"`
	Status     string   `json:"status"`
	CategoryID *int64   `json:"category_id"`
	Tags       []string `json:"tags"`
	Pinned     *bool    `json:"pinned"`
	// Cover was missing here while the editor already sent it on create
	// (contract drift found in the round-33 cross-stack audit): Go's JSON
	// decoder silently drops unknown fields, so a cover chosen at creation
	// time vanished. UpdatePostInput has always had it; create now matches.
	Cover string `json:"cover"`
}

// ListPostsQuery carries the optional filters + pagination for ListPosts.
type ListPostsQuery struct {
	Status   string
	Search   string
	Page     int
	PageSize int
	// Category is the raw ?category= value: a numeric category id (the
	// pre-slug form, still supported) or a category name. Names make the
	// shareable URL human-readable — ?category=PHP instead of ?category=2 —
	// and match how ?tag=Go already works. The service resolves it, so the
	// handler stays free of lookups.
	Category string
	Tag      string
	// AuthorID scopes the result to a single author's posts. Used by the
	// personal center ("my page shows my content only"); empty = no scoping.
	AuthorID string
	// SortBy whitelists the sort column: "created_at" (default) or "views".
	// Order is "asc" or "desc" (default). Pinned posts always sort first.
	SortBy string
	Order  string
}

// PostPage is the paginated response for GET /api/posts.
type PostPage struct {
	Items    []models.Post `json:"items"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
}

// UserStats aggregates the current user's own post metrics for the personal
// center banner (spec 20): total post count + sum of view counts.
type UserStats struct {
	PostsTotal int64 `json:"posts_total"`
	ViewsTotal int64 `json:"views_total"`
}

// UpdatePostInput is the decoded payload for PUT /api/posts/{id}.
// Spec 22: same two-visibility model as CreatePostInput; no scheduled_at.
type UpdatePostInput struct {
	Title string `json:"title"`
	// Slug: empty means "leave the permanent link alone" (a link must not
	// change just because the title was edited).
	Slug       string   `json:"slug"`
	Body       string   `json:"body"`
	Status     string   `json:"status"`
	CategoryID *int64   `json:"category_id"`
	Tags       []string `json:"tags"`
	Pinned     *bool    `json:"pinned"`
	Cover      string   `json:"cover"`
}

// CreateCategoryInput is the decoded payload for POST /api/categories.
type CreateCategoryInput struct {
	Name string `json:"name"`
	// ParentID makes the new category a child (spec 27). nil/0 = top level.
	ParentID *int64 `json:"parent_id"`
}

// UpdateCategoryInput is the decoded payload for PUT /api/categories/{id}.
// ParentID semantics: omitted (nil) = keep the current parent; 0 = move to
// top level; other = move under that parent.
type UpdateCategoryInput struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id"`
}

// UpdateTagInput is the decoded payload for PUT /api/tags/{id} (spec 25).
type UpdateTagInput struct {
	Name string `json:"name"`
}

// UserWithDisabled is a user row including its admin-disabled flag and the
// round-19 profile/audit fields (display_name, last-login IP/at).
type UserWithDisabled struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	Email       string     `json:"email"`
	Role        string     `json:"role"`
	Disabled    bool       `json:"disabled"`
	DisplayName string     `json:"display_name"`
	LastLoginIP string     `json:"last_login_ip"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// TrashPost is a soft-deleted post that is still within the recovery window.
type TrashPost struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	AuthorID   string    `json:"author_id"`
	DeletedAt  time.Time `json:"deleted_at"`
	Status     string    `json:"status"`
	Pinned     bool      `json:"pinned"`
	ViewCount  int64     `json:"view_count"`
	ReviewNote string    `json:"review_note,omitempty"`
}

// BackupStats summarises the counts written into a backup dump.
type BackupStats struct {
	Users      int64 `json:"users"`
	Posts      int64 `json:"posts"`
	Categories int64 `json:"categories"`
	Tags       int64 `json:"tags"`
	// Revisions and UploadFiles are what a full backup carries that a content
	// migration does not: the edit history and the image library.
	Revisions   int64 `json:"revisions"`
	UploadFiles int64 `json:"upload_files"`
}

// RestoreSnapshotOptions carries the three gates a database restore must pass.
//
// They are not optional ergonomics: before this existed, one POST naming a
// stale or empty snapshot replaced the live site — content, accounts and all —
// with whatever that file happened to hold. ImportBackup already had a confirm
// phrase and a dry run; the strictly more destructive operation did not.
type RestoreSnapshotOptions struct {
	// Confirm must equal RestoreConfirmPhrase.
	Confirm string
	// DryRun reports what the snapshot holds and changes nothing.
	DryRun bool
	// AllowDataLoss permits restoring a snapshot that would delete every post.
	// It is a separate flag rather than a different phrase because the two
	// failures need different answers: a missing phrase is a client bug, while
	// this one is a human decision.
	AllowDataLoss bool
}

// ImportResult is the response shape for the enhanced import endpoint:
// either a dry-run stats preview or the applied BackupStats.
type ImportResult struct {
	DryRun  bool        `json:"dry_run"`
	Applied bool        `json:"applied"`
	Stats   BackupStats `json:"stats"`
	// SnapshotFile is the auto-created pre-import snapshot name (only when
	// the import was actually applied, never for dry runs).
	SnapshotFile string `json:"snapshot_file,omitempty"`
}

// Backup schema metadata written into every dump (spec #6). Importers verify
// these before applying so a dump from an incompatible app version is
// rejected up front rather than silently corrupting the database.
const (
	BackupSchemaVersion = 1
	BackupAppVersion    = "lopezpress-2026"
)

// BackupResult is returned by ExportBackup.
type BackupResult struct {
	Filename string      `json:"filename"`
	Size     int64       `json:"size"`
	Stats    BackupStats `json:"stats"`
	// Warning flags sensitive content so the admin UI can surface a
	// "store this securely" notice. Backups include bcrypt password hashes,
	// which are an offline-cracking target if the file leaks.
	Warning string `json:"warning,omitempty"`
}

// PostNav is the response for prev/next post navigation.
type PostNav struct {
	Prev *models.Post `json:"prev"`
	Next *models.Post `json:"next"`
}

// TagWithCount is a tag with its associated post count.
type TagWithCount struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// CategoryWithCount is a category row plus how many live, visible posts it
// holds. ParentID is included so the admin table and the public sidebar can
// both build the two-level tree without a second request (spec 27).
type CategoryWithCount struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id"`
	Count    int64  `json:"count"`
}

// UserPublic is the user-facing (sanitized) representation of models.User.
// It is the shape returned to clients (no password hash).
type UserPublic struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	Avatar      string `json:"avatar"`
	// MustChangePassword signals the client to redirect the user to the
	// "change password" screen after login (first-boot admin flow).
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt          time.Time `json:"created_at"`
}

// AuthResult is what Login returns: a signed JWT, its expiry, and the
// public view of the authenticated user.
type AuthResult struct {
	Token   string     `json:"token"`
	Expires time.Time  `json:"expires_at"`
	User    UserPublic `json:"user"`
}

// toPublic converts a models.User into the user-facing public shape.
func toPublic(u models.User) UserPublic {
	return UserPublic{
		ID:                 u.ID,
		Username:           u.Username,
		Email:              u.Email,
		Role:               u.Role,
		DisplayName:        u.DisplayName,
		Bio:                u.Bio,
		Avatar:             u.Avatar,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:          u.CreatedAt,
	}
}

// --- input / meta DTOs (converged from domain files, 2026-10-23) ---

// CreateUserInput is the decoded payload for POST /api/users.
type CreateUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserInput is the decoded payload for PUT /api/users/{id}.
type UpdateUserInput struct {
	Role     string `json:"role"`
	Disabled *bool  `json:"disabled"`
}

// ChangePasswordInput is the decoded payload for POST /api/auth/change-password.
type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UpdateMeInput is the decoded payload for PUT /api/auth/me.
//
// Pointer fields distinguish "absent" (nil = leave unchanged) from "explicit
// empty" (non-nil = clear the value), so a profile save can both change a
// field and wipe it back to empty without an ambiguous sentinel.
type UpdateMeInput struct {
	Username    *string `json:"username"`
	Email       *string `json:"email"`
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	Avatar      *string `json:"avatar"`
}

// LoginMeta carries request context (IP / User-Agent) used for the audit log.
type LoginMeta struct {
	IP string
	UA string
}

// ImportData is the decoded payload for POST /api/backup/import: the same
// shape ExportBackup produces. All slices are empty-safe (nil ok). The
// schema_version / app_version fields (spec #6) let the importer reject
// dumps from an incompatible app before touching the database.
type ImportData struct {
	SchemaVersion int    `json:"schema_version"`
	AppVersion    string `json:"app_version"`
	ExportedAt    string `json:"exported_at"`
	// Confirm must equal ImportConfirmPhrase before a non-dry-run import is
	// allowed to replace the database. It is not part of the exported dump:
	// the client adds it to the payload it re-submits after a dry run, so an
	// accidental "import this file" click cannot wipe the site on its own.
	Confirm        string            `json:"confirm,omitempty"`
	Users          []models.User     `json:"users"`
	PasswordHashes map[string]string `json:"password_hashes,omitempty"`
	Categories     []models.Category `json:"categories"`
	Tags           []models.Tag      `json:"tags"`
	Posts          []models.Post     `json:"posts"`
	PostTags       []models.PostTag  `json:"post_tags"`
	// The three slices below are what made a restore a *restore* rather than a
	// content migration. Before they existed, exporting a site and importing it
	// back silently dropped the whole site configuration (name, theme, SMTP,
	// quotas), every article's revision history, and the entire image library —
	// so a "backup" could not rebuild the site it came from.
	SiteConfigs []models.SiteConfig   `json:"site_configs,omitempty"`
	Revisions   []models.PostRevision `json:"revisions,omitempty"`
	UploadFiles []models.UploadFile   `json:"upload_files,omitempty"`
	// UploadLiveKeys carries the de-duplication key for each upload, keyed by
	// name. models.UploadFile marks LiveKey json:"-" (it is an internal key),
	// but the UNIQUE index on it means an import that left it empty would make
	// every row collide — and a dump that dropped it would silently break
	// de-duplication for every image restored from it.
	UploadLiveKeys map[string]string `json:"upload_livekeys,omitempty"`
}

// BatchPostInput is the decoded payload of POST /api/admin/posts/batch.
type BatchPostInput struct {
	IDs        []string `json:"ids"`
	Action     string   `json:"action"`
	CategoryID *int64   `json:"category_id,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// MarkdownFile is one uploaded markdown file.
type MarkdownFile struct {
	Name string
	Body []byte
}

// ReviewPostInput is the decoded payload for the review endpoint.
// Action is "approve" (publish) or "reject" (send back to the author with a
// note). Note is the reviewer's free-text comment, required for reject and
// optional for approve.
type ReviewPostInput struct {
	// Action is the review decision: "approve" | "reject". The JSON tag is
	// "decision" (the wire name used by the admin UI).
	Action string `json:"decision"` // "approve" | "reject"
	Note   string `json:"note"`
}

// ViewMeta carries the request-derived inputs for view de-duplication.
//
// Now is injectable so tests can move between days without sleeping; a zero
// value means time.Now().
type ViewMeta struct {
	IP  string
	UA  string
	Now time.Time
}

// --- response DTOs (converged from domain files, 2026-10-23) ---

// PresetResult reports what applying the preset tree actually changed, so the
// UI can say "created 5, re-parented 2" instead of a bare "done".
type PresetResult struct {
	Created    int `json:"created"`
	Reparented int `json:"reparented"`
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
	Posts []models.Post `json:"posts"`
}

// DashboardData aggregates the admin dashboard widgets.
type DashboardData struct {
	PostsTotal int64 `json:"posts_total"`
	ViewsTotal int64 `json:"views_total"`
	// Spec 33: the dashboard answers "is anyone reading?" not just "how many
	// posts exist". PendingTotal is what needs a human today; ViewTrend is the
	// last 30 days of de-duplicated reads; TopPosts is what people actually read.
	PendingTotal int64             `json:"pending_total"`
	ViewTrend    []DayViews        `json:"view_trend"`
	TopPosts     []TopPost         `json:"top_posts"`
	RecentPosts  []models.Post     `json:"recent_posts"`
	RecentLogins []models.LoginLog `json:"recent_logins"`
}

// DayViews is one day of the view trend. Days with no reads are present with
// Views = 0 so the client never has to fill gaps itself.
type DayViews struct {
	Day   string `json:"day"`
	Views int64  `json:"views"`
}

// TopPost is a most-read post. Only published posts qualify: a banned or
// pending post's read count is not something an author should be paid
// attention for.
type TopPost struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	ViewCount int64  `json:"view_count"`
}

// AutoBackupStatus is what the settings page shows about the daily snapshot.
type AutoBackupStatus struct {
	Enabled bool   `json:"enabled"`
	Keep    int    `json:"keep"`
	LastAt  string `json:"last_at"` // RFC3339, empty when never run
	Files   int    `json:"files"`
}

// BatchSkip records one post that could not be processed, with the
// human-readable reason (the service error message, in English per the error
// contract).
type BatchSkip struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// BatchPostResult is the response body: how many rows changed, and which were
// skipped and why. Partial success is deliberate — one 403 must not roll back
// the whole batch.
type BatchPostResult struct {
	Affected int         `json:"affected"`
	Skipped  []BatchSkip `json:"skipped"`
}

// MarkdownSkip records a file that was not imported, with the reason.
type MarkdownSkip struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// MarkdownImportResult is the response of POST /api/admin/import/markdown.
type MarkdownImportResult struct {
	Created int            `json:"created"`
	Skipped []MarkdownSkip `json:"skipped"`
}

// UploadResult is what an upload endpoint returns (spec 36). `deduped` tells
// the editor "the library already had this exact picture, nothing was stored
// twice", which it surfaces as a small notice instead of silently pretending a
// new file appeared.
type UploadResult struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	Size    int64  `json:"size"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Mime    string `json:"mime"`
	Kind    string `json:"kind"`
	Deduped bool   `json:"deduped"`
	// Remote marks an image that lives on the external host (SM.MS). It has no
	// row in upload_files: this instance does not own the bytes.
	Remote bool `json:"remote,omitempty"`
}

// UploadFileInfo is one row of the image library / "my uploads" list.
// OwnerName, OriginalName, IP and Sha256 are admin-visible metadata; none of
// them ever appears in the URL, which is just /uploads/<opaque-id>.<ext>.
type UploadFileInfo struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	URL          string     `json:"url"`
	ThumbURL     string     `json:"thumb_url"`
	Size         int64      `json:"size"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	Mime         string     `json:"mime"`
	Kind         string     `json:"kind"`
	OwnerID      string     `json:"owner_id"`
	OwnerName    string     `json:"owner_name"`
	OriginalName string     `json:"original_name"`
	IP           string     `json:"ip"`
	Sha256       string     `json:"sha256"`
	RefCount     int        `json:"ref_count"`
	Legacy       bool       `json:"legacy"`
	Deleted      bool       `json:"deleted"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// UploadStats are the storage counters shown above the gallery.
//
// DeletedBytes is reported separately on purpose: soft-deleted files still
// occupy the disk, so a site that looks 40% full to its users can be 90% full
// in reality. The admin needs to see both numbers to know whether "clean up"
// means "remove withdrawn uploads" or "the site is genuinely out of room".
type UploadStats struct {
	LiveCount    int64 `json:"live_count"`
	LiveBytes    int64 `json:"live_bytes"`
	DeletedCount int64 `json:"deleted_count"`
	DeletedBytes int64 `json:"deleted_bytes"`
	OrphanCount  int64 `json:"orphan_count"`
	TotalCount   int64 `json:"total_count"`
	TotalBytes   int64 `json:"total_bytes"`
}

// UploadPage is one page of the library plus the quota counters, so the page
// opens with a single request instead of three.
type UploadPage struct {
	Items          []UploadFileInfo `json:"items"`
	Total          int64            `json:"total"`
	Page           int              `json:"page"`
	Size           int              `json:"size"`
	Stats          UploadStats      `json:"stats"`
	UserUsedBytes  int64            `json:"user_used_bytes"`
	UserQuotaBytes int64            `json:"user_quota_bytes"`
	SiteUsedBytes  int64            `json:"site_used_bytes"`
	SiteQuotaBytes int64            `json:"site_quota_bytes"`
}

// UploadListQuery is the filter for both the admin gallery and "my uploads".
// OwnerID is what turns the admin view into a personal one; the handlers decide
// which of the two a caller gets, not the service.
type UploadListQuery struct {
	OwnerID string
	Kind    string
	// Status: "" (live only, the default), "deleted", or "all".
	Status string
	Page   int
	Size   int
}

// UploadDeleteResult tells the caller what a delete actually did. AffectedRefs
// is non-zero only for a hard delete of an image that articles embed — the UI
// uses it for the "this will break N articles" warning.
type UploadDeleteResult struct {
	Name         string `json:"name"`
	Hard         bool   `json:"hard"`
	AffectedRefs int64  `json:"affected_refs"`
}

// SnapshotInfo describes one VACUUM INTO snapshot file on disk.
type SnapshotInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// ViewResult is the body of POST /api/posts/{id}/view (spec 23).
//
// Counted=false comes with a Reason ("duplicate" / "author" / "bot") so the
// frontend can decide whether to show anything; the count is always the
// post's current value so the UI can update itself even when not counted.
type ViewResult struct {
	Counted   bool   `json:"counted"`
	ViewCount int64  `json:"view_count"`
	Reason    string `json:"reason,omitempty"`
}
