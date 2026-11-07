package services

import (
	"context"
	"log"
	"net/http"

	"lopezpress/internal/models"
)

// --- thumbnail outcome bookkeeping and admin retry -----------------
//
// Kept apart from the worker in thumbs.go so neither file has to grow past
// the 350-line gate: the worker owns decoding, this file owns "what happened"
// and the one manual escape hatch.

// failThumb records a failed render. The first failure leaves the row pending
// so the periodic sweep tries once more — a transient read error deserves a
// second look — while the second one is terminal. A corrupt image simply fails
// both times and lands in "failed"; nothing loops.
func (s *Service) failThumb(ctx context.Context, row models.UploadFile) {
	attempts := row.ThumbAttempts + 1
	status := ThumbStatusFailed
	if attempts == 1 {
		status = ThumbStatusPending
		log.Printf("thumbnail queue: %s failed once, will retry", row.ID)
	}
	s.markThumb(ctx, row.ID, status, "", attempts)
}

// markThumb records the outcome. Errors are ignored on purpose: the column is
// an optimisation hint, and failing to update it must never fail the upload or
// the delete that triggered it.
func (s *Service) markThumb(ctx context.Context, id, status, name string, attempts int) {
	if err := s.db.WithContext(ctx).Model(&models.UploadFile{}).Where("id = ?", id).
		Updates(map[string]any{
			"thumb_status":   status,
			"thumb_name":     name,
			"thumb_attempts": attempts,
		}).Error; err != nil {
		log.Printf("WARN thumbnail queue: cannot record %s for %s: %v", status, id, err)
	}
}

// RetryUploadThumb resets one row's thumbnail state and queues it again. This
// is the manual escape hatch for a row stuck in "failed" (M-8): the worker
// retries once automatically, but an admin who knows the file was replaced can
// ask for another attempt without touching the database.
//
// Ready thumbnails are refused: regenerating one would leave the previous file
// name behind when the new encode picks the other extension, and there is
// nothing wrong with it to fix.
func (s *Service) RetryUploadThumb(ctx context.Context, actor models.User, name string) error {
	if !IsAdminOrRoot(actor) {
		return newErr(http.StatusForbidden, "only an admin or the root can retry a thumbnail")
	}
	var row models.UploadFile
	if err := s.db.WithContext(ctx).Where("name = ?", name).First(&row).Error; err != nil {
		return newErr(http.StatusNotFound, "upload not found")
	}
	if row.DeletedAt != nil {
		return newErr(http.StatusBadRequest, "a withdrawn image has no thumbnail")
	}
	if row.ThumbStatus == ThumbStatusReady {
		return newErr(http.StatusBadRequest, "thumbnail is already ready")
	}
	if err := s.db.WithContext(ctx).Model(&models.UploadFile{}).Where("id = ?", row.ID).
		Updates(map[string]any{
			"thumb_status":   ThumbStatusPending,
			"thumb_name":     "",
			"thumb_attempts": 0,
		}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to queue the thumbnail")
	}
	s.enqueueThumb(row.ID)
	return nil
}
