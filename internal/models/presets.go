package models

// PresetCategory is one node of the shipped category tree (spec 27). It lives
// in models — not in services or config — because BOTH the fresh-database
// seeder (internal/config) and the "apply preset" endpoint (internal/services)
// need it, and duplicating the tree in two places would guarantee drift.
type PresetCategory struct {
	Name     string
	Children []string
}

// PresetCategories is the default two-level structure for a personal tech
// blog. Names are Chinese to match the existing content; the apply endpoint is
// idempotent, so running it on a database that already has some of these
// categories only fills in what is missing (and re-parents orphans by name).
var PresetCategories = []PresetCategory{
	{Name: "技术笔记", Children: []string{"后端开发", "前端开发", "数据库", "运维部署"}},
	{Name: "系统与网络", Children: []string{"Windows", "Linux", "macOS"}},
	{Name: "生活随笔", Children: []string{"折腾记录", "读书笔记"}},
}
