package services

import (
	"context"
	"net/http"
	"strconv"

	"lopezpress/internal/models"
)

// Revision history (spec 31).
//
// Every UpdatePost first snapshots the CURRENT row, so what you get back is
// "the state before this save". Rolling back is therefore just another update:
// it snapshots again, which means even a bad restore is undoable.
//
// Cap: 20 rows per post. On a 1 vCPU box the alternative (unbounded history)
// means a blog edited daily for two years carries ~700 copies of every long
// article in a single SQLite file.

const maxRevisionsPerPost = 20

// snapshotRevision stores one copy of the pre-edit state and prunes the pool.
// Best-effort: a failure here must never block the user's save.
func (s *Service) snapshotRevision(ctx context.Context, post models.Post, editorID string) {
	rec := models.PostRevision{
		PostID:   post.ID,
		Title:    post.Title,
		Body:     post.Body,
		Status:   post.Status,
		EditorID: editorID,
	}
	if err := s.db.WithContext(ctx).Create(&rec).Error; err != nil {
		return
	}
	var ids []int64
	if err := s.db.WithContext(ctx).Model(&models.PostRevision{}).
		Where("post_id = ?", post.ID).
		Order("created_at DESC, id DESC").
		Pluck("id", &ids).Error; err != nil {
		return
	}
	if len(ids) > maxRevisionsPerPost {
		_ = s.db.WithContext(ctx).Where("post_id = ? AND id IN ?", post.ID, ids[maxRevisionsPerPost:]).
			Delete(&models.PostRevision{}).Error
	}
}

// ListRevisions returns the history of one post, newest first.
//
// Permission mirrors UpdatePost: the author, or an admin/root who is allowed
// to manage that author. A plain user may not read someone else's history —
// old drafts of a private post are as sensitive as the post itself.
func (s *Service) ListRevisions(ctx context.Context, user models.User, postID string) ([]models.PostRevision, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", postID).First(&post).Error; err != nil {
		return nil, newErr(http.StatusNotFound, "post not found")
	}
	if err := s.assertCanEdit(ctx, user, post); err != nil {
		return nil, err
	}
	var revs []models.PostRevision
	if err := s.db.WithContext(ctx).Where("post_id = ?", postID).
		Order("created_at DESC, id DESC").Limit(maxRevisionsPerPost).
		Find(&revs).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load revisions")
	}
	if revs == nil {
		revs = []models.PostRevision{}
	}
	return revs, nil
}

// RestoreRevision writes an old version back over the current content.
//
// The status is deliberately NOT restored: rolling back text must not roll
// back a moderation decision (e.g. resurrecting a rejected post into
// published). Only title/body travel back.
func (s *Service) RestoreRevision(ctx context.Context, user models.User, postID string, revID int64) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", postID).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	if err := s.assertCanEdit(ctx, user, post); err != nil {
		return models.Post{}, err
	}
	var rev models.PostRevision
	if err := s.db.WithContext(ctx).Where("id = ? AND post_id = ?", revID, postID).First(&rev).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "revision not found")
	}
	// UpdatePost snapshots the current state first, so this restore is itself
	// undoable. It also writes its own "update_post" row; the extra row here
	// records WHICH revision was restored, which the generic update cannot
	// express.
	restored, err := s.UpdatePost(ctx, user, postID, UpdatePostInput{Title: rev.Title, Body: rev.Body})
	if err != nil {
		return models.Post{}, err
	}
	s.audit(ctx, user, "restore_revision", "post:"+postID, "revision="+strconv.FormatInt(revID, 10))
	return restored, nil
}

// assertCanEdit is the shared "may this caller edit this post" gate used by the
// revision endpoints. It intentionally duplicates the rule already inside
// UpdatePost rather than re-ordering that function: the two must agree, and a
// single predicate is easier to spot-check than two interleaved blocks.
func (s *Service) assertCanEdit(ctx context.Context, user models.User, post models.Post) error {
	if post.AuthorID == user.ID || user.Role == models.RoleRoot {
		return nil
	}
	if user.Role != models.RoleAdmin {
		return newErr(http.StatusForbidden, "you can only edit your own posts")
	}
	var author models.User
	if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
		return newErr(http.StatusForbidden, "you can only edit your own posts")
	}
	if author.Role != models.RoleUser {
		return newErr(http.StatusForbidden, "you can only edit your own posts")
	}
	return nil
}
