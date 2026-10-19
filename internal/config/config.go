package config

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"echo-content-api/internal/models"
)

// Config holds runtime configuration loaded from .env (or defaults).
//
// Budget notes for the target VPS (1 vCPU / 1.5 GB RAM, hard 70% cap -> ~1.0 GB
// usable, 5 GB disk -> ~3.5 GB usable):
//   - GORM pool is intentionally small (MaxOpen 4 / MaxIdle 2). SQLite is a
//     single-file engine; large pools just waste goroutine/memory on a 1 GB box.
//   - GORM logger defaults to Warn level (not Info) to keep the log/IO volume
//     low on a low-spec machine.
//   - Rate limiting uses an in-memory token bucket per identity; no Redis.
type Config struct {
	Port             string
	JWTSecret        string
	JWTExpiryHours   int
	DBPath           string
	DBMaxOpen        int
	DBMaxIdle        int
	LogLevel         string
	RateLimitRPS     float64
	RateLimitBurst   int
	UploadDir        string // where uploaded images are stored on disk
	UploadMaxBytes   int64 // per-file upload size cap
	BackupDir        string // where JSON backup dumps are written
	AdminInitialPassword string // optional; if set, used as admin's initial password on first boot
	CORSOrigins      string // comma-separated allowed origins; "*" allows all (dev default)
	SmmsToken        string // SM.MS image-host token held server-side (spec #7); optional
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port:           envOr("PORT", "8080"),
		JWTSecret:      envOr("JWT_SECRET", "dev-secret-change-me-in-prod"),
		JWTExpiryHours: envIntOr("JWT_EXPIRY_HOURS", 1),
		DBPath:         envOr("DB_PATH", "data/app.db"),
		// Small pool on purpose (see budget notes above).
		DBMaxOpen:  envIntOr("DB_MAX_OPEN", 4),
		DBMaxIdle:  envIntOr("DB_MAX_IDLE", 2),
		LogLevel:   envOr("LOG_LEVEL", "warn"),
		// General per-identity rate limit. Kept generous for the common
		// case: a single signed-in admin browsing the console fires a burst
		// of parallel requests on page mount (session + each tab + review
		// queue) that easily exceeds a tight 5 rps / 10 burst and surfaces
		// as 429 on the next screen load. Brute-force protection still
		// lives in the dedicated login limiter, so loosening the general
		// bucket does not open a bypass.
		RateLimitRPS:   envFloatOr("RATE_LIMIT_RPS", 20),
		RateLimitBurst: envIntOr("RATE_LIMIT_BURST", 50),
		// Upload directory sits alongside the DB by default; override in prod.
		UploadDir:      envOr("UPLOAD_DIR", "data/uploads"),
		UploadMaxBytes: int64(envIntOr("UPLOAD_MAX_BYTES", 2*1024*1024)), // 2 MB per image
		// Backup dumps live under data/backups by default; override in prod.
		BackupDir: envOr("BACKUP_DIR", "data/backups"),
		// If ADMIN_INITIAL_PASSWORD is set, admin's initial password uses it;
		// otherwise main.go generates a random one.
		AdminInitialPassword: os.Getenv("ADMIN_INITIAL_PASSWORD"),
		// CORS origins whitelist. Default "*" (dev); in prod, set to your real
		// frontend origin(s) comma-separated. Never pair "*" with credentials.
		CORSOrigins: envOr("CORS_ORIGINS", "*"),
		// SM.MS image-host token held server-side (spec #7). The browser no
		// longer carries this secret; /api/upload proxies to SM.MS when it
		// is set. Falls back to the site-config token when empty.
		SmmsToken: os.Getenv("SM_MS_TOKEN"),
	}
}

