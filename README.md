> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# lopezpress

基于 **Go 标准库（`net/http`）+ GORM + SQLite + Preact** 的内容型站点全栈项目，前后端分离。

- 后端：Go **1.26.10** 标准库 `ServeMux`（方法 + 通配路由），纯 Go SQLite 驱动（无需 CGO，可交叉编译）
- 前端：Preact + Vite + TypeScript（`web/`），构建为单页应用（SPA）
- 鉴权：JWT（HS256）+ bcrypt；浏览器端 token 存 **`lp_token` HttpOnly + SameSite=Lax Cookie**，`Authorization` 头保留给 API 客户端；`token_version` 支持服务端吊销
- 限流：自写令牌桶中间件（按身份分桶，token 从 cookie 或 header 读，默认 **20 rps / burst 50**，`.env` 的 `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` 可调）+ 全局匿名桶（200 rps / burst 400，防 IP 轮换稀释）+ 登录专用限流（IP/账号双桶 + 5 次失败锁）

## 功能

**内容**
- 文章 CRUD（作者 / 管理员权限控制）、Markdown 渲染 + XSS 白名单清洗
- 五态状态机：`published` / `private` / `pending` / `rejected` / `banned`（`banned` 仅 root 可见）
- **永久链接**：`/p/标题`（旧 `/read/{uuid}` 永久有效，sitemap/RSS 自动用 slug）
- 两层分类树（父/子，含预设树）+ 标签（many2many）、置顶、上下篇、**相关文章**
- 阅读页：目录高亮、阅读进度、阅读时长、字数、图片灯箱、代码复制、面包屑
- **纯分页**（`?page=N`，每页条数由后台定 5/10/20/30，保证链接可分享）
- 归档页（服务端分页 50/页）、RSS + sitemap + **robots.txt**（跟随公开开关）
- 全文搜索（`LIKE`）、封面图、列表缩略图、图片上传（本地 / SM.MS 双模）
- **浏览量指纹去重**：同一访客同一天同一篇只计一次，作者本人与爬虫不计
- **修订历史**：保存即快照，每篇留最近 20 版，可一键回滚（只回标题/正文）
- **Markdown 批量导入**：多选 `.md`，支持 frontmatter，保留原始发布时间

**账号与权限**
- 注册 / 登录 / 当前会话 / 登出 / 登出所有设备（`token_version` 吊销）
- 三角色层级：`root`（全库唯一，不可经任何 API 创建或提升）/ `admin` / `user`
- 内容审核流（user→admin 审、admin→root 审）+ root 封禁库（送出/永久删）
- 个人中心（我的主页 · 内容管理 · 安全中心：改密 / 登录历史 / 吊销会话）
- 操作审计日志（`ActionLog`）、登录历史、禁用账号即时失效

**运维**
- JSON 备份导出 / 导入（含 dry-run 与导入前自动快照）、快照列表与恢复（root-only）、回收站
- **每日自动备份**：root 一键开启，每天一份 `auto-YYYYMMDD.db`，保留份数 1–90（不需要 cron）
- **仪表盘统计**：近 30 天去重浏览量趋势（内联 SVG，无图表库）、待审计数、最多阅读 Top5
- **后台文章管理**：筛选 / 勾选 / 批量发布·转私密·改分类·加减标签·删除（逐条复用单条权限逻辑）
- **公开阅读开关**：后台一键切换"匿名可读 / 必须登录"，前后端同时执行（API 也 401）
- SMTP 邮件通知（登录失败 / 改密 / 异地登录，手写零依赖客户端）
- root 密码离线恢复：`lopezpress -reset-root-password`（随机新密码 + 强制改密 + 吊销旧会话）
- 首启自动建表 + 写入演示数据（root 账号 `admin` + 3 篇示例文章 + **12 个两层预设分类** + 5 标签）

## 项目结构

```
lopezpress/
├── cmd/server/main.go         # 入口：装配依赖、初始化 DB、启动 HTTP 服务（优雅关闭）
├── internal/
│   ├── config/                # 配置（.env）+ 数据库连接 + 迁移 + 种子数据
│   ├── models/                # GORM 数据模型（User / Post / Category / Tag / ActionLog / LoginLog / SiteConfig）
│   ├── handlers/              # 薄处理器（auth / accounts / posts / categories / moderation / upload / admin / feed / query）
│   ├── services/              # 业务厚层（含 roles.go 权限唯一入口、canSeePost 可见性权威）
│   ├── middleware/            # 自写中间件：Chain / Logger / CORS / Recover / 通用限流 / 登录限流
│   ├── router/                # 路由装配（Go 1.22 ServeMux 方法+通配 + 中间件链）
│   └── httpx/                 # 请求/响应小工具
├── web/                       # 前端（Preact + Vite + TS）：27 个页面 + 个人中心子组件 + core 层（api/auth/i18n/router/theme/md/format/http）
├── deploy/                    # 部署脚本（systemd / Nginx / Docker Compose / 一键上传）
├── docs/                      # 2 份：待办清单 + 上线安全基线
├── scripts/
│   ├── dev.sh                 # Linux 一键开发：后端 + 前端，数据隔离到 data/dev
│   └── contract_check.py      # 前后端 method+path 契约自动核对
├── .env                       # 运行时配置（端口 / JWT / DB 路径 / 限流）
└── go.mod
```

