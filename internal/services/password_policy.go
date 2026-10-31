package services

import (
	"net/http"
	"strings"
	"unicode/utf8"
)

// --- password policy --------------------------------------------------------
//
// Until this file existed the only rule anywhere was `len(password) < 8`, copy
// pasted into five call sites (Register, CreateUser, ResetPassword,
// ChangePassword, ResetPasswordByCode). Three separate problems came out of
// that:
//
//  1. "12345678" passed. So did "password", "qwertyui", "aaaaaaaa" — the eight
//     characters were there, the entropy was not. A single VPS login endpoint
//     fronted by a 5-attempt lockout is not protected by lockout alone: the
//     lockout slows online guessing, and the hash is what protects a stolen
//     database. Against an offline attack on bcrypt(cost 10), "12345678" falls
//     in the first thousand candidates of any wordlist.
//
//  2. There was no upper bound. bcrypt silently refuses inputs over 72 bytes
//     (ErrPasswordTooLong), so a pasted passphrase came back as a 500
//     "failed to hash password" — a server error caused by a legal user action.
//
//  3. Five copies of one rule is five chances to forget one. The Markdown
//     importer and the backup restore path never had the check at all; they
//     arrive here as hashes, which is fine, but the pattern is how a rule rots.
//
// The policy follows NIST SP 800-63B rather than a corporate "1 upper, 1 digit,
// 1 symbol" rule: length plus a blocklist, no composition requirements.
// Composition rules push people toward "Password1!" — which is on every
// wordlist — while a 20-character passphrase with no symbols is nowhere on it.

const (
	// MinPasswordRunes is the floor, counted in characters rather than bytes so
	// that an eight-character Chinese passphrase is not measured as 24.
	MinPasswordRunes = 8

	// MaxPasswordBytes is bcrypt's hard limit, not a taste call. Anything longer
	// is rejected by HashPassword with a 500, so it must be a 400 here first.
	MaxPasswordBytes = 72

	// maxAllDigitsLen is how long an all-digit password may be before we stop
	// calling it weak. A 12-digit PIN is already ~40 bits of real entropy in
	// practice (birthdays, phone numbers, 5201314) far less than the digits
	// suggest; beyond that length the space genuinely grows.
	maxAllDigitsLen = 12
)

// ValidatePassword is the single gate every password write must pass. It is
// called from Register, CreateUser, ResetPassword, ChangePassword and
// ResetPasswordByCode — and from anywhere a password is added later, because
// the rule lives here and not at the call sites.
func ValidatePassword(pw string) error {
	if strings.TrimSpace(pw) == "" {
		return newErr(http.StatusBadRequest, "password is required")
	}
	if utf8.RuneCountInString(pw) < MinPasswordRunes {
		return newErr(http.StatusBadRequest, "password must be at least 8 characters")
	}
	// Byte length, not rune length: bcrypt counts bytes, and 25 Chinese
	// characters are already 75 bytes.
	if len(pw) > MaxPasswordBytes {
		return newErr(http.StatusBadRequest, "password is too long (max 72 bytes; a Chinese character or emoji counts as 3-4)")
	}
	if reason := weakPasswordReason(pw); reason != "" {
		return newErr(http.StatusBadRequest, "password is too weak: "+reason)
	}
	return nil
}

// ValidatePasswordForUser adds the account-specific half of the policy: a
// password that contains the username or the address it protects is weak no
// matter how long it is. NIST 800-63B calls these "context-specific words" and
// asks for exactly this check.
//
// The username and email are optional; an empty value skips its check, so a
// caller that does not know the account (none today) can still use the generic
// rule.
func ValidatePasswordForUser(pw, username, email string) error {
	if err := ValidatePassword(pw); err != nil {
		return err
	}
	low := strings.ToLower(pw)
	if len(username) >= 3 && strings.Contains(low, strings.ToLower(username)) {
		return newErr(http.StatusBadRequest, "password must not contain your username")
	}
	if local := emailLocalPart(email); len(local) >= 3 && strings.Contains(low, local) {
		return newErr(http.StatusBadRequest, "password must not contain your email address")
	}
	return nil
}

// emailLocalPart returns the lower-cased part of an address before the "@".
func emailLocalPart(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 0 {
		return ""
	}
	return strings.ToLower(email[:at])
}

// weakPasswordReason names why a password is too predictable, or returns "" for
// an acceptable one. Returning the reason (rather than a bool) is what lets the
// message tell the user what to change instead of a bare "rejected".
func weakPasswordReason(pw string) string {
	if allSameRune(pw) {
		return "a single repeated character"
	}
	if isAllDigits(pw) && len(pw) <= maxAllDigitsLen {
		return "digits only"
	}

	skel := passwordSkeleton(pw)
	// An empty skeleton means the password was nothing but digits and trailing
	// padding ("12345678", "9876543210123", "00000000000000!") — there is no
	// word left to check, and there was never much entropy to begin with.
	if skel == "" {
		return "digits and symbols only"
	}
	if _, bad := commonPasswords[skel]; bad {
		return "it is one of the most commonly used passwords"
	}
	if len(pw) < 24 && strings.Contains(skel, "lopezpress") {
		return "it contains this site's name"
	}
	// Monotonic runs ("12345678", "abcdefgh", "qwerty") are the other half of
	// every wordlist and are cheap to recognise once the common table has been
	// stripped of its padding.
	if isMonotonicRun(skel) {
		return "a simple sequence"
	}
	return ""
}