// ValidateSecurity enforces security-sensitive configuration.
//
// Rules:
//   - APP_ENV of "dev" (or unset): warning only, never blocks.
//   - APP_ENV of "prod"/"production" (or any unknown value): treated as prod,
//     and JWT_SECRET must be non-empty, not a known default, at least 32
//     bytes, and not a repeated single character.
func (c *Config) ValidateSecurity() error {
	appEnv := envOr("APP_ENV", "dev")
	strict := appEnv != "dev" // prod / production / anything else -> prod

	if strict {
		if err := validateJWTSecret(c.JWTSecret); err != nil {
			return fmt.Errorf("APP_ENV=%s: %w", appEnv, err)
		}
		if strings.TrimSpace(c.CORSOrigins) == "*" {
			return fmt.Errorf("APP_ENV=%s: CORS_ORIGINS must be an explicit origin list in production (not \"*\")", appEnv)
		}
	} else {
		if c.JWTSecret == "" || isWeakSecret(c.JWTSecret) {
			fmt.Fprintf(os.Stderr, "WARN: dev mode with weak/empty JWT_SECRET; set a strong random secret for production\n")
		}
		if strings.TrimSpace(c.CORSOrigins) == "*" {
			fmt.Fprintf(os.Stderr, "WARN: dev mode with CORS_ORIGINS=*; set an explicit origin list for production\n")
		}
	}
	return nil
}

func isKnownDefaultSecret(s string) bool {
	switch s {
	case "secret", "changeme", "echo-secret", "your-secret-key", "dev-secret-change-me-in-prod", "":
		return true
	}
	return false
}

// isRepeatingChar reports whether s is a single character repeated.
func isRepeatingChar(s string) bool {
	if len(s) < 2 {
		return false
	}
	for i := 1; i < len(s); i++ {
		if s[i] != s[0] {
			return false
		}
	}
	return true
}

func isWeakSecret(s string) bool {
	return isKnownDefaultSecret(s) || isRepeatingChar(s) || len(s) < 32
}

func validateJWTSecret(s string) error {
	switch {
	case s == "":
		return fmt.Errorf("JWT_SECRET must not be empty in production")
	case isKnownDefaultSecret(s):
		return fmt.Errorf("JWT_SECRET must not be a known default value")
	case isRepeatingChar(s):
		return fmt.Errorf("JWT_SECRET must not be a repeated single character")
	case len(s) < 32:
		return fmt.Errorf("JWT_SECRET must be at least 32 bytes (got %d)", len(s))
	}
	return nil
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// envFloatOr reads a float env var, falling back to def.
func envFloatOr(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// gormLogMode maps LOG_LEVEL to a gorm logger level so the GORM SQL logger
// stays quiet on a low-spec box by default.
func gormLogMode(level string) logger.LogLevel {
	switch level {
	case "debug":
		return logger.Info
	case "info":
		return logger.Warn
	default: // "warn", "error", ""
		return logger.Error
	}
}

// quietLogger wraps a gorm logger and swallows "record not found" errors so
// routine queries that legitimately return zero rows (first-boot seeding,
// soft-delete "not found", anonymous browse of private posts, etc.) don't
// flood the log with red warnings on every request.
type quietLogger struct {
	logger.Interface
}

func (l quietLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	l.Interface.Trace(ctx, begin, fc, err)
}

// dirOf returns the parent directory of a file path.
func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}

// InitDB opens the SQLite database, runs schema migrations and seeds demo
// data. It returns the database and the seeded admin password (empty if an
// admin already existed or seeding failed).
func InitDB(cfg *Config) (*gorm.DB, string, error) {
	if err := os.MkdirAll(dirOf(cfg.DBPath), 0o755); err != nil {
		return nil, "", fmt.Errorf("create db dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: quietLogger{logger.Default.LogMode(gormLogMode(cfg.LogLevel))},
	})
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite: %w", err)
	}

	// Tight connection pool for a low-memory VPS (1.5 GB, ~1 GB usable).
	// SQLite is single-file; a large pool only wastes memory on goroutines.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, "", fmt.Errorf("sql handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpen)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdle)

	// One-shot manual migrations run BEFORE AutoMigrate. AutoMigrate only
	// adds missing columns/tables and never drops; dropping the deprecated
	// posts.scheduled_at column (spec #8) requires the hand-written
	// migration below, gated by the schema_migrations table.
	if err := runManualMigrations(db); err != nil {
		return nil, "", fmt.Errorf("manual migration: %w", err)
	}

	// Only AutoMigrate tables that do not yet exist. On an already-migrated
	// database GORM's copy-rename rebuild path is unreliable on the glebarez
	// driver: it drops NOT NULL constraints on rebuilt tables (e.g. the
	// classic "posts__temp.title constraint failed" crash), so we must never
	// let AutoMigrate touch a table that already exists. The hand-written
	// migrations in runManualMigrations own column-level schema changes for
	// pre-existing tables; AutoMigrate here is strictly for creating
	// brand-new tables on a fresh database.
	allModels := []interface{}{
		&models.User{}, &models.Post{}, &models.Category{},
		&models.Tag{}, &models.PostTag{}, &models.SiteConfig{},
		&models.LoginLog{}, &models.ActionLog{}, &models.SchemaMigration{},
	}
	existing := map[string]bool{}
	if err := collectExistingTables(db, existing); err != nil {
		return nil, "", fmt.Errorf("collect existing tables: %w", err)
	}
	missing := make([]interface{}, 0, len(allModels))
	for _, m := range allModels {
		if !existing[commonTableName(m)] {
			missing = append(missing, m)
		}
	}
	if len(missing) > 0 {
		if err := db.AutoMigrate(missing...); err != nil {
			return nil, "", fmt.Errorf("migrate: %w", err)
		}
	}

	seededPwd := seedDemoData(db, cfg.AdminInitialPassword)
	seedSiteConfig(db)
	return db, seededPwd, nil
}

