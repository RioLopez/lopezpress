package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- PDF attachments --------------------------------------------------------
//
// A PDF is a binary document, not article text. It lives in upload_files like
// an image (owner, size, hash, quota, dedup), but it is never decoded or
// re-encoded: the standard library has no PDF parser, and pretending to
// validate one would be worse than admitting the limit. What is checked is the
// magic header, the transport MIME sniff and the hard byte cap.
//
// Serving is the existing /uploads/{name} route. The reader opens a PDF in a
// new tab; framing it is blocked by SecurityHeaders (X-Frame-Options: DENY).

const (
	// MaxPDFBytes is both the per-file ceiling and the handler's read cap.
	// It stays below the route's 24 MiB body limit, so the transport ceiling
	// is never the value that silently wins (security_check S9's direction).
	MaxPDFBytes = 20 << 20

	// MaxAttachmentsPerPost bounds one article. The storage quota already
	// bounds the bytes; this bounds the row count and the editor UI.
	MaxAttachmentsPerPost = 10
)

// isAllowedPDFContent reports whether the bytes start with a PDF header and
// are recognised as application/pdf by the stdlib sniffer. Some legitimate
// files carry leading bytes before %PDF-, so the magic is searched in the
// first KiB rather than required at offset zero.
func isAllowedPDFContent(data []byte) bool {
	if len(data) < 5 || int64(len(data)) > MaxPDFBytes {
		return false
	}
	head := data
	if len(head) > 1024 {
		head = head[:1024]
	}
	if !bytes.Contains(head, []byte("%PDF-")) {
		return false
	}
	return http.DetectContentType(data) == "application/pdf"
}

// UploadPDF stores one PDF attachment. The caller is the authenticated
// uploader; quotas and de-duplication are shared with images.
func (s *Service) UploadPDF(ctx context.Context, filename string, data []byte) (UploadResult, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".pdf" {
		return UploadResult{}, newErr(http.StatusBadRequest, "only PDF files are allowed")
	}
	if int64(len(data)) > MaxPDFBytes {
		return UploadResult{}, newErr(http.StatusRequestEntityTooLarge, "PDF is too large (max 20 MB)")
	}
	if !isAllowedPDFContent(data) {
		return UploadResult{}, newErr(http.StatusBadRequest, "file content is not a PDF")
	}
	siteCfg, _ := s.GetSiteConfig(ctx, false)
	sum := sha256.Sum256(data)
	// A PDF is never transformed, so its source hash and final hash are the
	// same value. Keeping both populated makes the library's grouping field
	// meaningful for every row rather than "empty means PDF".
	hash := hex.EncodeToString(sum[:])
	return s.storeLocalUpload(ctx, ".pdf", filename, data, UploadKindPDF, siteCfg, hash, "")
}

// validatePDFAttachmentIDsTx validates the requested attachment set for one
// post. Every id must be a PDF row in the library owned by the caller (root may
// use any). A missing id is a 400 rather than a silent drop, because the
// editor would otherwise save an article with fewer documents than selected.
func validatePDFAttachmentIDsTx(db *gorm.DB, ids []string, actor models.User) ([]string, error) {
	out := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) > MaxAttachmentsPerPost {
		return nil, newErr(http.StatusBadRequest, "too many attachments (max 10)")
	}
	if len(out) == 0 {
		return out, nil
	}
	var rows []models.UploadFile
	q := db.Model(&models.UploadFile{}).
		Where("id IN ? AND kind = ?", out, UploadKindPDF)
	if !IsRoot(actor) {
		// An author may attach what they uploaded, not somebody else's file.
		// Root is the recovery path and may attach anything in the library.
		q = q.Where("owner_id = ?", actor.ID)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to check attachments")
	}
	if len(rows) != len(out) {
		return nil, newErr(http.StatusBadRequest, "attachment not found or not a PDF you uploaded")
	}
	return out, nil
}
