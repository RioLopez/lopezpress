> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。

# LopezPress（原 Echo Content API）— 交接文档（AI 接手用）

> 版本：v3.0
> 更新时间：2026-10-24（第 79 轮文档校准）
> 用途：**新接手的 AI / 工程师的第一份必读**。本文档覆盖项目全貌、当前实现状态、如何跑起来、关键陷阱、已知问题、未决决策，以及自测清单。
> 配套文档：`docs/ARCHITECTURE.md`（设计稿）、`docs/AUDIT_2026-10-15.md`（最近一次全栈深度审计）、`PROGRESS.md`（第 1–79 轮开发全记录）、`README.md`（快速上手）、`docs/specs/*`（36 份功能规格 + 模板）、`docs/decisions/*`（6 项重大决策）。
> **AI 协作者另读**：根目录 `AGENTS.md`（速查 + 红线）与 `docs/AI_STANDARD.md`（五阶段工作流 + 验证门禁 + 十条红线）——2026-10-19 建立的协作标准，本文件 §7 的陷阱是它的红线来源。
> **只想恢复上下文**：直接读 `MASTER.md`（唯一入口 + 自驱协议 + 事实基线），不必逐份翻。

---

## 0. 先读这一段（最重要）

- **技术方向已定稿，不需要换框架。** 后端 Go 标准库 `net/http` + GORM + 纯 Go SQLite；前端 Preact + Vite + TS。目标机型是低配 VPS（1 vCPU / 1.5 GB / 5 GB 磁盘）。这套组合在"低资源 + 可维护"维度是最优解，不要为了"更现代"去改框架（详见 §9 决策记录）。
- **当前代码已落地第 1–79 轮开发**（含 10 项安全加固、7 项 P0 修复、8 阶段架构债、图库与邮箱验证两族新功能），全部 `go build` / `go vet` / `go test` / `tsc --noEmit` / `vite build` 通过（2026-10-24 第 79 轮本机实测，基线见 `PROGRESS.md` §七与 `MASTER.md` §7）。
- **有一个会反复咬人的陷阱**：`glebarez/sqlite` 驱动 + GORM `AutoMigrate` 在"误判需要重建表"时会走 copy-rename 路径，生成的 `INSERT` 可能漏掉 NOT NULL 列导致启动崩溃。修复方式见 §7.1，改 `internal/config/config.go` 时务必小心，**不要回退成无条件的 `AutoMigrate`**。
- **AI 协作先读 `AGENTS.md` + `docs/AI_STANDARD.md`**：本仓库有一整套驱动 AI 干活的标准（五阶段工作流、验证门禁、十条红线、提示词模板库 `docs/prompts/`）。新接手的 AI 按它执行，用户说「继续」即触发「读上下文 → 选下一优先级 TODO → 实现 → 门禁 → 记录」循环。
- **本地一键启动用 `LopezPress.bat`**（Windows）。它已检测端口、避免重复启动、自动开浏览器。服务器进程请用「独立窗口 / 后台任务」方式运行，否则对话会话结束会被回收（见 §3）。
- **首启账号是唯一 root（用户名 `admin`）**，密码随机生成只在 stderr 打印一次；登录后会被强制跳到 `/change-password` 改密（除非你用 `ADMIN_INITIAL_PASSWORD` 固定）。⚠️ root 密码丢失**没有 HTTP 恢复路径**（admin 重置 root 会被 403 拦下，刻意如此），改走离线命令 `lopezpress -reset-root-password`（随机新密码 + 强制改密 + 吊销旧会话，见 `docs/specs/22-root-password-recovery.md`）。

---

## 1. 项目一句话

面向低配 VPS 的**可自托管全栈内容站点**：Go 单二进制 + SQLite 单文件（后端），Preact 单页（前端），前后端分离。定位"小而美的个人博客 / 轻 CMS"，功能深度优先于广度，追求极致资源占用比与可维护性。

