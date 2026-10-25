package services

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- post creation internals -------------------------------------------------
//
// Split out of posts.go when that file crossed the 350-line gate. These three
// pieces are one mechanism: the retry loop that reacts to the unique index on
// posts.slug, the transaction body it re-runs, and the check that recognises
// the conflict.

// slugRetryLimit caps how many suffixed slugs CreatePost will try after the
// unique index rejects one. Eight is far past any realistic collision burst and
// keeps the worst case bounded.
const slugRetryLimit = 8

// createPostTx writes the post row plus its tag associations and image
// references inside the caller's transaction. Extracted from CreatePost so the
// slug-conflict retry can re-run the whole unit without duplicating it.
func createPostTx(ctx context.Context, tx *gorm.DB, s *Service, post models.Post, in CreatePostInput, out *models.Post) error {
	if err := tx.Create(&post).Error; err != nil {
		return err
	}
	if len(in.Tags) > 0 {
		tags, ferr := s.findOrCreateTagsTx(ctx, tx, in.Tags)
		if ferr != nil {
			return ferr
		}
		if err := linkPostTagsTx(tx, post.ID, tags); err != nil {
			return err
		}
	}
	// Spec 36: record which library images this body uses, in the same
	// transaction as the post itself. The ledger is what lets an admin see
	// "deleting this image breaks 3 articles" before acting.
	if err := syncPostUploadRefsTx(tx, post.ID, post.AuthorID, post.Body); err != nil {
		return err
	}
	*out = post
	return nil
}

// isSlugConflict reports whether an error is the UNIQUE index on posts.slug
// refusing a duplicate permanent link (R6-02). The glebarez driver surfaces it
// as a plain SQLite constraint error, so the test is on the message rather than
// on a sentinel type the driver does not expose.
func isSlugConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "idx_posts_slug_unique") ||
		strings.Contains(msg, "UNIQUE constraint failed: posts.slug")
}

// insertPostWithSlugRetry writes the post, retrying with the next slug suffix
// when the UNIQUE index on posts.slug rejects the one the probe picked.
//
// The retry exists because slugFor's COUNT is a probe, not a lock: concurrent
// creates of the same title both see "free" and both insert. Before migration
// 0014 nothing stopped the second one, and R6-02 measured four posts sharing a
// single permanent link out of twenty concurrent creates.
func (s *Service) insertPostWithSlugRetry(ctx context.Context, post models.Post, in CreatePostInput, slug string) (models.Post, error) {
	var created models.Post
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return createPostTx(ctx, tx, s, post, in, &created)
	})
	if err == nil {
		return created, nil
	}
	// A refusal raised INSIDE the transaction (too many tags, a rejected cover,
	// a category that no longer exists) is a *ServiceError carrying its own
	// status. Mapping everything to 500 told the client "server broke" when the
	// truth was "your request was wrong", which is both misleading and a
	// support cost. Pass it through unchanged.
	if se := AsServiceError(err); se != nil {
		return models.Post{}, se
	}
	if !isSlugConflict(err) {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to create post")
	}
	for attempt := 2; attempt <= slugRetryLimit; attempt++ {
		post.Slug = nextSlugCandidate(slug, attempt)
		created = models.Post{}
		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return createPostTx(ctx, tx, s, post, in, &created)
		})
		if err == nil {
			break
		}
		if se := AsServiceError(err); se != nil {
			return models.Post{}, se
		}
		if !isSlugConflict(err) {
			break
		}
	}
	if err != nil {
		return models.Post{}, newErr(http.StatusInternalServerError, "failed to create post")
	}
	return created, nil
}
