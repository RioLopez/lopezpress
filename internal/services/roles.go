package services

import (
	"echo-content-api/internal/models"
)

// Centralised role-privilege helpers. Every "can X do Y" decision in the
// service layer goes through one of these functions so the three-tier
// hierarchy (root > admin > user) lives in a single, auditable place.
//
// The hierarchy:
//   - root:   single super-admin. May do anything. May be created only by
//     the bootstrap seeder / migration, never by an API path.
//   - admin:  ordinary admin. May manage users and content, but cannot
//     promote/demote an admin, cannot touch root, and cannot see the
//     banned vault.
//   - user:   regular member. May author posts (which enter the review
//     queue) and edit/delete their own.

// IsRoot reports whether u is the single super-admin.
func IsRoot(u models.User) bool { return u.Role == models.RoleRoot }

// IsAdminOrRoot reports whether u is an admin or the root.
func IsAdminOrRoot(u models.User) bool {
	return u.Role == models.RoleAdmin || u.Role == models.RoleRoot
}

// canManageUser reports whether actor may manage target via the user-
// management endpoints (create / update / delete / disable / reset-password).
//
// Rules:
//   - root may manage everyone except the root itself (root is immutable).
//   - admin may manage plain users only — not admins, not root, not self.
//   - user may not manage anyone.
func canManageUser(actor, target models.User) bool {
	if target.Role == models.RoleRoot {
		return false // the unique root is immutable through any API path
	}
	if target.ID == actor.ID {
		return false // self-protection
	}
	switch actor.Role {
	case models.RoleRoot:
		return true
	case models.RoleAdmin:
		// An admin manages plain users only.
		return target.Role == models.RoleUser
	default:
		return false
	}
}

// canReviewPost reports whether actor may approve / reject / ban / send-back
// a post currently in the review queue or the banned vault.
//
// Rules (spec: "user 被 admin 或 root 审核；admin 被 root 审核"):
//   - root may review / ban / send-back any post.
//   - admin may review posts whose author is a plain user, and may ban
//     plain users' posts.
//   - user may not review or ban anything.
func canReviewPost(actor, author models.User, post models.Post) bool {
	switch actor.Role {
	case models.RoleRoot:
		return true
	case models.RoleAdmin:
		// Admins handle only plain-user content.
		return author.Role == models.RoleUser
	default:
		return false
	}
}
