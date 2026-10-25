package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"lopezpress/internal/models"
)

// The three tables a backup silently dropped — site_configs, revisions,
// upload_files — are what separate a *backup* (restore the site you had) from a
// *content migration* (restore some articles). This is a full export→import
// round trip: take a site with a non-default config, an edit history, and an
// image library, dump it, wreck those tables, restore, and prove all three came
// back.
func TestBackupRoundTripsConfigRevisionsAndLibrary(t *testing.T) {
	svc, db := newFileTestService(t)
	svc.cfg.BackupDir = t.TempDir()
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)

	// 1. A non-default site configuration.
	if err := db.Model(&models.SiteConfig{}).Where("id = 1").Updates(map[string]any{
		"site_name":   "RoundTrip Blog",
		"public_read": false,
	}).Error; err != nil {
		t.Fatalf("set site config: %v", err)
	}

	// 2. A post with an edit, so there is a revision.
	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "v1", Body: "one"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Body: "two"}); err != nil {
		t.Fatalf("edit: %v", err)
	}

	// 3. An image library row (with a live key — the de-dup key that must
	// round-trip even though it is json:"-" on the model).
	if err := db.Create(&models.UploadFile{
		ID: "u-rt", Name: "pic.png", LiveKey: "sha-rt", OwnerID: author.ID,
		Size: 10, Sha256: "sha-rt", Kind: "post", ThumbStatus: ThumbStatusSkip,
	}).Error; err != nil {
		t.Fatalf("create upload: %v", err)
	}

	// Export, then load the dump the way an import actually receives it.
	res, err := svc.ExportBackup(ctx)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(svc.cfg.BackupDir, res.Filename))
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	var in ImportData
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatalf("decode dump: %v", err)
	}
	if len(in.SiteConfigs) == 0 || len(in.Revisions) == 0 || len(in.UploadFiles) == 0 {
		t.Fatalf("the dump must carry all three tables: configs=%d revisions=%d uploads=%d",
			len(in.SiteConfigs), len(in.Revisions), len(in.UploadFiles))
	}
	in.Confirm = ImportConfirmPhrase

	// Wreck the three tables to simulate restoring onto a bare site.
	if err := db.Where("1 = 1").Delete(&models.SiteConfig{}).Error; err != nil {
		t.Fatalf("wreck config: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&models.PostRevision{}).Error; err != nil {
		t.Fatalf("wreck revisions: %v", err)
	}
	if err := db.Unscoped().Where("1 = 1").Delete(&models.UploadFile{}).Error; err != nil {
		t.Fatalf("wreck uploads: %v", err)
	}

	if _, err := svc.ImportBackup(ctx, root, in, false); err != nil {
		t.Fatalf("import: %v", err)
	}

	// Configuration came back.
	var cfg models.SiteConfig
	if err := db.First(&cfg).Error; err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if cfg.SiteName != "RoundTrip Blog" || cfg.PublicRead {
		t.Fatalf("site config lost: name=%q public_read=%v", cfg.SiteName, cfg.PublicRead)
	}

	// Revision history came back.
	var revCount int64
	if err := db.Model(&models.PostRevision{}).Count(&revCount).Error; err != nil {
		t.Fatalf("count revisions: %v", err)
	}
	if revCount == 0 {
		t.Fatal("revision history did not survive the round trip")
	}

	// The image library came back, live key included.
	var up models.UploadFile
	if err := db.Where("name = ?", "pic.png").First(&up).Error; err != nil {
		t.Fatalf("reload upload: %v", err)
	}
	if up.LiveKey != "sha-rt" {
		t.Fatalf("live key lost: got %q, want %q", up.LiveKey, "sha-rt")
	}
}
