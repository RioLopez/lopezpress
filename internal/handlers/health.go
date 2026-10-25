package handlers

import (
	"log"
	"net/http"
)

// Healthz is the liveness probe: is this process running and able to answer?
//
// It intentionally does NOT touch the database. A supervisor that restarts the
// process because the disk is full turns a degraded site into a restart loop,
// and no restart has ever fixed a corrupt SQLite file. "The process is up" and
// "the instance should receive traffic" are different questions; this one
// answers only the first.
//
// Answers 200 always, with no body beyond a fixed string: an unauthenticated
// endpoint should not describe the internals of the process.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// Ready tells a load balancer or a supervisor whether this instance can
// actually serve a request.
//
// It probes the database with a short deadline, because a process that is
// running but cannot read its own data will fail every real request — the exact
// condition a probe exists to surface. The response carries no error detail:
// the reason is written to the log, and a probe is unauthenticated.
func Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := svc.ReadinessProbe(); err != nil {
		// Log once per failed probe with the real cause; answer with a code.
		// A supervisor polling every few seconds would otherwise fill the log
		// with the same line, so this relies on the caller's own rate — which
		// is a handful per minute, not per request.
		logReadinessFailure(r, err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"unavailable"}`))
		return
	}
	_, _ = w.Write([]byte(`{"status":"ready"}`))
}

// logReadinessFailure keeps the real error in the server log while the response
// stays blank. Separated so a test can assert the response detail without
// depending on log capture.
func logReadinessFailure(r *http.Request, err error) {
	log.Printf("readiness probe failed: %v", err)
}
