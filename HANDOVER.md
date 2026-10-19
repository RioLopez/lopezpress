# Echo Content API — 交接文档（AI 接手用）

> 版本：v2.0
> 更新时间：2026-09-24
> 用途：**新接手的 AI / 工程师的第一份必读**。本文档覆盖项目全貌、当前实现状态、如何跑起来、关键陷阱、已知问题、未决决策，以及自测清单。
> 配套文档：`docs/ARCHITECTURE.md`（设计稿）、`docs/AUDIT_REPORT.md`（审计与已知问题）、`PROGRESS.md`（26 轮开发全记录）、`README.md`（快速上手）、`docs/specs/*`（21 份功能规格）、`docs/decisions/*`（重大决策）。
> **AI 协作者另读**：根目录 `AGENTS.md`（速查 + 红线）与 `docs/AI_STANDARD.md`（五阶段工作流 + 验证门禁 + 十条红线）——2026-10-19 建立的协作标准，本文件 §7 的陷阱是它的红线来源。

---

## 0. 先读这一段（最重要）

- **技术方向已定稿，不需要换框架。** 后端 Go 标准库 `net/http` + GORM + 纯 Go SQLite；前端 Preact + Vite + TS。目标机型是低配 VPS（1 vCPU / 1.5 GB / 5 GB 磁盘）。这套组合在"低资源 + 可维护"维度是最优解，不要为了"更现代"去改框架（详见 §9 决策记录）。
- **当前代码已落地第 1–22 轮开发 + 10 项安全加固**，全部 `go build` / `go vet` / `go test` / `tsc --noEmit` / `vite build` 通过。
- **有一个会反复咬人的陷阱**：`glebarez/sqlite` 驱动 + GORM `AutoMigrate` 在"误判需要重建表"时会走 copy-rename 路径，生成的 `INSERT` 可能漏掉 NOT NULL 列导致启动崩溃。修复方式见 §7.1，改 `internal/config/config.go` 时务必小心，**不要回退成无条件的 `AutoMigrate`**。
- **AI 协作先读 `AGENTS.md` + `docs/AI_STANDARD.md`**：本仓库有一整套驱动 AI 干活的标准（五阶段工作流、验证门禁、十条红线、提示词模板库 `docs/prompts/`）。新接手的 AI 按它执行，用户说「继续」即触发「读上下文 → 选下一优先级 TODO → 实现 → 门禁 → 记录」循环。
- **本地一键启动用 `EchoAPI.bat`**（Windows）。它已检测端口、避免重复启动、自动开浏览器。服务器进程请用「独立窗口 / 后台任务」方式运行，否则对话会话结束会被回收（见 §3）。
- **首启 admin 密码是随机生成的**，只在 stderr 打印一次；登录后会被强制跳到 `/change-password` 改密（除非你用 `ADMIN_INITIAL_PASSWORD` 固定）。

---

## 1. 项目一句话

面向低配 VPS 的**可自托管全栈内容站点**：Go 单二进制 + SQLite 单文件（后端），Preact 单页（前端），前后端分离。定位"小而美的个人博客 / 轻 CMS"，功能深度优先于广度，追求极致资源占用比与可维护性。

- 单二进制（Go，零 CGO）+ 单文件数据库（SQLite）= 部署即解压，无系统依赖。
- 前端静态产物（~180 kB JS / ~48 kB CSS gzip 后 ~56 kB / ~8.8 kB），放 Nginx 任意静态目录即可。
- 总依赖极少：后端 ~12 模块 + 前端 2 个 npm 包（`wouter` + `marked`），供应链可审计。

---

## 2. 技术栈（当前真实状态）