> 启动命令：`go run ./cmd/server`，默认监听 `:8080`。
> **一键启动（Linux，推荐）**：`scripts/dev.sh up` 同时拉起后端（:8080）和前端（:5173），并强制数据隔离到 `data/dev/`；支持 `up` / `down` / `restart` / `status` / `logs` / `build` / `gate` / `reset-root` / `help` 子命令，已做端口幂等检测。
> **后端路由总量**：84 条（前端已接入 77 条，`scripts/contract_check.py` 可复核；未被前端调用的 6 条为 `/feed`、`/health`、`/health/ready`、`/robots.txt`、`/sitemap.xml`、`/uploads/`）。

## 快速开始（本地开发）

### 1. 安装 Go

需要 Go **1.26.10**（`go.mod` 声明的最低版本；标准库 `ServeMux` 方法通配路由特性）。

```bash
go version                 # 需要 ≥ 1.26.10
# 若发行版自带版本偏低，GOTOOLCHAIN 会在首次构建时自动下载 1.26.10：
export GOTOOLCHAIN=go1.26.10
go version
```

> `scripts/dev.sh` 已固定 `GOTOOLCHAIN=go1.26.10`，手动跑测试时也建议带上该变量。

### 2. 启动后端

```bash
cd lopezpress
GOTOOLCHAIN=go1.26.10 go mod tidy
GOTOOLCHAIN=go1.26.10 go run ./cmd/server   # 默认监听 :8080
```

首次运行会在 `data/app.db` 创建 SQLite 数据库并写入演示数据。

### 3. 启动前端

```bash
cd web
npm ci                    # 按 package-lock 安装，含 Linux 原生 rollup/esbuild
npm run dev               # http://127.0.0.1:5173
```

前端 dev server 会把 `/api/*` 和 `/health` 反代到 `http://localhost:8080`，可同源自由调用后端。

### 4. 本地预览生产构建（可选）

```bash
cd web
npm run build            # 产出 dist/（SPA）
npm run preview          # http://127.0.0.1:4173，同样带 API 反代
```

`preview` 与 `dev` 一样把 `/api`、`/health` 反代到后端，可用于验收构建产物；线上则用 Nginx 托管 `dist/` + 反代后端（见部署）。

### 5. 环境配置（可选）

默认值可直接运行。需要覆盖时（完整清单见 `.env.example` 与 `internal/config/config.go`）：

```bash
export PORT="8080"
export APP_ENV="dev"              # prod 时强制校验 JWT_SECRET 强度 + CORS 非 "*"
export JWT_SECRET="change-me"     # prod 必须 ≥32 字节、≥128bit 熵的强随机值，否则启动失败
export JWT_EXPIRY_HOURS="24"
export DB_PATH="data/app.db"
export CORS_ORIGINS="*"           # prod 改为显式来源列表，逗号分隔
export RATE_LIMIT_RPS="20"
export RATE_LIMIT_BURST="50"
export SM_MS_TOKEN=""             # 可选：SM.MS 图床 token 服务端持有
```

> 生产部署务必：更换 `JWT_SECRET`（≥32 字节且通过 128-bit 熵下限）、`APP_ENV=prod`、`CORS_ORIGINS` 显式列表、启用 HTTPS。**上线前请逐项核对上线安全基线**（内部维护，不随仓库分发）：防住了什么、还剩哪两件没做完、哪些风险是明确接受的。

## 演示账号

| 用户名 | 邮箱 | 密码 | 角色 |
| --- | --- | --- | --- |
| admin | admin@example.com | 首次全新启动随机生成（仅终端打印一次） | **root**（全库唯一超级管理员） |

