package services

import (
	"context"
	"net/http"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// CreateUser creates an account on behalf of an admin. It is NOT subject to
// the allow_register gate (admins can always provision users).
//
// Role rules (three-tier model, spec: role model):
//   - Only a root actor may create accounts with role "admin".
//   - A plain admin may create accounts but only with role "user".
//   - No creation path may ever produce "root" — the single root is
//     established at bootstrap / migration and is not user-mutable.
//   - An unknown / empty role falls back to "user".
func (s *Service) CreateUser(ctx context.Context, actor models.User, in CreateUserInput) (models.User, error) {
	if in.Username == "" || in.Email == "" || in.Password == "" {
		return models.User{}, newErr(http.StatusBadRequest, "username, email and password are required")
	}
	if err := ValidatePasswordForUser(in.Password, in.Username, in.Email); err != nil {
		return models.User{}, err
	}
	if !isValidUsername(in.Username) {
		return models.User{}, newErr(http.StatusBadRequest, "username must be 3-32 characters: letters, digits, _ or -, starting with a letter or digit")
	}
	if !isValidEmail(in.Email) {
		return models.User{}, newErr(http.StatusBadRequest, "invalid email")
	}
	// Service-level privilege check (defense in depth): only admin-or-root may
	// provision accounts. The route table already enforces this (P4), but the
	// service is the authoritative layer and must not trust the transport to
	// have done it.
	if !IsAdminOrRoot(actor) {
		return models.User{}, newErr(http.StatusForbidden, "only admin or root can create users")
	}
	var existing models.User
	if s.db.WithContext(ctx).Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", in.Username, in.Email).
		First(&existing).RowsAffected == 1 {
		return models.User{}, newErr(http.StatusConflict, "username or email already in use")
	}
	// Resolve the requested role against the actor's privilege.
	var role string
	switch in.Role {
	case "", models.RoleUser:
		role = models.RoleUser
	case models.RoleAdmin:
		// Only a root may elevate to admin.
		if actor.Role != models.RoleRoot {
			return models.User{}, newErr(http.StatusForbidden, "only the root can create an admin")
		}
		role = models.RoleAdmin
	case models.RoleRoot:
		// The single root is never created through this endpoint.
		return models.User{}, newErr(http.StatusForbidden, "the root role cannot be assigned here")
	default:
		// Unknown role falls back to the safe default.
		role = models.RoleUser
	}
	hash, err := models.HashPassword(in.Password)
	if err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to hash password")
	}
	u := models.User{Username: in.Username, Email: in.Email, PasswordHash: hash, Role: role}
	if err := s.db.WithContext(ctx).Create(&u).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to create user")
	}
	s.audit(ctx, actor, "create_user", "user:"+u.ID, "username="+in.Username+"; role="+u.Role)
	return u, nil
}

// UpdateUser changes a user's role with hierarchical protection.
//
// Role changes:
//   - The target role must be one of user/admin/root; anything else → 400.
//   - root is the unique super-admin and is UNMUTABLE through this path:
//     attempting to demote, disable, or retarget root → 403.
//   - Only a root actor may grant admin. A plain admin cannot elevate anyone
//     to admin, and cannot touch another admin or root.
//   - An admin may demote a user to user (no-op), but may not demote an
//     admin or themselves below admin.
//
// Enable/disable is NOT handled here. It used to be, and that second door was
// strictly weaker than the one it duplicated: it refused disabling an admin
// but happily re-enabled an admin that root had shut down, and it wrote the
// whole row back from a stale snapshot. PUT /api/users/{id}/disabled
// (SetUserDisabled) is the only path, and it applies the full rule set.
func (s *Service) UpdateUser(ctx context.Context, actor models.User, id string, in UpdateUserInput) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}
	prevRole := user.Role

	if in.Role != "" {
		if err := applyRoleChange(&user, actor, in.Role); err != nil {
			return models.User{}, err
		}
	}
	// Nothing to write: return the record as-is. Doing this before the UPDATE
	// keeps "PUT with an empty body against the root" a 200 no-op instead of a
	// conflict, which is what it was before the row guard existed.
	if user.Role == prevRole {
		return user, nil
	}

	// Column-level update, not Save. A whole-row write echoes back the snapshot
	// read at the top of this function, so a password reset or a disable
	// landing in between would be silently overwritten with stale values —
	// quietly undoing a session revocation. `role <> 'root'` makes the unique
	// root immutable even if something raced the check above.
	res := s.db.WithContext(ctx).Model(&models.User{}).
		Where("id = ? AND role <> ?", id, models.RoleRoot).
		Update("role", user.Role)
	if res.Error != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update user")
	}
	if res.RowsAffected == 0 {
		return models.User{}, newErr(http.StatusConflict, "user changed while you were editing; reload and try again")
	}
	s.audit(ctx, actor, "update_user", "user:"+id, "role="+prevRole+"->"+in.Role)
	return user, nil
}

