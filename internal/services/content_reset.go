package services

import (
	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- wholesale content replacement -----------------------------------------
//
// Two operations replace the entire site at once: ImportBackup (an exported
// dump) and RestoreSnapshot (a VACUUM INTO file). They do NOT replace the same
// set of tables, and that difference is deliberate — see the doc comment on
// RestoreSnapshot for why the image library and the site configuration stay put
// when a snapshot is restored.
//
// The content tables, however, are the same five either way, and keeping one
// copy of that list per caller is how it drifted: the import cleared
// post_revisions and the snapshot restore did not. A restore therefore left
// revision rows pointing at posts it had just destroyed — and, because a
// snapshot is usually of this same site, the ids came back attached to
// articles whose text is now different, so every "restore this revision" button
// would write pre-restore content over restored content.
//
// Order is dependency order: join rows before the rows they join.
func clearContentTablesTx(tx *gorm.DB) error {
	steps := []struct {
		model    any
		unscoped bool // soft-delete-aware tables must be purged including trashed rows
	}{
		{&models.PostTag{}, true},
		{&models.Post{}, true},
		{&models.PostRevision{}, false},
		{&models.Tag{}, false},
		{&models.Category{}, false},
	}
	for _, st := range steps {
		q := tx
		if st.unscoped {
			q = tx.Unscoped()
		}
		if err := q.Where("1 = 1").Delete(st.model).Error; err != nil {
			return err
		}
	}

	// The post-level edges now describe articles that are gone. Dropped here
	// instead of being left to the caller's rebuild, so a third caller cannot
	// forget: an edge pointing at a missing post pins that file at
	// ref_count > 0 with nothing left to release it. Avatar and branding edges
	// stay — those slots belong to the instance and are handled with it.
	var orphaned []string
	if err := tx.Model(&models.UploadRef{}).Where("post_id <> ''").
		Pluck("upload_id", &orphaned).Error; err != nil {
		return err
	}
	if err := tx.Where("post_id <> ''").Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}
	return recountUploadRefsTx(tx, orphaned)
}
