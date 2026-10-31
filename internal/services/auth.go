package services

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lopezpress/internal/models"
)

// --- auth ---

// Register creates a new user account and signs them straight in, returning
// the same AuthResult shape as Login. Registration must log the user in
// automatically, so the client can drop straight into the app
// without a second round trip to /api/auth/login. Returns 409 if the username
// or email is already taken.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	if in.Username == "" || in.Email == "" || in.Password == "" {
		return nil, newErr(http.StatusBadRequest, "username, email and password are required")
	}
	if err := ValidatePasswordForUser(in.Password, in.Username, in.Email); err != nil {
		return nil, err
	}
	if !isValidUsername(in.Username) {
		return nil, newErr(http.StatusBadRequest, "username must be 3-32 characters: letters, digits, _ or -, starting with a letter or digit")
	}
	if !isValidEmail(in.Email) {
		return nil, newErr(http.StatusBadRequest, "invalid email")
	}

	// Public registration is gated by the site-level allow_register flag
	// (default false for a single-user site). Admins create users via the
	// separate POST /api/users endpoint, which is NOT subject to this gate.
	if !s.allowRegister(ctx) {
		return nil, newErr(http.StatusForbidden, "registration is disabled")
	}

	// Case-insensitive: the unique index is byte-exact, so without LOWER()
	// "Admin" could be registered next to "admin".
	var existing models.User
	if s.db.WithContext(ctx).Where("LOWER(username) = LOWER(?) OR LOWER(email) = LOWER(?)", in.Username, in.Email).
		First(&existing).RowsAffected == 1 {
		return nil, newErr(http.StatusConflict, "username or email already in use")
	}

	// (experimental, off by default): prove the address before the
	// account exists. Ordered after the duplicate check so a rejected sign-up
	// does not burn the code the user paid an email for.
	if err := s.verifyRegisterCode(ctx, in); err != nil {
		return nil, err
	}

	hash, err := models.HashPassword(in.Password)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to hash password")
	}

	u := models.User{Username: in.Username, Email: in.Email, PasswordHash: hash}
	if err := s.db.WithContext(ctx).Create(&u).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to create user")
	}

	token, expires, err := s.SignToken(u.ID, u.Role, u.TokenVersion)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to issue token")
	}

	return &AuthResult{
		Token:   token,
		Expires: expires,
		User:    PublicUser(u),
	}, nil
}

// Login verifies credentials (by username OR email) and returns a signed JWT
// along with the public view of the user. Every attempt (success or failure)
// is recorded in login_log, and a successful login updates the user's
// last_login_ip / last_login_at.
func (s *Service) Login(ctx context.Context, in LoginInput, meta LoginMeta) (*AuthResult, error) {
	// The identifier arrives in "email" but may be a username or an address.
	identifier := cmp.Or(in.Email, in.Username)
	if identifier == "" || in.Password == "" {
		return nil, newErr(http.StatusBadRequest, "email (or username) and password are required")
	}

	recordFail := func(userID, id string, code int, msg string) (*AuthResult, error) {
		s.RecordLogin(ctx, userID, id, meta.IP, meta.UA, false)
		return nil, newErr(code, msg)
	}

	var user models.User
	// Match by username OR email so the client can send either in the same field.
	if err := s.db.WithContext(ctx).
		Where("username = ? OR email = ?", identifier, identifier).
		First(&user).Error; err != nil {
		// Burn a real bcrypt comparison before answering, or the clock becomes a
		// username oracle: "no such user" returns in a microsecond while "wrong
		// password" spends ~60 ms hashing. The 401 body is identical either way.
		models.VerifyPasswordAgainstNothing(in.Password)
		return recordFail("", identifier, http.StatusUnauthorized, "invalid credentials")
	}
	if !models.VerifyPassword(in.Password, user.PasswordHash) {
		// Security notification: notify the account owner of a failed attempt
		// (throttled to 15 min / user so brute force can't spam the inbox).
		s.SendMail(ctx, user, MailLoginFailure, map[string]string{
			"ip": meta.IP, "time": time.Now().Format("2006-01-02 15:04 UTC"), "ua": meta.UA,
		})
		return recordFail(user.ID, identifier, http.StatusUnauthorized, "invalid credentials")
	}
	// Disabled is checked *after* the password on purpose: checking it first
	// handed 403 "account is disabled" to anyone who typed a suspended account's
	// address — an existence check needing no password and no lockout budget.
	if user.Disabled {
		return recordFail(user.ID, identifier, http.StatusForbidden, "account is disabled")
	}

	// Success: record audit + update last-login fields (best-effort).
	// Detect a "new location" (IP or browser differs from the previous login)
	// and notify the owner, then persist the new last-login state.
	s.notifyNewLocation(ctx, user, meta)
	s.RecordLogin(ctx, user.ID, identifier, meta.IP, meta.UA, true)
	_ = s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"last_login_ip": meta.IP, "last_login_at": time.Now(),
	}).Error

	token, expires, err := s.SignToken(user.ID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to issue token")
	}

	return &AuthResult{
		Token:   token,
		Expires: expires,
		User:    PublicUser(user),
	}, nil
}

