package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// Listing cards must load the thumbnail, not the original.
//
// A listing renders ten covers at once and an upload is often a multi-megabyte
// phone photo displayed a few hundred pixels wide. The service therefore
// resolves cover → thumbnail per page (one query, not one per card) and leaves
// the field empty whenever it cannot guarantee the file exists.
func TestListPageAttachesCoverThumb(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()
	author := makeUser(t, db, "ct-author", "ct-author@example.com", "user")

	seed := func(name, thumb, status string) {
		t.Helper()
		f := models.UploadFile{
			ID: "up-" + name, Name: name, LiveKey: "key-" + name,
			Mime: "image/jpeg", Size: 2048, Kind: "post",
			ThumbName: thumb, ThumbStatus: status,
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	seed("ready.jpg", "ready_thumb.jpg", ThumbStatusReady)
	seed("waiting.jpg", "", ThumbStatusPending)

	mk := func(cover string) models.Post {
		t.Helper()
		p := makePostNoTime(t, db, author, "published", "body", "post "+cover)
		if err := db.Model(&models.Post{}).Where("id = ?", p.ID).Update("cover", cover).Error; err != nil {
			t.Fatalf("set cover: %v", err)
		}
		return p
	}
	withThumb := mk("/uploads/ready.jpg")
	withoutThumb := mk("/uploads/waiting.jpg")
	external := mk("https://cdn.example.com/pic.jpg")

	page, err := svc.ListPostsPage(ctx, anonymous(), ListPostsQuery{PageSize: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := make(map[string]string, len(page.Items))
	for _, p := range page.Items {
		got[p.ID] = p.CoverThumb
	}
	if got[withThumb.ID] != "/uploads/ready_thumb.jpg" {
		t.Fatalf("a ready thumbnail should be offered to the card, got %q", got[withThumb.ID])
	}
	if got[withoutThumb.ID] != "" {
		t.Fatalf("an unfinished thumbnail must not be advertised, got %q", got[withoutThumb.ID])
	}
	if got[external.ID] != "" {
		t.Fatalf("a remote cover has no local thumbnail, got %q", got[external.ID])
	}
}
