> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# LopezPress（原 Echo Content API）— 交接文档（AI 接手用）

> 版本：v2.1
> 更新时间：2026-10-22（第 38 轮文档校准）
> 用途：**新接手的 AI / 工程师的第一份必读**。本文档覆盖项目全貌、当前实现状态、如何跑起来、关键陷阱、已知问题、未决决策，以及自测清单。
> 配套文档：`docs/ARCHITECTURE.md`（设计稿）、`docs/AUDIT_2026-10-15.md`（最近一次全栈深度审计）、`PROGRESS.md`（第 1–38 轮开发全记录）、`README.md`（快速上手）、`docs/specs/*`（20 份功能规格 + 模板）、`docs/decisions/*`（4 项重大决策）。
> **AI 协作者另读**：根目录 `AGENTS.md`（速查 + 红线）与 `docs/AI_STANDARD.md`（五阶段工作流 + 验证门禁 + 十条红线）——2026-10-19 建立的协作标准，本文件 §7 的陷阱是它的红线来源。

---

## 0. 先读这一段（最重要）

- **技术方向已定稿，不需要换框架。** 后端 Go 标准库 `net/http` + GORM + 纯 Go SQLite；前端 Preact + Vite + TS。目标机型是低配 VPS（1 vCPU / 1.5 GB / 5 GB 磁盘）。这套组合在"低资源 + 可维护"维度是最优解，不要为了"更现代"去改框架（详见 §9 决策记录）。
- **当前代码已落地第 1–38 轮开发 + 10 项安全加固 + 7 项 P0 修复**，全部 `go build` / `go vet` / `go test` / `tsc --noEmit` / `vite build` 通过（2026-10-22 第 38 轮本机实测，基线见 `PROGRESS.md` §七）。
- **有一个会反复咬人的陷阱**：`glebarez/sqlite` 驱动 + GORM `AutoMigrate` 在"误判需要重建表"时会走 copy-rename 路径，生成的 `INSERT` 可能漏掉 NOT NULL 列导致启动崩溃。修复方式见 §7.1，改 `internal/config/config.go` 时务必小心，**不要回退成无条件的 `AutoMigrate`**。
- **AI 协作先读 `AGENTS.md` + `docs/AI_STANDARD.md`**：本仓库有一整套驱动 AI 干活的标准（五阶段工作流、验证门禁、十条红线、提示词模板库 `docs/prompts/`）。新接手的 AI 按它执行，用户说「继续」即触发「读上下文 → 选下一优先级 TODO → 实现 → 门禁 → 记录」循环。
- **本地一键启动用 `LopezPress.bat`**（Windows）。它已检测端口、避免重复启动、自动开浏览器。服务器进程请用「独立窗口 / 后台任务」方式运行，否则对话会话结束会被回收（见 §3）。
- **首启账号是唯一 root（用户名 `admin`）**，密码随机生成只在 stderr 打印一次；登录后会被强制跳到 `/change-password` 改密（除非你用 `ADMIN_INITIAL_PASSWORD` 固定）。⚠️ root 密码丢失**没有 HTTP 恢复路径**（admin 重置 root 会被 403 拦下，刻意如此），改走离线命令 `lopezpress -reset-root-password`（随机新密码 + 强制改密 + 吊销旧会话，见 `docs/specs/22-root-password-recovery.md`）。

---

## 1. 项目一句话

面向低配 VPS 的**可自托管全栈内容站点**：Go 单二进制 + SQLite 单文件（后端），Preact 单页（前端），前后端分离。定位"小而美的个人博客 / 轻 CMS"，功能深度优先于广度，追求极致资源占用比与可维护性。

- 单二进制（Go，零 CGO）+ 单文件数据库（SQLite）= 部署即解压，无系统依赖。
- 前端静态产物（~180 kB JS / ~48 kB CSS gzip 后 ~56 kB / ~8.8 kB），放 Nginx 任意静态目录即可。
- 总依赖极少：后端 `go.mod` 直接依赖 6 个（+10 indirect）、前端 3 个 npm 包（`preact` / `wouter` / `marked`），供应链可审计。

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

### 3.1 本地开发（Windows，推荐 `LopezPress.bat`）

项目根目录的 `LopezPress.bat` 已做好：
- **幂等**：启动前检测 `:8080` / `:5173` 是否已在监听，已运行就跳过，不重复拉起。
- 后端优先用现成 `lopezpress.exe`（`start /MIN` 独立窗口），否则回退 `go run ./cmd/server`。
- 前端 `npx vite --port 5173`。
- 启动完自动打开浏览器到 `http://localhost:5173`。

子命令：

