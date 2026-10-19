# 架构设计文档 — lopezpress（Go 内容型站点后端）

> 状态：设计稿（未改代码）。本文档描述目标架构、技术选型与后续实施路线，供决策与落地参考。
> 组合取向：**标准库 `net/http`（传输层）+ GORM（数据层）+ SQLite（起步存储）**。

---

## 1. 背景与目标

- 内容型站点，前后端分离；后端只提供 REST API。
- 功能范围（基础版）：用户注册/登录 + JWT 鉴权 + 文章 CRUD。
- 取向：**性能好、资源占用少**，同时保留 GORM 的开发效率。

### 取向说明（重要）
"资源少" 与 "GORM" 是两个不同维度的决策：
- HTTP 层选 **标准库 `net/http` + 原生 `ServeMux`**（Go 1.22+）→ 零第三方依赖，包体与内存最低。
- 数据层选 **GORM** → 开发快、ORM 全功能，但比裸 SQL 多内存/编译开销。
若后续追求极致资源，可将 GORM 平滑替换为 `sqlx`（半 ORM）或 `database/sql`（零 ORM），**接口层不动**，因为已按分层架构隔离。详见 §7。

---

## 2. 技术选型总览

| 层 | 选型 | 理由 | 备选 |
| --- | --- | --- | --- |
| HTTP 框架 | `net/http` + `http.ServeMux`（Go 1.22+ 方法/通配符模式） | 零依赖、包体最小、原生路由匹配 | chi（标准库风格+路由）、Echo/Gin |
| 路由注册 | `http.NewServeMux()`，支持 `"POST /api/posts"` 与 `"GET /api/posts/{id}"` | Go 1.22 起原生支持，无需自写 router | `chi` router |
| 中间件 | 自写（日志、CORS、限流、JWT、Recover、耗时） | 零依赖；标准库无中间件链 | `chi/middleware` |
| 参数绑定/校验 | `encoding/json` + 手写校验（或 `go-playground/validator`） | 保持轻量 | `validator` |
| 鉴权 | **JWT**（HS256），`golang-jwt/v5` | 无状态、前后端分离标准做法 | 有状态 Session/Cookie |
| 密码哈希 | `golang.org/x/crypto/bcrypt` | 事实标准 | Argon2 |
| 数据访问 | **GORM** | 自动迁移、关联、Hook，开发最快 | `sqlx`（半 ORM）/ `database/sql` |
| 数据库 | **SQLite**（`glebarez/sqlite` 纯 Go 驱动，无 CGO） | 零运维、单机起步、跨平台好部署 | PostgreSQL/MySQL（上多实例时换驱动） |
| 日志 | `log/slog`（Go 1.21+ 结构化，零依赖） | 标准库、够用 | `zerolog`/`zap`（高性能/采样） |
| 配置 | 环境变量 + 默认值（`os.Getenv`） | 零依赖、12-factor | `godotenv`/`viper` |
| 限流 | `golang.org/x/time/rate` | 标准库生态、防刷 | 自写令牌桶 |
| 测试 | 标准库 `testing` + `httptest` | 零依赖、HTTP 测试友好 | `testify`（断言糖衣） |

---

## 3. 项目目录结构（分层架构）

```
lopezpress/
├── cmd/
│   └── server/
│       └── main.go            # 唯一 main：装配配置→初始化 DB→构建 Router→启动 HTTP（薄壳）
├── internal/                  # 不可被外部 import 的私有包
│   ├── config/
│   │   └── config.go          # 配置加载（env + 默认值）
│   ├── db/
│   │   ├── db.go             # GORM 连接 + 迁移 + 种子数据（封装，便于换驱动）
│   │   └── sqlite.go        # 可选：纯 Go sqlite 驱动注册
│   ├── models/
│   │   └── models.go          # User / Post 等 GORM 模型（含 BeforeCreate Hook）
│   ├── handlers/             # HTTP 薄层：解析请求、调用 service、写响应（不含业务）
│   │   ├── auth.go
│   │   └── posts.go
│   ├── services/             # 业务厚层：注册/登录/文章 CRUD 逻辑（可单测、可换框架复用）
│   │   ├── auth.go
│   │   └── posts.go
│   ├── middleware/           # 自写中间件（标准库无中间件链）
│   │   ├── middleware.go     # 类型定义 + 链式执行器
│   │   ├── logger.go        # log/slog 结构化访问日志 + 耗时
│   │   ├── cors.go          # CORS 预检与响应头
│   │   ├── limiter.go      # 限流（x/time/rate）
│   │   ├── recover.go       # panic 兜底
│   │   └── auth.go          # JWT 校验，注入 caller 上下文
│   ├── router/
│   │   └── router.go        # ServeMux 注册全部路由 + 中间件装配
│   └── httpx/               # 可选小工具：JSON 响应/错误
│       └── response.go
├── go.mod
├── .env.example
├── .gitignore
├── README.md
└── api.md
```

