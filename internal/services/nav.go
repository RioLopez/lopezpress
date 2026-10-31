package services

import (
	"context"
	"net/http"

	"lopezpress/internal/models"
)

// --- post navigation ---

// GetPostPrevNext returns the previous and next published posts (by created_at)
// relative to the given post id.
//
// The anchor post is checked against canSeePost first: this endpoint takes an
// id, and an endpoint that answers "200 with a neighbour" for a post you cannot
// read while answering 404 for an id that does not exist is an existence
// oracle. The neighbours themselves are always published, so they need no
// further filtering.
func (s *Service) GetPostPrevNext(ctx context.Context, me models.User, id string) (PostNav, error) {
	var current models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&current).Error; err != nil {
		return PostNav{}, newErr(http.StatusNotFound, "post not found")
	}
	if !canSeePost(me, current) {
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
		prev.SanitizeAuthor()
		prev.SanitizeReview()
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
		next.SanitizeAuthor()
		next.SanitizeReview()
		nav.Next = &next
	}

	return nav, nil
}