// applyRoleChange validates one role transition and applies it to user.
//
// It receives the whole user rather than just its role so that every guard
// reads the same snapshot: the unique root, the admin hierarchy and the
// self-demotion rule all depend on the target's current role, and spreading
// that knowledge across callers is how the three-tier model drifts apart.
func applyRoleChange(user *models.User, actor models.User, newRole string) error {
	// The root role can never be assigned or retargeted here.
	if newRole == models.RoleRoot {
		return newErr(http.StatusForbidden, "the root role cannot be assigned")
	}
	if newRole != models.RoleAdmin && newRole != models.RoleUser {
		return newErr(http.StatusBadRequest, "invalid role")
	}
	// Protect the unique root: no demotion, no retarget.
	if user.Role == models.RoleRoot {
		return newErr(http.StatusForbidden, "the root account cannot be demoted")
	}
	// Hierarchical privilege checks.
	switch {
	case newRole == models.RoleAdmin:
		// Granting admin is a root-only power.
		if actor.Role != models.RoleRoot {
			return newErr(http.StatusForbidden, "only the root can promote to admin")
		}
	case actor.Role == models.RoleAdmin:
		// A plain admin may manage plain users only — never an admin.
		if user.Role == models.RoleAdmin {
			return newErr(http.StatusForbidden, "admin cannot modify another admin")
		}
	}
	// Self-demotion guard: an admin demoting their own account below admin
	// would break recovery; root is already protected above.
	if user.ID == actor.ID && newRole != actor.Role {
		return newErr(http.StatusForbidden, "cannot change your own role")
	}
	user.Role = newRole
	return nil
}

// ResetPassword sets a new password hash for a user on behalf of an admin.
// The actor is checked against the three-tier hierarchy: no caller may reset
// the unique root's password, and a plain admin may only reset plain users
// (never another admin, never self). This mirrors the other user-management
// endpoints, which all route through canManageUser.
func (s *Service) ResetPassword(ctx context.Context, actor models.User, id, password string) error {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return newErr(http.StatusNotFound, "user not found")
	}
	// Hierarchical guard: reuse the single authority for "may actor manage
	// target" so this endpoint can never diverge from UpdateUser/DeleteUser.
	// It runs BEFORE the password policy on purpose: an admin asking to reset
	// the root's password must be told they may not, not that their proposed
	// password is weak — otherwise the answer describes a request that was
	// never going to be honoured.
	if !canManageUser(actor, user) {
		return newErr(http.StatusForbidden, "you do not have permission to reset this password")
	}
	// Password policy after the lookup, not before: the context check needs the
	// account's own username and address. This endpoint is admin-only, so the
	// order leaks nothing an admin cannot already read from the user list.
	if err := ValidatePasswordForUser(password, user.Username, user.Email); err != nil {
		return err
	}
	hash, err := models.HashPassword(password)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to hash password")
	}
	// Bump the target's token version so an admin-forced reset actually kicks
	// every session the account already holds (otherwise a stolen/left-open
	// JWT would keep working for up to its 24h lifetime even after the reset).
	if err := s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"password_hash": hash,
		"token_version": gorm.Expr("token_version + 1"),
	}).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to reset password")
	}
	s.audit(ctx, actor, "reset_password", "user:"+id, "")
	return nil
}