| 维度 | 选型 | 说明 |
|------|------|------|
| 后端语言/路由 | Go 1.22 + `net/http` 标准库 `ServeMux`（方法 + 通配） | 零框架，资源占用最低 |
| 数据层 | GORM v1.25.x + SQLite（纯 Go 驱动 `github.com/glebarez/sqlite`） | 单二进制，无 CGO，可交叉编译 |
| 鉴权 | `golang-jwt/jwt/v5` HS256 | JWT 无状态 |
| 密码 | bcrypt（`golang.org/x/crypto/bcrypt`） | 非明文 |
| 限流 | `golang.org/x/time/rate` 令牌桶 | 内存态，单实例 OK；多实例需换 Redis |
| 前端 | Preact + Vite 5 + TypeScript | 核心 ~3 KB，手写 i18n（中英）/ 主题 / 路由（wouter） |
| Markdown | `marked`（同步）+ DOMParser 清洗 | 前端 `web/src/core/md.ts` |
| 可见性模型 | `published`（公开）/ `private`（仅作者） | 无 draft / 无 scheduled（第 22 轮定稿移除） |
| 角色 | `user` / `admin` 两角色 | 文章维度仅公开/私密两态 |

> ⚠️ **FTS5 全文搜索不可用**：纯 Go 驱动无 FTS5 扩展，搜索走 `LIKE '%...%'`（千级文章内可接受）。若要 FTS5 需换 cgo 驱动（`mattn/go-sqlite3`），代价是失去零 CGO 卖点 —— 当前决策是**不换**。
> ⚠️ **不跨设备同步草稿**：草稿仅浏览器 `localStorage`（30 天 TTL）。

---

## 3. 如何运行

### 3.1 本地开发（Windows，推荐 `EchoAPI.bat`）

项目根目录的 `EchoAPI.bat` 已做好：
- **幂等**：启动前检测 `:8080` / `:5173` 是否已在监听，已运行就跳过，不重复拉起。
- 后端优先用现成 `echo-api.exe`（`start /MIN` 独立窗口），否则回退 `go run ./cmd/server`。
- 前端 `npx vite --port 5173`。
- 启动完自动打开浏览器到 `http://localhost:5173`。

子命令：

```bat
EchoAPI.bat            :: 启动后端 + 前端，开浏览器到 :5173
EchoAPI.bat status     :: 显示 :8080 / :5173 是否在监听
EchoAPI.bat stop       :: 杀掉两个服务
EchoAPI.bat restart    :: 先 stop 再 start
EchoAPI.bat dev        :: 只起前端 vite
EchoAPI.bat build      :: vite build 打包到 web/dist
EchoAPI.bat help       :: 查看子命令
```

> 双击即用。Go 路径脚本会自动探测 `PATH` → `%USERPROFILE%\go-install\go\bin` → `C:\go\bin`。

### 3.2 手动启动（等价于 EchoAPI.bat 内部逻辑）

**后端**（需要 Go 1.22+，脚本里 Go 在 `C:\Users\Lopez\go-install\go\bin`）：

```powershell
cd echo-content-api
go run ./cmd/server          # 或：go build -o echo-api.exe ./cmd/server && .\echo-api.exe
# 监听 :8080；首启会在 stderr 打印随机 admin 密码横幅
```

**前端**（需要 Node + npm）：

```powershell
cd web
npm install
npm run dev                  # http://127.0.0.1:5173，/api/* 反代到 :8080
```

**验证端点（直连 :8080）**：

```powershell
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/auth/login -H "Content-Type: application/json" -d '{"email":"admin@example.com","password":"<首启随机密码>"}'
curl http://localhost:5173/api/site-config    # 经前端代理应 200
```

> ⚠️ **VPS 运维陷阱（踩过）**：之前用"会话后台任务"拉起 `go run` 进程，对话轮次切换/电脑重启后进程被回收，`:8080` 不监听，前端 vite 代理打不通后端，浏览器看到所有 `/api/*` 全是 500（实为 502 被代理层包成 500）。**长期运行请用 `EchoAPI.bat`（独立窗口）或 `deploy/` 的 systemd / nohup，不要依赖对话内后台进程。**

### 3.3 生产部署

见 `deploy/`：`build.sh`（交叉编译 Linux）、`deploy-to-vps.sh` / `deploy-to-vps.ps1`（一键上传）、`echo-content-api.service`（systemd）、`echo-content-api.env`（生产环境变量）、`nginx.conf`（反代 + 前端静态托管）、`install-systemd.sh`、`LOCAL.md`、`README.md`。生产务必设置：强 `JWT_SECRET`、`CORS_ORIGINS` 显式列表、`APP_ENV=prod`、HTTPS。

---

## 4. 如何测试 / 验证

