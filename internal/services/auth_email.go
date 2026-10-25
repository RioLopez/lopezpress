package services

import (
	"context"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- the three business flows built on email codes (spec 37) ---
//
// Mechanics (mint / mail / verify / attempt counting) live in email_codes.go.
// This file holds the rules: which purpose a flow may use, what it does to the
// account, and what it refuses to reveal.
//
// Enumeration policy: a request for a code always answers the same, whether or
// not the address belongs to an account. Only a *successful* code check can
// reach the branch that touches the account, and holding a valid code already
// proves the caller owns that inbox — so "this email is not registered" is only
// ever said to somebody who could have read the mail anyway.

// RequestEmailCode is the anonymous endpoint. It deliberately accepts only the
// two unauthenticated purposes: a change-email code must be requested from an
// authenticated session (RequestEmailChange), otherwise an attacker could
// pre-mint a code for a victim's address and wait.
func (s *Service) RequestEmailCode(ctx context.Context, in EmailCodeRequestInput) error {
	purpose := strings.TrimSpace(in.Purpose)
	if purpose == "" {
		purpose = models.EmailPurposeRegister
	}
	if purpose == models.EmailPurposeChangeMail {
		return newErr(http.StatusForbidden, "that purpose requires a signed-in session")
	}
	email := normalizeEmail(in.Email)
	if !isValidEmail(email) {
		return newErr(http.StatusBadRequest, "invalid email")
	}
	return s.IssueEmailCode(ctx, email, purpose, "")
}

// verifyRegisterCode enforces the code requirement at sign-up. It is a no-op
// when the feature is off, so Register keeps working unchanged for every
// existing deployment.
func (s *Service) verifyRegisterCode(ctx context.Context, in RegisterInput) error {
	if !s.emailCodePolicy(ctx).Enabled {
		return nil
	}
	return s.verifyEmailCode(ctx, in.Email, models.EmailPurposeRegister, in.Code)
}

// ResetPasswordByCode finishes a self-service password reset: the code proves
// ownership of the address, so the current password is not required (that is
// the whole point — the user forgot it).
func (s *Service) ResetPasswordByCode(ctx context.Context, in PasswordResetInput) error {
	if !s.emailCodePolicy(ctx).Enabled {
		return newErr(http.StatusForbidden, "email verification is disabled on this site")
	}
	email := normalizeEmail(in.Email)
	if !isValidEmail(email) {
		return newErr(http.StatusBadRequest, "invalid email")
	}
	if len(in.Password) < 8 {
		return newErr(http.StatusBadRequest, "password must be at least 8 characters")
	}
	// Code first: it is the only proof of ownership in this flow.
	if err := s.verifyEmailCode(ctx, email, models.EmailPurposeReset, in.Code); err != nil {
		return err
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("LOWER(email) = ?", email).First(&user).Error; err != nil {
		// The code was valid, so the caller owns the inbox; there is simply no
		// account behind it (it may have been deleted between request and
		// confirm). Saying so is safe and actionable.
		return newErr(http.StatusBadRequest, "no account uses this address")
	}
	hash, err := models.HashPassword(in.Password)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to hash password")
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Bumping token_version revokes every session: a reset usually happens
		// because the account may be compromised, so leaving old tokens alive
		// would defeat it.
		return tx.Model(&models.User{}).Where("id = ?", user.ID).
			Updates(map[string]any{"password_hash": hash, "token_version": gorm.Expr("token_version + 1")}).Error
	})
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to reset password")
	}
	s.audit(ctx, user, "reset_password_by_email", "user:"+user.ID, "via=email_code")
	s.SendMail(ctx, user, MailPasswordChange, map[string]string{
		"ip":   ClientIPFromContext(ctx),
		"time": time.Now().Format("2006-01-02 15:04:05"),
	})
	return nil
}

// RequestEmailChange mails a confirmation code to the *new* address. Sending it
// there (not to the old one) is what makes it a proof: whoever reads that inbox
// controls the address the account is about to use.
func (s *Service) RequestEmailChange(ctx context.Context, me models.User, newEmail string) error {
	if !s.emailCodePolicy(ctx).Enabled {
		return newErr(http.StatusForbidden, "email verification is disabled on this site")
	}
	email := normalizeEmail(newEmail)
	if !isValidEmail(email) {
		return newErr(http.StatusBadRequest, "invalid email")
	}
	if email == strings.ToLower(me.Email) {
		return newErr(http.StatusBadRequest, "that is already your email address")
	}
	var dup models.User
	if s.db.WithContext(ctx).Where("LOWER(email) = ? AND id <> ?", email, me.ID).
		First(&dup).RowsAffected == 1 {
		return newErr(http.StatusConflict, "email already in use")
	}
	return s.IssueEmailCode(ctx, email, models.EmailPurposeChangeMail, me.ID)
}

// ConfirmEmailChange applies the new address after the code check. Username and
// password are untouched, and the current session survives: a JWT carries the
// user id, not the address, so there is nothing to invalidate.
func (s *Service) ConfirmEmailChange(ctx context.Context, me models.User, in EmailChangeInput) (models.User, error) {
	if !s.emailCodePolicy(ctx).Enabled {
		return models.User{}, newErr(http.StatusForbidden, "email verification is disabled on this site")
	}
	email := normalizeEmail(in.Email)
	if !isValidEmail(email) {
		return models.User{}, newErr(http.StatusBadRequest, "invalid email")
	}
	// Cheap check before consuming the code: if somebody else grabbed the
	// address meanwhile, the user should keep their code usable elsewhere.
	var dup models.User
	if s.db.WithContext(ctx).Where("LOWER(email) = ? AND id <> ?", email, me.ID).
		First(&dup).RowsAffected == 1 {
		return models.User{}, newErr(http.StatusConflict, "email already in use")
	}
	if err := s.verifyEmailCode(ctx, email, models.EmailPurposeChangeMail, in.Code); err != nil {
		return models.User{}, err
	}
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", me.ID).
		Update("email", email).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to update email")
	}
	var fresh models.User
	if err := s.db.WithContext(ctx).Where("id = ?", me.ID).First(&fresh).Error; err != nil {
		return models.User{}, newErr(http.StatusInternalServerError, "failed to reload account")
	}
	s.audit(ctx, fresh, "change_email", "user:"+fresh.ID, "from="+me.Email+"; to="+email+" (verified)")
	// Tell the *old* address what happened: if this was not the owner, the
	// previous inbox is the only place that can still notice.
	if old := strings.TrimSpace(me.Email); old != "" && !strings.EqualFold(old, email) {
		s.SendMail(ctx, models.User{ID: me.ID, Email: old, Username: me.Username},
			MailEmailChanged, map[string]string{
				"old":  old,
				"new":  email,
				"ip":   ClientIPFromContext(ctx),
				"time": time.Now().Format("2006-01-02 15:04:05"),
			})
	}
	return fresh, nil
}
