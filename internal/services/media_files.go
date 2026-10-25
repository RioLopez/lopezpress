package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lopezpress/internal/models"
)

// --- upload file system primitives -----------------------------------------
//
// Everything that touches the upload directory lives here, so the storage
// policy (media.go / media_library.go) never mixes with file I/O.

// uploadResult renders a metadata row as the transport-facing struct.
func uploadResult(f models.UploadFile, deduped bool) UploadResult {
	return UploadResult{
		ID:      f.ID,
		Name:    f.Name,
		URL:     "/uploads/" + f.Name,
		Size:    f.Size,
		Width:   f.Width,
		Height:  f.Height,
		Mime:    f.Mime,
		Kind:    f.Kind,
		Deduped: deduped,
	}
}

// uploadDir returns the configured upload directory (always absolute-safe
// relative to the process working directory).
func (s *Service) uploadDir() string {
	if s.cfg != nil && s.cfg.UploadDir != "" {
		return s.cfg.UploadDir
	}
	return "data/uploads"
}

// uploadMaxBytes is the process-level body cap, kept as the fallback used by
// the HTTP layer when the site row has no policy yet.
func (s *Service) uploadMaxBytes() int64 {
	if s.cfg != nil && s.cfg.UploadMaxBytes > 0 {
		return s.cfg.UploadMaxBytes
	}
	return 5 * 1024 * 1024
}

// writeUploadFile writes the bytes under the configured upload directory.
func (s *Service) writeUploadFile(name string, data []byte) error {
	dir := s.uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return newErr(http.StatusInternalServerError, "cannot create upload directory")
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return newErr(http.StatusInternalServerError, "cannot write file")
	}
	return nil
}

// removeUploadFile deletes a file this instance wrote, used to roll back a
// failed metadata insert. It never reports an error to the caller: the row is
// already the source of truth, and a leftover unreferenced file is harmless
// compared with failing an upload that actually succeeded.
func (s *Service) removeUploadFile(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return nil
	}
	return os.Remove(filepath.Join(s.uploadDir(), name))
}

// DeleteLocalUpload removes a locally-stored file by name without touching
// metadata. It is the low-level primitive used after the row has been removed
// (and by tests); callers that own a database must go through DeleteUpload,
// which keeps the row and the file in step.
//
// The name is validated strictly against the upload directory (no path
// traversal), so it can never escape into the surrounding filesystem.
func (s *Service) DeleteLocalUpload(ctx context.Context, name string) error {
	_ = ctx
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return newErr(http.StatusBadRequest, "invalid file name")
	}
	// Defence in depth (aligned with ServeBackup): resolve the absolute upload
	// directory and confirm the joined path did not escape it. Without this,
	// a non-absolute uploadDir or an unusual name could let os.Remove reach
	// outside the upload folder.
	absDir, err := filepath.Abs(s.uploadDir())
	if err != nil {
		return newErr(http.StatusInternalServerError, "cannot resolve upload directory")
	}
	full := filepath.Join(absDir, name)
	if rel, err := filepath.Rel(absDir, full); err != nil || strings.HasPrefix(rel, "..") {
		return newErr(http.StatusBadRequest, "invalid file name")
	}
	if err := os.Remove(full); err != nil {
		if os.IsNotExist(err) {
			return newErr(http.StatusNotFound, "file not found")
		}
		return newErr(http.StatusInternalServerError, "failed to delete file")
	}
	return nil
}

// ResolveBackupFile validates a backup file name and returns its absolute path
// and size, ready for the transport layer to stream.
//
// `name` is the bare file name returned by ExportBackup. It is validated the
// same way as uploads: no separators, no "..", and the resolved path must stay
// inside the backup directory. Returns 404 for anything suspicious so probing
// cannot distinguish "bad name" from "missing file".
func (s *Service) ResolveBackupFile(name string) (path string, size int64, err error) {
	// Reject traversal outright: bare name required.
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", 0, newErr(http.StatusNotFound, "backup not found")
	}
	if filepath.Base(name) != name {
		return "", 0, newErr(http.StatusNotFound, "backup not found")
	}

	dir, err := filepath.Abs(s.backupDir())
	if err != nil {
		return "", 0, newErr(http.StatusInternalServerError, "failed to resolve backup directory")
	}
	full := filepath.Join(dir, name)
	// Defence in depth: confirm the join did not escape the directory.
	if rel, err := filepath.Rel(dir, full); err != nil || strings.HasPrefix(rel, "..") {
		return "", 0, newErr(http.StatusNotFound, "backup not found")
	}

	info, err := os.Stat(full)
	if err != nil || info.IsDir() {
		return "", 0, newErr(http.StatusNotFound, "backup not found")
	}
	return full, info.Size(), nil
}

// ResolveUploadFile resolves a locally-stored upload into an absolute path.
// It resolves the file strictly inside cfg.UploadDir to prevent path traversal
// (a request for /uploads/../etc/passwd is rejected).
//
// The caller serves the file. This function used to call http.ServeFile and
// http.NotFound directly (review finding A1).
//
// Note that a soft-deleted upload is still served: articles that embed it were
// published before the uploader withdrew it, and turning those into broken
// images would punish the article's author for someone else's decision. What a
// soft delete removes is discoverability and re-usability, not the bytes.
func (s *Service) ResolveUploadFile(name string) (string, error) {
	// Every path here ends up inside http.ServeFile, which answers 404 for a
	// missing file but 500 for an "illegal" one. A name that can never exist —
	// an embedded NUL, a path separator, a Windows drive or alternate-data
	// colon, a control byte, or a length past any filesystem limit — must be
	// rejected as 404 up front, not passed to the OS and surfaced as a 500
	// (which tells the caller the server broke, and fingerprints it as Go).
	if name == "" || len(name) > 255 ||
		strings.ContainsAny(name, "/\\:\x00") ||
		strings.Contains(name, "..") ||
		strings.IndexFunc(name, func(r rune) bool { return r < 0x20 }) >= 0 {
		return "", newErr(http.StatusNotFound, "not found")
	}
	return filepath.Join(s.uploadDir(), name), nil
}
