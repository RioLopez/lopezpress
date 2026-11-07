package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- one-time email codes (experimental) ---
//
// This file owns the mechanics: mint a code, store its HMAC, mail it, and
// verify a submitted one. The business rules of the three flows that use it
// (register, password reset, email change) live in auth_email.go.
//
// Threat model, because a six-digit code is only as strong as the guards
// around it:
//
//   - Guessing: max 5 attempts per code, then it is burned. At one request per
//     attempt that is a 1-in-200,000 chance, and the account lockout on the
//     login path is separate.
//   - Replay: a code is single-use (UsedAt) and bound to one purpose, so a
//     register code cannot reset a password.
//   - Enumeration: asking for a code always answers the same way whether or
//     not the address exists. The flows in auth_email.go honour that by never
//     branching on "user not found" in the response.
//   - Database leak: the stored value is an HMAC keyed by JWT_SECRET, so a
//     dumped table does not let anyone compute the codes.

const (
	// EmailCodeMaxAttempts is how many wrong guesses a single code tolerates.
	EmailCodeMaxAttempts = 5

	emailCodeTTLDefault      = 10 * time.Minute
	emailCodeCooldownDefault = 60 * time.Second
	// emailCodeRetention is how long spent/expired rows are kept for the
	// audit trail before the opportunistic prune drops them.
	emailCodeRetention = 24 * time.Hour
	// emailCodeSendTimeout caps one SMTP round-trip for a code mail. Shorter
	// than the generic 30s because the visitor is staring at a spinner.
	emailCodeSendTimeout = 15 * time.Second
)

// emailPurposeValid reports whether p is one of the three flows. Anything else
// is rejected before touching the database, so a hand-built request cannot
// mint codes for an unknown purpose (and cannot make the verify step accept a
// code that belongs to another flow).
func emailPurposeValid(p string) bool {
	switch p {
	case models.EmailPurposeRegister, models.EmailPurposeReset, models.EmailPurposeChangeMail:
		return true
	}
	return false
}

// emailCodePolicy reads the site config and clamps the two timings.
//
// Clamping here (not only on save) matters because a database restored from an
// older backup can carry values that predate the validation.
func (s *Service) emailCodePolicy(ctx context.Context) EmailCodePolicy {
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return EmailCodePolicy{Enabled: false, TTL: emailCodeTTLDefault, Cooldown: emailCodeCooldownDefault}
	}
	p := EmailCodePolicy{
		Enabled:  cfg.EmailVerifyEnabled,
		TTL:      time.Duration(cfg.EmailCodeTTLMin) * time.Minute,
		Cooldown: time.Duration(cfg.EmailCodeCooldown) * time.Second,
	}
	if p.TTL < 2*time.Minute {
		p.TTL = emailCodeTTLDefault
	}
	if p.TTL > time.Hour {
		p.TTL = time.Hour
	}
	if p.Cooldown < 30*time.Second {
		p.Cooldown = emailCodeCooldownDefault
	}
	if p.Cooldown >= p.TTL {
		// A cooldown longer than the lifetime means the first code expires
		// before a second can be requested: the user is stuck with no way out
		// except waiting, and the "resend" button lies to them.
		p.Cooldown = p.TTL / 2
	}
	return p
}

// hashCode binds the code to the address and the purpose, keyed by the
// deployment secret. The key is what stops a 10^6-entry rainbow table from
// recovering a live code out of a database dump.
func (s *Service) hashCode(email, purpose, code string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.JWTSecret))
	mac.Write([]byte(email + "|" + purpose + "|" + code))
	return hex.EncodeToString(mac.Sum(nil))
}

// newCode returns a uniformly random six-digit string, zero-padded. crypto/rand
// (not math/rand): a predictable code turns the whole feature into decoration.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	digits := n.String()
	if len(digits) < 6 {
		digits = strings.Repeat("0", 6-len(digits)) + digits
	}
	return digits, nil
}

