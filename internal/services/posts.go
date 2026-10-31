package services

import (
	"context"
	"net/http"
	"strings"

	"lopezpress/internal/models"
)

// --- posts ---

// CreatePost creates a post owned by user. AuthorID is set from user.ID.
// The returned post has Author preloaded so the client sees the author.
//
// Initial status: a request to publish goes through review, a request to stay
// private does not.
//
//   - root: published or private exactly as requested.
//   - admin / user asking for private: stored as private — really private.
//   - admin / user asking for anything else (including nothing): pending, so
//     a tampered request cannot bypass the review queue.
//
// The middle case used to be folded into "pending", which made the editor lie.
// It offered a "只有你自己能看到，其他人访问会显示 404" radio, sent
// status=private, and the post landed in the review queue; approving it then
// flipped it to published. The author had chosen privacy and got publicity,
// which is worse than having no such option at all.
//
// Review exists to stop harmful content from being *published*. A post that is
// private reaches no reader, so there is nothing to moderate — the same reason
// canSeePost keeps private/draft out of the review queue's reach. Skipping
// review costs nothing; going public from private still queues for review (see
// UpdatePost). Note that "no reader" does not mean "no operator": admin and
// root can read a private post, because handling a report about it requires
// reading it.
func (s *Service) CreatePost(ctx context.Context, user models.User, in CreatePostInput) (models.Post, error) {
	if err := ValidatePostContent(in.Title, in.Body); err != nil {
		return models.Post{}, err
	}
	// Resolve the initial status by role.
	var status string
	switch {
	case user.Role == models.RoleRoot:
		// Root publishes or keeps private as requested.
		switch in.Status {
		case models.PostPrivate:
			status = models.PostPrivate
		default:
			status = models.PostPublished
		}
	case in.Status == models.PostPrivate:
		// Anyone may write a private post; it never becomes public on its own.
		status = models.PostPrivate
	default:
		// admin / user / unknown asking for public: enter the review queue.
		status = models.PostPending
	}

	// The cover is a URL the reader's browser will fetch. Validate it before it
	// reaches the row: a stored `javascript:` or a 1 MB data: URI is a value
	// every downstream reader (RSS, sitemap, any future client) would have to
	// defend against separately.
	if err := ValidateCoverURL(in.Cover); err != nil {
		return models.Post{}, err
	}

	post := models.Post{
		Title:      in.Title,
		Body:       in.Body,
		Status:     status,
		AuthorID:   user.ID,
		CategoryID: in.CategoryID,
		Cover:      in.Cover,
	}
	// Allocate the permanent link before the insert so a single
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
		// Pinning floats a post to the top of every public list for every
		// reader, so it is a site-wide editorial decision, not an author
		// choice: a plain member could otherwise buy the front-page slot by
		// ticking a checkbox before submitting for review.
		if *in.Pinned && !IsAdminOrRoot(user) {
			return models.Post{}, newErr(http.StatusForbidden, "only an admin can pin a post")
		}
		post.Pinned = *in.Pinned
	}
	// Create the post and its tag associations atomically: if the tag
	// association step fails, the whole post (incl. its row) is rolled back so
	// we never end up with a "saved but tagless" orphan. Association errors are
	// propagated now instead of being silently swallowed.
	//
	// R6-02: the slug was allocated by COUNT above, which is a probe and not a
	// lock — two concurrent creates of the same title both see "free" and both
	// insert. Migration 0014 puts a UNIQUE index on posts.slug, so the loser's
	// INSERT now fails on the constraint instead of silently producing a second
	// post with the same permanent link (the reported run: 20 concurrent creates,
	// four of them sharing "r6slug-col-2026-7"). Retry with the next suffix so
	// the caller gets "title-2" — exactly what the probe would have returned had
	// it seen the winner — rather than a 500.
	created, err := s.insertPostWithSlugRetry(ctx, post, in, slug)
	if err != nil {
		return models.Post{}, err
	}

	// Preload author and category so the returned post carries the full objects.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", created.ID).First(&created).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	s.audit(ctx, user, "create_post", "post:"+created.ID, auditDetail("title="+created.Title, "status="+created.Status))
	// The author is the caller themselves; still strip the account-internal
	// fields so no post payload ever carries an email / login-IP.
	created.SanitizeAuthor()
	return created, nil
}

// GetPost returns a single post by id with the author preloaded.
// Returns 404 when the post does not exist or the caller may not see it.
//
// It does NOT touch view_count: counting moved to the explicit
// POST /api/posts/{id}/view endpoint so the read path stays side-effect free
// and refreshes/previews no longer inflate the number.
//
// Visibility by status (spec: review + banned vault):
//   - published: everyone (incl. anonymous).
//   - private / draft: the author, plus admin and root. The content never
//     reaches a reader, so it skips review — but the operators can open it,
//     because handling a report about an article requires reading it.
//   - pending / rejected: author, admin, or root (the review flow needs it).
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
	post.SanitizeAuthor()
	post.SanitizeReview()
	return post, nil
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
// Browsable anonymously: the archive only ever lists posts whose
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
	models.SanitizePostAuthors(posts)
	models.SanitizePostReviews(posts)
	return posts, nil
}

// ListArchivePage is the paginated form of ListArchive.
//
// The unpaginated version loads every published post on every visit, which is
// fine for 12 posts and terrible for 2000 on a 1 vCPU box. This returns one
// page at a time (default 50, cap 200) and still strips the body so the
// payload stays lean.
func (s *Service) ListArchivePage(ctx context.Context, page, size int) (PostPage, error) {
	// Clamped, not merely floored: an unclamped page overflows the offset and
	// silently serves page one (see ClampPageNumber).
	page = ClampPageNumber(page)
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
	models.SanitizePostAuthors(posts)
	models.SanitizePostReviews(posts)
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
	models.SanitizePostAuthors(posts)
	models.SanitizePostReviews(posts)
	return posts, nil
}

// GetPostPublic returns a published post without requiring authentication.
// Only posts with status "published" are accessible.
//
// It does NOT touch view_count — see ReportView / GetPost.
func (s *Service) GetPostPublic(ctx context.Context, id string) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").
		Where("id = ? AND status = ?", id, "published").First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	post.SanitizeAuthor()
	post.SanitizeReview()
	return post, nil
}

// escapeLike escapes SQL LIKE wildcards (% and _) and the escape character
// itself (\) so untrusted search input is treated as literal text. Pair it
// with a "LIKE ? ESCAPE '\'" clause.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
