// Package services holds ALL business logic for the application. The
// HTTP handlers (internal/handlers) are a thin transport layer that
// delegates to *Service and maps returned errors to status codes.
package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/config"
	"lopezpress/internal/models"
)

// Service carries the shared dependencies (db + cfg) and implements every
// business operation. Handlers are thin: they decode/encode JSON and map
// ServiceErrors to HTTP status codes; they never touch gorm directly.
type Service struct {
	db  *gorm.DB
	cfg *config.Config

	// mailT is the per-user throttle state for login-failure notifications.
	// Guarded by the fact that SendMail is the only writer and reads are
	// confined to the single goroutine that spawned it; for simplicity the
	// map is only touched from the request goroutine (single instance).
	mailT *mailThrottler
}

// New builds a Service backed by db and cfg.
func New(db *gorm.DB, cfg *config.Config) *Service {
	return &Service{db: db, cfg: cfg, mailT: newMailThrottler()}
}

// --- error plumbing: map domain errors to HTTP status codes ---

// ServiceError is the typed error returned by every Service method. Handlers
// map Status to a concrete HTTP code (400/401/403/404/409/500) and use Msg
// for the JSON "error" body.
type ServiceError struct {
	Status int    // HTTP status code
	Msg    string // human-readable message for the client
}

func (e *ServiceError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("[%d] %s", e.Status, e.Msg)
}

// newErr is a small constructor.
func newErr(status int, msg string) *ServiceError {
	return &ServiceError{Status: status, Msg: msg}
}

// StatusOf extracts the HTTP status from a ServiceError. Non-ServiceError
// values (e.g. raw gorm errors that leaked through) map to 500.
func StatusOf(err error) int {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Status
	}
	return http.StatusInternalServerError
}

// MsgOf extracts the client-facing message. It unwraps ServiceError; for any
// other error it returns a generic message so internals are not leaked.
func MsgOf(err error) string {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Msg
	}
	return "internal server error"
}

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
	Title      string   `json:"title"`
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
	Items  []models.Post `json:"items"`
	Total  int64         `json:"total"`
	Page   int           `json:"page"`
	PageSize int          `json:"page_size"`
}

// UserStats aggregates the current user's own post metrics for the personal
// center banner (spec 20): total post count + sum of view counts.
type UserStats struct {
	PostsTotal int64 `json:"posts_total"`
	ViewsTotal int64 `json:"views_total"`
}

// UserStats returns the caller's post count and total views. Admin stats are
// scoped to the admin's own posts (author_id = user.ID), matching the
// personal-center "this is my page" semantics.
func (s *Service) UserStats(ctx context.Context, user models.User) (UserStats, error) {
	var postsTotal int64
	if err := s.db.WithContext(ctx).
		Model(&models.Post{}).
		Where("author_id = ?", user.ID).
		Count(&postsTotal).Error; err != nil {
		return UserStats{}, newErr(http.StatusInternalServerError, "failed to count posts")
	}

	var viewsTotal int64
	if err := s.db.WithContext(ctx).
		Model(&models.Post{}).
		Where("author_id = ?", user.ID).
		Select("COALESCE(SUM(view_count), 0)").
		Scan(&viewsTotal).Error; err != nil {
		return UserStats{}, newErr(http.StatusInternalServerError, "failed to sum views")
	}

	return UserStats{PostsTotal: postsTotal, ViewsTotal: viewsTotal}, nil
}

// UpdatePostInput is the decoded payload for PUT /api/posts/{id}.
// Spec 22: same two-visibility model as CreatePostInput; no scheduled_at.
type UpdatePostInput struct {
	Title      string   `json:"title"`
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
}

// UpdateCategoryInput is the decoded payload for PUT /api/categories/{id}.
type UpdateCategoryInput struct {
	Name string `json:"name"`
}

