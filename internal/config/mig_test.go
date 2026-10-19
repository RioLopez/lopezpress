package config

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"echo-content-api/internal/models"
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

// TestDropScheduledAt verifies the hand-written migration drops the column,
// preserves data, and is idempotent on re-run (and a no-op on fresh DBs).
func TestDropScheduledAt(t *testing.T) {
	// Legacy DB: has scheduled_at.
	path := t.TempDir() + "/legacy.db"
	db := buildOldSchema(t, path)
	_ = db.Migrator().CreateTable(&models.SchemaMigration{})

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
	// Idempotent: a second run must be a no-op (gate row already recorded).
	if err := dropScheduledAt(db); err != nil {
		t.Fatalf("second run on legacy: %v", err)
	}
	var cnt2 int64
	db.Model(&models.Post{}).Count(&cnt2)
	if cnt2 != 2 {
		t.Fatalf("data lost on re-run: got %d", cnt2)
	}
	// Close the SQLite handle so the temp dir can be cleaned up on Windows.
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	// Fresh DB: no scheduled_at column at all (model no longer declares it).
	freshPath := t.TempDir() + "/fresh.db"
	fdb, err := gorm.Open(sqlite.Open(freshPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	// Manually create a posts table WITHOUT scheduled_at to mimic a brand-new
	// install where the column was never added.
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
	_ = fdb.Migrator().CreateTable(&models.SchemaMigration{})
	if err := dropScheduledAt(fdb); err != nil {
		t.Fatalf("drop on fresh (no column): %v", err)
	}
	// Gate row should still be recorded so the version is consistent.
	var rec int64
	fdb.Model(&models.SchemaMigration{}).Where("version = ?", "0001_drop_scheduled_at").Count(&rec)
	if rec != 1 {
		t.Fatalf("expected migration gate recorded for fresh DB, got %d", rec)
	}
	// Close the fresh SQLite handle too.
	if fsqlDB, err := fdb.DB(); err == nil {
		_ = fsqlDB.Close()
	}
}