- 单二进制（Go，零 CGO）+ 单文件数据库（SQLite）= 部署即解压，无系统依赖。
- 前端静态产物（JS 261 kB / CSS 67 kB，gzip 后 ~81 kB / ~12 kB），放 Nginx 任意静态目录即可。
- 总依赖极少：后端 `go.mod` 直接依赖 7 个（+11 indirect）、前端 3 个运行依赖（`preact` / `wouter` / `marked`）+ 4 个 devDependency，供应链可审计。

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
| 图片 | `golang.org/x/image`（`draw` 缩放 + `webp` 解码） | 第 78 轮新增；缩略图队列用 CatmullRom，上传校验仍走魔数 |
| 图像存储 | 本地磁盘 + `upload_files` 元数据表 | sha256 去重、三层配额、软删撤下（文件仍留盘）、引用台账，见 spec 36 |
| 邮件 | `net/smtp` 纯标准库 | 通知类「未配 SMTP 即静默跳过」；验证码类必须 503 报错，见 spec 37 |
| 可见性模型 | `published`（公开）/ `private`（仅作者） | 无 draft / 无 scheduled（第 22 轮定稿移除）；`pending`/`rejected` 是审核态 |
| 角色 | `user` / `admin` / `root` 三层 | root 全库唯一且**不可经任何 API 创建**（第 25 轮起）；权限 helper 集中在 `services/roles.go` |

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

> 双击即用。Go 路径脚本会自动探测 `PATH` → `%USERPROFILE%\go-install\go\bin` → `%SystemDrive%\go\bin`。
> **本机实际路径是 `C:\Users\LopezPC\go-sdk\go\bin`，不属于上面三个位置**，所以要么手动 `set PATH=C:\Users\LopezPC\go-sdk\go\bin;%PATH%`，要么依赖已构建好的 `lopezpress.exe`（脚本优先用它）。

### 3.2 手动启动（等价于 LopezPress.bat 内部逻辑）

**后端**（需要 Go 1.22+，本机 Go 在 `C:\Users\LopezPC\go-sdk\go\bin`）：

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
go test -count=1 ./internal/...   # 单测（48 个 _test.go 文件 / 212 用例，全绿）
go test ./internal/services/ -run Visibility -v   # 可见性三分支等

# 前端
cd web
node node_modules/typescript/bin/tsc --noEmit   # 类型检查（.bin/tsc 是 bash 脚本，别直接喂 node）
NODE_OPTIONS= npm test     # vitest，5 文件 43 用例（沙箱里 NODE_OPTIONS 注入 fs shim 会让 vitest 报 EPERM，故清空）
npm run build              # 生产构建

