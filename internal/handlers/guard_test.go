package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lopezpress/internal/models"
	"lopezpress/internal/services"
)

// TestRequireRole locks the P4 route-level role guard: it must admit callers
// whose role is listed, reject everyone else 403, and keep the pre-P4 error
// messages ("root only" / "admin only") for a stable client contract.
func TestRequireRole(t *testing.T) {
	cases := []struct {
		name    string
		roles   []string
		user    models.User
		want    int
		wantMsg string // substring expected in the JSON body; "" = no check
		called  bool   // whether the wrapped handler was reached
	}{
		{
			name:   "root allowed on root-only",
			roles:  []string{models.RoleRoot},
			user:   models.User{Role: models.RoleRoot},
			want:   http.StatusOK,
			called: true,
		},
		{
			name:    "admin denied on root-only",
			roles:   []string{models.RoleRoot},
			user:    models.User{Role: models.RoleAdmin},
			want:    http.StatusForbidden,
			wantMsg: "root only",
			called:  false,
		},
		{
			name:    "anonymous denied on root-only",
			roles:   []string{models.RoleRoot},
			user:    models.User{},
			want:    http.StatusForbidden,
			wantMsg: "root only",
			called:  false,
		},
		{
			name:   "admin allowed on admin|root",
			roles:  []string{models.RoleAdmin, models.RoleRoot},
			user:   models.User{Role: models.RoleAdmin},
			want:   http.StatusOK,
			called: true,
		},
		{
			name:   "root allowed on admin|root",
			roles:  []string{models.RoleAdmin, models.RoleRoot},
			user:   models.User{Role: models.RoleRoot},
			want:   http.StatusOK,
			called: true,
		},
		{
			name:    "user denied on admin|root",
			roles:   []string{models.RoleAdmin, models.RoleRoot},
			user:    models.User{Role: models.RoleUser},
			want:    http.StatusForbidden,
			wantMsg: "admin only",
			called:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			}

			guard := RequireRole(tc.roles...)(next)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(services.WithActor(req.Context(), tc.user, ""))
			rec := httptest.NewRecorder()
			guard(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d (%s)", tc.want, rec.Code, rec.Body.String())
			}
			if called != tc.called {
				t.Fatalf("handler called = %v, want %v", called, tc.called)
			}
			if tc.wantMsg != "" && !strings.Contains(rec.Body.String(), tc.wantMsg) {
				t.Fatalf("expected body to contain %q, got %q", tc.wantMsg, rec.Body.String())
			}
		})
	}
}
