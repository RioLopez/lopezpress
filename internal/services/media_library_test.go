package services

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// tinyPNG builds a real (decodable) PNG so upload validation accepts it. A
// hand-written byte literal would be shorter but would also silently stop
// exercising the DecodeConfig check if the format ever changed.
func tinyPNG(t *testing.T, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			img.Set(x, y, color.RGBA{R: shade, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// freshUploadService returns a service whose upload directory is a temp dir, so
// tests never write into the repository's data/uploads.
func freshUploadService(t *testing.T) (*Service, *gorm.DB, string) {
	t.Helper()
	svc, db := newTestService(t)
	dir := t.TempDir()
	svc.cfg.UploadDir = dir
	return svc, db, dir
}

func countFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	return len(entries)
}

// TestUploadDeduplicatesIdenticalBytes covers dedup: the second upload of
// the same picture must reuse the first row, write nothing to disk, and say so.
func TestUploadDeduplicatesIdenticalBytes(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	defer closeDB(db)

	owner := makeUser(t, db, "uploader", "uploader@example.com", models.RoleUser)
	ctx := WithActor(context.Background(), owner, "10.0.0.9")
	data := tinyPNG(t, 0x33)

	first, err := svc.UploadImage(ctx, "first.png", data, UploadKindPost)
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}
	if first.Deduped {
		t.Fatalf("first upload must not be marked deduped")
	}
	if first.Width != 3 || first.Height != 2 {
		t.Fatalf("geometry = %dx%d, want 3x2", first.Width, first.Height)
	}
	if !fileExists(filepath.Join(dir, first.Name)) {
		t.Fatalf("first upload file missing from %s", dir)
	}

	second, err := svc.UploadImage(ctx, "copy.png", data, UploadKindPost)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if !second.Deduped {
		t.Fatalf("identical bytes must be de-duplicated")
	}
	if second.Name != first.Name {
		t.Fatalf("dedup returned %q, want the original %q", second.Name, first.Name)
	}
	if n := countFiles(t, dir); n != 1 {
		t.Fatalf("upload dir holds %d files after a dedup, want 1", n)
	}

	var rows int64
	db.Model(&models.UploadFile{}).Count(&rows)
	if rows != 1 {
		t.Fatalf("upload_files has %d rows, want 1", rows)
	}
}

// TestSoftDeleteWithdrawsWithoutBreakingLinks is the round-77 deletion rule:
// the owner can take the picture down, the file keeps being served (published
// articles must not break), it leaves every list, and — the part that matters
// legally — nobody can be handed those bytes again as a dedup hit.
func TestSoftDeleteWithdrawsWithoutBreakingLinks(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	defer closeDB(db)

	owner := makeUser(t, db, "owneralice", "owneralice@example.com", models.RoleUser)
	other := makeUser(t, db, "bob", "bob@example.com", models.RoleUser)
	ctx := WithActor(context.Background(), owner, "10.0.0.9")
	data := tinyPNG(t, 0x44)

	up, err := svc.UploadImage(ctx, "mine.png", data, UploadKindPost)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	res, err := svc.DeleteUpload(ctx, owner, up.Name, false)
	if err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if res.Hard {
		t.Fatalf("owner delete must be soft, got hard")
	}
	if !fileExists(filepath.Join(dir, up.Name)) {
		t.Fatalf("soft delete removed the file; published articles would break")
	}

	// Still resolvable (the served URL keeps working).
	if _, err := svc.ResolveUploadFile(up.Name); err != nil {
		t.Fatalf("soft-deleted upload must stay resolvable: %v", err)
	}

	// Gone from the owner's list.
	page, err := svc.ListUploads(ctx, UploadListQuery{OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("owner list still shows %d withdrawn upload(s)", len(page.Items))
	}

	// Somebody else uploading the same bytes must NOT get the withdrawn file.
	ctxBob := WithActor(context.Background(), other, "10.0.0.10")
	again, err := svc.UploadImage(ctxBob, "same.png", data, UploadKindPost)
	if err != nil {
		t.Fatalf("re-upload after withdrawal: %v", err)
	}
	if again.Deduped {
		t.Fatalf("a withdrawn image must never be handed to another user")
	}
	if again.Name == up.Name {
		t.Fatalf("re-upload reused the withdrawn file name %q", up.Name)
	}
	if n := countFiles(t, dir); n != 2 {
		t.Fatalf("upload dir holds %d files, want 2", n)
	}
}

// TestUploadDeletePermissions locks the authorisation split: a plain user can
// only withdraw their own image, and only an admin can destroy the file.
func TestUploadDeletePermissions(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	defer closeDB(db)

	owner := makeUser(t, db, "owner", "owner@example.com", models.RoleUser)
	stranger := makeUser(t, db, "stranger", "stranger@example.com", models.RoleUser)
	admin := makeUser(t, db, "mod", "mod@example.com", models.RoleAdmin)

	ctx := WithActor(context.Background(), owner, "")
	up, err := svc.UploadImage(ctx, "pic.png", tinyPNG(t, 0x55), UploadKindPost)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	if _, err := svc.DeleteUpload(WithActor(context.Background(), stranger, ""), stranger, up.Name, false); err == nil {
		t.Fatalf("a stranger must not be able to delete someone else's upload")
	} else if StatusOf(err) != http.StatusForbidden {
		t.Fatalf("stranger delete: status %d, want 403", StatusOf(err))
	}

	if _, err := svc.DeleteUpload(ctx, owner, up.Name, true); err == nil {
		t.Fatalf("a plain user must not be able to hard-delete")
	} else if StatusOf(err) != http.StatusForbidden {
		t.Fatalf("owner hard delete: status %d, want 403", StatusOf(err))
	}

	// Admin hard delete wins even when the owner disagrees, and reports refs.
	res, err := svc.DeleteUpload(WithActor(context.Background(), admin, ""), admin, up.Name, true)
	if err != nil {
		t.Fatalf("admin hard delete: %v", err)
	}
	if !res.Hard {
		t.Fatalf("admin delete must be hard")
	}
	if fileExists(filepath.Join(dir, up.Name)) {
		t.Fatalf("hard delete left the file on disk")
	}
	var rows int64
	db.Model(&models.UploadFile{}).Count(&rows)
	if rows != 0 {
		t.Fatalf("hard delete left %d metadata row(s)", rows)
	}
}

// TestUploadUserQuota covers the per-account ceiling: a user at their limit is
// refused, and the refusal happens before anything is written.
func TestUploadUserQuota(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	defer closeDB(db)

	// Tight budget so a single small PNG exceeds it.
	if err := db.Model(&models.SiteConfig{}).Where("id = ?", 1).
		Update("user_quota_bytes", int64(64)).Error; err != nil {
		t.Fatalf("tighten quota: %v", err)
	}

	user := makeUser(t, db, "quotauser", "quota@example.com", models.RoleUser)
	ctx := WithActor(context.Background(), user, "")

	if _, err := svc.UploadImage(ctx, "big.png", tinyPNG(t, 0x66), UploadKindPost); err == nil {
		t.Fatalf("upload over the user quota should fail")
	} else if StatusOf(err) != 413 {
		t.Fatalf("quota rejection status = %d, want 413", StatusOf(err))
	}
	if n := countFiles(t, dir); n != 0 {
		t.Fatalf("rejected upload wrote %d file(s) to disk", n)
	}
}

// TestPostBodyImageRefs covers the reference ledger: saving an article records
// its images, editing the body releases the ones that disappeared, and deleting
// the article releases them all.
func TestPostBodyImageRefs(t *testing.T) {
	svc, db, _ := freshUploadService(t)
	defer closeDB(db)

	author := makeUser(t, db, "writer", "writer@example.com", models.RoleUser)
	ctx := WithActor(context.Background(), author, "")

	up, err := svc.UploadImage(ctx, "inline.png", tinyPNG(t, 0x77), UploadKindPost)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	post, err := svc.CreatePost(ctx, author, CreatePostInput{
		Title: "with an image",
		Body:  "hello\n\n![x](" + up.URL + ")\n",
	})
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	refCount := func() int {
		var row models.UploadFile
		if err := db.Where("name = ?", up.Name).First(&row).Error; err != nil {
			t.Fatalf("load upload: %v", err)
		}
		return row.RefCount
	}
	if got := refCount(); got != 1 {
		t.Fatalf("ref_count after create = %d, want 1", got)
	}

	// Edit the body so the image is gone.
	body := "no pictures here"
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Body: body}); err != nil {
		t.Fatalf("update post: %v", err)
	}
	if got := refCount(); got != 0 {
		t.Fatalf("ref_count after removing the image = %d, want 0", got)
	}

	// Re-add it, then delete the article.
	body2 := "![y](" + up.URL + ")"
	if _, err := svc.UpdatePost(ctx, author, post.ID, UpdatePostInput{Body: body2}); err != nil {
		t.Fatalf("re-add image: %v", err)
	}
	if got := refCount(); got != 1 {
		t.Fatalf("ref_count after re-adding = %d, want 1", got)
	}
	if err := svc.DeletePost(ctx, author, post.ID); err != nil {
		t.Fatalf("delete post: %v", err)
	}
	if got := refCount(); got != 0 {
		t.Fatalf("ref_count after deleting the article = %d, want 0", got)
	}
}

