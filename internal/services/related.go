package services

import (
	"context"
	"net/http"

	"lopezpress/internal/models"
)

// RelatedPosts returns up to limit published posts that share this post's
// category or any of its tags (spec 35).
//
// Design notes:
//   - One query, no in-Go ranking. The ORDER BY is "popular first, then
//     newest", which is the honest ranking available without a read-history
//     model: a post that more people read is more likely to be what this
//     reader wants next.
//   - Only published posts are eligible, so the endpoint is safe for anonymous
//     callers and never leaks a pending/banned draft as "related".
//   - Returns an empty slice, not an error, when the post has no category and
//     no tags. "Nothing related" is a normal state, and the reader page simply
//     hides the block.
func (s *Service) RelatedPosts(ctx context.Context, id string, limit int) ([]models.Post, error) {
	if limit <= 0 || limit > 20 {
		limit = 5
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return nil, newErr(http.StatusNotFound, "post not found")
	}

	// database/sql has no driver.Value for *int64: unwrap it to a plain
	// interface (nil when the post is uncategorised) or the query would fail
	// with "unsupported type".
	var cat any
	if post.CategoryID != nil {
		cat = *post.CategoryID
	}

	var out []models.Post
	err := s.db.WithContext(ctx).Raw(`
		SELECT p.id, p.title, p.slug, p.cover, p.created_at, p.view_count
		FROM posts p
		WHERE p.status = 'published'
		  AND p.deleted_at IS NULL
		  AND p.id <> ?
		  AND (
		    (? IS NOT NULL AND p.category_id = ?)
		    OR EXISTS (
		      SELECT 1 FROM post_tags pt
		      WHERE pt.post_id = p.id
		        AND pt.tag_id IN (SELECT tag_id FROM post_tags WHERE post_id = ?)
		    )
		  )
		ORDER BY p.view_count DESC, p.created_at DESC
		LIMIT ?`, post.ID, cat, cat, post.ID, limit).Scan(&out).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load related posts")
	}
	if out == nil {
		out = []models.Post{}
	}
	return out, nil
}
