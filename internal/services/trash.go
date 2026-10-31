package services

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- trash (soft-deleted post recovery, 7-day window) ---

// trashWindow is how long a deleted post stays recoverable.
const trashWindow = 7 * 24 * time.Hour

// mayManageAuthorOf reports whether `me` may take destructive action on a post
// written by `authorID`. It is the single rule behind DeletePost and every
// trash endpoint.
//
// These used to disagree. DeletePost refused to let a plain admin delete a
// root's or another admin's post (403), but EmptyTrash took no actor at all and
// purged whatever was soft-deleted — so the same admin who could not delete a
// root's article could still destroy it irreversibly one click later, and
// RestoreTrash could publish it back. Two paths, one piece of content, opposite
// answers. Now there is one answer.
//
//   - the author may always act on their own content
//   - root may act on anyone's
//   - a plain admin may act on plain users' content, never on a peer's or root's
func (s *Service) mayManageAuthorOf(ctx context.Context, me models.User, authorID string) bool {
	if authorID == me.ID {
		return true
	}
	if me.Role == models.RoleRoot {
		return true
	}
	if me.Role != models.RoleAdmin {
		return false
	}
	var author models.User
	if err := s.db.WithContext(ctx).Select("role").Where("id = ?", authorID).First(&author).Error; err != nil {
		// Unknown author: treat as unmanageable rather than assume the safe case.
		return false
	}
	return author.Role == models.RoleUser
}

// trashScope narrows a trash query to the rows `me` is allowed to manage. It
// mirrors mayManageAuthorOf in SQL so listing and acting cannot drift apart:
// an admin sees their own deleted posts plus plain users', and nothing else.
func (s *Service) trashScope(q *gorm.DB, me models.User) *gorm.DB {
	if me.Role == models.RoleRoot {
		return q
	}
	plainUsers := s.db.Model(&models.User{}).Select("id").Where("role = ?", models.RoleUser)
	return q.Where("(author_id = ? OR author_id IN (?))", me.ID, plainUsers)
}

// ListTrash returns soft-deleted posts whose deletion is within the recovery
// window, most recent first. Authors are preloaded so the UI can show who
// wrote each recovered post.
//
// The list is scoped to what the caller may actually act on: showing an admin a
// root's post they cannot restore (or purge) is only an invitation to try.
func (s *Service) ListTrash(ctx context.Context, me models.User) ([]TrashPost, error) {
	cutoff := time.Now().Add(-trashWindow)
	var rows []models.Post
	q := s.db.WithContext(ctx).
		Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at >= ?", cutoff)
	err := s.trashScope(q, me).
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

// errLeftTrash is the sentinel RestoreTrash returns from inside its transaction
// when a concurrent restore/empty won the race. Mapping it to 409 happens
// outside, after the transaction has rolled back.
var errLeftTrash = errors.New("post is no longer in the trash")

// RestoreTrash resurrects a soft-deleted post. Only posts inside the 7-day
// window are restorable; older rows return 410 Gone.
//
// R4-11: "is it in the trash" used to be checked on the row we read and then
// not enforced on the write, so a restore racing "empty trash" answered
// "restored" to the operator while the row was permanently destroyed a moment
// later — and the post was gone from both the trash and the site. The
// conditions are now part of the UPDATE, and `RowsAffected == 0` means the row
// left the trash under us.
//
// The image ledger is restored in the same transaction. DeletePost released
// this post's edges when it entered the trash (a deleted article must not keep
// an image alive, or nothing would ever become an orphan), so a restore that
// only clears deleted_at brings back an article whose images read as unused —
// and the gallery will happily let an admin hard-delete them out from under the
// freshly restored content.
func (s *Service) RestoreTrash(ctx context.Context, me models.User, id string) error {
	var post models.Post
	err := s.db.WithContext(ctx).Unscoped().Where("id = ?", id).First(&post).Error
	if err != nil {
		return newErr(http.StatusNotFound, "post not found")
	}
	// Permission before state: an admin must not learn "that id is restorable"
	// for content they could never restore.
	if !s.mayManageAuthorOf(ctx, me, post.AuthorID) {
		return newErr(http.StatusForbidden, "you do not have permission to manage this post")
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
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Unscoped().Model(&models.Post{}).
			Where("id = ? AND deleted_at = ?", id, post.DeletedAt.Time).
			Update("deleted_at", nil)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errLeftTrash
		}
		return syncPostUploadRefsTx(tx, post.ID, post.AuthorID, post.Body, post.Cover)
	})
	if errors.Is(err, errLeftTrash) {
		return newErr(http.StatusConflict, "this post is no longer in the trash")
	}
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to restore post")
	}
	s.audit(ctx, me, "restore_post", "post:"+id, "title="+post.Title)
	return nil
}

// EmptyTrash permanently deletes every soft-deleted post the caller is allowed
// to manage (regardless of age), releasing their SQLite rows. Returns how many
// posts were removed.
//
// Scoped, not global: "empty the trash" empties what this operator can see and
// could have deleted one by one. An admin purging a root's article through this
// door while DeletePost tells them 403 is exactly the contradiction this fixes.
//
// The image ledger is cleaned up in the same transaction: a purged article can
// no longer reference anything, and stale edges would make the gallery warn
// about breaking articles that do not exist.
//
// Same transaction, not "best effort afterwards": the earlier version deleted
// the rows and then tried the ledger cleanup in a separate transaction, so a
// failure there left ref_count permanently too high with no endpoint that could
// ever recompute it — the operator sees "still used by 1 article" for a post
// that no longer exists, and cannot delete the image. SQLite serialises writers,
// so the pair is atomic at no cost.
func (s *Service) EmptyTrash(ctx context.Context, me models.User) (int, error) {
	var ids []string
	q := s.db.WithContext(ctx).Unscoped().Model(&models.Post{}).
		Where("deleted_at IS NOT NULL")
	if err := s.trashScope(q, me).Pluck("id", &ids).Error; err != nil {
		return 0, newErr(http.StatusInternalServerError, "failed to read trash")
	}
	if len(ids) == 0 {
		return 0, nil
	}

	var removed int
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete by the ids resolved above, re-applying the scope so a post
		// whose author changed role between the two statements cannot slip
		// through.
		del := tx.Unscoped().Where("deleted_at IS NOT NULL AND id IN ?", ids)
		res := s.trashScope(del, me).Delete(&models.Post{})
		if res.Error != nil {
			return res.Error
		}
		removed = int(res.RowsAffected)

		// Ledger cleanup is expressed as "edges pointing at a post row that no
		// longer exists" rather than "edges for these ids". The two differ
		// exactly when the scope predicate dropped a row at delete time, and in
		// that case the id-list form would strip the edges of a post that
		// survived. The subquery is raw SQL on purpose: it must see soft-deleted
		// rows too (a trashed post can still be restored, so its edges stay).
		const orphanEdges = "post_id <> '' AND post_id NOT IN (SELECT id FROM posts)"
		var uploadIDs []string
		if err := tx.Model(&models.UploadRef{}).Where(orphanEdges).
			Pluck("upload_id", &uploadIDs).Error; err != nil {
			return err
		}
		if err := tx.Where(orphanEdges).Delete(&models.UploadRef{}).Error; err != nil {
			return err
		}
		return recountUploadRefsTx(tx, uploadIDs)
	})
	if err != nil {
		return 0, newErr(http.StatusInternalServerError, "failed to empty trash")
	}

	s.audit(ctx, me, "empty_trash", "trash", "deleted="+strconv.Itoa(removed))
	return removed, nil
}
