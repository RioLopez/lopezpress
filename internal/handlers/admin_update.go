package handlers

import (
	"encoding/json"
	"net/http"
)

// Self-update surface (root-only; the router table enforces that, handlers do
// not re-check roles — see the P4 note in router.go).
//
// The three endpoints deliberately mirror the backup surface's shape:
//   - status is a poll target (the UI refreshes it every couple of seconds
//     while a job runs), so it never talks to GitHub synchronously on the hot
//     path beyond the 60-second cache inside the service;
//   - apply returns 202 immediately and the work continues in a background
//     goroutine — nginx's 60 s proxy_read_timeout would kill a synchronous
//     download-and-swap, so "fire + poll" is the only honest shape;
//   - rollback is local file renames and genuinely finishes fast.

// GetUpdateStatus reports the running build, the repo's latest release
// (cached), rollback options on disk, the current job state and history.
// GET /api/admin/update/status (auth, root)
func GetUpdateStatus(w http.ResponseWriter, r *http.Request) {
	dto, err := svc.UpdateStatus(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

// ApplyUpdate starts an update job toward the latest release (empty body /
// empty version) or a pinned tag ("1.0.32" / "v1.0.32").
// POST /api/admin/update/apply  body: { "version": "" } (auth, root)
func ApplyUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version string `json:"version"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			// An empty body is a valid "update to latest"; anything unparsable
			// that is not empty is a client bug.
			if r.ContentLength != 0 {
				writeError(w, http.StatusBadRequest, "请求体不是合法 JSON")
				return
			}
		}
	}
	if err := svc.StartUpdate(in.Version); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"started": true})
}

// RollbackUpdate swaps the running build for a kept lopezpress.v<ver> +
// web.v<ver> pair, then restarts. Returns before the restart fires so the
// client can render the "restarting" state.
// POST /api/admin/update/rollback  body: { "version": "1.0.30" } (auth, root)
func RollbackUpdate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "请求体不是合法 JSON")
		return
	}
	if err := svc.RollbackUpdate(in.Version); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"restarting": true})
}
