package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	// Register the decoders the geometry probe needs. The point of the check
	// is validation, not image processing. WebP is registered in thumbs.go,
	// where the decoder is needed for real; the registration is process-wide,
	// so it also covers the probes here and in media_validate.go.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/imghost"
	"lopezpress/internal/models"
)

// --- image upload ---

// Upload kinds. The kind decides the per-file ceiling and where the image is
// allowed to be used; it is chosen by the endpoint, never trusted from the
// client beyond the allowed set.
const (
	UploadKindPost   = "post"
	UploadKindAvatar = "avatar"
	UploadKindSite   = "site"
)

// UploadImage validates client-supplied bytes and stores them.
//
// Round 77 changes (spec 36):
//   - the file name is an opaque 22-char base62 id, not "<unix-ms>-<hex>",
//     so a URL no longer leaks when the image was uploaded;
//   - the bytes are hashed and de-duplicated: uploading the same picture twice
//     reuses the first row and writes nothing to disk;
//   - a metadata row is written for every new file (owner, size, geometry,
//     hash, source IP), which is what makes the quotas and the admin gallery
//     possible at all;
//   - the per-file ceiling depends on the kind (article image vs avatar).
//
// The handler owns everything transport-shaped: reading the body, parsing
// multipart, capping sizes. This function takes plain bytes, which is what
// makes the service layer testable without a request (review finding A1).
func (s *Service) UploadImage(ctx context.Context, filename string, data []byte, kind string) (UploadResult, error) {
	kind = normalizeUploadKind(kind)
	ext := strings.ToLower(filepath.Ext(filename))
	if !isAllowedImageExt(ext) {
		return UploadResult{}, newErr(http.StatusBadRequest, "only image files are allowed (jpg, png, webp, gif)")
	}
	// The extension is attacker-controlled, so verify the bytes are actually an
	// image. Without this, an HTML/SVG payload renamed to ".png" would be
	// stored and later served; the nosniff header stops a browser executing it,
	// but rejecting it outright is defense in depth and keeps the "image
	// library" honest about what it contains.
	if !isAllowedImageContent(data) {
		return UploadResult{}, newErr(http.StatusBadRequest, "file content is not a supported image")
	}
	// Full decode, re-encode, and the compression-ratio guard: three questions
	// about the same thing ("are these bytes really an image we should store"),
	// asked in one place.
	data, err := prepareUploadBytes(data, ext)
	if err != nil {
		return UploadResult{}, err
	}

	siteCfg, _ := s.GetSiteConfig(ctx, false)

	// Per-file ceiling by purpose. Checked here rather than only in the
	// handler because the handler caps the *request body* (one number for
	// every kind); this is the per-purpose rule.
	if limit := uploadFileLimit(siteCfg, kind); limit > 0 && int64(len(data)) > limit {
		return UploadResult{}, newErr(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("file too large for a %s image (max %s)", kind, humanBytes(limit)))
	}

	if mode, token := s.imageHostMode(siteCfg); mode == "smms" {
		if res, ok := s.uploadToRemote(ctx, filename, data, kind, token); ok {
			return res, nil
		}
		// A failed remote upload falls through to local storage, as before: the
		// author still gets their picture instead of an opaque 502.
	}
	return s.storeLocalUpload(ctx, ext, filename, data, kind, siteCfg)
}

// imageHostMode resolves which storage backend is active and the token for it.
// Token resolution order:
//  1. cfg.SmmsToken (from .env SM_MS_TOKEN) — the primary, server-held secret.
//     The browser never learns it.
//  2. site-config ImgHostToken (DB row) — legacy fallback so existing setups
//     that stored the token in site settings keep working.
func (s *Service) imageHostMode(cfg models.SiteConfig) (mode, token string) {
	mode = cfg.ImgHostMode
	token = strings.TrimSpace(cfg.ImgHostToken)
	if s.cfg != nil && strings.TrimSpace(s.cfg.SmmsToken) != "" {
		mode = "smms"
		token = strings.TrimSpace(s.cfg.SmmsToken)
	}
	return mode, token
}

