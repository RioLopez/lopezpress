package handlers

import (
	"net/http"
)

// UploadImage handles POST /api/upload. The client sends a multipart/form-data
// body with a single "file" field containing the image. On success the response
// is { "url": "/uploads/<name>" } which the client can embed in Markdown.
//
// Requires a valid JWT (wrapped by withAuth in the router).
func UploadImage(w http.ResponseWriter, r *http.Request) {
	// Cap the request body before the service parses it, so an oversized
	// upload is rejected with 413 instead of buffered into memory.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes())

	url, err := svc.UploadImage(r.Context(), r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": url})
}

// maxUploadBytes mirrors the service's cap so the body limit and the file-size
// check agree.
func maxUploadBytes() int64 {
	if cfg != nil && cfg.UploadMaxBytes > 0 {
		return cfg.UploadMaxBytes
	}
	return 2 * 1024 * 1024
}

// ServeUploads handles GET /uploads/{file} for images stored on disk.
// Path traversal is rejected inside the service; the router maps the
// wildcard "{file}" to this handler.
func ServeUploads(w http.ResponseWriter, r *http.Request) {
	svc.ServeUploads(w, r)
}

// ListUploads returns the locally-stored images in the image library.
// GET /api/uploads  (requires JWT). Remote (SM.MS) images are excluded.
func ListUploads(w http.ResponseWriter, r *http.Request) {
	imgs, err := svc.ListLocalUploads(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, imgs)
}

// DeleteUpload removes a locally-stored image by name.
// DELETE /api/uploads/{name}  (auth, admin).
//
// The image library is an administrative feature, not a per-user one: uploads
// are not attributed to an owner, so allowing any authenticated user to delete
// arbitrary files would let one member wipe another member's images. Deletion
// is therefore restricted to admins/root, matching the other management
// endpoints (users, categories, trash, ...).
func DeleteUpload(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	name := r.PathValue("name")
	if err := svc.DeleteLocalUpload(r.Context(), name); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