// allSameRune reports whether every character is the same ("aaaaaaaa").
func allSameRune(s string) bool {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return false
	}
	for _, c := range s {
		if c != r {
			return false
		}
	}
	return true
}

// isAllDigits reports whether the whole string is ASCII digits.
func isAllDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// isMonotonicRun reports whether the skeleton is one ascending or descending
// character run: "12345678", "abcdefgh", "hgfedcba". Only ASCII letters and
// digits are considered, and the run must be at least six long, so ordinary
// words and the tail of a real passphrase are left alone.
func isMonotonicRun(skel string) bool {
	if len(skel) < 6 {
		return false
	}
	up, down := true, true
	for i := 1; i < len(skel); i++ {
		prev, cur := skel[i-1], skel[i]
		if cur != prev+1 {
			up = false
		}
		if cur != prev-1 {
			down = false
		}
		if !up && !down {
			return false
		}
	}
	return up || down
}

// passwordSkeleton folds away the decoration people add to a weak word so that
// one entry in the blocklist covers a hundred of its disguises.
//
// "p@ssw0rd", "Password", "password123" and "PASSWORD!!" all collapse to
// "password". The folding is deliberately lossy in the attacker's favour: a
// false positive costs the user one more try, a false negative costs the
// account.
//
// Steps, in order:
//  1. lower-case and trim;
//  2. drop trailing digits and symbols ("password123", "password!!", "123456")
//     — noteself-referential, but this is the single most common disguise;
//  3. map the usual leetspeak substitutions back to letters;
//  4. drop trailing digits again, since "p@ssw0rd1" only reveals its last digit
//     after step 3.
func passwordSkeleton(pw string) string {
	s := strings.ToLower(strings.TrimSpace(pw))
	s = strings.TrimRight(s, "0123456789!@#$%^&*._-~?")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '0':
			r = 'o'
		case '1', '!', '|':
			r = 'i'
		case '3':
			r = 'e'
		case '4', '@':
			r = 'a'
		case '5', '$':
			r = 's'
		case '7':
			r = 't'
		}
		b.WriteRune(r)
	}
	return strings.TrimRight(b.String(), "0123456789")
}

// commonPasswordList is the blocklist: the words that sit at the top of every
// published breach corpus, plus the patterns Chinese users reach for
// (languages, pinyin dates, "woaini", "5201314"). It is a raw string split at
// init so the list stays readable here instead of becoming a 200-line slice
// literal.
//
// Entries are written the way a person would type them and folded through
// passwordSkeleton at startup, so "p@ssw0rd" needs no entry of its own.
//
// Length is not a goal — coverage of the *shape* is. Anything past the top few
// hundred is handled by the length floor, the digits rule and the username
// check rather than by growing this string.
const commonPasswordList = `
123456 123456789 12345678 1234567890 1234567 12345 12345678910 111111 000000
666666 888888 123123 112233 121212 654321 987654321 5201314 1314520 520520
password password1 password12 password123 passw0rd p@ssword p@ssw0rd pa55word
qwerty qwerty123 qwertyui qwertyuiop 1qaz2wsx 1q2w3e4r 1q2w3e4r5t zaq12wsx qweasdzxc
asdfgh asdfghjkl zxcvbnm zxcvbnm123 abc123 abc123456 abcd1234 a123456 a12345678
admin admin123 admin888 administrator root toor guest test test123 test1234
letmein letmein123 welcome welcome123 iloveyou iloveyou1 monkey dragon sunshine
princess football baseball superman batman master shadow michael jennifer
trustno1 starwars whatever freedom qazwsx qazwsxedc 159357 753951 852456
woaini woaini1314 woshishui buzhidao wangba 123qwe qwe123
aaaaaaaa bbbbbbbb 11111111 22222222 88888888 abcdefgh abcdefg 88888888a 99999999
lopezpress lopezpress123 lopez lopez123 echolopez echo123 lopezpc
changeme default secret mypassword newpassword temp123 pass123 passw0rd1
aa123456 ab123456 abc12345 aini1314 7758521 147258369 741852963 963852741
`

// commonPasswords is the folded lookup built once from commonPasswordList.
var commonPasswords = func() map[string]struct{} {
	m := make(map[string]struct{}, 256)
	for _, w := range strings.Fields(commonPasswordList) {
		m[passwordSkeleton(w)] = struct{}{}
	}
	return m
}()