```bat
LopezPress.bat            :: 启动后端 + 前端，开浏览器到 :5173
LopezPress.bat status     :: 显示 :8080 / :5173 是否在监听
LopezPress.bat stop       :: 杀掉两个服务
LopezPress.bat restart    :: 先 stop 再 start
LopezPress.bat dev        :: 只起前端 vite
LopezPress.bat build      :: vite build 打包到 web/dist
LopezPress.bat help       :: 查看子命令
```

> 双击即用。Go 路径脚本会自动探测 `PATH` → `%USERPROFILE%\go-install\go\bin` → `C:\go\bin`。

### 3.2 手动启动（等价于 LopezPress.bat 内部逻辑）

**后端**（需要 Go 1.22+，脚本里 Go 在 `C:\Users\Lopez\go-install\go\bin`）：

```powershell
cd lopezpress
go run ./cmd/server          # 或：go build -o lopezpress.exe ./cmd/server && .\lopezpress.exe
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

> ⚠️ **VPS 运维陷阱（踩过）**：之前用"会话后台任务"拉起 `go run` 进程，对话轮次切换/电脑重启后进程被回收，`:8080` 不监听，前端 vite 代理打不通后端，浏览器看到所有 `/api/*` 全是 500（实为 502 被代理层包成 500）。**长期运行请用 `LopezPress.bat`（独立窗口）或 `deploy/` 的 systemd / nohup，不要依赖对话内后台进程。**

### 3.3 生产部署

见 `deploy/`：`build.sh`（交叉编译 Linux）、`deploy-to-vps.sh` / `deploy-to-vps.ps1`（一键上传）、`lopezpress.service`（systemd）、`lopezpress.env`（生产环境变量）、`nginx.conf`（反代 + 前端静态托管）、`install-systemd.sh`、`LOCAL.md`、`README.md`。生产务必设置：强 `JWT_SECRET`、`CORS_ORIGINS` 显式列表、`APP_ENV=prod`、HTTPS。

---

## 4. 如何测试 / 验证

```powershell
# 后端
cd lopezpress
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
lopezpress/
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
├── LopezPress.bat               # 一键启动（Windows）
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

### 7.8 浏览量只在显式上报里加，读接口是纯读（第 43 轮，spec 23）

- `GetPost` / `GetPostPublic` **不再**修改 `view_count`。计数唯一入口是 `POST /api/posts/{id}/view` → `services.ReportView`。
- 去重的**唯一机制**是 `post_views` 上的 `UNIQUE(post_id, visitor_hash, day)` + `INSERT ... ON CONFLICT DO NOTHING` 的 `RowsAffected`。**不要**改成"先查再插"或加应用层 if——并发下会重复计数。
- 指纹 = `sha256(ip|ua|JWT_SECRET)`，**改 `JWT_SECRET` 会让当天已计数的访客再计一次**（可接受，但要知道）。
- 新增文章状态或改可见性时注意：只有 `published` 可计数，其余状态 `ReportView` 返回 404（与公开读一致，避免泄露存在性）。
- 前端 `ReadPage` 在 `status === 'published'` 时上报一次；**不要**在列表页/预览页也上报。

### 7.9 客户端 IP 只认可信代理（第 43 轮，修 P2-6）

- 所有 IP 获取统一走 `internal/clientip.Client(r)`（`middleware` 与 `handlers` 里曾各有一份会漂移的实现，已合并）。
- 只有 `RemoteAddr` 命中 `TRUSTED_PROXIES`（默认仅回环）才采信 `X-Forwarded-For`；否则忽略。**部署到 VPS 时必须设 `TRUSTED_PROXIES`**，否则 nginx 后面的 IP 全是 `127.0.0.1`，浏览量去重会退化成"全站一天一次"。
- 非法配置在 `main` 启动时 `log.Fatalf`，不会静默降级。

### 7.10 每页条数由服务端决定（第 43 轮，spec 24）
- 公共列表前端**只传 `page`，不传 `size`**；`ListPostsPage` 在 `PageSize < 1` 时读 `site_config.page_size`（`ClampPageSize` 钳到 5/10/20/30）。
- `site_configs.page_size` 是**手工迁移**加的列（`0005_add_page_size`，`ALTER TABLE`），因为该表早已存在、不能交给 AutoMigrate（见 §7.1）。
- 别为了"灵活"让前端传 size：`?page=N` 会对不同人含义不同，链接就不能分享了。

### 7.11 批量操作必须复用单条 service 方法（第 44 轮，spec 26）

- `BatchUpdatePosts`（`services/batch.go`）**逐条调用 `UpdatePost` / `DeletePost`**，刻意不写第二套权限判断。新增批量动作时也照此办理——一旦在批量里手写 `if me.Role == ...`，就会绕过 `roles.go` 与状态机，重演第 25 轮的 P0 事故模式。
- 批量返回 `affected` + `skipped[]`（带英文原因），**部分成功是设计而非 bug**；只有基础设施级错误（空 ids / 未知动作 / >100 条 / admin 越权发布）才整体 4xx。
- `publish` 仅 root；admin 走 `POST /api/admin/review/{id}`。别在批量里给 admin 开口子。
- `add_tags` 交给 `UpdatePost` 前必须做**集合运算**：`UpdatePost` 在 `in.Tags != nil` 时**整体替换**标签，直接把增量传进去会静默删掉原有标签。

### 7.12 新加的模型字段必须配套手工迁移（第 46 轮，血泪）

- **已存在的表永远不会被 AutoMigrate 碰**（§7.1 的代价）。所以给模型加字段后，如果不同时写一条 `ALTER TABLE ADD COLUMN` 的手工迁移，**这一列在旧库里根本不存在**，错误只会在调用到它的那个接口上以 500 暴露。
- 第 46 轮就是踩了这个：`site_configs.site_url` 从来没有迁移，**保存站点设置一直 500**，因为没人点过保存所以一直没发现。修法：迁移 `0008_add_site_url`。
- 防线：`config.checkSchemaDrift` 在每次启动时用反射把模型字段与真实表结构比对，缺列就打
  `WARN schema drift: <表>.<列> is missing`。**看到这条日志不要忽略**，它意味着某个功能已经坏了。
- 加字段的标准动作：改 `models/*.go` → 在 `runManualMigrations` 里加 `addXxxColumn`（`schema_migrations` 门控 + `HasColumn` 判断 + 新库直接 `recordMigration`）→ 在测试库/真库各启动一次看是否还有 WARN。

### 7.13 阅读开关与分类树（第 46 轮，spec 27）

- 匿名读的**唯一权威**是 `site_configs.public_read`，由 `WithOptionalAuth` 强制（关闭时匿名一律 401）。前端 `Readable` 守卫只是 UX，不是安全边界——**不要**在 handler 里另写判断。
- 该值在 `Service` 里有内存缓存，**改配置必须走 `UpdateSiteConfig`**（它会 `invalidatePublicRead`）；直接 SQL 改库不会立刻生效，需重启。
- 分类**只允许两层**：父级不得再有父级。`validateCategoryParent` 同时做了存在性、自引用、三层和环路四道检查。删父级时子级上移到祖父级。
- 按父级筛选会**包含子级文章**（`categoryWithDescendants`）；不要改回 `category_id = ?`，否则父节点永远显示为空。

### 7.14 原生 SQL 不认 `*int64`（第 52 轮）

- `posts.category_id` 是 `*int64`。直接把它当参数传给 `db.Raw(...)` 会在运行时报
  `sql: converting argument ... unsupported type *int64`，因为 `database/sql` 的 driver.Value 白名单里没有指针。
- 正确写法：先解包 `var cat any; if post.CategoryID != nil { cat = *post.CategoryID }`，再传 `cat`。
- `nil` 传进去是合法的（匹配 `IS NULL` 的语义），所以"无分类"不需要特判 SQL 字符串。
- 相关文章（spec 35）就是这条的现场：ORM 一切正常，只有 `Raw` 那一条会炸。

### 7.15 整表单保存会冲掉你没提交的字段（第 52 轮）

- 站点设置是「GET 全量 → 前端持有 → PUT 全量」，`SiteConfigFromMap` 对**缺失字段取零值**。
- 于是：root 打开了每日备份，admin 在同一个页面点保存（他的 payload 里没有 `auto_backup`）→ 备份被静默关掉。
- 修法两层：① 前端只对 root 渲染该开关；② **服务端**在 `UpdateSiteConfig` handler 里，非 root 时显式继承已有的 `auto_backup` / `backup_keep`。
- 通用教训：任何"整对象 PUT"的接口，凡有字段只允许特定角色改，都必须在服务端做继承，不能只靠前端不渲染。

### 7.16 每日快照的三个隐含约定（第 52 轮，spec 34）

- **文件名即状态机**：`auto-YYYYMMDD.db` 存在就代表"今天备过了"。不要引入"last_backup_at"字段——文件和状态会不一致，而文件才是真的。
- **前缀必须与导入快照分开**（`auto-` vs `snapshot-`）：否则上午做过一次导入，当天的自动备份就会误判为已完成。
- **淘汰只碰自己创建的文件**：`pruneAutoBackups` 只删 `auto-*`；`pruneSnapshots` 只删 `snapshot-*`。混在一起删会删掉"导入前那份救命快照"。
- 定时器是 30 分钟一次的 `stat`，不是 cron。进程没跑的那天不会补，**保证是"每天最多一次"而不是"每天正好一次"**——这个语义要跟用户说清楚。

---

### 7.17 Preload 出来的关联对象会整份序列化（第 72 轮，HIGH 级泄漏）

- `Post.Author *User` 一旦被 `Preload("Author")` 加载，`models.User` 的 **JSON tag 会连同 `email` / `last_login_ip` / `last_login_at` / `disabled` / `must_change_password` / `token_version` 一起吐给前端**。匿名 `GET /api/posts` 曾经就是这样泄露全站作者账号信息。
- 修法：`models.Post.SanitizeAuthor()` 复制一份再置空，**必须复制**——一次 Preload 可能把同一个 `*User` 指针挂到多篇文章上，原地改会跨文章静默污染。
- 完整覆盖的 9 个出站口见 `PROGRESS.md` 第 72 轮；**唯一例外**是 `ListPending`（审核队列要 email 识别作者，且 admin 限权）。
- 新增任何"带 author 的返回路径"，**先想想它是否会绕过裁剪**。回归测试：`services/author_sanitize_test.go`。

### 7.18 改密码必须 bump `token_version`，但要回发新 token（第 72 轮）

- 只改 `password_hash` 不改 `token_version` = **旧 token 永久有效**（改密码毫无安全收益）。
- 但只 bump 不回发新 token = 用户改完自己的密码当场被踢下线（体验倒退）。
- 正确做法：`bump` + 用新版本 `SignToken` 签发，响应里带 `{token, expires}`，前端 `adoptToken()` 写回 localStorage 再 `getMe()` 重校验。`ResetPassword` 走另一条：只 bump，目标用户下次登录自然失效。

### 7.19 上传必须验内容，不能只验扩展名（第 72 轮）

- 旧实现只查后缀 ∈ `{png,jpg,jpeg,gif,webp}`，`<html><script>alert(1)</script></html>` 改名 `.png` 直接落 `data/uploads/` 并被 nginx 静态托管 → 存储型 XSS 素材。
- 用标准库 `http.DetectContentType`（前 ≤512 字节 magic bytes）拦，**不引入图片解码库**（守住「6 个直接依赖」）。仅允许 `image/png|jpeg|gif|webp`。
- nginx 侧 `X-Content-Type-Options: nosniff` 是第二道闸，两道都要有。

### 7.20 nginx `add_header` 不继承——安全头会凭空消失（第 72 轮）

- `add_header` 在 `location` 里一旦出现，**server 级声明的所有安全头在该 location 内全部失效**（nginx 的继承规则，不是 bug）。
- 本仓库 `deploy/nginx.conf` 的静态资源 location 因此需要**重复声明** 4 个安全头；同理 SPA fallback 会把 `/uploads/` `/feed` `/sitemap.xml` `/robots.txt` 全吃成 `index.html`，必须各自 `location` 反代到 8080。
- 结论：**改 nginx 后不能只看浏览器，要 `curl -I` 逐个 location 验头**。

---

## 8. 已知问题 & 待修复清单

| 等级 | 问题 | 位置 / 状态 |
|------|------|------------|
| 🟡 | 普通用户编辑器暴露「公开/私密」两个 radio，但 `CreatePost` 对非 root 一律强制 `pending` | `PostEditor.tsx` + `services/posts.go`；用户以为存成私密、实则进审核，通过后变公开。**待拍板**：隐藏该选项 or 明确提示「先过审核」 |
| 🟢 | `PostEditor` 把后端返回的 `pending` 误显示为「公开」 | `PostEditor.tsx` 初始化逻辑 `status==='private'?'private':'published'`，**待拍板** |
| 🟡 | 编辑器草稿仅 localStorage，不跨设备 | `PostEditor.tsx`（30 天 TTL，可接受）；服务端修订历史只管已保存版本 |
| 🟡 | 第 25 轮未写独立功能 spec（用户口述需求直接实现） | `docs/specs/` 暂无 21 号；规则与端点已记入 HANDOVER §5.3/§7.7，建议后补 spec |
| 🟢 | 登录限流器账号桶 key：`email` 优先、回退 `username`（7.2 已加固） | `login_limiter.go` + 单测 |
| 🟢 | `/archive` 已改服务端分页（第 47 轮，50/页） | `ListArchivePage`；旧全量端点保留兼容 |
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
3. 双击 `LopezPress.bat` → 浏览器开 `http://localhost:5173` → 首页/列表/归档/作者页能加载（无 500）
4. 用首启随机密码登录 admin → 自动跳 `/change-password` → 改密成功
5. 连续 5 次错误密码登录 → 第 6 次应 429 + `Retry-After` 头
6. 管理页 → 备份导入 URL 加 `?dry_run=1` → 返回统计不落库
7. `APP_ENV=prod` + 弱 `JWT_SECRET` 启动 → 应 `log.Fatalf` 拒绝（安全校验生效）
8. 把窗口拖窄到 ≤768px → 用户菜单从 hover 浮层变 overlay+tap（#10）

若以上任一不过，先对照 §7 陷阱，再回到对应源文件。大部分"500 / 启动崩溃"都来自 §7.1 的 AutoMigrate 门控被改动。
