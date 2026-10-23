package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"lopezpress/internal/models"
)

// --- auth ---

// Register creates a new user account and signs them straight in, returning
// the same AuthResult shape as Login. Spec 04 AC-2.2 requires registration to
// log the user in automatically, so the client can drop straight into the app
// without a second round trip to /api/auth/login. Returns 409 if the username
// or email is already taken.
func (s *Service) Register(ctx context.Context, in RegisterInput) (*AuthResult, error) {
	if in.Username == "" || in.Email == "" || in.Password == "" {
		return nil, newErr(http.StatusBadRequest, "username, email and password are required")
	}
	if len(in.Password) < 8 {
		return nil, newErr(http.StatusBadRequest, "password must be at least 8 characters")
	}

	// Public registration is gated by the site-level allow_register flag
	// (default false for a single-user site). Admins create users via the
	// separate POST /api/users endpoint, which is NOT subject to this gate.
	if !s.allowRegister(ctx) {
		return nil, newErr(http.StatusForbidden, "registration is disabled")
	}

	var existing models.User
	if s.db.WithContext(ctx).Where("username = ? OR email = ?", in.Username, in.Email).
		First(&existing).RowsAffected == 1 {
		return nil, newErr(http.StatusConflict, "username or email already in use")
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
		User:    toPublic(u),
	}, nil
}

// Login verifies credentials (by username OR email) and returns a signed JWT
// along with the public view of the user. Every attempt (success or failure)
// is recorded in login_log, and a successful login updates the user's
// last_login_ip / last_login_at (spec 14).
func (s *Service) Login(ctx context.Context, in LoginInput, meta LoginMeta) (*AuthResult, error) {
	// The identifier is normally sent in "email" (it may be either a username
	// or an email); a client may also send it explicitly in "username".
	identifier := in.Email
	if identifier == "" {
		identifier = in.Username
	}
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
		return recordFail("", identifier, http.StatusUnauthorized, "invalid credentials")
	}
	if user.Disabled {
		return recordFail(user.ID, identifier, http.StatusForbidden, "account is disabled")
	}
	if !models.VerifyPassword(in.Password, user.PasswordHash) {
		// Security notification: notify the account owner of a failed attempt
		// (throttled to 15 min / user so brute force can't spam the inbox).
		s.SendMail(ctx, user, MailLoginFailure, map[string]string{
			"ip": meta.IP, "time": time.Now().Format("2006-01-02 15:04 UTC"), "ua": meta.UA,
		})
		return recordFail(user.ID, identifier, http.StatusUnauthorized, "invalid credentials")
	}

	// Success: record audit + update last-login fields (best-effort).
	// Detect a "new location" (IP or browser differs from the previous login)
	// and notify the owner, then persist the new last-login state.
	s.notifyNewLocation(ctx, user, meta)
	s.RecordLogin(ctx, user.ID, identifier, meta.IP, meta.UA, true)
	_ = s.db.WithContext(ctx).Model(&user).Updates(map[string]any{
		"last_login_ip": meta.IP,
		"last_login_at": time.Now(),
	}).Error

	token, expires, err := s.SignToken(user.ID, user.Role, user.TokenVersion)
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to issue token")
	}

	return &AuthResult{
		Token:   token,
		Expires: expires,
		User:    toPublic(user),
	}, nil
}

// SignToken issues a signed HS256 JWT for the given user, using cfg.JWTSecret
// (with a documented dev fallback) and cfg.JWTExpiryHours (default 24h). The
// supplied tokenVersion is embedded as the "ver" claim so that bumping a
// user's TokenVersion (e.g. on "log out all devices") invalidates the token.
func (s *Service) SignToken(userID, role string, tokenVersion int64) (string, time.Time, error) {
	secret := s.signingSecret()
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

// signingSecret returns the JWT signing secret, falling back to the
// documented dev default when cfg is unset.
func (s *Service) signingSecret() string {
	if s.cfg != nil && s.cfg.JWTSecret != "" {
		return s.cfg.JWTSecret
	}
	return "dev-secret-change-me-in-prod"
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
	// (as-built audit Q6)
	if user.Disabled {
		return models.User{}, newErr(http.StatusForbidden, "account is disabled")
	}
	return user, nil
}

// parseToken parses and validates a JWT. Accepts both "Bearer <token>" and
// bare token forms.
func (s *Service) parseToken(header, secret string) (jwt.MapClaims, error) {
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
