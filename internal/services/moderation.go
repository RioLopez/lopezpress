package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"lopezpress/internal/models"
)

// --- review workflow + banned vault (spec: content moderation) ---

// ReviewPostInput is the decoded payload for the review endpoint.
// Action is "approve" (publish) or "reject" (send back to the author with a
// note). Note is the reviewer's free-text comment, required for reject and
// optional for approve.
type ReviewPostInput struct {
	// Action is the review decision: "approve" | "reject". The JSON tag is
	// "decision" (the wire name used by the admin UI).
	Action string `json:"decision"` // "approve" | "reject"
	Note   string `json:"note"`
}

// ReviewPost approves or rejects a post that is currently in the pending
// queue (or, for reject, a previously rejected post being re-reviewed).
//
// Reviewer rules (spec: "user 被 admin 或 root 审核；admin 被 root 审核"):
//   - A post authored by a plain user may be reviewed by any admin or by root.
//   - A post authored by an admin (or root) may ONLY be reviewed by root.
//
// approve -> status "published", review metadata cleared.
// reject   -> status "rejected", ReviewNote/ReviewedBy/ReviewedAt recorded.
func (s *Service) ReviewPost(ctx context.Context, actor models.User, id string, in ReviewPostInput) (models.Post, error) {
	// Only admins / root can act on the review queue.
	if actor.Role != models.RoleAdmin && actor.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "only an admin or the root can review")
	}
	switch in.Action {
	case "approve", "reject":
		// valid
	default:
		return models.Post{}, newErr(http.StatusBadRequest, "action must be \"approve\" or \"reject\"")
	}

	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}

	// State precondition (as-built audit Q1): review only transitions posts that
	// are actually awaiting review. Acting on a published or private post would
	// suddenly unpublish public content; acting on a banned post would skip the
	// mandatory send-back-to-author step and republish without the author's fixes.
	// Only the review-queue states (pending, rejected) are valid review targets.
	if post.Status != models.PostPending && post.Status != models.PostRejected {
		return models.Post{}, newErr(http.StatusConflict, "post is not awaiting review")
	}

	// Review scope: who may review this post depends on the author's role.
	// Centralised in canReviewPost (root reviews anything; an admin reviews
	// only plain-user content) so the three-tier rule lives in one place.
	var author models.User
	if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
		// Author already deleted — fall through with a zero-value author, which
		// canReviewPost treats as "not a plain user", i.e. root-only.
		author = models.User{}
	}
	if !canReviewPost(actor, author, post) {
		return models.Post{}, newErr(http.StatusForbidden, "only the root can review this post")
	}

	now := time.Now()
	updates := map[string]any{}
	if in.Action == "approve" {
		updates = map[string]any{
			"status":      models.PostPublished,
			"review_note": "",
			"reviewed_by": actor.ID,
			"reviewed_at": now,
		}
	} else { // reject
		// A reject must carry a note so the author knows what to fix.
		if strings.TrimSpace(in.Note) == "" {
			return models.Post{}, newErr(http.StatusBadRequest, "a note is required when rejecting a post")
		}
		updates = map[string]any{
			"status":      models.PostRejected,
			"review_note": in.Note,
			"reviewed_by": actor.ID,
			"reviewed_at": now,
		}
	}
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to review post")
	}

	// Re-load with relations so the response is complete.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	// Best-effort: notify the author of the review outcome (mail may be unset).
	if post.Author != nil {
		s.SendMail(ctx, *post.Author, MailReviewResult, map[string]string{
			"title":  post.Title,
			"action": in.Action,
			"note":   in.Note,
			"time":   now.Format("2006-01-02 15:04 UTC"),
		})
	}
	s.audit(ctx, actor, "review_post", "post:"+post.ID, "action="+in.Action+"; title="+post.Title)
	return post, nil
}

// BanPost hard-moderates a post into the banned vault (root-only).
// Only plain users' and admins' posts may be banned; root's own posts are
// out of scope. The post is retained (soft) with status "banned" and is
// visible to root only.
func (s *Service) BanPost(ctx context.Context, actor models.User, id string) (models.Post, error) {
	if actor.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "only the root can ban a post")
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	// State precondition (as-built audit Q1): a post already in the banned vault
	// cannot be re-banned. Banning is a root take-down for any live/pending/
	// rejected/private post, but re-banning a banned row is a no-op that should
	// fail fast rather than silently rewrite the reviewer timestamps.
	if post.Status == models.PostBanned {
		return models.Post{}, newErr(http.StatusConflict, "post is already in the banned vault")
	}
	// The root cannot ban its own posts.
	if post.AuthorID == actor.ID {
		return models.Post{}, newErr(http.StatusBadRequest, "the root cannot ban its own post")
	}
	now := time.Now()
	updates := map[string]any{
		"status":      models.PostBanned,
		"reviewed_by": actor.ID,
		"reviewed_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to ban post")
	}
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	s.audit(ctx, actor, "ban_post", "post:"+post.ID, "title="+post.Title)
	return post, nil
}

