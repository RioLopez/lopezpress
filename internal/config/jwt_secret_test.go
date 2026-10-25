package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A hard-coded default signing key is not a weak default, it is a published
// one: APP_ENV defaults to "dev", ValidateSecurity only WARNS there, so a
// deployment that forgot JWT_SECRET ran with a key printed in the source tree —
// anyone who could read the repository could mint a token for any account,
// root included.
func TestUnsetJWTSecretIsGeneratedNotDefaulted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", "")

	c := &Config{DBPath: filepath.Join(dir, "app.db")}
	if err := c.EnsureJWTSecret(); err != nil {
		t.Fatalf("dev mode must generate a secret, got error: %v", err)
	}
	if c.JWTSecret == "" {
		t.Fatal("secret is still empty")
	}
	if strings.Contains(c.JWTSecret, "dev-secret") {
		t.Fatal("fell back to the hard-coded default")
	}
	if len(c.JWTSecret) < 32 {
		t.Fatalf("generated secret is too short: %d", len(c.JWTSecret))
	}

	// Two processes must not share a secret by accident... except in dev, where
	// it is deliberately cached so a `go run` loop does not log you out.
	c2 := &Config{DBPath: filepath.Join(dir, "app.db")}
	if err := c2.EnsureJWTSecret(); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if c2.JWTSecret != c.JWTSecret {
		t.Fatal("dev cache did not reuse the generated secret")
	}

	// A DIFFERENT database directory must get a different secret.
	c3 := &Config{DBPath: filepath.Join(t.TempDir(), "app.db")}
	if err := c3.EnsureJWTSecret(); err != nil {
		t.Fatalf("third: %v", err)
	}
	if c3.JWTSecret == c.JWTSecret {
		t.Fatal("two unrelated databases received the same generated secret")
	}

	// The cache file must exist.
	info, err := os.Stat(filepath.Join(dir, ".dev-jwt-secret"))
	if err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	// Windows maps no useful subset of the permission bits (only read-only), so
	// asserting 0600 there would fail for a reason that has nothing to do with
	// this code. The Unix behaviour is the one that ships.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache file is too permissive: %v", info.Mode().Perm())
	}
}

// In strict mode an ephemeral key would log every user out on restart, which is
// an outage. Refuse to start instead of limping.
func TestProdRefusesToGenerateEphemeralSecret(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("JWT_SECRET", "")
	c := &Config{DBPath: filepath.Join(t.TempDir(), "app.db")}
	if err := c.EnsureJWTSecret(); err == nil {
		t.Fatal("production must refuse to run without JWT_SECRET")
	}
	// And it must not leave a cache file behind in prod.
	if _, err := os.Stat(filepath.Join(filepath.Dir(c.DBPath), ".dev-jwt-secret")); err == nil {
		t.Fatal("production must not write a cached secret")
	}
}

// An explicitly configured secret is used verbatim and never cached.
func TestConfiguredSecretWins(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", strings.Repeat("k", 48))
	c := &Config{DBPath: filepath.Join(t.TempDir(), "app.db"), JWTSecret: strings.Repeat("k", 48)}
	if err := c.EnsureJWTSecret(); err != nil {
		t.Fatalf("configured secret: %v", err)
	}
	if c.JWTSecret != strings.Repeat("k", 48) {
		t.Fatal("configured secret was replaced")
	}
}