// uploadToRemote proxies the bytes to the external image host. ok=false means
// "fall back to local storage".
//
// Remote files are NOT recorded in upload_files: this instance does not own
// them, cannot meter their size, and could not delete them. The local library
// stays the single source of truth for storage accounting.
func (s *Service) uploadToRemote(ctx context.Context, filename string, data []byte, kind, token string) (UploadResult, bool) {
	if token == "" {
		return UploadResult{}, false
	}
	url, err := imghost.Upload(ctx, filename, data, token)
	if err != nil {
		return UploadResult{}, false
	}
	s.audit(ctx, models.User{}, "upload_image", "upload:"+url, "host=smms; name="+filename+"; kind="+kind)
	return UploadResult{
		URL: url, Name: filepath.Base(url), Size: int64(len(data)),
		Mime: http.DetectContentType(data), Kind: kind, Remote: true,
	}, true
}

// storeLocalUpload is the local path: de-duplicate, enforce quotas, write the
// file, record the row. The order matters — every rejection happens before the
// first byte reaches disk, so a refused upload never leaves a stray file.
func (s *Service) storeLocalUpload(ctx context.Context, ext, filename string, data []byte, kind string, siteCfg models.SiteConfig) (UploadResult, error) {
	// live_key is sha256 for a live row and "d:<id>" once soft-deleted, so this
	// lookup can never resurrect an image its owner withdrew. The unique index
	// makes it authoritative even under concurrent identical uploads (see the
	// Create error path below).
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	var existing models.UploadFile
	lookupErr := s.db.WithContext(ctx).Where("live_key = ?", hash).First(&existing).Error
	switch {
	case lookupErr == nil:
		// Re-use. No quota is consumed because no byte was written.
		return s.reuseUpload(ctx, existing, filename), nil
	case !errors.Is(lookupErr, gorm.ErrRecordNotFound):
		return UploadResult{}, newErr(http.StatusInternalServerError, "failed to check image library")
	}

	me := ActorFromContext(ctx)
	id, idErr := models.NewOpaqueID()
	if idErr != nil {
		return UploadResult{}, newErr(http.StatusInternalServerError, "cannot generate file name")
	}
	name := id + ext
	if err := s.writeUploadFile(name, data); err != nil {
		return UploadResult{}, err
	}

	row := models.UploadFile{
		ID: id, Name: name, LiveKey: hash, OwnerID: me.ID,
		Size: int64(len(data)), Sha256: hash,
		OriginalName: sanitizeOriginalName(filename),
		IP:           ClientIPFromContext(ctx),
		Kind:         kind,
		// The thumbnail is produced out of band (thumbs.go). Marking the row
		// pending here is what survives a restart; the channel hint below may
		// be dropped without consequence.
		ThumbStatus: ThumbStatusPending,
	}
	row.Width, row.Height, row.Mime = imageGeometry(data)

	if err := s.recordUpload(ctx, row, siteCfg, me.ID); err != nil {
		if AsServiceError(err) != nil {
			// Refused on purpose (quota): remove the file we just wrote and
			// report the refusal.
			_ = s.removeUploadFile(name)
			return UploadResult{}, err
		}
		// Anything else is a lost INSERT race; uploadCreateFailed retries it as
		// a de-duplication hit and cleans the file up either way.
		return s.uploadCreateFailed(ctx, name, hash, filename)
	}

	s.audit(ctx, me, "upload_image", "upload:"+name,
		fmt.Sprintf("kind=%s; size=%d; dedup=0; name=%s", kind, len(data), filename))
	s.enqueueThumb(row.ID)
	return uploadResult(row, false), nil
}

