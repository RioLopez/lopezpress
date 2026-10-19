package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"echo-content-api/internal/services"
)

// ListCategories returns all categories.
// GET /api/categories (auth)
func ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := svc.ListCategories(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

// ListCategoriesWithCount returns all categories with post counts.
// GET /api/categories/with-count (auth, admin)
func ListCategoriesWithCount(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	cats, err := svc.ListCategoriesWithCount(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

// CreateCategory creates a new category.
// POST /api/categories (auth, admin)
func CreateCategory(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	var in services.CreateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	cat, err := svc.CreateCategory(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

// UpdateCategory renames a category.
// PUT /api/categories/{id} (auth, admin)
func UpdateCategory(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var in services.UpdateCategoryInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	cat, err := svc.UpdateCategory(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

// DeleteCategory removes a category and detaches posts.
// DELETE /api/categories/{id} (auth, admin)
func DeleteCategory(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := svc.DeleteCategory(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTags returns all tags.
// GET /api/tags (auth)
func ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := svc.ListTags(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// ListTagsWithCount returns all tags with post counts.
// GET /api/tags/with-count (auth, admin)
func ListTagsWithCount(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	tags, err := svc.ListTagsWithCount(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// DeleteTag removes a tag and all its associations.
// DELETE /api/tags/{id} (auth, admin)
func DeleteTag(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := svc.DeleteTag(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RecentPosts returns the 5 most recently published posts.
// GET /api/posts/recent (auth)
func RecentPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := svc.GetRecentPosts(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}
