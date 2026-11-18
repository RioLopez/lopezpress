package services

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"lopezpress/internal/models"
)

// --- restore an uploads snapshot ---------------------------------------------
//
// A database snapshot restores rows; the images and PDFs they point at live in
// the upload directory and are only captured when "include uploads" is on.
// Until this endpoint existed the companion folder had no consumer, so
// "attachments are backed up" was true but not actionable.
//
// The restore is deliberately additive: it copies files that are MISSING from
// the live directory and never overwrites one that exists. Uploaded files are
// immutable (the name is an opaque id and the bytes never change), so a file
// that is still there is by definition the one the snapshot holds; the only
// files this can add are the ones that were deleted or lost. That also makes
// the operation safe to repeat.
//
// It does not touch the database. The intended order is "restore the .db
// snapshot, then the uploads", so rows and files come from the same moment.

// validUploadsSnapshotName accepts exactly uploads-YYYYMMDD, the shape the job
// writes. Anything else — a separator, "..", an absolute path — is refused
// before it is joined onto a directory.
func validUploadsSnapshotName(name string) bool {
	if !strings.HasPrefix(name, uploadsSnapshotPrefix) || filepath.Base(name) != name {
		return false
	}
	raw := strings.TrimPrefix(name, uploadsSnapshotPrefix)
	if len(raw) != 8 {
		return false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// RestoreUploadsSnapshot copies every missing file back from one snapshot.
func (s *Service) RestoreUploadsSnapshot(ctx context.Context, actor models.User, name string) (UploadsRestoreResult, error) {
	if !IsRoot(actor) {
		return UploadsRestoreResult{}, newErr(http.StatusForbidden, "only the root can restore uploads")
	}
	if !validUploadsSnapshotName(name) {
		return UploadsRestoreResult{}, newErr(http.StatusBadRequest, "invalid uploads snapshot name")
	}
	src := filepath.Join(s.backupDir(), name)
	info, err := os.Stat(src)
	if err != nil || !info.IsDir() {
		return UploadsRestoreResult{}, newErr(http.StatusNotFound, "uploads snapshot not found")
	}
	dst := s.uploadDir()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return UploadsRestoreResult{}, err
	}

	res := UploadsRestoreResult{Name: name}
	walkErr := filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		return restoreOneUpload(src, dst, path, d, &res)
	})
	if walkErr != nil {
		return UploadsRestoreResult{}, newErr(http.StatusInternalServerError, "failed to restore uploads")
	}
	s.audit(ctx, actor, "restore_uploads", "backup:"+name,
		fmt.Sprintf("restored=%d; skipped=%d; bytes=%d", res.Restored, res.Skipped, res.Bytes))
	return res, nil
}

// restoreOneUpload copies one snapshot file into the live directory when it is
// missing there. A file that already exists is skipped, never overwritten.
func restoreOneUpload(src, dst, path string, d fs.DirEntry, res *UploadsRestoreResult) error {
	// Snapshots only ever hold regular files; a symlink could point outside the
	// tree, so it is counted as skipped instead of followed.
	if d.Type()&os.ModeSymlink != 0 || !d.Type().IsRegular() {
		res.Skipped++
		return nil
	}
	rel, relErr := filepath.Rel(src, path)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		res.Skipped++
		return nil
	}
	target := filepath.Join(dst, rel)
	if _, statErr := os.Stat(target); statErr == nil {
		res.Skipped++ // already present: restore never overwrites
		return nil
	}
	info, infoErr := d.Info()
	if infoErr != nil {
		return infoErr
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := copyFilePreservingTime(path, target, info); err != nil {
		return err
	}
	res.Restored++
	res.Bytes += info.Size()
	return nil
}
