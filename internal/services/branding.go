package services

import (
	"net/http"
	"net/url"
	"strings"
)

// validateBrandingURL accepts an empty value (slot cleared), a local path
// (e.g. "/uploads/logo.png", the normal case), or an absolute http(s) URL. It
// rejects any other scheme (javascript:, data:, ftp: …) so a stored branding
// value can never be a script primitive. Local paths have no scheme, so they
// pass; only an explicit foreign scheme is blocked.
func validateBrandingURL(raw, label string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && u.Scheme != "http" && u.Scheme != "https" {
		return newErr(http.StatusBadRequest, label+" must be an absolute http(s) URL")
	}
	return nil
}