// seedSiteConfig ensures the single-row SiteConfig (id=1) exists with safe
// defaults. Public registration is OFF by default (single-user site); the
// admin can enable it from the site-settings page if multi-user is wanted.
func seedSiteConfig(db *gorm.DB) {
	var cfg models.SiteConfig
	if db.Where("id = ?", 1).First(&cfg).Error == nil {
		return // already seeded
	}
	cfg = models.SiteConfig{
		ID:           1,
		SiteName:     "Echo",
		SiteTitle:    "Echo",
		ImgHostMode:  "local",
		AllowRegister: false,
	}
	_ = db.Create(&cfg).Error
}

// seedDemoData creates a demo author and a few sample posts on first boot.
//
// initialPassword is the password to seed for the "admin" user. When it is
// empty, a random 16-byte base64url password is generated and returned so the
// caller can print it in a console banner (the password never lands in log
// files). It only returns the password when admin was actually created — an
// existing admin account is never reset.
func seedDemoData(db *gorm.DB, initialPassword string) string {
	var user models.User
	var seededPwd string
	if db.Where("username = ?", "admin").First(&user).Error != nil {
		seededPwd = initialPassword
		if seededPwd == "" {
			seededPwd = randomPassword()
		}
		hash, err := models.HashPassword(seededPwd)
		if err != nil {
			seededPwd = ""
		} else {
			// The first-boot admin is the site's root (single super-admin). It
			// is seeded with a random (or env-supplied) password that the user
			// did not choose, so they are forced to pick their own at first
			// login (MustChangePassword flag). Root is the only role that
			// exists on first boot; admins create further users from there.
			user = models.User{Username: "admin", Email: "admin@example.com", PasswordHash: hash, Role: models.RoleRoot, MustChangePassword: true}
			if db.Create(&user).Error != nil {
				seededPwd = ""
			}
		}
	}

	var count int64
	db.Model(&models.Post{}).Count(&count)
	if count == 0 {
		// Seed categories.
		cats := []models.Category{
			{Name: "技术"},
			{Name: "随笔"},
			{Name: "教程"},
		}
		for i := range cats {
			_ = db.Create(&cats[i]).Error
		}

		// Seed tags.
		tagNames := []string{"Go", "Preact", "Markdown", "架构", "前端"}
		tagIDs := make([]models.Tag, 0, len(tagNames))
		for _, name := range tagNames {
			var t models.Tag
			if db.Where("name = ?", name).First(&t).Error != nil {
				t = models.Tag{Name: name}
				_ = db.Create(&t).Error
			}
			tagIDs = append(tagIDs, t)
		}

		posts := []models.Post{
			{
				Title: "欢迎来到内容平台",
				Body:  "这是一篇示例文章，展示 **Markdown 渲染效果**。\n\n## 特性\n\n- 标题、列表、代码块\n- 图片链接（本地 / 图床）\n- 引用块与分隔线\n\n> 好的内容，不需要华丽的装饰。\n\n```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello, echo\")\n}\n```\n\n---\n\n如果你正在读这篇种子文章，说明后端初始化成功了。去写你自己的第一篇文章吧！",
				AuthorID: user.ID,
			},
			{
				Title: "Go 后端最佳实践",
				Body:  "从项目结构、错误处理到数据库访问，聊聊 Go Web 服务的工程化经验。\n\n## 项目结构\n\n```\ninternal/\n  config/    # 配置加载\n  models/    # GORM 模型\n  services/  # 业务逻辑（厚层）\n  handlers/  # HTTP 薄层\n  middleware/ # 中间件\n```\n\n错误处理的原则：**把业务错误包装成 `*ServiceError`**，不要将 `gorm` 原始错误泄漏给前端。\n\n### 连接池\n\n低配 VPS（1 GB RAM）上，SQLite 的 `MaxOpenConns` 刻意设为 4，`MaxIdleConns` 设为 2。更大的值只会白白消耗内存。\n\n---\n\n*本文是项目示例文章，仅供演示 Markdown 渲染。*",
				AuthorID: user.ID,
			},
			{
				Title: "前后端分离架构",
				Body:  "REST API + 独立前端：职责清晰、可独立部署、易于扩展。\n\n## 为什么选择 Preact\n\nPreact 是 React 的轻量替代，核心约 **3 KB**（gzip），API 完全兼容。本项目还用了：\n\n| 依赖 | 用途 | 体积 |\n|------|------|------|\n| `wouter` | URL 路由 | ~4 KB |\n| `marked` | Markdown 渲染 | ~34 KB |\n| 手写 i18n / 主题 | 无额外依赖 | 0 KB |\n\n## 设计原则\n\n1. **厚薄分层**：业务逻辑进 `services` / `core`，页面只做布局。\n2. **低资源**：不用 Redis、不用 ORM 之外的重型库。\n3. **简约视觉**：矩形、无圆角、无阴影、不用 emoji。\n\n> \"少即是多。\" — Mies van der Rohe\n\n---\n\n*本系统面向低配 VPS 设计，追求小而美。*",
				AuthorID: user.ID,
			},
		}
		for i := range posts {
			// Assign a category round-robin so each demo post lands in one.
			posts[i].CategoryID = &cats[i%len(cats)].ID
			_ = db.Create(&posts[i]).Error

			// Attach the seeded tags to each post.
			if len(tagIDs) > 0 {
				_ = db.Model(&posts[i]).Association("Tags").Append(tagIDs[:min(len(tagIDs), i+2)])
			}
		}
	}
	return seededPwd
}

