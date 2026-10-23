package services

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"lopezpress/internal/models"
)

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
	s.audit(ctx, models.User{}, "restore_post", "post:"+id, "title="+post.Title)
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
	s.audit(ctx, models.User{}, "empty_trash", "trash", "deleted="+strconv.Itoa(int(res.RowsAffected)))
	return int(res.RowsAffected), nil
}