// ChangePassword lets the caller change their own password. It verifies the
// current password, enforces a minimum new-password length, and clears the
// must_change_password flag so the flag can never re-trigger after the user
// has picked a password of their own. Used by the first-boot admin flow
//
// It also bumps the user's TokenVersion, which revokes every JWT issued
// before the change (a compromised token stops working the moment the owner
// rotates the password) — and then issues a FRESH token at the new version so
// the caller's own session keeps working without a second login. The new
// token is returned to the handler, which hands it back to the client.
func (s *Service) ChangePassword(ctx context.Context, user models.User, in ChangePasswordInput) (string, time.Time, error) {
	if !models.VerifyPassword(in.CurrentPassword, user.PasswordHash) {
		return "", time.Time{}, newErr(http.StatusUnauthorized, "current password is incorrect")
	}
	if err := ValidatePasswordForUser(in.NewPassword, user.Username, user.Email); err != nil {
		return "", time.Time{}, err
	}
	hash, err := models.HashPassword(in.NewPassword)
	if err != nil {
		return "", time.Time{}, newErr(http.StatusInternalServerError, "failed to hash password")
	}
	newVer := user.TokenVersion + 1
	upd := map[string]any{"password_hash": hash, "token_version": newVer}
	if user.MustChangePassword {
		upd["must_change_password"] = false
	}
	if err := s.db.WithContext(ctx).Model(&user).Updates(upd).Error; err != nil {
		return "", time.Time{}, newErr(http.StatusInternalServerError, "failed to change password")
	}
	// Security notification to the owner (best-effort; a broken SMTP never
	// fails the password change itself).
	s.SendMail(ctx, user, MailPasswordChange, map[string]string{
		"time": time.Now().Format("2006-01-02 15:04 UTC"),
	})
	// An admin-forced reset was already audited; a self-service change being
	// invisible in the same trail was an asymmetry worth closing.
	s.audit(ctx, user, "change_password", "user:"+user.ID, "")
	// Re-issue at the new version so the caller stays signed in while every
	// older token is now rejected by Authenticate.
	token, expires, tErr := s.SignToken(user.ID, user.Role, newVer)
	if tErr != nil {
		return "", time.Time{}, newErr(http.StatusInternalServerError, "failed to issue token")
	}
	return token, expires, nil
}

// RevokeAllSessions invalidates every JWT the user currently holds by
// incrementing their TokenVersion. Tokens embed the version at sign time, so
// any previously issued token fails the Authenticate version check on the
// next request — this is the "log out all devices" primitive for an
// otherwise-stateless JWT scheme. The caller's own token is invalidated too,
// so the client must clear its local session afterwards.
func (s *Service) RevokeAllSessions(ctx context.Context, user models.User) error {
	if err := s.db.WithContext(ctx).Model(&user).
		Update("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to revoke sessions")
	}
	return nil
}

// DeleteUser removes a user with hierarchical protection:
//   - cannot delete self;
//   - the unique root is undeletable by any path;
//   - the last admin cannot be deleted (admin/root accounts only);
//   - an admin cannot delete another admin (only root can);
//   - cannot delete a user that still owns posts.
func (s *Service) DeleteUser(ctx context.Context, actor models.User, id string) error {
	if id == actor.ID {
		return newErr(http.StatusBadRequest, "cannot delete your own account")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return newErr(http.StatusNotFound, "user not found")
	}
	// The unique root account is undeletable.
	if user.Role == models.RoleRoot {
		return newErr(http.StatusForbidden, "the root account cannot be deleted")
	}
	// A plain admin cannot delete another admin (or root).
	if user.Role == models.RoleAdmin && actor.Role != models.RoleRoot {
		return newErr(http.StatusForbidden, "only the root can delete an admin")
	}
	// Preserve the last-admin invariant for admin accounts.
	if user.Role == models.RoleAdmin {
		var adminCount int64
		s.db.WithContext(ctx).Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&adminCount)
		if adminCount <= 1 {
			return newErr(http.StatusBadRequest, "cannot delete the last admin")
		}
	}
	var postCount int64
	// Unscoped so soft-deleted (trashed) posts are also counted — otherwise a
	// user whose only posts live in the trash could be deleted, leaving those
	// rows with an orphaned author_id that the trash UI cannot resolve.
	s.db.WithContext(ctx).Unscoped().Model(&models.Post{}).Where("author_id = ?", id).Count(&postCount)
	if postCount > 0 {
		return newErr(http.StatusConflict, "please transfer or delete this user's posts first")
	}
	// The avatar edge in the image ledger is owned by the user row, not by a
	// post, so nothing else releases it. Left behind, that image is pinned at
	// ref_count = 1 for an account that no longer exists — the gallery never
	// offers it for deletion again. The predicate is "user-level edge"
	// (empty post_id) rather than kind = 'avatar' so any future user-owned slot
	// inherits the same cleanup. Post edges need no handling here: the count
	// above already refused the delete while any post references this author.
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var uploadIDs []string
		if err := tx.Model(&models.UploadRef{}).
			Where("post_id = '' AND user_id = ?", id).Pluck("upload_id", &uploadIDs).Error; err != nil {
			return err
		}
		if err := tx.Where("post_id = '' AND user_id = ?", id).Delete(&models.UploadRef{}).Error; err != nil {
			return err
		}
		if err := recountUploadRefsTx(tx, uploadIDs); err != nil {
			return err
		}
		return tx.Delete(&user).Error
	})
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete user")
	}
	s.audit(ctx, actor, "delete_user", "user:"+id, "username="+user.Username)
	return nil
}

// ListUserLogins returns the most recent login_log rows for a user.
