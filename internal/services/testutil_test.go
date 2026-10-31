package services

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// makePost creates a post owned by author with the given status/body and
// returns it. Used by the visibility/archive/author tests.
func makePost(t *testing.T, db *gorm.DB, author models.User, status, body, title string, at time.Time) models.Post {
	t.Helper()
	p := models.Post{
		Title:     title,
		Body:      body,
		Status:    status,
		AuthorID:  author.ID,
		CreatedAt: at,
		UpdatedAt: at,
	}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create post %q: %v", title, err)
	}
	return p
}

// makePostNoTime creates a post with the DB-assigned timestamps.
func makePostNoTime(t *testing.T, db *gorm.DB, author models.User, status, body, title string) models.Post {
	t.Helper()
	p := models.Post{Title: title, Body: body, Status: status, AuthorID: author.ID}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("create post %q: %v", title, err)
	}
	return p
}

// anonymous returns the zero user (ID "") that GetPostPublic / ListArchive
// / GetAuthor treat as a logged-out visitor.
func anonymous() models.User {
	return models.User{}
}
