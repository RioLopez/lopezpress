package services

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"lopezpress/internal/models"
)

// ListCategories returns all categories, ordered by name.
func (s *Service) ListCategories(ctx context.Context) ([]models.Category, error) {
	var cats []models.Category
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&cats).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load categories")
	}
	if cats == nil {
		cats = []models.Category{}
	}
	return cats, nil
}

// ListTags returns all distinct tag names, ordered by name.
func (s *Service) ListTags(ctx context.Context) ([]models.Tag, error) {
	var tags []models.Tag
	if err := s.db.WithContext(ctx).Order("name ASC").Find(&tags).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load tags")
	}
	if tags == nil {
		tags = []models.Tag{}
	}
	return tags, nil
}

// GetRecentPosts returns the 5 most recently created published posts,
// with category and tags preloaded.
func (s *Service) GetRecentPosts(ctx context.Context) ([]models.Post, error) {
	var posts []models.Post
	err := s.db.WithContext(ctx).
		Where("status = ?", "published").
		Preload("Category").
		Preload("Tags").
		Order("created_at DESC").
		Limit(5).
		Find(&posts).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load recent posts")
	}
	if posts == nil {
		posts = []models.Post{}
	}
	return posts, nil
}

// --- category CRUD ---

// CreateCategory creates a new category. Returns 409 on duplicate name.
// An optional parent_id makes it a child (spec 27: two levels only).
func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (models.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.Category{}, newErr(http.StatusBadRequest, "category name is required")
	}
	var existing models.Category
	if s.db.WithContext(ctx).Where("name = ?", name).First(&existing).RowsAffected == 1 {
		return models.Category{}, newErr(http.StatusConflict, "category already exists")
	}
	if in.ParentID != nil {
		if err := s.validateCategoryParent(ctx, 0, *in.ParentID); err != nil {
			return models.Category{}, err
		}
	}
	cat := models.Category{Name: name, ParentID: in.ParentID}
	if err := s.db.WithContext(ctx).Create(&cat).Error; err != nil {
		return models.Category{}, newErr(http.StatusInternalServerError, "failed to create category")
	}
	s.audit(ctx, models.User{}, "create_category", "category:"+strconv.FormatInt(cat.ID, 10), "name="+cat.Name)
	return cat, nil
}

// UpdateCategory renames a category and/or moves it under another parent.
// Returns 404 when not found, 409 on duplicate, 400 on an illegal parent.
func (s *Service) UpdateCategory(ctx context.Context, id int64, in UpdateCategoryInput) (models.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.Category{}, newErr(http.StatusBadRequest, "category name is required")
	}
	var cat models.Category
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&cat).Error; err != nil {
		return models.Category{}, newErr(http.StatusNotFound, "category not found")
	}
	// Check for duplicate name (excluding self).
	var dup models.Category
	if s.db.WithContext(ctx).Where("name = ? AND id != ?", name, id).First(&dup).RowsAffected == 1 {
		return models.Category{}, newErr(http.StatusConflict, "a category with this name already exists")
	}
	if in.ParentID != nil {
		if err := s.validateCategoryParent(ctx, id, *in.ParentID); err != nil {
			return models.Category{}, err
		}
	}
	cat.Name = name
	// ParentID is *int64: nil in the payload means "leave as is", which is why
	// the caller must send an explicit 0 to lift a child back to top level.
	if in.ParentID != nil {
		if *in.ParentID == 0 {
			cat.ParentID = nil
		} else {
			cat.ParentID = in.ParentID
		}
	}
	if err := s.db.WithContext(ctx).Save(&cat).Error; err != nil {
		return models.Category{}, newErr(http.StatusInternalServerError, "failed to update category")
	}
	s.audit(ctx, models.User{}, "update_category", "category:"+strconv.FormatInt(cat.ID, 10), "name="+cat.Name)
	return cat, nil
}

// validateCategoryParent enforces the two-level rule (spec 27):
//   - the parent must exist;
//   - a category cannot be its own parent;
//   - the parent must not itself have a parent (no grandchildren);
//   - walking up from the parent must never reach self (cycle guard).
func (s *Service) validateCategoryParent(ctx context.Context, selfID, parentID int64) error {
	if parentID == 0 {
		return nil
	}
	if parentID == selfID {
		return newErr(http.StatusBadRequest, "category cannot be its own parent")
	}
	var parent models.Category
	if err := s.db.WithContext(ctx).Where("id = ?", parentID).First(&parent).Error; err != nil {
		return newErr(http.StatusBadRequest, "parent category not found")
	}
	if parent.ParentID != nil {
		return newErr(http.StatusBadRequest, "only two levels are supported")
	}
	// Defensive cycle walk: even though the two-level rule already prevents a
	// cycle, a corrupted row (hand-edited DB, old backup) must not hang here.
	seen := map[int64]bool{selfID: true}
	cursor := parent
	for cursor.ParentID != nil {
		if seen[cursor.ID] {
			return newErr(http.StatusBadRequest, "category parent cycle detected")
		}
		seen[cursor.ID] = true
		var next models.Category
		if err := s.db.WithContext(ctx).Where("id = ?", *cursor.ParentID).First(&next).Error; err != nil {
			break
		}
		cursor = next
	}
	return nil
}

