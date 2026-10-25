package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- backup import (restore) ---
//
// Split out of backup.go when that file crossed the 350-line gate. The export
// half lives there; everything that PUTS a dump back lives here, because the
// import is the destructive direction and deserves its own file, its own
// confirmation phrase, and its own gates.

// --- backup import (restore) ---

// ImportConfirmPhrase is the literal the client must echo back before an
// import is allowed to replace every table. See ImportData.Confirm.
const ImportConfirmPhrase = "REPLACE-ALL-DATA"

// RestoreConfirmPhrase is the literal the client must echo back before a
// snapshot restore replaces the database.
//
// It is deliberately a DIFFERENT phrase from ImportConfirmPhrase: the two
// operations read the same to a caller ("put this file back") but undo
// different things, and a phrase that travels with an exported backup file must
// not double as permission to overwrite a live site from a local snapshot.
const RestoreConfirmPhrase = "RESTORE-SNAPSHOT"

// validateImportPayload enforces the schema gate before any write.
// It returns a 400 ServiceError describing the first incompatibility found.
//
// Older dumps without the metadata fields are still accepted (the fields
// simply default to zero/empty and pass) so existing backups keep working —
// but a payload that cannot produce a working site is always rejected, no
// matter how old it claims to be. An empty object (`{}`) previously passed
// this check and wiped users/posts/categories/tags in one request; the checks
// below are what makes that impossible now.
func validateImportPayload(in ImportData) error {
	if in.SchemaVersion != 0 && in.SchemaVersion != BackupSchemaVersion {
		return newErr(http.StatusBadRequest, fmt.Sprintf(
			"backup schema_version %d is not supported (this app expects %d)", in.SchemaVersion, BackupSchemaVersion))
	}
	if in.AppVersion != "" && in.AppVersion != BackupAppVersion {
		return newErr(http.StatusBadRequest, fmt.Sprintf(
			"backup app_version %q does not match this app (%q); refusing to import", in.AppVersion, BackupAppVersion))
	}
	// A whole-database replace with no users leaves nobody able to log in, and
	// the import deletes every existing account first — an unrecoverable
	// lockout. Likewise a dump without the unique root would leave the site
	// with no account that can run the restore that undoes this.
	if len(in.Users) == 0 {
		return newErr(http.StatusBadRequest, "backup contains no users; refusing to replace the database")
	}
	roots := 0
	for _, u := range in.Users {
		if strings.TrimSpace(u.ID) == "" || strings.TrimSpace(u.Username) == "" || strings.TrimSpace(u.Email) == "" {
			return newErr(http.StatusBadRequest, "backup contains a user row without id/username/email")
		}
		if u.Role == models.RoleRoot {
			roots++
		}
	}
	if roots == 0 {
		return newErr(http.StatusBadRequest, "backup contains no root account; importing it would lock the site out")
	}
	return nil
}

