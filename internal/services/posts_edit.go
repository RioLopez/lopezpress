package services

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// UpdatePost edits a post. The author may edit their own post; a plain admin
// may only edit plain users' posts; root may edit anyone's. Returns 403 when
// the caller lacks permission, 404 when the post does not exist.
//
// Status changes are role-gated: root may set published/private; admin and
// user may only set "private" (a restricted, non-public value). Any other
// value is ignored so a tampered client cannot accidentally publish.
func (s *Service) UpdatePost(ctx context.Context, me models.User, id string, in UpdatePostInput) (models.Post, error) {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusNotFound, "post not found")
	}
	// Banned posts are frozen for everyone but the root: once content is
	// pulled into the vault the author must not be able to keep editing it
	// (nor mutate it field-by-field through this endpoint). Root keeps full
	// editorial access so corrections remain possible. (as-built audit Q2)
	if post.Status == models.PostBanned && me.Role != models.RoleRoot {
		return models.Post{}, newErr(http.StatusForbidden, "banned posts can only be edited by the root")
	}
	// Edit permission: the author, or an admin/root who may manage the target.
	// A plain admin may NOT edit another admin's or root's post. A plain user
	// may only edit their own posts — anyone else (including another plain
	// user) is forbidden.
	if post.AuthorID != me.ID {
		if me.Role == models.RoleRoot {
			// root may edit anyone's content.
		} else if me.Role == models.RoleAdmin {
			var author models.User
			if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
				return models.Post{}, newErr(http.StatusForbidden, "you can only edit your own posts")
			}
			if author.Role != models.RoleUser {
				return models.Post{}, newErr(http.StatusForbidden, "you can only edit your own posts")
			}
		} else {
			// A plain user may not edit anyone else's post.
			return models.Post{}, newErr(http.StatusForbidden, "you can only edit your own posts")
		}
	}

	// Spec 31: keep the pre-edit state so the user can roll back. Snapshotting
	// before the assignment (not after) is what makes "restore" mean
	// "the state before this save".
	s.snapshotRevision(ctx, post, me.ID)

	if in.Title != "" {
		post.Title = in.Title
	}
	if in.Body != "" {
		post.Body = in.Body
	}
	// Status whitelist by role (spec: review + banned vault).
	//
	// The private transition is deliberately gated so the author cannot use
	// "save as private" to pull a post out of the review queue (P1-14). An
	// author may only make their own *published* post private (a normal
	// unpublish); reaching "private" from pending/rejected is ignored so the
	// post stays in its moderation lane until reviewed. Banned posts are
	// root-only and root may set them private freely.
	switch {
	case in.Status == models.PostPrivate:
		isAuthor := post.AuthorID == me.ID
		if isAuthor && me.Role != models.RoleRoot {
			// Author path: only published → private is a legal unpublish.
			if post.Status == models.PostPublished {
				post.Status = in.Status
			}
			// pending / rejected / banned: leave the status untouched so the
			// author can't dodge the queue by saving as private.
		} else {
			// root (or an admin editing a user's post): private is a valid,
			// restricted state.
			post.Status = in.Status
		}
	case in.Status == models.PostPending:
		// An author may resubmit their own rejected post back into the
		// review queue, closing the "rejected → fix → resubmit" loop. A
		// fresh submission awaits a new review, so the previous reviewer's
		// note is cleared. No other caller and no other transition may
		// reach "pending" through this endpoint — only root may publish.
		if post.Status == models.PostRejected && post.AuthorID == me.ID {
			post.Status = in.Status
			post.ReviewNote = ""
		}
	case me.Role == models.RoleRoot && in.Status == models.PostPublished:
		// only root can publish (the review outcome).
		post.Status = in.Status
	}
	// Spec 29: an explicit slug overrides the permanent link; otherwise the
	// existing one is never touched (links must stay stable across renames).
	if strings.TrimSpace(in.Slug) != "" {
		slug, slugErr := s.ensureUniqueSlug(ctx, Slugify(in.Slug), post.ID)
		if slugErr != nil {
			return models.Post{}, slugErr
		}
		post.Slug = slug
	}
	// CategoryID is *int64 — nil means "don't change", otherwise set.
	if in.CategoryID != nil {
		post.CategoryID = in.CategoryID
	}
	if in.Pinned != nil {
		post.Pinned = *in.Pinned
	}
	// Cover is an optional field. A non-empty value replaces the stored
	// cover; an empty string is ignored so partial edits (title-only) don't
	// wipe an existing cover.
	if in.Cover != "" {
		post.Cover = in.Cover
	}

	// Persist the post fields and (when supplied) the tag associations in a
	// single transaction. Previously the tag Clear/Append used "_ =", so a
	// failure there left a "saved but tagless" (or partially-tagged) post with
	// a 200 response. Now any association error aborts and rolls back the whole
	// update so the post and its tags stay consistent.
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&post).Error; err != nil {
			return err
		}
		if in.Tags != nil {
			// Drop existing associations, then link the new set. We write the
			// join rows directly instead of using gorm's Association helper
			// (which re-opens a Session and panics inside a Transaction).
			if err := tx.Where("post_id = ?", post.ID).Delete(&models.PostTag{}).Error; err != nil {
				return err
			}
			tags, ferr := s.findOrCreateTagsTx(ctx, tx, in.Tags)
			if ferr != nil {
				return ferr
			}
			if err := linkPostTagsTx(tx, post.ID, tags); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to update post")
	}

	// Re-load with author, category, tags so the response carries full objects.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", post.ID).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	s.audit(ctx, me, "update_post", "post:"+post.ID, "title="+post.Title+"; status="+post.Status)
	return post, nil
}

// DeletePost removes a post. The author may delete their own post; a plain
// admin may delete plain users' posts; root may delete anyone's (including
// pulling a post into the recovery trash). Returns 403 when the caller lacks
// permission, 404 when the post does not exist.
func (s *Service) DeletePost(ctx context.Context, me models.User, id string) error {
	var post models.Post
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&post).Error; err != nil {
		return newErr(http.StatusNotFound, "post not found")
	}
	// A banned post may leave the vault only through the root-only vault
	// endpoints (send-back / permanent delete). Soft-deleting one here would
	// make it vanish from every list while its row stays banned forever —
	// and would 404 the root's permanent-delete path, which reads the post
	// through the soft-delete-aware scope. (as-built audit Q2)
	if post.Status == models.PostBanned {
		return newErr(http.StatusForbidden, "banned posts must be managed in the banned vault")
	}
	// Delete permission: the author, or an admin/root who may manage the target.
	// A plain admin may NOT delete another admin's or root's post. A plain user
	// may only delete their own posts.
	if post.AuthorID != me.ID {
		if me.Role == models.RoleRoot {
			// root may delete anyone's content.
		} else if me.Role == models.RoleAdmin {
			var author models.User
			if err := s.db.WithContext(ctx).Where("id = ?", post.AuthorID).First(&author).Error; err != nil {
				return newErr(http.StatusForbidden, "you can only delete your own posts")
			}
			if author.Role != models.RoleUser {
				return newErr(http.StatusForbidden, "you can only delete your own posts")
			}
		} else {
			// A plain user may not delete anyone else's post.
			return newErr(http.StatusForbidden, "you can only delete your own posts")
		}
	}
	if err := s.db.WithContext(ctx).Delete(&post).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete post")
	}
	s.audit(ctx, me, "delete_post", "post:"+post.ID, "title="+post.Title)
	return nil
}
