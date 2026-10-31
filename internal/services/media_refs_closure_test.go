package services

import (
	"context"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// The ledger only answers one question — "does deleting this file break
// something visible?" — so every place a library image can be displayed has to
// be an edge in it. These tests cover the four that were missing: post covers,
// user avatars, site branding, and the two operations that rewrite the tables
// wholesale (a restore/import rebuild, and taking a post back out of the trash).

func refCountOf(t *testing.T, db *gorm.DB, id string) int {
	t.Helper()
	var row models.UploadFile
	if err := db.Select("ref_count").Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("reload upload %s: %v", id, err)
	}
	return row.RefCount
}

func makeLibraryImage(t *testing.T, db *gorm.DB, id, name, owner, kind string) models.UploadFile {
	t.Helper()
	up := models.UploadFile{
		ID: id, Name: name, LiveKey: "sha-" + id, OwnerID: owner,
		Size: 2048, Sha256: "sha-" + id, Kind: kind, ThumbStatus: ThumbStatusSkip,
	}
	if err := db.Create(&up).Error; err != nil {
		t.Fatalf("create upload %s: %v", name, err)
	}
	return up
}

func TestPostCoverCountsAsAReference(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "cover-author", "cover-author@example.com", models.RoleUser)
	up := makeLibraryImage(t, db, "up-cover", "cover.png", author.ID, "post")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Has a cover", Body: "no image in the body", Cover: "/uploads/cover.png",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if got := refCountOf(t, db, up.ID); got != 1 {
		t.Fatalf("ref_count = %d after saving a cover, want 1", got)
	}

	// The same image both embedded and used as the cover is still ONE
	// dependency. Counting the mention twice would make the gallery warn
	// "deleting this breaks 2 articles" about a single post.
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{
		Body: "now ![](/uploads/cover.png) too",
	}); err != nil {
		t.Fatalf("edit: %v", err)
	}
	if got := refCountOf(t, db, up.ID); got != 1 {
		t.Fatalf("ref_count = %d when body and cover share the image, want 1", got)
	}

	// Pointing the cover elsewhere releases the edge: a body with no images left.
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{
		Body: "plain text", Cover: strPtr("/uploads/cover.png"),
	}); err != nil {
		t.Fatalf("second edit: %v", err)
	}
	if got := refCountOf(t, db, up.ID); got != 1 {
		t.Fatalf("ref_count = %d while the cover still points at it, want 1", got)
	}
}

func TestSiteBrandingCountsAsAReference(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	root := makeUser(t, db, "brand-root", "brand-root@example.com", models.RoleRoot)
	logo := makeLibraryImage(t, db, "up-logo", "logo.png", root.ID, "site")
	alt := makeLibraryImage(t, db, "up-logo-alt", "logo-alt.png", root.ID, "site")

	cfg, err := svc.GetSiteConfig(ctx, true)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.SiteLogo = "/uploads/logo.png"
	if _, err := svc.UpdateSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("save logo: %v", err)
	}
	if got := refCountOf(t, db, logo.ID); got != 1 {
		t.Fatalf("ref_count = %d for a site logo, want 1", got)
	}

	// Using the same file as the favicon is a second, independent edge. It only
	// survives because the two slots are different kinds: upload_refs is unique
	// on (upload_id, post_id, kind, user_id) and both have an empty post_id and
	// user_id, so a shared kind would let one overwrite the other.
	cfg.SiteFavicon = "/uploads/logo.png"
	if _, err := svc.UpdateSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("save favicon: %v", err)
	}
	if got := refCountOf(t, db, logo.ID); got != 2 {
		t.Fatalf("ref_count = %d for logo+favicon on one file, want 2", got)
	}

	// Switching the logo moves its edge; the old file keeps the favicon edge.
	cfg.SiteLogo = "/uploads/logo-alt.png"
	if _, err := svc.UpdateSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("switch logo: %v", err)
	}
	if got := refCountOf(t, db, logo.ID); got != 1 {
		t.Fatalf("old logo ref_count = %d, want 1 (favicon still points at it)", got)
	}
	if got := refCountOf(t, db, alt.ID); got != 1 {
		t.Fatalf("new logo ref_count = %d, want 1", got)
	}
}

