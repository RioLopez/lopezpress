package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"echo-content-api/internal/models"
	"echo-content-api/internal/services"
)

// requireAdmin checks the caller is an admin or the root and writes 403
// otherwise. Returns true when the request may continue.
func requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !services.IsAdminOrRoot(userFromContext(r)) {
		writeError(w, http.StatusForbidden, "admin only")
		return false
	}
	return true
}

// requireRoot checks the caller is the single super-admin and writes 403
// otherwise. Used for the banned-vault endpoints, which are root-only.
func requireRoot(w http.ResponseWriter, r *http.Request) bool {
	if !services.IsRoot(userFromContext(r)) {
		writeError(w, http.StatusForbidden, "root only")
		return false
	}
	return true
}

// toUserRow projects a models.User onto the admin-facing UserWithDisabled DTO.
func toUserRow(u models.User) services.UserWithDisabled {
	return services.UserWithDisabled{
		ID:           u.ID,
		Username:     u.Username,
		Email:        u.Email,
		Role:         u.Role,
		Disabled:     u.Disabled,
		DisplayName:  u.DisplayName,
		LastLoginIP:  u.LastLoginIP,
		LastLoginAt:  u.LastLoginAt,
		CreatedAt:    u.CreatedAt,
	}
}

// DownloadBackup streams a backup file written by ExportBackup to the browser.
// GET /api/backup/download?file=<name> (auth, admin)
//
// The file name must be a bare name (no path separators); the service layer
// re-validates it so a tampered query cannot read files outside the backup
// directory. Callers get 404 rather than 403 to avoid leaking existence.
func DownloadBackup(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	name := r.URL.Query().Get("file")
	if err := svc.ServeBackup(w, r, name); err != nil {
		writeServiceError(w, err)
		return
	}
}

// ListUsers returns every account with its admin-disabled flag.
// GET /api/users (auth, admin)
func ListUsers(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	users, err := svc.ListUsers(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, users)
}

// SetUserDisabled toggles the disabled flag on a user.
// PUT /api/users/{id}/disabled  body: { "disabled": bool } (auth, admin)
func SetUserDisabled(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")

	var in struct {
		Disabled bool `json:"disabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}

	actor := userFromContext(r)
	user, err := svc.SetUserDisabled(r.Context(), actor, id, in.Disabled)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "set_user_disabled", "user:"+id, "disabled="+strconvBool(in.Disabled))
	writeJSON(w, http.StatusOK, services.UserWithDisabled{
		ID: user.ID, Username: user.Username, Email: user.Email,
		Role: user.Role, Disabled: user.Disabled, CreatedAt: user.CreatedAt,
	})
}

// strconvBool is a tiny helper used only for audit detail strings.
func strconvBool(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ExportBackup writes a JSON dump of the database and returns the result.
// POST /api/backup (auth, admin)
func ExportBackup(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	res, err := svc.ExportBackup(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "export_backup", "backup:"+res.Filename, "size="+strconv.FormatInt(res.Size, 10))
	writeJSON(w, http.StatusOK, res)
}

// ImportBackup restores the database from an exported backup JSON file. The
// body is the raw backup payload (same shape ExportBackup produces).
// POST /api/backup/import (auth, admin) — destructive: replaces current data.
//
// Spec #6 enhancements:
//   - ?dry_run=1 returns the validated stats WITHOUT applying the import and
//     without taking a snapshot, so an operator can preview before committing.
//   - The payload's schema_version / app_version are validated up front; an
//     incompatible dump is rejected with 400 before any write happens.
//   - A non-dry-run import automatically takes a VACUUM INTO snapshot (keep 3)
//     before the destructive replace, and returns the snapshot file name.
// ImportBackup restores the database from an exported backup JSON file. The
// payload can carry arbitrary user rows (incl. role "root" and foreign
// password hashes), so importing is a whole-database replace and is
// root-only (as-built audit Q7). The service re-checks the actor.
func ImportBackup(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	dryRun := r.URL.Query().Get("dry_run") == "1" || r.URL.Query().Get("dry_run") == "true"
	var in services.ImportData
	// Cap the decoded body so a maliciously/huge upload cannot exhaust the
	// 1.5 GB instance's memory (P1-6). A backup is a few MB at most.
	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "backup too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid backup JSON")
		return
	}
	res, err := svc.ImportBackup(r.Context(), userFromContext(r), in, dryRun)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "import_backup", "backup", "dry_run="+strconvBool(dryRun))
	writeJSON(w, http.StatusOK, res)
}

// RestoreSnapshot restores the database from a previously created VACUUM INTO
// snapshot file (spec #6). The name is validated server-side (snapshot-*.db,
// no path components).
// POST /api/backup/restore-snapshot  body: {"file":"snapshot-20260101-000000.db"}
// (auth, root-only: whole-database replace, as-built audit Q7)
func RestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	var in struct {
		File string `json:"file"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	stats, err := svc.RestoreSnapshot(r.Context(), userFromContext(r), in.File)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// A destructive database restore is a security-relevant event; record who
	// did it and from which snapshot (P1-7).
	recordAudit(r, "restore_snapshot", "snapshot:"+in.File, "ok")
	writeJSON(w, http.StatusOK, stats)
}

// ListSnapshots returns the available VACUUM INTO snapshot files for the
// restore UI (P2-14). Root-only: a snapshot holds every account and password
// hash, so it is gated like RestoreSnapshot itself (as-built audit Q7).
// GET /api/backup/snapshots (auth, root)
func ListSnapshots(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	snaps, err := svc.ListSnapshots(r.Context(), userFromContext(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

// ListTrash returns soft-deleted posts inside the 7-day recovery window.
// GET /api/trash (auth, admin)
func ListTrash(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	rows, err := svc.ListTrash(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// RestoreTrash resurrects a single soft-deleted post.
// POST /api/trash/{id}/restore (auth, admin)
func RestoreTrash(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	if err := svc.RestoreTrash(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "restore_post", "post:"+id, "")
	w.WriteHeader(http.StatusNoContent)
}

// EmptyTrash permanently deletes every soft-deleted post.
// POST /api/trash/empty (auth, admin)
func EmptyTrash(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	n, err := svc.EmptyTrash(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	recordAudit(r, "empty_trash", "trash", "deleted="+strconv.Itoa(n))
	writeJSON(w, http.StatusOK, map[string]int{"deleted": n})
}

// ListActions returns the site's recent admin/content operations (audit trail).
// GET /api/admin/actions?limit=50&actor=<user-id> (auth, admin)
func ListActions(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	limit := parseLimit(r.URL.Query(), "limit", 50, 200)
	actor := r.URL.Query().Get("actor")
	rows, err := svc.ListActions(r.Context(), actor, limit)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}