```powershell
# 后端
cd echo-content-api
go build ./...              # 编译
go vet ./...               # 静态检查
go test ./...              # 单测（14 个 _test.go 文件，全绿）
go test ./internal/services/... -run Visibility -v   # 可见性三分支等

# 前端
cd web
npx tsc --noEmit           # 类型检查
npm run build              # 生产构建
```

测试文件清单（当前 14 个）：
- `internal/config/`：`config_test.go`、`mig_test.go`（Windows Close 修复）
- `internal/handlers/`：`handlers_test.go`
- `internal/middleware/`：`limiter_test.go`、`login_limiter_test.go`（5 用例）、`middleware_test.go`
- `internal/models/`：`models_test.go`
- `internal/services/`：`fts_test.go`、`round19_test.go`、`services_backup_test.go`（dry-run/schema 校验/snapshot）、`services_extra_test.go`、`services_test.go`、`services_visibility_test.go`（可见性三分支 + 私密 404 + 白名单兜底 + ListArchive + GetAuthor）、`testutil_test.go`

---

## 5. 当前实现状态（截至 2026-10-15）

### 5.1 功能轮次（PROGRESS.md 完整记录，这里只列结论）

第 1–25 轮已全部完成。除第 1–22 轮全部内容外，新增：

- **第 23 轮**：登录限流账号桶加固（HANDOVER §7.2）
- **第 24 轮**：容器化部署（`deploy/docker/` 多阶段 + compose + distroless；本机仅验证静态编译与 vite build，未 docker build）
- **第 25 轮（三层角色 + 审核流 + 封禁库）**：
  - 角色三级 `root`/`admin`/`user`，**root 全库唯一且不可经 API 创建/提升**
  - 文章状态五值 `published`/`private`/`pending`/`rejected`/`banned`（banned 仅 root 可见）
  - 审核流：user 发文→admin 审；admin 发文→root 审；通过 published / 打回 rejected（必带意见）
  - 封禁库：root-only，可「发回原作者 + 意见」或「永久删除」
  - 迁移 `0003_root_migration`（最早 admin 升 root）+ `0004_add_review_columns`
  - 后端新端点 6 个（见 §5.3）；前端新增「内容审核」「封禁库」两个后台页面

### 5.2 10 项安全加固（本轮新增，已全部完成并通过）

> 顺序按落地顺序 1→2→4→5→10→8→9→3→6→7。以下是"改了什么 / 在哪 / 怎么验"。

| # | 加固项 | 关键改动位置 | 验证方式 |
|---|--------|-------------|---------|
| 1 | **JWT_SECRET 启动强制校验** | `config.ValidateSecurity()`（main.go 开库前调用）；非空/非默认值/≥32 字节/非纯重复字符；`dev` 仅 warn，`prod`/`未知环境` 严格失败 | `APP_ENV=prod` 用弱 secret 启动应 `log.Fatalf`；dev 仅 stderr 警告 |
| 2 | **首启 admin 随机密码 + 强制改密** | `config.seedDemoData` 生成随机密码（console 横幅）；`User.MustChangePassword`；`POST /api/auth/change-password`（`handlers/auth.go` + `services.ChangePassword`） | 首启用随机密码登录 → 前端跳 `/change-password`；改密后 `must_change_password` 清零 |
| 3 | **登录接口单独限流 + 失败锁定** | `internal/middleware/login_limiter.go`（IP+账号双桶 0.5rps/burst3；5 次失败锁 15min；`Retry-After`）；`router.New` 仅套 `POST /api/auth/login` | 同一账号连错 5 次 → 429 + `Retry-After`；成功重置计数 |
| 4 | **Token 存储写死 + 加固** | 前端 `localStorage` 写死；`core/md.ts` DOMPurify 白名单；不引第三方脚本；JWT 1h 有效期 | 决策见 `docs/decisions/0003-token-storage.md` |
| 5 | **CORS 文档 + 白名单** | `CORS_ORIGINS` 逗号分隔；`*` 与 credentials 永不同时（`middleware.CORS`）；决策见 `docs/decisions/0004-cors.md` | prod 用 `*` → 启动失败；dev 任何源可跨域但无 credentials |
| 6 | **备份导入 dry-run + schema 校验 + 自动快照** | `services.ImportBackup(ctx, in, dryRun)`（`?dry_run=1` 返回统计不落库）；导出顶层 `schema_version`/`app_version`/`exported_at`；`VACUUM INTO` 快照保留 3 份；`POST /api/backup/restore-snapshot` | `?dry_run=1` 返回 `applied:false`；`restore-snapshot` 回滚到导入前快照 |
| 7 | **SM.MS token 服务端持有** | `.env` `SM_MS_TOKEN`；`/api/upload` 代理上传（withAuth）；未配置任何 token 返回 503；前端清理 `smms_token` localStorage | 配 `SM_MS_TOKEN` 后 SM.MS 模式走服务端；不配且站点设置也无 token → 503 |
| 8 | **清理 ScheduledAt 废弃字段 + listPosts() 死代码** | `models.Post` 删 `ScheduledAt`；`dropScheduledAt` 手写迁移（`posts_new` + `schema_migrations` 门控，幂等）；删前端 `listPosts()`；更新 AUDIT_REPORT | 旧库 `scheduled_at` 列被原地 drop 且数据保留；`go test` 全绿 |
| 9 | **补 service 层单测** | `services_visibility_test.go`、`services_backup_test.go` 等 | 可见性三分支 / 私密 404 / 白名单兜底 / ListArchive / GetAuthor 仅公开 / 备份 dry-run |
| 10 | **移动端用户菜单交互写死** | `web/src/ui/UserMenu.tsx` overlay 仅移动端渲染（`{isMobile && ...}` + `@media`）；PC hover 300ms / 移动端 tap+overlay；a11y | ≤768px 拖窄窗口，hover 浮层被 overlay+tap 替代 |

