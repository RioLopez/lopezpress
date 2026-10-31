package config

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/models"
)

// newRecoverTestDB opens an in-memory database carrying just the users table.
func newRecoverTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("migrate users: %v", err)
	}
	return db
}

// TestResetRootPasswordForCLIRotates covers the happy path plus every side
// effect the recovery path promises:
// a new random password that verifies, forced rotation, and a bumped
// token_version that kills existing sessions.
func TestResetRootPasswordForCLIRotates(t *testing.T) {
	db := newRecoverTestDB(t)

	oldHash, err := models.HashPassword("original-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	root := models.User{
		Username: "admin", Email: "admin@example.com", PasswordHash: oldHash,
		Role: models.RoleRoot, TokenVersion: 3,
	}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root: %v", err)
	}

	pwd, err := ResetRootPasswordForCLI(db)
	if err != nil {
		t.Fatalf("ResetRootPasswordForCLI: %v", err)
	}
	if len(pwd) != 22 {
		t.Fatalf("new password len = %d, want 22 (%q)", len(pwd), pwd)
	}

	var got models.User
	if err := db.First(&got, "id = ?", root.ID).Error; err != nil {
		t.Fatalf("reload root: %v", err)
	}
	if !models.VerifyPassword(pwd, got.PasswordHash) {
		t.Fatal("new password does not verify against stored hash")
	}
	if models.VerifyPassword("original-password", got.PasswordHash) {
		t.Fatal("old password still works after reset")
	}
	if !got.MustChangePassword {
		t.Fatal("must_change_password not set: operator would keep a never-chosen password")
	}
	if got.TokenVersion != 4 {
		t.Fatalf("token_version = %d, want 4 (pre-reset sessions must die)", got.TokenVersion)
	}
	if got.Role != models.RoleRoot || got.Disabled {
		t.Fatalf("reset changed role/state: role=%q disabled=%v", got.Role, got.Disabled)
	}
}

// TestResetRootPasswordForCLITouchesOnlyRoot makes sure the recovery path
// cannot be abused as a blanket "reset whoever" tool.
func TestResetRootPasswordForCLITouchesOnlyRoot(t *testing.T) {
	db := newRecoverTestDB(t)

	rootHash, _ := models.HashPassword("root-password")
	userHash, _ := models.HashPassword("user-password")
	root := models.User{Username: "admin", Email: "a@example.com", PasswordHash: rootHash, Role: models.RoleRoot}
	user := models.User{Username: "writer", Email: "w@example.com", PasswordHash: userHash, Role: models.RoleUser}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root: %v", err)
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := ResetRootPasswordForCLI(db); err != nil {
		t.Fatalf("ResetRootPasswordForCLI: %v", err)
	}

	var got models.User
	if err := db.First(&got, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if !models.VerifyPassword("user-password", got.PasswordHash) {
		t.Fatal("non-root user's password was modified by a root reset")
	}
	if got.TokenVersion != 0 {
		t.Fatalf("non-root token_version = %d, want 0", got.TokenVersion)
	}
}

// TestResetRootPasswordForCLIWithoutRoot pins AC-2.2: recovery must never
// fabricate a root row (red line R2).
func TestResetRootPasswordForCLIWithoutRoot(t *testing.T) {
	db := newRecoverTestDB(t)

	adminHash, _ := models.HashPassword("admin-password")
	if err := db.Create(&models.User{
		Username: "plainadmin", Email: "pa@example.com", PasswordHash: adminHash, Role: models.RoleAdmin,
	}).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}

	if _, err := ResetRootPasswordForCLI(db); !errors.Is(err, ErrNoRootAccount) {
		t.Fatalf("err = %v, want ErrNoRootAccount", err)
	}

	var count int64
	db.Model(&models.User{}).Where("role = ?", models.RoleRoot).Count(&count)
	if count != 0 {
		t.Fatalf("root rows = %d, want 0 (reset must not promote anyone)", count)
	}
}