// SignToken issues a signed HS256 JWT for the given user, using cfg.JWTSecret
// (with a documented dev fallback) and cfg.JWTExpiryHours (default 24h). The
// supplied tokenVersion is embedded as the "ver" claim so that bumping a
// user's TokenVersion (e.g. on "log out all devices") invalidates the token.
func (s *Service) SignToken(userID, role string, tokenVersion int64) (string, time.Time, error) {
	secret := s.signingSecret()
	if secret == "" {
		// Fail closed: signing with an empty or published key would mint a token
		// any attacker can reproduce. Startup guarantees a secret exists, so this
		// is a programming error, not a user-recoverable one.
		return "", time.Time{}, newErr(http.StatusInternalServerError, "jwt signing secret is not configured")
	}
	expiryHours := 24
	if s.cfg != nil && s.cfg.JWTExpiryHours > 0 {
		expiryHours = s.cfg.JWTExpiryHours
	}
	expires := time.Now().Add(time.Duration(expiryHours) * time.Hour)
	claims := jwt.MapClaims{
		"sub":  userID,
		"role": role,
		"ver":  tokenVersion,
		"exp":  expires.Unix(),
		"iat":  time.Now().Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString([]byte(secret))
	return token, expires, err
}

// signingSecret returns the JWT signing secret, or "" when none is configured.
//
// It must NOT fall back to a constant: a published default is a signing key
// anyone can read from the source, which turns "forgot to configure" into
// "every token is forgeable". config.EnsureJWTSecret guarantees a non-empty
// value at startup (dev generates and caches one, prod refuses to boot), so an
// empty result here means a service built by hand with a nil config, or a test
// that forgot to set it — and both callers fail closed on the empty string.
func (s *Service) signingSecret() string {
	if s.cfg != nil {
		return s.cfg.JWTSecret
	}
	return ""
}

// Authenticate parses and validates a raw JWT (from an Authorization header
// value, with or without the "Bearer " prefix) and loads the matching user.
// Used by the WithAuth handler to resolve the caller.
func (s *Service) Authenticate(ctx context.Context, rawToken string) (models.User, error) {
	claims, err := s.parseToken(rawToken, s.signingSecret())
	if err != nil {
		return models.User{}, newErr(http.StatusUnauthorized, "invalid or expired token")
	}
	var user models.User
	if err := s.db.WithContext(ctx).Where("id = ?", claims["sub"]).First(&user).Error; err != nil {
		return models.User{}, newErr(http.StatusUnauthorized, "user not found")
	}
	// Token-version check: bumping a user's TokenVersion (e.g. on "log out all
	// devices") revokes every previously issued token. New tokens always carry
	// a "ver" claim; legacy tokens (no claim) are only accepted while the
	// stored version is still 0, so the first deploy is backward compatible.
	if raw, ok := claims["ver"]; ok {
		var ver int64
		switch v := raw.(type) {
		case float64:
			ver = int64(v)
		case int64:
			ver = v
		}
		if ver != user.TokenVersion {
			return models.User{}, newErr(http.StatusUnauthorized, "token revoked")
		}
	} else if user.TokenVersion != 0 {
		return models.User{}, newErr(http.StatusUnauthorized, "token revoked")
	}
	// Account-state gate: a disabled account loses API access immediately,
	// not only at the next login attempt. Login already refuses disabled
	// accounts, but a stateless JWT remains valid until expiry (up to 24h) —
	// without this gate, disabling a compromised or abusive account would
	// leave every session it already holds fully usable (including writes)
	// until the token happens to expire. This is the Authenticate-level
	// counterpart of the must_change_password gate in the handlers.
	if user.Disabled {
		return models.User{}, newErr(http.StatusForbidden, "account is disabled")
	}
	return user, nil
}

// parseToken parses and validates a JWT. Accepts both "Bearer <token>" and
// bare token forms.
func (s *Service) parseToken(header, secret string) (jwt.MapClaims, error) {
	if secret == "" {
		// No configured secret means no token is valid — the opposite of
		// accepting one signed with a published default.
		return nil, fmt.Errorf("jwt secret not configured")
	}
	raw := header
	if len(header) > 7 && header[:7] == "Bearer " {
		raw = header[7:]
	}
	t, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != "HS256" {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !t.Valid {
		// jwt.Parse can report an invalid signature with a nil error
		// (only t.Valid is set), so surface a concrete error to keep the
		// 401 mapping in Authenticate intact.
		if err == nil {
			err = fmt.Errorf("invalid token signature")
		}
		return nil, err
	}
	claims, _ := t.Claims.(jwt.MapClaims)
	return claims, nil
}

// ResolveLoginIdentity returns the account behind a login identifier, without
// verifying any credential.
//
// It exists because the login lockout has to be keyed by ACCOUNT rather than by
// the string a client typed. Login matches `username = ? OR email = ?`, so a
// lockout keyed on the typed string is a per-spelling speed bump: lock "admin"
// with five wrong guesses and then submit "admin@example.com" with the right
// password, and the counter starts from zero. Alternating the two spellings
// also doubled the number of attempts available before anything locked.
//
// Nothing about the answer is secret — the caller is the login limiter, which
// runs after the per-IP rate limit and before the password check.
func (s *Service) ResolveLoginIdentity(ctx context.Context, identifier string) (userID, role string, ok bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", "", false
	}
	var row struct {
		ID   string
		Role string
	}
	// Scan into a struct rather than First(): "no such user" is an ordinary
	// answer here, not an error, and First would log it as one.
	if err := s.db.WithContext(ctx).Model(&models.User{}).
		Select("id, role").
		Where("username = ? OR email = ?", identifier, identifier).
		Scan(&row).Error; err != nil {
		return "", "", false
	}
	if row.ID == "" {
		return "", "", false
	}
	return row.ID, row.Role, true
}
