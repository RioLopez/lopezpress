package config

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"lopezpress/internal/clientip"
	"lopezpress/internal/models"
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
	Port string
	// Host is the interface the HTTP server binds to. Empty means 127.0.0.1
	// (see ListenAddr): behind the same-host reverse proxy this project
	// deploys, nothing outside the box has any reason to reach the Go port.
	//
	// The server used to bind ":" + Port, i.e. every interface. That is not a
	// theoretical exposure: with the port merely reachable — a forgotten
	// security-group rule, a ufw insert, a container port publish — a scanner
	// that finds :8081 talks to Go directly and skips nginx entirely, which
	// also skips every header and deny rule nginx would have applied, and gets
	// answers straight from the application. Loopback by default makes "the
	// app is only reachable through the proxy" the default rather than a thing
	// the operator must remember to arrange.
	//
	// Set HOST=0.0.0.0 explicitly for Docker or a multi-host setup.
	Host                 string
	JWTSecret            string
	JWTExpiryHours       int
	DBPath               string
	DBMaxOpen            int
	DBMaxIdle            int
	LogLevel             string
	RateLimitRPS         float64
	RateLimitBurst       int
	UploadDir            string // where uploaded images are stored on disk
	UploadMaxBytes       int64  // per-file upload size cap
	BackupDir            string // where JSON backup dumps are written
	AdminInitialPassword string // optional; if set, used as admin's initial password on first boot
	CORSOrigins          string // comma-separated allowed origins; "*" allows all (dev default)
	SmmsToken            string // SM.MS image-host token held server-side; optional
	// TrustedProxies is the comma-separated IP/CIDR allow-list whose
	// X-Forwarded-For headers are believed. Empty
	// falls back to loopback only — see internal/clientip.
	TrustedProxies string
	// CaptchaEnabled turns the login challenge on. Default true: the whole
	// point is that it is not something an operator has to remember to switch
	// on before an incident.
	CaptchaEnabled bool
	// HealthShowVersion makes GET /health include the build version and commit.
	// Default FALSE: the probe is unauthenticated, so on a public deployment it
	// is a free "which build are you running" oracle for anyone port-scanning —
	// and once an attacker knows the exact commit, "is this build vulnerable to
	// <the CVE fixed in the next commit>" needs no guessing. The version is
	// available where it belongs: `lopezpress -version` over ssh, and the
	// startup log line. Turn this on only for a private/staging deploy where a
	// load balancer needs to read it over HTTP.
	HealthShowVersion bool
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		Port: envOr("PORT", "8080"),
		Host: envOr("HOST", "127.0.0.1"), // loopback unless told otherwise
		// JWT_SECRET has NO default. An empty value is replaced at startup with
		// a freshly generated random secret (see ensureJWTSecret), because a
		// hard-coded default is not a weak default — it is a published one.
		//
		// This used to be "dev-secret-change-me-in-prod". APP_ENV defaults to
		// "dev", and ValidateSecurity only WARNS in dev, so a deployment that
		// forgot one environment variable started happily with a signing key
		// printed in the source tree. Anyone who can read the repository could
		// mint a token for any account, root included. The failure was silent:
		// one stderr line among the boot log.
		//
		// Generating a random secret makes "forgot to configure it" degrade to
		// "tokens do not survive a restart" — annoying, visible, and safe —
		// instead of "the site is open to anyone who read the source".
		JWTSecret:      os.Getenv("JWT_SECRET"),
		JWTExpiryHours: envIntOr("JWT_EXPIRY_HOURS", 1),
		DBPath:         envOr("DB_PATH", "data/app.db"),
		// Small pool on purpose (see budget notes above).
		DBMaxOpen: envIntOr("DB_MAX_OPEN", 4),
		DBMaxIdle: envIntOr("DB_MAX_IDLE", 2),
		LogLevel:  envOr("LOG_LEVEL", "warn"),
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
		// SM.MS image-host token held server-side. The browser no
		// longer carries this secret; /api/upload proxies to SM.MS when it
		// is set. Falls back to the site-config token when empty.
		SmmsToken: os.Getenv("SM_MS_TOKEN"),
		// Proxy trust list for client-IP resolution.
		// Empty -> internal/clientip trusts loopback only (nginx on the same
		// host). Set it in prod when the proxy lives on another host.
		TrustedProxies: envOr("TRUSTED_PROXIES", clientip.DefaultTrustedProxySpec),
		// Login captcha. On by default and it stays on: an operator who had to
		// remember to enable it before an incident is an operator who did not.
		CaptchaEnabled: envBoolOr("CAPTCHA_ENABLED", true),
		// Default off: a public probe should not answer "which build is this".
		HealthShowVersion: envBoolOr("HEALTH_SHOW_VERSION", false),
	}
}