# 门禁（改代码 / 改文档后各跑对应的）
python scripts/arch_check.py      # 架构棘轮，只许降不许升
python scripts/contract_check.py  # 前后端 method+path 契约
python scripts/doc_check.py       # 母文档是否失真
```

测试文件清单（完整清单以 `ls internal/**/*_test.go` 为准，当前 48 个）：
- `internal/config/`：`config_test.go`、`mig_test.go`、`recover_test.go`
- `internal/handlers/`：`handlers_test.go`、`upload_test.go`、`mail_test.go`、`moderation_test.go`、`query_test.go`、`guard_test.go`、`feed_test.go`
- `internal/middleware/`：`limiter_test.go`、`login_limiter_test.go`、`middleware_test.go`
- `internal/models/`：`models_test.go`
- `internal/services/`：`services_visibility_test.go`（可见性五态）、`services_backup_test.go`、`media_library_test.go`（去重/配额/软删/台账）、`thumbs_test.go`（缩放/透明/skip）、`email_codes_test.go`（验证码全链路）、`update_me_test.go`、`testutil_test.go` 等

---

## 5. 当前实现状态（截至 2026-10-24 第 79 轮）

### 5.1 功能轮次（PROGRESS.md 完整记录，这里只列结论）

第 1–79 轮已全部完成。第 1–25 轮见下方原有记录，第 26 轮之后按主题归类：

| 区间 | 主题 | 结论 |
|---|---|---|
| 26–42 | AI 协作体系与母文档 | `AGENTS.md` / `docs/AI_STANDARD.md` / `docs/prompts/` / `MASTER.md` / `scripts/doc_check.py`；第 40 轮首次推送 GitHub |
| 43–52 | 读者端 + 写作端 + 运营端补全 | 浏览量去重、后台文章管理、阅读开关、分类层级、slug 永久链接、买家企业审查、修订历史、批量导入、统计趋势、每日自动备份、相关文章 |
| 53–67 | 架构债（`ARCH_REVIEW` 8 阶段） | 棘轮门禁 `arch_check.py`；审计单点化；拆 `services.go`；路由表声明化；迁移注册表；前端数据层 `core/http.ts` + `useQuery`；i18n 按域拆；DTO 全进 `dto.go`；查询参数统一。**R1/R3/R4/R5/R7/R8/R9/R10 全 0** |
| 68–75 | 真机走查 + 安全收口 | wouter query 失效、越权渲染、硬刷新掉登录；外部渗透报告两轮核实（9 真 + 8 类真问题），权限面/上传/备份/安全头全部收紧 |
| 76–79 | 媒体与身份（本轮） | 图库（去重/配额/引用台账/软删/后台/缩略图队列）+ 头像裁剪 + 邮箱验证（注册码/找回密码/换绑，默认关闭） |

### 5.1.1 图库与存储（第 77–78 轮，spec 36）

- 表：`upload_files`（`live_key` 唯一索引：活=sha256、软删=`d:<id>`、历史=`legacy:<name>`）+ `upload_refs`（文章引用台账）。
- 上传：magic-bytes → 真实解码（边长/像素上限）→ sha256 去重 → 三层配额 → 写盘。文件名 `crypto/rand` 22 字符 base62。
- 删除：用户「撤下」= 软删（文件仍在盘上继续服务已发布文章，但离开图库且**永不可被他人复用**）；admin 可硬删（连带缩略图），返回受影响引用数。
- 缩略图：库内 `thumb_status` 是唯一待办真相；单 goroutine + 150 ms 间隔 + 30 s sweep；长边 480，CatmullRom，透明走 PNG。
- 前端：`/admin/media` 图库页；`/settings` 头像点击 → `AvatarCropper`（拖拽 + 缩放，输出 512×512 WebP）。

### 5.1.2 邮箱验证（第 79 轮，spec 37，**实验性默认关闭**）

- 三条流程：注册码（`/register`）、找回密码（`/forgot-password`）、换绑邮箱（`/settings`）。
- 安全：6 位码 5 次错误作废、一次性、绑用途、库里只存 HMAC；同邮箱 60 s 冷却（落表）+ 同 IP 每小时 10 封。
- **开着验证时 `PUT /api/auth/me` 改邮箱直接 403**（否则一次 PUT 绕过整套）。
- 开关 root 专属；SMTP 未配置时接口 503 而不是假装发送成功。

### 5.1.3 第 23–25 轮（历史记录，保留原文）

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

### 5.2 10 项安全加固（2026-10-15 那轮，已全部完成并通过）

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
├── cmd/server/main.go        # 入口：loadDotEnv → Load → ValidateSecurity → InitDB → 建 svc → 启缩略图 worker → 双限流器 → 优雅关闭
├── internal/
│   ├── config/
│   │   ├── config.go         # .env 读取 + ValidateSecurity + InitDB（AutoMigrate 只对新表）+ 种子数据
│   │   ├── migrate.go        # 有序迁移注册表 0001–0013 + ensureColumns 泛型（R7 门禁禁止另写 add*Column）
│   │   ├── uploads_backfill.go # 启动时把 data/uploads 里的历史文件登记为 legacy 行（幂等）
│   │   └── recover.go        # root 密码离线恢复 CLI
│   ├── models/
│   │   ├── models.go         # User/Post/Category/Tag/PostTag/LoginLog/ActionLog/PostRevision/...
│   │   ├── all.go            # AutoMigrate 的唯一实体清单（含 UploadFile/UploadRef/EmailCode）
│   │   ├── site_config.go    # SiteConfig（最宽的实体，独立成文）
│   │   ├── uploads.go        # UploadFile（live_key 唯一索引）+ UploadRef（引用台账）
│   │   ├── email_codes.go    # EmailCode（只存 HMAC）
│   │   ├── ids.go            # NewOpaqueID：crypto/rand → base62 22 字符
│   │   └── presets.go        # 预设分类树
│   ├── services/             # 领域层（36 个文件，按域切分，最大 314 行）
│   │   ├── posts.go / posts_list.go / posts_edit.go / moderation.go / trash.go / revisions.go / slug.go / tags.go / taxonomy.go / related.go / views.go / batch.go
│   │   ├── media.go / media_files.go / media_library.go / media_refs.go / thumbs.go   # 图库（spec 36）
│   │   ├── email_codes.go / auth_email.go / mail.go / mail_code.go / mail_smtp.go    # 邮件与验证码（spec 37）
│   │   ├── site_config.go / site_config_policy.go / accounts.go / accounts_profile.go / users.go / access.go
│   │   ├── audit.go（全库唯一 ActionLog 写者）/ roles.go（权限 helper 集中地）/ dto.go（全部 API 契约）
│   │   ├── backup.go / restore.go / autobackup.go / import_md.go / nav.go / auth.go / service.go
│   ├── handlers/             # 薄处理器（auth/auth_email/posts/accounts/admin/upload/categories/moderation/import/feed/query）
│   ├── middleware/           # Chain/Logger/CORS/Recover/Limiter/LoginLimiter/SecurityHeaders
│   ├── imghost/              # 出站图床客户端（SM.MS），service 层不再碰 HTTP
│   └── router/router.go      # 路由声明表 `var routes`（83 条，含 auth/roles 列）
├── web/                      # 前端（Preact + Vite + TS，26 页面 + core/{http,useQuery,api,types,i18n,roles,...}）
├── deploy/                   # systemd / Nginx / Docker
├── docs/                     # ARCHITECTURE / specs(36+模板) / decisions(0001-0006) / TODO / 审计归档
├── scripts/                  # arch_check.py / contract_check.py / doc_check.py / fakesmtp.py
├── LopezPress.bat            # 一键启动（Windows）
└── PROGRESS.md               # 79 轮开发全记录
```

