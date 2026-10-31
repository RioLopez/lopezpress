package services

import (
	"strings"
	"testing"
)

// TestPasswordLengthFloor pins the two length rules and, importantly, the two
// units they use: characters for the floor (so a Chinese passphrase is not
// measured as three times its length) and bytes for the ceiling (because bcrypt
// counts bytes and refuses anything past 72).
func TestPasswordLengthFloor(t *testing.T) {
	cases := []struct {
		name string
		pw   string
		ok   bool
	}{
		{"seven ascii", "aG7!kLp", false},
		{"eight ascii", "aG7!kLpq", true},
		{"seven chinese", "密码密码密码密", false},
		{"eight chinese", "密码密码密码密码", true},
		{"24 chinese = 72 bytes", strings.Repeat("密码", 12), true},
		{"25 chinese = 75 bytes", strings.Repeat("密码", 12) + "密", false},
		{"73 ascii", "aG7!kLpq" + strings.Repeat("x", 65), false},
		{"72 ascii", "aG7!kLpq" + strings.Repeat("x", 64), true},
		{"empty", "", false},
		{"spaces only", "        ", false},
		{"tab and newline", "\t\n\t\n\t\n\t\n", false},
	}
	for _, c := range cases {
		err := ValidatePassword(c.pw)
		if c.ok && err != nil {
			t.Errorf("%s: expected accept, got %v", c.name, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: expected reject, got accept", c.name)
		}
	}
}

// TestWeakPasswordsRejected is the behaviour change that motivated the file:
// every one of these passed the old `len < 8` check.
func TestWeakPasswordsRejected(t *testing.T) {
	weak := []string{
		// raw corpus entries
		"12345678", "123456789", "password", "qwertyui", "abcdefgh",
		"11111111", "aaaaaaaa", "iloveyou", "letmein1", "woaini520",
		// decoration on a corpus word
		"Password", "password1", "password123", "p@ssw0rd", "P@ssw0rd!",
		"passw0rd1", "qwerty123", "admin123", "admin888", "lopezpress123",
		// sequences
		"1234567890", "abcdefgh", "hgfedcba", "13579246",
		// digits only
		"19900101", "13800138000", "001122334455",
		// site / product words
		"lopezpress", "LopezPress1", "echolopez1",
	}
	for _, pw := range weak {
		if err := ValidatePassword(pw); err == nil {
			t.Errorf("expected %q to be rejected, it was accepted", pw)
		}
	}
}

// TestStrongPasswordsAccepted guards against the policy becoming a nuisance: a
// passphrase with no symbols, a random-looking string, and a long blocklist
// near-miss must all pass. If a change to passwordSkeleton starts rejecting
// these, that is a false positive worth knowing about before a user hits it.
func TestStrongPasswordsAccepted(t *testing.T) {
	strong := []string{
		"correct horse battery",   // words, spaces, no symbols
		"vN7#kQp2Lm",              // random-ish
		"green river valley 2019", // word passphrase + year
		"my-bike-is-blue",         // punctuation
		"Tr0ub4dor&3x",            // classic xkcd, not on the list
		"northern-lights-2026",    // near-miss for the site-name rule
		"密码是绿色山谷的意思",              // Chinese passphrase
		"9f3b2c1d8e7a6f5b4c3d",    // hex blob
	}
	for _, pw := range strong {
		if err := ValidatePassword(pw); err != nil {
			t.Errorf("%q should be accepted, got %v", pw, err)
		}
	}
}

// TestPasswordContextChecks covers the account-specific half: a password that
// contains the username or the address it protects.
func TestPasswordContextChecks(t *testing.T) {
	if err := ValidatePasswordForUser("alice-in-wonderland", "alice", "alice@example.com"); err == nil {
		t.Error("password containing the username should be rejected")
	}
	if err := ValidatePasswordForUser("Alice-in-Wonderland", "alice", "alice@example.com"); err == nil {
		t.Error("username match must be case-insensitive")
	}
	if err := ValidatePasswordForUser("bobsled-team-2026", "alice", "bob@example.com"); err == nil {
		t.Error("password containing the email local part should be rejected")
	}
	if err := ValidatePasswordForUser("example-com-2026", "alice", "bob@example.com"); err != nil {
		t.Errorf("the address domain must not be treated as a local part, got %v", err)
	}
	if err := ValidatePasswordForUser("quiet-harbour-73", "bob", "bob@example.com"); err != nil {
		t.Errorf("unrelated password should pass, got %v", err)
	}
	// A two-character username is too short to be a meaningful substring: any
	// password containing "bo" would be rejected for no security gain.
	if err := ValidatePasswordForUser("rainbow-dolphin", "bo", ""); err != nil {
		t.Errorf("short username must not trigger the substring rule, got %v", err)
	}
	// The generic rule still runs first.
	if err := ValidatePasswordForUser("12345678", "bob", "bob@example.com"); err == nil {
		t.Error("ValidatePasswordForUser must still enforce the general policy")
	}
}

// TestPasswordSkeletonIsLossyInTheRightDirection documents the folding that
// makes one blocklist entry cover its disguises. These are not style checks:
// each pair is "two strings a user considers different passwords", and the
// blocklist is only effective if the fold maps them together.
func TestPasswordSkeletonIsLossyInTheRightDirection(t *testing.T) {
	groups := [][]string{
		{"password", "Password", "password1", "password123", "p@ssw0rd", "P@ssw0rd!"},
		{"admin", "admin123", "admin888", "Admin2024"},
		{"qwerty", "qwerty123", "QWERTY"},
	}
	for _, g := range groups {
		want := passwordSkeleton(g[0])
		for _, pw := range g[1:] {
			if got := passwordSkeleton(pw); got != want {
				t.Errorf("skeleton(%q) = %q, want %q (same group as %q)", pw, got, want, g[0])
			}
		}
	}
	// And the fold must not be so aggressive that unrelated passwords collide.
	if passwordSkeleton("correct horse") == passwordSkeleton("password") {
		t.Error("skeleton collapsed two unrelated passwords")
	}
}

// TestPasswordPolicyErrorMessages checks the responses a user actually reads.
// A bare "rejected" tells them nothing to change; every branch must say what
// was wrong.
func TestPasswordPolicyErrorMessages(t *testing.T) {
	cases := map[string]string{
		"aG7!kLp":    "at least 8 characters",
		"12345678":   "too weak",
		"password":   "too weak",
		"aaaaaaaa":   "too weak",
		"lopezpress": "too weak",
	}
	for pw, want := range cases {
		err := ValidatePassword(pw)
		if err == nil {
			t.Errorf("%q: expected an error", pw)
			continue
		}
		se := AsServiceError(err)
		if se == nil {
			t.Errorf("%q: error is not a ServiceError: %T", pw, err)
			continue
		}
		if se.Status != 400 {
			t.Errorf("%q: status = %d, want 400", pw, se.Status)
		}
		if !strings.Contains(se.Msg, want) {
			t.Errorf("%q: message %q does not mention %q", pw, se.Msg, want)
		}
	}
}
