package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"lopezpress/internal/services"
)

// UploadPDF handles POST /api/uploads/pdf. Same multipart contract as
// UploadImage, but the file must be a PDF and is stored without decoding.
func UploadPDF(w http.ResponseWriter, r *http.Request) {
	if wait, ok := svc.AllowUpload(userFromContext(r).ID); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "too many uploads in a row; wait a few seconds and try again")
		return
	}

	limit := int64(services.MaxPDFBytes)
	// Allow a little room for the multipart envelope; the file itself is
	// checked against the exact limit below.
	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
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
		writeError(w, http.StatusRequestEntityTooLarge, "PDF is too large (max 20 MB)")
		return
	}

	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read file")
		return
	}
	out, err := svc.UploadPDF(r.Context(), header.Filename, data)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
