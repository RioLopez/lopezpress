package models

import "time"

// Email verification purposes. A code is only ever valid for the purpose it
// was issued for: a register code cannot reset a password, and a code mailed
// to a new address cannot be replayed to move the account again.
const (
	EmailPurposeRegister   = "register"
	EmailPurposeReset      = "reset_password"
	EmailPurposeChangeMail = "change_email"
)

// EmailCode is one pending (or spent) one-time code.
//
// The plaintext code is never stored. CodeHash is an HMAC over
// "email|purpose|code" keyed by the deployment's JWT secret, because a plain
// SHA-256 of a six-digit number is a 10^6 lookup table — the whole point of
// hashing would be gone.
//
// Rows are kept after use (UsedAt set) rather than deleted: the cooldown check
// reads "when did this address last ask for a code", and an audit trail of
// "somebody tried to change the email on this account" is worth more than the
// few bytes it costs. A janitor prunes rows older than a day.
type EmailCode struct {
	ID      int64  `gorm:"primaryKey" json:"id"`
	Email   string `gorm:"size:160;index:idx_email_codes_lookup,priority:1" json:"email"`
	Purpose string `gorm:"size:24;index:idx_email_codes_lookup,priority:2" json:"purpose"`
	// CodeHash is an HMAC-SHA256 hex digest; see the type comment.
	CodeHash string `gorm:"size:64" json:"-"`
	// UserID is the account the code was issued for, or "" for the two
	// anonymous flows (register / password reset), where the address is the
	// only identity we have.
	UserID string `gorm:"size:36;index" json:"user_id"`
	// Attempts counts wrong guesses. At EmailCodeMaxAttempts the code is
	// burned: brute-forcing six digits at ~1 guess/request is not viable, but
	// only because the counter exists.
	Attempts  int        `gorm:"default:0" json:"attempts"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	IP        string     `gorm:"size:64" json:"ip"`
	CreatedAt time.Time  `json:"created_at"`
}