// TestAvatarMustComeFromLibrary covers the avatar hardening: an external URL is
// refused unless the site owner enabled it, and a library image is accepted and
// recorded as an avatar reference.
func TestAvatarMustComeFromLibrary(t *testing.T) {
	svc, db, _ := freshUploadService(t)
	defer closeDB(db)

	user := makeUser(t, db, "avataruser", "avatar@example.com", models.RoleUser)
	ctx := WithActor(context.Background(), user, "")

	external := "https://tracker.example.com/pixel.gif"
	if _, err := svc.UpdateMe(ctx, user.ID, UpdateMeInput{Avatar: &external}); err == nil {
		t.Fatalf("external avatar should be rejected while allow_external_avatar is off")
	}

	up, err := svc.UploadImage(ctx, "face.png", tinyPNG(t, 0x88), UploadKindAvatar)
	if err != nil {
		t.Fatalf("avatar upload: %v", err)
	}
	updated, err := svc.UpdateMe(ctx, user.ID, UpdateMeInput{Avatar: &up.URL})
	if err != nil {
		t.Fatalf("set library avatar: %v", err)
	}
	if updated.Avatar != up.URL {
		t.Fatalf("avatar = %q, want %q", updated.Avatar, up.URL)
	}

	var refs int64
	db.Model(&models.UploadRef{}).Where("kind = ? AND user_id = ?", "avatar", user.ID).Count(&refs)
	if refs != 1 {
		t.Fatalf("avatar refs = %d, want 1", refs)
	}

	// Disabling the switch blocks further changes.
	if err := db.Model(&models.SiteConfig{}).Where("id = ?", 1).
		Update("allow_user_change_avatar", false).Error; err != nil {
		t.Fatalf("disable avatar changes: %v", err)
	}
	empty := ""
	if _, err := svc.UpdateMe(ctx, user.ID, UpdateMeInput{Avatar: &empty}); err == nil {
		t.Fatalf("avatar change should be blocked by the site switch")
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// R4-09 / R6-04: the upload path only ever read the file HEADER
// (image.DecodeConfig stops at IHDR), so a 64×64 PNG followed by 1 MB of
// arbitrary bytes — or by "<?php system($_GET['c']); ?>" — was accepted and
// served back with Content-Type: image/png. The site became a free binary host
// whose files looked like images. The stored bytes are now a re-encode of the
// decoded bitmap, which by construction contains nothing the decoder ignored.
func TestUploadStripsBytesAfterTheImage(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	ctx := context.Background()
	u := makeUser(t, db, "polyglot", "polyglot@example.com", models.RoleUser)
	ctx = WithActor(ctx, u, "203.0.113.5")

	// A real PNG with a payload glued on after IEND.
	base := tinyPNG(t, 10)
	payload := []byte("<?php system($_GET['c']); ?>")
	data := append(append([]byte{}, base...), payload...)

	res, err := svc.UploadImage(ctx, "evil.png", data, "image")
	if err != nil {
		t.Fatalf("upload should succeed (the header is a genuine PNG): %v", err)
	}

	stored, err := os.ReadFile(filepath.Join(dir, res.Name))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if bytes.Contains(stored, payload) {
		t.Fatal("the appended payload survived into the stored file — the image was not re-encoded")
	}
	// The stored file must still be a valid image of the same dimensions.
	cfg, _, err := image.DecodeConfig(bytes.NewReader(stored))
	if err != nil {
		t.Fatalf("stored file is no longer a decodable image: %v", err)
	}
	if cfg.Width != 3 || cfg.Height != 2 {
		t.Fatalf("stored geometry changed: got %dx%d, want 3x2", cfg.Width, cfg.Height)
	}

	// A megabyte of random trailing bytes must not survive either.
	junk := append(append([]byte{}, base...), make([]byte, 1<<20)...)
	res2, err := svc.UploadImage(ctx, "fat.png", junk, "image")
	if err != nil {
		t.Fatalf("upload with trailing junk: %v", err)
	}
	stored2, err := os.ReadFile(filepath.Join(dir, res2.Name))
	if err != nil {
		t.Fatalf("read stored: %v", err)
	}
	if len(stored2) > 1<<16 {
		t.Fatalf("stored file kept %d bytes of trailing data (limit 64 KiB for a 3x2 image)", len(stored2))
	}
}

// A file whose header parses but whose pixels do not (truncated mid-stream)
// must be refused rather than stored: it would render as a broken image and
// still occupy a quota slot.
func TestUploadRejectsTruncatedImageData(t *testing.T) {
	svc, db, _ := freshUploadService(t)
	ctx := context.Background()
	u := makeUser(t, db, "trunc", "trunc@example.com", models.RoleUser)
	ctx = WithActor(ctx, u, "203.0.113.6")

	base := tinyPNG(t, 20)
	truncated := base[:len(base)/2] // header intact, IDAT cut short
	if _, err := svc.UploadImage(ctx, "cut.png", truncated, "image"); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("a truncated PNG must be refused with 400, got %v (err=%v)", StatusOf(err), err)
	}
}

// R4-08 / R6-03: the quota was a read-then-write pair, so N concurrent uploads
// each read the same "under quota" total and each inserted — 8 concurrent
// uploads measured 101.8% of quota, 10 measured 1.5 MB over. The check and the
// INSERT now share one transaction, and the writer lock is taken at BEGIN
// (_txlock=immediate), so concurrent uploads serialise through the ceiling.
//
// The assertion that matters is the last one: the RECORDED usage must never
// exceed the quota. Before the fix every racer saw the same "under quota"
// total and every one of them inserted, so the sum overshot by roughly
// (N-1) × file size. How many uploads happen to succeed is not asserted,
// because it depends on the order the writer lock hands out — the ceiling is.
func TestUploadQuotaIsNotExceededUnderConcurrency(t *testing.T) {
	// A FILE-backed database: :memory: gives every pooled connection its own
	// private store, so concurrent racers would be reading and writing
	// different databases and the result would prove nothing.
	svc, db := newFileTestService(t)
	dir := t.TempDir()
	svc.cfg.UploadDir = dir
	u := makeUser(t, db, "racer", "racer@example.com", models.RoleUser)

	// A quota comfortably above one file but well below all six.
	const quota = 4096
	var cfg models.SiteConfig
	if err := db.Where("id = ?", 1).First(&cfg).Error; err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.UserQuotaBytes = quota
	cfg.SiteQuotaBytes = 0
	if err := db.Save(&cfg).Error; err != nil {
		t.Fatalf("set quota: %v", err)
	}

	// Six DISTINCT images. Distinct bytes matter twice: de-duplication must not
	// swallow them (a repeated upload costs no quota and would prove nothing),
	// and the re-encode step keeps them distinct so each insert is real.
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 251)
	}
	variants := make([][]byte, 6)
	for i := range variants {
		img.Set(i%48, i/48, color.RGBA{R: uint8(i * 40), G: 7, B: 3, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("encode: %v", err)
		}
		if buf.Len() >= quota {
			t.Fatalf("test image is %d bytes, must stay under the %d quota", buf.Len(), quota)
		}
		variants[i] = buf.Bytes()
	}

	var wg sync.WaitGroup
	results := make([]error, len(variants))
	for i, v := range variants {
		wg.Add(1)
		go func(i int, v []byte) {
			defer wg.Done()
			c := WithActor(context.Background(), u, "203.0.113.7")
			_, results[i] = svc.UploadImage(c, "q.png", v, "image")
		}(i, v)
	}
	wg.Wait()

	ok := 0
	for _, err := range results {
		switch {
		case err == nil:
			ok++
		case StatusOf(err) == http.StatusRequestEntityTooLarge:
			// expected for the losers
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok == 0 {
		t.Fatal("no upload succeeded — the quota is too tight for the test to mean anything")
	}
	if ok == len(variants) {
		t.Fatalf("all %d uploads passed a %d-byte quota — the ceiling was not enforced at all", ok, quota)
	}

	// The ceiling itself: recorded usage must never exceed what was allowed.
	var used int64
	db.Raw(`SELECT COALESCE(SUM(size), 0) FROM upload_files WHERE owner_id = ? AND deleted_at IS NULL`, u.ID).Scan(&used)
	if used > quota {
		t.Fatalf("recorded usage %d exceeds the %d-byte quota — the check and the insert are not atomic", used, quota)
	}
}

// R6-05: the storage quota counts BYTES while the cost of an image is counted
// in PIXELS. A 6000×4000 solid-colour PNG is ~23 KB — so twenty of them fit
// inside a 20 MB quota — yet each decodes to a ~96 MB bitmap. Four of them
// (93.8 KB of "storage") moved RSS by 95 MB and pinned the thumbnail worker
// for about a minute, which on the 1.5 GB target box is a denial of service
// reachable by any author with a script.
func TestUploadRejectsImplausiblyCompressibleImage(t *testing.T) {
	svc, db, _ := freshUploadService(t)
	ctx := context.Background()
	u := makeUser(t, db, "bomb", "bomb@example.com", models.RoleUser)
	ctx = WithActor(ctx, u, "203.0.113.8")

	// The predicate first, with the reported numbers: 24 MP in 23 KB is the
	// shape R6-05 measured.
	if !isImplausiblyCompressible(23400, 6000*4000) {
		t.Fatal("24 MP in 23 KB must be flagged as implausible")
	}
	// A real photograph runs the other way and must not be flagged.
	if isImplausiblyCompressible(4*1024*1024, 12*1000*1000) {
		t.Fatal("a 4 MB 12 MP photo must not be flagged")
	}

	// Then the upload path itself. A Gray image is 1 byte per pixel, so this
	// costs 4 MB of RAM instead of the 96 MB an RGBA canvas of the reported size
	// would — and the earlier version of this test, which built that canvas, was
	// enough to OOM the whole package on a 16 GB machine.
	img := image.NewGray(image.Rect(0, 0, 2100, 2000)) // 4.2 MP
	for i := range img.Pix {
		img.Pix[i] = 0x80
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if buf.Len() > 256*1024 {
		t.Fatalf("test image is %d bytes; a flat canvas should stay tiny", buf.Len())
	}

	if _, err := svc.UploadImage(ctx, "flat.png", buf.Bytes(), "image"); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("a 4.2 MP image of %d bytes must be refused, got %v (err=%v)", buf.Len(), StatusOf(err), err)
	}
}

// The guard must not touch ordinary photographs: a small file for a small
// image is normal, and a large file for a large image is normal. Only the
// impossible combination is rejected.
func TestUploadAcceptsNormalImagesRegardlessOfCompression(t *testing.T) {
	svc, db, _ := freshUploadService(t)
	ctx := context.Background()
	u := makeUser(t, db, "normal", "normal@example.com", models.RoleUser)
	ctx = WithActor(ctx, u, "203.0.113.9")

	// Small image, tiny file: below the pixel threshold the check does not run.
	small := tinyPNG(t, 33)
	if _, err := svc.UploadImage(ctx, "small.png", small, "image"); err != nil {
		t.Fatalf("a small legitimate image must be accepted: %v", err)
	}

	// A large image carrying real data: the predicate must leave it alone.
	// Asserted on the predicate rather than by encoding a 12 MP canvas — the
	// encode alone allocated 48 MB, which is not worth spending on a boundary
	// check that is a pure function of two integers.
	if isImplausiblyCompressible(3*1024*1024, 4000*3000) {
		t.Fatal("a 3 MB 12 MP image must not be flagged")
	}
	// And a genuinely normal, noisy image must upload.
	big := image.NewGray(image.Rect(0, 0, 600, 400))
	for i := range big.Pix {
		big.Pix[i] = uint8(i * 2654435761 >> 13)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, big); err != nil {
		t.Fatalf("encode: %v", err)
	}
	if _, err := svc.UploadImage(ctx, "noisy.png", buf.Bytes(), "image"); err != nil {
		t.Fatalf("a normal image must be accepted: %v", err)
	}
}

// A path that can never be a real upload must answer 404 at the resolution
// step, not fall through to the OS and surface as a 500 from http.ServeFile.
// R7 reported exactly that: a NUL byte or an overlong name made the handler
// return 500, which fingerprints the stack and tells the caller the server is
// broken rather than that the file does not exist.
func TestResolveUploadFileRejectsIllegalNames(t *testing.T) {
	svc, _ := newFileTestService(t)
	bad := []string{
		"\x00.png",               // embedded NUL
		"a\x01b.png",             // control byte
		"..\\..\\data\\app.db",   // backslash traversal (Windows separator)
		"c:/windows/win.ini",     // drive colon
		"a/b.png",                // forward slash
		strings.Repeat("x", 300), // over any filesystem limit
	}
	for _, name := range bad {
		if _, err := svc.ResolveUploadFile(name); StatusOf(err) != http.StatusNotFound {
			t.Fatalf("ResolveUploadFile(%q) must be 404, got %v (err=%v)", name, StatusOf(err), err)
		}
	}
	// A normal generated name still resolves.
	if _, err := svc.ResolveUploadFile("abc123.png"); err != nil {
		t.Fatalf("a normal name must resolve: %v", err)
	}
}

// R7-07 (pixel ceiling): a 24 MP image in a few hundred KB passes the
// compression guard — its bytes are not "impossibly small for its pixels" — yet
// decoding it allocates ~96 MB. Before the ceiling, the request path would
// happily decode an image that large, and a handful of concurrent ones was how
// a 1.5 GB box died. The ceiling is checked from the header (DecodeConfig)
// BEFORE the full decode, so the test crafts only a header declaring a huge
// canvas, never the pixels themselves — encoding a real 30 MP image would cost
// 120 MB of test RAM.
func TestUploadRejectsImagesOverThePixelCeiling(t *testing.T) {
	// A minimal PNG whose IHDR declares 30 MP. DecodeConfig reads the header
	// and returns the dimensions; it never needs the IDAT pixels, but it DOES
	// validate the IHDR CRC, so the test computes a real one.
	pngHeader := func(w, h uint32) []byte {
		var out bytes.Buffer
		out.Write([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a})
		ihdr := make([]byte, 13)
		binary.BigEndian.PutUint32(ihdr[0:], w)
		binary.BigEndian.PutUint32(ihdr[4:], h)
		ihdr[8] = 8 // bit depth
		ihdr[9] = 2 // colour type (RGB)
		var lenB [4]byte
		binary.BigEndian.PutUint32(lenB[:], uint32(len(ihdr)))
		out.Write(lenB[:])
		out.WriteString("IHDR")
		out.Write(ihdr)
		crc := crc32.NewIEEE()
		crc.Write([]byte("IHDR"))
		crc.Write(ihdr)
		var crcB [4]byte
		binary.BigEndian.PutUint32(crcB[:], crc.Sum32())
		out.Write(crcB[:])
		return out.Bytes()
	}

	if _, err := prepareUploadBytes(pngHeader(10000, 3000), ".png"); StatusOf(err) != http.StatusRequestEntityTooLarge {
		t.Fatalf("a 30 MP image must be refused with 413, got %v (err=%v)", StatusOf(err), err)
	}

	// A 24 MP image is still allowed (the ceiling is inclusive and matches the
	// thumbnail worker's limit, so the two never disagree).
	// A 24 MP image is still allowed (the ceiling is inclusive and matches the
	// thumbnail worker's limit). It is header-only so the full decode rejects
	// it as corrupt — but the point is the ceiling did NOT fire.
	if _, err := prepareUploadBytes(pngHeader(6000, 4000), ".png"); StatusOf(err) == http.StatusRequestEntityTooLarge {
		t.Fatalf("a 24 MP image must pass the ceiling (the decode may reject it as corrupt, got %v)", StatusOf(err))
	}
}
