package services

import (
	"context"
	"net/http"
	"testing"

	"lopezpress/internal/models"
)

// R4-03: a restore rewrites `posts` but leaves `upload_refs` and
// `upload_files.ref_count` describing the pre-restore content. The gallery's
// "deleting this image breaks N articles" warning is the admin's only guard
// before a hard delete, so a stale count is a data-loss bug, not a cosmetic
// one: it either scares the admin off a safe delete or clears the way for a
// destructive one.
func TestRestoreRebuildsTheImageLedger(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	root := makeUser(t, db, "root", "root@example.com", models.RoleRoot)
	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)

	// An upload that exists in the library.
	up := models.UploadFile{
		ID: "up-1", Name: "keepme.png", LiveKey: "sha-keepme", OwnerID: author.ID,
		Size: 1024, Sha256: "sha-keepme", Kind: "post", ThumbStatus: ThumbStatusSkip,
	}
	if err := db.Create(&up).Error; err != nil {
		t.Fatalf("create upload: %v", err)
	}

	// A live post that references it → ref_count should be 1.
	p, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Uses the image", Body: "text ![](/uploads/keepme.png) more",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}
	var before models.UploadFile
	db.Where("id = ?", up.ID).First(&before)
	if before.RefCount != 1 {
		t.Fatalf("setup: ref_count = %d, want 1", before.RefCount)
	}

	// An import whose post body does NOT reference the image. After it lands,
	// the ledger must say 0 — the article that used it is gone.
	in := ImportData{
		SchemaVersion: BackupSchemaVersion,
		AppVersion:    BackupAppVersion,
		Confirm:       ImportConfirmPhrase,
		Users: []models.User{
			{ID: root.ID, Username: root.Username, Email: root.Email, Role: models.RoleRoot, PasswordHash: root.PasswordHash},
			{ID: author.ID, Username: author.Username, Email: author.Email, Role: models.RoleUser, PasswordHash: author.PasswordHash},
		},
		PasswordHashes: map[string]string{
			root.ID:   root.PasswordHash,
			author.ID: author.PasswordHash,
		},
		Posts: []models.Post{{
			ID: "imported-1", AuthorID: author.ID, Title: "No image here",
			Body: "plain text with no upload reference", Status: models.PostPublished,
			Slug: "no-image-here",
		}},
		// The import replaces the WHOLE database, upload_files included, so the
		// dump must carry the library row for it to survive. LiveKey is json:"-"
		// and travels in the companion map.
		UploadFiles: []models.UploadFile{{
			ID: "up-1", Name: "keepme.png", OwnerID: author.ID,
			Size: 1024, Sha256: "sha-keepme", Kind: "post", ThumbStatus: ThumbStatusSkip,
		}},
		UploadLiveKeys: map[string]string{"keepme.png": "sha-keepme"},
	}
	if _, err := svc.ImportBackup(ctx, root, in, false); err != nil {
		t.Fatalf("import: %v", err)
	}

	var after models.UploadFile
	if err := db.Where("id = ?", up.ID).First(&after).Error; err != nil {
		t.Fatalf("reload upload: %v", err)
	}
	if after.RefCount != 0 {
		t.Fatalf("ref_count = %d after an import that dropped the referencing post, want 0", after.RefCount)
	}
	if p.ID == "" {
		t.Fatal("sanity")
	}

	// And the other direction: an import whose body DOES reference it must set
	// the count back to 1, so the gallery stops saying "unused".
	in2 := in
	in2.Posts = []models.Post{{
		ID: "imported-2", AuthorID: author.ID, Title: "Explains the image",
		Body: "look ![](/uploads/keepme.png)", Status: models.PostPublished,
		Slug: "explains-the-image",
	}}
	if _, err := svc.ImportBackup(ctx, root, in2, false); err != nil {
		t.Fatalf("second import: %v", err)
	}
	db.Where("id = ?", up.ID).First(&after)
	if after.RefCount != 1 {
		t.Fatalf("ref_count = %d after an import that introduced a reference, want 1", after.RefCount)
	}
}

