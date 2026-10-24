package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- data backup ---

// ExportBackup serialises every table to a timestamped JSON file under the
// configured backup directory and returns the file path + size + row counts.
func (s *Service) ExportBackup(ctx context.Context) (BackupResult, error) {
	db := s.db.WithContext(ctx)

	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read users")
	}
	var categories []models.Category
	if err := db.Find(&categories).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read categories")
	}
	var tags []models.Tag
	if err := db.Find(&tags).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read tags")
	}
	// Backup includes soft-deleted posts so a dump can restore them.
	var posts []models.Post
	if err := db.Unscoped().Find(&posts).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read posts")
	}
	// The users table carries the password hash; models.User marks it json:"-"
	// so it is exported separately here to keep a backup restorable (import
	// re-applies the hashes keyed by user id).
	var pwRows []struct {
		ID           string
		PasswordHash string
	}
	if err := db.Model(&models.User{}).Select("id, password_hash").Scan(&pwRows).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read user hashes")
	}
	pwHashes := make(map[string]string, len(pwRows))
	for _, r := range pwRows {
		pwHashes[r.ID] = r.PasswordHash
	}
	// Also dump the many-to-many join table so tag associations round-trip.
	var postTags []models.PostTag
	if err := db.Find(&postTags).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read post_tags")
	}

	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "cannot create backup directory")
	}

	now := time.Now()
	filename := fmt.Sprintf("backup-%s.json", now.UTC().Format("20060102-150405"))
	full := filepath.Join(dir, filename)

	out, err := os.Create(full)
	if err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "cannot write backup file")
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	payload := map[string]any{
		// Spec #6: top-level metadata so importers can validate schema/app
		// compatibility before applying a destructive restore.
		"schema_version": BackupSchemaVersion,
		"app_version":    BackupAppVersion,
		"exported_at":    now.UTC().Format(time.RFC3339),
		// Security note travels inside the file so a leaked/recovered dump
		// still warns the reader that it embeds bcrypt password hashes.
		"security_note":   "This backup contains bcrypt password hashes. Anyone with the file can attempt offline cracking, so store it like a secrets file: restrict access, never commit to version control or share publicly.",
		"users":           users,
		"password_hashes": pwHashes,
		"categories":      categories,
		"tags":            tags,
		"posts":           posts,
		"post_tags":       postTags,
	}
	if err := enc.Encode(payload); err != nil {
		_ = os.Remove(full)
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to write backup")
	}

	info, err := out.Stat()
	if err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read backup size")
	}

	s.audit(ctx, models.User{}, "export_backup", "backup:"+filename, "size="+strconv.FormatInt(info.Size(), 10))
	return BackupResult{
		Filename: filename,
		Size:     info.Size(),
		Stats: BackupStats{
			Users:      int64(len(users)),
			Posts:      int64(len(posts)),
			Categories: int64(len(categories)),
			Tags:       int64(len(tags)),
		},
		Warning: "The backup file contains bcrypt password hashes, equivalent to an offline cracking target for the plaintext passwords. Store it like a secret key: restrict access, never commit it to a code repository or share it publicly.",
	}, nil
}

// backupDir returns the configured backup directory.
func (s *Service) backupDir() string {
	if s.cfg != nil && s.cfg.BackupDir != "" {
		return s.cfg.BackupDir
	}
	return "data/backups"
}

// --- backup import (restore) ---

// ImportConfirmPhrase is the literal the client must echo back before an
// import is allowed to replace every table. See ImportData.Confirm.
const ImportConfirmPhrase = "REPLACE-ALL-DATA"

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
			Users:      int64(len(in.Users)),
			Posts:      int64(len(in.Posts)),
			Categories: int64(len(in.Categories)),
			Tags:       int64(len(in.Tags)),
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
		Users:      int64(len(in.Users)),
		Posts:      int64(len(in.Posts)),
		Categories: int64(len(in.Categories)),
		Tags:       int64(len(in.Tags)),
	}

	// Auto snapshot before the destructive replace (VACUUM INTO is atomic
	// and file-level, so it is safe to run outside the import transaction).
	snapshotFile, err := s.snapshotBeforeImport(ctx)
	if err != nil {
		return ImportResult{}, newErr(http.StatusInternalServerError, "failed to create pre-import snapshot: "+err.Error())
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Clear existing rows. Unscoped ensures soft-deleted posts go too.
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
		// A restore brings back the token_version values captured in the dump,
		// which would silently re-validate JWTs that had already been revoked
		// (the exact side effect seen after the 2026-10-24 wipe-and-restore).
		// Bump every account so no pre-restore token survives; the operator
		// signs in again with the restored credentials.
		if err := tx.Model(&models.User{}).Where("1 = 1").
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return ImportResult{}, newErr(http.StatusInternalServerError, "failed to import backup")
	}
	s.audit(ctx, actor, "import_backup", "backup", "dry_run=false; snapshot="+snapshotFile)
	return ImportResult{DryRun: false, Applied: true, Stats: stats, SnapshotFile: snapshotFile}, nil
}

// snapshotBeforeImport runs VACUUM INTO to write a file-level copy of the
// current database into the backup dir, then prunes so only the 3 most
// recent snapshots remain. It returns the new snapshot's base file name.
func (s *Service) snapshotBeforeImport(ctx context.Context) (string, error) {
	dir := s.backupDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// VACUUM INTO writes a full, consistent copy of the live DB file.
	now := time.Now()
	target := filepath.Join(dir, fmt.Sprintf("snapshot-%s.db", now.UTC().Format("20060102-150405")))
	if err := s.db.Exec("VACUUM INTO ?", target).Error; err != nil {
		return "", err
	}
	// Keep only the 3 most recent snapshots.
	s.pruneSnapshots(dir, 3)
	base := filepath.Base(target)
	return base, nil
}

// isSnapshotName reports whether a file in the backup dir is a restorable
// database copy: either a pre-import snapshot or a daily auto snapshot.
func isSnapshotName(name string) bool {
	if !strings.HasSuffix(name, ".db") {
		return false
	}
	return strings.HasPrefix(name, "snapshot-") || strings.HasPrefix(name, autoBackupPrefix)
}

// pruneSnapshots removes oldest snapshot-*.db files beyond keep.
func (s *Service) pruneSnapshots(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var snaps []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "snapshot-") && strings.HasSuffix(e.Name(), ".db") {
			snaps = append(snaps, e.Name())
		}
	}
	if len(snaps) <= keep {
		return
	}
	// File names embed a monotonically increasing UTC timestamp, so a
	// lexicographic sort is chronological. Remove the oldest (len-keep).
	for _, old := range snaps[:len(snaps)-keep] {
		_ = os.Remove(filepath.Join(dir, old))
	}
}
