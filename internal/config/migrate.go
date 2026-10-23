package config

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"lopezpress/internal/models"
)

// migration is one hand-written, one-shot schema or data change. AutoMigrate
// is deliberately never run against an existing table on this driver (the
// glebarez copy-rename path drops NOT NULL constraints — the classic
// "posts__temp.title constraint failed" crash), so column-level changes are
// owned here, gated by a row in schema_migrations so each runs exactly once.
type migration struct {
	version string
	up      func(db *gorm.DB) error
}

// migrations is the single, ordered registry of every hand-written migration.
// Order matters: later migrations may depend on tables or columns created by
// earlier ones. To add a column, append one entry using ensureColumns — do not
// copy a new addXxxColumn function, which is exactly the clone class this
// registry removes.
var migrations = []migration{
	// 0001: drop the deprecated posts.scheduled_at (spec 8).
	{version: "0001_drop_scheduled_at", up: dropScheduledAt},
	// 0002: SMTP settings for security notifications.
	{version: "0002_add_smtp_columns", up: ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "smtp_host", ddl: "ALTER TABLE site_configs ADD COLUMN smtp_host VARCHAR(128)"},
		{name: "smtp_port", ddl: "ALTER TABLE site_configs ADD COLUMN smtp_port INTEGER"},
		{name: "smtp_user", ddl: "ALTER TABLE site_configs ADD COLUMN smtp_user VARCHAR(128)"},
		{name: "smtp_pass", ddl: "ALTER TABLE site_configs ADD COLUMN smtp_pass TEXT"},
	})},
	// 0003: introduce the three-tier role model (single root).
	{version: "0003_root_migration", up: promoteFirstAdminToRoot},
	// 0004: content-moderation review metadata on posts.
	{version: "0004_add_review_columns", up: ensureColumns(&models.Post{}, []columnSpec{
		{name: "review_note", ddl: "ALTER TABLE posts ADD COLUMN review_note TEXT"},
		{name: "reviewed_by", ddl: "ALTER TABLE posts ADD COLUMN reviewed_by VARCHAR(36)"},
		{name: "reviewed_at", ddl: "ALTER TABLE posts ADD COLUMN reviewed_at DATETIME"},
	})},
	// 0005: site_configs.page_size (posts per page on the public list).
	{version: "0005_add_page_size", up: ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "page_size", ddl: "ALTER TABLE site_configs ADD COLUMN page_size INTEGER DEFAULT 10"},
	}, "UPDATE site_configs SET page_size = 10 WHERE page_size IS NULL OR page_size <= 0")},
	// 0006: site_configs.public_read (anonymous reading).
	{version: "0006_add_public_read", up: ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "public_read", ddl: "ALTER TABLE site_configs ADD COLUMN public_read INTEGER DEFAULT 1"},
	}, "UPDATE site_configs SET public_read = 1 WHERE public_read IS NULL")},
	// 0007: categories.parent_id (two-level tree).
	{version: "0007_add_category_parent", up: ensureColumns(&models.Category{}, []columnSpec{
		{name: "parent_id", ddl: "ALTER TABLE categories ADD COLUMN parent_id INTEGER"},
	}, "CREATE INDEX IF NOT EXISTS idx_categories_parent_id ON categories(parent_id)")},
	// 0008: site_configs.site_url (found live in round 46: PUT answered 500
	// because the column was missing on databases seeded before SiteURL).
	{version: "0008_add_site_url", up: ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "site_url", ddl: "ALTER TABLE site_configs ADD COLUMN site_url TEXT"},
	})},
	// 0009: posts.slug (human-readable permanent link, spec 29).
	{version: "0009_add_post_slug", up: ensureColumns(&models.Post{}, []columnSpec{
		{name: "slug", ddl: "ALTER TABLE posts ADD COLUMN slug TEXT"},
	}, "CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug)")},
	// 0010: daily snapshot settings (spec 34).
	{version: "0010_add_auto_backup", up: ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "auto_backup", ddl: "ALTER TABLE site_configs ADD COLUMN auto_backup INTEGER DEFAULT 0"},
		{name: "backup_keep", ddl: "ALTER TABLE site_configs ADD COLUMN backup_keep INTEGER DEFAULT 7"},
	}, "UPDATE site_configs SET backup_keep = 7 WHERE backup_keep IS NULL OR backup_keep <= 0")},
	// 0011: users.token_version ("log out all devices"). The field existed on
	// the model since v0.1.0 but no migration was ever written for it, so any
	// database created before the column appeared 500'd on login/logout. The
	// drift check flagged it; this entry is the fix.
	{version: "0011_add_token_version", up: ensureColumns(&models.User{}, []columnSpec{
		{name: "token_version", ddl: "ALTER TABLE users ADD COLUMN token_version INTEGER DEFAULT 0"},
	})},
}

// runManualMigrations applies every registered migration in order. Each is
// gated by a row in schema_migrations so it runs exactly once, and the whole
// pass runs before AutoMigrate (which then only ever creates tables that do
// not exist yet).
func runManualMigrations(db *gorm.DB) error {
	if exists := db.Migrator().HasTable(&models.SchemaMigration{}); !exists {
		if err := db.Migrator().CreateTable(&models.SchemaMigration{}); err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}
	}
	for _, m := range migrations {
		if err := apply(db, m); err != nil {
			return err
		}
	}
	return nil
}

