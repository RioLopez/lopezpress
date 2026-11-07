package services

import (
	"context"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

var menuGroups = map[string]bool{"": true, "content": true, "users": true, "system": true}

// ListMenuItems returns the sidebar in display order. Hidden rows are included
// on purpose: the management page has to show them to un-hide them.
func (s *Service) ListMenuItems(ctx context.Context) ([]models.MenuItem, error) {
	var out []models.MenuItem
	if err := s.db.WithContext(ctx).Order("sort ASC, id ASC").Find(&out).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load menu")
	}
	if out == nil {
		out = []models.MenuItem{}
	}
	return out, nil
}

func validateMenuInput(in MenuItemInput, custom bool) error {
	if custom {
		label := strings.TrimSpace(in.Label)
		if label == "" || len(label) > MaxMenuLabelLen {
			return newErr(http.StatusBadRequest, "label must be 1-64 characters")
		}
	}
	if custom || in.Path != "" {
		if err := validateMenuPath(in.Path); err != nil {
			return err
		}
	}
	if custom || in.GroupKey != "" {
		if !menuGroups[in.GroupKey] {
			return newErr(http.StatusBadRequest, "unknown menu group")
		}
	}
	return nil
}

// validateMenuPath keeps the sidebar inside this site. A leading "/" is
// required and "//" is refused because browsers read it as a protocol-relative
// external URL; "javascript:" therefore never reaches href/navigate.
func validateMenuPath(p string) error {
	if p == "" || len(p) > MaxMenuPathLen || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") {
		return newErr(http.StatusBadRequest, "path must start with a single /")
	}
	if strings.ContainsAny(p, " \t\r\n") {
		return newErr(http.StatusBadRequest, "path cannot contain whitespace")
	}
	return nil
}

// CreateMenuItem appends an operator link to the end of the sidebar.
func (s *Service) CreateMenuItem(ctx context.Context, in MenuItemInput) (models.MenuItem, error) {
	if err := validateMenuInput(in, true); err != nil {
		return models.MenuItem{}, err
	}
	visible := true
	if in.Visible != nil {
		visible = *in.Visible
	}
	var maxSort int
	_ = s.db.WithContext(ctx).Model(&models.MenuItem{}).Select("COALESCE(MAX(sort), 0)").Scan(&maxSort).Error
	row := models.MenuItem{
		Label: strings.TrimSpace(in.Label), Path: in.Path,
		GroupKey: in.GroupKey, Visible: visible, Sort: maxSort + 10,
	}
	if err := s.db.WithContext(ctx).Create(&row).Error; err != nil {
		return models.MenuItem{}, newErr(http.StatusInternalServerError, "failed to create menu item")
	}
	s.audit(ctx, models.User{}, "create_menu", row.Label, row.Path)
	return row, nil
}

// UpdateMenuItem edits a row. Built-in rows accept label and visibility only;
// custom rows also accept path and group.
func (s *Service) UpdateMenuItem(ctx context.Context, id int64, in MenuItemInput) (models.MenuItem, error) {
	var row models.MenuItem
	if err := s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return models.MenuItem{}, newErr(http.StatusNotFound, "menu item not found")
	}
	custom := row.Key == ""
	if custom {
		if err := validateMenuInput(in, true); err != nil {
			return models.MenuItem{}, err
		}
		row.Label = strings.TrimSpace(in.Label)
		row.Path = in.Path
		row.GroupKey = in.GroupKey
	} else {
		label := strings.TrimSpace(in.Label)
		if len(label) > MaxMenuLabelLen {
			return models.MenuItem{}, newErr(http.StatusBadRequest, "label must be 1-64 characters")
		}
		// Empty means "fall back to the i18n label", which is how an operator
		// undoes a translation override.
		row.Label = label
	}
	if in.Visible != nil {
		row.Visible = *in.Visible
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return models.MenuItem{}, newErr(http.StatusInternalServerError, "failed to update menu item")
	}
	s.audit(ctx, models.User{}, "update_menu", row.Label, row.Path)
	return row, nil
}

// DeleteMenuItem removes an operator link. Built-ins can only be hidden: the
// route and its role gate still exist, and deleting the row would erase the
// only way to bring the link back.
func (s *Service) DeleteMenuItem(ctx context.Context, id int64) error {
	var row models.MenuItem
	if err := s.db.WithContext(ctx).First(&row, id).Error; err != nil {
		return newErr(http.StatusNotFound, "menu item not found")
	}
	if row.Key != "" {
		return newErr(http.StatusBadRequest, "built-in menu items can only be hidden")
	}
	if err := s.db.WithContext(ctx).Delete(&row).Error; err != nil {
		return newErr(http.StatusInternalServerError, "failed to delete menu item")
	}
	s.audit(ctx, models.User{}, "delete_menu", row.Label, row.Path)
	return nil
}

// ReorderMenuItems rewrites sort to the request order. The client sends every
// row, so a short list is rejected rather than silently dropping entries.
func (s *Service) ReorderMenuItems(ctx context.Context, in MenuReorderInput) ([]models.MenuItem, error) {
	var rows []models.MenuItem
	if err := s.db.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to load menu")
	}
	if len(rows) == 0 || len(in.IDs) != len(rows) {
		return nil, newErr(http.StatusBadRequest, "the order must list every menu item exactly once")
	}
	seen := make(map[int64]bool, len(in.IDs))
	for _, id := range in.IDs {
		if seen[id] {
			return nil, newErr(http.StatusBadRequest, "duplicate menu item in order")
		}
		seen[id] = true
	}
	for _, row := range rows {
		if !seen[row.ID] {
			return nil, newErr(http.StatusBadRequest, "the order is missing a menu item")
		}
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range in.IDs {
			if err := tx.Model(&models.MenuItem{}).Where("id = ?", id).Update("sort", i*10).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, newErr(http.StatusInternalServerError, "failed to reorder menu")
	}
	s.audit(ctx, models.User{}, "reorder_menus", "", "")
	return s.ListMenuItems(ctx)
}