// ListenAddr returns the host:port the HTTP server binds to, e.g.
// "127.0.0.1:8081". JoinHostPort is used rather than string concatenation so
// a bare IPv6 HOST ("::1") becomes "[::1]:8081" instead of an unparseable
// "::1:8081".
func (c *Config) ListenAddr() string {
	host := strings.TrimSpace(c.Host)
	if host == "" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, c.Port)
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

// envBoolOr reads a boolean env var. An unparsable value falls back to def
// instead of erroring: a typo in CAPTCHA_ENABLED must not keep the process
// from starting, and both defaults here are the safe ones.
func envBoolOr(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def
	}
	return b
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

// sqliteDSN appends the connection-scoped pragmas to the database path.
// busy_timeout and synchronous are per-connection settings, so they have to be
// applied on every pooled connection — which is what the driver's _pragma
// parameter does.
//
// _txlock=immediate makes every transaction take the write lock at BEGIN
// instead of on its first write. That matters for one specific reason: the
// upload quota was a read-then-write sequence ("SUM the bytes, compare, then
// INSERT"), and two concurrent uploads could both pass the comparison before
// either inserted — R4-08 / R6-03 measured 8 concurrent uploads landing 7
// successes with the user 1.4 MB over quota. Serialising writers at BEGIN
// closes that window without adding a bookkeeping table or a background
// reconciler. Reads are unaffected: WAL still lets them run concurrently with
// the single writer.
func sqliteDSN(path string) string {
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + "_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_txlock=immediate"
}

// logPragmas prints the pragmas that actually took effect. SQLite settings are
// silent and connection-scoped, so "configured" and "in force" can drift apart
// without any symptom until an incident, when guessing is expensive.
func logPragmas(db *gorm.DB) {
	var mode string
	var busy, sync int
	db.Raw("PRAGMA journal_mode").Scan(&mode)
	db.Raw("PRAGMA busy_timeout").Scan(&busy)
	db.Raw("PRAGMA synchronous").Scan(&sync)
	log.Printf("sqlite: journal_mode=%s busy_timeout=%dms synchronous=%d", mode, busy, sync)
}