// apply runs one migration unless its version is already recorded, then
// records the version. The gate check is centralized here — it used to be
// copy-pasted into every migration function.
func apply(db *gorm.DB, m migration) error {
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", m.version).Count(&count).Error; err != nil {
		return fmt.Errorf("check %s migration gate: %w", m.version, err)
	}
	if count > 0 {
		return nil // already applied
	}
	if err := m.up(db); err != nil {
		return fmt.Errorf("migration %s: %w", m.version, err)
	}
	return recordMigration(db, m.version)
}

func recordMigration(db *gorm.DB, version string) error {
	return db.Create(&models.SchemaMigration{Version: version, AppliedAt: time.Now()}).Error
}

// columnSpec pairs a column name (for the existence probe) with the full
// ALTER TABLE ... ADD COLUMN statement.
type columnSpec struct {
	name string
	ddl  string
}

// ensureColumns builds an "up" function that adds the given columns to an
// existing table, then runs the idempotent `after` SQL (indexes / backfills).
// On a fresh database (table not yet created) it is a no-op: AutoMigrate
// creates the full table afterwards.
func ensureColumns(model interface{}, cols []columnSpec, after ...string) func(*gorm.DB) error {
	return func(db *gorm.DB) error {
		if !db.Migrator().HasTable(model) {
			return nil
		}
		for _, c := range cols {
			if db.Migrator().HasColumn(model, c.name) {
				continue
			}
			if err := db.Exec(c.ddl).Error; err != nil {
				return fmt.Errorf("add %s: %w", c.name, err)
			}
		}
		for _, s := range after {
			if err := db.Exec(s).Error; err != nil {
				return fmt.Errorf("post-migration: %w", err)
			}
		}
		return nil
	}
}

// dropScheduledAt removes posts.scheduled_at on databases that still carry it.
func dropScheduledAt(db *gorm.DB) error {
	// Probe whether the column actually exists (fresh databases created after
	// the model no longer declares it will not).
	var colCount int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('posts') WHERE name = 'scheduled_at'").Scan(&colCount).Error; err != nil {
		return fmt.Errorf("probe scheduled_at: %w", err)
	}
	if colCount == 0 {
		return nil // nothing to drop
	}

	// Copy → drop → rename (SQLite-inplace is not universally available).
	steps := []string{
		`CREATE TABLE posts_new (
			id TEXT PRIMARY KEY,
			title VARCHAR(200) NOT NULL,
			body TEXT,
			status VARCHAR(16) DEFAULT 'published',
			author_id VARCHAR(36),
			category_id INTEGER,
			pinned BOOLEAN DEFAULT 0,
			cover VARCHAR(512),
			view_count INTEGER DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME,
			deleted_at DATETIME
		)`,
		`INSERT INTO posts_new (id, title, body, status, author_id, category_id, pinned, cover, view_count, created_at, updated_at, deleted_at)
		 SELECT id, title, body, status, author_id, category_id, pinned, cover, view_count, created_at, updated_at, deleted_at FROM posts`,
		`DROP TABLE posts`,
		`ALTER TABLE posts_new RENAME TO posts`,
		// Re-create indexes that GORM auto-created on the original table.
		`CREATE INDEX idx_posts_author_id ON posts (author_id)`,
		`CREATE INDEX idx_posts_deleted_at ON posts (deleted_at)`,
	}
	for i, stmt := range steps {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("step %d: %w", i+1, err)
		}
	}
	return nil
}

// promoteFirstAdminToRoot introduces the three-tier role model. On databases
// created before the root role existed, the oldest "admin" account becomes the
// single "root"; on brand-new databases this is a no-op (the seeder already
// created root directly). It also enforces the invariant that there is at most
// one root in the whole table: if several roots somehow coexist, every root
// except the oldest is demoted to admin so the single-root guarantee holds.
func promoteFirstAdminToRoot(db *gorm.DB) error {
	// Fresh databases do not have the users table yet (AutoMigrate runs after
	// runManualMigrations). Nothing to promote.
	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableCount).Error; err != nil {
		return fmt.Errorf("probe users table: %w", err)
	}
	if tableCount == 0 {
		return nil
	}

	// If a root already exists, we only need to enforce uniqueness.
	var roots []models.User
	if err := db.Model(&models.User{}).Where("role = ?", models.RoleRoot).
		Order("created_at ASC, id ASC").Find(&roots).Error; err != nil {
		return fmt.Errorf("list roots: %w", err)
	}
	if len(roots) >= 1 {
		// Demote any extra roots (keep the oldest) so the single-root invariant
		// holds even on a manually-corrupted database.
		for i := 1; i < len(roots); i++ {
			if err := db.Model(&models.User{}).Where("id = ?", roots[i].ID).
				Update("role", models.RoleAdmin).Error; err != nil {
				return fmt.Errorf("demote extra root: %w", err)
			}
		}
		return nil
	}

	// No root exists yet: promote the oldest admin. "created_at, id ASC" picks
	// the account that was provisioned first — the de-facto founder of the site.
	var admin models.User
	err := db.Where("role = ?", models.RoleAdmin).
		Order("created_at ASC, id ASC").First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No admin either (empty table on a fresh DB that AutoMigrate will seed
		// right after). The seeder will create root directly.
		return nil
	}
	if err != nil {
		return fmt.Errorf("find oldest admin: %w", err)
	}
	if err := db.Model(&models.User{}).Where("id = ?", admin.ID).
		Update("role", models.RoleRoot).Error; err != nil {
		return fmt.Errorf("promote to root: %w", err)
	}
	return nil
}
