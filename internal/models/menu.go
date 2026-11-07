package models

import "time"

// MenuItem is one row of the admin sidebar navigation (B5). Before B5 the
// sidebar was a hardcoded array in AdminLayout.tsx, so an operator could not
// reorder it or add a link to a tool of their own.
//
// Built-in rows carry a stable Key and the i18n LabelKey they have always used.
// Custom rows have Key == "" and a literal Label. Deleting a built-in row is
// refused (it can only be hidden) so the sidebar cannot lose an entry that the
// rest of the UI — breadcrumbs, role gating — still expects to exist.
type MenuItem struct {
	ID int64 `gorm:"primaryKey" json:"id"`
	// Key identifies a built-in entry ("dashboard", "posts", …). Empty for
	// operator-created links.
	Key string `gorm:"size:64;index" json:"key"`
	// LabelKey is the i18n key used when Label is empty, e.g. "admin.postsTitle".
	LabelKey string `gorm:"size:64" json:"label_key"`
	// Label overrides the i18n value (built-ins) or is the whole label (custom).
	Label string `gorm:"size:64" json:"label"`
	// Path is a site-relative URL ("/admin/posts"). External links are not
	// accepted: the sidebar is for this installation's pages, and a stored
	// "javascript:" value is exactly the kind of foot-gun this table avoids.
	Path string `gorm:"size:256" json:"path"`
	// GroupKey is "content", "users", "system" or "" (ungrouped). Custom rows
	// may choose any of them; built-in grouping is fixed by the seed.
	GroupKey string `gorm:"size:16" json:"group_key"`
	// RootOnly keeps the built-in role gate (the ban vault). Custom rows are
	// never root-only: the person creating them is root, and hiding a custom
	// link from admins would have no purpose.
	RootOnly  bool      `gorm:"not null;default:false" json:"root_only"`
	Visible   bool      `gorm:"not null;default:true" json:"visible"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
