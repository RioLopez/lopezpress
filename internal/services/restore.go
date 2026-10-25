package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/models"
)

// RestoreSnapshot re-imports a previously created VACUUM INTO snapshot file
// (spec #6: /api/backup/restore-snapshot). The snapshot is a raw .db file, so
// restoring means copying it over the live DB file after a clean close. For
// SQLite this is done by: 1) VACUUM INTO the live db to a temp, 2) the
// snapshot is already a full consistent file so we just point the app at the
// next restart — but to restore IN-PROCESS we instead delete all rows and
// re-insert from the snapshot's JSON? No: a VACUUM INTO snapshot is binary.
//
// The simplest correct approach: the handler reads the snapshot .db, and the
// service performs the destructive replace by opening the snapshot as a
// second SQLite handle and copying table contents row-by-row into the live
// DB inside one transaction. This keeps everything in-process and atomic.
//
// Root-only (as-built audit Q7): restoring replaces the whole database
// including the users table, so it shares ImportBackup's takeover risk and
// is gated to the single root at the service layer.
func (s *Service) RestoreSnapshot(ctx context.Context, actor models.User, filename string, opt RestoreSnapshotOptions) (BackupStats, error) {
	if !IsRoot(actor) {
		return BackupStats{}, newErr(http.StatusForbidden, "only the root can restore a snapshot")
	}
	name := filepath.Base(filename) // reject any path traversal
	if name == "" || name == "." || name == ".." {
		return BackupStats{}, newErr(http.StatusBadRequest, "invalid snapshot name")
	}
	snapPath := filepath.Join(s.backupDir(), name)
	// Spec 34: both the pre-import snapshots and the daily auto snapshots are
	// restorable — they are written by the same VACUUM INTO path.
	if !isSnapshotName(name) {
		return BackupStats{}, newErr(http.StatusBadRequest, "only snapshot-*.db / auto-*.db files can be restored")
	}
	if _, err := os.Stat(snapPath); err != nil {
		if os.IsNotExist(err) {
			return BackupStats{}, newErr(http.StatusNotFound, "snapshot not found")
		}
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read snapshot: "+err.Error())
	}

	// Open the snapshot as a read-only second handle.
	snapDB, err := gorm.Open(sqlite.Open(snapPath+"?mode=ro"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot open snapshot: "+err.Error())
	}
	defer func() {
		if sql, e := snapDB.DB(); e == nil {
			_ = sql.Close()
		}
	}()

	// Count what the snapshot holds BEFORE anything is destroyed. Three gates
	// hang off these numbers, and all three exist because a snapshot restore is
	// the most destructive single request the app exposes: it replaces the
	// whole content set, plus every account and password hash, in one call.
	var snapStats BackupStats
	if err := snapDB.Model(&models.User{}).Count(&snapStats.Users).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read snapshot users: "+err.Error())
	}
	if err := snapDB.Unscoped().Model(&models.Post{}).Count(&snapStats.Posts).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read snapshot posts: "+err.Error())
	}
	if err := snapDB.Model(&models.Category{}).Count(&snapStats.Categories).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read snapshot categories: "+err.Error())
	}
	if err := snapDB.Model(&models.Tag{}).Count(&snapStats.Tags).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read snapshot tags: "+err.Error())
	}

	// A dry run stops here, BEFORE every gate. The preview's whole job is to
	// report what a snapshot holds — including the dangerous cases — so the
	// operator can see "0 users" or "0 articles" and decide. It needs no
	// confirmation phrase and passes no gate: it changes nothing, so there is
	// nothing to protect.
	if opt.DryRun {
		return snapStats, nil
	}

	// Gate 1 — the confirmation phrase. Import has had this since spec #6; the
	// snapshot restore, which is strictly more destructive (it also replaces
	// the account table), did not. A single POST against a stale or empty
	// snapshot replaced a live site with an empty one.
	if opt.Confirm != RestoreConfirmPhrase {
		return BackupStats{}, newErr(http.StatusBadRequest,
			"refusing to restore without confirmation: preview with ?dry_run=1, then resend with confirm=\""+RestoreConfirmPhrase+"\"")
	}

	// Gate 2 — a snapshot with no accounts cannot describe a usable site. There
	// is no recovery from it: the root account that could restore something else
	// is gone with it.
	if snapStats.Users == 0 {
		return BackupStats{}, newErr(http.StatusBadRequest,
			"this snapshot contains no users, so restoring it would leave the site with no way to log in; refusing")
	}

	// Gate 3 — the case that actually happened: an empty (or nearly empty)
	// snapshot silently replaced a populated site. An operator restoring a
	// genuinely older snapshot may well have FEWER posts, so the rule is not
	// "never fewer" — it is "not everything". Deleting all content requires
	// saying so explicitly.
	var livePosts int64
	if err := s.db.WithContext(ctx).Unscoped().Model(&models.Post{}).Count(&livePosts).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "cannot read the live post count")
	}
	if snapStats.Posts == 0 && livePosts > 0 && !opt.AllowDataLoss {
		return BackupStats{}, newErr(http.StatusConflict,
			"this snapshot has no posts but the site has "+itoa(int(livePosts))+
				"; restoring it would delete every article. Resend with allow_data_loss=true if that is really intended")
	}

	// Auto snapshot the current state first (so the operator can undo this
	// restore by restoring the older snapshot).
	if _, err := s.snapshotBeforeImport(ctx); err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to snapshot before restore: "+err.Error())
	}

	// The live token ceiling must be read BEFORE the transaction rewrites the
	// users table, because the rewrite destroys it (see token_epoch.go).
	liveMax := s.maxTokenVersion(ctx)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear live tables in dependency order.
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.Post{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Category{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.User{}).Error; err != nil {
			return err
		}
		// Copy users (+ password hashes) from the snapshot.
		var users []models.User
		if err := snapDB.Find(&users).Error; err != nil {
			return err
		}
		var pwRows []struct {
			ID           string
			PasswordHash string
		}
		if err := snapDB.Model(&models.User{}).Select("id, password_hash").Scan(&pwRows).Error; err != nil {
			return err
		}
		pwMap := make(map[string]string, len(pwRows))
		for _, r := range pwRows {
			pwMap[r.ID] = r.PasswordHash
		}
		for i := range users {
			if h, ok := pwMap[users[i].ID]; ok {
				users[i].PasswordHash = h
			}
			if err := tx.Create(&users[i]).Error; err != nil {
				return err
			}
		}
		var cats []models.Category
		if err := snapDB.Find(&cats).Error; err != nil {
			return err
		}
		for i := range cats {
			if err := tx.Create(&cats[i]).Error; err != nil {
				return err
			}
		}
		var tags []models.Tag
		if err := snapDB.Find(&tags).Error; err != nil {
			return err
		}
		for i := range tags {
			if err := tx.Create(&tags[i]).Error; err != nil {
				return err
			}
		}
		var posts []models.Post
		if err := snapDB.Unscoped().Find(&posts).Error; err != nil {
			return err
		}
		for i := range posts {
			if err := tx.Unscoped().Create(&posts[i]).Error; err != nil {
				return err
			}
		}
		var pts []models.PostTag
		if err := snapDB.Find(&pts).Error; err != nil {
			return err
		}
		for i := range pts {
			if err := tx.Create(&pts[i]).Error; err != nil {
				return err
			}
		}
		// R4-01: the snapshot carries the token_version values from when it was
		// taken, so restoring it would resurrect tokens revoked after that point
		// — and a plain "+1" lands exactly on the version of a token issued
		// between the snapshot and the operator's logout-all. Land everyone above
		// the pre-restore ceiling instead (see token_epoch.go).
		if err := raiseAllTokenVersions(tx, tokenRevokeFloor(liveMax)); err != nil {
			return err
		}
		// R4-03: the image ledger lives outside the snapshot, so it now
		// describes the pre-restore content. Rebuild it from the restored posts
		// or the gallery will warn about the wrong articles — and clear the
		// warning for images the restored content actually uses.
		return rebuildUploadRefsTx(tx)
	})
	if err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to restore snapshot")
	}
	// Return the post-restore counts.
	var users, posts, cats, tags int64
	// Best-effort: these counts feed the response summary. If any single
	// Count fails we surface the error rather than silently returning zeros
	// (which would look like an empty database and mask a real problem).
	if err := s.db.Model(&models.User{}).Count(&users).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count users after restore")
	}
	if err := s.db.Model(&models.Post{}).Count(&posts).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count posts after restore")
	}
	if err := s.db.Model(&models.Category{}).Count(&cats).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count categories after restore")
	}
	if err := s.db.Model(&models.Tag{}).Count(&tags).Error; err != nil {
		return BackupStats{}, newErr(http.StatusInternalServerError, "failed to count tags after restore")
	}
	s.audit(ctx, actor, "restore_snapshot", "snapshot:"+name, "ok")
	return BackupStats{Users: users, Posts: posts, Categories: cats, Tags: tags}, nil
}