> 前端相关文件：`web/src/core/types.ts`（删 `scheduled_at`、加 `must_change_password`）、`web/src/core/api.ts`（加 `changePassword`、删 `listPosts`/`uploadImageSmms`）、`web/src/pages/ChangePasswordPage.tsx`、`web/src/pages/LoginPage.tsx`（登录后跳改密）、`web/src/main.tsx`（加 `/change-password` 路由）、`web/src/core/i18n.tsx`（加 `password` 区块）、`web/src/ui/UserMenu.tsx`、`web/src/index.css`、`web/src/pages/AdminSitePage.tsx`（SM.MS 服务端持有提示）。

### 5.3 内容审核 / 封禁库端点（第 25 轮新增）

| 方法 | 路径 | 角色 | 说明 |
|------|------|------|------|
| GET | `/api/admin/pending` | admin \| root | 审核队列；admin 只见 user 发文的 pending，root 全见 |
| POST | `/api/admin/review/{id}` | admin \| root | 体 `{"decision":"approve"\|"reject","note":"..."}`；reject 必带 note |
| POST | `/api/admin/ban/{id}` | **root** | 把文章拉入封禁库 |
| GET | `/api/admin/banned` | **root** | 封禁库列表 |
| POST | `/api/admin/banned/{id}/send-back` | **root** | 体 `{"note":"..."}`；banned→rejected 并邮件通知作者 |
| DELETE | `/api/admin/banned/{id}` | **root** | 硬删（Unscoped，不可恢复） |

> 注：`ReviewPostInput` 的 JSON 字段名是 `decision`（不是 `action`）——前端 `reviewPost()` 与 handler 注释均已对齐。

---

## 6. 目录结构与关键文件

```
echo-content-api/
├── cmd/server/main.go        # 入口：loadDotEnv → Load → ValidateSecurity → InitDB → 建 svc → 建双限流器 → 优雅关闭
├── internal/
│   ├── config/config.go      # .env 读取 + ValidateSecurity + InitDB（手动迁移门控 + AutoMigrate 门控）+ 种子数据
│   ├── models/models.go      # GORM 实体（User/Post/Category/Tag/PostTag/SiteConfig/LoginLog/SchemaMigration）
│   ├── services/
│   │   ├── services.go       # 核心业务（Post CRUD + 可见性 + 备份 + 回收站 + 站点配置 + 用户统计 + 备份导出/导入/snapshot）
│   │   └── accounts.go       # 用户/登录日志/注册/改密/个人资料/作者页/仪表盘
│   ├── handlers/             # 薄处理器（auth/posts/categories/accounts/admin/upload）
│   ├── middleware/           # Chain/Logger/CORS/Recover/Limiter + LoginLimiter（#3）
│   └── router/router.go      # 路由装配（Go 1.22 ServeMux + 中间件链 + 登录限流器）
├── web/                      # 前端（Preact + Vite + TS）
├── deploy/                   # 部署脚本（systemd / Nginx / 一键上传）
├── docs/                     # ARCHITECTURE / AUDIT_REPORT / specs(20) / decisions(0001-0004)
├── EchoAPI.bat               # 一键启动（Windows）
├── .env.example              # 配置项示例（含全部新增项）
├── go.mod
└── PROGRESS.md               # 22 轮开发全记录
```

