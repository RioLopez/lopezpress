package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	// Register the decoders DecodeConfig needs. Standard library only: the
	// point of the check below is validation, not image processing.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lopezpress/internal/imghost"
	"lopezpress/internal/models"
)

// --- image upload ---

// UploadImage stores client-supplied image bytes under cfg's upload directory
// and returns the public URL to it. The file name is randomized (time + 8
// random hex chars + original extension) so two uploads of the same name never
// collide. Only a small whitelist of image extensions is accepted.
//
// The handler owns everything transport-shaped: reading the body, parsing
// multipart, capping sizes. This function takes plain bytes, which is what
// makes the service layer testable without a request (review finding A1).
// The previous signature accepted *http.Request and began with `_ = ctx`,
// i.e. it took a context it did not use and an HTTP type it should not know.
//
// The returned URL is relative ("/uploads/<file>"); the reverse proxy or
// static file handler is responsible for serving that path. In dev, the Go
// server serves it directly (see router).
func (s *Service) UploadImage(ctx context.Context, filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !isAllowedImageExt(ext) {
		return "", newErr(http.StatusBadRequest, "only image files are allowed (jpg, png, webp, gif)")
	}
	// The extension is attacker-controlled, so verify the bytes are actually an
	// image. Without this, an HTML/SVG payload renamed to ".png" would be
	// stored and later served; the nosniff header stops a browser executing it,
	// but rejecting it outright is defense in depth and keeps the "image
	// library" honest about what it contains.
	if !isAllowedImageContent(data) {
		return "", newErr(http.StatusBadRequest, "file content is not a supported image")
	}

	// Route by site config: the server holds the image-host token, so the
	// browser never sees it (replaces the old localStorage token, spec #7).
	// Token resolution order:
	//  1. cfg.SmmsToken (from .env SM_MS_TOKEN) — the primary, server-held
	//     secret. The browser never learns it.
	//  2. site-config ImgHostToken (DB row) — legacy fallback so existing
	//     setups that stored the token in site settings keep working.
	//
	// When SM.MS mode is selected but NO token is available anywhere, the
	// endpoint returns 503 Service Unavailable so the client knows the host
	// is misconfigured rather than silently landing in local storage.
	siteCfg, _ := s.GetSiteConfig(ctx, false)
	mode := siteCfg.ImgHostMode
	if s.cfg != nil && strings.TrimSpace(s.cfg.SmmsToken) != "" {
		mode = "smms"
	}
	smmsToken := strings.TrimSpace(siteCfg.ImgHostToken)
	if s.cfg != nil && strings.TrimSpace(s.cfg.SmmsToken) != "" {
		smmsToken = strings.TrimSpace(s.cfg.SmmsToken)
	}

	if mode == "smms" {
		if smmsToken == "" {
			return "", newErr(http.StatusServiceUnavailable, "SM.MS is enabled but no token is configured (set SM_MS_TOKEN or the site-config token)")
		}
		if url, e := imghost.Upload(ctx, filename, data, smmsToken); e == nil {
			s.audit(ctx, models.User{}, "upload_image", "upload:"+url, "host=smms; name="+filename)
			return url, nil
		}
	}

	url, err := s.saveLocalImage(ext, data)
	if err != nil {
		return "", err
	}
	s.audit(ctx, models.User{}, "upload_image", "upload:"+url, "host=local; name="+filename)
	return url, nil
}

// saveLocalImage writes image bytes to the configured upload directory and
// returns the public /uploads/<name> URL.
func (s *Service) saveLocalImage(ext string, data []byte) (string, error) {
	dir := s.uploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", newErr(http.StatusInternalServerError, "cannot create upload directory")
	}
	name := randomName(ext)
	dst := filepath.Join(dir, name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", newErr(http.StatusInternalServerError, "cannot write file")
	}
	return "/uploads/" + name, nil
}

// uploadDir returns the configured upload directory (always absolute-safe
// relative to the process working directory).
func (s *Service) uploadDir() string {
	if s.cfg != nil && s.cfg.UploadDir != "" {
		return s.cfg.UploadDir
	}
	return "data/uploads"
}

