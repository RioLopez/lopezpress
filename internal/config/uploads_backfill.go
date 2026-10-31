package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	// DecodeConfig needs the format registrations; the backfill only reads the
	// header, never the pixels.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// maxBackfillFileBytes caps what the startup backfill is willing to read from
// disk. A file larger than this was not written by this application (the
// upload cap is 5 MB), so hashing it would mean reading someone else's 2 GB
// dump into memory during boot. It is still registered — just without a hash,
// which also keeps it out of the de-duplication pool.
const maxBackfillFileBytes = 32 << 20 // 32 MB

// backfillUploads gives every file already present in the upload directory a
// metadata row.
//
// Why at startup and not as a schema migration: the manual migrations run
// BEFORE AutoMigrate (a rebuilt table must exist first), so at migration time
// upload_files may not exist yet on a fresh database. AutoMigrate creates it,
// and this runs right after, which is the first moment the table is guaranteed
// to be there on both a fresh and an existing database.
//
// It is idempotent and cheap on the steady state: one SELECT builds the set of
// known names, and a directory whose every file is known costs a single query.
// Legacy rows are owned by nobody ("legacy:<name>" live key, Legacy=true): the
// site quota counts them, but they never appear in a user's own list and never
// satisfy a de-duplication lookup.
func backfillUploads(db *gorm.DB, dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("WARN upload backfill: cannot read %s: %v", dir, err)
		}
		return
	}

	var known []string
	if err := db.Model(&models.UploadFile{}).Pluck("name", &known).Error; err != nil {
		log.Printf("WARN upload backfill: cannot list known uploads: %v", err)
		return
	}
	seen := make(map[string]bool, len(known))
	for _, n := range known {
		seen[n] = true
	}

	added := 0
	for _, e := range entries {
		if e.IsDir() || seen[e.Name()] || !isImageFileName(e.Name()) {
			continue
		}
		// R2-04: a generated thumbnail is named <upload-id>_t.<ext>. It lives in
		// the same directory as the originals, so an extension-only filter
		// matched it and the backfill inserted an owner-less "legacy" row for
		// every thumbnail on disk — 63 of them in the reported run, roughly half
		// the admin gallery, all un-attributable and inflating the site's used
		// bytes. A derived file is not an upload.
		if isDerivedImageName(e.Name()) {
			continue
		}
		if err := backfillOne(db, dir, e); err != nil {
			log.Printf("WARN upload backfill: cannot record %s: %v", e.Name(), err)
			continue
		}
		added++
	}
	if added > 0 {
		log.Printf("upload backfill: recorded %d pre-existing image(s) from %s", added, dir)
	}
}

// thumbSuffix is the marker the thumbnail worker appends before the extension
// when it writes a derived copy (see services.buildThumb: id + "_t" + ext).
const thumbSuffix = "_t"

// isDerivedImageName reports whether a file name is a generated thumbnail
// rather than something a user uploaded. Kept next to isImageFileName because
// the two are always used together: the extension filter decides "could this
// be an image", this one decides "is it OURS".
//
// Tested in uploads_backfill_test.go.
func isDerivedImageName(name string) bool {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return strings.HasSuffix(base, thumbSuffix)
}

// isImageFileName reports whether the extension is one this application writes.
func isImageFileName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	}
	return false
}

// mimeFromName maps a file extension to the MIME type stored in metadata. The
// bytes themselves are sniffed on upload; for legacy files the extension is
// all we have without reading the whole file, which the backfill already does
// only within its size cap.
func mimeFromName(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/jpeg"
	}
}

// backfillOne records a single pre-existing file. It returns an error only for
// a failed insert; unreadable or oversized files are still registered, just
// without a hash (which keeps them out of the de-duplication pool).
func backfillOne(db *gorm.DB, dir string, e os.DirEntry) error {
	name := e.Name()
	info, err := e.Info()
	if err != nil || info.Size() == 0 {
		return nil
	}
	row := models.UploadFile{
		Name:         name,
		LiveKey:      "legacy:" + name,
		Size:         info.Size(),
		Mime:         mimeFromName(name),
		Kind:         "post",
		Legacy:       true,
		OriginalName: name,
		CreatedAt:    info.ModTime(),
	}
	if info.Size() <= maxBackfillFileBytes {
		if data, rerr := os.ReadFile(filepath.Join(dir, name)); rerr == nil {
			sum := sha256.Sum256(data)
			row.Sha256 = hex.EncodeToString(sum[:])
			if cfgImg, _, derr := image.DecodeConfig(bytes.NewReader(data)); derr == nil {
				row.Width = cfgImg.Width
				row.Height = cfgImg.Height
			}
		}
	}
	return db.Create(&row).Error
}
