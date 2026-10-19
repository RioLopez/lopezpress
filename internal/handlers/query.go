package handlers

import (
	"net/url"
	"strconv"
)

// parseLimit reads an optional "?key=" integer query parameter and clamps it to
// the inclusive range [1, max]. A missing, non-numeric, non-positive, or
// over-max value falls back to def.
//
// Centralizing this keeps every list endpoint defense-in-depth bounded at the
// handler layer, independent of whatever the service layer enforces. Before it
// existed, the three list endpoints each parsed ?limit= with a different
// convention — one (admin logins) had no lower bound and two (actions, admin
// logins) had no upper bound, relying entirely on the service to clamp.
func parseLimit(q url.Values, key string, def, max int) int {
	v := q.Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
