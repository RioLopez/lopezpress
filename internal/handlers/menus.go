package handlers

import (
	"encoding/json"
	"net/http"

	"lopezpress/internal/services"
)

// ListMenuItems returns the admin sidebar, including hidden rows.
// GET /api/admin/menus (auth, admin)
func ListMenuItems(w http.ResponseWriter, r *http.Request) {
	rows, err := svc.ListMenuItems(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}

// CreateMenuItem adds an operator link to the sidebar.
// POST /api/admin/menus (auth, root)
func CreateMenuItem(w http.ResponseWriter, r *http.Request) {
	var in services.MenuItemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	row, err := svc.CreateMenuItem(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

// UpdateMenuItem edits label/visibility (built-in) or the whole custom row.
// PUT /api/admin/menus/{id} (auth, root)
func UpdateMenuItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id", "menu item id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var in services.MenuItemInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	row, err := svc.UpdateMenuItem(r.Context(), id, in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

// DeleteMenuItem removes a custom link; built-ins can only be hidden.
// DELETE /api/admin/menus/{id} (auth, root)
func DeleteMenuItem(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id", "menu item id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := svc.DeleteMenuItem(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReorderMenuItems persists a full sidebar order.
// POST /api/admin/menus/reorder (auth, root)
func ReorderMenuItems(w http.ResponseWriter, r *http.Request) {
	var in services.MenuReorderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeInvalidBody(w)
		return
	}
	rows, err := svc.ReorderMenuItems(r.Context(), in)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rows)
}
