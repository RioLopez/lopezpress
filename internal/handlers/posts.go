package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"lopezpress/internal/services"
)

// GetPostPublic returns a published post without requiring authentication.
// Only posts with status "published" are accessible. Increments view_count.
// GET /api/posts/{id} (public, no auth)
func GetPostPublic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	post, err := svc.GetPostPublic(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, post)
}

// GetPostPrevNext returns the previous and next published post.
// GET /api/posts/{id}/nav (auth)
func GetPostPrevNext(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	nav, err := svc.GetPostPrevNext(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, nav)
}

// ListArchive returns all published posts for the /archive page.
// GET /api/posts/archive (auth). Body is stripped server-side.
func ListArchive(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)

	posts, err := svc.ListArchive(r.Context(), me)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, posts)
}

// ListPosts returns posts, newest first.
// GET /api/posts (auth)
//
// Optional query params:
//   - status:   filter by post status
//   - q:        keyword search across title / body / author username
//   - page:     1-based page number (default 1)
//   - size:     page size, 1..100 (default 20)
//   - category: category id filter
//   - tag:      tag name filter
func ListPosts(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)

	query := r.URL.Query()
	status := query.Get("status")
	search := query.Get("q")
	pageStr := query.Get("page")
	sizeStr := query.Get("size")
	catStr := query.Get("category")
	tag := query.Get("tag")
	sort := query.Get("sort")
	order := query.Get("order")

	// Parse pagination params. An explicitly-supplied but non-numeric value
	// (e.g. ?page=abc) is a client error: reject it with 400 instead of
	// silently degrading to the legacy bare-array branch (P2-7).
	page, pageErr := strconv.Atoi(pageStr)
	size, sizeErr := strconv.Atoi(sizeStr)
	cat, catErr := strconv.Atoi(catStr)
	if (pageStr != "" && pageErr != nil) || (sizeStr != "" && sizeErr != nil) || (catStr != "" && catErr != nil) {
		writeError(w, http.StatusBadRequest, "page, size and category must be integers")
		return
	}

	// Legacy bare-array form: no pagination/search requested.
	if page == 0 && size == 0 && search == "" && cat == 0 && tag == "" {
		posts, err := svc.ListPostsPage(r.Context(), me, services.ListPostsQuery{
			Status: status,
			SortBy: sort,
			Order:  order,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, posts.Items)
		return
	}

	res, err := svc.ListPostsPage(r.Context(), me, services.ListPostsQuery{
		Status:   status,
		Search:   search,
		Page:     page,
		PageSize: size,
		Category: int64(cat),
		Tag:      tag,
		SortBy:   sort,
		Order:    order,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// GetPost returns a single post by id.
// GET /api/posts/{id} (auth)
func GetPost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	me := userFromContext(r)

	post, err := svc.GetPost(r.Context(), me, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, post)
}

// CreatePost creates a post owned by the authenticated user.
// POST /api/posts (auth)
func CreatePost(w http.ResponseWriter, r *http.Request) {
	var in services.CreatePostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}

	me := userFromContext(r)
	post, err := svc.CreatePost(r.Context(), me, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, post)
}

// UpdatePost edits a post. Only the author or an admin may modify.
// PUT /api/posts/{id} (auth)
func UpdatePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var in services.UpdatePostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}

	me := userFromContext(r)
	post, err := svc.UpdatePost(r.Context(), me, id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, post)
}

// DeletePost removes a post. Only the author or an admin may delete.
// DELETE /api/posts/{id} (auth)
func DeletePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	me := userFromContext(r)
	if err := svc.DeletePost(r.Context(), me, id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
