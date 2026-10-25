package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// rootRequest builds a request carrying a root identity on its context, which
// is what WithAuth would have done. The site-config handlers read the actor and
// the service through package level, so the test wires both via setup().
func rootRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	root := models.User{ID: "root-id", Username: "root", Role: models.RoleRoot}
	return req.WithContext(services.WithActor(req.Context(), root, "203.0.113.1"))
}

// R6-08: PUT /api/admin/site-config is a full-entity save, and three of its
// fields have a zero value that is itself a MEANINGFUL SETTING. A client that
// sent only {"site_name":...} used to switch anonymous reading off (the whole
// site went dark for logged-out visitors), switch the daily backup off, and
// reset the retention count — all with a 200 and no warning.
func TestPartialSiteConfigSaveKeepsSwitches(t *testing.T) {
	svc, db := setup(t)

	// Start from a locked-down-but-on configuration.
	cur, err := svc.GetSiteConfig(context.Background(), true)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	upd := cur
	upd.PublicRead = true
	upd.AutoBackup = true
	upd.BackupKeep = 3
	if _, err := svc.UpdateSiteConfig(context.Background(), upd); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	// A partial save that carries neither switch nor retention.
	req := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Echo","site_title":"Echo","img_host_mode":"local"}`)
	rec := httptest.NewRecorder()
	UpdateSiteConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("partial save should succeed, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var after models.SiteConfig
	if err := db.Where("id = ?", 1).First(&after).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !after.PublicRead {
		t.Error("public_read must not be switched off by a payload that omitted it")
	}
	if !after.AutoBackup {
		t.Error("auto_backup must not be switched off by a payload that omitted it")
	}
	if after.BackupKeep != 3 {
		t.Errorf("backup_keep = %d, want the stored 3 (omitted must not reset it)", after.BackupKeep)
	}
}

// R5-04: the admin site-config read used to return smtp_pass verbatim to root.
// That put the mail password — which operators routinely reuse as their login
// password — in every response body, in browser memory, and in any log that
// captured it. It is now never returned, for any role.
func TestAdminSiteConfigNeverEchoesSecrets(t *testing.T) {
	svc, _ := setup(t)

	cur, err := svc.GetSiteConfig(context.Background(), true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cur.SmtpHost = "smtp.example.com"
	cur.SmtpUser = "mailer"
	cur.SmtpPass = "super-secret-pass"
	cur.ImgHostToken = "smms-token-value"
	if _, err := svc.UpdateSiteConfig(context.Background(), cur); err != nil {
		t.Fatalf("seed: %v", err)
	}

	req := rootRequest(t, http.MethodGet, "/api/admin/site-config", "")
	rec := httptest.NewRecorder()
	GetAdminSiteConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "super-secret-pass") {
		t.Error("smtp_pass must never be returned, not even to root")
	}
	if strings.Contains(body, "smms-token-value") {
		t.Error("img_host_token must never be returned")
	}

	// The non-secret fields still come through, so the settings form works.
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["smtp_host"] != "smtp.example.com" {
		t.Errorf("smtp_host should still be readable, got %v", payload["smtp_host"])
	}
}

// R5-04 (write half): because the read endpoint no longer returns the secret,
// the settings form round-trips it EMPTY. An empty value must therefore mean
// "unchanged" — otherwise every root save would silently wipe the mail
// credentials and the site would stop sending mail with no visible cause.
func TestRootSaveWithEmptySecretKeepsStoredOne(t *testing.T) {
	svc, db := setup(t)

	cur, err := svc.GetSiteConfig(context.Background(), true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cur.SmtpHost = "smtp.example.com"
	cur.SmtpUser = "mailer"
	cur.SmtpPass = "keep-me-please"
	if _, err := svc.UpdateSiteConfig(context.Background(), cur); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// The form saves everything it knows, which now includes an empty password.
	req := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Echo","site_title":"Echo","img_host_mode":"local",
		  "smtp_host":"smtp.example.com","smtp_user":"mailer","smtp_pass":""}`)
	rec := httptest.NewRecorder()
	UpdateSiteConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save should succeed, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var after models.SiteConfig
	if err := db.Where("id = ?", 1).First(&after).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.SmtpPass != "keep-me-please" {
		t.Errorf("an empty smtp_pass must leave the stored one alone, got %q", after.SmtpPass)
	}

	// A non-empty value does replace it.
	req2 := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Echo","site_title":"Echo","img_host_mode":"local",
		  "smtp_host":"smtp.example.com","smtp_user":"mailer","smtp_pass":"rotated"}`)
	rec2 := httptest.NewRecorder()
	UpdateSiteConfig(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("save should succeed, got %d", rec2.Code)
	}
	if err := db.Where("id = ?", 1).First(&after).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.SmtpPass != "rotated" {
		t.Errorf("a non-empty smtp_pass must replace the stored one, got %q", after.SmtpPass)
	}
}

// R6-09: a client that sent friend_links as an ARRAY (rather than the string
// this endpoint expects) had its links silently replaced by an empty value.
func TestFriendLinksWrongTypeIsRejected(t *testing.T) {
	_, _ = setup(t)

	req := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Echo","site_title":"Echo","img_host_mode":"local",
		  "friend_links":[{"name":"x","url":"https://example.com/"}]}`)
	rec := httptest.NewRecorder()
	UpdateSiteConfig(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an array friend_links must be refused with 400, got %d (body=%s)",
			rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "friend_links") {
		t.Errorf("the error should name the offending field, got %s", rec.Body.String())
	}
}