// InitDB opens the SQLite database, runs schema migrations and seeds demo
// data. It returns the database and the seeded admin password (empty if an
// admin already existed or seeding failed).
func InitDB(cfg *Config) (*gorm.DB, string, error) {
	if err := os.MkdirAll(dirOf(cfg.DBPath), 0o755); err != nil {
		return nil, "", fmt.Errorf("create db dir: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(sqliteDSN(cfg.DBPath)), &gorm.Config{
		Logger: quietLogger{logger.Default.LogMode(gormLogMode(cfg.LogLevel))},
	})
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite: %w", err)
	}

	// Rollback-journal mode (the default) makes every commit fsync twice and
	// briefly take an exclusive lock, so a burst of writes (thumbnail worker,
	// uploads, audit rows) can starve every reader and, if a writer dies
	// mid-commit, leave a hot journal that wedges the whole file until the
	// process restarts. WAL lets readers proceed during a write and turns
	// recovery into an open-time replay, so a stuck writer costs latency
	// instead of the entire site. It is a persistent property of the file, so
	// it is set once here rather than per-connection: running
	// PRAGMA journal_mode=WAL while another connection holds a read lock can
	// itself return SQLITE_BUSY and break connection setup.
	if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
		return nil, "", fmt.Errorf("enable wal: %w", err)
	}

	// Tight connection pool for a low-memory VPS (1.5 GB, ~1 GB usable).
	// SQLite is single-file; a large pool only wastes memory on goroutines.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, "", fmt.Errorf("sql handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpen)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdle)

	logPragmas(db)

	// One-shot manual migrations run BEFORE AutoMigrate. AutoMigrate only
	// adds missing columns/tables and never drops; dropping the deprecated
	// posts.scheduled_at column requires the hand-written
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
	//
	// models.All() is the single entity list: the drift check below uses the
	// same one, so a new table cannot be added to one path and forgotten in
	// the other (that is exactly how PostRevision escaped the drift check).
	allModels := models.All()
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

	// The partial UNIQUE index on posts.slug (R6-02). Migration 0014 creates it
	// on an existing database, but on a FRESH install it runs before AutoMigrate
	// has created the posts table and therefore skips — leaving a brand-new
	// database with a plain index and no uniqueness at all. Re-asserting it here,
	// after AutoMigrate, is what makes both paths converge. Idempotent
	// (IF NOT EXISTS), so a restart costs nothing.
	//
	// It has to be partial (WHERE slug <> '') because "no slug yet" is stored as
	// an empty string, and a plain UNIQUE index would reject the second such row.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_slug_unique ON posts(slug) WHERE slug <> ''`).Error; err != nil {
		return nil, "", fmt.Errorf("create slug index: %w", err)
	}

	// Presets MUST be seeded before the demo data — otherwise the
	// demo's own 技术/随笔/教程 rows win the "categories are empty" check and
	// the designed two-level tree never appears on a fresh install.
	seedPresetCategories(db)
	seededPwd := seedDemoData(db, cfg.AdminInitialPassword)
	seedSiteConfig(db)
	// Adopt whatever is already on disk into the upload metadata
	// table. Must run after AutoMigrate created upload_files on a fresh DB,
	// and it is idempotent so a restart is a single SELECT.
	backfillUploads(db, cfg.UploadDir)
	checkSchemaDrift(db)
	return db, seededPwd, nil
}

