package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

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
	// The state we read here is the optimistic-lock token for the write below:
	// if anything else moves the post while this request is in flight, the save
	// fails instead of silently rolling that move back.
	originalStatus := post.Status
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

	// R5-05: a reviewer is not an author.
	//
	// An admin could rewrite any plain user's title and body, and for an
	// already-published post the change went live immediately — no review, no
	// trace, and the article still carried the original author's name. That is
	// content impersonation reachable by anyone who obtains an admin session
	// (a weak password, an XSS on the console), and it is strictly more than
	// the review flow needs: a reviewer who wants changes uses `reject` with a
	// note, and the author makes them.
	//
	// A plain admin keeps every editorial action — unpublish to private,
	// recategorise, retag, and approve/reject through ReviewPost — but cannot
	// alter the words. Root keeps full access, because root is the recovery
	// path for every other permission on the site.
	//
	// `slug` is in the protected set: it is the permanent link, so changing it
	// silently breaks every existing reference and lets a reviewer re-point an
	// author's URL to different content. Leaving it out of the first version of
	// this check was an omission, not a decision — a URL is as much the
	// author's as the text is.
	if post.AuthorID != me.ID && me.Role == models.RoleAdmin {
		if in.Title != "" || in.Body != "" || in.Cover != "" || in.Slug != "" {
			return models.Post{}, newErr(http.StatusForbidden,
				"only the author or the root can change a post's title, body, cover or slug; "+
					"send it back to the author with a review note instead")
		}
	}

	// Spec 31: keep the pre-edit state so the user can roll back. Snapshotting
	// before the assignment (not after) is what makes "restore" mean
	// "the state before this save".
	//
	// The content ceilings are checked before the snapshot, so an over-long
	// write is refused without leaving a revision behind: otherwise every
	// rejected attempt would still consume one of the 20 revision slots.
	if in.Title != "" || in.Body != "" {
		nextTitle, nextBody := post.Title, post.Body
		if in.Title != "" {
			nextTitle = in.Title
		}
		if in.Body != "" {
			nextBody = in.Body
		}
		if err := ValidatePostContent(nextTitle, nextBody); err != nil {
			return models.Post{}, err
		}
	}
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
		// See CreatePost: pinning is a site-wide editorial decision. Only a
		// *transition* to pinned is privileged — re-sending the current value
		// (a legacy user post that is already pinned) and unpinning stay open
		// so an ordinary edit never fails on a field the UI did not touch.
		if *in.Pinned && !post.Pinned && !IsAdminOrRoot(me) {
			return models.Post{}, newErr(http.StatusForbidden, "only an admin can pin a post")
		}
		post.Pinned = *in.Pinned
	}
	// Cover is an optional field. A non-empty value replaces the stored
	// cover; an empty string is ignored so partial edits (title-only) don't
	// wipe an existing cover.
	if in.Cover != "" {
		if err := ValidateCoverURL(in.Cover); err != nil {
			return models.Post{}, err
		}
		post.Cover = in.Cover
	}

	// Persist the post fields and (when supplied) the tag associations in a
	// single transaction. Previously the tag Clear/Append used "_ =", so a
	// failure there left a "saved but tagless" (or partially-tagged) post with
	// a 200 response. Now any association error aborts and rolls back the whole
	// update so the post and its tags stay consistent.
	//
	// R4-11: the save no longer writes `status`. A whole-row Save echoed back
	// the status we read at the top of this function, so an author saving a
	// typo at the same moment a reviewer approved the post silently undid the
	// approval (pending written over published) — and neither party saw an
	// error. The state change is now a separate, conditional transition from
	// the state we actually read; if someone moved it first, this save is
	// refused with 409 instead of rolling their decision back.
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if post.Status != originalStatus {
			won, terr := transitionPostTx(tx, post.ID, []string{originalStatus},
				map[string]any{"status": post.Status, "review_note": post.ReviewNote})
			if terr != nil {
				return terr
			}
			if !won {
				return errStateChanged
			}
		}
		// Content only. Everything here is owned by the author; nothing an
		// admin or the review flow sets is included.
		//
		// `slug` is in this list because the permanent link is part of the
		// author's content: UpdatePost already resolved it above (with its own
		// uniqueness probe), and leaving it out of the column list silently
		// made `in.Slug` a no-op.
		if err := tx.Model(&models.Post{}).Where("id = ?", post.ID).Updates(map[string]any{
			"title":       post.Title,
			"body":        post.Body,
			"slug":        post.Slug,
			"category_id": post.CategoryID,
			"cover":       post.Cover,
			"pinned":      post.Pinned,
		}).Error; err != nil {
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
		// Spec 36: re-sync the image ledger against the body that was just
		// saved. It is a diff, so a title-only edit writes nothing.
		if err := syncPostUploadRefsTx(tx, post.ID, post.AuthorID, post.Body); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, errStateChanged) {
		return models.Post{}, newErr(http.StatusConflict,
			"this post was reviewed or moderated while you were editing; reload and try again")
	}
	// A refusal raised inside the transaction (too many tags, a rejected cover)
	// carries its own status; mapping it to 500 would tell the client the
	// server broke when in fact the request was wrong.
	if se := AsServiceError(err); se != nil {
		return models.Post{}, se
	}
	if err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to update post")
	}

	// Re-load with author, category, tags so the response carries full objects.
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", post.ID).First(&post).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	// An edit performed by someone other than the author is the interesting
	// case for an audit trail: "root changed the category of alice's post" is
	// what an operator searches for after a dispute, and it is invisible if the
	// row only records the action. Author edits stay in the compact form.
	detail := auditDetail("title="+post.Title, "status="+post.Status)
	if post.AuthorID != me.ID {
		detail = auditDetail("title="+post.Title, "status="+post.Status,
			"by="+me.Username, "author_id="+post.AuthorID)
	}
	s.audit(ctx, me, "update_post", "post:"+post.ID, detail)
	post.SanitizeAuthor()
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
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// R4-11: the soft delete is the claim, so it carries the state
		// preconditions. Reading "not banned" above is only a fast fail — a
		// concurrent ban between the read and the write would otherwise move a
		// just-vaulted post into the trash, where it vanishes from every list
		// while its row stays banned forever.
		res := tx.Model(&models.Post{}).
			Where("id = ? AND deleted_at IS NULL AND status <> ?", id, models.PostBanned).
			Update("deleted_at", time.Now())
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errStateChanged
		}
		// The article is gone from every list; its edges in the image ledger
		// must not keep claiming the files are still in use.
		return releasePostUploadRefsTx(tx, post.ID)
	})
	if errors.Is(err, errStateChanged) {
		return newErr(http.StatusConflict, "post changed state; reload and try again")
	}
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete post")
	}
	s.audit(ctx, me, "delete_post", "post:"+post.ID, auditDetail("title="+post.Title))
	return nil
}
