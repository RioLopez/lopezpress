package services

import (
	"net/http"
	"strings"

	"lopezpress/internal/models"
)

// Appearance (B4): accent colour and home layout. Split out of site_config.go
// both to keep that file inside the size gate and because these two settings
// are applied entirely by the browser — the server only validates and stores
// them.

// applyAppearancePolicy merges the accent/layout pair from the submitted row.
func applyAppearancePolicy(dst *models.SiteConfig, in models.SiteConfig) {
	dst.ThemeColor = normalizeThemeColor(in.ThemeColor)
	dst.HomeLayout = normalizeHomeLayout(in.HomeLayout)
}

// validateAppearance rejects values the CSS layer could not interpret safely.
func validateAppearance(in models.SiteConfig) error {
	// ThemeColor is written into a CSS custom property, so it must be a literal
	// hex colour — never a var() expression or an arbitrary string that the
	// browser would then interpret as a new declaration.
	if in.ThemeColor != "" && !isHexColor(in.ThemeColor) {
		return newErr(http.StatusBadRequest, "theme_color must be #rrggbb")
	}
	switch in.HomeLayout {
	case "", "list", "cards":
	default:
		return newErr(http.StatusBadRequest, "home_layout must be \"list\" or \"cards\"")
	}
	return nil
}

// isHexColor accepts exactly the six-digit form the admin colour picker emits.
func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// normalizeThemeColor lowercases the saved value. An empty string is kept:
// it means "use the built-in palette", which lets an operator undo a custom
// colour without having to remember the original hex.
func normalizeThemeColor(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return "#" + strings.ToLower(strings.TrimPrefix(s, "#"))
}

// normalizeHomeLayout maps the absent value onto the current default.
func normalizeHomeLayout(s string) string {
	if s == "cards" {
		return "cards"
	}
	return "list"
}
