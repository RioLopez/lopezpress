package services

import (
	"context"
	"fmt"
	"net/http"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// --- image library: quotas, stats, listing ------------------------
//
// Deletion and the reference ledger live in media_refs.go.

const (
	uploadDefaultPageSize = 24
	uploadMaxPageSize     = 100
)

// humanBytes renders a byte count for an error message or a UI badge.
func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	value := float64(n)
	for _, u := range units {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, u)
		}
	}
	return fmt.Sprintf("%.1f PB", value/1024)
}

// checkUploadQuota enforces the two aggregate ceilings before a byte is
// written, so a rejected upload never leaves a half-written file behind.
//
// The site total counts soft-deleted rows as well: those bytes are still on
// disk, which is exactly what this ceiling protects. A user's own usage counts
// only their live rows, because "delete it to free space" has to actually work
// from their point of view.
//
// Prefer checkUploadQuotaTx: this standalone form is what makes the TOCTOU
// window measurable, because the comparison and the INSERT it guards are two
// separate round trips. It is kept for callers that legitimately only need to
// report usage (the admin "would this fit?" preview), never for a write path.
func (s *Service) checkUploadQuota(ctx context.Context, cfg models.SiteConfig, ownerID string, add int64) error {
	return s.checkUploadQuotaTx(s.db.WithContext(ctx), cfg, ownerID, add)
}

// checkUploadQuotaTx is checkUploadQuota bound to a caller's transaction. The
// upload path runs the check and the INSERT inside one transaction so the two
// cannot be interleaved by a concurrent upload — see storeLocalUpload.
func (s *Service) checkUploadQuotaTx(db *gorm.DB, cfg models.SiteConfig, ownerID string, add int64) error {
	var siteUsed int64
	if err := db.Raw(`SELECT COALESCE(SUM(size), 0) FROM upload_files`).Scan(&siteUsed).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to read storage usage")
	}
	if cfg.SiteQuotaBytes > 0 && siteUsed+add > cfg.SiteQuotaBytes {
		return newErr(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("site storage is full (%s of %s used)", humanBytes(siteUsed), humanBytes(cfg.SiteQuotaBytes)))
	}

	if ownerID == "" {
		return nil
	}
	var mine int64
	if err := db.Raw(`SELECT COALESCE(SUM(size), 0) FROM upload_files WHERE owner_id = ? AND deleted_at IS NULL`, ownerID).
		Scan(&mine).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to read your storage usage")
	}
	if cfg.UserQuotaBytes > 0 && mine+add > cfg.UserQuotaBytes {
		return newErr(http.StatusRequestEntityTooLarge,
			fmt.Sprintf("your storage quota is full (%s of %s used)", humanBytes(mine), humanBytes(cfg.UserQuotaBytes)))
	}
	return nil
}

// uploadStats collects the counters the gallery header shows.
//
// DeletedBytes is reported separately on purpose: soft-deleted files still
// occupy the disk, so a site that looks 40% full to its users can be 90% full
// in reality. The admin needs both numbers to know whether "clean up" means
// "remove withdrawn uploads" or "the site is genuinely out of room".
func (s *Service) uploadStats(ctx context.Context) UploadStats {
	db := s.db.WithContext(ctx)
	var st UploadStats
	db.Raw(`SELECT COUNT(*) FROM upload_files WHERE deleted_at IS NULL`).Scan(&st.LiveCount)
	db.Raw(`SELECT COALESCE(SUM(size), 0) FROM upload_files WHERE deleted_at IS NULL`).Scan(&st.LiveBytes)
	db.Raw(`SELECT COUNT(*) FROM upload_files WHERE deleted_at IS NOT NULL`).Scan(&st.DeletedCount)
	db.Raw(`SELECT COALESCE(SUM(size), 0) FROM upload_files WHERE deleted_at IS NOT NULL`).Scan(&st.DeletedBytes)
	db.Raw(`SELECT COUNT(*) FROM upload_files WHERE deleted_at IS NULL AND ref_count = 0`).Scan(&st.OrphanCount)
	st.TotalCount = st.LiveCount + st.DeletedCount
	st.TotalBytes = st.LiveBytes + st.DeletedBytes
	return st
}