// normalizeEmail is the single canonical form used for lookup, storage and
// hashing. Without it "Foo@Bar.com" and "foo@bar.com" would be two buckets for
// the cooldown and two rows for the same person.
func normalizeEmail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// IssueEmailCode mints a code for (email, purpose) and mails it. It returns the
// code only for the debug log; callers must never put it in a response.
//
// Ordering is deliberate: cooldown -> burn old codes -> send -> persist. The
// send happens before the row is written so a failed SMTP round-trip leaves no
// "valid code that nobody received"; the row is written after a successful send.
func (s *Service) IssueEmailCode(ctx context.Context, email, purpose, userID string) error {
	email = normalizeEmail(email)
	if !isValidEmail(email) {
		return newErr(http.StatusBadRequest, "invalid email")
	}
	if !emailPurposeValid(purpose) {
		return newErr(http.StatusBadRequest, "invalid code purpose")
	}

	pol := s.emailCodePolicy(ctx)
	if !pol.Enabled {
		return newErr(http.StatusForbidden, "email verification is disabled on this site")
	}

	// Per-origin budget first: the address cooldown below cannot see a client
	// that uses a different address on every attempt.
	if !s.allowCodeFromIP(ClientIPFromContext(ctx)) {
		return newErr(http.StatusTooManyRequests, "too many verification emails from this network; try again later")
	}
	if err := s.ensureCodeCooldown(ctx, email, purpose, pol.Cooldown); err != nil {
		return err
	}
	if err := s.requireSMTP(ctx); err != nil {
		return err
	}

	code, err := newCode()
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to generate code")
	}

	// Send first, persist second (see the doc comment).
	if err := s.sendCodeMail(ctx, email, purpose, code, pol.TTL); err != nil {
		return newErr(http.StatusBadGateway, "failed to send the verification email; check the SMTP settings")
	}

	// Any earlier live code for the same address+purpose dies now: two valid
	// codes would double the guessing surface and confuse the user who clicked
	// "resend" and then typed the first mail's number.
	s.db.WithContext(ctx).Model(&models.EmailCode{}).
		Where("email = ? AND purpose = ? AND used_at IS NULL", email, purpose).
		Update("used_at", time.Now())

	row := models.EmailCode{
		Email: email, Purpose: purpose, CodeHash: s.hashCode(email, purpose, code),
		UserID: userID, ExpiresAt: time.Now().Add(pol.TTL),
		IP: ClientIPFromContext(ctx),
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to record code")
	}
	// Debug-only echo: a real deployment runs at warn/info, so the code never
	// reaches the log. Without it, self-hosted testing without a working SMTP
	// inbox is impossible.
	if strings.EqualFold(s.cfg.LogLevel, "debug") {
		log.Printf("EMAIL_CODE email=%s purpose=%s code=%s", email, purpose, code)
	}
	s.pruneEmailCodes(ctx)
	return nil
}

// ensureCodeCooldown enforces the per-address wait between code requests.
//
// Read from the table (not memory) so it survives a restart — otherwise
// killing the process would be a way to spam a stranger's inbox. It is checked
// before the SMTP configuration because a client hammering the endpoint should
// hear "slow down", and because that keeps the rule testable without a live
// mail server.
func (s *Service) ensureCodeCooldown(ctx context.Context, email, purpose string, cooldown time.Duration) error {
	var last models.EmailCode
	q := s.db.WithContext(ctx).Where("email = ? AND purpose = ?", email, purpose).
		Order("created_at DESC").Limit(1).Find(&last)
	if q.Error != nil || q.RowsAffected != 1 {
		return nil
	}
	elapsed := time.Since(last.CreatedAt)
	if elapsed >= cooldown {
		return nil
	}
	wait := int((cooldown - elapsed).Seconds())
	if wait < 1 {
		wait = 1
	}
	return newErr(http.StatusTooManyRequests, "please wait "+itoa(wait)+" seconds before requesting another code")
}

// requireSMTP refuses the flow when the operator has not configured a mail
// server. sendMailFn treats "no SMTP" as success (it is a no-op for the
// notification kinds), which here would mean telling the visitor a code is on
// its way when nothing was sent.
func (s *Service) requireSMTP(ctx context.Context) error {
	cfg, err := s.GetSiteConfig(ctx, false)
	if err != nil {
		return newErr(http.StatusInternalServerError, "failed to load site config")
	}
	if strings.TrimSpace(cfg.SmtpHost) == "" {
		return newErr(http.StatusServiceUnavailable, "SMTP is not configured; the site owner must set it up first")
	}
	return nil
}

