package services

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"lopezpress/internal/models"
)

// --- ledger rebuild ---------------------------------------------------------
//
// Split out of media_refs.go: the rest of that file maintains the ledger
// incrementally (one post at a time, as authors save), while this rebuilds it
// from scratch for the operations that replace the posts table wholesale.

// rebuildBatchSize is how many edges go into one INSERT. SQLite caps the number
// of bound parameters per statement (traditionally 999, and 32766 in builds
// from 3.32), and each edge binds four, so 500 stays well clear of both while
// keeping the round-trip count low.
const rebuildBatchSize = 500

// rebuildUploadRefsTx rebuilds the entire image ledger from the posts table.
//
// Needed after a restore/import (R4-03). A restore rewrites `posts` wholesale
// but leaves `upload_refs` and `upload_files.ref_count` untouched, so the
// gallery keeps warning "deleting this image breaks 3 articles" about articles
// that no longer reference it — or, worse, reports 0 references for an image
// the restored content does embed, and the admin hard-deletes it and breaks the
// site. Rebuilding from the source of truth is the only correct answer; there is
// nothing to reconcile against.
//
// Soft-deleted posts are skipped: DeletePost releases their edges, so
// re-adding them would resurrect references for content the author removed.
//
// This is bulk on purpose. The obvious implementation — loop the posts and call
// syncPostUploadRefsTx on each — costs three statements per article (read the
// uploads, read the old edges, recount), so a 5,000-post site meant 15,000
// round trips inside one write transaction, holding the SQLite write lock for
// the whole time. Instead the whole mapping is read once and the edges are
// inserted in batches: four statements regardless of article count.
func rebuildUploadRefsTx(tx *gorm.DB) error {
	if err := tx.Where("1 = 1").Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}

	// name → id for every image in the library, read once.
	var uploads []struct {
		ID   string
		Name string
	}
	if err := tx.Model(&models.UploadFile{}).Select("id, name").Find(&uploads).Error; err != nil {
		return err
	}
	byName := make(map[string]string, len(uploads))
	for _, u := range uploads {
		byName[u.Name] = u.ID
	}

	var posts []struct {
		ID       string
		AuthorID string
		Body     string
	}
	if err := tx.Model(&models.Post{}).Select("id, author_id, body").Find(&posts).Error; err != nil {
		return err
	}

	// Build the edge set in memory. A post that embeds the same image twice
	// yields one edge, matching what syncPostUploadRefsTx does (the join has a
	// composite primary key, so a duplicate would be rejected anyway).
	refs := make([]models.UploadRef, 0, len(posts))
	for _, p := range posts {
		seen := make(map[string]bool, 4)
		for _, name := range extractUploadNames(p.Body) {
			id, ok := byName[name]
			if !ok || seen[id] {
				continue
			}
			seen[id] = true
			refs = append(refs, models.UploadRef{
				UploadID: id, PostID: p.ID, Kind: "body", UserID: p.AuthorID,
			})
		}
	}
	if len(refs) > 0 {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&refs, rebuildBatchSize).Error; err != nil {
			return err
		}
	}

	// Recompute every count from the ledger. The explicit `1 = 1` is not
	// decorative: GORM refuses a global UPDATE without a WHERE clause ("WHERE
	// conditions required"), and this one is global on purpose — every upload's
	// count comes from the ledger, including the ones that just lost their last
	// reference and would otherwise keep a stale number.
	return tx.Model(&models.UploadFile{}).Where("1 = 1").Update("ref_count",
		gorm.Expr("(SELECT COUNT(*) FROM upload_refs WHERE upload_refs.upload_id = upload_files.id)")).Error
}