**审计优先阅读顺序**（见 AUDIT_REPORT §4.1）：`cmd/server/main.go` → `internal/config` → `internal/router` → `internal/services/*`（业务规则全在这）→ `internal/handlers/auth.go`（`WithAuth`/`WithOptionalAuth`）→ `internal/middleware/*` → `internal/models` → `web/src/core` + `web/src/pages`。

---

## 7. ⚠️ 关键陷阱与回归风险（务必读完）

### 7.1 GORM AutoMigrate + glebarez/sqlite 启动崩溃 ⚠️⚠️⚠️

**现象**：
```
failed to init database: migrate: constraint failed: NOT NULL constraint failed: posts__temp.title (1299)
```

**根因**：`glebarez/sqlite` 驱动 + GORM `AutoMigrate` 在判定某张表"需要重建"（如要加 FK 约束、改类型）时，会走 copy-and-rename 路径：`CREATE posts__temp` → `INSERT INTO posts__temp SELECT ... FROM posts` → `DROP` → `RENAME`。这条 `INSERT` 在该驱动下**可能漏掉 NOT NULL 列**（实测漏了 `title`），触发约束违反。库本身通常是健康的——失败纯属 GORM 在"不该重建"时误判 + 该驱动重建实现有缺陷。

**当前修复**（`internal/config/config.go` `InitDB`）：
1. 手动迁移 `runManualMigrations` 先跑（其中 `CreateTable(schema_migrations)` 已做成幂等——表存在则跳过）。
2. **AutoMigrate 加门控**：`collectExistingTables` 收集现存表，**只有当某张表尚不存在时才对那张表调 `AutoMigrate`**；已存在的表由手动迁移（#8）管理，不再让 GORM 走重建路径。首启（无任何表）仍会跑完整 `AutoMigrate` 建出全部 schema。

**接手后改 `config.go` 的红线**：
- ❌ 不要改回「无条件 `db.AutoMigrate(...)`」。
- ❌ 不要调整顺序使 `AutoMigrate` 早于 `runManualMigrations`（手动迁移门控依赖 `schema_migrations` 表先存在）。
- ❌ 当给 `Post` 模型改列时，手动迁移 `dropScheduledAt` 里的 `posts_new` 列清单（`internal/config/config.go`）必须与 GORM `Post` 模型字段**保持同步**，否则重建会漏列。
- ✅ 新增表：在 `AutoMigrate` 列表和 `collectExistingTables` 的 `commonTableName` 映射里都加上即可（新表不存在 → 会被 AutoMigrate 建出）。
- ✅ 删除/改列等破坏性迁移：继续走「手写 `posts_new` + `schema_migrations` 版本门控」的老路（参考 `dropScheduledAt`）。

### 7.2 登录限流器只认 `email` 字段

`LoginLimiter.Middleware` 从请求体 JSON 读登录标识作为"账号桶" key（`login_limiter.go`）。`LoginInput.Email` 文档约定"可携带用户名或邮箱"，前端 `login()` 也把用户名/邮箱都放进 `email` 字段发出，因此当前账号级失败锁定**实际生效**。历史隐患在于限流器 identifier 曾与 `email` 单字段硬耦合：一旦未来改成独立 `username` 字段登录，账号桶 key 会取空、账号级锁定失效。**已加固（2026-10-14）：限流器改为 `email` 优先、回退 `username`，二者任一非空即作 key；补 `TestLoginLimiterUsernameFieldFallback` 单测锁定该行为。**

### 7.3 SM.MS token 服务端持有（#7）

