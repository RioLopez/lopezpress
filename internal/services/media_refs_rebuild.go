package services

import (
	"errors"

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

// rebuildUploadRefsTx rebuilds the entire image ledger from the tables that
// actually hold the references.
//
// Needed after a restore/import: those rewrite `posts` (and often users and
// site_configs) wholesale but leave `upload_refs` and `upload_files.ref_count`
// untouched, so the gallery keeps warning "deleting this image breaks 3
// articles" about articles that no longer reference it — or, worse, reports 0
// references for an image the restored content does embed, and the admin
// hard-deletes it and breaks the site. Rebuilding from the source of truth is
// the only correct answer; there is nothing to reconcile against.
//
// "Source of truth" is FOUR places, not one: post bodies, post covers, user
// avatars, and the two site branding slots. The ledger is what makes an image
// look used, so a rebuild that only reads bodies silently drops the other three
// and turns every avatar and the site logo into a delete-able orphan.
//
// Soft-deleted posts are skipped: DeletePost releases their edges, so
// re-adding them would resurrect references for content the author removed.
// RestoreTrash re-establishes the edges for the one post it brings back.
//
// This is bulk on purpose. The obvious implementation — loop the posts and call
// syncPostUploadRefsTx on each — costs three statements per article (read the
// uploads, read the old edges, recount), so a 5,000-post site meant 15,000
// round trips inside one write transaction, holding the SQLite write lock for
// the whole time. Instead the whole mapping is read once and the edges are
// inserted in batches: a fixed number of statements regardless of article count.
func rebuildUploadRefsTx(tx *gorm.DB) error {
	if err := tx.Where("1 = 1").Delete(&models.UploadRef{}).Error; err != nil {
		return err
	}
	byName, err := libraryNameIndexTx(tx)
	if err != nil {
		return err
	}

	postRefs, err := postRefsTx(tx, byName)
	if err != nil {
		return err
	}
	avatarRefs, err := avatarRefsTx(tx, byName)
	if err != nil {
		return err
	}
	siteRefs, err := siteRefsTx(tx, byName)
	if err != nil {
		return err
	}
	refs := make([]models.UploadRef, 0, len(postRefs)+len(avatarRefs)+len(siteRefs))
	refs = append(refs, postRefs...)
	refs = append(refs, avatarRefs...)
	refs = append(refs, siteRefs...)

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

// libraryNameIndexTx returns name → id for every image in the library, read
// once. Soft-deleted rows are included: UploadFile.DeletedAt is a plain
// *time.Time, not gorm.DeletedAt, so GORM does not filter them out — and it
// should not, because an article that already embeds a withdrawn image still
// depends on that file being served.
func libraryNameIndexTx(tx *gorm.DB) (map[string]string, error) {
	var uploads []struct {
		ID   string
		Name string
	}
	if err := tx.Model(&models.UploadFile{}).Select("id, name").Find(&uploads).Error; err != nil {
		return nil, err
	}
	byName := make(map[string]string, len(uploads))
	for _, u := range uploads {
		byName[u.Name] = u.ID
	}
	return byName, nil
}

// postRefsTx collects the body + cover edges. A post that mentions the same
// image twice yields one edge, matching what syncPostUploadRefsTx writes (the
// join has a composite primary key, so a duplicate would be rejected anyway).
func postRefsTx(tx *gorm.DB, byName map[string]string) ([]models.UploadRef, error) {
	var posts []struct {
		ID       string
		AuthorID string
		Body     string
		Cover    string
	}
	if err := tx.Model(&models.Post{}).Select("id, author_id, body, cover").Find(&posts).Error; err != nil {
		return nil, err
	}
	refs := make([]models.UploadRef, 0, len(posts))
	for _, p := range posts {
		seen := make(map[string]bool, 4)
		for _, name := range extractUploadRefNames(p.Body, p.Cover) {
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
	return refs, nil
}

// avatarRefsTx collects one edge per user whose avatar lives in the library. An
// external avatar URL resolves to no row and is skipped.
func avatarRefsTx(tx *gorm.DB, byName map[string]string) ([]models.UploadRef, error) {
	var users []struct {
		ID     string
		Avatar string
	}
	if err := tx.Model(&models.User{}).Select("id, avatar").Find(&users).Error; err != nil {
		return nil, err
	}
	refs := make([]models.UploadRef, 0, len(users))
	for _, u := range users {
		if id, ok := byName[uploadNameFromURL(u.Avatar)]; ok {
			refs = append(refs, models.UploadRef{UploadID: id, Kind: "avatar", UserID: u.ID})
		}
	}
	return refs, nil
}

// siteRefsTx collects the logo and favicon edges. site_configs is a single-row
// table; a missing row is normal on a fresh database and means nothing is
// branded yet.
func siteRefsTx(tx *gorm.DB, byName map[string]string) ([]models.UploadRef, error) {
	var cfg struct {
		SiteLogo    string
		SiteFavicon string
	}
	err := tx.Model(&models.SiteConfig{}).Select("site_logo, site_favicon").
		Where("id = ?", 1).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	refs := make([]models.UploadRef, 0, 2)
	if id, ok := byName[uploadNameFromURL(cfg.SiteLogo)]; ok {
		refs = append(refs, models.UploadRef{UploadID: id, Kind: siteRefLogo})
	}
	if id, ok := byName[uploadNameFromURL(cfg.SiteFavicon)]; ok {
		refs = append(refs, models.UploadRef{UploadID: id, Kind: siteRefFavicon})
	}
	return refs, nil
}