**读代码的推荐顺序**：`cmd/server/main.go` → `internal/router/router.go`（**先看路由表，它现在就是端点清单**）→ `internal/services/*`（业务规则全在这）→ `internal/handlers/*`（只做解析与序列化）→ `internal/middleware/*` → `internal/models/*` → `web/src/core/`（`http.ts` 是唯一 fetch，`api.ts` 是纯类型化 facade）→ `web/src/pages/*`。

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

### 7.5 可见性判定全在 service 层（第 25 轮五档 → 第 73 轮收紧「私密」）

任何"谁能看谁"的判定都在 `services.ListPostsPage` / `ListArchive` / `GetAuthor` / `GetPostPublic` / `canSeePost`，不在 handler/router。`canSeePost` 是唯一权威判定（第 73 轮起 `private` 严格作者可见）：

| 状态 | 谁能看 |
|------|--------|
| `published` | 所有人（含匿名） |
| `private` | **仅作者本人**（admin / root 都读不到；「仅自己可见」字面成立） |
| `draft`（第 22 轮已废的遗留值） | **仅作者本人**（与 private 同档） |
| `pending` | 作者 + admin + root（审核流需要） |
| `rejected` | 作者 + admin + root（审核流需要） |
| `banned` | **仅 root**（作者本人和其他 admin 都看不到） |

- **为什么 admin/root 读不到别人的 private**：私密文章永远不会进入公开区，也就没有审核/治理需求；而只要管理员能读，"仅自己可见"就是一句假话。审核流需要看的 `pending`/`rejected` 保持 admin/root 可读。
- 列表过滤（`posts_list.go` 的 `applyFilters`）用同一个 `authorOnly := []string{PostPrivate, PostDraft}` 切片驱动，**改可见性时列表与单篇必须同步改**，否则会出现"列表里有、点进去 404"。
- 后台「文章管理」因此对 admin 少看到别人的 private/draft 行，这是预期行为。
- 遗留 `draft`/`scheduled` 值在 `CreatePost`/`UpdatePost` 一律白名单兜底。改可见性逻辑只看 service，不要往 handler 里加权限判断。

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

### 7.21 可分享的筛选链接用名字，不用自增 id（第 74 轮）

- 首页列表的分类筛选原来是 `?category=2`，同一个筛选里标签却是 `?tag=Go`——两套标识，前者贴出去没人看得懂，后者一眼就明白。
- 现在 `?category=` 收**分类名**（`?category=PHP`），数字 id 继续兼容（旧书签不失效）。解析不在 handler：`internal/handlers/query.go` 的 `parsePostListParams` 只把原始字符串塞进 `ListPostsQuery.Category`，`services/taxonomy.go` 的 `categoryFilterIDs` 才查库。
- **未知分类必须返回空页，不能退化成「不筛选」**：否则 `?category=打错字` 会静默列出全站文章，读者看不出筛选其实没生效。
- 前端改标识时容易漏的三处深层链接：`AdminCategoriesPage` 的计数链接、`ReadPage` 的面包屑、`ListPage` 的 `<select value>`——都要用 `encodeURIComponent(name)`，中文分类名否则会被 URL 编码规则打断。

