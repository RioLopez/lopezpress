package services

import (
	"context"
	"net/http"
	"strconv"

	"lopezpress/internal/models"
)

// --- anonymous read switch (spec 27) ---

// AllowAnonymous reports whether visitors without a token may read the site.
//
// The flag lives in site_configs (admin-editable), so reading it per request
// would add a query to every single read. It is cached in memory and
// invalidated by UpdateSiteConfig; a stale cache for at most one write is
// acceptable because the switch is not security-critical in the "must fail
// closed" sense (it gates *public* content only).
func (s *Service) AllowAnonymous(ctx context.Context) bool {
	s.publicReadMu.RLock()
	if s.publicReadSet {
		v := s.publicRead
		s.publicReadMu.RUnlock()
		return v
	}
	s.publicReadMu.RUnlock()

	cfg, err := s.GetSiteConfig(ctx, false)
	value := true // a missing row means "public", matching the column default
	if err == nil {
		value = cfg.PublicRead
	}

	s.publicReadMu.Lock()
	s.publicRead = value
	s.publicReadSet = true
	s.publicReadMu.Unlock()
	return value
}

// invalidatePublicRead drops the cached flag so the next request re-reads it.
func (s *Service) invalidatePublicRead() {
	s.publicReadMu.Lock()
	s.publicReadSet = false
	s.publicReadMu.Unlock()
}

// --- preset category tree (spec 27) ---

// PresetResult reports what applying the preset tree actually changed, so the
// UI can say "created 5, re-parented 2" instead of a bare "done".
type PresetResult struct {
	Created    int `json:"created"`
	Reparented int `json:"reparented"`
}

// ApplyPresetCategories materialises models.PresetCategories into the
// database, idempotently.
//
// Rules (spec 27 §5):
//   - missing parent/child → created;
//   - existing child with no parent → adopted (parent set, nothing recreated);
//   - existing child that already has a parent → left alone (respect manual
//     decisions);
//   - a category that itself has children is never adopted (that would create
//     a third level, which the two-level rule forbids).
func (s *Service) ApplyPresetCategories(ctx context.Context) (PresetResult, error) {
	res := PresetResult{}

	for _, root := range models.PresetCategories {
		rootID, created, _, err := s.ensurePresetCategory(ctx, root.Name, nil)
		if err != nil {
			return res, err
		}
		if created {
			res.Created++
		}
		parentID := rootID
		for _, childName := range root.Children {
			_, childCreated, adopted, err := s.ensurePresetCategory(ctx, childName, &parentID)
			if err != nil {
				return res, err
			}
			if childCreated {
				res.Created++
			}
			if adopted {
				res.Reparented++
			}
		}
	}
	s.audit(ctx, models.User{}, "apply_preset_categories", "categories",
		"created="+strconv.Itoa(res.Created)+"; reparented="+strconv.Itoa(res.Reparented))
	return res, nil
}

// ensurePresetCategory finds or creates one node. It returns the id, whether
// the row was created, and whether an existing row was adopted under parentID.
func (s *Service) ensurePresetCategory(ctx context.Context, name string, parentID *int64) (int64, bool, bool, error) {
	var cat models.Category
	err := s.db.WithContext(ctx).Where("name = ?", name).First(&cat).Error
	if err == nil {
		// Exists. Adopt only when it is currently top-level and childless.
		if parentID == nil || cat.ParentID != nil || cat.ID == *parentID {
			return cat.ID, false, false, nil
		}
		var kids int64
		if err := s.db.WithContext(ctx).Model(&models.Category{}).Where("parent_id = ?", cat.ID).Count(&kids).Error; err != nil {
			return cat.ID, false, false, newErr(http.StatusInternalServerError, "failed to inspect categories")
		}
		if kids > 0 {
			return cat.ID, false, false, nil // would create a third level
		}
		if err := s.db.WithContext(ctx).Model(&models.Category{}).Where("id = ?", cat.ID).
			Update("parent_id", *parentID).Error; err != nil {
			return cat.ID, false, false, newErr(http.StatusInternalServerError, "failed to re-parent category")
		}
		return cat.ID, false, true, nil
	}

	created := models.Category{Name: name, ParentID: parentID}
	if err := s.db.WithContext(ctx).Create(&created).Error; err != nil {
		// A concurrent apply could have inserted it between the read and the
		// write; re-read once so the operation stays idempotent.
		var again models.Category
		if e2 := s.db.WithContext(ctx).Where("name = ?", name).First(&again).Error; e2 == nil {
			return again.ID, false, false, nil
		}
		return 0, false, false, newErr(http.StatusInternalServerError, "failed to create preset category")
	}
	return created.ID, true, false, nil
}