// randomPassword generates a cryptographically random 22-character
// base64url-encoded password (16 bytes of entropy) using crypto/rand.
func randomPassword() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		// Fallback: time-based entropy; only reachable when /dev/urandom is
		// unavailable (extremely unlikely on any modern OS).
		b := make([]byte, 16)
		copy(b, []byte(time.Now().UTC().Format("20060102150405")))
		return base64.RawURLEncoding.EncodeToString(b)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// runManualMigrations applies one-shot schema changes that AutoMigrate cannot
// perform (column drops). Each migration is gated by a row in
// schema_migrations so it runs exactly once.
//
// Spec #8: drop the deprecated posts.scheduled_at column. Scheduled
// publishing was removed when the visibility model collapsed to
// public/private; the column is dead and is dropped via the
// create-table / copy / drop / rename dance (SQLite cannot ALTER TABLE DROP
// COLUMN on older toolchains, and the copy approach is version-agnostic).
func runManualMigrations(db *gorm.DB) error {
	if exists := db.Migrator().HasTable(&models.SchemaMigration{}); !exists {
		if err := db.Migrator().CreateTable(&models.SchemaMigration{}); err != nil {
			return fmt.Errorf("create schema_migrations: %w", err)
		}
	}

	if err := dropScheduledAt(db); err != nil {
		return err
	}
	if err := addSmtpColumns(db); err != nil {
		return err
	}
	// Role model (spec: three-tier roles). Promote the oldest admin to root
	// and guarantee exactly one root across the whole database.
	if err := promoteFirstAdminToRoot(db); err != nil {
		return err
	}
	// Content moderation (spec: review + banned vault). Add the post review
	// metadata columns idempotently.
	if err := addReviewColumns(db); err != nil {
		return err
	}
	return nil
}