// seedPresetCategories fills the shipped two-level category tree on a fresh
// database only. An existing install keeps whatever the owner built;
// they can still apply the preset explicitly from the admin page, which is
// idempotent and never renames or deletes anything.
func seedPresetCategories(db *gorm.DB) {
	var n int64
	if err := db.Model(&models.Category{}).Count(&n).Error; err != nil || n > 0 {
		return
	}
	for _, root := range models.PresetCategories {
		parent := models.Category{Name: root.Name}
		if err := db.Create(&parent).Error; err != nil {
			continue
		}
		for _, childName := range root.Children {
			pid := parent.ID
			_ = db.Create(&models.Category{Name: childName, ParentID: &pid}).Error
		}
	}
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
		ID:            1,
		SiteName:      "LopezPress",
		SiteTitle:     "LopezPress",
		ImgHostMode:   "local",
		AllowRegister: false,
		PageSize:      10,
		PublicRead:    true,
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
		// Reuse the preset tree when it exists (it normally does — presets are
		// seeded first); only fall back to the legacy trio on an install that
		// somehow has none. Creating a second, parallel set of categories was
		// exactly the bug that hid the preset tree on a fresh database.
		var cats []models.Category
		if err := db.Order("id ASC").Find(&cats).Error; err != nil || len(cats) == 0 {
			cats = []models.Category{{Name: "技术"}, {Name: "随笔"}, {Name: "教程"}}
			for i := range cats {
				_ = db.Create(&cats[i]).Error
			}
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
				Title:    "欢迎来到内容平台",
				Body:     "这是一篇示例文章，展示 **Markdown 渲染效果**。\n\n## 特性\n\n- 标题、列表、代码块\n- 图片链接（本地 / 图床）\n- 引用块与分隔线\n\n> 好的内容，不需要华丽的装饰。\n\n```go\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello, lopezpress\")\n}\n```\n\n---\n\n如果你正在读这篇种子文章，说明后端初始化成功了。去写你自己的第一篇文章吧！",
				AuthorID: user.ID,
			},
			{
				Title:    "Go 后端最佳实践",
				Body:     "从项目结构、错误处理到数据库访问，聊聊 Go Web 服务的工程化经验。\n\n## 项目结构\n\n```\ninternal/\n  config/    # 配置加载\n  models/    # GORM 模型\n  services/  # 业务逻辑（厚层）\n  handlers/  # HTTP 薄层\n  middleware/ # 中间件\n```\n\n错误处理的原则：**把业务错误包装成 `*ServiceError`**，不要将 `gorm` 原始错误泄漏给前端。\n\n### 连接池\n\n低配 VPS（1 GB RAM）上，SQLite 的 `MaxOpenConns` 刻意设为 4，`MaxIdleConns` 设为 2。更大的值只会白白消耗内存。\n\n---\n\n*本文是项目示例文章，仅供演示 Markdown 渲染。*",
				AuthorID: user.ID,
			},
			{
				Title:    "前后端分离架构",
				Body:     "REST API + 独立前端：职责清晰、可独立部署、易于扩展。\n\n## 为什么选择 Preact\n\nPreact 是 React 的轻量替代，核心约 **3 KB**（gzip），API 完全兼容。本项目还用了：\n\n| 依赖 | 用途 | 体积 |\n|------|------|------|\n| `wouter` | URL 路由 | ~4 KB |\n| `marked` | Markdown 渲染 | ~34 KB |\n| 手写 i18n / 主题 | 无额外依赖 | 0 KB |\n\n## 设计原则\n\n1. **厚薄分层**：业务逻辑进 `services` / `core`，页面只做布局。\n2. **低资源**：不用 Redis、不用 ORM 之外的重型库。\n3. **简约视觉**：矩形、无圆角、无阴影、不用 emoji。\n\n> \"少即是多。\" — Mies van der Rohe\n\n---\n\n*本系统面向低配 VPS 设计，追求小而美。*",
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

// checkSchemaDrift logs (never fixes) any model column that is missing from the
// live schema.
//
// This is the durable guard for the class of bug that produced 0008: because
// AutoMigrate is deliberately never run against existing tables,
// a column added to a model without a matching hand-written migration simply
// does not exist, and the failure only shows up later as a 500 on one
// endpoint. A startup WARN makes that immediately visible instead.
func checkSchemaDrift(db *gorm.DB) {
	// Driven by models.All(), not a hand-copied list: the previous list had 9
	// of 11 entities, so post_revisions was never checked for missing columns.
	for _, m := range models.All() {
		if !db.Migrator().HasTable(m) {
			continue
		}
		typ := reflect.TypeOf(m)
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
		table := commonTableName(m)
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			if f.Anonymous || f.Tag.Get("gorm") == "-" {
				continue
			}
			if !isColumnField(f.Type) {
				continue // relations (slices / structs) are not columns
			}
			name := db.NamingStrategy.ColumnName("", f.Name)
			if !db.Migrator().HasColumn(m, name) {
				log.Printf("WARN schema drift: %s.%s is missing — add a hand-written migration (AutoMigrate never alters existing tables)", table, name)
			}
		}
	}
}

// isColumnField reports whether a struct field maps to a database column
// (relations and time.Time pointers do not follow the slice/struct skip rule).
func isColumnField(t reflect.Type) bool {
	timeType := reflect.TypeOf(time.Time{})
	switch t.Kind() {
	case reflect.Slice, reflect.Map:
		return false
	case reflect.Struct:
		return t == timeType
	case reflect.Ptr:
		elem := t.Elem()
		if elem == timeType {
			return true
		}
		return elem.Kind() != reflect.Struct
	}
	return true
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
	case *models.ActionLog:
		return "action_logs"
	case *models.PostView:
		return "post_views"
	case *models.PostRevision:
		return "post_revisions"
	}
	return ""
}

