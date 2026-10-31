package handlers

import (
	"encoding/json"
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

// The probe answers with the build identity, so "which binary is live" stops
// being an ssh question. The fields must never be empty: a blank version reads
// as "no answer" and gets ignored, while the honest defaults ("0.0.0-dev",
// "unknown") send the reader to check the build.
// /health is unauthenticated, so by default it must say nothing about which
// build is running. It used to answer with the version and commit sha on the
// theory that a public repository makes them harmless — which misses that the
// probe is on the internet for anyone scanning, not just for people who already
// know the project. This test pins the default.
func TestHealthzHidesBuildIdentityByDefault(t *testing.T) {
	setup(t)
	prev := cfg.HealthShowVersion
	cfg.HealthShowVersion = false
	t.Cleanup(func() { cfg.HealthShowVersion = prev })

	rec := httptest.NewRecorder()
	Healthz(rec, httptest.NewRequest("GET", "/health", nil))

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body is not JSON: %v (%s)", err, rec.Body.String())
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %q, want ok", body["status"])
	}
	if v, ok := body["version"]; ok {
		t.Fatalf("version must not be exposed by default, got %q", v)
	}
	if v, ok := body["commit"]; ok {
		t.Fatalf("commit must not be exposed by default, got %q", v)
	}
}

// HEALTH_SHOW_VERSION=true is the opt-in for a private/staging deployment whose
// load balancer reads the build over HTTP. The switch is the whole feature; a
// test that only checked the "on" path would pass with the default flipped.
func TestHealthzReportsBuildIdentityWhenEnabled(t *testing.T) {
	setup(t)
	prev := cfg.HealthShowVersion
	cfg.HealthShowVersion = true
	t.Cleanup(func() { cfg.HealthShowVersion = prev })

	rec := httptest.NewRecorder()
	Healthz(rec, httptest.NewRequest("GET", "/health", nil))

	var body struct {
		Status  string `json:"status"`
		Version string `json:"version"`
		Commit  string `json:"commit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("health body is not JSON: %v (%s)", err, rec.Body.String())
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want ok", body.Status)
	}
	if body.Version == "" || body.Commit == "" {
		t.Fatalf("build identity missing: version=%q commit=%q", body.Version, body.Commit)
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
