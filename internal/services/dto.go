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
// field may carry either the username or the email (per existing behaviour).
type LoginInput struct {
	Email    string `json:"email"`
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
	Category int64
	Tag      string
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
