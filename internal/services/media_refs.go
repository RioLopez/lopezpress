package services

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lopezpress/internal/models"
)

// --- image library: deletion and the reference ledger -------------
//
// Deletion policy (decided in round 77):
//   - An author may withdraw their own image. That is a SOFT delete: the row
//     survives with deleted_at/deleted_by set and live_key rotated, so the file
//     keeps being served to articles that already embed it (breaking published
//     content would be the greater harm) while it disappears from every list
//     and can never be handed to another user as a dedup hit.
//   - An admin may also hard-delete any image, including one that articles
//     reference. That is deliberate: moderating a bad image must not be blocked
//     by the people who embedded it, and the caller is told how many refs will
//     break.

// maxUploadRefsPerPost bounds the regex scan. A 10k-image article is not a
// real use case; a body crafted to make the scan quadratic is.
const maxUploadRefsPerPost = 300

// uploadRefPattern matches the only URLs this instance can own. The character
// class is exactly what the uploader can produce (base62 id + extension) plus
// the legacy "<ms>-<hex>" form, which keeps old articles in the ledger.
var uploadRefPattern = regexp.MustCompile(`/uploads/([A-Za-z0-9][A-Za-z0-9._-]{0,150})`)

// DeleteUpload soft- or hard-deletes one image.
//
// Permissions are decided here, not by the route: the admin route and the
// self-service route both land in this function, and the difference between
// them is only who the caller is. A plain user may withdraw their own upload
// (soft, and only while the site owner allows it); admin/root may withdraw
// anybody's and may hard-delete.
func (s *Service) DeleteUpload(ctx context.Context, me models.User, name string, hard bool) (UploadDeleteResult, error) {
	var row models.UploadFile
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&row).Error; err != nil {
		return UploadDeleteResult{}, newErr(http.StatusNotFound, "image not found")
	}

	privileged := IsAdminOrRoot(me)
	if hard && !privileged {
		return UploadDeleteResult{}, newErr(http.StatusForbidden, "only an admin can permanently delete an image")
	}
	if !privileged {
		if row.OwnerID != me.ID {
			return UploadDeleteResult{}, newErr(http.StatusForbidden, "you can only delete your own uploads")
		}
		cfg, _ := s.GetSiteConfig(ctx, false)
		if !cfg.AllowUserDeleteUpload {
			return UploadDeleteResult{}, newErr(http.StatusForbidden, "deleting your own uploads is disabled by the site owner")
		}
	}

	if hard {
		return s.hardDeleteUpload(ctx, me, row)
	}
	return s.softDeleteUpload(ctx, me, row)
}

// softDeleteUpload withdraws an image: invisible and un-reusable, still served.
func (s *Service) softDeleteUpload(ctx context.Context, me models.User, row models.UploadFile) (UploadDeleteResult, error) {
	if row.DeletedAt != nil {
		return UploadDeleteResult{Name: row.Name, Hard: false}, nil
	}
	res := s.db.WithContext(ctx).Model(&models.UploadFile{}).
		Where("id = ? AND deleted_at IS NULL", row.ID).
		Updates(map[string]any{
			"deleted_at": time.Now(),
			"deleted_by": me.ID,
			// Rotating the live key is what makes "a withdrawn image can never
			// be handed to somebody else" a database invariant: the dedup
			// lookup searches live_key, and this row no longer answers to its
			// own hash.
			"live_key": "d:" + row.ID,
		})
	if res.Error != nil {
		return UploadDeleteResult{}, newErr(http.StatusInternalServerError, "failed to delete image")
	}
	if res.RowsAffected == 0 {
		return UploadDeleteResult{Name: row.Name, Hard: false}, nil
	}
	s.audit(ctx, me, "delete_upload", "upload:"+row.Name,
		fmt.Sprintf("soft=1; refs=%d; size=%d", row.RefCount, row.Size))
	return UploadDeleteResult{Name: row.Name, Hard: false, AffectedRefs: int64(row.RefCount)}, nil
}

// hardDeleteUpload removes the row and the file. References are counted first
// so the caller can be told what will break; the deletion is not blocked on
// them, because moderating an image must not require the consent of everyone
// who embedded it.
func (s *Service) hardDeleteUpload(ctx context.Context, me models.User, row models.UploadFile) (UploadDeleteResult, error) {
	var refs int64
	if err := s.db.WithContext(ctx).Model(&models.UploadRef{}).
		Where("upload_id = ?", row.ID).Count(&refs).Error; err != nil {
		return UploadDeleteResult{}, newErr(http.StatusInternalServerError, "failed to count references")
	}
	// File first: if the row is gone but the file survives, nothing can ever
	// find it again (the row was the only index). The other order leaves a
	// broken image in the gallery. The thumbnail goes with it — it is a
	// derived artefact of bytes that no longer exist.
	if err := s.DeleteLocalUpload(ctx, row.Name); err != nil && !isNotFound(err) {
		return UploadDeleteResult{}, err
	}
	s.removeThumb(ctx, row.ThumbName)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("upload_id = ?", row.ID).Delete(&models.UploadRef{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", row.ID).Delete(&models.UploadFile{}).Error
	})
	if err != nil {
		return UploadDeleteResult{}, newErr(http.StatusInternalServerError, "failed to delete image record")
	}
	s.audit(ctx, me, "delete_upload", "upload:"+row.Name,
		fmt.Sprintf("soft=0; refs=%d; size=%d", refs, row.Size))
	return UploadDeleteResult{Name: row.Name, Hard: true, AffectedRefs: refs}, nil
}

