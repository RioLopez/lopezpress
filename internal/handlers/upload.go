package handlers

import (
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"lopezpress/internal/services"
)

// hardUploadCeiling is the absolute body cap for any upload, whatever the site
// policy says. It exists so a misconfigured quota cannot turn a single request
// into a memory exhaustion event on a 1 GB VPS: the bytes are read into memory
// before the service can inspect them.
//
// 20 MiB is not a round number picked for looks — it is the top of the range
// applyStoragePolicy clamps upload_max_image to, and it sits below the route's
// maxUploadBodyLimit (24 MiB). Both bounds matter and they are different:
//
//   - Below the clamp, so this constant can never be the thing that silently
//     overrides an operator's configured limit.
//   - Below the route ceiling, because the router runs first and whichever cap
//     is SMALLER wins. A 32 MiB ceiling here would be dead code AND a lie: the
//     documented "largest accepted image" would be 32 MiB while no request over
//     24 MiB could ever reach the handler. security_check.py (S9) flags exactly
//     this direction; a runtime value like this one it cannot evaluate, which is
//     why the reasoning has to be written down instead.
const hardUploadCeiling = 20 << 20 // 20 MB

// UploadImage handles POST /api/upload. The client sends a multipart/form-data
// body with a "file" field and an optional "kind" field ("post" by default,
// "avatar" for the avatar picker). On success the response carries the public
// URL plus the geometry and a `deduped` flag.
//
// Requires a valid JWT (wrapped by withAuth in the router).
//
// All request parsing happens here. The service takes plain bytes, so it never
// sees *http.Request (review finding A1).
func UploadImage(w http.ResponseWriter, r *http.Request) {
	// Rate limit first: a refused request should cost nothing, and parsing a
	// multipart body is exactly the work this is meant to protect.
	if wait, ok := svc.AllowUpload(userFromContext(r).ID); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "too many uploads in a row; wait a few seconds and try again")
		return
	}

	limit := maxUploadBytes(r)
	// Cap the request body before parsing, so an oversized upload is rejected
	// instead of buffered into memory.
	r.Body = http.MaxBytesReader(w, r.Body, limit)

	const maxFormBytes = 1 << 20 // 1 MB of non-file form fields
	if err := r.ParseMultipartForm(maxFormBytes); err != nil {
		// MaxBytesReader above truncates an oversized body *during* parsing, so
		// the explicit header.Size check below would never fire for a real
		// over-limit upload. Surface that as 413, not a misleading 400.
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "file too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	if header.Size > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}

	// Read the whole file into memory (capped) so the service can either proxy
	// it to a remote image host or save it locally from the same bytes.
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}

	kind := r.FormValue("kind")
	out, err := svc.UploadImage(r.Context(), header.Filename, data, kind)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// maxUploadBytes mirrors the widest per-file ceiling the site policy allows,
// so the transport cap and the service's per-kind cap agree. Reading the config
// here costs one indexed SQLite lookup per upload, which is noise next to the
// work of decoding an image.
func maxUploadBytes(r *http.Request) int64 {
	limit := int64(5 << 20)
	if cfg != nil && cfg.UploadMaxBytes > limit {
		limit = cfg.UploadMaxBytes
	}
	if c, err := svc.GetSiteConfig(r.Context(), false); err == nil {
		if c.UploadMaxImage > limit {
			limit = c.UploadMaxImage
		}
		if c.UploadMaxAvatar > limit {
			limit = c.UploadMaxAvatar
		}
	}
	if limit > hardUploadCeiling {
		limit = hardUploadCeiling
	}
	return limit
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
	// An upload URL identifies one immutable object, which is the precondition
	// a long max-age demands. The stored name is 22 random characters chosen
	// when the row is created, and it is never reused: identical bytes resolve
	// to the same name through the sha256 dedup, and any change in bytes creates
	// a new name rather than overwriting the old one. Nothing ever rewrites a
	// file in place, so a given URL's content cannot drift.
	//
	// Without this header the browser had only Last-Modified to work with and
	// revalidated every image on every visit. On the reading page images are the
	// largest byte cost, and a conditional request per image is a full round
	// trip per image even when the body is a 304. On a 1C box this is the
	// cheapest saving available: one header, zero behaviour change.
	//
	// 30 days, NOT `immutable`, and the difference is deliberate:
	//   - `immutable` forbids revalidation entirely, so a bad image an admin
	//     hard-deletes — the one action the deletion policy exists for — would
	//     keep displaying in every browser that already saw it, for the whole
	//     max-age. Moderation would be a no-op for existing readers.
	//   - 30 days still covers essentially all revisits (the same person rarely
	//     opens the same article twice a month later), and the cost when the
	//     window does expire is one 304, which carries no body.
	//
	// The stat is not redundant with ServeFile's own. A name that resolves but
	// no longer exists (the row was hard-deleted) is answered with 404, and a
	// 404 carrying a month-long max-age would be cached just as long — a file
	// restored from backup would stay invisible to that browser. Only a real
	// regular file earns the cache header.
	if fi, statErr := os.Stat(path); statErr == nil && fi.Mode().IsRegular() {
		w.Header().Set("Cache-Control", "public, max-age=2592000")
	}
	http.ServeFile(w, r, path)
}