func TestRebuildRestoresAvatarsAndBranding(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rb-author", "rb-author@example.com", models.RoleUser)
	avatar := makeLibraryImage(t, db, "up-avatar", "avatar.png", author.ID, "avatar")
	logo := makeLibraryImage(t, db, "up-brand", "brand.png", author.ID, "site")

	if err := db.Model(&models.User{}).Where("id = ?", author.ID).
		Update("avatar", "/uploads/avatar.png").Error; err != nil {
		t.Fatalf("set avatar: %v", err)
	}
	cfg, err := svc.GetSiteConfig(ctx, true)
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	cfg.SiteLogo = "/uploads/brand.png"
	if _, err := svc.UpdateSiteConfig(ctx, cfg); err != nil {
		t.Fatalf("save logo: %v", err)
	}

	// The state a restore leaves behind: the tables point at the files, the
	// ledger is empty and every count is zero. Rebuilding from posts alone — the
	// old behaviour — would leave both of these looking unused.
	if err := db.Where("1 = 1").Delete(&models.UploadRef{}).Error; err != nil {
		t.Fatalf("clear refs: %v", err)
	}
	if err := db.Model(&models.UploadFile{}).Where("1 = 1").Update("ref_count", 0).Error; err != nil {
		t.Fatalf("zero counts: %v", err)
	}

	if err := rebuildUploadRefsTx(db); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if got := refCountOf(t, db, avatar.ID); got != 1 {
		t.Fatalf("avatar ref_count = %d after rebuild, want 1", got)
	}
	if got := refCountOf(t, db, logo.ID); got != 1 {
		t.Fatalf("site logo ref_count = %d after rebuild, want 1", got)
	}

	// Idempotent, and a second pass must not double the edges.
	if err := rebuildUploadRefsTx(db); err != nil {
		t.Fatalf("rebuild twice: %v", err)
	}
	if got := refCountOf(t, db, avatar.ID); got != 1 {
		t.Fatalf("avatar ref_count = %d after a second rebuild, want 1", got)
	}
}

func TestRestoreTrashRebuildsTheImageLedger(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "trash-author", "trash-author@example.com", models.RoleUser)
	pic := makeLibraryImage(t, db, "up-trash", "trash.png", author.ID, "post")

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Embeds an image", Body: "look ![](/uploads/trash.png)",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if got := refCountOf(t, db, pic.ID); got != 1 {
		t.Fatalf("setup: ref_count = %d, want 1", got)
	}

	// Deleting releases the edge: a deleted article must not keep an image alive,
	// or nothing would ever become an orphan.
	if err := svc.DeletePost(ctx, author, post.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := refCountOf(t, db, pic.ID); got != 0 {
		t.Fatalf("ref_count = %d after the post was trashed, want 0", got)
	}

	// And restoring must put it back. Without this the article returns to the
	// site while the gallery offers to delete the image it displays.
	if err := svc.RestoreTrash(ctx, author, post.ID); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if got := refCountOf(t, db, pic.ID); got != 1 {
		t.Fatalf("ref_count = %d after the post was restored, want 1", got)
	}
}

// Endpoints that take a post id must answer 404 for a post you cannot read. An
// endpoint that answers 200 (even with an empty body) for a private post while
// answering 404 for an id that does not exist is an existence oracle: it tells
// an anonymous caller that the private post is there.
func TestRelatedAndNavRefuseInvisiblePosts(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "oracle-author", "oracle-author@example.com", models.RoleUser)
	stranger := makeUser(t, db, "oracle-stranger", "oracle-stranger@example.com", models.RoleUser)

	post, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "Secret", Body: "hidden"})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	if err := db.Model(&models.Post{}).Where("id = ?", post.ID).
		Update("status", models.PostPrivate).Error; err != nil {
		t.Fatalf("set private: %v", err)
	}

	for label, me := range map[string]models.User{
		"anonymous": {},
		"stranger":  stranger,
	} {
		if _, err := svc.RelatedPosts(ctx, me, post.ID, 5); StatusOf(err) != http.StatusNotFound {
			t.Fatalf("related as %s: want 404, got %v (err=%v)", label, StatusOf(err), err)
		}
		if _, err := svc.GetPostPrevNext(ctx, me, post.ID); StatusOf(err) != http.StatusNotFound {
			t.Fatalf("nav as %s: want 404, got %v (err=%v)", label, StatusOf(err), err)
		}
	}

	// The author still gets a normal answer, so the guard costs no functionality.
	if _, err := svc.RelatedPosts(ctx, author, post.ID, 5); err != nil {
		t.Fatalf("author must still read related: %v", err)
	}
	if _, err := svc.GetPostPrevNext(ctx, author, post.ID); err != nil {
		t.Fatalf("author must still read nav: %v", err)
	}
}
