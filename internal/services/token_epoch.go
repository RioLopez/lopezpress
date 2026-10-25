package services

import (
	"context"
	"crypto/rand"
	"math/big"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- token-version epoch across restores -----------------------------------
//
// A restore or import rewrites the users table, and with it the
// token_version column that decides which JWTs are still valid. The naive
// fix — "bump everyone by one after the copy" — is not enough, and the reason
// is a race the bump itself creates:
//
//	snapshot taken        -> every account sits at ver = N
//	attacker's token live -> signed at ver = N+1 (issued after the snapshot,
//	                         before the operator's logout-all)
//	restore               -> column rewound to N, then +1  =>  N+1
//	attacker's token      -> matches again
//
// R4-01 walked exactly that path and revived an already-revoked session. The
// fix is to stop reasoning about the snapshot's values at all: read the live
// ceiling BEFORE the table is rewritten, then land every account above it by
// a margin, so nothing issued before the operation can match. The margin is
// random so two restores of the same snapshot produce different floors — a
// replay cannot be aimed at a value the attacker can predict.

// tokenEpochJump is the fixed part of the margin. Large enough that no
// realistic sequence of password changes / logouts between two restores can
// climb back over it, small enough to stay far from int64 overflow.
const tokenEpochJump = 1000

// maxTokenVersion returns the highest token_version currently stored, or 0 on
// an empty table. Called before the users table is replaced.
func (s *Service) maxTokenVersion(ctx context.Context) int64 {
	var n int64
	// COALESCE so an empty table yields 0 rather than NULL scanning into 0
	// silently (which would also work, but only by accident).
	_ = s.db.WithContext(ctx).Model(&models.User{}).
		Select("COALESCE(MAX(token_version), 0)").Scan(&n).Error
	if n < 0 {
		return 0
	}
	return n
}

// tokenRevokeFloor computes the token_version every account is raised to after
// a restore/import. liveMax is the ceiling read before the rewrite.
func tokenRevokeFloor(liveMax int64) int64 {
	jitter := int64(0)
	if n, err := rand.Int(rand.Reader, big.NewInt(tokenEpochJump)); err == nil {
		jitter = n.Int64()
	}
	return liveMax + tokenEpochJump + jitter
}

// raiseAllTokenVersions lands every account on the post-restore floor. tx is
// the transaction that has just rewritten the users table.
func raiseAllTokenVersions(tx *gorm.DB, floor int64) error {
	return tx.Model(&models.User{}).Where("1 = 1").
		UpdateColumn("token_version", floor).Error
}
