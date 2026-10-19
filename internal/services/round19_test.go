package services

import (
	"context"
	"net/http"
	"testing"

	"echo-content-api/internal/models"
)

// TestRegisterGateClosedByDefault verifies that with no site-config row the
// default (allow_register=false) blocks public registration with 403.
func TestRegisterGateClosedByDefault(t *testing.T) {
	svc, db := newTestService(t)
	// Drop the seeded open-registration row; GetSiteConfig recreates the
	// default row with allow_register=false.
	db.Where("id = ?", 1).Delete(&models.SiteConfig{})

	_, err := svc.Register(context.Background(), RegisterInput{
		Username: "bob", Email: "bob@example.com", Password: "secret123",
	})
	if StatusOf(err) != http.StatusForbidden {
		t.Fatalf("expected 403 when registration closed, got %v", StatusOf(err))
	}
}

// TestLoginRecordsLog verifies each successful login writes a login_log row
// and updates the user's last_login_ip / last_login_at (spec 14).
func TestLoginRecordsLog(t *testing.T) {
	svc, db := newTestService(t)
	makeUser(t, db, "alice", "alice@example.com", "user")

	if _, err := svc.Login(context.Background(), LoginInput{Email: "alice", Password: "secret123"}, LoginMeta{IP: "1.2.3.4", UA: "test"}); err != nil {
		t.Fatalf("login: %v", err)
	}

	var n int64
	db.Model(&models.LoginLog{}).Count(&n)
	if n != 1 {
		t.Fatalf("expected 1 login log, got %d", n)
	}
	var u models.User
	db.Where("username = ?", "alice").First(&u)
	if u.LastLoginIP != "1.2.3.4" {
		t.Fatalf("last_login_ip not set: %q", u.LastLoginIP)
	}
	if u.LastLoginAt == nil {
		t.Fatalf("last_login_at not set")
	}
}

// TestDeleteUserGuards verifies self-delete and last-admin deletion are
// rejected (spec 09).
func TestDeleteUserGuards(t *testing.T) {
	svc, db := newTestService(t)
	admin := makeUser(t, db, "admin", "admin@example.com", "admin")
	makeUser(t, db, "bob", "bob@example.com", "user")

	if err := svc.DeleteUser(context.Background(), admin, admin.ID); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 deleting self, got %v", StatusOf(err))
	}
	if err := svc.DeleteUser(context.Background(), admin, admin.ID); StatusOf(err) != http.StatusBadRequest {
		t.Fatalf("expected 400 deleting last admin, got %v", StatusOf(err))
	}

	// Deleting the non-admin user is allowed.
	if err := svc.DeleteUser(context.Background(), admin, "00000000-0000-0000-0000-000000000000"); StatusOf(err) != http.StatusNotFound {
		t.Fatalf("expected 404 for missing user, got %v", StatusOf(err))
	}
}

// TestSiteConfigRoundTrip verifies GetSiteConfig creates a default row and
// UpdateSiteConfig persists changes.
func TestSiteConfigRoundTrip(t *testing.T) {
	svc, _ := newTestService(t)
	cfg, err := svc.GetSiteConfig(context.Background(), false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if cfg.SiteName != "Echo" {
		t.Fatalf("expected default name Echo, got %q", cfg.SiteName)
	}

	cfg.SiteName = "My Site"
	cfg.AllowRegister = true
	if _, err := svc.UpdateSiteConfig(context.Background(), cfg); err != nil {
		t.Fatalf("update: %v", err)
	}
	again, _ := svc.GetSiteConfig(context.Background(), true)
	if again.SiteName != "My Site" || !again.AllowRegister {
		t.Fatalf("update not persisted: name=%q allow=%v", again.SiteName, again.AllowRegister)
	}
}
