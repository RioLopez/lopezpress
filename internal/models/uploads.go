package models

import (
	"time"

	"gorm.io/gorm"
)

// UploadFile is the metadata row for one image stored in the upload directory.
// Before this table existed the image "library" was a directory listing: no
// owner, no size accounting, no hash, no way to tell whose bytes were on disk.
// Everything the quota system, the dedup path and the admin gallery need comes
// from here.
//
// DeletedAt is a plain *time.Time, NOT gorm.DeletedAt, on purpose: a soft-
// deleted upload must keep being served. Articles that already reference it
// would break otherwise, and breaking published content is a worse outcome
// than a file sitting on disk. "Deleted" means "invisible and un-reusable",
// not "gone" — the row stays queryable, and every read path filters explicitly.
type UploadFile struct {
	ID   string `gorm:"primaryKey;size:32" json:"id"`
	Name string `gorm:"uniqueIndex;size:160;not null" json:"name"`
	// LiveKey is the de-duplication key of a *live* row: the sha256 hex for a
	// normal upload, and "d:<id>" once the row is soft-deleted. The UNIQUE
	// index on it makes two invariants structural instead of remembered:
	//   1. two live rows can never share the same bytes (concurrent uploads
	//      collide here, not in application code);
	//   2. a soft-deleted image can never come back as a dedup hit.
	// A partial index (WHERE deleted_at IS NULL) would be the direct way to
	// say this, but AutoMigrate cannot express one. Rotating the key is the
	// portable equivalent. Historical rows use "legacy:<name>" so they never
	// participate in de-duplication at all.
	LiveKey      string `gorm:"uniqueIndex;size:80;not null" json:"-"`
	OwnerID      string `gorm:"size:36;index" json:"owner_id"`
	Size         int64  `json:"size"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Mime         string `gorm:"size:64" json:"mime"`
	Sha256       string `gorm:"size:64;index" json:"sha256"`
	OriginalName string `gorm:"size:255" json:"original_name"`
	IP           string `gorm:"size:64" json:"ip"`
	// Kind is the upload purpose: "post" (inline article image, the default),
	// "avatar", or "site" (logo / favicon). It drives the per-file size cap.
	Kind     string `gorm:"size:16;default:'post'" json:"kind"`
	RefCount int    `gorm:"default:0" json:"ref_count"`
	// Legacy marks the files that were already in data/uploads when the table
	// was introduced. They are counted against the site quota but belong to
	// nobody, so they never appear in a user's own upload list.
	Legacy bool `gorm:"default:false" json:"legacy"`
	// Thumbnail pipeline state, owned by the background queue.
	// "" = never queued, "pending" = waiting, "ready" = ThumbName exists,
	// "failed" = gave up (the list falls back to the original).
	ThumbName   string `gorm:"size:160" json:"thumb_name"`
	ThumbStatus string `gorm:"size:16" json:"thumb_status"`
	// DeletedAt / DeletedBy record WHO withdrew the image. A soft delete is
	// the owner's action ("I don't want this on the site any more") and is
	// reversible by an admin; a hard delete removes the file.
	DeletedAt *time.Time `json:"deleted_at"`
	DeletedBy string     `gorm:"size:36" json:"deleted_by"`
	CreatedAt time.Time  `json:"created_at"`
}

// BeforeCreate assigns the opaque public identifier (22-char base62, generated
// from crypto/rand). The id is also the file name stem, so the URL carries no
// upload time, no user and no original name.
func (u *UploadFile) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		id, err := NewOpaqueID()
		if err != nil {
			return err
		}
		u.ID = id
	}
	return nil
}

// UploadRef is one "this image is used here" edge. It exists because a
// de-duplicated image can be referenced by people who never uploaded it: the
// uploader's own deletion must not break somebody else's article, and an admin
// about to hard-delete needs to know what will break.
//
// The unique index (upload_id, post_id, kind, user_id) makes ref syncing
// idempotent: saving the same article twice inserts nothing the second time.
// user_id is part of the key because non-article uses carry an empty post_id —
// without it, two users picking the same de-duplicated image as their avatar
// would collide on (upload, "", "avatar").
type UploadRef struct {
	ID       int64  `gorm:"primaryKey" json:"id"`
	UploadID string `gorm:"size:32;not null;uniqueIndex:idx_upload_refs" json:"upload_id"`
	// PostID is empty for non-article uses (avatar / site logo).
	PostID    string    `gorm:"size:36;not null;default:'';uniqueIndex:idx_upload_refs" json:"post_id"`
	Kind      string    `gorm:"size:16;not null;default:'body';uniqueIndex:idx_upload_refs" json:"kind"`
	UserID    string    `gorm:"size:36;not null;default:'';uniqueIndex:idx_upload_refs" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}
