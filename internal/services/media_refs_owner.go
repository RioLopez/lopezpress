package services

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lopezpress/internal/models"
)

// --- ledger edges for non-article slots -------------------------------------
//
// Everything here answers the same question for a slot that is not an article
// body: "which library image is this thing currently displaying?" There are
// three such slots — a user avatar and the two site branding fields — and they
// all update the same way, so they share one writer.

// The site branding slots are distinct kinds on purpose: upload_refs is unique
// on (upload_id, post_id, kind, user_id) and both have an empty post_id and
// user_id, so a shared kind would let the logo edge overwrite the favicon one.
const (
	siteRefLogo    = "site_logo"
	siteRefFavicon = "site_favicon"
)

// setSlotUploadRefTx points one non-article slot at url; an empty url clears it.
//
// Only URLs that resolve to a library row are tracked: an external avatar has
// nothing on disk to protect. The delete-then-insert only runs when the target
// actually changed, so saving a settings form without touching the logo writes
// nothing.
//
// Without these edges the branded images are invisible to the ledger, so a logo
// picked from the library reads as ref_count = 0, the gallery counts it as an
// orphan, and an operator clearing "unused" images deletes the file every page
// header points at. The failure is silent: no error, no failed request, just a
// broken image on every page and a dead URL left behind in site_config.
func setSlotUploadRefTx(tx *gorm.DB, kind, userID, url string) error {
	var oldIDs []string
	if err := tx.Model(&models.UploadRef{}).
		Where("kind = ? AND user_id = ?", kind, userID).Pluck("upload_id", &oldIDs).Error; err != nil {
		return err
	}
	var newID string
	if name := uploadNameFromURL(url); name != "" {
		var row models.UploadFile
		if err := tx.Select("id").Where("name = ?", name).First(&row).Error; err == nil {
			newID = row.ID
		}
	}
	if len(oldIDs) == 1 && newID == oldIDs[0] {
		return nil // already correct, nothing to write
	}
	if err := tx.Where("kind = ? AND user_id = ?", kind, userID).Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}
	if newID != "" {
		ref := models.UploadRef{UploadID: newID, Kind: kind, UserID: userID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ref).Error; err != nil {
			return err
		}
	}
	return recountUploadRefsTx(tx, append(oldIDs, newID))
}

// uploadNameFromURL extracts the file name from an /uploads/<name> URL and
// returns "" for anything else (external URLs, empty strings).
func uploadNameFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	const prefix = "/uploads/"
	if !strings.HasPrefix(raw, prefix) {
		return ""
	}
	name := raw[len(prefix):]
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return ""
	}
	return name
}