// DeleteCategory removes a category and detaches it from all posts. Children
// are lifted to the deleted category's own parent (deleting a parent must not
// cascade-delete content structure).
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	var cat models.Category
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&cat).Error; err != nil {
		return newErr(http.StatusNotFound, "category not found")
	}
	if err := s.db.WithContext(ctx).Model(&models.Category{}).Where("parent_id = ?", id).
		Update("parent_id", cat.ParentID).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to move child categories")
	}
	// Null out category_id on all posts that reference this category.
	if err := s.db.WithContext(ctx).Model(&models.Post{}).Where("category_id = ?", id).
		Update("category_id", nil).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to detach posts from category")
	}
	if err := s.db.WithContext(ctx).Delete(&cat).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete category")
	}
	s.audit(ctx, models.User{}, "delete_category", "category:"+strconv.FormatInt(cat.ID, 10), "name="+cat.Name)
	return nil
}

// categoryWithDescendants returns id plus every direct child id. Used so that
// filtering the public list by a PARENT category also returns its children's
// posts (spec 27 AC-2.5) — otherwise a parent node would look empty even
// while all its content sits one level down.
// categoryFilterIDs resolves a ?category= reference to the id set a list
// query should match. The reference is either a name (the readable form the
// frontend links with) or a numeric id (the original form, kept so old links
// and bookmarks keep working). The set includes the category's children:
// selecting a parent must also return the posts filed under it.
//
// ok=false means no such category exists. The caller has to treat that as an
// empty result rather than as "no filter" — otherwise a bogus URL silently
// lists the whole site.
func (s *Service) categoryFilterIDs(ctx context.Context, ref string) ([]int64, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, false
	}
	var id int64
	if n, err := strconv.ParseInt(ref, 10, 64); err == nil {
		id = n
		// A numeric reference still has to point at a real row, otherwise
		// ?category=999 would quietly list everything.
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.Category{}).
			Where("id = ?", id).Count(&count).Error; err != nil || count == 0 {
			return nil, false
		}
	} else {
		var cat models.Category
		if err := s.db.WithContext(ctx).Where("name = ?", ref).First(&cat).Error; err != nil {
			return nil, false
		}
		id = cat.ID
	}
	if id == 0 {
		return nil, false
	}
	return s.categoryWithDescendants(ctx, id), true
}

func (s *Service) categoryWithDescendants(ctx context.Context, id int64) []int64 {
	ids := []int64{id}
	var children []models.Category
	if err := s.db.WithContext(ctx).Where("parent_id = ?", id).Find(&children).Error; err == nil {
		for _, c := range children {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// ListCategoriesWithCount returns all categories with post counts.
//
// The count only includes live, publicly-managed posts: soft-deleted posts
// (the trash) are excluded via the join condition, and banned posts (the
// root-only vault) are excluded so the admin counts match what authors can
// actually see. The filter lives in the JOIN ON clause (not WHERE) so that
// categories with zero matching posts still return a row with count 0.
func (s *Service) ListCategoriesWithCount(ctx context.Context) ([]CategoryWithCount, error) {
	var results []CategoryWithCount
	err := s.db.WithContext(ctx).
		Table("categories").
		Select("categories.id, categories.name, categories.parent_id, COUNT(posts.id) as count").
		Joins("LEFT JOIN posts ON posts.category_id = categories.id AND posts.deleted_at IS NULL AND posts.status <> ?", models.PostBanned).
		Group("categories.id").
		Order("categories.name ASC").
		Scan(&results).Error
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load categories with counts")
	}
	if results == nil {
		results = []CategoryWithCount{}
	}
	return results, nil
}

// --- tag management ---

// ListTagsWithCount returns all tags with post counts.
//
// Mirrors ListCategoriesWithCount: the count excludes soft-deleted posts and
// banned posts. The join to posts carries the filter so tag rows whose only
// posts are trashed/banned still return a row with count 0, and we COUNT the
// (nullable) posts.id so banned posts — which yield a NULL posts.id on the
// filtered join — are not tallied via their lingering post_tags rows.
func (s *Service) ListTagsWithCount(ctx context.Context) ([]TagWithCount, error) {
	var results []TagWithCount
	err := s.db.WithContext(ctx).
		Table("tags").
		Select("tags.id, tags.name, COUNT(posts.id) as count").
		Joins("LEFT JOIN post_tags ON post_tags.tag_id = tags.id").
		Joins("LEFT JOIN posts ON posts.id = post_tags.post_id AND posts.deleted_at IS NULL AND posts.status <> ?", models.PostBanned).
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

// UpdateTag renames a tag (spec 25). Returns 404 when not found and 409 on a
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
	if len(name) > 64 {
		// Mirrors the column width (size:64); reject here so the client gets
		// a 400 instead of a driver-level 500.
		return models.Tag{}, newErr(http.StatusBadRequest, "tag name is too long (max 64)")
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