// ListUploads returns the site-wide image library.
// GET /api/uploads  (auth, admin/root)
//
// This endpoint used to be open to every authenticated user, which made the
// blog a free image host: any member could enumerate every image on the site
// and hot-link it. The library is a site-owner asset view.
//
// Query: ?owner=<user-id> ?kind=<post|avatar|site> ?status=<live|deleted|all>
//
//	?page=N ?size=N
func ListUploads(w http.ResponseWriter, r *http.Request) {
	q, ok := parseUploadListQuery(w, r)
	if !ok {
		return
	}
	q.OwnerID = strings.TrimSpace(r.URL.Query().Get("owner"))
	page, err := svc.ListUploads(r.Context(), q)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// ListMyUploads returns only the caller's own uploads.
// GET /api/me/uploads  (auth)
//
// The owner filter is forced here, never taken from the query string: this is
// the endpoint behind "withdraw an image I uploaded", and accepting ?owner=
// would turn it back into a directory of everybody's files.
func ListMyUploads(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r)
	q, ok := parseUploadListQuery(w, r)
	if !ok {
		return
	}
	q.OwnerID = me.ID
	// A user has no business seeing other people's withdrawn files, and their
	// own withdrawn ones are already gone from their point of view.
	q.Status = ""
	page, err := svc.ListUploads(r.Context(), q)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// parseUploadListQuery reads the shared paging/filter parameters. It writes the
// 400 itself and reports ok=false so both list handlers stay tiny.
func parseUploadListQuery(w http.ResponseWriter, r *http.Request) (services.UploadListQuery, bool) {
	vals := r.URL.Query()
	nums, err := parseInts(vals, "page and size", "page", "size")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return services.UploadListQuery{}, false
	}
	return services.UploadListQuery{
		Kind:   strings.TrimSpace(vals.Get("kind")),
		Status: strings.TrimSpace(vals.Get("status")),
		Page:   nums[0],
		Size:   nums[1],
	}, true
}

// DeleteUpload removes a locally-stored image.
// DELETE /api/uploads/{name}           (auth, admin/root)  — soft by default
// DELETE /api/uploads/{name}?hard=1    (auth, admin/root)  — removes the file
//
// Both routes pass through the same service call: which of soft/hard is legal
// depends on who is asking, and the service is where "who" is already known.
func DeleteUpload(w http.ResponseWriter, r *http.Request) {
	hard := r.URL.Query().Get("hard") == "1"
	out, err := svc.DeleteUpload(r.Context(), userFromContext(r), r.PathValue("name"), hard)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// DeleteMyUpload withdraws one of the caller's own uploads.
// DELETE /api/me/uploads/{name}  (auth)
//
// Always soft: an author can take their picture off the site, but only an
// admin can destroy the file (and the references other articles may have).
func DeleteMyUpload(w http.ResponseWriter, r *http.Request) {
	out, err := svc.DeleteUpload(r.Context(), userFromContext(r), r.PathValue("name"), false)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// RestoreUpload brings a withdrawn image back into the library.
// POST /api/uploads/{name}/restore  (auth, admin/root)
func RestoreUpload(w http.ResponseWriter, r *http.Request) {
	if err := svc.RestoreUpload(r.Context(), userFromContext(r), r.PathValue("name")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "restored"})
}