// ListSnapshots returns the available snapshot-*.db files, newest first.
// Root-only: a snapshot is a whole-database artifact (it contains every
// account and password hash), so listing it is gated exactly like restoring
// it (as-built audit Q7). The frontend restore UI (P2-14) needs this to
// offer anything to restore.
func (s *Service) ListSnapshots(ctx context.Context, actor models.User) ([]SnapshotInfo, error) {
	if !IsRoot(actor) {
		return nil, newErr(http.StatusForbidden, "only the root can list snapshots")
	}
	entries, err := os.ReadDir(s.backupDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotInfo{}, nil // no backup dir yet -> no snapshots
		}
		return nil, newErr(http.StatusInternalServerError, "failed to read backup directory")
	}
	out := []SnapshotInfo{}
	for _, e := range entries {
		if e.IsDir() || !isSnapshotName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // transient stat failure: skip the file, keep the list usable
		}
		out = append(out, SnapshotInfo{Name: e.Name(), Size: info.Size(), ModTime: info.ModTime()})
	}
	// Newest first. File names embed a UTC timestamp, but sorting by mtime is
	// authoritative regardless of naming.
	slices.SortFunc(out, func(a, b SnapshotInfo) int {
		return b.ModTime.Compare(a.ModTime)
	})
	return out, nil
}
