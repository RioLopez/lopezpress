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

// maxPageNumber caps the page parameter. Its only job is to keep
// (page-1)*pageSize inside int64: past this point the multiply overflows, the
// offset goes negative, and SQLite reads a negative OFFSET as 0 — which
// silently serves page 1 for an absurd page number (R6-06).
//
// 100,000 pages at 10 posts each is a million articles, which is years past
// anything this deployment will hold; a legitimate caller never reaches it.
const maxPageNumber = 100000

// ClampPageNumber keeps a page number inside [1, maxPageNumber].
//
// Every list endpoint needs this, and the reason is not cosmetic: the offset is
// computed as (page-1)*size, so an unclamped page of MaxInt64 overflows int64,
// goes negative, and SQLite reads a negative OFFSET as 0 -- the request silently
// returns PAGE ONE with a 200. A caller that walks pages until it sees an empty
// result therefore loops forever, and a caller checking "am I at the end" gets
// the wrong answer. R6-06 fixed /api/posts; /archive/paged and /api/uploads kept
// the bug because each had its own copy of the arithmetic.
func ClampPageNumber(page int) int {
	if page < 1 {
		return 1
	}
	if page > maxPageNumber {
		return maxPageNumber
	}
	return page
}

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
		// Author-only states: nobody but the author sees these, whatever the
		// caller's role (see canSeePost). Kept in one slice so the list filter
		// and the single-post guard cannot drift apart.
		authorOnly := []string{models.PostPrivate, models.PostDraft}
		switch {
		case user.Role == models.RoleRoot:
			// root: the moderation view — every post including the banned
			// vault, EXCEPT other people's author-only posts.
			query = query.Where("posts.status NOT IN ? OR posts.author_id = ?", authorOnly, user.ID)
		case user.Role == models.RoleAdmin:
			// admin: everything except the banned vault (root-only) and other
			// people's author-only posts.
			query = query.Where(
				"(posts.status NOT IN ? AND posts.status <> ?) OR posts.author_id = ?",
				authorOnly, models.PostBanned, user.ID)
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
		if q.AuthorID != "" {
			// Scope to a single author (personal center). Applied on top of the
			// role visibility above, so a user scoping to their own id still
			// only sees what their role permits (their own restricted posts,
			// never the banned vault).
			query = query.Where("posts.author_id = ?", q.AuthorID)
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
		if q.Category != "" {
			// Spec 27: selecting a parent also returns its children's posts,
			// otherwise a parent node looks empty while its content sits one
			// level down.
			//
			// The reference may be a name (?category=PHP) or a legacy numeric
			// id (?category=2). An unknown reference gets an empty page: the
			// URL claims a filter is on, so silently listing every post would
			// be a lie the reader cannot see through.
			ids, ok := s.categoryFilterIDs(ctx, q.Category)
			if !ok {
				query = query.Where("1 = 0")
			} else {
				query = query.Where("posts.category_id IN ?", ids)
			}
		}
		if q.Tag != "" {
			// Case-insensitive so ?tag=go finds the "Go" tag: the tag list is
			// de-duplicated without regard to case, so the filter must match
			// the same way or a reader would see an empty page for a tag that
			// visibly exists.
			query = query.Joins("JOIN post_tags pt ON pt.post_id = posts.id").
				Joins("JOIN tags tag ON tag.id = pt.tag_id").
				Where("tag.name COLLATE NOCASE = ?", q.Tag)
		}
		return query
	}

	var total int64
	if err := applyFilters(s.db).Select("COUNT(*)").Scan(&total).Error; err != nil {
		return PostPage{}, newErr(http.StatusInternalServerError, "failed to count posts")
	}

	// R6-06: (page-1)*pageSize overflows int64 for a large enough page, goes
	// negative, and SQLite treats a negative OFFSET as 0 — so
	// ?page=922337203685477580 answered with the FIRST page's data while
	// echoing the impossible page number back. A caller cannot distinguish
	// "page 1" from "page 9 × 10^17", and a cache keyed on the query string
	// stores two different answers for what looks like one page.
	//
	// Clamping before the multiply is the fix: the ceiling is far above any
	// real page count (a site would need 100,000 posts at 10 per page to reach
	// it) and it keeps the arithmetic inside int64 by construction.
	if q.Page > maxPageNumber {
		return PostPage{}, newErr(http.StatusBadRequest,
			"page number is too large (max "+itoa(maxPageNumber)+")")
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
	// Strip the author's account-internal fields (email, login IP/time,
	// account flags) before the page leaves the service. This is the public
	// list an anonymous visitor hits, so it must never carry them.
	models.SanitizePostAuthors(posts)
	models.SanitizePostReviews(posts)
	return PostPage{Items: posts, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
}