// SendBackPost returns a banned post to its author for revision, attaching
// the reviewer's note. This is the "发回原作者修改并备注意见" disposition
// from the banned vault (root-only). The post becomes status "rejected" so
// it re-enters the normal review flow, and the note is preserved so the
// author knows what to fix.
func (s *Service) SendBackPost(ctx context.Context, actor models.User, id, note string) (models.Post, error) {
	if actor.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "only the root can send a post back")
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	if post.Status != models.PostBanned {
		return models.Post{}, newErr(http.StatusBadRequest, "post is not in the banned vault")
	}
	if strings.TrimSpace(note) == "" {
		return models.Post{}, newErr(http.StatusBadRequest, "a note is required when sending a post back")
	}
	now := time.Now()
	updates := map[string]any{
		"status":      models.PostRejected,
		"review_note": note,
		"reviewed_by": actor.ID,
		"reviewed_at": now,
	}
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to send back post")
	}
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	// Best-effort: notify the author of the send-back and its note.
	if post.Author != nil {
		s.SendMail(ctx, *post.Author, MailReviewResult, map[string]string{
			"title":  post.Title,
			"action": "reject",
			"note":   note,
			"time":   now.Format("2006-01-02 15:04 UTC"),
		})
	}
	s.audit(ctx, actor, "send_back_post", "post:"+post.ID, "title="+post.Title+"; note="+note)
	return post, nil
}

// ListBanned returns every post currently in the banned vault (status
// "banned"), root-only. Author + relations are preloaded for the vault UI.
func (s *Service) ListBanned(ctx context.Context, actor models.User) ([]models.Post, error) {
	if actor.Role != models.RoleRoot {
		return nil, newErr(http.StatusForbidden, "only the root can view the banned vault")
	}
	var posts []models.Post
	if err := s.db.WithContext(ctx).
		Where("posts.status = ?", models.PostBanned).
		Preload("Author").Preload("Category").Preload("Tags").
		Order("posts.created_at DESC").Find(&posts).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load banned posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// ListPending returns every post in the review queue, scoped to what the
// reviewer may act on. For root, all pending posts; for a plain admin, only
// pending posts authored by plain users.
func (s *Service) ListPending(ctx context.Context, actor models.User) ([]models.Post, error) {
	if actor.Role != models.RoleAdmin && actor.Role != models.RoleRoot {
		return nil, newErr(http.StatusForbidden, "only an admin or the root can list the review queue")
	}
	query := s.db.WithContext(ctx).Model(&models.Post{}).
		Where("posts.status = ?", models.PostPending).
		Preload("Author").Preload("Category").Preload("Tags")
	if actor.Role == models.RoleAdmin {
		// Admins see only pending posts by plain users (their own and other
		// admins' posts require root review and are not in an admin's queue).
		query = query.Joins("JOIN users AS author ON author.id = posts.author_id").
			Where("author.role = ?", models.RoleUser)
	}
	query = query.Order("posts.created_at ASC") // oldest first — FIFO queue.
	var posts []models.Post
	if err := query.Find(&posts).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load pending posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// DeleteBannedPermanently purges a banned post from the vault (hard delete,
// root-only). This is the final "删除" disposition from the banned vault.
func (s *Service) DeleteBannedPermanently(ctx context.Context, actor models.User, id string) error {
	if actor.Role != models.RoleRoot {
		return newErr(http.StatusForbidden, "only the root can permanently delete a banned post")
	}
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return newErr(http.StatusNotFound, "post not found")
	}
	if post.Status != models.PostBanned {
		return newErr(http.StatusBadRequest, "post is not in the banned vault")
	}
	// Remove the tag join rows first so the hard delete leaves no orphan
	// associations (which would otherwise keep inflating tag counts).
	if err := s.db.WithContext(ctx).Where("post_id = ?", id).Delete(&models.PostTag{}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete banned post tags")
	}
	// Hard delete so the post is gone from the vault and from the recovery
	// trash alike (banned content is not recoverable).
	if err := s.db.WithContext(ctx).Unscoped().Delete(&models.Post{}, "id = ?", id).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete banned post")
	}
	s.audit(ctx, actor, "delete_banned", "post:"+post.ID, "title="+post.Title)
	return nil
}
