package services

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

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
//
// R4-11: "is it in the trash" used to be checked on the row we read and then
// not enforced on the write, so a restore racing "empty trash" answered
// "restored" to the operator while the row was permanently destroyed a moment
// later — and the post was gone from both the trash and the site. The
// conditions are now part of the UPDATE, and `RowsAffected == 0` means the row
// left the trash under us.
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
	// Unscoped + deleted_at IS NOT NULL: only a row still IN the trash can be
	// taken out of it. The timestamp is echoed back so a concurrent second
	// restore also matches nothing.
	res := s.db.WithContext(ctx).Unscoped().Model(&models.Post{}).
		Where("id = ? AND deleted_at = ?", id, post.DeletedAt.Time).
		Update("deleted_at", nil)
	if res.Error != nil {
		return newErr(http.StatusInternalServerError, "failed to restore post")
	}
	if res.RowsAffected == 0 {
		return newErr(http.StatusConflict, "this post is no longer in the trash")
	}
	s.audit(ctx, models.User{}, "restore_post", "post:"+id, "title="+post.Title)
	return nil
}

// EmptyTrash permanently deletes every soft-deleted post (regardless of age),
// releasing their SQLite rows. Returns how many posts were removed.
//
// The image ledger is cleaned up in the same pass: a purged article can no
// longer reference anything, and stale edges would make the gallery warn about
// breaking articles that do not exist.
func (s *Service) EmptyTrash(ctx context.Context) (int, error) {
	var ids []string
	if err := s.db.WithContext(ctx).Unscoped().Model(&models.Post{}).
		Where("deleted_at IS NOT NULL").Pluck("id", &ids).Error; err != nil {
		return 0, newErr(http.StatusInternalServerError, "failed to read trash")
	}

	res := s.db.WithContext(ctx).Unscoped().
		Where("deleted_at IS NOT NULL").
		Delete(&models.Post{})
	if res.Error != nil {
		return 0, newErr(http.StatusInternalServerError, "failed to empty trash")
	}

	if len(ids) > 0 {
		// Best effort: the posts are already gone, which is what the caller
		// asked for. A failed ledger cleanup only makes ref_count stale.
		_ = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var uploadIDs []string
			if err := tx.Model(&models.UploadRef{}).Where("post_id IN ?", ids).
				Pluck("upload_id", &uploadIDs).Error; err != nil {
				return err
			}
			if err := tx.Where("post_id IN ?", ids).Delete(&models.UploadRef{}).Error; err != nil {
				return err
			}
			return recountUploadRefsTx(tx, uploadIDs)
		})
	}

	s.audit(ctx, models.User{}, "empty_trash", "trash", "deleted="+strconv.Itoa(int(res.RowsAffected)))
	return int(res.RowsAffected), nil
}