// recordUpload writes the library row, with the quota check inside the same
// transaction.
//
// Split out of storeLocalUpload because the atomicity is the whole point and
// it deserves to be readable on its own. The quota check and the INSERT are
// one transaction: apart, they are a TOCTOU window in which N concurrent
// uploads each read the same "under quota" total and each insert, so the
// ceiling is exceeded by roughly (N-1) × file size — R4-08 measured 10
// concurrent uploads landing the user 1.5 MB over a 20 MB quota, R6-03 got 8
// uploads 1.4 MB over. _txlock=immediate (see config.sqliteDSN) already
// serialises writers at BEGIN; the transaction makes the guarantee explicit
// rather than incidental, so it survives someone removing the DSN flag.
//
// The file is on disk before this runs, deliberately: a rollback cannot
// un-write it, and the row is what makes it deletable.
func (s *Service) recordUpload(ctx context.Context, row models.UploadFile, siteCfg models.SiteConfig, ownerID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.checkUploadQuotaTx(tx, siteCfg, ownerID, row.Size); err != nil {
			return err
		}
		return tx.Create(&row).Error
	})
}

// uploadCreateFailed resolves an INSERT that lost the race. Two outcomes are
// possible and only one of them is an error:
//   - the unique live_key index fired because someone else uploaded the
//     identical bytes between our SELECT and our INSERT — that is a normal
//     concurrent de-duplication and must resolve to a successful upload
//     pointing at the winner's row;
//   - the database is genuinely unhappy.
func (s *Service) uploadCreateFailed(ctx context.Context, name, hash, filename string) (UploadResult, error) {
	_ = s.removeUploadFile(name)
	var winner models.UploadFile
	if err := s.db.WithContext(ctx).Where("live_key = ?", hash).First(&winner).Error; err == nil {
		return s.reuseUpload(ctx, winner, filename), nil
	}
	return UploadResult{}, newErr(http.StatusInternalServerError, "failed to record upload")
}

// reuseUpload returns an existing row as a de-duplicated result. It also asks
// the thumbnail worker for a pass when the row has none yet: the row may
// predate the worker, or its last hint may have been dropped from a full
// queue. The worker is idempotent, so a redundant hint costs one query.
func (s *Service) reuseUpload(ctx context.Context, existing models.UploadFile, filename string) UploadResult {
	if needsThumb(existing) {
		s.enqueueThumb(existing.ID)
	}
	s.audit(ctx, models.User{}, "upload_image", "upload:"+existing.Name, "dedup=1; name="+filename)
	return uploadResult(existing, true)
}

// normalizeUploadKind maps anything unexpected onto the default kind. The
// client sends "avatar" for the avatar endpoint; every other value (including
// an empty one) is an article image.
func normalizeUploadKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case UploadKindAvatar:
		return UploadKindAvatar
	case UploadKindSite:
		return UploadKindSite
	default:
		return UploadKindPost
	}
}

// uploadFileLimit returns the per-file ceiling for a kind, falling back to the
// process-level default when the site row carries no usable value (a database
// seeded before spec 36 has zeros in those columns).
func uploadFileLimit(cfg models.SiteConfig, kind string) int64 {
	var limit int64
	switch kind {
	case UploadKindAvatar:
		limit = cfg.UploadMaxAvatar
		if limit <= 0 {
			limit = 512 * 1024
		}
	default:
		limit = cfg.UploadMaxImage
		if limit <= 0 {
			limit = 5 * 1024 * 1024
		}
	}
	return limit
}

// imageGeometry extracts the dimensions and MIME type from the header. Zero
// dimensions mean "the decoder could not read this format", which the gallery
// renders as "—" rather than as a lie. Since thumbs.go registers the WebP
// decoder, WebP now reports real dimensions too — except animated WebP, which
// golang.org/x/image/webp still declines to parse.
func imageGeometry(data []byte) (w, h int, mime string) {
	mime = http.DetectContentType(data)
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		w, h = cfg.Width, cfg.Height
	}
	return w, h, mime
}

// sanitizeOriginalName trims the client-supplied name to something that fits
// the column and cannot confuse a terminal. It is display-only metadata: the
// stored file never uses it.
func sanitizeOriginalName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\x00", "")
	if len([]rune(name)) > 120 {
		name = string([]rune(name)[:120])
	}
	return name
}

func isAllowedImageExt(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}