// ImportBackup restores the database from an exported backup. When dryRun is
// true it only validates + reports the counts without touching the database.
// When applied, it first takes an automatic VACUUM INTO snapshot (spec #6,
// keep 3 most recent), runs the destructive replace inside a transaction, and
// returns the applied stats plus the snapshot file name.
//
// Root-only (as-built audit Q7): the payload is attacker-controllable JSON
// that can carry arbitrary user rows — including role "root" and foreign
// password hashes. Letting a plain admin import one would be a complete
// root-takeover path around the "root is never created via API" invariant,
// so the service refuses anything but the root, mirroring CreateUser's
// defence-in-depth (the handler enforces the same gate).
func (s *Service) ImportBackup(ctx context.Context, actor models.User, in ImportData, dryRun bool) (ImportResult, error) {
	if !IsRoot(actor) {
		return ImportResult{}, newErr(http.StatusForbidden, "only the root can import a backup")
	}
	// Spec #6: validate schema compatibility before doing anything.
	if err := validateImportPayload(in); err != nil {
		return ImportResult{}, err
	}
	if dryRun {
		// Preview is non-destructive: report the counts the payload would
		// produce and let the caller ask for explicit confirmation.
		stats := BackupStats{
			Users:       int64(len(in.Users)),
			Posts:       int64(len(in.Posts)),
			Categories:  int64(len(in.Categories)),
			Tags:        int64(len(in.Tags)),
			Revisions:   int64(len(in.Revisions)),
			UploadFiles: int64(len(in.UploadFiles)),
		}
		// A preview is a privileged read of a whole-database dump, and the
		// handler used to audit it; keep that trail now that the write lives
		// here (P2: audit moved with the operation, not left behind).
		s.audit(ctx, actor, "import_backup", "backup", "dry_run=true")
		return ImportResult{DryRun: true, Applied: false, Stats: stats}, nil
	}
	// Applying replaces every table, so it needs an explicit confirmation in
	// the payload. Without this an accidental request (or a truncated file
	// that still decodes) is enough to destroy the site.
	if in.Confirm != ImportConfirmPhrase {
		return ImportResult{}, newErr(http.StatusBadRequest,
			"refusing to replace the database without confirmation: preview with ?dry_run=1, then resend the same payload with confirm=\""+ImportConfirmPhrase+"\"")
	}

	stats := BackupStats{
		Users:       int64(len(in.Users)),
		Posts:       int64(len(in.Posts)),
		Categories:  int64(len(in.Categories)),
		Tags:        int64(len(in.Tags)),
		Revisions:   int64(len(in.Revisions)),
		UploadFiles: int64(len(in.UploadFiles)),
	}

	// Auto snapshot before the destructive replace (VACUUM INTO is atomic
	// and file-level, so it is safe to run outside the import transaction).
	snapshotFile, err := s.snapshotBeforeImport(ctx)
	if err != nil {
		return ImportResult{}, newErr(http.StatusInternalServerError, "failed to create pre-import snapshot: "+err.Error())
	}

	// The live token ceiling must be read BEFORE the transaction rewrites the
	// users table, because the rewrite destroys it (see token_epoch.go).
	liveMax := s.maxTokenVersion(ctx)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear existing rows. Unscoped ensures soft-deleted rows go too.
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.PostTag{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.Post{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.PostRevision{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Tag{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.Category{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&models.UploadFile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.SiteConfig{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&models.User{}).Error; err != nil {
			return err
		}

		// Users (with re-applied password hashes).
		for i := range in.Users {
			u := in.Users[i]
			if h, ok := in.PasswordHashes[u.ID]; ok {
				u.PasswordHash = h
			}
			if err := tx.Create(&u).Error; err != nil {
				return err
			}
		}
		// Categories.
		for i := range in.Categories {
			if err := tx.Create(&in.Categories[i]).Error; err != nil {
				return err
			}
		}
		// Tags.
		for i := range in.Tags {
			if err := tx.Create(&in.Tags[i]).Error; err != nil {
				return err
			}
		}
		// Posts (Unscoped create so DeletedAt is respected as-is).
		for i := range in.Posts {
			if err := tx.Unscoped().Create(&in.Posts[i]).Error; err != nil {
				return err
			}
		}
		// Join table.
		for i := range in.PostTags {
			if err := tx.Create(&in.PostTags[i]).Error; err != nil {
				return err
			}
		}
		// Revision history. It references posts and users, both already present.
		for i := range in.Revisions {
			if err := tx.Select("*").Create(&in.Revisions[i]).Error; err != nil {
				return err
			}
		}
		// The image library. LiveKey is json:"-" on the model, so it travels in
		// the separate upload_livekeys map; it is UNIQUE and non-null, so an
		// empty value would collide on the second row.
		//
		// Select("*") matters here and below: several columns carry a `default`
		// GORM tag, and without it a value equal to the default (Kind "post",
		// Legacy false, a zero RefCount) is silently dropped and replaced by the
		// default — which is wrong for a restore whose whole point is fidelity.
		for i := range in.UploadFiles {
			u := in.UploadFiles[i]
			if lk, ok := in.UploadLiveKeys[u.Name]; ok {
				u.LiveKey = lk
			}
			if err := tx.Select("*").Unscoped().Create(&u).Error; err != nil {
				return err
			}
		}
		// Site configuration (a single row, id=1).
		//
		// This is the one table that must NOT go through a struct Create. Several
		// columns carry a `default` GORM tag, and GORM treats a zero value as
		// "unset — use the default": PublicRead=false (the zero value) would come
		// back as true (default:true), and a quota deliberately set to "0 =
		// unlimited" would come back as 5 MB. A map is the only shape GORM writes
		// verbatim, so every column is listed explicitly.
		for i := range in.SiteConfigs {
			c := in.SiteConfigs[i]
			if err := tx.Model(&models.SiteConfig{}).Create(map[string]any{
				"id":                       c.ID,
				"site_name":                c.SiteName,
				"site_title":               c.SiteTitle,
				"site_logo":                c.SiteLogo,
				"site_favicon":             c.SiteFavicon,
				"site_url":                 c.SiteURL,
				"footer_text":              c.FooterText,
				"icp_no":                   c.IcpNo,
				"friend_links":             c.FriendLinks,
				"img_host_mode":            c.ImgHostMode,
				"img_host_token":           c.ImgHostToken,
				"allow_register":           c.AllowRegister,
				"page_size":                c.PageSize,
				"public_read":              c.PublicRead,
				"auto_backup":              c.AutoBackup,
				"backup_keep":              c.BackupKeep,
				"smtp_host":                c.SmtpHost,
				"smtp_port":                c.SmtpPort,
				"smtp_user":                c.SmtpUser,
				"smtp_pass":                c.SmtpPass,
				"upload_max_image":         c.UploadMaxImage,
				"upload_max_avatar":        c.UploadMaxAvatar,
				"user_quota_bytes":         c.UserQuotaBytes,
				"site_quota_bytes":         c.SiteQuotaBytes,
				"allow_user_delete_upload": c.AllowUserDeleteUpload,
				"allow_user_change_avatar": c.AllowUserChangeAvatar,
				"allow_external_avatar":    c.AllowExternalAvatar,
				"email_verify_enabled":     c.EmailVerifyEnabled,
				"email_code_ttl_min":       c.EmailCodeTTLMin,
				"email_code_cooldown":      c.EmailCodeCooldown,
				"updated_at":               c.UpdatedAt,
			}).Error; err != nil {
				return err
			}
		}
		// R4-01: the dump carries the token_version values from when it was
		// taken, so restoring it would resurrect tokens revoked after that point
		// — and a plain "+1" lands exactly on the version of a token issued
		// between the dump and the operator's logout-all. Land everyone above
		// the pre-import ceiling instead (see token_epoch.go).
		if err := raiseAllTokenVersions(tx, tokenRevokeFloor(liveMax)); err != nil {
			return err
		}
		// R4-03: the posts table just changed wholesale, so the image ledger is
		// stale in both directions — it still claims references for articles the
		// import replaced, and misses references the imported bodies introduce.
		// A stale count is not cosmetic: the gallery's "deleting this breaks 3
		// articles" warning is the admin's only guard before a hard delete.
		return rebuildUploadRefsTx(tx)
	})
	if err != nil {
		return ImportResult{}, newErr(http.StatusInternalServerError, "failed to import backup")
	}
	s.audit(ctx, actor, "import_backup", "backup", "dry_run=false; snapshot="+snapshotFile)
	return ImportResult{DryRun: false, Applied: true, Stats: stats, SnapshotFile: snapshotFile}, nil
}
