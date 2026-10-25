package services

import (
	"encoding/json"
	"net/http"
	"strings"
)

// validateFriendLinks checks the footer link list.
//
// R6-09 added the two missing halves of this check. The name had no length
// limit at all, so a 5000-character one inflated the PUBLIC site-config
// response (and the footer that renders it) by kilobytes per reader. And the
// url half existed but the type half did not: a client that sent the array
// instead of the serialised string was read by cfgStr as "" and stored as
// such, silently wiping the operator's links. The wrong TYPE is now an error
// rather than a silent empty value, because "the links vanished" is
// indistinguishable from "there were never any".
//
// Extracted from validateSiteConfig so each function stays readable: the
// config has twenty fields and the link list is the only one with a
// sub-structure of its own.
func validateFriendLinks(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var links []map[string]string
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return newErr(http.StatusBadRequest, "friend links must be a JSON array")
	}
	if len(links) > maxFriendLinks {
		return newErr(http.StatusBadRequest, "too many friend links (max "+itoa(maxFriendLinks)+")")
	}
	for _, l := range links {
		u := strings.TrimSpace(l["url"])
		low := strings.ToLower(u)
		if !strings.HasPrefix(low, "http://") && !strings.HasPrefix(low, "https://") {
			return newErr(http.StatusBadRequest, "friend link url must start with http:// or https://")
		}
		if len([]rune(u)) > maxFriendLinkURLLen {
			return newErr(http.StatusBadRequest, "friend link url too long")
		}
		if n := strings.TrimSpace(l["name"]); len([]rune(n)) > maxFriendLinkNameLen {
			return newErr(http.StatusBadRequest,
				"friend link name too long (max "+itoa(maxFriendLinkNameLen)+" characters)")
		}
	}
	return nil
}
