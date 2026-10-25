package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

func strPtr(s string) *string { return &s }

// TestUpdateMe_UsernameEmail covers the profile edit path that lets a caller
// change their own username / email (with uniqueness + format validation) and
// clear avatar / bio via an explicit empty string.
func TestUpdateMe_UsernameEmail(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// Seed two users: the caller and a second user to collide against.
	hash, _ := models.HashPassword("password123")
	caller := models.User{Username: "alice", Email: "alice@example.com", PasswordHash: hash, Role: models.RoleUser}
	if err := db.Create(&caller).Error; err != nil {
		t.Fatalf("seed caller: %v", err)
	}
	other := models.User{Username: "bob", Email: "bob@example.com", PasswordHash: hash, Role: models.RoleUser}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("seed other: %v", err)
	}

	// 1. Change username and email to fresh values.
	u, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{
		Username: strPtr("alice2"),
		Email:    strPtr("alice2@example.com"),
	})
	if err != nil {
		t.Fatalf("update username/email: %v", err)
	}
	if u.Username != "alice2" || u.Email != "alice2@example.com" {
		t.Fatalf("got %q/%q, want alice2/alice2@example.com", u.Username, u.Email)
	}

	// 2. Username collision against another user -> conflict.
	if _, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{Username: strPtr("bob")}); err == nil {
		t.Fatalf("expected conflict for duplicate username, got nil")
	}

	// 3. Email collision -> conflict.
	if _, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{Email: strPtr("bob@example.com")}); err == nil {
		t.Fatalf("expected conflict for duplicate email, got nil")
	}

	// 4. Malformed email -> rejected.
	if _, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{Email: strPtr("not-an-email")}); err == nil {
		t.Fatalf("expected error for malformed email, got nil")
	}

	// 5. Empty username / email -> rejected (required fields).
	if _, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{Username: strPtr("")}); err == nil {
		t.Fatalf("expected error for empty username, got nil")
	}
	if _, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{Email: strPtr("")}); err == nil {
		t.Fatalf("expected error for empty email, got nil")
	}

	// 6. Set avatar/bio then clear them with explicit empty strings.
	//
	// Since spec 36 an avatar must point at an image that is actually in the
	// library — a free-form string let any member render a tracking pixel on
	// every article they wrote. Seed the row the avatar will reference.
	avatarRow := models.UploadFile{
		Name: "test-avatar.png", LiveKey: "test-avatar-hash",
		Size: 64, Width: 8, Height: 8, Mime: "image/png", Kind: UploadKindAvatar,
	}
	if err := db.Create(&avatarRow).Error; err != nil {
		t.Fatalf("seed avatar upload: %v", err)
	}
	if _, err := svc.UpdateMe(ctx, caller.ID, UpdateMeInput{
		Avatar: strPtr("/uploads/test-avatar.png"),
		Bio:    strPtr("hello"),
	}); err != nil {
		t.Fatalf("set avatar/bio: %v", err)
	}
	u, err = svc.UpdateMe(ctx, caller.ID, UpdateMeInput{
		Avatar: strPtr(""),
		Bio:    strPtr(""),
	})
	if err != nil {
		t.Fatalf("clear avatar/bio: %v", err)
	}
	if u.Avatar != "" || u.Bio != "" {
		t.Fatalf("got avatar=%q bio=%q, want both cleared", u.Avatar, u.Bio)
	}

	// 7. Nil fields leave values unchanged (partial save).
	u, err = svc.UpdateMe(ctx, caller.ID, UpdateMeInput{DisplayName: strPtr("Alice Display")})
	if err != nil {
		t.Fatalf("partial update: %v", err)
	}
	if u.Username != "alice2" || u.Email != "alice2@example.com" {
		t.Fatalf("partial save clobbered username/email: %q/%q", u.Username, u.Email)
	}
	if u.DisplayName != "Alice Display" {
		t.Fatalf("display name = %q, want Alice Display", u.DisplayName)
	}
}