// UserWithDisabled is a user row including its admin-disabled flag and the
// round-19 profile/audit fields (display_name, last-login IP/at).
type UserWithDisabled struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	Disabled     bool       `json:"disabled"`
	DisplayName  string     `json:"display_name"`
	LastLoginIP  string     `json:"last_login_ip"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
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
	DryRun bool         `json:"dry_run"`
	Applied bool        `json:"applied"`
	Stats  BackupStats  `json:"stats"`
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

// CategoryWithCount is a category with its associated post count.
type CategoryWithCount struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

// UserPublic is the user-facing (sanitized) representation of models.User.
// It is the shape returned to clients (no password hash).
type UserPublic struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	Role        string    `json:"role"`
	DisplayName string    `json:"display_name"`
	Bio         string    `json:"bio"`
	Avatar      string    `json:"avatar"`
	// MustChangePassword signals the client to redirect the user to the
	// "change password" screen after login (first-boot admin flow).
	MustChangePassword bool      `json:"must_change_password"`
	CreatedAt   time.Time `json:"created_at"`
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
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Role:        u.Role,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		Avatar:      u.Avatar,
		MustChangePassword: u.MustChangePassword,
		CreatedAt:   u.CreatedAt,
	}
}

// --- auth ---

// Register validates, hashes and creates a new user.
// Returns 409 if the username or email is already taken.
// Register creates a new user account and signs them straight in, returning
// the same AuthResult shape as Login. Spec 04 AC-2.2 requires registration to
// log the user in automatically, so the client can drop straight into the app
// without a second round trip to /api/auth/login.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	if in.Username == "" || in.Email == "" || in.Password == "" {
		return nil, newErr(http.StatusBadRequest, "username, email and password are required")
	}
	if len(in.Password) < 8 {
		return nil, newErr(http.StatusBadRequest, "password must be at least 8 characters")
	}

	// Public registration is gated by the site-level allow_register flag
	// (default false for a single-user site). Admins create users via the
	// separate POST /api/users endpoint, which is NOT subject to this gate.
	if !s.allowRegister(ctx) {
		return nil, newErr(http.StatusForbidden, "registration is disabled")
	}

	var existing models.User
	if s.db.WithContext(ctx).Where("username = ? OR email = ?", in.Username, in.Email).
		First(&existing).RowsAffected == 1 {
		return nil, newErr(http.StatusConflict, "username or email already in use")
	}

	hash, err := models.HashPassword(in.Password)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to hash password")
	}

	u := models.User{Username: in.Username, Email: in.Email, PasswordHash: hash}
	if err := s.db.WithContext(ctx).Create(&u).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to create user")
	}

	token, expires, err := s.SignToken(u.ID, u.Role, u.TokenVersion)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to issue token")
	}

	return &AuthResult{
		Token:   token,
		Expires: expires,
		User:    toPublic(u),
	}, nil
}

// LoginMeta carries request context (IP / User-Agent) used for the audit log.
type LoginMeta struct {
	IP string
	UA string
}

// Login verifies credentials (by username OR email) and returns a signed JWT
// along with the public view of the user. Every attempt (success or failure)
// is recorded in login_log, and a successful login updates the user's
// last_login_ip / last_login_at (spec 14).
func (s *Service) Login(ctx context.Context, in LoginInput, meta LoginMeta) (*AuthResult, error) {
	if in.Email == "" || in.Password == "" {
		return nil, newErr(http.StatusBadRequest, "email and password are required")
	}

	recordFail := func(userID, identifier string, code int, msg string) (*AuthResult, error) {
		s.RecordLogin(ctx, userID, identifier, meta.IP, meta.UA, false)
		return nil, newErr(code, msg)
	}

	var user models.User
	// Match by username OR email so the client can send either in the same field.
	if err := s.db.WithContext(ctx).
		Where("username = ? OR email = ?", in.Email, in.Email).
		First(&user).Error; err != nil {
		return recordFail("", in.Email, http.StatusUnauthorized, "invalid credentials")
	}
	if user.Disabled {
		return recordFail(user.ID, in.Email, http.StatusForbidden, "account is disabled")
	}
	if !models.VerifyPassword(in.Password, user.PasswordHash) {
		// Security notification: notify the account owner of a failed attempt
		// (throttled to 15 min / user so brute force can't spam the inbox).
		s.SendMail(ctx, user, MailLoginFailure, map[string]string{
			"ip": meta.IP, "time": time.Now().Format("2006-01-02 15:04 UTC"), "ua": meta.UA,
		})
		return recordFail(user.ID, in.Email, http.StatusUnauthorized, "invalid credentials")
	}

	// Success: record audit + update last-login fields (best-effort).
	// Detect a "new location" (IP or browser differs from the previous login)
	// and notify the owner, then persist the new last-login state.
	s.notifyNewLocation(ctx, user, meta)
	s.RecordLogin(ctx, user.ID, in.Email, meta.IP, meta.UA, true)
	_ = s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"last_login_ip": meta.IP,
		"last_login_at": time.Now(),
	}).Error

	token, expires, err := s.SignToken(user.ID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to issue token")
	}

	return &AuthResult{
		Token:   token,
		Expires: expires,
		User:    toPublic(user),
	}, nil
}

// SignToken issues a signed HS256 JWT for the given user, using cfg.JWTSecret
// (with a documented dev fallback) and cfg.JWTExpiryHours (default 24h). The
// supplied tokenVersion is embedded as the "ver" claim so that bumping a
// user's TokenVersion (e.g. on "log out all devices") invalidates the token.
func (s *Service) SignToken(userID, role string, tokenVersion int64) (string, time.Time, error) {
	secret := s.signingSecret()
	expiryHours := 24
	if s.cfg != nil && s.cfg.JWTExpiryHours > 0 {
		expiryHours = s.cfg.JWTExpiryHours
	}
	expires := time.Now().Add(time.Duration(expiryHours) * time.Hour)
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"ver":  tokenVersion,
		"exp":  expires.Unix(),
		"iat":  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(secret))
	return token, expires, err
}

// signingSecret returns the JWT signing secret, falling back to the
// documented dev default when cfg is unset.
func (s *Service) signingSecret() string {
	if s.cfg != nil && s.cfg.JWTSecret != "" {
		return s.cfg.JWTSecret
	}
	return "dev-secret-change-me-in-prod"
}

// Authenticate parses and validates a raw JWT (from an Authorization header
// value, with or without the "Bearer " prefix) and loads the matching user.
// Used by the WithAuth handler to resolve the caller.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (models.User, error) {
	claims, err := s.parseToken(rawToken, s.signingSecret())
	if err != nil {
		return models.User{}, newErr(http.StatusUnauthorized, "invalid or expired token")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", claims["sub"]).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusUnauthorized, "user not found")
	}
	// Token-version check: bumping a user's TokenVersion (e.g. on "log out all
	// devices") revokes every previously issued token. New tokens always carry
	// a "ver" claim; legacy tokens (no claim) are only accepted while the
	// stored version is still 0, so the first deploy is backward compatible.
	if raw, ok := claims["ver"]; ok {
		var ver int64
		switch v := raw.(type) {
		case float64:
			ver = int64(v)
		case int64:
			ver = v
		}
		if ver != user.TokenVersion {
			return models.User{}, newErr(http.StatusUnauthorized, "token revoked")
		}
	} else if user.TokenVersion != 0 {
		return models.User{}, newErr(http.StatusUnauthorized, "token revoked")
	}
	// Account-state gate: a disabled account loses API access immediately,
	// not only at the next login attempt. Login already refuses disabled
	// accounts, but a stateless JWT remains valid until expiry (up to 24h) —
	// without this gate, disabling a compromised or abusive account would
	// leave every session it already holds fully usable (including writes)
	// until the token happens to expire. This is the Authenticate-level
	// counterpart of the must_change_password gate in the handlers.
	// (as-built audit Q6)
	if user.Disabled {
		return models.User{}, newErr(http.StatusForbidden, "account is disabled")
	}
	return user, nil
}

// parseToken parses and validates a JWT. Accepts both "Bearer <token>" and
// bare token forms.
func (s *Service) parseToken(header, secret string) (jwt.MapClaims, error) {
	raw := header
	if len(header) > 7 && header[:7] == "Bearer " {
		raw = header[7:]
	}
	t, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid {
		// jwt.Parse can report an invalid signature with a nil error
		// (only t.Valid is set), so surface a concrete error to keep the
		// 401 mapping in Authenticate intact.
		if err == nil {
			err = fmt.Errorf("invalid token signature")
		}
		return nil, err
	}
	claims, _ := t.Claims.(jwt.MapClaims)
	return claims, nil
}

// --- posts ---

// CreatePost creates a post owned by user. AuthorID is set from user.ID.
// The returned post has Author preloaded so the client sees the author.
//
// Role-based initial status (spec: review workflow):
//   - root: published (no review needed).
//   - admin / user: pending (enters the review queue; admin posts are
//     reviewed by root, user posts by admin or root).
//
// An explicit status is only honoured when it is one of published / private
// AND the caller is root. For admin/user the status is forced to "pending"
// so a tampered request cannot bypass the review queue.
func (s *Service) CreatePost(ctx context.Context, user models.User, in CreatePostInput) (models.Post, error) {
	if in.Title == "" {
		return models.Post{}, newErr(http.StatusBadRequest, "title is required")
	}
	if strings.TrimSpace(in.Body) == "" {
		return models.Post{}, newErr(http.StatusBadRequest, "post body must not be empty")
	}
	// Resolve the initial status by role.
	var status string
	switch user.Role {
	case models.RoleRoot:
		// Root publishes or keeps private as requested.
		switch in.Status {
		case "", models.PostPublished:
			status = models.PostPublished
		case models.PostPrivate:
			status = models.PostPrivate
		default:
			status = models.PostPublished
		}
	default:
		// admin / user / unknown: always enter the pending review queue.
		status = models.PostPending
	}

	post := models.Post{
		Title:      in.Title,
		Body:       in.Body,
		Status:     status,
		AuthorID:   user.ID,
		CategoryID: in.CategoryID,
		Cover:      in.Cover,
	}
	if in.Pinned != nil {
		post.Pinned = *in.Pinned
	}
	// Create the post and its tag associations atomically: if the tag
	// association step fails, the whole post (incl. its row) is rolled back so
	// we never end up with a "saved but tagless" orphan. Association errors are
	// propagated now instead of being silently swallowed.
	var created models.Post
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		if len(in.Tags) > 0 {
			tags, ferr := s.findOrCreateTagsTx(ctx, tx, in.Tags)
			if ferr != nil {
				return ferr
			}
			if err := linkPostTagsTx(tx, post.ID, tags); err != nil {
				return err
			}
		}
		created = post
		return nil
	})
	if err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to create post")
	}

	// Preload author and category so the returned post carries the full objects.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", created.ID).First(&created).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  user,
		Action: "create_post",
		Target: "post:" + created.ID,
		Detail: "title=" + created.Title + "; status=" + created.Status,
	})
	return created, nil
}

// GetPost returns a single post by id with the author preloaded.
// Returns 404 when the post does not exist or the caller may not see it.
// Increments view_count.
//
// Visibility by status (spec: review + banned vault):
//   - published: everyone (incl. anonymous).
//   - private / pending / rejected: author, admin, or root.
//   - banned: root only (the banned vault is invisible to the author and to
//     other admins).
func (s *Service) GetPost(ctx context.Context, user models.User, id string) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	if !canSeePost(user, post) {
		// Same 404 as "not found" so we never leak the post's existence.
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	// Increment view count (best-effort, ignore errors).
	_ = s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
	post.ViewCount++
	return post, nil
}

// canSeePost reports whether the caller may view the given post, by its status
// and the caller's role/id. It is the single source of truth shared by the
// single-post read path and the list path so the two never drift apart.
func canSeePost(user models.User, post models.Post) bool {
	switch post.Status {
	case models.PostPublished:
		return true // public
	case models.PostBanned:
		// Banned vault: only root sees it.
		return user.Role == models.RoleRoot
	case models.PostPrivate, models.PostPending, models.PostRejected:
		// Restricted content: the author, or any admin/root, may read.
		return post.AuthorID == user.ID || user.Role == models.RoleAdmin || user.Role == models.RoleRoot
	default:
		return false
	}
}

// ListPosts returns posts ordered newest first, filtered by author_id unless
// the caller is an admin. Returns the FULL matching set (no pagination).
// Kept as an internal helper for tests; production code should use ListPostsPage.
func (s *Service) ListPosts(ctx context.Context, user models.User, status string) ([]models.Post, error) {
	page, err := s.ListPostsPage(ctx, user, ListPostsQuery{Status: status, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// ListArchive returns every published post for the /archive page, newest
// first. The author and category are preloaded for display, but the body is
// excluded to keep the payload small on the low-budget VPS.
//
// Browsable anonymously (spec 22): the archive only ever lists posts whose
// status is "published", which is by definition public — so every visitor
// (logged-out or logged-in) sees the same full public set. The user
// parameter is kept for call-signature stability but no longer scopes the
// result, because there is no "published-but-restricted" state.
func (s *Service) ListArchive(ctx context.Context, user models.User) ([]models.Post, error) {
	_ = user
	q := s.db.WithContext(ctx).Model(&models.Post{}).Where("posts.status = ?", "published").
		Preload("Author").Preload("Category").Preload("Tags").
		Order("posts.created_at DESC")
	var posts []models.Post
	if err := q.Find(&posts).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load archive")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	// Clear the heavy body field so the archive response stays lean.
	for i := range posts {
		posts[i].Body = ""
	}
	return posts, nil
}

// FeedPosts returns the most recent published posts (Author preloaded) for
// the RSS feed and sitemap. Published posts are, by definition, public, so
// this never scopes by caller. The body is retained because the feed uses a
// short excerpt of it.
func (s *Service) FeedPosts(ctx context.Context, limit int) ([]models.Post, error) {
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	var posts []models.Post
	if err := s.db.WithContext(ctx).Model(&models.Post{}).
		Where("status = ?", "published").
		Preload("Author").
		Order("created_at DESC").
		Limit(limit).
		Find(&posts).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load feed")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// GetPostPublic returns a published post without requiring authentication.
// Only posts with status "published" are accessible. Increments view_count.
func (s *Service) GetPostPublic(ctx context.Context, id string) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").
		Where("id = ? AND status = ?", id, "published").First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	_ = s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
	post.ViewCount++
	return post, nil
}

// escapeLike escapes SQL LIKE wildcards (% and _) and the escape character
// itself (\) so untrusted search input is treated as literal text. Pair it
// with a "LIKE ? ESCAPE '\'" clause.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// ListPostsPage returns a paginated slice of posts, supporting status +
// keyword search + page/size + category + tag filters.
// Pinned posts appear first (within the same status), ordered by created_at DESC.
func (s *Service) ListPostsPage(ctx context.Context, user models.User, q ListPostsQuery) (PostPage, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}

	applyFilters := func(db *gorm.DB) *gorm.DB {
		query := db.WithContext(ctx).Model(&models.Post{})
		switch {
		case user.Role == models.RoleRoot:
			// root: sees every post, including the banned vault.
		case user.Role == models.RoleAdmin:
			// admin: sees everything EXCEPT the banned vault (root-only).
			// pending / rejected / private are all visible.
			query = query.Where("posts.status <> ?", models.PostBanned)
		case user.ID != "":
			// Signed-in member (user): public posts PLUS their own
			// restricted posts (private / pending / rejected). They never see
			// other people's restricted content nor the banned vault.
			query = query.Where(
				"(posts.status = ? OR posts.author_id = ?) AND posts.status <> ?",
				models.PostPublished, user.ID, models.PostBanned)
		default:
			// Anonymous visitor: only public posts are browseable.
			query = query.Where("posts.status = ?", models.PostPublished)
		}
		if q.Status != "" {
			query = query.Where("posts.status = ?", q.Status)
		}
		if q.Search != "" {
			// Full-text search over title/body/author username. The driver
			// here is a pure-Go SQLite build that does not ship FTS5, so we
			// use LIKE with a leading wildcard. The underlying indexes and
			// table scans keep this acceptable for the intended VPS scale
			// (hundreds-to-thousands of posts); the pattern can be swapped
			// for a dedicated FTS5 table if the driver ever gains FTS5.
			// User input is escaped so a search for literal "%" or "_" is
			// treated as text, not a wildcard (otherwise "%" matches every
			// row and "_" matches any single char).
			like := "%" + escapeLike(q.Search) + "%"
			query = query.Joins("LEFT JOIN users AS author ON author.id = posts.author_id").
				Where("posts.title LIKE ? ESCAPE '\\' OR posts.body LIKE ? ESCAPE '\\' OR author.username LIKE ? ESCAPE '\\'", like, like, like)
		}
		if q.Category > 0 {
			query = query.Where("posts.category_id = ?", q.Category)
		}
		if q.Tag != "" {
			query = query.Joins("JOIN post_tags pt ON pt.post_id = posts.id").
				Joins("JOIN tags tag ON tag.id = pt.tag_id").
				Where("tag.name = ?", q.Tag)
		}
		return query
	}

	var total int64
	if err := applyFilters(s.db).Select("COUNT(*)").Scan(&total).Error; err != nil {
		return PostPage{}, newErr(http.StatusInternalServerError, "failed to count posts")
	}

	offset := (q.Page - 1) * q.PageSize
	// Whitelist the sort column + direction so the raw Order() string cannot
	// be abused for SQL injection via query params.
	orderCol := "posts.created_at"
	if q.SortBy == "views" {
		orderCol = "posts.view_count"
	}
	orderDir := "DESC"
	if q.Order == "asc" {
		orderDir = "ASC"
	}
	orderClause := "posts.pinned DESC, " + orderCol + " " + orderDir
	find := applyFilters(s.db)
	var posts []models.Post
	err := find.Preload("Author").
		Preload("Category").
		Preload("Tags").
		Order(orderClause).
		Offset(offset).
		Limit(q.PageSize).
		Find(&posts).Error
	if err != nil {
		return PostPage{}, newErr(http.StatusInternalServerError, "failed to load posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return PostPage{Items: posts, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}

// UpdatePost edits a post. The author may edit their own post; a plain admin
// may only edit plain users' posts; root may edit anyone's. Returns 403 when
// the caller lacks permission, 404 when the post does not exist.
//
// Status changes are role-gated: root may set published/private; admin and
// user may only set "private" (a restricted, non-public value). Any other
// value is ignored so a tampered client cannot accidentally publish.
func (s *Service) UpdatePost(ctx context.Context, me models.User, id string, in UpdatePostInput) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	// Banned posts are frozen for everyone but the root: once content is
	// pulled into the vault the author must not be able to keep editing it
	// (nor mutate it field-by-field through this endpoint). Root keeps full
	// editorial access so corrections remain possible. (as-built audit Q2)
	if post.Status == models.PostBanned && me.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "banned posts can only be edited by the root")
	}
	// Edit permission: the author, or an admin/root who may manage the target.
	// A plain admin may NOT edit another admin's or root's post. A plain user
	// may only edit their own posts — anyone else (including another plain
	// user) is forbidden.
	if post.AuthorID != me.ID {
		if me.Role == models.RoleRoot {
			// root may edit anyone's content.
		} else if me.Role == models.RoleAdmin {
			var author models.User
			if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
				return models.Post{}, newErr(http.StatusForbidden, "you can only edit your own posts")
			}
			if author.Role != models.RoleUser {
				return models.Post{}, newErr(http.StatusForbidden, "you can only edit your own posts")
			}
		} else {
			// A plain user may not edit anyone else's post.
			return models.Post{}, newErr(http.StatusForbidden, "you can only edit your own posts")
		}
	}

	if in.Title != "" {
		post.Title = in.Title
	}
	if in.Body != "" {
		post.Body = in.Body
	}
	// Status whitelist by role (spec: review + banned vault).
	//
	// The private transition is deliberately gated so the author cannot use
	// "save as private" to pull a post out of the review queue (P1-14). An
	// author may only make their own *published* post private (a normal
	// unpublish); reaching "private" from pending/rejected is ignored so the
	// post stays in its moderation lane until reviewed. Banned posts are
	// root-only and root may set them private freely.
	switch {
	case in.Status == models.PostPrivate:
		isAuthor := post.AuthorID == me.ID
		if isAuthor && me.Role != models.RoleRoot {
			// Author path: only published → private is a legal unpublish.
			if post.Status == models.PostPublished {
				post.Status = in.Status
			}
			// pending / rejected / banned: leave the status untouched so the
			// author can't dodge the queue by saving as private.
		} else {
			// root (or an admin editing a user's post): private is a valid,
			// restricted state.
			post.Status = in.Status
		}
	case in.Status == models.PostPending:
		// An author may resubmit their own rejected post back into the
		// review queue, closing the "rejected → fix → resubmit" loop. A
		// fresh submission awaits a new review, so the previous reviewer's
		// note is cleared. No other caller and no other transition may
		// reach "pending" through this endpoint — only root may publish.
		if post.Status == models.PostRejected && post.AuthorID == me.ID {
			post.Status = in.Status
			post.ReviewNote = ""
		}
	case me.Role == models.RoleRoot && in.Status == models.PostPublished:
		// only root can publish (the review outcome).
		post.Status = in.Status
	}
	// CategoryID is *int64 — nil means "don't change", otherwise set.
	if in.CategoryID != nil {
		post.CategoryID = in.CategoryID
	}
	if in.Pinned != nil {
		post.Pinned = *in.Pinned
	}
	// Cover is an optional field. A non-empty value replaces the stored
	// cover; an empty string is ignored so partial edits (title-only) don't
	// wipe an existing cover.
	if in.Cover != "" {
		post.Cover = in.Cover
	}

	// Persist the post fields and (when supplied) the tag associations in a
	// single transaction. Previously the tag Clear/Append used "_ =", so a
	// failure there left a "saved but tagless" (or partially-tagged) post with
	// a 200 response. Now any association error aborts and rolls back the whole
	// update so the post and its tags stay consistent.
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&post).Error; err != nil {
			return err
		}
		if in.Tags != nil {
			// Drop existing associations, then link the new set. We write the
			// join rows directly instead of using gorm's Association helper
			// (which re-opens a Session and panics inside a Transaction).
			if err := tx.Where("post_id = ?", post.ID).Delete(&models.PostTag{}).Error; err != nil {
				return err
			}
			tags, ferr := s.findOrCreateTagsTx(ctx, tx, in.Tags)
			if ferr != nil {
				return ferr
			}
			if err := linkPostTagsTx(tx, post.ID, tags); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to update post")
	}

	// Re-load with author, category, tags so the response carries full objects.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", post.ID).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  me,
		Action: "update_post",
		Target: "post:" + post.ID,
		Detail: "title=" + post.Title + "; status=" + post.Status,
	})
	return post, nil
}

// DeletePost removes a post. The author may delete their own post; a plain
// admin may delete plain users' posts; root may delete anyone's (including
// pulling a post into the recovery trash). Returns 403 when the caller lacks
// permission, 404 when the post does not exist.
func (s *Service) DeletePost(ctx context.Context, me models.User, id string) error {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return newErr(http.StatusNotFound, "post not found")
	}
	// A banned post may leave the vault only through the root-only vault
	// endpoints (send-back / permanent delete). Soft-deleting one here would
	// make it vanish from every list while its row stays banned forever —
	// and would 404 the root's permanent-delete path, which reads the post
	// through the soft-delete-aware scope. (as-built audit Q2)
	if post.Status == models.PostBanned {
		return newErr(http.StatusForbidden, "banned posts must be managed in the banned vault")
	}
	// Delete permission: the author, or an admin/root who may manage the target.
	// A plain admin may NOT delete another admin's or root's post. A plain user
	// may only delete their own posts.
	if post.AuthorID != me.ID {
		if me.Role == models.RoleRoot {
			// root may delete anyone's content.
		} else if me.Role == models.RoleAdmin {
			var author models.User
			if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
				return newErr(http.StatusForbidden, "you can only delete your own posts")
			}
			if author.Role != models.RoleUser {
				return newErr(http.StatusForbidden, "you can only delete your own posts")
			}
		} else {
			// A plain user may not delete anyone else's post.
			return newErr(http.StatusForbidden, "you can only delete your own posts")
		}
	}
	if err := s.db.WithContext(ctx).Delete(&post).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete post")
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  me,
		Action: "delete_post",
		Target: "post:" + post.ID,
		Detail: "title=" + post.Title,
	})
	return nil
}

// --- review workflow + banned vault (spec: content moderation) ---

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

// ReviewPost approves or rejects a post that is currently in the pending
// queue (or, for reject, a previously rejected post being re-reviewed).
//
// Reviewer rules (spec: "user 被 admin 或 root 审核；admin 被 root 审核"):
//   - A post authored by a plain user may be reviewed by any admin or by root.
//   - A post authored by an admin (or root) may ONLY be reviewed by root.
//
// approve -> status "published", review metadata cleared.
// reject   -> status "rejected", ReviewNote/ReviewedBy/ReviewedAt recorded.
func (s *Service) ReviewPost(ctx context.Context, actor models.User, id string, in ReviewPostInput) (models.Post, error) {
	// Only admins / root can act on the review queue.
	if actor.Role != models.RoleAdmin && actor.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "only an admin or the root can review")
	}
	switch in.Action {
	case "approve", "reject":
		// valid
	default:
		return models.Post{}, newErr(http.StatusBadRequest, "action must be \"approve\" or \"reject\"")
	}

	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}

	// State precondition (as-built audit Q1): review only transitions posts that
	// are actually awaiting review. Acting on a published or private post would
	// suddenly unpublish public content; acting on a banned post would skip the
	// mandatory send-back-to-author step and republish without the author's fixes.
	// Only the review-queue states (pending, rejected) are valid review targets.
	if post.Status != models.PostPending && post.Status != models.PostRejected {
		return models.Post{}, newErr(http.StatusConflict, "post is not awaiting review")
	}

	// Review scope: who may review this post depends on the author's role.
	var author models.User
	if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
		// Author already deleted — only root may still act.
		if actor.Role != models.RoleRoot {
			return models.Post{}, newErr(http.StatusForbidden, "only the root can review this post")
		}
	} else if author.Role == models.RoleAdmin || author.Role == models.RoleRoot {
		// admin-authored (or root-authored) posts require root review.
		if actor.Role != models.RoleRoot {
			return models.Post{}, newErr(http.StatusForbidden, "admin posts can only be reviewed by the root")
		}
	}

	now := time.Now()
	updates := map[string]any{}
	if in.Action == "approve" {
		updates = map[string]any{
			"status":      models.PostPublished,
			"review_note": "",
			"reviewed_by": actor.ID,
			"reviewed_at": now,
		}
	} else { // reject
		// A reject must carry a note so the author knows what to fix.
		if strings.TrimSpace(in.Note) == "" {
			return models.Post{}, newErr(http.StatusBadRequest, "a note is required when rejecting a post")
		}
		updates = map[string]any{
			"status":      models.PostRejected,
			"review_note": in.Note,
			"reviewed_by": actor.ID,
			"reviewed_at": now,
		}
	}
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to review post")
	}

	// Re-load with relations so the response is complete.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	// Best-effort: notify the author of the review outcome (mail may be unset).
	if post.Author != nil {
		s.SendMail(ctx, *post.Author, MailReviewResult, map[string]string{
			"title": post.Title,
			"action": in.Action,
			"note":  in.Note,
			"time":  now.Format("2006-01-02 15:04 UTC"),
		})
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  actor,
		Action: "review_post",
		Target: "post:" + post.ID,
		Detail: "action=" + in.Action + "; title=" + post.Title,
	})
	return post, nil
}

// BanPost hard-moderates a post into the banned vault (root-only).
// Only plain users' and admins' posts may be banned; root's own posts are
// out of scope. The post is retained (soft) with status "banned" and is
// visible to root only.
func (s *Service) BanPost(ctx context.Context, actor models.User, id string) (models.Post, error) {
	if actor.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "only the root can ban a post")
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	// State precondition (as-built audit Q1): a post already in the banned vault
	// cannot be re-banned. Banning is a root take-down for any live/pending/
	// rejected/private post, but re-banning a banned row is a no-op that should
	// fail fast rather than silently rewrite the reviewer timestamps.
	if post.Status == models.PostBanned {
		return models.Post{}, newErr(http.StatusConflict, "post is already in the banned vault")
	}
	// The root cannot ban its own posts.
	if post.AuthorID == actor.ID {
		return models.Post{}, newErr(http.StatusBadRequest, "the root cannot ban its own post")
	}
	now := time.Now()
	updates := map[string]any{
		"status":      models.PostBanned,
		"reviewed_by": actor.ID,
		"reviewed_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to ban post")
	}
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  actor,
		Action: "ban_post",
		Target: "post:" + post.ID,
		Detail: "title=" + post.Title,
	})
	return post, nil
}

// SendBackPost returns a banned post to its author for revision, attaching
// the reviewer's note. This is the "发回原作者修改并备注意见" disposition
// from the banned vault (root-only). The post becomes status "rejected" so
// it re-enters the normal review flow, and the note is preserved so the
// author knows what to fix.
func (s *Service) SendBackPost(ctx context.Context, actor models.User, id, note string) (models.Post, error) {
	if actor.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "only the root can send a post back")
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	if post.Status != models.PostBanned {
		return models.Post{}, newErr(http.StatusBadRequest, "post is not in the banned vault")
	}
	if strings.TrimSpace(note) == "" {
		return models.Post{}, newErr(http.StatusBadRequest, "a note is required when sending a post back")
	}
	now := time.Now()
	updates := map[string]any{
		"status":      models.PostRejected,
		"review_note": note,
		"reviewed_by": actor.ID,
		"reviewed_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to send back post")
	}
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	// Best-effort: notify the author of the send-back and its note.
	if post.Author != nil {
		s.SendMail(ctx, *post.Author, MailReviewResult, map[string]string{
			"title":  post.Title,
			"action": "reject",
			"note":   note,
			"time":   now.Format("2006-01-02 15:04 UTC"),
		})
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  actor,
		Action: "send_back_post",
		Target: "post:" + post.ID,
		Detail: "title=" + post.Title + "; note=" + note,
	})
	return post, nil
}

// ListBanned returns every post currently in the banned vault (status
// "banned"), root-only. Author + relations are preloaded for the vault UI.
func (s *Service) ListBanned(ctx context.Context, actor models.User) ([]models.Post, error) {
	if actor.Role != models.RoleRoot {
		return nil, newErr(http.StatusForbidden, "only the root can view the banned vault")
	}
	var posts []models.Post
	if err := s.db.WithContext(ctx).
		Where("posts.status = ?", models.PostBanned).
		Preload("Author").Preload("Category").Preload("Tags").
		Order("posts.created_at DESC").Find(&posts).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load banned posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// ListPending returns every post in the review queue, scoped to what the
// reviewer may act on. For root, all pending posts; for a plain admin, only
// pending posts authored by plain users.
func (s *Service) ListPending(ctx context.Context, actor models.User) ([]models.Post, error) {
	if actor.Role != models.RoleAdmin && actor.Role != models.RoleRoot {
		return nil, newErr(http.StatusForbidden, "only an admin or the root can list the review queue")
	}
	query := s.db.WithContext(ctx).Model(&models.Post{}).
		Where("posts.status = ?", models.PostPending).
		Preload("Author").Preload("Category").Preload("Tags")
	if actor.Role == models.RoleAdmin {
		// Admins see only pending posts by plain users (their own and other
		// admins' posts require root review and are not in an admin's queue).
		query = query.Joins("JOIN users AS author ON author.id = posts.author_id").
			Where("author.role = ?", models.RoleUser)
	}
	query = query.Order("posts.created_at ASC") // oldest first — FIFO queue.
	var posts []models.Post
	if err := query.Find(&posts).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load pending posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// DeleteBannedPermanently purges a banned post from the vault (hard delete,
// root-only). This is the final "删除" disposition from the banned vault.
func (s *Service) DeleteBannedPermanently(ctx context.Context, actor models.User, id string) error {
	if actor.Role != models.RoleRoot {
		return newErr(http.StatusForbidden, "only the root can permanently delete a banned post")
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return newErr(http.StatusNotFound, "post not found")
	}
	if post.Status != models.PostBanned {
		return newErr(http.StatusBadRequest, "post is not in the banned vault")
	}
	// Remove the tag join rows first so the hard delete leaves no orphan
	// associations (which would otherwise keep inflating tag counts).
	if err := s.db.WithContext(ctx).Where("post_id = ?", id).Delete(&models.PostTag{}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete banned post tags")
	}
	// Hard delete so the post is gone from the vault and from the recovery
	// trash alike (banned content is not recoverable).
	if err := s.db.WithContext(ctx).Unscoped().Delete(&models.Post{}, "id = ?", id).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete banned post")
	}
	s.RecordAction(ctx, ActionInput{
		Actor:  actor,
		Action: "delete_banned",
		Target: "post:" + post.ID,
		Detail: "title=" + post.Title,
	})
	return nil
}

// --- categories / tags ---

// findOrCreateTags resolves a list of tag names to Tag records, creating any
// that do not yet exist. Runs on the shared connection (used outside a
// transaction) and delegates to findOrCreateTagsTx.
func (s *Service) findOrCreateTags(ctx context.Context, names []string) []models.Tag {
	tags, _ := s.findOrCreateTagsTx(ctx, s.db, names)
	return tags
}

// findOrCreateTagsTx is the transaction-aware variant of findOrCreateTags: it
// runs every lookup/create on the supplied *gorm.DB (which may be a
// transaction handle) so tag resolution participates in the caller's
// transaction and rolls back together with the post write. It returns an error
// if a tag cannot be resolved or created, and skips empty names. A creation
// that loses a concurrent unique-race is recovered by re-querying, so we never
// append a zero-value (ID=0) tag.
func (s *Service) findOrCreateTagsTx(ctx context.Context, db *gorm.DB, names []string) ([]models.Tag, error) {
	var out []models.Tag
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		var tag models.Tag
		if db.WithContext(ctx).Where("name = ?", name).First(&tag).Error != nil {
			tag = models.Tag{Name: name}
			if err := db.WithContext(ctx).Create(&tag).Error; err != nil {
				// Likely a concurrent insert of the same name: re-query so we
				// still attach the existing tag instead of dropping it.
				if db.WithContext(ctx).Where("name = ?", name).First(&tag).Error != nil {
					return nil, newErr(http.StatusInternalServerError, "failed to resolve tag")
				}
			}
		}
		out = append(out, tag)
	}
	return out, nil
}

// linkPostTagsTx writes the post←tag associations directly into the post_tags
// join table inside the supplied transaction. We deliberately avoid gorm's
// Association helper here: it re-opens a Session internally and panics when
// called from within a Transaction callback. Writing the join rows by hand
// keeps the whole create/update atomic and lets association errors roll back
// the post write. Duplicate (post_id, tag_id) pairs are de-duplicated via
// FirstOrCreate on the composite primary key.
func linkPostTagsTx(tx *gorm.DB, postID string, tags []models.Tag) error {
	for _, tag := range tags {
		if tag.ID == 0 {
			continue
		}
		pt := models.PostTag{PostID: postID, TagID: tag.ID}
		if err := tx.Where("post_id = ? AND tag_id = ?", postID, tag.ID).FirstOrCreate(&pt).Error; err != nil {
			return err
		}
	}
	return nil
}

// ListCategories returns all categories, ordered by name.
func (s *Service) ListCategories(ctx context.Context) ([]models.Category, error) {
	var cats []models.Category
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&cats).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load categories")
	}
	if cats == nil {
		cats = []models.Category{}
	}
	return cats, nil
}

// ListTags returns all distinct tag names, ordered by name.
func (s *Service) ListTags(ctx context.Context) ([]models.Tag, error) {
	var tags []models.Tag
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&tags).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load tags")
	}
	if tags == nil {
		tags = []models.Tag{}
	}
	return tags, nil
}

// GetRecentPosts returns the 5 most recently created published posts,
// with category and tags preloaded.
func (s *Service) GetRecentPosts(ctx context.Context) ([]models.Post, error) {
	var posts []models.Post
	err := s.db.WithContext(ctx).
		Where("status = ?", "published").
		Preload("Category").
		Preload("Tags").
		Order("created_at DESC").
		Limit(5).
		Find(&posts).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load recent posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// --- category CRUD ---

// CreateCategory creates a new category. Returns 409 on duplicate name.
func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (models.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.Category{}, newErr(http.StatusBadRequest, "category name is required")
	}
	var existing models.Category
	if s.db.WithContext(ctx).Where("name = ?", name).First(&existing).RowsAffected == 1 {
		return models.Category{}, newErr(http.StatusConflict, "category already exists")
	}
	cat := models.Category{Name: name}
	if err := s.db.WithContext(ctx).Create(&cat).Error; err != nil {
		return models.Category{}, newErr(http.StatusInternalServerError, "failed to create category")
	}
	return cat, nil
}

// UpdateCategory renames a category. Returns 404 when not found, 409 on duplicate.
func (s *Service) UpdateCategory(ctx context.Context, id int64, in UpdateCategoryInput) (models.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.Category{}, newErr(http.StatusBadRequest, "category name is required")
	}
	var cat models.Category
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&cat).Error; err != nil {
		return models.Category{}, newErr(http.StatusNotFound, "category not found")
	}
	// Check for duplicate name (excluding self).
	var dup models.Category
	if s.db.WithContext(ctx).Where("name = ? AND id != ?", name, id).First(&dup).RowsAffected == 1 {
		return models.Category{}, newErr(http.StatusConflict, "a category with this name already exists")
	}
	cat.Name = name
	if err := s.db.WithContext(ctx).Save(&cat).Error; err != nil {
		return models.Category{}, newErr(http.StatusInternalServerError, "failed to update category")
	}
	return cat, nil
}

// DeleteCategory removes a category and detaches it from all posts.
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	var cat models.Category
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&cat).Error; err != nil {
		return newErr(http.StatusNotFound, "category not found")
	}
	// Null out category_id on all posts that reference this category.
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("category_id = ?", id).
		Update("category_id", nil).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to detach posts from category")
	}
	if err := s.db.WithContext(ctx).Delete(&cat).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete category")
	}
	return nil
}

// ListCategoriesWithCount returns all categories with post counts.
//
// The count only includes live, publicly-managed posts: soft-deleted posts
// (the trash) are excluded via the join condition, and banned posts (the
// root-only vault) are excluded so the admin counts match what authors can
// actually see. The filter lives in the JOIN ON clause (not WHERE) so that
// categories with zero matching posts still return a row with count 0.
func (s *Service) ListCategoriesWithCount(ctx context.Context) ([]CategoryWithCount, error) {
	var results []CategoryWithCount
	err := s.db.WithContext(ctx).
		Table("categories").
		Select("categories.id, categories.name, COUNT(posts.id) as count").
		Joins("LEFT JOIN posts ON posts.category_id = categories.id AND posts.deleted_at IS NULL AND posts.status <> ?", models.PostBanned).
		Group("categories.id").
		Order("categories.name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load categories with counts")
	}
	if results == nil {
		results = []CategoryWithCount{}
	}
	return results, nil
}

// --- tag management ---

// ListTagsWithCount returns all tags with post counts.
//
// Mirrors ListCategoriesWithCount: the count excludes soft-deleted posts and
// banned posts. The join to posts carries the filter so tag rows whose only
// posts are trashed/banned still return a row with count 0, and we COUNT the
// (nullable) posts.id so banned posts — which yield a NULL posts.id on the
// filtered join — are not tallied via their lingering post_tags rows.
func (s *Service) ListTagsWithCount(ctx context.Context) ([]TagWithCount, error) {
	var results []TagWithCount
	err := s.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, COUNT(posts.id) as count").
		Joins("LEFT JOIN post_tags ON post_tags.tag_id = tags.id").
		Joins("LEFT JOIN posts ON posts.id = post_tags.post_id AND posts.deleted_at IS NULL AND posts.status <> ?", models.PostBanned).
		Group("tags.id").
		Order("tags.name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load tags with counts")
	}
	if results == nil {
		results = []TagWithCount{}
	}
	return results, nil
}

// DeleteTag removes a tag and all its associations.
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	var tag models.Tag
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&tag).Error; err != nil {
		return newErr(http.StatusNotFound, "tag not found")
	}
	// Remove join records.
	if err := s.db.WithContext(ctx).Where("tag_id = ?", id).Delete(&models.PostTag{}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to remove tag associations")
	}
	if err := s.db.WithContext(ctx).Delete(&tag).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete tag")
	}
	return nil
}

// --- post navigation ---

// GetPostPrevNext returns the previous and next published posts (by created_at)
// relative to the given post id.
func (s *Service) GetPostPrevNext(ctx context.Context, id string) (PostNav, error) {
	var current models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&current).Error; err != nil {
		return PostNav{}, newErr(http.StatusNotFound, "post not found")
	}

	var nav PostNav

	// Previous: newer published post (created_at > current, ordered DESC, limit 1).
	var prev models.Post
	err := s.db.WithContext(ctx).
		Where("status = ? AND created_at > ?", "published", current.CreatedAt).
		Order("created_at DESC").
		Limit(1).
		Preload("Author").Preload("Category").Preload("Tags").
		First(&prev).Error
	if err == nil {
		nav.Prev = &prev
	}

	// Next: older published post (created_at < current, ordered ASC, limit 1).
	var next models.Post
	err = s.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", "published", current.CreatedAt).
		Order("created_at ASC").
		Limit(1).
		Preload("Author").Preload("Category").Preload("Tags").
		First(&next).Error
	if err == nil {
		nav.Next = &next
	}

	return nav, nil
}

// --- user management (admin) ---

// ListUsers returns every account with its admin-disabled flag.
func (s *Service) ListUsers(ctx context.Context) ([]UserWithDisabled, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&users).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load users")
	}
	out := make([]UserWithDisabled, 0, len(users))
	for _, u := range users {
		out = append(out, UserWithDisabled{
			ID:           u.ID,
			Username:     u.Username,
			Email:        u.Email,
			Role:         u.Role,
			Disabled:     u.Disabled,
			DisplayName:  u.DisplayName,
			LastLoginIP:  u.LastLoginIP,
			LastLoginAt:  u.LastLoginAt,
			CreatedAt:    u.CreatedAt,
		})
	}
	return out, nil
}

// SetUserDisabled toggles the admin-disabled flag for a user. A user cannot
// disable their own account (would lock themselves out of recovery). The
// unique root account is never disableable. A plain admin may only toggle
// plain users; root may toggle admin and user accounts.
func (s *Service) SetUserDisabled(ctx context.Context, actor models.User, id string, disabled bool) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}
	if id == actor.ID {
		return models.User{}, newErr(http.StatusForbidden, "cannot disable your own account")
	}
	// The unique root account is never disableable.
	if user.Role == models.RoleRoot {
		return models.User{}, newErr(http.StatusForbidden, "the root account cannot be disabled")
	}
	// A plain admin cannot toggle an admin account.
	if actor.Role == models.RoleAdmin && user.Role == models.RoleAdmin {
		return models.User{}, newErr(http.StatusForbidden, "only the root can disable an admin")
	}
	if user.Disabled == disabled {
		return user, nil // no-op
	}
	if err := s.db.WithContext(ctx).Model(&user).Update("disabled", disabled).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update user")
	}
	user.Disabled = disabled
	return user, nil
}

// --- data backup ---

// ExportBackup serialises every table to a timestamped JSON file under the
// configured backup directory and returns the file path + size + row counts.
func (s *Service) ExportBackup(ctx context.Context) (BackupResult, error) {
	db := s.db.WithContext(ctx)

	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read users")
	}
	var categories []models.Category
	if err := db.Find(&categories).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read categories")
	}
	var tags []models.Tag
	if err := db.Find(&tags).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read tags")
	}
	// Backup includes soft-deleted posts so a dump can restore them.
	var posts []models.Post
	if err := db.Unscoped().Find(&posts).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read posts")
	}
	// The users table carries the password hash; models.User marks it json:"-"
	// so it is exported separately here to keep a backup restorable (import
	// re-applies the hashes keyed by user id).
	var pwRows []struct {
		ID           string
		PasswordHash string
	}
	if err := db.Model(&models.User{}).Select("id, password_hash").Scan(&pwRows).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read user hashes")
	}
	pwHashes := make(map[string]string, len(pwRows))
	for _, r := range pwRows {
		pwHashes[r.ID] = r.PasswordHash
	}
	// Also dump the many-to-many join table so tag associations round-trip.
	var postTags []models.PostTag
	if err := db.Find(&postTags).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read post_tags")
	}

	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "cannot create backup directory")
	}

	now := time.Now()
	filename := fmt.Sprintf("backup-%s.json", now.UTC().Format("20060102-150405"))
	full := filepath.Join(dir, filename)

	out, err := os.Create(full)
	if err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "cannot write backup file")
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	payload := map[string]any{
		// Spec #6: top-level metadata so importers can validate schema/app
		// compatibility before applying a destructive restore.
		"schema_version": BackupSchemaVersion,
		"app_version":    BackupAppVersion,
		"exported_at":     now.UTC().Format(time.RFC3339),
		// Security note travels inside the file so a leaked/recovered dump
		// still warns the reader that it embeds bcrypt password hashes.
		"security_note": "This backup contains bcrypt password hashes. Anyone with the file can attempt offline cracking, so store it like a secrets file: restrict access, never commit to version control or share publicly.",
		"users":          users,
		"password_hashes": pwHashes,
		"categories":      categories,
		"tags":            tags,
		"posts":           posts,
		"post_tags":       postTags,
	}
	if err := enc.Encode(payload); err != nil {
		_ = os.Remove(full)
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to write backup")
	}

	info, err := out.Stat()
	if err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read backup size")
	}

	return BackupResult{
		Filename: filename,
		Size:     info.Size(),
		Stats: BackupStats{
			Users:      int64(len(users)),
			Posts:      int64(len(posts)),
			Categories: int64(len(categories)),
			Tags:       int64(len(tags)),
		},
		Warning: "The backup file contains bcrypt password hashes, equivalent to an offline cracking target for the plaintext passwords. Store it like a secret key: restrict access, never commit it to a code repository or share it publicly.",
	}, nil
}

// backupDir returns the configured backup directory.
func (s *Service) backupDir() string {
	if s.cfg != nil && s.cfg.BackupDir != "" {
		return s.cfg.BackupDir
	}
	return "data/backups"
}

// --- backup import (restore) ---

// ImportData is the decoded payload for POST /api/backup/import: the same
// shape ExportBackup produces. All slices are empty-safe (nil ok). The
// schema_version / app_version fields (spec #6) let the importer reject
// dumps from an incompatible app before touching the database.
type ImportData struct {
	SchemaVersion  int              `json:"schema_version"`
	AppVersion     string           `json:"app_version"`
	ExportedAt     string           `json:"exported_at"`
	Users          []models.User    `json:"users"`
	PasswordHashes map[string]string `json:"password_hashes,omitempty"`
	Categories     []models.Category `json:"categories"`
	Tags           []models.Tag       `json:"tags"`
	Posts          []models.Post      `json:"posts"`
	PostTags       []models.PostTag   `json:"post_tags"`
}

// validateImportPayload enforces the spec #6 schema gate before any write.
// It returns a 400 ServiceError describing the first incompatibility found.
// Older dumps without the metadata fields are still accepted (the fields
// simply default to zero/empty and pass), so existing backups keep working.
func validateImportPayload(in ImportData) error {
	if in.SchemaVersion != 0 && in.SchemaVersion != BackupSchemaVersion {
		return newErr(http.StatusBadRequest, fmt.Sprintf(
			"backup schema_version %d is not supported (this app expects %d)", in.SchemaVersion, BackupSchemaVersion))
	}
	if in.AppVersion != "" && in.AppVersion != BackupAppVersion {
		return newErr(http.StatusBadRequest, fmt.Sprintf(
			"backup app_version %q does not match this app (%q); refusing to import", in.AppVersion, BackupAppVersion))
	}
	return nil
}

// ImportBackup restores the database from an exported backup. When dryRun is
// true it only validates + reports the counts without touching the database.
// When applied, it first takes an automatic VACUUM INTO snapshot (spec #6,
// keep 3 most recent), runs the destructive replace inside a transaction, and
// returns the applied stats plus the snapshot file name.
//
// Root-only (as-built audit Q7): the payload is attacker-controllable JSON
// that can carry arbitrary user rows — including role "root" and foreign
// password hashes. Letting a plain admin import one would be a complete
// root-takeover path around the "root is never created via API" invariant,
// so the service refuses anything but the root, mirroring CreateUser's
// defence-in-depth (the handler enforces the same gate).
func (s *Service) ImportBackup(ctx context.Context, actor models.User, in ImportData, dryRun bool) (ImportResult, error) {
	if !IsRoot(actor) {
		return ImportResult{}, newErr(http.StatusForbidden, "only the root can import a backup")
	}
	// Spec #6: validate schema compatibility before doing anything.
	if err := validateImportPayload(in); err != nil {
		return ImportResult{}, err
	}

	stats := BackupStats{
		Users:      int64(len(in.Users)),
		Posts:      int64(len(in.Posts)),
		Categories: int64(len(in.Categories)),
		Tags:       int64(len(in.Tags)),
	}
	if dryRun {
		return ImportResult{DryRun: true, Applied: false, Stats: stats}, nil
	}

	// Auto snapshot before the destructive replace (VACUUM INTO is atomic
	// and file-level, so it is safe to run outside the import transaction).
	snapshotFile, err := s.snapshotBeforeImport(ctx)
	if err != nil {
		return ImportResult{}, newErr(http.StatusInternalServerError, "failed to create pre-import snapshot: "+err.Error())
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear existing rows. Unscoped ensures soft-deleted posts go too.
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.Post{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Category{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.User{}).Error; err != nil {
			return err
		}

		// Users (with re-applied password hashes).
		for i := range in.Users {
			u := in.Users[i]
			if h, ok := in.PasswordHashes[u.ID]; ok {
				u.PasswordHash = h
			}
			if err := tx.Create(&u).Error; err != nil {
				return err
			}
		}
		// Categories.
		for i := range in.Categories {
			if err := tx.Create(&in.Categories[i]).Error; err != nil {
				return err
			}
		}
		// Tags.
		for i := range in.Tags {
			if err := tx.Create(&in.Tags[i]).Error; err != nil {
				return err
			}
		}
		// Posts (Unscoped create so DeletedAt is respected as-is).
		for i := range in.Posts {
			if err := tx.Unscoped().Create(&in.Posts[i]).Error; err != nil {
				return err
			}
		}
		// Join table.
		for i := range in.PostTags {
			if err := tx.Create(&in.PostTags[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ImportResult{}, newErr(http.StatusInternalServerError, "failed to import backup")
	}
	return ImportResult{DryRun: false, Applied: true, Stats: stats, SnapshotFile: snapshotFile}, nil
}

// snapshotBeforeImport runs VACUUM INTO to write a file-level copy of the
// current database into the backup dir, then prunes so only the 3 most
// recent snapshots remain. It returns the new snapshot's base file name.
func (s *Service) snapshotBeforeImport(ctx context.Context) (string, error) {
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// VACUUM INTO writes a full, consistent copy of the live DB file.
	now := time.Now()
	target := filepath.Join(dir, fmt.Sprintf("snapshot-%s.db", now.UTC().Format("20060102-150405")))
	if err := s.db.Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", err
	}
	// Keep only the 3 most recent snapshots.
	s.pruneSnapshots(dir, 3)
	base := filepath.Base(target)
	return base, nil
}

// pruneSnapshots removes oldest snapshot-*.db files beyond keep.
func (s *Service) pruneSnapshots(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var snaps []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "snapshot-") && strings.HasSuffix(e.Name(), ".db") {
			snaps = append(snaps, e.Name())
		}
	}
	if len(snaps) <= keep {
		return
	}
	// File names embed a monotonically increasing UTC timestamp, so a
	// lexicographic sort is chronological. Remove the oldest (len-keep).
	for _, old := range snaps[:len(snaps)-keep] {
		_ = os.Remove(filepath.Join(dir, old))
	}
}

// RestoreSnapshot re-imports a previously created VACUUM INTO snapshot file
// (spec #6: /api/backup/restore-snapshot). The snapshot is a raw .db file, so
// restoring means copying it over the live DB file after a clean close. For
// SQLite this is done by: 1) VACUUM INTO the live db to a temp, 2) the
// snapshot is already a full consistent file so we just point the app at the
// next restart — but to restore IN-PROCESS we instead delete all rows and
// re-insert from the snapshot's JSON? No: a VACUUM INTO snapshot is binary.
//
// The simplest correct approach: the handler reads the snapshot .db, and the
// service performs the destructive replace by opening the snapshot as a
// second SQLite handle and copying table contents row-by-row into the live
// DB inside one transaction. This keeps everything in-process and atomic.
//
// Root-only (as-built audit Q7): restoring replaces the whole database
// including the users table, so it shares ImportBackup's takeover risk and
// is gated to the single root at the service layer.
func (s *Service) RestoreSnapshot(ctx context.Context, actor models.User, filename string) (BackupStats, error) {
	if !IsRoot(actor) {
		return BackupStats{}, newErr(http.StatusForbidden, "only the root can restore a snapshot")
	}
	name := filepath.Base(filename) // reject any path traversal
	if name == "" || name == "." || name == ".." {
		return BackupStats{}, newErr(http.StatusBadRequest, "invalid snapshot name")
	}
	snapPath := filepath.Join(s.backupDir(), name)
	if !strings.HasPrefix(name, "snapshot-") || !strings.HasSuffix(name, ".db") {
		return BackupStats{}, newErr(http.StatusBadRequest, "only snapshot-*.db files can be restored")
	}
	if _, err := os.Stat(snapPath); err != nil {
		if os.IsNotExist(err) {
			return BackupStats{}, newErr(http.StatusNotFound, "snapshot not found")
		}
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read snapshot: "+err.Error())
	}

	// Open the snapshot as a read-only second handle.
	snapDB, err := gorm.Open(sqlite.Open(snapPath+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot open snapshot: "+err.Error())
	}
	defer func() {
		if sql, e := snapDB.DB(); e == nil {
			_ = sql.Close()
		}
	}()

	// Auto snapshot the current state first (so the operator can undo this
	// restore by restoring the older snapshot).
	if _, err := s.snapshotBeforeImport(ctx); err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to snapshot before restore: "+err.Error())
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear live tables in dependency order.
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.Post{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Category{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.User{}).Error; err != nil {
			return err
		}
		// Copy users (+ password hashes) from the snapshot.
		var users []models.User
		if err := snapDB.Find(&users).Error; err != nil {
			return err
		}
		var pwRows []struct {
			ID           string
			PasswordHash string
		}
		if err := snapDB.Model(&models.User{}).Select("id, password_hash").Scan(&pwRows).Error; err != nil {
			return err
		}
		pwMap := make(map[string]string, len(pwRows))
		for _, r := range pwRows {
			pwMap[r.ID] = r.PasswordHash
		}
		for i := range users {
			if h, ok := pwMap[users[i].ID]; ok {
				users[i].PasswordHash = h
			}
			if err := tx.Create(&users[i]).Error; err != nil {
				return err
			}
		}
		var cats []models.Category
		if err := snapDB.Find(&cats).Error; err != nil {
			return err
		}
		for i := range cats {
			if err := tx.Create(&cats[i]).Error; err != nil {
				return err
			}
		}
		var tags []models.Tag
		if err := snapDB.Find(&tags).Error; err != nil {
			return err
		}
		for i := range tags {
			if err := tx.Create(&tags[i]).Error; err != nil {
				return err
			}
		}
		var posts []models.Post
		if err := snapDB.Unscoped().Find(&posts).Error; err != nil {
			return err
		}
		for i := range posts {
			if err := tx.Unscoped().Create(&posts[i]).Error; err != nil {
				return err
			}
		}
		var pts []models.PostTag
		if err := snapDB.Find(&pts).Error; err != nil {
			return err
		}
		for i := range pts {
			if err := tx.Create(&pts[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to restore snapshot")
	}
	// Return the post-restore counts.
	var users, posts, cats, tags int64
	// Best-effort: these counts feed the response summary. If any single
	// Count fails we surface the error rather than silently returning zeros
	// (which would look like an empty database and mask a real problem).
	if err := s.db.Model(&models.User{}).Count(&users).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count users after restore")
	}
	if err := s.db.Model(&models.Post{}).Count(&posts).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count posts after restore")
	}
	if err := s.db.Model(&models.Category{}).Count(&cats).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count categories after restore")
	}
	if err := s.db.Model(&models.Tag{}).Count(&tags).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count tags after restore")
	}
	return BackupStats{Users: users, Posts: posts, Categories: cats, Tags: tags}, nil
}

// SnapshotInfo describes one VACUUM INTO snapshot file on disk.
type SnapshotInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// ListSnapshots returns the available snapshot-*.db files, newest first.
// Root-only: a snapshot is a whole-database artifact (it contains every
// account and password hash), so listing it is gated exactly like restoring
// it (as-built audit Q7). The frontend restore UI (P2-14) needs this to
// offer anything to restore.
func (s *Service) ListSnapshots(ctx context.Context, actor models.User) ([]SnapshotInfo, error) {
	if !IsRoot(actor) {
		return nil, newErr(http.StatusForbidden, "only the root can list snapshots")
	}
	entries, err := os.ReadDir(s.backupDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotInfo{}, nil // no backup dir yet -> no snapshots
		}
		return nil, newErr(http.StatusInternalServerError, "failed to read backup directory")
	}
	out := []SnapshotInfo{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "snapshot-") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // transient stat failure: skip the file, keep the list usable
		}
		out = append(out, SnapshotInfo{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	// Newest first. File names embed a UTC timestamp, but sorting by mtime is
	// authoritative regardless of naming.
	slices.SortFunc(out, func(a, b SnapshotInfo) int {
		return b.ModTime.Compare(a.ModTime)
	})
	return out, nil
}

// --- trash (soft-deleted post recovery, 7-day window) ---

// trashWindow is how long a deleted post stays recoverable.
const trashWindow = 7 * 24 * time.Hour

// ListTrash returns soft-deleted posts whose deletion is within the recovery
// window, most recent first. Authors are preloaded so the UI can show who
// wrote each recovered post.
func (s *Service) ListTrash(ctx context.Context) ([]TrashPost, error) {
	cutoff := time.Now().Add(-trashWindow)
	var rows []models.Post
	err := s.db.WithContext(ctx).
		Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at >= ?", cutoff).
		Preload("Author").
		Order("deleted_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load trash")
	}
	out := make([]TrashPost, 0, len(rows))
	for _, p := range rows {
		var deletedAt time.Time
		if p.DeletedAt.Valid {
			deletedAt = p.DeletedAt.Time
		}
		out = append(out, TrashPost{
			ID: p.ID, Title: p.Title, AuthorID: p.AuthorID,
			DeletedAt: deletedAt, Status: p.Status, Pinned: p.Pinned, ViewCount: p.ViewCount,
		})
	}
	return out, nil
}

// RestoreTrash resurrects a soft-deleted post. Only posts inside the 7-day
// window are restorable; older rows return 410 Gone.
func (s *Service) RestoreTrash(ctx context.Context, id string) error {
	var post models.Post
	err := s.db.WithContext(ctx).Unscoped().Where("id = ?", id).First(&post).Error
	if err != nil {
		return newErr(http.StatusNotFound, "post not found")
	}
	if !post.DeletedAt.Valid {
		return newErr(http.StatusBadRequest, "post is not in trash")
	}
	if time.Since(post.DeletedAt.Time) > trashWindow {
		return newErr(http.StatusGone, "this post is older than 7 days and can no longer be restored")
	}
	if err := s.db.WithContext(ctx).Unscoped().Model(&post).Update("deleted_at", nil).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to restore post")
	}
	return nil
}

// EmptyTrash permanently deletes every soft-deleted post (regardless of age),
// releasing their SQLite rows. Returns how many posts were removed.
func (s *Service) EmptyTrash(ctx context.Context) (int, error) {
	res := s.db.WithContext(ctx).Unscoped().
		Where("deleted_at IS NOT NULL").
		Delete(&models.Post{})
	if res.Error != nil {
		return 0, newErr(http.StatusInternalServerError, "failed to empty trash")
	}
	return int(res.RowsAffected), nil
}

// --- image upload ---

// UploadImage stores a client-supplied image file on local disk under cfg's
// upload directory and returns the public URL to it. The file name is
// randomized (time + 8 random hex chars + original extension) so two uploads
// of the same name never collide. Only a small whitelist of image extensions
// is accepted; oversized files are rejected.
//
// The returned URL is relative ("/uploads/<file>"); the reverse proxy or
// static file handler is responsible for serving that path. In dev, the Go
// server serves it directly (see router).
func (s *Service) UploadImage(ctx context.Context, r *http.Request) (string, error) {
	_ = ctx
	// Parse multipart; cap the form size so a hostile client can't ram memory
	// with a giant body. Memory caps are enforced at the handler via
	// r.Body = http.MaxBytesReader before this is called.
	const maxFormBytes = 1 << 20 // 1 MB
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		return "", newErr(http.StatusBadRequest, "invalid multipart form")
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return "", newErr(http.StatusBadRequest, "missing 'file' field")
	}
	defer file.Close()

	// Enforce per-file size (header.Size is declared; read to verify).
	if header.Size > s.uploadMaxBytes() {
		return "", newErr(http.StatusRequestEntityTooLarge, "file too large")
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !isAllowedImageExt(ext) {
		return "", newErr(http.StatusBadRequest, "only image files are allowed (jpg, png, webp, gif)")
	}

	// Read the whole file into memory (capped) so we can either proxy it to a
	// remote image host or save it locally from the same bytes.
	data, err := io.ReadAll(io.LimitReader(file, s.uploadMaxBytes()))
	if err != nil {
		return "", newErr(http.StatusInternalServerError, "failed to read file")
	}

	// Route by site config: the server holds the image-host token, so the
	// browser never sees it (replaces the old localStorage token, spec #7).
	// Token resolution order:
	//  1. cfg.SmmsToken (from .env SM_MS_TOKEN) — the primary, server-held
	//     secret. The browser never learns it.
	//  2. site-config ImgHostToken (DB row) — legacy fallback so existing
	//     setups that stored the token in site settings keep working.
	//
	// When SM.MS mode is selected but NO token is available anywhere, the
	// endpoint returns 503 Service Unavailable so the client knows the host
	// is misconfigured rather than silently landing in local storage.
	siteCfg, _ := s.GetSiteConfig(ctx, false)
	mode := siteCfg.ImgHostMode
	if s.cfg != nil && strings.TrimSpace(s.cfg.SmmsToken) != "" {
		mode = "smms"
	}
	smmsToken := strings.TrimSpace(siteCfg.ImgHostToken)
	if s.cfg != nil && strings.TrimSpace(s.cfg.SmmsToken) != "" {
		smmsToken = strings.TrimSpace(s.cfg.SmmsToken)
	}

	if mode == "smms" {
		if smmsToken == "" {
			return "", newErr(http.StatusServiceUnavailable, "SM.MS is enabled but no token is configured (set SM_MS_TOKEN or the site-config token)")
		}
		if url, e := s.uploadToSmms(ctx, header.Filename, header.Header.Get("Content-Type"), data, smmsToken); e == nil {
			return url, nil
		}
	}

	return s.saveLocalImage(ext, data)
}

// saveLocalImage writes image bytes to the configured upload directory and
// returns the public /uploads/<name> URL.
func (s *Service) saveLocalImage(ext string, data []byte) (string, error) {
	dir := s.uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", newErr(http.StatusInternalServerError, "cannot create upload directory")
	}
	name := randomName(ext)
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", newErr(http.StatusInternalServerError, "cannot write file")
	}
	return "/uploads/" + name, nil
}

// smmsClient is a dedicated HTTP client for the SM.MS image proxy. It carries
// a hard timeout so a slow or hung remote cannot pin a goroutine — and the
// enclosing upload handler — indefinitely on a low-core VPS. We deliberately
// avoid http.DefaultClient, which has no timeout (as-built audit Q11).
var smmsClient = &http.Client{Timeout: 10 * time.Second}

// uploadToSmms proxies an image to the SM.MS v2 upload API. The server holds
// the token (set in site config), so this is a privileged, browser-free call.
func (s *Service) uploadToSmms(ctx context.Context, filename, contentType string, data []byte, token string) (string, error) {
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("smfile", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	_ = w.Close()

	// Tie the call to the request context (cancels on client disconnect) and
	// let smmsClient's 10s timeout act as a backstop against a hung remote.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://sm.ms/api/v2/upload", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := smmsClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Success bool `json:"success"`
		Data    struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || !out.Success || out.Data.URL == "" {
		return "", fmt.Errorf("smms upload failed")
	}
	return out.Data.URL, nil
}

// uploadDir returns the configured upload directory (always absolute-safe
// relative to the process working directory).
func (s *Service) uploadDir() string {
	if s.cfg != nil && s.cfg.UploadDir != "" {
		return s.cfg.UploadDir
	}
	return "data/uploads"
}

func (s *Service) uploadMaxBytes() int64 {
	if s.cfg != nil && s.cfg.UploadMaxBytes > 0 {
		return s.cfg.UploadMaxBytes
	}
	return 2 * 1024 * 1024 // 2 MB default
}

func isAllowedImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}

// randomName returns a collision-resistant file name: <unix-ms>-<8hex><ext>.
func randomName(ext string) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), hex.EncodeToString(buf), ext)
}

// UploadImageInfo describes one locally-stored upload in the image library.
type UploadImageInfo struct {
	Name    string    `json:"name"`    // file name (relative to the upload dir)
	URL     string    `json:"url"`     // public /uploads/<name> URL
	Size    int64     `json:"size"`    // bytes
	ModTime time.Time `json:"mod_time"` // last modified
}

// ListLocalUploads lists the files currently in the local upload directory,
// sorted newest-first. Remote (SM.MS) images are not managed here — deleting
// them would leave broken links, so the library only governs files this
// instance wrote to disk.
func (s *Service) ListLocalUploads(ctx context.Context) ([]UploadImageInfo, error) {
	_ = ctx
	dir := s.uploadDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing upload dir just means "nothing has been uploaded yet".
		if os.IsNotExist(err) {
			return []UploadImageInfo{}, nil
		}
		return nil, newErr(http.StatusInternalServerError, "cannot read upload directory")
	}

	var out []UploadImageInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isAllowedImageExt(strings.ToLower(filepath.Ext(name))) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, UploadImageInfo{
			Name:    name,
			URL:     "/uploads/" + name,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	if out == nil {
		out = []UploadImageInfo{}
	}
	// Newest first. Use the standard library sort instead of a hand-rolled
	// O(n²) bubble pass (P2 code-debt: replace hand-written standard-library
	// logic).
	slices.SortFunc(out, func(a, b UploadImageInfo) int {
		switch {
		case a.ModTime.Equal(b.ModTime):
			return 0
		case a.ModTime.After(b.ModTime):
			return -1
		default:
			return 1
		}
	})
	return out, nil
}

// DeleteLocalUpload removes a locally-stored image by its file name. The name
// is validated strictly against the upload directory (no path traversal), so
// it can never escape into the surrounding filesystem.
func (s *Service) DeleteLocalUpload(ctx context.Context, name string) error {
	_ = ctx
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return newErr(http.StatusBadRequest, "invalid file name")
	}
	// Defence in depth (aligned with ServeBackup): resolve the absolute upload
	// directory and confirm the joined path did not escape it. Without this,
	// a non-absolute uploadDir or an unusual name could let os.Remove reach
	// outside the upload folder.
	absDir, err := filepath.Abs(s.uploadDir())
	if err != nil {
		return newErr(http.StatusInternalServerError, "cannot resolve upload directory")
	}
	full := filepath.Join(absDir, name)
	if rel, err := filepath.Rel(absDir, full); err != nil || strings.HasPrefix(rel, "..") {
		return newErr(http.StatusBadRequest, "invalid file name")
	}
	if err := os.Remove(full); err != nil {
		if os.IsNotExist(err) {
			return newErr(http.StatusNotFound, "file not found")
		}
		return newErr(http.StatusInternalServerError, "failed to delete file")
	}
	return nil
}

// ServeBackup streams a previously generated backup file to the client.
// `name` is the bare file name returned by ExportBackup. It is validated the
// same way as uploads: no separators, no "..", and the resolved path must stay
// inside the backup directory. Returns 404 for anything suspicious so probing
// cannot distinguish "bad name" from "missing file".
func (s *Service) ServeBackup(w http.ResponseWriter, r *http.Request, name string) error {
	// Reject traversal outright: bare name required.
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return newErr(http.StatusNotFound, "backup not found")
	}
	if filepath.Base(name) != name {
		return newErr(http.StatusNotFound, "backup not found")
	}

	dir, err := filepath.Abs(s.backupDir())
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to resolve backup directory")
	}
	full := filepath.Join(dir, name)
	// Defence in depth: confirm the join did not escape the directory.
	if rel, err := filepath.Rel(dir, full); err != nil || strings.HasPrefix(rel, "..") {
		return newErr(http.StatusNotFound, "backup not found")
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return newErr(http.StatusNotFound, "backup not found")
	}

	f, err := os.Open(full)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to read backup")
	}
	defer f.Close()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
	return nil
}

// ServeUploads is a small static handler for the /uploads/<file> paths.
// It resolves the file strictly inside cfg.UploadDir to prevent path
// traversal (a request for /uploads/../etc/passwd is rejected).
func (s *Service) ServeUploads(w http.ResponseWriter, r *http.Request) {
	// Path looks like /uploads/<name>; the router strips nothing, so the
	// last path segment is the file name.
	name := r.URL.Path
	// Strip leading "/uploads/"
	name = strings.TrimPrefix(name, "/uploads/")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(s.uploadDir(), name)
	http.ServeFile(w, r, full)
}