// applyUploadFilter translates the query struct into WHERE clauses.
func applyUploadFilter(db *gorm.DB, q UploadListQuery) *gorm.DB {
	switch q.Status {
	case "deleted":
		db = db.Where("deleted_at IS NOT NULL")
	case "all":
	default:
		db = db.Where("deleted_at IS NULL")
	}
	if q.OwnerID != "" {
		db = db.Where("owner_id = ?", q.OwnerID)
	}
	if q.Kind != "" {
		db = db.Where("kind = ?", q.Kind)
	}
	return db
}

// ListUploads returns one page of the library plus the storage counters.
// OwnerID is what turns the admin view into a personal one; the handlers
// decide which of the two a caller gets, not the service.
func (s *Service) ListUploads(ctx context.Context, q UploadListQuery) (UploadPage, error) {
	// Same overflow trap as the post lists: an unclamped page number makes the
	// offset wrap and the library silently answers with page one.
	q.Page = ClampPageNumber(q.Page)
	if q.Size <= 0 || q.Size > uploadMaxPageSize {
		q.Size = uploadDefaultPageSize
	}

	var total int64
	if err := applyUploadFilter(s.db.WithContext(ctx).Model(&models.UploadFile{}), q).
		Count(&total).Error; err != nil {
		return UploadPage{}, newErr(http.StatusInternalServerError, "failed to count uploads")
	}

	var rows []models.UploadFile
	if err := applyUploadFilter(s.db.WithContext(ctx).Model(&models.UploadFile{}), q).
		Order("created_at DESC").Limit(q.Size).Offset((q.Page - 1) * q.Size).
		Find(&rows).Error; err != nil {
		return UploadPage{}, newErr(http.StatusInternalServerError, "failed to list uploads")
	}

	page := UploadPage{
		Items: make([]UploadFileInfo, 0, len(rows)),
		Total: total,
		Page:  q.Page,
		Size:  q.Size,
		Stats: s.uploadStats(ctx),
	}

	// Resolve owner names in one query instead of a join that would repeat for
	// every row (and would need a LEFT JOIN anyway for legacy, owner-less rows).
	names := s.ownerNames(ctx, rows)
	cfg, _ := s.GetSiteConfig(ctx, false)
	page.SiteQuotaBytes = cfg.SiteQuotaBytes
	page.SiteUsedBytes = page.Stats.TotalBytes
	page.UserQuotaBytes = cfg.UserQuotaBytes
	if q.OwnerID != "" {
		s.db.WithContext(ctx).
			Raw(`SELECT COALESCE(SUM(size), 0) FROM upload_files WHERE owner_id = ? AND deleted_at IS NULL`, q.OwnerID).
			Scan(&page.UserUsedBytes)
	}

	for _, r := range rows {
		page.Items = append(page.Items, uploadFileInfo(r, names[r.OwnerID]))
	}
	return page, nil
}

// ownerNames maps owner ids to display names for one page of rows.
func (s *Service) ownerNames(ctx context.Context, rows []models.UploadFile) map[string]string {
	ids := make([]string, 0, len(rows))
	seen := map[string]bool{}
	for _, r := range rows {
		if r.OwnerID == "" || seen[r.OwnerID] {
			continue
		}
		seen[r.OwnerID] = true
		ids = append(ids, r.OwnerID)
	}
	out := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return out
	}
	var users []models.User
	if err := s.db.WithContext(ctx).Select("id", "username", "display_name").
		Where("id IN ?", ids).Find(&users).Error; err != nil {
		return out
	}
	for _, u := range users {
		if u.DisplayName != "" {
			out[u.ID] = u.DisplayName
		} else {
			out[u.ID] = u.Username
		}
	}
	return out
}

// uploadFileInfo maps the row to the API shape. Owner-less legacy rows get an
// empty owner name, which the UI renders as "legacy".
func uploadFileInfo(f models.UploadFile, ownerName string) UploadFileInfo {
	return UploadFileInfo{
		ID: f.ID, Name: f.Name, URL: "/uploads/" + f.Name, ThumbURL: thumbURL(f),
		Size: f.Size, Width: f.Width, Height: f.Height, Mime: f.Mime,
		Kind: f.Kind, OwnerID: f.OwnerID, OwnerName: ownerName,
		OriginalName: f.OriginalName, IP: f.IP, Sha256: f.Sha256,
		RefCount: f.RefCount, Legacy: f.Legacy,
		Deleted: f.DeletedAt != nil, DeletedAt: f.DeletedAt,
		CreatedAt: f.CreatedAt,
	}
}
