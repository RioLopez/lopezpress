package services

import (
	"context"
	"net/http"
	"strconv"

	"lopezpress/internal/models"
)

// ListTagsWithCount returns all tags with the number of published posts
// carrying them.
//
// Same rule and same reason as ListCategoriesWithCount: the badge has to match
// what the link opens. The filter lives in the join so a tag whose only posts
// are trashed, banned or unpublished still returns a row with count 0, and we
// COUNT the (nullable) posts.id so those posts — which yield a NULL posts.id
// on the filtered join — are not tallied via their lingering post_tags rows.
func (s *Service) ListTagsWithCount(ctx context.Context) ([]TagWithCount, error) {
	var results []TagWithCount
	err := s.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, COUNT(posts.id) as count").
		Joins("LEFT JOIN post_tags ON post_tags.tag_id = tags.id").
		Joins("LEFT JOIN posts ON posts.id = post_tags.post_id AND posts.deleted_at IS NULL AND posts.status = ?", models.PostPublished).
		Group("tags.id").
		Order("tags.name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load tags with counts")
	}
	if results == nil {
		results = []TagWithCount{}
	}
	return results, nil
}

// DeleteTag removes a tag and all its associations.
func (s *Service) DeleteTag(ctx context.Context, id int64) error {
	var tag models.Tag
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&tag).Error; err != nil {
		return newErr(http.StatusNotFound, "tag not found")
	}
	// Remove join records.
	if err := s.db.WithContext(ctx).Where("tag_id = ?", id).Delete(&models.PostTag{}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to remove tag associations")
	}
	if err := s.db.WithContext(ctx).Delete(&tag).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete tag")
	}
	s.audit(ctx, models.User{}, "delete_tag", "tag:"+strconv.FormatInt(tag.ID, 10), "name="+tag.Name)
	return nil
}

// UpdateTag renames a tag. Returns 404 when not found and 409 on a
// duplicate name.
//
// Only tags.name changes: the post_tags join rows are keyed by tag id, so
// every association survives the rename. This is the whole point — before
// this endpoint the only way to fix a typo'd tag was delete + re-create, which
// silently dropped the tag from every post that carried it.
func (s *Service) UpdateTag(ctx context.Context, id int64, in UpdateTagInput) (models.Tag, error) {
	name := normalizeTagName(in.Name)
	if name == "" {
		return models.Tag{}, newErr(http.StatusBadRequest, "tag name is required")
	}
	if err := ValidateTaxonomyName(name, MaxTagNameLen); err != nil {
		// Rune-based, like every other name check. The previous byte-based test
		// was accidentally stricter for non-ASCII names: a 30-character Chinese
		// tag is 90 bytes and was refused here while being accepted by
		// CreatePost, so the same name behaved differently depending on which
		// endpoint touched it.
		return models.Tag{}, err
	}
	var tag models.Tag
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&tag).Error; err != nil {
		return models.Tag{}, newErr(http.StatusNotFound, "tag not found")
	}
	// Duplicate check excludes self so a pure case change ("go" -> "Go") is
	// allowed, while a rename that would collide with a *different* tag
	// differing only in case is still rejected.
	var dup models.Tag
	if s.db.WithContext(ctx).Where("name COLLATE NOCASE = ? AND id != ?", name, id).First(&dup).RowsAffected == 1 {
		return models.Tag{}, newErr(http.StatusConflict, "a tag with this name already exists")
	}
	tag.Name = name
	if err := s.db.WithContext(ctx).Save(&tag).Error; err != nil {
		return models.Tag{}, newErr(http.StatusInternalServerError, "failed to update tag")
	}
	s.audit(ctx, models.User{}, "update_tag", "tag:"+strconv.FormatInt(tag.ID, 10), "name="+tag.Name)
	return tag, nil
}