// verifyEmailCode consumes a code. A nil error means "this code was correct and
// is now spent".
//
// A correct code spends no guess from the budget (E-10). The old sequence
// charged an attempt before comparing hashes, so two concurrent *correct*
// submissions could burn two of the five slots — and a double-click could
// exhaust the budget and void the code it had just accepted. The hash is
// compared first; only a mismatch spends a unit. The guess ceiling and the
// single-use guarantee still rest on conditional UPDATEs plus RowsAffected,
// which is what stopped R2-05's 5-guess limit from degrading to 19 under load
// and R4-02's double-spend of one intercepted mail.
func (s *Service) verifyEmailCode(ctx context.Context, email, purpose, code string) error {
	email = normalizeEmail(email)
	code = strings.TrimSpace(code)
	if code == "" {
		return newErr(http.StatusBadRequest, "verification code is required")
	}
	var row models.EmailCode
	err := s.db.WithContext(ctx).
		Where("email = ? AND purpose = ? AND used_at IS NULL", email, purpose).
		Order("created_at DESC").First(&row).Error
	if err != nil {
		return newErr(http.StatusBadRequest, "no valid code for this address; request a new one")
	}

	now := time.Now()
	if !row.ExpiresAt.After(now) {
		return newErr(http.StatusBadRequest, "the code has expired; request a new one")
	}
	if row.Attempts >= EmailCodeMaxAttempts {
		// Burn it: the row stays unspent but unreachable, so the visitor must
		// ask for a new one and the counter cannot keep climbing.
		s.db.WithContext(ctx).Model(&models.EmailCode{}).Where("id = ?", row.ID).
			Update("used_at", now)
		return newErr(http.StatusBadRequest, "too many attempts; request a new code")
	}

	// Step 1 — compare. An attempt is NOT spent here: a correct code never
	// pays for a guess.
	if !hmac.Equal([]byte(s.hashCode(email, purpose, code)), []byte(row.CodeHash)) {
		return s.spendWrongGuess(ctx, row, now)
	}

	// Step 2 — consume. Two callers that both matched still race here, and
	// only one can move used_at off NULL; the loser is told the code is spent
	// instead of silently completing a second reset.
	return s.consumeEmailCode(ctx, row.ID, now)
}

// consumeEmailCode marks a matched code as spent. A zero RowsAffected means
// another request won the race, so the caller is told it is already used.
func (s *Service) consumeEmailCode(ctx context.Context, id int64, now time.Time) error {
	consume := s.db.WithContext(ctx).Model(&models.EmailCode{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now)
	if consume.Error != nil {
		return newErr(http.StatusInternalServerError, "failed to consume code")
	}
	if consume.RowsAffected == 0 {
		return newErr(http.StatusBadRequest, "this code has already been used; request a new one")
	}
	return nil
}

// spendWrongGuess charges one attempt for a mismatched code. The WHERE clause
// carries the whole guard — unspent, unexpired, still inside the budget — so a
// request that raced past the same check cannot spend a second attempt.
func (s *Service) spendWrongGuess(ctx context.Context, row models.EmailCode, now time.Time) error {
	spend := s.db.WithContext(ctx).Model(&models.EmailCode{}).
		Where("id = ? AND used_at IS NULL AND expires_at > ? AND attempts < ?",
			row.ID, now, EmailCodeMaxAttempts).
		UpdateColumn("attempts", gorm.Expr("attempts + 1"))
	if spend.Error != nil {
		return newErr(http.StatusInternalServerError, "failed to verify code")
	}
	if spend.RowsAffected == 0 {
		// Lost the race, or the row changed under us. Re-read once so the
		// message says which of the three guards fired instead of the generic
		// "no valid code" (R2-06: the burned and the missing case used to be
		// indistinguishable, which made a working resend look broken).
		var live models.EmailCode
		if e := s.db.WithContext(ctx).Where("id = ?", row.ID).First(&live).Error; e == nil {
			if live.UsedAt != nil {
				return newErr(http.StatusBadRequest, "this code has already been used; request a new one")
			}
			if live.Attempts >= EmailCodeMaxAttempts {
				s.db.WithContext(ctx).Model(&models.EmailCode{}).Where("id = ?", row.ID).
					Update("used_at", now)
				return newErr(http.StatusBadRequest, "too many attempts; request a new code")
			}
			return newErr(http.StatusBadRequest, "the code has expired; request a new one")
		}
		return newErr(http.StatusBadRequest, "no valid code for this address; request a new one")
	}
	return newErr(http.StatusBadRequest, "incorrect verification code")
}

// pruneEmailCodes drops rows past retention. Called opportunistically on issue
// (the flow already hit the database), so there is no background goroutine to
// schedule or to leak.
func (s *Service) pruneEmailCodes(ctx context.Context) {
	cutoff := time.Now().Add(-emailCodeRetention)
	s.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&models.EmailCode{})
}
