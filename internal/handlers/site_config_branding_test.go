package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lopezpress/internal/models"
)

// A partial PUT used to zero the branding/presentation fields: SiteConfigFromMap
// decodes an absent key as "", and UpdateSiteConfig copied every field from the
// decoded struct onto the stored row. So a client that sent only {"site_name":…}
// wiped logo, favicon, url, description, footer, icp and friend_links. Absent
// keys now inherit the stored value, like the boolean switches already did.
func TestPartialSiteConfigSaveKeepsBranding(t *testing.T) {
	svc, db := setup(t)
	ctx := context.Background()

	cur, err := svc.GetSiteConfig(ctx, true)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cur.SiteName = "Echo"
	cur.SiteTitle = "Echo"
	cur.SiteLogo = "/uploads/logo.png"
	cur.SiteFavicon = "/uploads/fav.ico"
	cur.SiteURL = "https://example.com"
	cur.SiteDescription = "a description"
	cur.FooterText = "a footer"
	cur.IcpNo = "icp-123"
	cur.FriendLinks = "[]"
	cur.ImgHostMode = "local"
	if _, err := svc.UpdateSiteConfig(ctx, cur); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Partial save: only the two name fields, no branding at all.
	req := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Renamed","site_title":"Renamed"}`)
	rec := httptest.NewRecorder()
	UpdateSiteConfig(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("partial save should succeed, got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var after models.SiteConfig
	if err := db.Where("id = ?", 1).First(&after).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if after.SiteLogo != "/uploads/logo.png" {
		t.Errorf("site_logo was zeroed by a partial save: %q", after.SiteLogo)
	}
	if after.SiteFavicon != "/uploads/fav.ico" {
		t.Errorf("site_favicon was zeroed by a partial save: %q", after.SiteFavicon)
	}
	if after.SiteURL != "https://example.com" {
		t.Errorf("site_url was zeroed by a partial save: %q", after.SiteURL)
	}
	if after.SiteDescription != "a description" {
		t.Errorf("site_description was zeroed by a partial save: %q", after.SiteDescription)
	}
	if after.FooterText != "a footer" {
		t.Errorf("footer_text was zeroed by a partial save: %q", after.FooterText)
	}
	if after.IcpNo != "icp-123" {
		t.Errorf("icp_no was zeroed by a partial save: %q", after.IcpNo)
	}
	if after.FriendLinks != "[]" {
		t.Errorf("friend_links was zeroed by a partial save: %q", after.FriendLinks)
	}
	if after.ImgHostMode != "local" {
		t.Errorf("img_host_mode was zeroed by a partial save: %q", after.ImgHostMode)
	}
}

// The branding slots render into <img src>/<link href>. A foreign scheme
// (javascript:, data:, …) must be rejected, while a local path is accepted.
func TestSiteConfigRejectsBadBrandingURL(t *testing.T) {
	_, _ = setup(t)

	req := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Echo","site_title":"Echo","img_host_mode":"local",
		  "site_logo":"javascript:alert(1)"}`)
	rec := httptest.NewRecorder()
	UpdateSiteConfig(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("javascript: logo must be refused with 400, got %d (body=%s)",
			rec.Code, rec.Body.String())
	}
	if !strings.Contains(strings.ToLower(rec.Body.String()), "site logo") {
		t.Errorf("the error should name the offending field, got %s", rec.Body.String())
	}

	// A local path is a legitimate value and must be accepted.
	req2 := rootRequest(t, http.MethodPut, "/api/admin/site-config",
		`{"site_name":"Echo","site_title":"Echo","img_host_mode":"local",
		  "site_logo":"/uploads/logo.png"}`)
	rec2 := httptest.NewRecorder()
	UpdateSiteConfig(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("local-path logo must be accepted, got %d (body=%s)",
			rec2.Code, rec2.Body.String())
	}
}
