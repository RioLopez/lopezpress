package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/models"
)

// buildOldSchema creates a DB with the legacy posts table (with scheduled_at)
// plus a few rows, to simulate a database created before spec #8.
func buildOldSchema(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stmts := []string{
		`CREATE TABLE posts (
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
			scheduled_at DATETIME,
			deleted_at DATETIME
		)`,
		`CREATE INDEX idx_posts_author_id ON posts (author_id)`,
		`INSERT INTO posts (id, title, body, status, author_id, view_count, scheduled_at) VALUES
		 ('a','t1','b1','published','auth',5,'2020-01-01 00:00:00'),
		 ('b','t2','b2','private','auth',0,NULL)`,
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			t.Fatalf("schema step: %v\n%s", err, s)
		}
	}
	return db
}

// TestDropScheduledAt verifies the drop migration's "up" function drops the
// column and preserves data, and is a no-op when the column is already gone
// (the gate/record concern lives in apply, not in the up function).
func TestDropScheduledAt(t *testing.T) {
	// Legacy DB: has scheduled_at.
	path := t.TempDir() + "/legacy.db"
	db := buildOldSchema(t, path)

	if err := dropScheduledAt(db); err != nil {
		t.Fatalf("drop on legacy: %v", err)
	}
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('posts') WHERE name = 'scheduled_at'").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("scheduled_at column still present after migration")
	}
	var cnt int64
	db.Model(&models.Post{}).Count(&cnt)
	if cnt != 2 {
		t.Fatalf("expected 2 posts preserved, got %d", cnt)
	}
	// Idempotent: a second call sees no column and is a no-op.
	if err := dropScheduledAt(db); err != nil {
		t.Fatalf("second run on legacy: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	// Fresh DB: no scheduled_at column at all (model no longer declares it).
	freshPath := t.TempDir() + "/fresh.db"
	fdb, err := gorm.Open(sqlite.Open(freshPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	if err := fdb.Exec(`CREATE TABLE posts (
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
		)`).Error; err != nil {
		t.Fatalf("create fresh posts: %v", err)
	}
	if err := dropScheduledAt(fdb); err != nil {
		t.Fatalf("drop on fresh (no column): %v", err)
	}
	if fsqlDB, err := fdb.DB(); err == nil {
		_ = fsqlDB.Close()
	}
}

// TestApplyMigrationGate verifies the centralized gate: a migration runs once,
// records its version, and is skipped on subsequent runs.
func TestApplyMigrationGate(t *testing.T) {
	path := t.TempDir() + "/gate.db"
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Migrator().CreateTable(&models.SchemaMigration{}); err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}

	ran := 0
	m := migration{
		version: "9999_test",
		up: func(db *gorm.DB) error {
			ran++
			return nil
		},
	}

	if err := apply(db, m); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if ran != 1 {
		t.Fatalf("expected migration to run once, got %d", ran)
	}
	if err := apply(db, m); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if ran != 1 {
		t.Fatalf("expected migration skipped on second run, ran %d times", ran)
	}
	var rec int64
	db.Model(&models.SchemaMigration{}).Where("version = ?", "9999_test").Count(&rec)
	if rec != 1 {
		t.Fatalf("expected gate row recorded once, got %d", rec)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// TestRunManualMigrationsFresh verifies the full registry runs cleanly on a
// brand-new database (no tables yet): every migration is a no-op, and every
// version is recorded so a later AutoMigrate owns the actual table creation.
func TestRunManualMigrationsFresh(t *testing.T) {
	path := t.TempDir() + "/fresh.db"
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := runManualMigrations(db); err != nil {
		t.Fatalf("runManualMigrations on fresh DB: %v", err)
	}
	var rec int64
	db.Model(&models.SchemaMigration{}).Count(&rec)
	if rec != int64(len(migrations)) {
		t.Fatalf("expected %d migration gates recorded, got %d", len(migrations), rec)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// TestEnsureColumns verifies the generic add-column helper that every column
// migration now routes through: it adds a missing column, skips an existing
// one, runs the idempotent `after` SQL, and is a no-op when the table does not
// exist yet (AutoMigrate owns the full table on a fresh database).
func TestEnsureColumns(t *testing.T) {
	path := t.TempDir() + "/cols.db"
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// No table yet: ensureColumns must be a no-op, not an error.
	up := ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "page_size", ddl: "ALTER TABLE site_configs ADD COLUMN page_size INTEGER DEFAULT 10"},
	})
	if err := up(db); err != nil {
		t.Fatalf("ensureColumns on missing table: %v", err)
	}

	// Create the table without the column, then ensureColumns adds it + backfill.
	if err := db.Exec(`CREATE TABLE site_configs (id INTEGER PRIMARY KEY)`).Error; err != nil {
		t.Fatalf("create site_configs: %v", err)
	}
	up = ensureColumns(&models.SiteConfig{}, []columnSpec{
		{name: "page_size", ddl: "ALTER TABLE site_configs ADD COLUMN page_size INTEGER DEFAULT 10"},
	}, "UPDATE site_configs SET page_size = 10 WHERE page_size IS NULL OR page_size <= 0")
	if err := up(db); err != nil {
		t.Fatalf("ensureColumns add: %v", err)
	}
	var has int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('site_configs') WHERE name = 'page_size'").Scan(&has).Error; err != nil {
		t.Fatal(err)
	}
	if has != 1 {
		t.Fatalf("page_size column not added")
	}
	// Idempotent: a second run must not error (column already present).
	if err := up(db); err != nil {
		t.Fatalf("ensureColumns second run: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}

// R6-02: the unique index on posts.slug cannot be created while duplicates
// exist, so migration 0014 has to disambiguate them first. This test feeds it
// the exact shape the pentest produced — four posts sharing one slug — and
// checks that the oldest keeps the slug and the rest get -2/-3/... suffixes,
// which is what the (previously racy) allocator would have chosen.
func TestDisambiguateDuplicateSlugs(t *testing.T) {
	// A file-backed DB so the partial unique index and the raw SQL both behave
	// exactly as they do against a real database file.
	path := filepath.Join(t.TempDir(), "mig.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(models.All()...); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mk := func(id, slug string, created time.Time) models.Post {
		p := models.Post{
			ID: id, AuthorID: "author-1", Title: "T " + id, Body: "b",
			Status: models.PostPublished, Slug: slug, CreatedAt: created,
		}
		if err := db.Create(&p).Error; err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		return p
	}
	base := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	mk("p1", "same-slug", base)
	mk("p2", "same-slug", base.Add(time.Minute))
	mk("p3", "same-slug", base.Add(2*time.Minute))
	mk("p4", "same-slug", base.Add(3*time.Minute))
	// An already-taken suffix the allocator must step over rather than collide
	// with, plus an empty slug that must be left alone.
	mk("p5", "same-slug-2", base.Add(4*time.Minute))
	mk("p6", "", base.Add(5*time.Minute))

	if err := disambiguateDuplicateSlugs(db); err != nil {
		t.Fatalf("disambiguate: %v", err)
	}

	got := map[string]string{}
	var rows []models.Post
	if err := db.Find(&rows).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	for _, r := range rows {
		got[r.ID] = r.Slug
	}
	want := map[string]string{
		"p1": "same-slug",     // oldest keeps it
		"p2": "same-slug-3",   // -2 was taken by p5, so the next free one
		"p3": "same-slug-4",
		"p4": "same-slug-5",
		"p5": "same-slug-2",   // untouched: it was never part of the group
		"p6": "",              // empty means "no slug yet", not a collision
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("post %s: slug = %q, want %q", id, got[id], w)
		}
	}

	// The unique index must now be buildable over the result.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_slug_unique ON posts(slug) WHERE slug <> ''`).Error; err != nil {
		t.Fatalf("unique index should be creatable after disambiguation: %v", err)
	}
}
