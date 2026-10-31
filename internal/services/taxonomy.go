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
	// This endpoint is authOptional, so an anonymous visitor can read it. The
	// reviewer's account id and the approval timestamp are moderation metadata
	// that no client here renders; every other public read path strips them, and
	// this one was the gap. SanitizeAuthor is a no-op today (no Author preload)
	// but stays for the day a card needs the byline.
	models.SanitizePostAuthors(posts)
	models.SanitizePostReviews(posts)
	return posts, nil
}

// --- category CRUD ---

// CreateCategory creates a new category. Returns 409 on duplicate name.
// An optional parent_id makes it a child (two levels only).
func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (models.Category, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return models.Category{}, newErr(http.StatusBadRequest, "category name is required")
	}
	// A category name is served by the ANONYMOUS /api/categories endpoint, so
	// its length is a length every visitor downloads. Only the importer and the
	// tag path checked this; the CRUD endpoint — the one an admin actually uses
	// — did not (R4-10's class, found again on the second sweep).
	if err := ValidateTaxonomyName(name, MaxCategoryNameLen); err != nil {
		return models.Category{}, err
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
	if err := ValidateTaxonomyName(name, MaxCategoryNameLen); err != nil {
		return models.Category{}, err
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

// validateCategoryParent enforces the two-level rule:
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
// posts — otherwise a parent node would look empty even
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
// ListCategoriesWithCount returns every category with the number of posts a
// reader would actually find behind it.
//
// The count is published-only. It used to exclude nothing but banned and
// soft-deleted, which made it a count of "posts that exist" — a number nobody
// browsing the site can verify. Once private posts became a real state an
// author can choose, the gap turned into a visible bug: a category badge
// saying 5 that opens onto 2 articles, because the other 3 are somebody's
// drafts and submissions. "How many can I read here" is the only question this
// number answers for a reader, so that is what it counts.
//
// Reviewers who want the moderation totals have the pending queue and the
// dashboard; neither reads this.
func (s *Service) ListCategoriesWithCount(ctx context.Context) ([]CategoryWithCount, error) {
	var results []CategoryWithCount
	err := s.db.WithContext(ctx).
		Table("categories").
		Select("categories.id, categories.name, categories.parent_id, COUNT(posts.id) as count").
		Joins("LEFT JOIN posts ON posts.category_id = categories.id AND posts.deleted_at IS NULL AND posts.status = ?", models.PostPublished).
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
