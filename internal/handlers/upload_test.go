package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestUploadImageOversizeReturns413 locks AR-1c: an over-limit upload must
// surface as 413 (not a misleading 400 "invalid multipart form"). MaxBytesReader
// truncates the body during ParseMultipartForm, so this asserts that error path
// is mapped back to RequestEntityTooLarge.
func TestUploadImageOversizeReturns413(t *testing.T) {
	_, _ = setup(t)

	// The body cap now depends on the site policy (spec 36), so probe it with a
	// bodyless request first and then write one byte past whatever it returns.
	probe := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	limit := maxUploadBytes(probe)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "big.jpg")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(bytes.Repeat([]byte("a"), int(limit)+1024)); err != nil {
		t.Fatalf("write file part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	UploadImage(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d (%s)", rec.Code, rec.Body.String())
	}
}