// addSmtpColumns ensures the site_configs table carries the SMTP settings
// columns added for security notifications. New databases get them from
// AutoMigrate; already-migrated databases have the table but lack the
// columns, so we add them idempotently via a probe (ALTER TABLE ADD COLUMN is
// not idempotent in SQLite).
func addSmtpColumns(db *gorm.DB) error {
	const version = "0002_add_smtp_columns"
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return fmt.Errorf("check smtp migration gate: %w", err)
	}
	if count > 0 {
		return nil // already applied
	}

	// Fresh databases do not yet have the site_configs table (AutoMigrate runs
	// after runManualMigrations). There is nothing to add here; the table
	// will be created with all columns by AutoMigrate. Record the version so
	// the gate is consistent for the fresh-DB path.
	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='site_configs'").Scan(&tableCount).Error; err != nil {
		return fmt.Errorf("probe site_configs: %w", err)
	}
	if tableCount == 0 {
		return recordMigration(db, version)
	}

	probe := func(col string) (bool, error) {
		var n int64
		if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('site_configs') WHERE name = ?", col).Scan(&n).Error; err != nil {
			return false, err
		}
		return n > 0, nil
	}

	stmts := []struct {
		col string
		ddl string
	}{
		{"smtp_host", `ALTER TABLE site_configs ADD COLUMN smtp_host VARCHAR(128)`},
		{"smtp_port", `ALTER TABLE site_configs ADD COLUMN smtp_port INTEGER`},
		{"smtp_user", `ALTER TABLE site_configs ADD COLUMN smtp_user VARCHAR(128)`},
		{"smtp_pass", `ALTER TABLE site_configs ADD COLUMN smtp_pass TEXT`},
	}
	for _, s := range stmts {
		exists, err := probe(s.col)
		if err != nil {
			return fmt.Errorf("probe %s: %w", s.col, err)
		}
		if exists {
			continue
		}
		if err := db.Exec(s.ddl).Error; err != nil {
			return fmt.Errorf("add %s: %w", s.col, err)
		}
	}
	return recordMigration(db, version)
}

// dropScheduledAt removes posts.scheduled_at on databases that still carry it.
func dropScheduledAt(db *gorm.DB) error {
	const version = "0001_drop_scheduled_at"
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return fmt.Errorf("check migration gate: %w", err)
	}
	if count > 0 {
		return nil // already applied
	}

	// Probe whether the column actually exists (fresh databases created after
	// the model no longer declares it will not).
	var colCount int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('posts') WHERE name = 'scheduled_at'").Scan(&colCount).Error; err != nil {
		return fmt.Errorf("probe scheduled_at: %w", err)
	}
	if colCount == 0 {
		// Nothing to drop; still record the version so the gate is consistent.
		return recordMigration(db, version)
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
	return recordMigration(db, version)
}

// promoteFirstAdminToRoot introduces the three-tier role model. On databases
// created before the root role existed, the oldest "admin" account becomes the
// single "root"; on brand-new databases this is a no-op (the seeder already
// created root directly). It also enforces the invariant that there is at most
// one root in the whole table: if several roots somehow coexist, every root
// except the oldest is demoted to admin so the single-root guarantee holds.
func promoteFirstAdminToRoot(db *gorm.DB) error {
	const version = "0003_root_migration"
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return fmt.Errorf("check root migration gate: %w", err)
	}
	if count > 0 {
		return nil // already applied
	}

	// Fresh databases do not have the users table yet (AutoMigrate runs after
	// runManualMigrations). Nothing to promote; record the version so the gate
	// is consistent for the fresh-DB path.
	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tableCount).Error; err != nil {
		return fmt.Errorf("probe users table: %w", err)
	}
	if tableCount == 0 {
		return recordMigration(db, version)
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
		return recordMigration(db, version)
	}

	// No root exists yet: promote the oldest admin. "created_at, id ASC" picks
	// the account that was provisioned first — the de-facto founder of the site.
	var admin models.User
	err := db.Where("role = ?", models.RoleAdmin).
		Order("created_at ASC, id ASC").First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No admin either (empty table on a fresh DB that AutoMigrate will seed
		// right after). Record the gate; the seeder will create root directly.
		return recordMigration(db, version)
	}
	if err != nil {
		return fmt.Errorf("find oldest admin: %w", err)
	}
	if err := db.Model(&models.User{}).Where("id = ?", admin.ID).
		Update("role", models.RoleRoot).Error; err != nil {
		return fmt.Errorf("promote to root: %w", err)
	}
	return recordMigration(db, version)
}

