package services

import (
	"context"
	"net/http"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// allowedPageSizes is the closed set an admin may choose from (spec 24).
var allowedPageSizes = []int{5, 10, 20, 30}

// DefaultPageSize is used when the site config is missing/illegal.
const DefaultPageSize = 10

// ListPageSize returns the configured number of posts per public list page.
//
// The value is clamped to allowedPageSizes so a hand-edited database (or a
// tampered request body) can never produce a 10 000-row page on a 1 GB VPS.
func (s *Service) ListPageSize(ctx context.Context) int {
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return DefaultPageSize
	}
	return ClampPageSize(cfg.PageSize)
}

// ClampPageSize keeps n inside allowedPageSizes, falling back to the default
// when n is not one of the offered values.
func ClampPageSize(n int) int {
	for _, v := range allowedPageSizes {
		if n == v {
			return v
		}
	}
	return DefaultPageSize
}

// ListPostsPage returns a paginated slice of posts, supporting status +
// keyword search + page/size + category + tag filters.
// Pinned posts appear first (within the same status), ordered by created_at DESC.
func (s *Service) ListPostsPage(ctx context.Context, user models.User, q ListPostsQuery) (PostPage, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		// Spec 24: the server — not the client — decides the page size, so
		// that ?page=N means the same thing for every visitor and stays
		// shareable. Explicit sizes (admin tables, personal center) still win.
		q.PageSize = s.ListPageSize(ctx)
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
			// Spec 27: selecting a parent also returns its children's posts,
			// otherwise a parent node looks empty while its content sits one
			// level down.
			query = query.Where("posts.category_id IN ?", s.categoryWithDescendants(ctx, q.Category))
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
