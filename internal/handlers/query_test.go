package handlers

import (
	"net/http"
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

func TestParseInts(t *testing.T) {
	cases := []struct {
		name    string
		raw     map[string]string // values to set (absent key = not set)
		want    []int
		wantErr bool
	}{
		{"all absent returns zeros", nil, []int{0, 0, 0}, false},
		{"numeric honors values", map[string]string{"page": "3", "size": "10", "category": "7"}, []int{3, 10, 7}, false},
		{"partial values fill in order", map[string]string{"size": "10"}, []int{0, 10, 0}, false},
		{"non-numeric errors", map[string]string{"page": "abc"}, nil, true},
		{"float errors", map[string]string{"size": "1.5"}, nil, true},
		{"negative is a value", map[string]string{"page": "-1"}, []int{-1, 0, 0}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := url.Values{}
			for k, v := range c.raw {
				q.Set(k, v)
			}
			got, err := parseInts(q, "page, size and category", "page", "size", "category")
			if (err != nil) != c.wantErr {
				t.Fatalf("parseInts err = %v, wantErr %v", err, c.wantErr)
			}
			if err == nil {
				for i := range c.want {
					if got[i] != c.want[i] {
						t.Fatalf("parseInts[%d] = %d, want %d", i, got[i], c.want[i])
					}
				}
			}
		})
	}
}

func TestParseID(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{"numeric id", "42", 42, false},
		{"non-numeric errors", "abc", 0, true},
		{"empty errors", "", 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := &http.Request{}
			r.SetPathValue("id", c.raw)
			got, err := parseID(r, "id", "category id")
			if (err != nil) != c.wantErr {
				t.Fatalf("parseID(%q) err = %v, wantErr %v", c.raw, err, c.wantErr)
			}
			if err == nil && got != c.want {
				t.Fatalf("parseID(%q) = %d, want %d", c.raw, got, c.want)
			}
		})
	}
}