> **admin 密码说明**：`seedDemoData` 仅在数据库**不存在 admin 账号**时新建，并使用随机 22 字符密码（在 stderr 打印一次横幅，不落日志），登录后强制跳 `/change-password` 改密。若数据库**已存在** admin 账号（如既有开发库），则沿用既有密码不被覆盖——历史开发库的 admin 密码可能是早期固定的 `[redacted]` 或此前某次随机值，**以实际数据库为准**；忘记密码可用 `ADMIN_INITIAL_PASSWORD` 环境变量在下次首启（无 admin 时）固定。
>
> **密码策略只管写入，不管既有密码**（1.0.1 起）：新建账号 / 改密 / 重置密码要求「≥ 8 个字符且 ≤ 72 字节，且不在常见密码表里」；但一个存量 `[redacted]` **仍然能登录**——策略拦截的是"设置弱密码"，不是"已有弱密码"。它会在下一次改密时被要求换掉。
>
> ⚠️ **首启账号是 root，不是 admin**（`config.go` seed 写入 `Role: RoleRoot`）。因此：
> - `POST /api/users/{id}/reset-password` **无法重置 root 密码**（admin 重置 root 会被 `canManageUser` 403 拦下）；
> - root 忘记密码走**离线恢复**（HTTP 无路径，刻意如此）：
>
> ```bash
> scripts/dev.sh down
> scripts/dev.sh reset-root   # 生成新随机密码并打印一次，同时吊销 root 所有旧会话
> scripts/dev.sh up
> ```
>
> 若要对非隔离库执行，用 `DB_PATH=data/app.db ./data/dev/lopezpress -reset-root-password`（先 `scripts/dev.sh build` 产出二进制）。
>
> root 没有 HTTP 恢复路径是**刻意设计**：任何在线入口都会变成提权面。只能上机执行上面那条命令。

## 验证

```bash
# 健康检查
curl http://localhost:8080/health

# 登录拿 token（admin；密码见上方"admin 密码说明"：全新库用终端横幅里的随机密码，已有库用既有密码）
curl -X POST http://localhost:8080/api/auth/login -H "Content-Type: application/json" -d '{"email":"admin@example.com","password":"<密码>"}'

# 当前会话
curl http://localhost:8080/api/auth/me -H "Authorization: Bearer <token>"

# 文章列表
curl http://localhost:8080/api/posts -H "Authorization: Bearer <token>"

# 创建文章
curl -X POST http://localhost:8080/api/posts -H "Content-Type: application/json" -H "Authorization: Bearer <token>" -d '{"title":"新文章","body":"正文"}'
```

## 前端说明（web/）

| 命令 | 说明 |
| --- | --- |
| `npm run dev` | Vite dev server（带 API 反代），`http://127.0.0.1:5173` |
| `npm run build` | 构建生产产物到 `web/dist/`（SPA） |
| `npm run preview` | 本地预览构建产物 |
| `npm run typecheck` | `tsc --noEmit` 类型检查 |

要点：
- 框架为 **Preact**（API 兼容 React），路由用 **`wouter`**（`core/router.tsx` 薄封装），**非** `react-router` / `preact-router`
- `tsconfig.json` 用 `jsx: "react-jsx"` + `jsxImportSource: "preact"`
- `createContext` 来自顶层 `preact`；`useContext/useEffect/useState/useRef` 来自 `preact/hooks`
- 子类型用 `ComponentChildren`（Preact 无 `Child` 类型）
- token 存 `lp_token` HttpOnly Cookie（前端不持有 JWT，只留非敏感 `echo.session` 标记），启动时用 `GET /api/auth/me` 重新校验有效性；请求统一走 `core/http.ts`
- 运行时依赖共 4 个：`preact` / `wouter` / `marked` / `vditor`。`vditor` 只被编辑器用到且走动态 `import()`，读者端不加载（第 39 轮已清掉 `@preact/signals`、`@preact/signals-core`、`preact-router`、`preact-render-to-string` 四个零 import 残留）

## 文档导航

仓库里只有 **3 份文档**（原来 90 多份已完成的设计稿与历史记录已删除）：

| 想了解 | 看这里 |
| --- | --- |
| 怎么跑起来 / 环境配置 / 演示账号 | 本文件 |
| 接口、端点、请求响应样例 | [`api.md`](api.md) |
| 部署 / 上线 / HTTPS / 排障 | [`deploy/README.md`](deploy/README.md) |

> 待办清单与上线安全基线属于内部维护文档，不随仓库分发。

## 部署

见 [deploy/README.md](deploy/README.md)：systemd 服务、Nginx 反代 + 前端静态托管、一键上传脚本、线上校验脚本（`check-live.sh`，确认服务器跑的就是这份代码）；容器化方案见 [deploy/docker/](deploy/docker)（未在容器环境中实跑验证）。

## 说明

- 默认 SQLite（`glebarez/sqlite` 纯 Go 驱动，无需 CGO，可交叉编译到 Linux）。换 PostgreSQL/MySQL 仅需改 `config.InitDB` 的驱动。
- JWT 无状态，但**已支持服务端吊销**：`User.token_version` 写入 token 的 `ver` claim，`POST /api/auth/logout-all` bump 版本使旧 token 全部 401（`POST /api/auth/logout` 仍是客户端信号，需要时可改用 logout-all）。
- 生产环境务必：更换 `JWT_SECRET`（≥32 字节且通过 128-bit 熵下限）、收紧 CORS、启用 HTTPS。