// addReviewColumns ensures the posts table carries the review-moderation
// columns (review_note / reviewed_by / reviewed_at) added for the content
// review workflow and the banned vault. New databases get them from
// AutoMigrate; already-migrated databases have the table but lack the columns,
// so we add them idempotently via a probe (ALTER TABLE ADD COLUMN is not
// idempotent in SQLite).
func addReviewColumns(db *gorm.DB) error {
	const version = "0004_add_review_columns"
	var count int64
	if err := db.Model(&models.SchemaMigration{}).Where("version = ?", version).Count(&count).Error; err != nil {
		return fmt.Errorf("check review migration gate: %w", err)
	}
	if count > 0 {
		return nil // already applied
	}

	// Fresh databases do not have the posts table yet (AutoMigrate runs after
	// runManualMigrations). Nothing to add; record the version for consistency.
	var tableCount int64
	if err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='posts'").Scan(&tableCount).Error; err != nil {
		return fmt.Errorf("probe posts table: %w", err)
	}
	if tableCount == 0 {
		return recordMigration(db, version)
	}

	probe := func(col string) (bool, error) {
		var n int64
		if err := db.Raw("SELECT COUNT(*) FROM pragma_table_info('posts') WHERE name = ?", col).Scan(&n).Error; err != nil {
			return false, err
		}
		return n > 0, nil
	}

	stmts := []struct {
		col string
		ddl string
	}{
		{"review_note", `ALTER TABLE posts ADD COLUMN review_note TEXT`},
		{"reviewed_by", `ALTER TABLE posts ADD COLUMN reviewed_by VARCHAR(36)`},
		{"reviewed_at", `ALTER TABLE posts ADD COLUMN reviewed_at DATETIME`},
	}
	for _, s := range stmts {
		exists, err := probe(s.col)
		if err != nil {
			return fmt.Errorf("probe %s: %w", s.col, err)
		}
		if exists {
			continue
		}
		if err := db.Exec(s.ddl).Error; err != nil {
			return fmt.Errorf("add %s: %w", s.col, err)
		}
	}
	return recordMigration(db, version)
}

func recordMigration(db *gorm.DB, version string) error {
	return db.Create(&models.SchemaMigration{Version: version, AppliedAt: time.Now()}).Error
}

// collectExistingTables populates a set of table names that already exist in
// the database (from sqlite_master). Used to decide whether AutoMigrate is
// needed: on a fresh db nothing exists and the full AutoMigrate runs; on an
// already-migrated db the hand-written migrations own the schema and GORM's
// rebuild path is unreliable on the glebarez driver.
func collectExistingTables(db *gorm.DB, out map[string]bool) error {
	var names []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'").Scan(&names).Error; err != nil {
		return err
	}
	for _, n := range names {
		out[n] = true
	}
	return nil
}

// commonTableName maps a GORM model to its table name (GORM pluralizes and
// snake_cases by default: User -> users, Post -> posts, etc.).
func commonTableName(m interface{}) string {
	type tableName interface {
		TableName() string
	}
	if t, ok := m.(tableName); ok {
		return t.TableName()
	}
	switch m.(type) {
	case *models.User:
		return "users"
	case *models.Post:
		return "posts"
	case *models.Category:
		return "categories"
	case *models.Tag:
		return "tags"
	case *models.PostTag:
		return "post_tags"
	case *models.SiteConfig:
		return "site_configs"
	case *models.LoginLog:
		return "login_logs"
	case *models.SchemaMigration:
		return "schema_migrations"
	}
	return ""
}
