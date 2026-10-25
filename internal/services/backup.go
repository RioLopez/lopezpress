package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"


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
	// The three tables below are what make the dump a *backup* and not just a
	// content migration. Each was silently omitted once: the site configuration
	// (name, SMTP, quotas), the revision history, and the image library. A dump
	// without them cannot rebuild the site it was taken from.
	var siteConfigs []models.SiteConfig
	if err := db.Find(&siteConfigs).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read site_configs")
	}
	var revisions []models.PostRevision
	if err := db.Find(&revisions).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read revisions")
	}
	var uploadFiles []models.UploadFile
	if err := db.Unscoped().Find(&uploadFiles).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read upload_files")
	}
	// LiveKey is json:"-" on the model (an internal de-dup key), but it is
	// covered by a UNIQUE index, so it has to round-trip or every imported row
	// collides on the empty string. Carry it beside the rows, keyed by name.
	var liveKeys []struct {
		Name    string
		LiveKey string
	}
	if err := db.Unscoped().Model(&models.UploadFile{}).
		Select("name, live_key").Scan(&liveKeys).Error; err != nil {
		return BackupResult{}, newErr(http.StatusInternalServerError, "failed to read upload live_keys")
	}
	uploadLiveKeys := make(map[string]string, len(liveKeys))
	for _, r := range liveKeys {
		uploadLiveKeys[r.Name] = r.LiveKey
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
		"site_configs":    siteConfigs,
		"revisions":       revisions,
		"upload_files":    uploadFiles,
		"upload_livekeys": uploadLiveKeys,
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
			Users:       int64(len(users)),
			Posts:       int64(len(posts)),
			Categories:  int64(len(categories)),
			Tags:        int64(len(tags)),
			Revisions:   int64(len(revisions)),
			UploadFiles: int64(len(uploadFiles)),
		},
		Warning: "The backup file contains bcrypt password hashes, equivalent to an offline cracking target for the plaintext passwords. Store it like a secret key: restrict access, never commit it to a code repository or share it publicly. It also does NOT contain the image files themselves: the bytes live on disk under data/uploads and must be backed up separately (rsync/copy that directory).",
	}, nil
}

// backupDir returns the configured backup directory.
func (s *Service) backupDir() string {
	if s.cfg != nil && s.cfg.BackupDir != "" {
		return s.cfg.BackupDir
	}
	return "data/backups"
}

// snapshotBeforeImport runs VACUUM INTO to write a file-level copy of the
// current database into the backup dir, then prunes so only the 3 most
// recent snapshots remain. It returns the new snapshot's base file name.
