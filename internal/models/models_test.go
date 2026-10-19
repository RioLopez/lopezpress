package models

import (
	"testing"
)

// TestHashAndPasswordVerifyRoundTrip verifies HashPassword produces a
// verifiable bcrypt hash and that VerifyPassword accepts the right password
// but rejects the wrong one.
func TestHashAndPasswordVerifyRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "correct-horse" {
		t.Fatalf("hash must not equal the plaintext")
	}
	if !VerifyPassword("correct-horse", hash) {
		t.Fatalf("VerifyPassword should accept the correct password")
	}
	if VerifyPassword("wrong", hash) {
		t.Fatalf("VerifyPassword must reject a wrong password")
	}
}

// TestHashPasswordDistinctPerCall verifies bcrypt salting: hashing the same
// plaintext twice yields different hashes.
func TestHashPasswordDistinctPerCall(t *testing.T) {
	a, err1 := HashPassword("same")
	b, err2 := HashPassword("same")
	if err1 != nil || err2 != nil {
		t.Fatalf("hash errors: %v / %v", err1, err2)
	}
	if a == b {
		t.Fatalf("expected distinct hashes (salted), got identical")
	}
}

// TestUserBeforeCreateDefaults verifies the GORM hook assigns a UUID and
// defaults the role to "user" when unset.
func TestUserBeforeCreateDefaults(t *testing.T) {
	u := User{}
	if err := u.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if u.ID == "" {
		t.Fatalf("expected a generated UUID, got empty")
	}
	if u.Role != "user" {
		t.Fatalf("expected default role user, got %q", u.Role)
	}

	// Pre-set values must be preserved.
	u2 := User{ID: "keep-me", Role: "admin"}
	if err := u2.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if u2.ID != "keep-me" || u2.Role != "admin" {
		t.Fatalf("BeforeCreate clobbered set values: %+v", u2)
	}
}

// TestPostBeforeCreateDefaults verifies the post hook sets a UUID and a
// "published" status when unset, and preserves explicit status.
func TestPostBeforeCreateDefaults(t *testing.T) {
	p := Post{}
	if err := p.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if p.ID == "" {
		t.Fatalf("expected a generated UUID, got empty")
	}
	if p.Status != "published" {
		t.Fatalf("expected default status published, got %q", p.Status)
	}

	p2 := Post{ID: "x", Status: "draft"}
	if err := p2.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate: %v", err)
	}
	if p2.Status != "draft" || p2.ID != "x" {
		t.Fatalf("BeforeCreate clobbered set values: %+v", p2)
	}
}