// Direct coverage of the rebuild primitive, so a failure there is reported with
// its real error instead of the generic "failed to import backup" the caller
// wraps it in.
func TestRebuildUploadRefsTxStandalone(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "rebuilder", "rebuilder@example.com", models.RoleUser)

	up := models.UploadFile{
		ID: "u-rebuild", Name: "pic.png", LiveKey: "lk-rebuild", OwnerID: author.ID,
		Size: 10, Sha256: "lk-rebuild", Kind: "post", ThumbStatus: ThumbStatusSkip,
	}
	if err := db.Create(&up).Error; err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "Refs it", Body: "![](/uploads/pic.png)",
	}); err != nil {
		t.Fatalf("post: %v", err)
	}

	if err := rebuildUploadRefsTx(db); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	var got models.UploadFile
	if err := db.Where("id = ?", up.ID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.RefCount != 1 {
		t.Fatalf("ref_count = %d, want 1", got.RefCount)
	}
	// Idempotent.
	if err := rebuildUploadRefsTx(db); err != nil {
		t.Fatalf("rebuild twice: %v", err)
	}
	db.Where("id = ?", up.ID).First(&got)
	if got.RefCount != 1 {
		t.Fatalf("ref_count = %d after a second rebuild, want 1", got.RefCount)
	}
}

// A single-threaded restore must work: the conditional UPDATE matches on the
// exact deleted_at value read back from the row, and a timestamp that does not
// survive the round trip would make every restore answer 409. This guards the
// precision assumption (GORM writes and compares the same formatted string).
func TestRestoreTrashWorksWhenNothingRaces(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "restorer", "restorer@example.com", models.RoleUser)

	p, err := svc.CreatePost(ctx, author, CreatePostInput{Title: "To trash", Body: "b"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.DeletePost(ctx, author, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.RestoreTrash(ctx, author, p.ID); err != nil {
		t.Fatalf("restore must succeed when nothing else touches the row: %v", err)
	}
	var row models.Post
	if err := db.Where("id = ?", p.ID).First(&row).Error; err != nil {
		t.Fatalf("the restored post should be live again: %v", err)
	}
	// Restoring twice is a conflict, not a silent success.
	if err := svc.RestoreTrash(ctx, author, p.ID); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("a second restore should be refused (not in trash), got %v (err=%v)", StatusOf(err), err)
	}
}

// The rebuild is bulk: it must scale to a library with many articles without
// doing per-article round trips. This does not measure time (flaky in CI) — it
// asserts the outbound result is right at a size where a per-article
// implementation would still pass but a broken batch would not.
func TestRebuildUploadRefsHandlesManyPosts(t *testing.T) {
	svc, db := newFileTestService(t)
	ctx := context.Background()
	author := makeUser(t, db, "bulk", "bulk@example.com", models.RoleUser)

	const uploads = 20
	for i := 0; i < uploads; i++ {
		name := "img" + string(rune('a'+i)) + ".png"
		u := models.UploadFile{
			ID: "u-" + string(rune('a'+i)), Name: name, LiveKey: "lk-" + name,
			OwnerID: author.ID, Size: 10, Sha256: "lk-" + name, Kind: "post",
			ThumbStatus: ThumbStatusSkip,
		}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
		// Three posts reference each image, so ref_count must land on 3.
		for j := 0; j < 3; j++ {
			if _, err := svc.CreatePost(ctx, author, CreatePostInput{
				Title: "P" + name + string(rune('0'+j)),
				Body:  "![](/uploads/" + name + ")",
			}); err != nil {
				t.Fatalf("post: %v", err)
			}
		}
	}

	if err := rebuildUploadRefsTx(db); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	var rows []models.UploadFile
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, u := range rows {
		if u.RefCount != 3 {
			t.Fatalf("upload %s: ref_count = %d, want 3", u.Name, u.RefCount)
		}
	}
	var edgeCount int64
	db.Model(&models.UploadRef{}).Count(&edgeCount)
	if edgeCount != uploads*3 {
		t.Fatalf("edge count = %d, want %d", edgeCount, uploads*3)
	}
}
