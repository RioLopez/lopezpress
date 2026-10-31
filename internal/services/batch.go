package services

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"lopezpress/internal/models"
)

// BatchPostAction is the closed set of bulk operations the admin console may
// request. Anything else is a 400 — never a silent no-op.
const (
	BatchPublish     = "publish"
	BatchPrivate     = "private"
	BatchDelete      = "delete"
	BatchSetCategory = "set_category"
	BatchAddTags     = "add_tags"
	BatchRemoveTags  = "remove_tags"
)

// maxBatchIDs caps a single request. The loop below reuses the single-item
// service methods (and thus their transactions), so a huge batch would hold
// the SQLite write lock for a long time on a 1 vCPU box.
const maxBatchIDs = 100

// BatchSkip records one post that could not be processed, with the
// BatchUpdatePosts applies one bulk action to many posts.
//
// DESIGN: every action delegates to the existing single-item service method
// (UpdatePost / DeletePost) instead of re-implementing the rules. That means
// the role matrix, the review state machine and the "banned posts are frozen"
// guard are inherited by construction — there is no second permission layer to
// drift out of sync. It is slower per item (each call opens its own
// transaction) and that is an accepted trade for correctness on a
// single-writer SQLite database with batches capped at 100.
func (s *Service) BatchUpdatePosts(ctx context.Context, actor models.User, in BatchPostInput) (BatchPostResult, error) {
	if len(in.IDs) == 0 {
		return BatchPostResult{}, newErr(http.StatusBadRequest, "ids is required")
	}
	if len(in.IDs) > maxBatchIDs {
		return BatchPostResult{}, newErr(http.StatusBadRequest, "too many ids (max 100)")
	}
	switch in.Action {
	case BatchPublish, BatchPrivate, BatchDelete, BatchSetCategory, BatchAddTags, BatchRemoveTags:
	default:
		return BatchPostResult{}, newErr(http.StatusBadRequest, "unknown action")
	}
	if in.Action == BatchSetCategory && in.CategoryID == nil {
		return BatchPostResult{}, newErr(http.StatusBadRequest, "category_id is required")
	}
	if (in.Action == BatchAddTags || in.Action == BatchRemoveTags) && len(in.Tags) == 0 {
		return BatchPostResult{}, newErr(http.StatusBadRequest, "tags is required")
	}
	// Publishing is root-only in the single-item path too; failing the whole
	// batch up front is clearer than returning N identical skips.
	if in.Action == BatchPublish && actor.Role != models.RoleRoot {
		return BatchPostResult{}, newErr(http.StatusForbidden, "only the root can publish posts")
	}

	res := BatchPostResult{Skipped: []BatchSkip{}}
	// De-duplicate ids so a malformed client cannot double-apply an action.
	seen := make(map[string]bool, len(in.IDs))
	for _, id := range in.IDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true

		if err := s.applyBatchOne(ctx, actor, id, in); err != nil {
			res.Skipped = append(res.Skipped, BatchSkip{ID: id, Reason: MsgOf(err)})
			continue
		}
		res.Affected++
	}
	// One summary row in addition to the per-post rows the single-item paths
	// already write: "who ran a bulk action over how many posts" is the thing
	// an operator actually searches for.
	s.audit(ctx, actor, "batch_"+in.Action, "posts",
		"affected="+strconv.Itoa(res.Affected)+"; skipped="+strconv.Itoa(len(res.Skipped)))
	return res, nil
}

// applyBatchOne routes a single id through the matching single-item method.
func (s *Service) applyBatchOne(ctx context.Context, actor models.User, id string, in BatchPostInput) error {
	switch in.Action {
	case BatchPublish:
		_, err := s.UpdatePost(ctx, actor, id, UpdatePostInput{Status: models.PostPublished})
		return err
	case BatchPrivate:
		_, err := s.UpdatePost(ctx, actor, id, UpdatePostInput{Status: models.PostPrivate})
		return err
	case BatchDelete:
		return s.DeletePost(ctx, actor, id)
	case BatchSetCategory:
		// A nil category in the payload is rejected above, so passing the
		// pointer straight through means "set to this category".
		_, err := s.UpdatePost(ctx, actor, id, UpdatePostInput{CategoryID: in.CategoryID})
		return err
	case BatchAddTags, BatchRemoveTags:
		var post models.Post
		if err := s.db.WithContext(ctx).Preload("Tags").Where("id = ?", id).First(&post).Error; err != nil {
			return newErr(http.StatusNotFound, "post not found")
		}
		merged := mergeTags(post.Tags, in.Tags, in.Action == BatchRemoveTags)
		_, err := s.UpdatePost(ctx, actor, id, UpdatePostInput{Tags: merged})
		return err
	}
	return newErr(http.StatusBadRequest, "unknown action")
}

// mergeTags computes the resulting tag-name set for add/remove. Add is a
// de-duplicated union (so adding an existing tag is a no-op); remove drops
// every name in `delta`, case-insensitively.
func mergeTags(current []models.Tag, delta []string, remove bool) []string {
	out := make([]string, 0, len(current)+len(delta))
	if remove {
		drop := make(map[string]bool, len(delta))
		for _, d := range delta {
			drop[strings.ToLower(strings.TrimSpace(d))] = true
		}
		for _, tag := range current {
			if !drop[strings.ToLower(tag.Name)] {
				out = append(out, tag.Name)
			}
		}
		// Never return nil: a nil slice means "don't touch tags" in
		// UpdatePost, which would silently ignore a "remove everything".
		if out == nil {
			out = []string{}
		}
		return out
	}
	seen := make(map[string]bool, len(current)+len(delta))
	for _, tag := range current {
		key := strings.ToLower(tag.Name)
		if !seen[key] {
			seen[key] = true
			out = append(out, tag.Name)
		}
	}
	for _, d := range delta {
		name := strings.TrimSpace(d)
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}
