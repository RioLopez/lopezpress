package handlers

import (
	"net/url"
	"testing"
)

func TestParseLimit(t *testing.T) {
	cases := []struct {
		name string
		raw  string // value of ?limit=
		def  int
		max  int
		want int
	}{
		{"missing falls back to def", "", 50, 200, 50},
		{"non-numeric falls back to def", "abc", 50, 200, 50},
		{"zero falls back to def", "0", 50, 200, 50},
		{"negative falls back to def", "-5", 50, 200, 50},
		{"over-max is capped", "999999", 50, 200, 200},
		{"exactly max allowed", "200", 50, 200, 200},
		{"within range honors value", "75", 50, 200, 75},
		{"self-login max 100", "500", 20, 100, 100},
		{"self-login within range", "10", 20, 100, 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := url.Values{}
			if c.raw != "" {
				q.Set("limit", c.raw)
			}
			got := parseLimit(q, "limit", c.def, c.max)
			if got != c.want {
				t.Fatalf("parseLimit(%q, def=%d, max=%d) = %d, want %d", c.raw, c.def, c.max, got, c.want)
			}
		})
	}
}