### 7.22 「层级」不做视觉区分就等于没做（第 74 轮）

- 后台能建两级分类，但侧边栏父子**同为 13px**、只差 `opacity: .9`，`php教程` 看起来就是 `PHP` 的兄弟节点。
- 现在的做法：父级 13.5px + 600 字重 + 正文色；子级 12.5px + muted + 左缩进 24px + `::before` 画的肘形引导线。三处必须同时改——侧边栏、筛选下拉、卡片 chip（`PHP › php教程`）；`<option>` 无法样式化，只能用 `\u3000└ ` 文本缩进。
- 排序逻辑抽到 `web/src/core/taxonomy.ts`（`orderCategories` / `categoryPath` / `categoryOptionLabel`），否则三处各写一遍必然漂移。

### 7.23 标签去重必须大小写不敏感，两端都要（第 74 轮）

- `findOrCreateTagsTx` 原来用 `name = ?` 精确匹配，`go` / `Go` / `GO` 会生成三个标签，同一个概念分裂成三条互不相通的筛选项。
- 查询改 `LOWER(name) = ?`，**先建的拼写胜出**（输入 `go` 复用已有的 `Go`）；`UpdateTag` 重名检查与 `?tag=` 过滤同步改 `COLLATE NOCASE`（SQLite 的 NOCASE / LOWER 只覆盖 ASCII，正好是大小写存在的范围，中文不受影响）。
- 前端补全用 `web/src/core/tags.ts` 的纯函数 `suggestTags`：大小写不敏感、前缀优先、排除已选、无匹配时末尾补「创建标签」行（且**已选的同名不再给 create 行**，否则像是上次没选上）。
- 下拉**不要用 `onBlur` 关**：实测 120ms 定时器会和输入竞态——下一次按键刚把列表打开，定时器随后把它关掉。改用 document `mousedown` + 容器 `contains` 判断。

---

### 7.24 「校验」写成「字段存在才校验」，空载荷就全跳过（第 75 轮）

`validateImportPayload` 原来的三条检查都是 `if in.SchemaVersion != 0 && in.SchemaVersion != X`、`if in.AppVersion != "" && ...`。空对象每条都短路通过，然后进事务清表——一次 `POST /api/backup/import {}` 就抹掉全站。**教训**：校验要写成「必须出现」而不是「出现了就必须对」；整体替换类操作还要有一个不依赖字段的兜底不变式（这里选了「必须至少有一个 root，否则导入后没人能登录也没人能恢复」）。

### 7.25 同一个权限面，列表接口 root-only 但下载接口 admin——等于没挡（第 75 轮）

`/api/backup/snapshots` 是 root-only，`/api/backup/download?file=` 却是 admin：admin 拿不到列表，但知道文件名（`auto-YYYYMMDD.db` 可猜）就能直接下整个 SQLite。**教训**：按「资源」而不是按「接口」定权限，同一个资源的读/写/枚举必须是同一档。同类错误还有 `site-config`（内容字段 admin、`smtp_pass` 是 root 级秘密，却整行返回）。

### 7.26 密码复用会把「泄露一个小秘密」变成 root 接管（第 75 轮）

`site-config` 里的 `smtp_pass` 与 root 登录口令同值，admin 一读 = root。代码层面能做的是**不给低权角色读任何凭据**；运维层面要避免口令复用。脱敏时注意「读屏蔽 / 写继承」要成对：只屏蔽读、不继承写，admin 一保存就会把秘密清空。

### 7.27 恢复备份会复活已撤销的 token（第 75 轮）

快照里带 `token_version`。恢复 = 回到当时的版本号，之前撤销过的 JWT 全部重新有效。修法是恢复事务末尾对全表 `token_version + 1`——**代价是恢复后所有人（含操作者）要重新登录**，这属于正确行为，要写进运维手册。

### 7.28 前端「隐藏按钮」不能替代后端的权限拒绝（第 75 轮）

