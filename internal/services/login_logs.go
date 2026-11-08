package services

import (
	"context"
	"net/http"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// Login history and its reader. Split out of site_config.go to keep that file
// inside the size gate; these two concerns share no state, they only happened
// to be written in the same pass.

// maxLoginIdentifierLen bounds the identifier written to login_logs. The
// column declares size:128 but SQLite does not enforce it, so without this
// clamp a login attempt carrying a multi-megabyte "email" field stores the
// whole string: R3-05 measured two such requests growing the database by
// 53 MB and leaving a 26,214,407-byte row that made every login-history
// query pull it back.
const maxLoginIdentifierLen = 254

// clampIdentifier trims an identifier to the log limit. Kept as a helper so
// the login path and any future caller share one bound.
func clampIdentifier(s string) string {
	if len(s) <= maxLoginIdentifierLen {
		return s
	}
	return s[:maxLoginIdentifierLen]
}

func (s *Service) RecordLogin(ctx context.Context, userID, identifier, ip, ua string, success bool) {
	log := models.LoginLog{
		UserID: userID, Identifier: clampIdentifier(identifier), IP: clampIdentifier(ip), UA: clampIdentifier(ua), Success: success,
	}
	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		_ = err // non-fatal
	}
}

// ListActions returns the most recent action_log rows (optionally filtered
// by an actor and a keyword). Admin endpoint — the route table enforces the
// role (P4).
func (s *Service) ListActions(ctx context.Context, actorID, q string, limit int) ([]models.ActionLog, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := actionQuery(s.db.WithContext(ctx), actorID, q)
	var out []models.ActionLog
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Find(&out).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load actions")
	}
	if out == nil {
		out = []models.ActionLog{}
	}
	return out, nil
}

// maxActionExportRows bounds one export. The audit trail grows forever and a
// CSV response is assembled in memory, so an unbounded "export everything"
// would be a denial-of-service vector on a small box. 20k rows is far more
// than an operator needs to reconcile one incident, and the response states
// when it was truncated so nobody mistakes a prefix for the whole history.
const maxActionExportRows = 20000

// ExportActions returns every action_log row matching the filters, newest
// first, up to maxActionExportRows.
func (s *Service) ExportActions(ctx context.Context, actorID, q string) ([]models.ActionLog, error) {
	query := actionQuery(s.db.WithContext(ctx), actorID, q)
	var out []models.ActionLog
	if err := query.Order("created_at DESC, id DESC").Limit(maxActionExportRows).Find(&out).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load actions")
	}
	if out == nil {
		out = []models.ActionLog{}
	}
	return out, nil
}

// actionQuery applies the two filters both readers share, so the page and the
// CSV export never disagree about what "matching rows" means.
func actionQuery(db *gorm.DB, actorID, q string) *gorm.DB {
	query := db.Model(&models.ActionLog{})
	if actorID != "" {
		query = query.Where("actor_id = ?", actorID)
	}
	if q != "" {
		like := "%" + escapeLike(q) + "%"
		query = query.Where(
			"actor_name LIKE ? ESCAPE '\\' OR action LIKE ? ESCAPE '\\' OR target LIKE ? ESCAPE '\\' OR detail LIKE ? ESCAPE '\\'",
			like, like, like, like,
		)
	}
	return query
}
