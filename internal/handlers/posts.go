package handlers

import (
	"encoding/json"
	"net/http"

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

// ReportView records one de-duplicated read of a post (spec 23).
//
// POST /api/posts/{id}/view (optional auth)
//
// Counters no longer move on the read endpoints (GET /api/posts/{id} and
// /public are side-effect free), so the frontend calls this once per article
// view. Only published posts count; everything else answers 404 to avoid
// leaking the existence of restricted content.
func ReportView(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	me := userFromContext(r)

	res, err := svc.ReportView(r.Context(), me, id, services.ViewMeta{
		IP: clientIP(r),
		UA: r.UserAgent(),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// GetPostBySlug resolves a permanent link (spec 29).
// GET /api/permalinks/{slug} (optional auth)
//
// Visibility is decided by canSeePost inside the service, so an unpublished
// post's slug answers the same 404 as an unknown one.
func GetPostBySlug(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	post, err := svc.GetPostBySlug(r.Context(), me, r.PathValue("slug"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

// ListRevisions returns the edit history of one post (spec 31).
// GET /api/posts/{id}/revisions (auth)
func ListRevisions(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	revs, err := svc.ListRevisions(r.Context(), me, r.PathValue("id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revs)
}

// RestoreRevision rolls a post's title/body back to one saved version.
// POST /api/posts/{id}/revisions/{revID}/restore (auth)
func RestoreRevision(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	id := r.PathValue("id")
	revID, err := parseID(r, "revID", "revision id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	post, rerr := svc.RestoreRevision(r.Context(), me, id, revID)
	if rerr != nil {
		writeServiceError(w, rerr)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

// GetRelatedPosts returns posts sharing a category or tag (spec 35).
// GET /api/posts/{id}/related (optional auth)
//
// Optional auth, not no-auth: the handler itself never needs the user, but
// keeping it on the optional surface means a locked site (public_read=false)
// is enforced by the same middleware as every other read route.
func GetRelatedPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := svc.RelatedPosts(r.Context(), r.PathValue("id"), 5)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
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

// ListArchivePaged is the paginated archive (spec 28).
// GET /api/posts/archive/paged?page=&size= (optional auth)
//
// The old GET /api/posts/archive keeps returning the full array for backward
// compatibility; the UI now uses this one so a 2000-post archive no longer
// ships in a single response.
func ListArchivePaged(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	nums, err := parseInts(query, "page and size", "page", "size")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := svc.ListArchivePage(r.Context(), nums[0], nums[1])
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
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
	tag := query.Get("tag")
	sort := query.Get("sort")
	order := query.Get("order")

	// Parse pagination params. An explicitly-supplied but non-numeric value
	// (e.g. ?page=abc) is a client error: reject it with 400 instead of
	// silently degrading to the legacy bare-array branch (P2-7).
	nums, err := parseInts(query, "page, size and category", "page", "size", "category")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	page, size, cat := nums[0], nums[1], nums[2]

	// Legacy bare-array form: no pagination/search requested. The explicit
	// PageSize keeps this branch's shape stable (spec 24 changed the *default*
	// page size for the paged path; this one is a diagnostic/back-compat
	// endpoint, not the public list).
	if page == 0 && size == 0 && search == "" && cat == 0 && tag == "" {
		posts, err := svc.ListPostsPage(r.Context(), me, services.ListPostsQuery{
			Status:   status,
			PageSize: 20,
			SortBy:   sort,
			Order:    order,
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

// BatchUpdatePosts applies one bulk action to many posts (spec 26).
// POST /api/admin/posts/batch (auth, admin)
//
// Partial success is intentional: the response reports affected rows plus the
// skipped ones with reasons, so a single 403 does not hide the rest.
func BatchUpdatePosts(w http.ResponseWriter, r *http.Request) {
	var in services.BatchPostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	me := userFromContext(r)
	res, err := svc.BatchUpdatePosts(r.Context(), me, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
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
