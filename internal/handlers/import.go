package handlers

import (
	"io"
	"net/http"

	"lopezpress/internal/services"
)

// ImportMarkdown bulk-creates posts from uploaded .md files (spec 32).
// POST /api/admin/import/markdown (auth, admin/root, multipart/form-data)
//
// Partial success is the contract: per-file results come back in `skipped`,
// so one broken file never blocks an import of 200.
func ImportMarkdown(w http.ResponseWriter, r *http.Request) {
	// Cap the whole request before touching disk/memory: 200 files × 2 MB.
	const maxBody = 24 << 20
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
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
