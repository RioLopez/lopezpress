package handlers

import (
	"encoding/json"
	"net/http"

	"echo-content-api/internal/services"
)

// --- content moderation + banned vault (spec: review workflow) ---
//
// These endpoints implement the three-tier review flow and the root-only
// banned vault:
//
//	GET  /api/admin/pending          admin|root  review queue
//	POST /api/admin/review/{id}     admin|root  approve / reject
//	POST /api/admin/ban/{id}        root        pull a post into the vault
//	GET  /api/admin/banned          root        list the vault
//	POST /api/admin/banned/{id}/send-back  root send back to author + note
//	DELETE /api/admin/banned/{id}   root        permanent delete from vault
//
// The service layer re-checks every privilege; the guards below are the
// HTTP-level first line of defence.

// ListPending returns the review queue scoped to the caller's role.
// admin sees pending posts by plain users; root sees every pending post.
// GET /api/admin/pending (auth, admin|root)
func ListPending(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	posts, err := svc.ListPending(r.Context(), userFromContext(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// ReviewPost approves or rejects a pending post.
// POST /api/admin/review/{id} (auth, admin|root)
// Body: { "decision": "approve"|"reject", "note": "..." } (note required on reject)
func ReviewPost(w http.ResponseWriter, r *http.Request) {
	if !requireAdmin(w, r) {
		return
	}
	id := r.PathValue("id")
	var in services.ReviewPostInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	post, err := svc.ReviewPost(r.Context(), userFromContext(r), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Audit is recorded by the service layer (RecordAction "review_post");
	// not duplicated here to keep a single source of truth (P1-8).
	writeJSON(w, http.StatusOK, post)
}

// BanPost pulls a post into the banned vault (root-only).
// POST /api/admin/ban/{id} (auth, root)
func BanPost(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	id := r.PathValue("id")
	post, err := svc.BanPost(r.Context(), userFromContext(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Audit recorded by service (RecordAction "ban_post").
	writeJSON(w, http.StatusOK, post)
}

// ListBanned returns every post in the banned vault (root-only).
// GET /api/admin/banned (auth, root)
func ListBanned(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	posts, err := svc.ListBanned(r.Context(), userFromContext(r))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, posts)
}

// SendBackPost returns a banned post to its author with a reviewer note.
// POST /api/admin/banned/{id}/send-back (auth, root)
// Body: { "note": "..." } (note required)
func SendBackPost(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	id := r.PathValue("id")
	var in struct {
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	post, err := svc.SendBackPost(r.Context(), userFromContext(r), id, in.Note)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	// Audit recorded by service (RecordAction "send_back_post").
	writeJSON(w, http.StatusOK, post)
}

// DeleteBannedPermanently hard-deletes a banned post from the vault.
// DELETE /api/admin/banned/{id} (auth, root)
func DeleteBannedPermanently(w http.ResponseWriter, r *http.Request) {
	if !requireRoot(w, r) {
		return
	}
	id := r.PathValue("id")
	if err := svc.DeleteBannedPermanently(r.Context(), userFromContext(r), id); err != nil {
		writeServiceError(w, err)
		return
	}
	// Audit recorded by service (RecordAction "delete_banned").
	w.WriteHeader(http.StatusNoContent)
}
