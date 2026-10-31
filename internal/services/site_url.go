package services

import (
	"net/http"
	"net/url"
	"strings"
)

// Site URL handling, kept apart from site_config.go so that file stays under
// the 350-line architecture ceiling and validateSiteConfig under 60 lines.
// The rules are short but the reasons behind them are not, and inlining them
// pushed both over the limit.

// validateSiteURL checks that a configured site_url is a bare origin:
// scheme + host + optional port, and nothing else.
//
// This is not pedantry. The field is only ever used as a prefix for absolute
// links in /feed and /sitemap.xml, so any trailing path is silently prepended
// to every permalink. Production ran with "http://<ip>:8088/admin" for a while,
// which turned every article URL in both files into "…/admin/p/<slug>" — a 404
// — and made the sitemap a list of dead links that also told crawlers where the
// admin lives. The previous check only looked at the scheme prefix, so a path
// sailed through and nothing surfaced the damage.
//
// Rejecting beats normalising: quietly stripping "/admin" would leave the
// operator believing the stored value is what they typed. The error names what
// to remove.
func validateSiteURL(raw string) error {
	u := strings.TrimSpace(raw)
	if u == "" {
		return nil
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		return newErr(http.StatusBadRequest, "site url must start with http:// or https://")
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return newErr(http.StatusBadRequest, "site url is not a valid URL, e.g. https://example.com")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return newErr(http.StatusBadRequest,
			"site url must not carry a query string or fragment; use only scheme://host[:port], e.g. https://example.com")
	}
	if p := strings.TrimRight(parsed.Path, "/"); p != "" {
		return newErr(http.StatusBadRequest,
			"site url must not contain a path (\""+p+"\"); use only scheme://host[:port], e.g. https://example.com — article links in the feed and sitemap are appended to this value, so a path makes every link wrong")
	}
	return nil
}

// canonicalSiteURL reduces a stored site_url to scheme://host[:port].
//
// validateSiteURL already rejects anything richer on the way in, so this is
// purely defensive: a value saved by an older build (the /admin incident) sits
// in the database and keeps poisoning /feed and /sitemap.xml until someone
// re-saves the settings form. Stripping the path on read means the fix does not
// depend on the operator remembering to touch that field.
//
// An unparseable value returns "" — the caller then falls back to the request
// Host, which is always a usable origin.
func canonicalSiteURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	parsed, err := url.Parse(u)
	if err != nil || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}