`/api/upload` 优先用 `cfg.SmmsToken`（`.env` 的 `SM_MS_TOKEN`），回退到站点设置 `img_host_token`。两者都无 token 且模式为 `smms` → 返回 **503**（前端提示管理员先配置），避免静默落到本地。前端已不再存 `smms_token` 到 localStorage。

### 7.4 匿名读接口 401 陷阱（第 22 轮修过，易复发）

读接口（`/api/posts`、`/api/tags`、`/api/categories`、`/api/posts/recent` 等）用的是 `WithOptionalAuth`：无 token 按匿名放行，有**坏/过期 token 仍 401**。曾因误套强鉴权导致未登录访客 401、前端重试又 429。改路由时**读接口务必保持 `withOptionalAuth`，不要换成 `withAuth`**。

### 7.5 可见性判定全在 service 层（第 25 轮扩为五档状态）

任何"谁能看谁"的判定都在 `services.ListPostsPage` / `ListArchive` / `GetAuthor` / `GetPostPublic` / `canSeePost`，不在 handler/router。**第 25 轮起文章状态为五值**（`models.PostPublished/PostPrivate/PostPending/PostRejected/PostBanned`），`canSeePost` 是唯一权威判定：

| 状态 | 谁能看 |
|------|--------|
| `published` | 所有人（含匿名） |
| `private` | 作者 + admin + root |
| `pending` | 作者 + admin + root |
| `rejected` | 作者 + admin + root |
| `banned` | **仅 root**（作者本人和其他 admin 都看不到） |

遗留 `draft`/`scheduled` 值在 `CreatePost`/`UpdatePost` 一律白名单兜底。改可见性逻辑只看 service，不要往 handler 里加权限判断。

### 7.7 三层角色：root 唯一且不可经 API 创建 ⚠️

- **root 全库有且仅有一个**，且**任何 API 路径都不能把用户提升为 root**：`UpdateUser` 对 `Role == RoleRoot` 直接 403，`CreateUser` 同样拒绝。唯一产生 root 的两条路：① 首启 `seedDemoData` 直接建 root；② 迁移 `0003_root_migration` 把最早 admin 升 root（多 root 则保留最早、其余降 admin）。
- **测试 fixture 要造 root/admin，必须直改 DB**（`db.Model(&models.User{}).Update("role", ...)`）——这正是 `handlers` 包 `setup(t)` 返回 `(*services.Service, *gorm.DB)` 的原因。
- `admin` 不能提权（不能把任何人改成 admin 之上），`root` 可把 user 提为 admin。
- 权限 helper 集中在 `internal/services/roles.go`（`IsRoot`/`IsAdminOrRoot`/`canManageUser`/`canReviewPost`），**不要再写散落的 `me.Role != "admin"` 字符串比较**。
- 前端后台页面的角色门槛需写成 `role !== 'admin' && role !== 'root'`（`AdminGuarded`、各 admin page）。

### 7.6 前端 token 自愈

`web/src/core/auth.tsx` 启动时 `GET /api/auth/me` 复核 token；`localStorage` 写死 token（#4）。JWT 1h 过期后无自动刷新——到期需重新登录（当前设计，非 bug）。

---

## 8. 已知问题 & 待修复清单

| 等级 | 问题 | 位置 / 状态 |
|------|------|------------|
| 🟡 | 编辑器草稿仅 localStorage，不跨设备 | `PostEditor.tsx`（30 天 TTL，可接受） |
| 🟡 | 第 25 轮未写独立功能 spec（用户口述需求直接实现） | `docs/specs/` 暂无 21 号；规则与端点已记入 HANDOVER §5.3/§7.7，建议后补 spec |
| 🟢 | 登录限流器账号桶 key：`email` 优先、回退 `username`（7.2 已加固） | `login_limiter.go` + 单测 |
| 🟢 | `/archive` 一次拉全部 published（已剥 body） | `ListArchive`，已知接受；量大改按年分页 |
| 🟢 | 全文搜索 LIKE `%...%`，千级后变慢 | 纯 Go 驱动无 FTS5，暂保留 |
| 🟢 | GORM `Select("COUNT(*)")` 污染后续 scope | 已修，需警惕同类写法 |
| 🟢 | JWT_SECRET 生产必须覆盖 | `.env.example` 已注明；`ValidateSecurity` 已校验 |
| 🟢 | 审核/封禁无邮件服务时的通知 | `mail.go` 为 best-effort（SendMail 失败不阻断审核），已接受 |

