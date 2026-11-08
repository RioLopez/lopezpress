package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"lopezpress/internal/models"
)

// ExportActions streams the audit trail as CSV (UTF-8 with BOM so Excel opens
// Chinese detail text correctly). The export honors the same actor/keyword
// filters as the page, so "what I can see is what I download".
//
// GET /api/admin/actions/export?actor=<user-id>&q=<keyword> (auth, admin)
func ExportActions(w http.ResponseWriter, r *http.Request) {
	rows, err := svc.ExportActions(r.Context(), r.URL.Query().Get("actor"), r.URL.Query().Get("q"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	filename := "actions-" + time.Now().Format("20060102-150405") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})

	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"time", "actor", "action", "target", "detail", "ip"})
	for _, row := range rows {
		_ = cw.Write(actionCSVRow(row))
	}
	cw.Flush()
}

func actionCSVRow(a models.ActionLog) []string {
	return []string{
		a.CreatedAt.Format("2006-01-02 15:04:05"),
		csvSafe(a.ActorName),
		csvSafe(a.Action),
		csvSafe(a.Target),
		csvSafe(a.Detail),
		csvSafe(a.IP),
	}
}

// csvSafe neutralizes spreadsheet formula injection. A target/detail value
// such as "=HYPERLINK(...)" is data to us, but a command to Excel, so cells
// that start with a formula sigil are prefixed with an apostrophe.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@':
		return "'" + s
	default:
		return s
	}
}
