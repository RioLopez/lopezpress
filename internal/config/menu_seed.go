package config

import (
	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// menuSeeds is the shipped admin sidebar, moved out of AdminLayout.tsx in B5.
// Path, group, role gate and order are unchanged; only the storage location is
// new, so an upgraded install looks exactly like the previous release until
// somebody edits the menu.
var menuSeeds = []models.MenuItem{
	{Key: "dashboard", LabelKey: "nav.dashboard", Path: "/admin", Sort: 0},
	{Key: "posts", LabelKey: "admin.postsTitle", Path: "/admin/posts", GroupKey: "content", Sort: 10},
	{Key: "review", LabelKey: "admin.reviewTitle", Path: "/admin/review", GroupKey: "content", Sort: 20},
	{Key: "categories", LabelKey: "admin.categoriesTitle", Path: "/admin/categories", GroupKey: "content", Sort: 30},
	{Key: "tags", LabelKey: "admin.tagsTitle", Path: "/admin/tags", GroupKey: "content", Sort: 40},
	{Key: "trash", LabelKey: "admin.trashTitle", Path: "/admin/trash", GroupKey: "content", Sort: 50},
	{Key: "banned", LabelKey: "admin.banVaultTitle", Path: "/admin/banned", GroupKey: "content", RootOnly: true, Sort: 60},
	{Key: "media", LabelKey: "media.title", Path: "/admin/media", GroupKey: "content", Sort: 70},
	{Key: "users", LabelKey: "admin.usersTitle", Path: "/admin/users", GroupKey: "users", Sort: 80},
	{Key: "roles", LabelKey: "admin.rolesTitle", Path: "/admin/roles", GroupKey: "users", Sort: 90},
	{Key: "site", LabelKey: "site.title", Path: "/admin/site", GroupKey: "system", Sort: 100},
	{Key: "menus", LabelKey: "menuAdmin.title", Path: "/admin/menus", GroupKey: "system", RootOnly: true, Sort: 110},
	{Key: "backup", LabelKey: "admin.backupTitle", Path: "/admin/backup", GroupKey: "system", Sort: 120},
	{Key: "audit", LabelKey: "audit.title", Path: "/admin/audit", GroupKey: "system", Sort: 130},
}

// seedMenuItems inserts the built-in rows once. An existing install keeps
// whatever the operator has reordered, hidden or added; a fresh install gets
// the same sidebar the hardcoded array used to produce.
func seedMenuItems(db *gorm.DB) {
	var n int64
	if err := db.Model(&models.MenuItem{}).Count(&n).Error; err != nil || n > 0 {
		return
	}
	for _, item := range menuSeeds {
		row := item
		row.Visible = true
		_ = db.Create(&row).Error
	}
}