---

## 9. 未决决策（需要你拍板或继续）

> 2026-10-15 收敛：原「生产化优先级」已由第 24 轮（容器化）部分回答；「第 25 轮是否补 spec」已补齐（`specs/21-roles-moderation.md`）。当前真正未决的只剩下面 3 条。

1. **`internal/db` 是否拆分**（ARCHITECTURE §7 曾画独立包）：当前 `config.InitDB` 把"配置+建库+迁移+种子"揉在一起。低配机场景下"揉一起"更直观，**倾向暂不拆**；等真要上 PostgreSQL 再按 §7 拆。请确认。
2. **生产部署验证**：`deploy/docker/` 已就绪但**未 docker build、未在真实 VPS 端到端跑过**（开发机无 Docker）。下一步需要一台 1 vCPU VPS 实测，或先在开发机装 Docker 做镜像构建验证。
3. **PWA / Service Worker 离线化**：列入后续方向，未实现。

> 框架选型已定稿不换：后端 Go 标准库 + GORM + 纯 Go SQLite、前端 Preact（见 `docs/decisions/` 与第 22 轮评审）。**不要为了"更现代"改框架**（除非有具体性能/功能瓶颈证据）。

---

## 10. 文档索引

| 文档 | 内容 |
|------|------|
| `docs/README.md` | **文档体系入口**：specs / decisions / 设计系统 / 问题清单的分工与索引 |
| `docs/ARCHITECTURE.md` | 设计稿（目标架构、技术选型、中间件链、审计指引、已知陷阱引用） |
| `docs/AUDIT_REPORT.md` | 安全审计与 10 项加固记录（2026-09-24 轮） |
| `docs/TODO.md` | **唯一权威待办清单**（P1/P2/P3 + 功能缺口 + 设计取舍；当前仅「评论」未做） |
| `docs/specs/` | 21 份功能 spec（01–21，13 号跳空；21 = 第 25 轮角色/审核/封禁库） |
| `docs/decisions/` | 重大决策：0001 纯 Go SQLite 无 FTS5、0002 localStorage 草稿 TTL、0003 token 存储、0004 CORS |
| `docs/PROJECT_OVERVIEW.md` | 代码地图：路由 / API 端点 / 页面 / 数据模型 / 7 条改码红线 |
| `docs/DESIGN_SYSTEM.md` | 全站 UI 唯一标准（改界面前先读） |
| `docs/archive/` | 已归档快照（`2026-10-14-FUNCTIONAL_AUDIT.md` 首轮审计，结论已落入 TODO.md） |
| `PROGRESS.md` | 25 轮开发全记录 + 风险 / 方向 / 验证基线 |
| `README.md` | 快速上手（运行/配置/演示账号） |
| `HANDOVER.md` | 本文（交接总览） |
| `deploy/` | VPS 部署脚本 |

---

## 11. 接手自测清单（建议你跑一遍再动手）

1. `go build ./...` ✅ / `go vet ./...` ✅ / `go test ./...` ✅（14 测试文件全绿）
2. `cd web && npx tsc --noEmit` ✅ / `npm run build` ✅
3. 双击 `EchoAPI.bat` → 浏览器开 `http://localhost:5173` → 首页/列表/归档/作者页能加载（无 500）
4. 用首启随机密码登录 admin → 自动跳 `/change-password` → 改密成功
5. 连续 5 次错误密码登录 → 第 6 次应 429 + `Retry-After` 头
6. 管理页 → 备份导入 URL 加 `?dry_run=1` → 返回统计不落库
7. `APP_ENV=prod` + 弱 `JWT_SECRET` 启动 → 应 `log.Fatalf` 拒绝（安全校验生效）
8. 把窗口拖窄到 ≤768px → 用户菜单从 hover 浮层变 overlay+tap（#10）

若以上任一不过，先对照 §7 陷阱，再回到对应源文件。大部分"500 / 启动崩溃"都来自 §7.1 的 AutoMigrate 门控被改动。