本轮 4 处改了前端（导出按钮、置顶开关、SMTP 输入框、root 行登录历史），每一处都有对应的服务层 403。反过来说，前端隐藏是为了不提供「点了必失败」的控件，不是安全边界。`arch_check` 的 R4 门禁（handler 里不许 `require*`）保证了路由表这一层不会漏，但**字段级**授权（谁能写 `pinned`、谁能读 `smtp_pass`）只能在 service 里做。

### 7.29 图片的"软删除"不能用 `gorm.DeletedAt`（第 77 轮）

用户撤下自己的图之后，**已经发表的文章里还嵌着这张图**。用 `gorm.DeletedAt` 会让所有查询（包括静态文件服务）都看不到它 → 文章破图 → 让文章作者替上传者受罚。所以 `UploadFile.DeletedAt` 是普通的 `*time.Time`：列表接口自己加 `deleted_at IS NULL`，而 `GET /uploads/{name}` 完全不看这个字段。

### 7.30 去重和所有权是同一件事，要靠数据不变式而不是纪律（第 77 轮）

"用户删了图，就不能再给别人用"——如果靠"查重时记得跳过已删行"，任何一处漏写就侵权。做法是把 `live_key` 做成唯一索引：活跃 = `sha256`，撤下 = `d:<id>`，历史文件 = `legacy:<name>`。**唯一复用入口是按 `live_key` 查**，于是结构上永远查不到已撤下的图；同时文件仍在盘上继续服务老文章。站长原话是"这是所有权问题、是侵权的"——这类要求应该落到 schema 上，不要落到"记得判断"上。

### 7.31 上传的三步顺序不能换：去重 → 配额 → 写盘（第 77 轮）

- 配额在**写盘之前**：否则被拒的请求会留下半截文件，成为永不回收的垃圾。
- 去重在配额**之前**：复用不产生新字节，不该计入额度。
- 由此得出一个测试陷阱：**用同一份字节测配额永远拿不到 413**，它会先去重成功。必须每次换不同的内容。

### 7.32 "缺省 = 关闭"的布尔项和"0 = 不限制"的数值项，必须从原始 map 判断是否出现（第 77/79 轮）

`PUT /api/admin/site-config` 收到的是整份配置。若某个字段没提交，Go 解出来就是 `false` / `0`，直接赋值等于**静默关掉磁盘保护或头像开关**。做法：handler 从 `map[string]any` 判断 key 是否存在，不存在就从库值继承；数值再 `clampInt64` 到安全区间（`upload_max_image` 64 KB–20 MB 等）。同类字段：`allow_register` / `auto_backup` / `smtp_pass` / `email_verify_enabled` / `email_code_*`。

### 7.33 后台 goroutine 的资源上限要比请求路径更严（第 78 轮）

缩略图 worker 在后台解码。上传校验允许 4000 万像素（用户看得见图、能忍），但 worker 只解到 2400 万——因为**后台 goroutine OOM 会杀掉整个进程**，而没有缩略图只是列表慢一点。同理：队列必须单 goroutine + 张与张之间留间隔，1 核机器上并发解码 8 张大图会让 HTTP 服务不可用。

### 7.34 队列的待办状态必须落在数据库（第 78 轮）

内存 channel 只做"有新活"的唤醒，并且**满了一律丢弃**——因为 `thumb_status='pending'` 才是真相，下次扫描会补。这样进程被 kill 不丢任务。反过来也有坑：只在启动时扫一次的话，**积压超过单批上限（200 条）就只能靠重启才动**，所以必须有周期 sweep（30 s）。

### 7.35 "发送成功"不能用通知类的邮件函数判断（第 79 轮）

`sendMailFn` 对通知类邮件把"没配 SMTP"当成功（对通知是合理的：不该因为没配邮件就让审核失败）。验证码照搬这个语义就是骗人。所以验证码走 `requireSMTP` 显式 503，并且顺序是"冷却 → 查 SMTP → 发信 → 落库"——发信失败不留"有效但没人收到"的行。

### 7.36 前端不渲染输入框 ≠ 服务端安全（第 79 轮）

开着邮箱验证时，前端 `/settings` 把邮箱显示成只读 + "更换"面板。但 `PUT /api/auth/me` 原本仍然接受 `email` 字段——**一次 PUT 就绕过整套验证码**。修法是 `applyIdentityChange` 里 403，并补单测。凡是"换了个入口做同一件事"的新流程，都要回头堵住旧入口。

### 7.37 真机冒烟的三个惯性误判（第 77–79 轮，都踩过）

