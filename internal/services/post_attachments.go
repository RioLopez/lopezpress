package services

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lopezpress/internal/models"
)

// --- post attachments -------------------------------------------------------
//
// The join table is the source of truth for "this article carries these PDFs".
// upload_refs mirrors it as kind="attachment" so the media library can answer
// "is deleting this file safe?" with the same ledger it uses for images.
//
// Every write here is inside the caller's transaction: a post save, a trash
// restore and a hard delete must not leave the two tables disagreeing.

// syncPostAttachmentsTx replaces one post's attachment set. It is a
// replace-all, not a diff: the editor sends the complete ordered list, and a
// diff would need stable ids for rows the client never saw.
func (s *Service) syncPostAttachmentsTx(ctx context.Context, tx *gorm.DB, postID, authorID string, ids []string, actor models.User) error {
	valid, err := validatePDFAttachmentIDsTx(tx, ids, actor)
	if err != nil {
		return err
	}
	var oldIDs []string
	if err := tx.Model(&models.UploadRef{}).
		Where("post_id = ? AND kind = ?", postID, "attachment").
		Pluck("upload_id", &oldIDs).Error; err != nil {
		return err
	}
	if err := tx.Where("post_id = ?", postID).Delete(&models.PostAttachment{}).Error; err != nil {
		return err
	}
	if err := tx.Where("post_id = ? AND kind = ?", postID, "attachment").
		Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}
	for i, uploadID := range valid {
		row := models.PostAttachment{PostID: postID, UploadID: uploadID, SortOrder: i}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		ref := models.UploadRef{UploadID: uploadID, PostID: postID, Kind: "attachment", UserID: authorID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
			return err
		}
	}
	touched := append(oldIDs, valid...)
	return recountUploadRefsTx(tx, touched)
}

// syncPostAttachmentRefsTx rebuilds the ledger edges from the join rows that
// already exist. Trash restore needs this: DeletePost released every edge, but
// the join rows stayed with the post so a restore can bring the documents back.
func syncPostAttachmentRefsTx(tx *gorm.DB, postID, authorID string) error {
	var ids []string
	if err := tx.Model(&models.PostAttachment{}).
		Where("post_id = ?", postID).Pluck("upload_id", &ids).Error; err != nil {
		return err
	}
	if err := tx.Where("post_id = ? AND kind = ?", postID, "attachment").
		Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}
	for _, uploadID := range ids {
		ref := models.UploadRef{UploadID: uploadID, PostID: postID, Kind: "attachment", UserID: authorID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
			return err
		}
	}
	return recountUploadRefsTx(tx, ids)
}

// releasePostAttachmentsTx removes the join rows of a post that is being
// purged for good. The upload ledger edges are released separately by
// releasePostUploadRefsTx, which covers every kind for the post at once.
func releasePostAttachmentsTx(tx *gorm.DB, postID string) error {
	return tx.Where("post_id = ?", postID).Delete(&models.PostAttachment{}).Error
}

// loadPostAttachments fills Post.Attachments for the detail read paths. A
// missing upload row (hard-deleted file) simply drops that attachment: the
// reader cannot open bytes that no longer exist, and showing a dead card would
// be worse than showing none.
func (s *Service) loadPostAttachments(ctx context.Context, post *models.Post) {
	var rows []models.PostAttachmentView
	err := s.db.WithContext(ctx).Table("post_attachments AS pa").
		Select("pa.id AS id, pa.upload_id AS upload_id, pa.sort_order AS sort_order, "+
			"uf.name AS name, uf.original_name AS original_name, uf.size AS size, uf.mime AS mime").
		Joins("JOIN upload_files AS uf ON uf.id = pa.upload_id").
		Where("pa.post_id = ?", post.ID).
		Order("pa.sort_order ASC, pa.id ASC").
		Scan(&rows).Error
	if err != nil {
		return
	}
	for i := range rows {
		rows[i].URL = "/uploads/" + rows[i].Name
	}
	post.Attachments = rows
}
