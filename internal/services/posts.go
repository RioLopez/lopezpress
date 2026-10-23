package services

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

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
	// Spec 29: allocate the permanent link before the insert so a single
	// transaction covers both. The id is generated here (not by BeforeCreate)
	// because the slug falls back to it when the title yields nothing usable.
	if post.ID == "" {
		post.ID = newID()
	}
	slug, slugErr := s.slugFor(ctx, in.Slug, in.Title, post.ID)
	if slugErr != nil {
		return models.Post{}, slugErr
	}
	post.Slug = slug
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
	s.audit(ctx, user, "create_post", "post:"+created.ID, "title="+created.Title+"; status="+created.Status)
	return created, nil
}

// GetPost returns a single post by id with the author preloaded.
// Returns 404 when the post does not exist or the caller may not see it.
//
// It does NOT touch view_count (spec 23): counting moved to the explicit
// POST /api/posts/{id}/view endpoint so the read path stays side-effect free
// and refreshes/previews no longer inflate the number.
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

// ListArchivePage is the paginated form of ListArchive (spec 28).
//
// The unpaginated version loads every published post on every visit, which is
// fine for 12 posts and terrible for 2000 on a 1 vCPU box. This returns one
// page at a time (default 50, cap 200) and still strips the body so the
// payload stays lean.
func (s *Service) ListArchivePage(ctx context.Context, page, size int) (PostPage, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}

	var total int64
	if err := s.db.WithContext(ctx).Model(&models.Post{}).
		Where("status = ?", models.PostPublished).Count(&total).Error; err != nil {
		return PostPage{}, newErr(http.StatusInternalServerError, "failed to count archive")
	}

	var posts []models.Post
	if err := s.db.WithContext(ctx).Model(&models.Post{}).
		Where("status = ?", models.PostPublished).
		Preload("Author").Preload("Category").
		Order("created_at DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&posts).Error; err != nil {
		return PostPage{}, newErr(http.StatusInternalServerError, "failed to load archive")
	}
	for i := range posts {
		posts[i].Body = ""
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return PostPage{Items: posts, Total: total, Page: page, PageSize: size}, nil
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
// Only posts with status "published" are accessible.
//
// It does NOT touch view_count (spec 23) — see ReportView / GetPost.
func (s *Service) GetPostPublic(ctx context.Context, id string) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").
		Where("id = ? AND status = ?", id, "published").First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	return post, nil
}

// escapeLike escapes SQL LIKE wildcards (% and _) and the escape character
// itself (\) so untrusted search input is treated as literal text. Pair it
// with a "LIKE ? ESCAPE '\'" clause.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
