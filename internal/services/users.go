package services

import (
	"context"
	"net/http"
	"strconv"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// UserStats returns the caller's post count and total views. Admin stats are
// scoped to the admin's own posts (author_id = user.ID), matching the
// personal-center "this is my page" semantics.
func (s *Service) UserStats(ctx context.Context, user models.User) (UserStats, error) {
	var postsTotal int64
	if err := s.db.WithContext(ctx).
		Model(&models.Post{}).
		Where("author_id = ?", user.ID).
		Count(&postsTotal).Error; err != nil {
		return UserStats{}, newErr(http.StatusInternalServerError, "failed to count posts")
	}

	var viewsTotal int64
	if err := s.db.WithContext(ctx).
		Model(&models.Post{}).
		Where("author_id = ?", user.ID).
		Select("COALESCE(SUM(view_count), 0)").
		Scan(&viewsTotal).Error; err != nil {
		return UserStats{}, newErr(http.StatusInternalServerError, "failed to sum views")
	}

	return UserStats{PostsTotal: postsTotal, ViewsTotal: viewsTotal}, nil
}

// --- user management (admin) ---

// ListUsers returns every account with its admin-disabled flag.
//
// A plain admin sees the root account exists (it must, to avoid "why can't I
// manage this row" confusion) but not its contact address or last login IP:
// those two fields are how an admin would profile the super-admin, and no
// user-management screen needs them.
func (s *Service) ListUsers(ctx context.Context, actor models.User) ([]UserWithDisabled, error) {
	var users []models.User
	if err := s.db.WithContext(ctx).Order("created_at ASC").Find(&users).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load users")
	}
	hideRootDetails := !IsRoot(actor)
	out := make([]UserWithDisabled, 0, len(users))
	for _, u := range users {
		row := UserWithDisabled{
			ID:          u.ID,
			Username:    u.Username,
			Email:       u.Email,
			Role:        u.Role,
			Disabled:    u.Disabled,
			DisplayName: u.DisplayName,
			LastLoginIP: u.LastLoginIP,
			LastLoginAt: u.LastLoginAt,
			CreatedAt:   u.CreatedAt,
		}
		if hideRootDetails && u.Role == models.RoleRoot {
			row.Email = ""
			row.LastLoginIP = ""
		}
		out = append(out, row)
	}
	return out, nil
}

// SetUserDisabled toggles the admin-disabled flag for a user. A user cannot
// disable their own account (would lock themselves out of recovery). The
// unique root account is never disableable. A plain admin may only toggle
// plain users; root may toggle admin and user accounts.
func (s *Service) SetUserDisabled(ctx context.Context, actor models.User, id string, disabled bool) (models.User, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusNotFound, "user not found")
	}
	if id == actor.ID {
		return models.User{}, newErr(http.StatusForbidden, "cannot disable your own account")
	}
	// The unique root account is never disableable.
	if user.Role == models.RoleRoot {
		return models.User{}, newErr(http.StatusForbidden, "the root account cannot be disabled")
	}
	// A plain admin cannot toggle an admin account.
	if actor.Role == models.RoleAdmin && user.Role == models.RoleAdmin {
		return models.User{}, newErr(http.StatusForbidden, "only the root can disable an admin")
	}
	if user.Disabled == disabled {
		return user, nil // no-op
	}
	// R6-01: disabling an account has to end its sessions, not just make the
	// next login fail. Authentication re-reads `disabled` on every request, so
	// a disable takes effect at once — but re-enabling the account (an appeal
	// won, a mistaken ban lifted, a temporary suspension over) used to hand the
	// very same pre-disable JWT its access back, because nothing had bumped
	// token_version. Every other revocation primitive in this codebase — change
	// password, admin reset, logout-all, password reset — bumps it; disable was
	// the one that did not, so "temporarily suspend" was not actually temporary.
	upd := map[string]any{"disabled": disabled}
	if disabled {
		upd["token_version"] = gorm.Expr("token_version + 1")
	}
	if err := s.db.WithContext(ctx).Model(&user).Updates(upd).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update user")
	}
	user.Disabled = disabled
	if disabled {
		user.TokenVersion++
	}
	s.audit(ctx, actor, "set_user_disabled", "user:"+id, "disabled="+strconv.FormatBool(disabled))
	return user, nil
}
