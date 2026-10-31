package handlers

import (
	"errors"
	"io"
	"net/http"

	"lopezpress/internal/services"
)

// ImportMarkdown bulk-creates posts from uploaded .md files.
// POST /api/admin/import/markdown (auth, admin/root, multipart/form-data)
//
// Partial success is the contract: per-file results come back in `skipped`,
// so one broken file never blocks an import of 200.
func ImportMarkdown(w http.ResponseWriter, r *http.Request) {
	// The overall cap (24 MiB = 200 files × 2 MB) is the route's
	// maxMarkdownImportLimit, applied by the router before this handler runs.
	// Only the non-file form field budget is decided here.
	//
	// An over-limit body surfaces as a parse error, so name the cause instead of
	// answering "malformed multipart": the router's reader is what failed, and
	// the operator's fix (fewer/smaller files) is not "fix your form".
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "import too large (max 24 MB total)")
			return
		}
		writeError(w, http.StatusBadRequest, "failed to parse upload (max 24 MB total)")
		return
	}
	headers := r.MultipartForm.File["files"]
	files := make([]services.MarkdownFile, 0, len(headers))
	for _, fh := range headers {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		// Read one byte past the per-file cap so the service can report
		// "file too large" instead of silently truncating the content.
		data, err := io.ReadAll(io.LimitReader(f, (2<<20)+1))
		_ = f.Close()
		if err != nil {
			continue
		}
		files = append(files, services.MarkdownFile{Name: fh.Filename, Body: data})
	}

	res, err := svc.ImportMarkdown(r.Context(), userFromContext(r), files)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
