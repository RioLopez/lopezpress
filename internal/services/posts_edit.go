package services

import (
	"context"
	"errors"
	"net/http"
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
	// editorial access so corrections remain possible.
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
		if in.Title != "" || in.Body != "" || in.Cover != nil || in.Slug != "" {
			return models.Post{}, newErr(http.StatusForbidden,
				"only the author or the root can change a post's title, body, cover or slug; "+
					"send it back to the author with a review note instead")
		}
	}

	// Keep the pre-edit state so the user can roll back. Snapshotting
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
	// Decide the resulting state in memory first, so the snapshot below knows
	// whether this save changes anything at all.
	//
	// Before this, the snapshot was unconditional: a `PUT {}` with no fields
	// left a revision behind. Once admins can read other people's private
	// posts, that becomes reachable on content they do not own — open a post,
	// save nothing, and one of the author's 20 revision slots is gone, stamped
	// with the operator's id. Twenty such saves erase the author's real
	// history. A revision is a record of a change, so no change means no row.
	next := post
	if in.Title != "" {
		next.Title = in.Title
	}
	if in.Body != "" {
		next.Body = in.Body
	}
	// "Content changed" is the test the review rule cares about: it is what a
	// reader would see differently. Cover counts, because it is the face of
	// the article in every listing; a swapped image is a changed article.
	// Slug, category, tags and pinning do not — those are how the post is
	// filed, not what it says, and demoting an article because an editor
	// fixed its category would make the console unusable.
	contentChanged := next.Title != post.Title || next.Body != post.Body ||
		(in.Cover != nil && *in.Cover != post.Cover)
	// Status is decided in one place; see post_status.go for the rule, why the
	// previous inline switch made "私密" mean "pending", and why a live post
	// whose words changed has to go back through review.
	resolveStatusChange(&next, me, in.Status, contentChanged)
	if next.Title != post.Title || next.Body != post.Body || next.Status != originalStatus {
		s.snapshotRevision(ctx, post, me.ID)
	}
	post = next
	// An explicit slug overrides the permanent link; otherwise the
	// existing one is never touched (links must stay stable across renames).
	//
	// See applyPostFields for what "absent" means for each pointer field: the
	// three rules (cover, category, slug) differ and all live in one place.
	if err := s.applyPostFields(ctx, &post, me, in); err != nil {
		return models.Post{}, err
	}

	// Persist fields and (when supplied) tags in one transaction: the tag
	// Clear/Append used to swallow its error, leaving a "saved but tagless" post
	// with a 200 response. Any association error now rolls the whole update back.
	//
	// This never writes `status`. A whole-row Save echoed back the status read at
	// the top of the function, so saving a typo at the same moment a reviewer
	// approved the post silently undid the approval — neither party saw an error.
	// The state change is a separate conditional transition from that same state;
	// if someone moved the row first, this save answers 409 instead.
	// A nil *int64 in an update map is written as NULL by the driver, but
	// saying so explicitly keeps that true through any later refactor that
	// starts skipping nil map entries — a cleared category is exactly what
	// such a refactor would break first.
	var category any = post.CategoryID
	if post.CategoryID == nil {
		category = gorm.Expr("NULL")
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Content first, guarded by the state read at the top of this function.
		// `slug` is here because the permanent link is the author's content too:
		// UpdatePost resolved it above with its own uniqueness probe, and
		// omitting it silently dropped it.
		//
		// `AND status = originalStatus` is the fix. The write used to carry only
		// the id, so a save racing an approval overwrote text nobody reviewed
		// while the approval stayed attached — and a root ban was bypassable the
		// same way. Zero rows means the row moved under us: answer 409.
		res := tx.Model(&models.Post{}).
			Where("id = ? AND status = ?", post.ID, originalStatus).
			Updates(map[string]any{
				"title":       post.Title,
				"body":        post.Body,
				"slug":        post.Slug,
				"category_id": category,
				"cover":       post.Cover,
				"pinned":      post.Pinned,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errStateChanged
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
		// Re-sync the image ledger against the body and cover that were just
		// saved. It is a diff, so a title-only edit writes nothing.
		if err := syncPostUploadRefsTx(tx, post.ID, post.AuthorID, post.Body, post.Cover); err != nil {
			return err
		}
		// Derived from the same originalStatus, so it cannot race the content
		// write above. Never echo back the status we read: that would undo a
		// reviewer's decision.
		if post.Status != originalStatus {
			updates := map[string]any{"status": post.Status, "review_note": post.ReviewNote}
			// Re-entering the queue invalidates the previous verdict: the
			// reviewer judged different text. reviewed_at is an explicit NULL
			// expression, not an untyped nil — a nil in the update map is
			// skipped by the driver, which left the old approval timestamp
			// behind while reviewed_by was already cleared.
			if post.Status == models.PostPending {
				updates["reviewed_by"] = ""
				updates["reviewed_at"] = gorm.Expr("NULL")
			}
			won, terr := transitionPostTx(tx, post.ID, []string{originalStatus}, updates)
			if terr != nil {
				return terr
			}
			if !won {
				return errStateChanged
			}
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

	// Re-load with author, category, tags. Into a fresh value, not back into
	// `post`: GORM scans into the struct it is given, and a column that just
	// became NULL kept its old contents on the reused variable, so the response
	// carried the approval timestamp the write had just cleared.
	var loaded models.Post
	if err := s.db.WithContext(ctx).Preload("Author").Preload("Category").Preload("Tags").Where("id = ?", post.ID).First(&loaded).Error; err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to load post")
	}
	post = loaded
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
	// The response is read by the author (and by reviewers), so it carries the
	// same outbound rule as every other post payload: the rejection note stays
	// (the author needs to know why), the reviewer's identity does not.
	post.SanitizeAuthor()
	post.SanitizeReview()
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
	// through the soft-delete-aware scope.
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
