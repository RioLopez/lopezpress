package handlers

import (
	"io"
	"net/http"
	"strings"
)

// UploadImage handles POST /api/upload. The client sends a multipart/form-data
// body with a single "file" field containing the image. On success the response
// is { "url": "/uploads/<name>" } which the client can embed in Markdown.
//
// Requires a valid JWT (wrapped by withAuth in the router).
//
// All request parsing happens here. The service takes plain bytes, so it never
// sees *http.Request (review finding A1).
func UploadImage(w http.ResponseWriter, r *http.Request) {
	// Cap the request body before parsing, so an oversized upload is rejected
	// instead of buffered into memory.
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes())

	const maxFormBytes = 1 << 20 // 1 MB of non-file form fields
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	if header.Size > maxUploadBytes() {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	// Read the whole file into memory (capped) so the service can either proxy
	// it to a remote image host or save it locally from the same bytes.
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	url, err := svc.UploadImage(r.Context(), header.Filename, data)
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
//
// The service returns a resolved path; serving it (Range, If-Modified-Since,
// conditional GET) is transport work and stays here (review finding A1).
func ServeUploads(w http.ResponseWriter, r *http.Request) {
	// Path looks like /uploads/<name>; the router strips nothing, so the
	// last path segment is the file name.
	name := strings.TrimPrefix(r.URL.Path, "/uploads/")
	path, err := svc.ResolveUploadFile(name)
	if err != nil {
		http.NotFound(w, r) // identical to the old behaviour: hide existence
		return
	}
	http.ServeFile(w, r, path)
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
	name := r.PathValue("name")
	if err := svc.DeleteLocalUpload(r.Context(), name); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