**核心原则**
- `handlers` 薄：只做「HTTP 协议翻译」（解析、鉴权上下文、JSON 序列化），**不写业务逻辑**。
- `services` 厚：所有业务规则在此，便于单元测试与"换框架/换 ORM 时业务不重写"。
- `internal/` 保证不被外部包依赖，边界清晰。

> **与当前代码的偏差（2026-09）**：本文档为设计稿，部分细节已演进。最重要的一条——DB 初始化与迁移当前位于 `internal/config/config.go`（`InitDB` / `runManualMigrations`），**并未**单独拆出 `internal/db/` 包；是否拆分见 HANDOVER.md §9 决策 1。实际源码结构以 `cmd/server/main.go` 与 `internal/*` 为准。

---

## 4. 模块依赖关系（仅单向向下）

```
cmd/server (main)
   │  装配
   ▼
internal/router  ──►  internal/middleware
   │  │                 │
   │ ▼                 ▼
   └► internal/handlers ─► internal/services ─► internal/models
                                        │
                                        ▼
                                   internal/db ─► GORM ─► SQLite
                                        │
                                        ▼
                                   internal/config (被 db / router / services 读取)
```
禁止反向依赖（如 services 引用 handlers，或 models 引用 services）。

---

## 5. 关键数据流

### 5.1 鉴权 + 创建文章（典型写请求）
1. 客户端 `POST /api/posts`，带 `Authorization: Bearer <jwt>`。
2. `router` 的 `auth` 中间件解析并校验 JWT → 把当前用户写入 request context（`caller`）。
3. `handlers/posts.go` 仅做：绑定 JSON、取 context 中用户、调用 `services.CreatePost(ctx, authorID, in)`。
4. `services` 执行业务（校验 title、设默认 status、落库 GORM Create）。
5. 返回 201 + 文章对象（JSON 序列化在 handler）。

### 5.2 登录
1. `POST /api/auth/login`（公开）→ `handlers/auth.go` 绑定。
2. `services.Login`：按用户名或邮箱查用户 → bcrypt 校验密码 → 签发 JWT（带 exp/iat/sub/role）。
3. handler 返回 `{ token, user, expires_at }`。

### 5.3 启动（main 装配顺序）
```
loadDotEnv()                 // 读 .env 注入环境（不覆盖已存在的变量）
config.Load()                // 读取 + 默认值
  → cfg.ValidateSecurity()   // 开库前校验 JWT_SECRET 强度（prod 严格，dev 仅 warn）
  → config.InitDB(cfg)       // 建目录、Open GORM、runManualMigrations（手写迁移 + schema_migrations 门控）、仅对"不存在的表"AutoMigrate、seedDemoData（首启随机 admin 密码横幅）
  → services.New(db, cfg)    // 业务厚层
  → handlers.Init(db, cfg, svc)
  → middleware.NewLimiter(...) + NewLoginLimiter()（仅套 POST /api/auth/login）
  → router.New(cfg, limiter, loginLimiter)   // 组装中间件链 + 注册路由
  → http.ListenAndServe(":PORT", mux)
  → 优雅关闭（SIGINT/SIGTERM → Shutdown，10s 超时）

> ⚠️ `glebarez/sqlite` + GORM `AutoMigrate` 在误判需重建表时会走有缺陷的 copy-rename 路径（漏 NOT NULL 列致启动崩溃）。当前用「仅对不存在的表 AutoMigrate + 手写迁移门控」规避；改 `internal/config/config.go` 务必谨慎（详见 HANDOVER.md §7.1）。
```

---

## 6. 中间件设计（标准库自写要点）

标准库没有中间件链，用统一签名 + 执行器封装：

```go
// middleware.go
type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
    for i := len(mws) - 1; i >= 0; i-- {
        h = mws[i](h)
    }
    return h
}
```

装配顺序（外→内）：`Recover → Logger → Limiter → CORS → JWTAuth(仅受保护组)`。
- **Logger**：`log/slog` 输出 method/path/status/耗时，JSON。
- **Limiter**（令牌桶 `x/time/rate`，**默认 5 rps / burst 10**）：
  - **粒度：用户 ID 优先，IP 兑底**。已登录请求用 `userID` 作限流 key；未登录请求用 `clientIP` 作 key。
  - `clientIP` 取自 `X-Forwarded-For` 首段（兼容反向代理/Nginx），无则回退 `RemoteAddr` 主机部分。
  - key 在内存 `sync.Map` 中缓存各自的 `*rate.Limiter`；上限 RPS 可被环境变量 `RATE_LIMIT_RPS` / `RATE_LIMIT_BURST` 覆盖（默认 `5` / `10`）。
  - 超限返回 `429 Too Many Requests`，并带 `Retry-After` 头（按 `Wait` 估算）。
  - 单机内 `sync.Map` 足够；多实例场景见 §7（换 Redis 计数）。
- **CORS**：处理 `OPTIONS` 预检 + 同源响应头；生产收紧 `AllowOrigins`。
- **JWTAuth**：解析 `Authorization`，校验签名/exp，把用户 ID 注入 `context`。

