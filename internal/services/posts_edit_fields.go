package services

import (
	"context"
	"net/http"
	"strings"

	"lopezpress/internal/models"
)

// applyPostFields copies the fields this request actually mentions onto post.
//
// It exists because "the client did not send this" and "the client asked for
// this to be empty" are different requests, and the rule is not uniform: an
// empty cover means "remove it", an empty slug means "keep the link", an empty
// category means "no category". One function holding all three rules is what
// keeps those decisions from being re-guessed at each call site.
//
// Title and body are not here: they are plain strings in the input, and
// ValidatePostContent already refuses an empty one, so an absent field and an
// empty field are the same request (and both a 400).
func (s *Service) applyPostFields(ctx context.Context, post *models.Post, me models.User, in UpdatePostInput) error {
	// An explicit slug overrides the permanent link; a blank one leaves it
	// alone, because links must stay stable across renames and a post always
	// has one.
	if strings.TrimSpace(in.Slug) != "" {
		slug, err := s.ensureUniqueSlug(ctx, Slugify(in.Slug), post.ID)
		if err != nil {
			return err
		}
		post.Slug = slug
	}
	// nil = unchanged, 0 = no category, >0 = that category.
	if in.CategoryID != nil {
		if *in.CategoryID > 0 {
			post.CategoryID = in.CategoryID
		} else {
			post.CategoryID = nil
		}
	}
	// Pinning is a site-wide editorial decision, so only a *transition* to
	// pinned is privileged: re-sending the current value (a legacy user post
	// that is already pinned) and unpinning stay open so an ordinary edit never
	// fails on a field the UI did not touch.
	if in.Pinned != nil {
		if *in.Pinned && !post.Pinned && !IsAdminOrRoot(me) {
			return newErr(http.StatusForbidden, "only an admin can pin a post")
		}
		post.Pinned = *in.Pinned
	}
	// nil = unchanged, "" = remove the cover. The empty case is validated away
	// (an empty URL is not a URL) rather than passed to ValidateCoverURL.
	if in.Cover != nil {
		if *in.Cover != "" {
			if err := ValidateCoverURL(*in.Cover); err != nil {
				return err
			}
		}
		post.Cover = *in.Cover
	}
	return nil
}
