package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// TestPublicPostPayloadStripsAuthorSecrets locks in the fix for the "author
// email / login IP leaked on the public post list" finding: any payload an
// anonymous visitor can reach must carry only display fields on the author,
// while the admin-only review queue (which legitimately shows the email) keeps
// the full record.
func TestPublicPostPayloadStripsAuthorSecrets(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	author := makeUser(t, db, "alice", "alice@example.com", models.RoleUser)
	// A login audit trail that must never reach a public payload.
	if err := db.Model(&models.User{}).Where("id = ?", author.ID).
		Update("last_login_ip", "203.0.113.7").Error; err != nil {
		t.Fatalf("seed last_login_ip: %v", err)
	}

	pub := models.Post{ID: "pub-1", Title: "Public", Body: "b", Status: models.PostPublished, AuthorID: author.ID, Slug: "public"}
	if err := db.Create(&pub).Error; err != nil {
		t.Fatalf("create post: %v", err)
	}

	// Anonymous list: the author block must carry no email / login IP.
	page, err := svc.ListPostsPage(ctx, models.User{}, ListPostsQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListPostsPage: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(page.Items))
	}
	a := page.Items[0].Author
	if a == nil {
		t.Fatal("author not preloaded")
	}
	if a.Email != "" || a.LastLoginIP != "" || a.LastLoginAt != nil {
		t.Fatalf("public list leaked author secrets: email=%q ip=%q at=%v", a.Email, a.LastLoginIP, a.LastLoginAt)
	}
	if a.Username != "alice" || a.Role != models.RoleUser {
		t.Fatalf("display fields must survive sanitisation: %+v", a)
	}

	// The single public read is sanitised too.
	one, err := svc.GetPostPublic(ctx, pub.ID)
	if err != nil {
		t.Fatalf("GetPostPublic: %v", err)
	}
	if one.Author == nil || one.Author.Email != "" || one.Author.LastLoginIP != "" {
		t.Fatalf("public single read leaked author secrets: %+v", one.Author)
	}

	// The review queue is admin-only and legitimately shows the author email,
	// so it must be preserved there.
	pending := models.Post{ID: "pend-1", Title: "Pending", Body: "b", Status: models.PostPending, AuthorID: author.ID, Slug: "pending"}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("create pending: %v", err)
	}
	admin := makeUser(t, db, "mod", "mod@example.com", models.RoleAdmin)
	list, err := svc.ListPending(ctx, admin)
	if err != nil {
		t.Fatalf("ListPending: %v", err)
	}
	if len(list) != 1 || list[0].Author == nil {
		t.Fatalf("expected 1 pending with author, got %+v", list)
	}
	if list[0].Author.Email != "alice@example.com" {
		t.Fatalf("review queue must keep the author email, got %q", list[0].Author.Email)
	}
}

// TestChangePasswordRevokesOldTokens verifies the token-version bump: after a
// password change the previously issued JWT is rejected, and the fresh token
// returned to the caller still works.
func TestChangePasswordRevokesOldTokens(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	u := makeUser(t, db, "carol", "carol@example.com", models.RoleUser)
	oldTok, _, err := svc.SignToken(u.ID, u.Role, u.TokenVersion)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	newTok, _, err := svc.ChangePassword(ctx, u, ChangePasswordInput{CurrentPassword: "Zp9!qLm2xR", NewPassword: "newsecret123"})
	if err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if newTok == "" || newTok == oldTok {
		t.Fatalf("expected a fresh token, got %q", newTok)
	}
	if _, err := svc.Authenticate(ctx, oldTok); err == nil {
		t.Fatal("old token must be revoked after a password change")
	}
	if _, err := svc.Authenticate(ctx, newTok); err != nil {
		t.Fatalf("fresh token must be valid: %v", err)
	}
}
