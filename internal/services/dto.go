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

// UploadImageInfo describes one locally-stored upload in the image library.
type UploadImageInfo struct {
	Name    string    `json:"name"`     // file name (relative to the upload dir)
	URL     string    `json:"url"`      // public /uploads/<name> URL
	Size    int64     `json:"size"`     // bytes
	ModTime time.Time `json:"mod_time"` // last modified
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
