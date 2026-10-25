package services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"lopezpress/internal/models"
)

// noisyPNG produces a deterministic, photographic-ish image. A flat colour
// would compress to a few kilobytes and make the "is the thumbnail smaller
// than the original?" branch untestable, because a 480px JPEG of a solid
// rectangle is usually *bigger* than a 1200px PNG of one.
func noisyPNG(t *testing.T, w, h int, alpha bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	rnd := rand.New(rand.NewSource(int64(w*1000 + h)))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if alpha && x < w/4 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8(rnd.Intn(256)), G: uint8(rnd.Intn(256)), B: uint8(rnd.Intn(256)), A: a,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestRenderThumbScaledToLongEdge(t *testing.T) {
	data := noisyPNG(t, 1200, 800, false)
	out, ext, status := renderThumb(data)
	if status != ThumbStatusReady {
		t.Fatalf("status = %q, want %q", status, ThumbStatusReady)
	}
	if ext != ".jpg" {
		t.Fatalf("ext = %q, want .jpg for an opaque source", ext)
	}
	if len(out) >= len(data) {
		t.Fatalf("thumbnail (%d B) is not smaller than the source (%d B)", len(out), len(data))
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("decode thumbnail: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("format = %q, want jpeg", format)
	}
	if cfg.Width != 480 || cfg.Height != 320 {
		t.Fatalf("size = %dx%d, want 480x320 (aspect ratio must be preserved)", cfg.Width, cfg.Height)
	}
}

func TestRenderThumbKeepsTransparency(t *testing.T) {
	data := noisyPNG(t, 900, 900, true)
	out, ext, status := renderThumb(data)
	if status != ThumbStatusReady {
		t.Fatalf("status = %q, want %q", status, ThumbStatusReady)
	}
	if ext != ".png" {
		t.Fatalf("ext = %q, want .png: JPEG would render every transparent pixel black", ext)
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(out)); err != nil || format != "png" {
		t.Fatalf("format = %q err = %v, want png", format, err)
	}
}

func TestRenderThumbSkipsSmallSource(t *testing.T) {
	// 3x2 PNG: already cheaper to send than any re-encode.
	if _, _, status := renderThumb(tinyPNG(t, 40)); status != ThumbStatusSkip {
		t.Fatalf("status = %q, want %q for a tiny source", status, ThumbStatusSkip)
	}
}

func TestRenderThumbRejectsGarbage(t *testing.T) {
	if _, _, status := renderThumb([]byte("this is not an image at all")); status != ThumbStatusFailed {
		t.Fatalf("status = %q, want %q", status, ThumbStatusFailed)
	}
}

func TestFitWithin(t *testing.T) {
	cases := []struct{ w, h, max, wantW, wantH int }{
		{1200, 800, 480, 480, 320},
		{800, 1200, 480, 320, 480},
		{480, 480, 480, 480, 480},
		{100, 50, 480, 100, 50},
		{5000, 1, 480, 480, 1}, // never collapses to zero
	}
	for _, c := range cases {
		gotW, gotH := fitWithin(c.w, c.h, c.max)
		if gotW != c.wantW || gotH != c.wantH {
			t.Fatalf("fitWithin(%d,%d,%d) = %dx%d, want %dx%d", c.w, c.h, c.max, gotW, gotH, c.wantW, c.wantH)
		}
	}
}

func TestBuildThumbWritesFileAndMarksReady(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	data := noisyPNG(t, 1000, 700, false)
	const name = "abcdefghijklmnopqrstuv.png"
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatalf("seed upload: %v", err)
	}
	row := models.UploadFile{
		Name: name, LiveKey: "legacy:" + name, Size: int64(len(data)),
		ThumbStatus: ThumbStatusPending,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	svc.buildThumb(context.Background(), row.ID)

	var got models.UploadFile
	if err := db.Where("id = ?", row.ID).First(&got).Error; err != nil {
		t.Fatalf("reload row: %v", err)
	}
	if got.ThumbStatus != ThumbStatusReady || got.ThumbName == "" {
		t.Fatalf("status = %q name = %q, want ready with a file", got.ThumbStatus, got.ThumbName)
	}
	if _, err := os.Stat(filepath.Join(dir, got.ThumbName)); err != nil {
		t.Fatalf("thumbnail file missing: %v", err)
	}
	if got.ThumbName != row.ID+"_t.jpg" {
		t.Fatalf("thumb name = %q, want %q", got.ThumbName, row.ID+"_t.jpg")
	}
	if url := thumbURL(got); url != "/uploads/"+got.ThumbName {
		t.Fatalf("thumbURL = %q", url)
	}
}

func TestBuildThumbIgnoresDeletedRow(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	data := noisyPNG(t, 800, 600, false)
	const name = "deletedrowimage12345.png"
	_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
	deletedAt := time.Now()
	row := models.UploadFile{
		Name: name, LiveKey: "legacy:" + name, Size: int64(len(data)),
		ThumbStatus: ThumbStatusPending, DeletedAt: &deletedAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	svc.buildThumb(context.Background(), row.ID)

	var got models.UploadFile
	_ = db.Where("id = ?", row.ID).First(&got).Error
	if got.ThumbName != "" || got.ThumbStatus != ThumbStatusPending {
		t.Fatalf("a withdrawn image must not be processed: status=%q name=%q", got.ThumbStatus, got.ThumbName)
	}
}

func TestNeedsThumbTerminalStates(t *testing.T) {
	for _, st := range []string{ThumbStatusReady, ThumbStatusSkip, ThumbStatusFailed} {
		if needsThumb(models.UploadFile{ThumbStatus: st}) {
			t.Fatalf("needsThumb(%q) = true, want false", st)
		}
	}
	for _, st := range []string{"", ThumbStatusPending} {
		if !needsThumb(models.UploadFile{ThumbStatus: st}) {
			t.Fatalf("needsThumb(%q) = false, want true", st)
		}
	}
}

func TestHardDeleteRemovesThumbnailFile(t *testing.T) {
	svc, db, dir := freshUploadService(t)
	data := noisyPNG(t, 900, 600, false)
	const name = "purgewithmethumb12345.png"
	_ = os.WriteFile(filepath.Join(dir, name), data, 0o644)
	thumbName := "purgewithmethumb12345_t.jpg"
	_ = os.WriteFile(filepath.Join(dir, thumbName), []byte("thumb"), 0o644)

	row := models.UploadFile{
		Name: name, LiveKey: "legacy:" + name, Size: int64(len(data)),
		ThumbStatus: ThumbStatusReady, ThumbName: thumbName,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	root := models.User{ID: "root-1", Username: "root", Role: models.RoleRoot}
	ctx := WithActor(context.Background(), root, "127.0.0.1")
	if _, err := svc.DeleteUpload(ctx, root, name, true); err != nil {
		t.Fatalf("hard delete: %v", err)
	}
	for _, f := range []string{name, thumbName} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists after hard delete (err=%v)", f, err)
		}
	}
}