// RestoreUpload brings a soft-deleted image back into the library. It fails
// with 409 when an identical image was uploaded in the meantime: the unique
// live_key index would reject the update anyway, and a clear message beats a
// database error.
func (s *Service) RestoreUpload(ctx context.Context, me models.User, name string) error {
	var row models.UploadFile
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&row).Error; err != nil {
		return newErr(http.StatusNotFound, "image not found")
	}
	if row.DeletedAt == nil {
		return nil
	}
	liveKey := "legacy:" + row.Name
	if row.Sha256 != "" {
		var clash int64
		if err := s.db.WithContext(ctx).Model(&models.UploadFile{}).
			Where("live_key = ? AND id <> ?", row.Sha256, row.ID).Count(&clash).Error; err != nil {
			return newErr(http.StatusInternalServerError, "failed to check duplicates")
		}
		if clash > 0 {
			return newErr(http.StatusConflict, "an identical image is already in the library")
		}
		liveKey = row.Sha256
	}
	if err := s.db.WithContext(ctx).Model(&models.UploadFile{}).Where("id = ?", row.ID).
		Updates(map[string]any{"deleted_at": nil, "deleted_by": "", "live_key": liveKey}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to restore image")
	}
	s.audit(ctx, me, "restore_upload", "upload:"+row.Name, "")
	return nil
}

// isNotFound reports whether err carries a 404. StatusOf already maps raw
// non-service errors to 500, so this never misreads a gorm error as "missing".
func isNotFound(err error) bool {
	return err != nil && StatusOf(err) == http.StatusNotFound
}

// --- reference ledger -------------------------------------------------------

// extractUploadNames returns the distinct upload file names referenced by a
// post body, in first-appearance order and capped at maxUploadRefsPerPost.
func extractUploadNames(body string) []string {
	matches := uploadRefPattern.FindAllStringSubmatch(body, maxUploadRefsPerPost)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, m := range matches {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		out = append(out, m[1])
	}
	return out
}

// extractUploadRefNames is the complete set of library images one article
// depends on: everything its body embeds plus its cover.
//
// The cover belongs here, not in a separate "cover" edge kind. Both answer the
// same question — "does deleting this file break an article?" — and giving them
// different kinds would make one article count twice, so the gallery would warn
// "deleting this breaks 2 articles" about a single post.
func extractUploadRefNames(body, cover string) []string {
	names := extractUploadNames(body)
	name := uploadNameFromURL(cover)
	if name == "" {
		return names
	}
	for _, n := range names {
		if n == name {
			return names
		}
	}
	// One extra slot over the body cap: the cover is a single URL, so this
	// stays bounded even when the body already hit maxUploadRefsPerPost.
	return append(names, name)
}

// syncPostUploadRefsTx reconciles one post's image references inside the
// caller's transaction, so an article and its ledger never disagree.
//
// It is a diff, not a rebuild: only the edges that appeared or disappeared are
// written. Saving the same body twice touches nothing, which matters because
// every save runs this.
func syncPostUploadRefsTx(tx *gorm.DB, postID, userID, body, cover string) error {
	names := extractUploadRefNames(body, cover)

	newIDs := make(map[string]bool, len(names))
	if len(names) > 0 {
		var rows []models.UploadFile
		if err := tx.Select("id").Where("name IN ?", names).Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			newIDs[r.ID] = true
		}
	}

	var oldIDs []string
	if err := tx.Model(&models.UploadRef{}).
		Where("post_id = ? AND kind = ?", postID, "body").
		Pluck("upload_id", &oldIDs).Error; err != nil {
		return err
	}
	oldSet := make(map[string]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}

	touched := make([]string, 0, len(newIDs)+len(oldIDs))
	for id := range newIDs {
		if oldSet[id] {
			continue
		}
		ref := models.UploadRef{UploadID: id, PostID: postID, Kind: "body", UserID: userID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
			return err
		}
		touched = append(touched, id)
	}

	removed := make([]string, 0, len(oldIDs))
	for _, id := range oldIDs {
		if !newIDs[id] {
			removed = append(removed, id)
			touched = append(touched, id)
		}
	}
	if len(removed) > 0 {
		if err := tx.Where("post_id = ? AND kind = ? AND upload_id IN ?", postID, "body", removed).
			Delete(&models.UploadRef{}).Error; err != nil {
			return err
		}
	}
	return recountUploadRefsTx(tx, touched)
}

// releasePostUploadRefsTx drops every edge owned by a post. Called when the
// post leaves the site (deleted or permanently purged) so the ledger does not
// claim that a gone article still needs the file.
func releasePostUploadRefsTx(tx *gorm.DB, postID string) error {
	var ids []string
	if err := tx.Model(&models.UploadRef{}).
		Where("post_id = ?", postID).Pluck("upload_id", &ids).Error; err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if err := tx.Where("post_id = ?", postID).Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}
	return recountUploadRefsTx(tx, ids)
}

// recountUploadRefsTx refreshes the cached ref_count of the given uploads from
// upload_refs, which is the authoritative source. One UPDATE with a correlated
// subquery beats N round trips on a low-end VPS.
func recountUploadRefsTx(tx *gorm.DB, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return tx.Model(&models.UploadFile{}).Where("id IN ?", ids).
		Update("ref_count",
			gorm.Expr("(SELECT COUNT(*) FROM upload_refs WHERE upload_refs.upload_id = upload_files.id)")).Error
}