1. **本机 `lopez` 账号是 `admin` 不是普通用户**——拿它断言"普通用户 403"会假通过。要用 root 新建一个 `role=user` 的账号。
2. **脚本连发几十个请求会撞通用令牌桶**（20 rps / burst 50），429 被误读成功能 bug。冒烟脚本要对 429 退避重试。
3. **验证码冷却按邮箱地址存在库里**，重跑脚本必须换邮箱后缀或重建 DB 副本，否则第一次请求就 429。

---

## 8. 已知问题 & 待修复清单

> 完整清单在 `docs/TODO.md`（唯一权威待办）。这里只留"接手后最该知道"的几条。

| 等级 | 问题 | 位置 / 状态 |
|------|------|------------|
| 🟡 | 普通用户编辑器暴露「公开/私密」两个 radio，但 `CreatePost` 对非 root 一律强制 `pending` | `PostEditor.tsx` + `services/posts.go`；用户以为存成私密、实则进审核，通过后变公开。**待拍板**：隐藏该选项 or 明确提示「先过审核」 |
| 🟢 | `PostEditor` 把后端返回的 `pending` 误显示为「公开」 | `PostEditor.tsx` 初始化逻辑 `status==='private'?'private':'published'`，**待拍板** |
| 🟡 | 编辑器三栏在 1440 宽窗口下右侧面板溢出被裁 | 标签输入框只露约 80px，等于没法用（T-4，非某轮引入） |
| 🟡 | 邮箱验证未用真实 SMTP 验过 | 只跑过 `scripts/fakesmtp.py`；上线前须用真邮箱验 TLS 与退信 |
| 🟡 | 回收站只有"清空"，没有"单条永久删除" | `services/trash.go`；用户想彻底删一篇得先进回收站再整清 |
| 🟢 | 缩略图 `failed` 是终态，不自动重试 | 需管理员手工清 `thumb_status`；解码失败的图本来也不值得重试 |
| 🟢 | `upload_refs` 只在 API 保存/删除文章时同步 | 绕过 API 直接改库会留孤儿引用；无夜间对账任务 |
| 🟢 | 邮箱验证码"同 IP 每小时 10 封"是内存固定窗口 | 重启清零、多实例各算各的；单实例部署可接受 |
| 🟡 | 编辑器草稿仅 localStorage，不跨设备 | `PostEditor.tsx`（30 天 TTL，可接受）；服务端修订历史只管已保存版本 |
| 🟢 | 登录限流器账号桶 key：`email` 优先、回退 `username`（7.2 已加固） | `login_limiter.go` + 单测 |
| 🟢 | 全文搜索 LIKE `%...%`，千级后变慢 | 纯 Go 驱动无 FTS5，暂保留 |
| 🟢 | JWT_SECRET 生产必须覆盖 | `.env.example` 已注明；`ValidateSecurity` 已校验 |
| 🟢 | 审核/封禁无邮件服务时的通知 | `mail.go` 为 best-effort（SendMail 失败不阻断审核），已接受 |
| 🟢 | `LopezPress.bat` 探测的 Go 路径不含本机实际位置 | 脚本找 `%USERPROFILE%\go-install\go\bin` 与 `%SystemDrive%\go\bin`，本机在 `go-sdk\go\bin`；有 `lopezpress.exe` 时无影响 |

---

## 9. 未决决策（需要你拍板或继续）

> 2026-10-15 收敛：原「生产化优先级」已由第 24 轮（容器化）部分回答。2026-10-24 第 79 轮再收敛一次——当前真正未决的只剩下面 4 条。

1. **普通用户的「私密」入口怎么处理**（阻塞体验）：编辑器给普通用户看得到"私密"，但 `CreatePost` 强制 `pending`。要么隐藏该选项，要么明说"你的文章需先过审核，通过后按你选的可见性发布"。两条路都要改前端 + 一句产品定义。**等你拍板。**
2. **生产部署验证**：`deploy/docker/` 已就绪但**未 docker build、未在真实 VPS 端到端跑过**（开发机无 Docker）。需要一台 1 vCPU VPS 实测，至少覆盖：nginx 的 4 个新 location、`/uploads` 静态与反代的边界、`TRUSTED_PROXIES` 配置。
3. **邮箱验证上线前的真机验证**：`E-7`——必须用真 SMTP（QQ / Gmail / 自建）走一遍三条流程，尤其验 TLS 端口 587/465 与退信行为。当前只有本地假服务器证据。
4. **PWA / Service Worker 离线化**：列入后续方向，未实现。

