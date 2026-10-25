package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A liveness probe must never touch the database. If it did, a disk problem
// would look like a dead process and a supervisor would restart-loop the
// service — turning a degraded site into a full outage.
func TestHealthzDoesNotDependOnTheDatabase(t *testing.T) {
	_, db := setup(t)
	// Break the pool completely.
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	rec := httptest.NewRecorder()
	Healthz(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness must answer 200 even with a dead database, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "ok") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

// The readiness probe is the one that has to notice. Before this existed,
// /health answered {"status":"ok"} unconditionally, so a site whose database had
// become unreadable looked healthy to every monitor while every request failed.
func TestReadyFailsWhenDatabaseIsUnusable(t *testing.T) {
	_, db := setup(t)

	rec := httptest.NewRecorder()
	Ready(rec, httptest.NewRequest("GET", "/health/ready", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("a working database must be ready, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ready") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	// Now make the database unusable and confirm the probe notices.
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	rec = httptest.NewRecorder()
	Ready(rec, httptest.NewRequest("GET", "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("a dead database must be reported as not ready, got %d", rec.Code)
	}
	// The response must not describe the failure: the probe is unauthenticated.
	body := rec.Body.String()
	if strings.Contains(body, "sql") || strings.Contains(body, "database") ||
		strings.Contains(body, "closed") {
		t.Fatalf("readiness response leaks internals: %s", body)
	}
}