// ensureJWTSecret guarantees the process has a usable signing key, and makes the
// absence of a configured one loud.
//
// An empty JWT_SECRET used to mean "use the value hard-coded in this file",
// which is a published secret rather than a missing one. Now it means "generate
// one": the site stays safe, and the only cost is that tokens do not survive a
// restart.
//
// In dev (APP_ENV unset or "dev") the generated secret is CACHED in a file next
// to the database, so a normal `go run` cycle does not log the developer out on
// every reload. Caching a secret is only acceptable there: the file sits inside
// the project directory with 0600, and a dev box is not the threat model. In
// strict mode nothing is cached — a secret that persists across restarts must be
// the operator's, so that a compromised host does not keep a working key.
func (c *Config) EnsureJWTSecret() error {
	if strings.TrimSpace(c.JWTSecret) != "" {
		return nil
	}
	appEnv := envOr("APP_ENV", "dev")
	strict := appEnv != "dev"

	if !strict {
		if cached := readCachedDevSecret(c.DBPath); cached != "" {
			c.JWTSecret = cached
			return nil
		}
	}

	secret, err := randomSecret(48)
	if err != nil {
		// Refusing to start is the correct answer: a process with no usable
		// signing key cannot authenticate anybody, and falling back to a
		// constant would undo the whole point of this function.
		return fmt.Errorf("cannot generate a JWT secret: %w", err)
	}
	c.JWTSecret = secret

	if strict {
		// In production an ephemeral key means every restart logs everyone out,
		// which is an outage in all but name. Refuse instead of limping.
		return fmt.Errorf("APP_ENV=%s: JWT_SECRET must be set; refusing to start with a "+
			"generated key that would invalidate every session on restart", appEnv)
	}

	if err := writeCachedDevSecret(c.DBPath, secret); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: JWT_SECRET not set and the dev cache could not be written (%v); "+
			"tokens will be invalidated on every restart. Set JWT_SECRET in your .env.\n", err)
		return nil
	}
	fmt.Fprintf(os.Stderr, "WARN: JWT_SECRET not set; generated a random one and cached it for dev. "+
		"Set JWT_SECRET in your .env before deploying.\n")
	return nil
}

// randomSecret returns n bytes of crypto/rand, hex-encoded.
func randomSecret(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// devSecretPath names the cache file. It lives beside the database so a project
// with several DB files (dev + smoke) also gets separate secrets.
func devSecretPath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), ".dev-jwt-secret")
}

func readCachedDevSecret(dbPath string) string {
	b, err := os.ReadFile(devSecretPath(dbPath))
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if len(s) < 32 {
		return "" // ignore anything that could not be a real secret
	}
	return s
}

func writeCachedDevSecret(dbPath, secret string) error {
	path := devSecretPath(dbPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// 0600: the file is a signing key, even in dev. The mode passed to WriteFile
	// only applies when the file is created; an existing file keeps its mode, so
	// Chmod afterwards makes the intent explicit on every write.
	//
	// Known platform limitation: Windows ignores these bits (only the read-only
	// flag maps), so on a Windows dev box the file is readable by other local
	// accounts. That is a dev-machine concern, not a deployment one — production
	// never writes this file at all, and refuses to start without an operator
	// supplied JWT_SECRET. Documented rather than papered over: a "0600" that
	// silently means "everyone" on one OS is exactly the kind of thing that gets
	// trusted when it should not be.
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
