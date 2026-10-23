package handlers

import (
	"fmt"
	"net/http"
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

// parseID reads an integer path parameter (e.g. {id}, {revID}). A missing or
// non-numeric value is a client error and returns a non-nil error, so the
// caller can answer 400 rather than letting a zero value leak into the service
// and turn a malformed id into a "not found".
func parseID(r *http.Request, name, label string) (int64, error) {
	n, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", label)
	}
	return n, nil
}

// parseInts parses several optional integer query parameters in one call,
// returning their values in the same order. Any explicitly supplied but
// non-numeric value is a client error (400), so a handler rejects all of them
// with a single check instead of repeating the same three lines per key.
func parseInts(q url.Values, label string, keys ...string) ([]int, error) {
	out := make([]int, len(keys))
	for i, key := range keys {
		v := q.Get(key)
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("%s must be integers", label)
		}
		out[i] = n
	}
	return out, nil
}
