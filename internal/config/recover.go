package config

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// ErrNoRootAccount is returned by ResetRootPasswordForCLI when the database
// holds no root row at all. Recovery cannot invent one: root must originate
// from seeding or the 0003 migration, never from a reset path (red line R2).
var ErrNoRootAccount = errors.New("no root account found in this database")

// ResetRootPasswordForCLI rotates the unique root account's password and
// returns the new plaintext exactly once, for the caller to print in a console
// banner.
//
// This is the offline escape hatch for the situation created by the P0-1 fix:
// an admin may no longer reset root, and root's own password change requires
// the current password, so a forgotten root password used to be unrecoverable
// through the API. Whoever can read the database file can already take over
// the site by editing password_hash directly; this function just makes that
// operation correct (bcrypt, forced rotation, session revocation) instead of
// hand-rolled.
//
// Deliberately NOT exposed over HTTP: a remote reset endpoint would be a
// takeover channel, which is exactly what "root cannot be created or promoted
// through any API" exists to prevent.
func ResetRootPasswordForCLI(db *gorm.DB) (string, error) {
	var root models.User
	// Order by created_at: the 0003 migration guarantees uniqueness, but if a
	// legacy database somehow carries more than one root we still deterministically
	// pick the oldest (same tie-break rule the migration uses).
	if err := db.Where("role = ?", models.RoleRoot).Order("created_at ASC").First(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNoRootAccount
		}
		return "", fmt.Errorf("look up root account: %w", err)
	}

	password := randomPassword()
	hash, err := models.HashPassword(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	// One UPDATE covers three security requirements:
	//   password_hash         -> the new credential
	//   must_change_password  -> force the operator to pick a memorable one
	//   token_version + 1     -> every JWT issued before this reset is dead
	// The last one matters because a lost password often means someone else
	// may be holding a live session; recovery is also an eviction.
	if err := db.Model(&models.User{}).Where("id = ?", root.ID).Updates(map[string]any{
		"password_hash":        hash,
		"must_change_password": true,
		"token_version":        gorm.Expr("token_version + 1"),
	}).Error; err != nil {
		return "", fmt.Errorf("update root password: %w", err)
	}

	return password, nil
}
