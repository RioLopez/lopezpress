package services

import (
	"context"
	"testing"

	"lopezpress/internal/models"
)

// R4-01: a restore/import used to land every account on `snapshot value + 1`.
// That is exactly the version of a token issued AFTER the snapshot was taken
// and BEFORE the operator's "log out all devices" — so restoring a snapshot
// revived a session that had already been revoked. The measured sequence was:
//
//	snapshot at ver=4 → login (ver=5) → logout-all (ver=6, token dead)
//	  → restore the same snapshot → ver back to 5 → the dead token works again
//
// The fix reads the live ceiling before the table is rewritten and lands
// everyone above it by a random margin, so no pre-restore token can match and
// two restores of one snapshot produce different floors.
func TestImportBackupCannotReviveRevokedToken(t *testing.T) {
	svc, db := newTestService(t)
	defer closeDB(db)
	ctx := context.Background()

	root := makeUser(t, db, "root", "root@example.com", "root")
	victim := makeUser(t, db, "victim", "victim@example.com", "user")

	// A token the attacker holds, signed at the CURRENT version.
	stolen, _, err := svc.SignToken(victim.ID, victim.Role, victim.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+stolen); err != nil {
		t.Fatalf("stolen token should work before revocation: %v", err)
	}

	// The operator revokes every session (logout-all).
	if err := svc.RevokeAllSessions(ctx, victim); err != nil {
		t.Fatalf("RevokeAllSessions: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+stolen); err == nil {
		t.Fatalf("stolen token must be dead after logout-all")
	}

	// Now a restore that carries the OLD (pre-revocation) token_version. The
	// dump's user rows are written verbatim, which is what used to rewind the
	// column.
	stale := victim
	stale.TokenVersion = victim.TokenVersion - 1 // what the snapshot captured
	in := ImportData{
		SchemaVersion: BackupSchemaVersion,
		AppVersion:    BackupAppVersion,
		Confirm:       ImportConfirmPhrase,
		Users: []models.User{
			{ID: root.ID, Username: root.Username, Email: root.Email, Role: models.RoleRoot, PasswordHash: root.PasswordHash},
			{ID: stale.ID, Username: stale.Username, Email: stale.Email, Role: models.RoleUser, PasswordHash: stale.PasswordHash, TokenVersion: stale.TokenVersion},
		},
		PasswordHashes: map[string]string{
			root.ID:  root.PasswordHash,
			stale.ID: stale.PasswordHash,
		},
	}
	if _, err := svc.ImportBackup(ctx, root, in, false); err != nil {
		t.Fatalf("import: %v", err)
	}

	// The stolen token must still be dead: the import has to lift every account
	// above the ceiling that existed before it, not merely add one to whatever
	// the dump said.
	if _, err := svc.Authenticate(ctx, "Bearer "+stolen); err == nil {
		t.Fatalf("import must not revive a token revoked before it ran")
	}

	// And a freshly signed token works — the account is usable again.
	var after models.User
	if err := db.Where("id = ?", stale.ID).First(&after).Error; err != nil {
		t.Fatalf("reload victim: %v", err)
	}
	if after.TokenVersion < victim.TokenVersion {
		t.Fatalf("post-import token_version %d must exceed the pre-import ceiling %d",
			after.TokenVersion, victim.TokenVersion)
	}
	fresh, _, err := svc.SignToken(after.ID, after.Role, after.TokenVersion)
	if err != nil {
		t.Fatalf("SignToken after import: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "Bearer "+fresh); err != nil {
		t.Fatalf("fresh token after import should work: %v", err)
	}
}

// Two restores of the same snapshot must not produce the same floor: a
// predictable value would let an attacker pre-sign a token at exactly the
// version the next restore will land on.
func TestTokenRevokeFloorIsNotPredictable(t *testing.T) {
	a := tokenRevokeFloor(7)
	b := tokenRevokeFloor(7)
	if a == b {
		t.Fatalf("two floors for the same ceiling are identical (%d): a replay could aim at it", a)
	}
	if a <= 7 || b <= 7 {
		t.Fatalf("floor must exceed the live ceiling: got %d and %d for ceiling 7", a, b)
	}
}
