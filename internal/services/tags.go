package services

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- categories / tags ---

// findOrCreateTags resolves a list of tag names to Tag records, creating any
// that do not yet exist. Runs on the shared connection (used outside a
// transaction) and delegates to findOrCreateTagsTx.
func (s *Service) findOrCreateTags(ctx context.Context, names []string) []models.Tag {
	tags, _ := s.findOrCreateTagsTx(ctx, s.db, names)
	return tags
}

// findOrCreateTagsTx is the transaction-aware variant of findOrCreateTags: it
// runs every lookup/create on the supplied *gorm.DB (which may be a
// transaction handle) so tag resolution participates in the caller's
// transaction and rolls back together with the post write. It returns an error
// if a tag cannot be resolved or created, and skips empty names. A creation
// that loses a concurrent unique-race is recovered by re-querying, so we never
// append a zero-value (ID=0) tag.
//
// Lookups are case-insensitive on purpose: authors type "go", "Go" and "GO"
// meaning the same thing, and a tag list polluted with three spellings of one
// concept makes filtering useless. The first spelling that was ever created
// wins and later ones reuse it (SQLite's LOWER is ASCII-only, which is exactly
// the range where case exists). Interior whitespace is collapsed so "web  dev"
// and "web dev" are the same tag too.
func (s *Service) findOrCreateTagsTx(ctx context.Context, db *gorm.DB, names []string) ([]models.Tag, error) {
	var out []models.Tag
	for _, raw := range names {
		name := normalizeTagName(raw)
		if name == "" {
			continue
		}
		lookup := func() (models.Tag, bool) {
			var tag models.Tag
			if err := db.WithContext(ctx).Where("LOWER(name) = ?", strings.ToLower(name)).First(&tag).Error; err != nil {
				return models.Tag{}, false
			}
			return tag, true
		}
		tag, found := lookup()
		if !found {
			created := models.Tag{Name: name}
			if err := db.WithContext(ctx).Create(&created).Error; err != nil {
				// Likely a concurrent insert of the same name: re-query so we
				// still attach the existing tag instead of dropping it.
				var ok bool
				if tag, ok = lookup(); !ok {
					return nil, newErr(http.StatusInternalServerError, "failed to resolve tag")
				}
			} else {
				tag = created
			}
		}
		out = append(out, tag)
	}
	return out, nil
}

// normalizeTagName trims a tag name and collapses runs of whitespace to a
// single space. Empty results are dropped by the caller.
func normalizeTagName(raw string) string {
	return strings.Join(strings.Fields(raw), " ")
}

// linkPostTagsTx writes the post←tag associations directly into the post_tags
// join table inside the supplied transaction. We deliberately avoid gorm's
// Association helper here: it re-opens a Session internally and panics when
// called from within a Transaction callback. Writing the join rows by hand
// keeps the whole create/update atomic and lets association errors roll back
// the post write. Duplicate (post_id, tag_id) pairs are de-duplicated via
// FirstOrCreate on the composite primary key.
func linkPostTagsTx(tx *gorm.DB, postID string, tags []models.Tag) error {
	for _, tag := range tags {
		if tag.ID == 0 {
			continue
		}
		pt := models.PostTag{PostID: postID, TagID: tag.ID}
		if err := tx.Where("post_id = ? AND tag_id = ?", postID, tag.ID).FirstOrCreate(&pt).Error; err != nil {
			return err
		}
	}
	return nil
}