func (s *Service) uploadMaxBytes() int64 {
	if s.cfg != nil && s.cfg.UploadMaxBytes > 0 {
		return s.cfg.UploadMaxBytes
	}
	return 2 * 1024 * 1024 // 2 MB default
}

func isAllowedImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}

// Bounds for a decoded image. They are not a substitute for the upload byte
// cap: a small PNG can decode to a gigapixel canvas (a decompression bomb),
// which is why the geometry is checked from the header before any pixel data
// is touched.
const (
	maxImageSide   = 12000
	maxImagePixels = 40 * 1000 * 1000
)

// isAllowedImageContent reports whether the bytes are a real raster image
// (PNG / JPEG / GIF / WebP) of sane dimensions.
//
// The first check is the stdlib content sniffer (512 bytes), which already
// rejects HTML/SVG payloads renamed to .png. But a sniffed header is only a
// few bytes: an 8-byte PNG signature followed by zeros used to be accepted,
// and so was a GIF89a header glued to an HTML document (polyglot). Decoding
// the header with image.DecodeConfig proves the format actually parses and
// yields its geometry, which closes both cases without adding a dependency.
//
// WebP is the exception: the stdlib has no WebP decoder, so magic bytes are
// the strongest check available there.
func isAllowedImageContent(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	switch http.DetectContentType(data) {
	case "image/webp":
		return true
	case "image/png", "image/jpeg", "image/gif":
	default:
		return false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return false
	}
	if cfg.Width > maxImageSide || cfg.Height > maxImageSide {
		return false
	}
	return int64(cfg.Width)*int64(cfg.Height) <= maxImagePixels
}

// randomName returns a collision-resistant file name: <unix-ms>-<8hex><ext>.
func randomName(ext string) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("%d-%s%s", time.Now().UnixMilli(), hex.EncodeToString(buf), ext)
}

// ListLocalUploads lists the files currently in the local upload directory,
// sorted newest-first. Remote (SM.MS) images are not managed here — deleting
// them would leave broken links, so the library only governs files this
// instance wrote to disk.
func (s *Service) ListLocalUploads(ctx context.Context) ([]UploadImageInfo, error) {
	_ = ctx
	dir := s.uploadDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		// A missing upload dir just means "nothing has been uploaded yet".
		if os.IsNotExist(err) {
			return []UploadImageInfo{}, nil
		}
		return nil, newErr(http.StatusInternalServerError, "cannot read upload directory")
	}

	var out []UploadImageInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !isAllowedImageExt(strings.ToLower(filepath.Ext(name))) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, UploadImageInfo{
			Name:    name,
			URL:     "/uploads/" + name,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}
	if out == nil {
		out = []UploadImageInfo{}
	}
	// Newest first. Use the standard library sort instead of a hand-rolled
	// O(n²) bubble pass (P2 code-debt: replace hand-written standard-library
	// logic).
	slices.SortFunc(out, func(a, b UploadImageInfo) int {
		switch {
		case a.ModTime.Equal(b.ModTime):
			return 0
		case a.ModTime.After(b.ModTime):
			return -1
		default:
			return 1
		}
	})
	return out, nil
}

// DeleteLocalUpload removes a locally-stored image by its file name. The name
// is validated strictly against the upload directory (no path traversal), so
// it can never escape into the surrounding filesystem.
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
	s.audit(ctx, models.User{}, "delete_upload", "upload:"+name, "")
	return nil
}

// ResolveBackupFile validates a backup file name and returns its absolute path
// and size, ready for the transport layer to stream.
//
// `name` is the bare file name returned by ExportBackup. It is validated the
// same way as uploads: no separators, no "..", and the resolved path must stay
// inside the backup directory. Returns 404 for anything suspicious so probing
// cannot distinguish "bad name" from "missing file".
//
// It returns a path instead of writing a response: the previous ServeBackup
// set Content-Type/Length/Disposition and ran io.Copy(w, f) itself, which put
// http.ResponseWriter in the domain layer (review finding A1).
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
func (s *Service) ResolveUploadFile(name string) (string, error) {
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
		return "", newErr(http.StatusNotFound, "not found")
	}
	return filepath.Join(s.uploadDir(), name), nil
}