---

## 7. 可扩展性 / 降级路线

| 场景 | 做法 | 影响面 |
| --- | --- | --- |
| 单机起步 | 保持 SQLite + GORM | 无 |
| 上多实例 | GORM 驱动切 PostgreSQL/MySQL（改 `db.Init` 一行 + 环境变量） | `internal/db` |
| 极致省资源 | GORM → `sqlx`（半 ORM）或 `database/sql`；models 的 Hook 改为显式 SQL | `internal/services` + `internal/models` |
| 高并发 | `slog` → `zerolog`/`zap`；引入 Redis 做会话/缓存/限流计数（替换 `sync.Map` 内存桶） | 日志 + 缓存 + 限流 |
| 内容量大 | 文章加全文检索（`pg_trgm`/Elasticsearch）+ 分页游标 | `services/posts` |
| 多角色 | 在 services 内做权限判定（已是 作者/admin 模型） | `services` |

**隔离收益**：因分层（handlers 薄 + services 厚 + db 封装），换 ORM/换数据库/加缓存都**不需要动 HTTP 协议与路由**，迁移成本低。

---

## 8. 配置项（`.env`）

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | 监听端口 |
| `JWT_SECRET` | 开发默认 | 生产必须强随机 |
| `JWT_EXPIRY_HOURS` | `24` | 令牌有效期 |
| `DB_PATH` | `data/app.db` | SQLite 文件路径 |
| `DB_MAX_OPEN` | `4` | GORM 连接池最大打开连接（SQLite 单文件，低配机刻意调小，避免浪费 goroutine/内存） |
| `DB_MAX_IDLE` | `2` | GORM 连接池最大空闲连接 |
| `LOG_LEVEL` | `warn` | GORM 日志级别（`debug`/`info`/`warn`/`error`），低配机默认降噪 |
| `RATE_LIMIT_RPS` | `5` | 令牌桶速率上限（每 key 每秒请求数） |
| `RATE_LIMIT_BURST` | `10` | 令牌桶突发上限（允许瞬时超出 RPS 的缓冲） |
| `CORS_ORIGINS` | `*` | 逗号分隔；生产收紧 |

> **目标机型**：1 vCPU / 1.5 GB 内存 / 5 GB 磁盘的 VPS，**硬性只用 70%**（≈1.0 GB 内存、≈3.5 GB 磁盘可用，1.5G 机型通常无 swap，内存超限即 OOM）。
> 上述 `DB_MAX_OPEN=4 / DB_MAX_IDLE=2 / LOG_LEVEL=warn` 三项默认值即为该预算收紧：GORM 池更小、日志更安静、限流桶缓存上限降到 512（`internal/middleware`）。

---

## 9. 安全与生产化清单

- [ ] 更换 `JWT_SECRET` 为强随机（≥32 字节）
- [ ] 收紧 `CORS_ORIGINS` 为真实前端域名
- [ ] 全链路 HTTPS（置于反向代理/TLS 终结）
- [ ] 启用限流，注册接口额外加验证码/邮箱校验防注册滥用
- [ ] 输入校验 + 参数化查询（GORM 已防注入）
- [ ] 日志脱敏（不打印 token/密码）
- [ ] 健康检查 `/health` + 就绪探针；优雅停机
- [ ] 依赖与镜像扫描；最小化基础镜像（`gcr.io/distroless`）

---

## 10. 实施路线（建议 5 步）

> 状态标记：✅ = 已完成；⬜ = 待办。
> 截至本次重构，**步骤 1 与步骤 2 已完成**。

1. ✅ **重构目录**：把现有单包 `main` 拆为 `cmd/server` + `internal/*`；抽 `services` 厚层。
2. ✅ **HTTP 层改标准库**：用 `http.ServeMux`（Go 1.22+）替换 Echo；自写 6 个中间件（Recover/Logger/Limiter/CORS/JWTAuth）。
   - **Limiter 现已改为纯标准库接入**：`internal/middleware` 的 Echo 桥（`EchoMiddleware` / `echoIdentity`）已删除，
     `resolveIdentity` 通过 `middleware.UserIDFrom(ctx)` 读取 JWT 鉴权处理器写入的用户 ID（`WithUserID`），无用户时回退到 `clientIP`。
   - 路由装配集中在 `internal/router/router.go`，中间件链顺序为 `limiter → CORS → Logger → Recover`（limiter 最内层，最先看到请求）。
3. **保留 GORM**：`models`/`db` 基本不动，仅把连接封装进 `internal/db`。
4. **补测试**：`httptest` 对 handlers + `testing` 对 services（登录、CRUD、权限 403）。
5. **生产化**：`.env` 示例、优雅关闭、`Dockerfile`、CI（`go build` + `go vet` + `go test`）。

### 验收标准
- `go build ./...` 通过；`go vet` 无告警。
- 所有 API 行为与现 `api.md` 一致（路由、状态码、错误 JSON）。
- 单元测试覆盖：注册、登录、JWT 失败、文章 CRUD、越权 403。