> 框架选型已定稿不换：后端 Go 标准库 + GORM + 纯 Go SQLite、前端 Preact（见 `docs/decisions/` 与第 22 轮评审）。**不要为了"更现代"改框架**（除非有具体性能/功能瓶颈证据）。
> 第 78 轮新增的 `golang.org/x/image` 是官方扩展库（纯 Go、无 CGO），属于对"不加重依赖"红线的一次**显式豁免**，理由记在 `docs/decisions/0005-media-ownership-and-thumbnails.md`。

---

## 10. 文档索引

| 文档 | 内容 |
|------|------|
| `docs/README.md` | **文档体系入口**：specs / decisions / 设计系统 / 问题清单的分工与索引 |
| `docs/ARCHITECTURE.md` | 设计稿（目标架构、技术选型、中间件链、审计指引、已知陷阱引用） |
| `docs/AUDIT_REPORT.md` | 安全审计与 10 项加固记录（2026-09-24 轮） |
| `docs/TODO.md` | **唯一权威待办清单**（P0–P3 + 架构债 + 媒体/邮箱两族 + 功能缺口 + 设计取舍） |
| `docs/specs/` | 36 份功能 spec（01–37，部分号跳空）+ 模板；36 = 图库、37 = 邮箱验证 |
| `docs/decisions/` | 重大决策：0001 纯 Go SQLite 无 FTS5、0002 localStorage 草稿 TTL、0003 token 存储、0004 CORS、0005 媒体所有权与缩略图、0006 邮箱验证默认关闭 |
| `docs/PROJECT_OVERVIEW.md` | 代码地图：路由 / API 端点 / 页面 / 数据模型 / 改码红线 |
| `docs/DESIGN_SYSTEM.md` | 全站 UI 唯一标准（改界面前先读） |
| `docs/archive/` | 已归档快照（`2026-10-14-FUNCTIONAL_AUDIT.md` 首轮审计，结论已落入 TODO.md） |
| `PROGRESS.md` | 79 轮开发全记录 + 风险 / 方向 / 编译验证基线 |
| `README.md` | 快速上手（运行/配置/演示账号） |
| `MASTER.md` | **唯一入口**：文档总索引 + 自驱协议 + 事实基线 + 当前断点 |
| `HANDOVER.md` | 本文（交接总览） |
| `deploy/` | VPS 部署脚本 |

---

## 11. 接手自测清单（建议你跑一遍再动手）

1. `go build ./...` ✅ / `go vet ./...` ✅ / `go test -count=1 ./internal/...` ✅（48 个测试文件 / 212 用例）
2. `python scripts/arch_check.py` ✅ 退出 0 / `python scripts/contract_check.py` ✅ 方法不匹配 0 / `python scripts/doc_check.py` ✅ 73 份一致
3. `cd web && node node_modules/typescript/bin/tsc --noEmit` ✅ / `NODE_OPTIONS= npm test` ✅ 43 用例 / `npm run build` ✅
4. 双击 `LopezPress.bat` → 浏览器开 `http://localhost:5173` → 首页/列表/归档/作者页能加载（无 500）
5. 用首启随机密码登录 admin → 自动跳 `/change-password` → 改密成功
6. 连续 5 次错误密码登录 → 第 6 次应 429 + `Retry-After` 头
7. 管理页 → 备份导入 URL 加 `?dry_run=1` → 返回统计不落库
8. `APP_ENV=prod` + 弱 `JWT_SECRET` 启动 → 应 `log.Fatalf` 拒绝（安全校验生效）
9. 登录后 `/settings` → 点头像 → 裁剪器能拖动缩放 → 保存后头像更新；下方"我的图库"显示配额条
10. 管理员 `/admin/media` → 能看到缩略图、引用数徽标；撤下后原图 URL 仍 200 但列表里消失
11. 把窗口拖窄到 ≤768px → 用户菜单从 hover 浮层变 overlay+tap（#10）

若以上任一不过，先对照 §7 陷阱，再回到对应源文件。大部分"500 / 启动崩溃"都来自 §7.1 的 AutoMigrate 门控被改动。
