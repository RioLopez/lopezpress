package config

import (
	"strings"
	"testing"
)

// TestRandomPassword verifies the generated password shape: 22 base64url
// characters (16 bytes of entropy), no padding, no URL-unsafe chars.
func TestRandomPassword(t *testing.T) {
	for i := 0; i < 50; i++ {
		p := randomPassword()
		if len(p) != 22 {
			t.Fatalf("randomPassword() = %q (len %d), want len 22", p, len(p))
		}
		for _, ch := range p {
			switch {
			case ch >= 'A' && ch <= 'Z':
			case ch >= 'a' && ch <= 'z':
			case ch >= '0' && ch <= '9':
			case ch == '-' || ch == '_': // base64url
			default:
				t.Fatalf("randomPassword() %q contains unexpected char %q", p, ch)
			}
		}
		if strings.ContainsAny(p, "+/") {
			t.Fatalf("randomPassword() %q uses non-base64url chars", p)
		}
	}
}

// TestValidateSecurity covers the JWT_SECRET validation rules:
//   - dev (default) only warns, never blocks.
//   - prod blocks on empty / known default / too-short / repeated-char secrets.
//   - prod accepts a strong secret.
func TestValidateSecurity(t *testing.T) {
	cases := []struct {
		name    string
		appEnv  string
		secret  string
		wantErr bool
	}{
		{"dev empty", "dev", "", false},
		{"dev weak", "dev", "secret", false},
		{"dev strong", "dev", "a3f9c1b2e7d48590f2a6c8e1d3b57049", false},
		{"prod empty", "prod", "", true},
		{"prod default changeme", "prod", "changeme", true},
		{"prod known dev default", "prod", "dev-secret-change-me-in-prod", true},
		{"prod too short", "prod", "abc", true},
		{"prod repeated char", "prod", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"prod strong", "prod", "a3f9c1b2e7d48590f2a6c8e1d3b57049", false},
		{"prod boundary 32", "prod", "abc123abc123abc123abc123abc123ab", false},
		{"unknown env treated as prod", "staging", "short", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.appEnv)
			c := &Config{JWTSecret: tc.secret}
			err := c.ValidateSecurity()
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateSecurity() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
