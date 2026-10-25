package services

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- state transitions ------------------------------------------------------
//
// Every post state change on this site is "read the row, check the state we
// expect, write the new state". Done as two statements that is a TOCTOU window:
// two reviewers both read "pending" and both approve (two mails, two audit
// rows, one of them describing a transition that never happened), and worse,
// an approve racing a delete re-publishes a post the author just removed —
// because `WHERE id = ?` matches a row in ANY state.
//
// R4-11 measured both: approve‖delete produced a 500, and restore‖empty-trash
// left the caller told "restored" while the row was permanently destroyed a
// moment later.
//
// The fix is one rule, applied everywhere: the expected current state goes INTO
// the UPDATE's WHERE clause, and a zero RowsAffected means "somebody else moved
// this row first" — answered with 409, never silently retried. The read before
// the write remains, because the permission checks need the row (whose post is
// it, what did it say), but it no longer decides whether the write is allowed.

// errStateChanged is returned when a conditional transition matched no row.
// Callers translate it into 409 with a message about the post, not the DB.
var errStateChanged = errors.New("post state changed concurrently")

// transitionPost applies updates only if the post is still in one of the
// `from` states, and reports whether this caller won the transition.
//
// from == nil means "no state precondition" (used by transitions that are
// valid from any live state); the row-existence and soft-delete conditions are
// still enforced by GORM's own scope plus the explicit deleted_at clause.
func (s *Service) transitionPost(ctx context.Context, id string, from []string, updates map[string]any) (bool, error) {
	q := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ? AND deleted_at IS NULL", id)
	if len(from) > 0 {
		q = q.Where("status IN ?", from)
	}
	res := q.Updates(updates)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// transitionPostTx is transitionPost bound to a caller's transaction, for the
// transitions that also touch related rows (tags, references) and must be
// all-or-nothing with the status change.
func transitionPostTx(tx *gorm.DB, id string, from []string, updates map[string]any) (bool, error) {
	q := tx.Model(&models.Post{}).Where("id = ? AND deleted_at IS NULL", id)
	if len(from) > 0 {
		q = q.Where("status IN ?", from)
	}
	res := q.Updates(updates)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
