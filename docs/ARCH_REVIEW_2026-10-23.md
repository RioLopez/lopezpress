> 类别 B · 项目参考（架构审计）— 由 `MASTER.md` §3 索引。本文描述"代码现在长什么样、为什么、应该长什么样"，不约束 AI 流程；其中被采纳的规则会另行写入 `AGENTS.md` / `docs/TODO.md`。

# 架构复审：AI 生成痕迹与重构方案（2026-10-23 · 第 53 轮）

> **复审对象**：后端 71 个 `.go` / 15,721 行，前端 47 个 `.ts(x)` / 9,773 行（第 52 轮基线）。
> **复审方式**：只读静态分析 + 计数取证，**未修改任何业务代码**。所有结论都能用文中给出的 `grep` 命令复现。
> **一句话结论**：这不是"写得烂"，而是**同一个问题在不同轮次被独立解决了一遍**——单点看每处都合理，合起来就是七套写法并存。系统能跑（门禁全绿），但**每加一个功能，不一致度就上升一格**，这是可维护性的天花板，不是 bug。

---

## 0. 摘要

### 0.1 评分（10 分制，只评"结构可维护性"，不评功能完整度）

| 维度 | 分 | 依据（详见 §2/§3） |
|---|---|---|
| 依赖控制 | 9 | 后端 6 直接依赖、前端 3 个包，全程零框架膨胀 |
| 文档-代码一致 | 8 | `doc_check.py` 机械校验母文档；仍有少量注释/待办漂移 |
| 错误契约 | 7 | `ServiceError` + `writeServiceError` 统一，但混入 4 处裸 `fmt.Errorf` |
| 权限模型 | 6 | 双层校验有效，但"唯一权威"`canReviewPost` **零调用**，规则在 5 处手写重述 |
| 分层清晰度 | 5 | handler 确实薄；**service 反向承担了 HTTP 职责**（3 处） |
| 命名一致 | 5 | 同一个"当前用户"三种形参名：`user` 15 处 / `actor` 19 处 / `me` 2 处 |
| 变更局部性 | 3 | `services.go` 单文件 2,767 行 = service 层代码的 **52.8%**，改哪都是改它 |
| 同题同解 | 3 | 审计 2 套、参数解析 3 套、fetch 4 套、角色守卫 2 层、迁移 9 段克隆 |
| 前端可测试性 | 1 | 前端 **0 个测试文件**，验证只有 `tsc` + `build` |

### 0.2 三个最该先动的地方

1. **`internal/services/services.go`（2,767 行）是"反架构"的核心**：认证、文章、审核、分类、标签、用户、备份、回收站、上传，外加**直接往 `http.ResponseWriter` 写响应**，全在一个文件里。域文件拆分只做了一半（views / batch / related / revisions / slug 抽了，posts / moderation / taxonomy / backup / media 没抽），拆与不拆取决于"哪一轮想起来了"。
2. **同一件事的两套写法已经产生真实缺口**：审计日志有 handler 侧 `recordAudit`（17 处）和 service 侧 `RecordAction`（7 处）两条通道，结果是 24 个写操作里 **6 个完全没有审计**（分类增删改 3 个、上传/删图 2 个、发文章 1 个）。
3. **前端没有数据层**：24 个页面各自手写 `useState` + `useEffect` + `load()` 三态机，`PersonalCenterPage` 一个页面 25 个 `useState` / 623 行；`api.ts` 有 4 处 `fetch`，其中 3 处把 401/429/错误解析又复制了一遍。

### 0.3 现状门禁（本轮实测，作为重构基线）

```
go build ./...      → OK
go vet ./...        → OK
go test -count=1 ./... → 全部 ok（clientip / config / handlers / middleware / models / services）
python scripts/doc_check.py → PASS（70 份文档一致）
```

---

## 1. 取证方法与基线数字

全程只读，不装工具、不加依赖。使用的命令（可复现）：

```bash
# 规模
find internal cmd -name "*.go" | xargs wc -l | sort -rn | head -20
find web/src -name "*.ts*" | xargs wc -l | sort -rn | head -20

# 一致性计数
grep -rn "func (s \*Service)" internal/services/services.go        # 方法清单
grep -rho "require[A-Za-z]*(" internal/handlers/*.go | sort | uniq -c
grep -rn "canReviewPost(" internal --include=*.go                  # 是否被调用
grep -rho "newErr(" internal/services/*.go | wc -l
grep -rn "strconv.Atoi" internal/handlers/*.go
grep -rn "fetch(" web/src --include=*.tsx --include=*.ts
grep -rc "instanceof ApiError" web/src --include=*.tsx
grep -rn "window.confirm\|confirmDialog(" web/src --include=*.tsx
```

| 指标 | 值 |
|---|---|
| 后端函数总数 / 平均长度 | 319 / 21.4 行（大括号匹配精确统计） |
| > 60 行 / > 80 行的函数 | 19 个 / 12 个 |
| 最长函数 | `router.New` 186 行、`UpdatePost` 146 行、`RestoreSnapshot` 141 行、`main` 134 行、`ListPostsPage` 99 行 |
| service 层总行数 | 5,242 行，其中 `services.go` 2,767 行（**52.8%**） |
| `services.go` 内混杂的 DTO 类型 | 27 个（`CreatePostInput`、`UserWithDisabled`、`SnapshotInfo`…散落在方法之间） |
| handler 层总行数 | 1,924 行 / 10 个文件 |
| 路由 | 76 条，全在 `router.go` 一个函数里（190 行） |
| 后端测试 | 39 文件 / 168 个 `Test*` |
| 前端测试 | **0** |

---

## 2. 一致性问题清单（"同一件事，几种写法"）

> 判定标准：**同一类需求存在 ≥2 种实现，且选择依据不是技术理由，而是"哪一轮先写的"**。
> 每条都给"统一到哪一种"，不给"你们自己看着办"。

### 2.1 后端

| # | 现象 | 证据 | 后果 | 统一到 |
|---|---|---|---|---|
| B1 | **service 层直接写 HTTP 响应** | `Service.ServeBackup(w, r, name)` 自己设 `Content-Type`/`Content-Length`/`Content-Disposition` 再 `io.Copy(w, f)`；`Service.ServeUploads(w, r)` 调 `http.ServeFile`、`http.NotFound`；`Service.UploadImage(ctx, r *http.Request)` 以 `_ = ctx` 开头直接解析 multipart | 业务层无法脱离 HTTP 测试；`net/http` 进了 domain 的 import 图；`ctx` 形同虚设（`_ = ctx` 是"我拿到了但我不用"的自证） | service 只返回「文件绝对路径 + 元信息 + 错误」；`w.WriteHeader`/`ServeFile` 全部回 handler |
| B2 | **handler 里写业务分支** | `ListPosts` 用手写 `if page==0 && size==0 && cat==0 && tag==""` 决定返回**裸数组**还是 `PostPage`，并在 handler 里硬编码 `PageSize: 20`；分页默认值在 handler 和 `site_config.page_size` 两处各有一套 | 同一端点两种响应形状（前端 `listPostsPaged` 的注释已承认"可能是数组"）；默认页大小绕过站点配置 | 一个端点一种形状；默认值只在 service 读 `site_config` |
| B3 | **"当前用户"三种形参名** | `grep` 统计：`user` 15 处、`actor` 19 处、`me` 2 处，语义完全相同（当前请求主体） | 读 `UpdatePost(me)` 与 `BanPost(actor)` 需要人脑做一次等价换算；新方法靠感觉选名字 | 统一 `actor`（写操作）/ `viewer`（读操作）二选一，按语义而非轮次 |
| B4 | **集中权限层是半死代码** | `roles.go` 文件头声明"Every 'can X do Y' decision goes through one of these functions"，实际：`canManageUser` **仅 1 处调用**，`canReviewPost` **0 处调用**；`ReviewPost`/`BanPost`/`SendBackPost`/`DeleteBannedPermanently` 各自手写 `switch actor.Role`，`UpdatePost` 再手写一套"admin 只能改 user 的文章" | 这正是 2026-10-15 那个 P0-1（admin 重置 root 密码）的成因结构——规则散落，漏一处就是一个越权 | 所有角色判定必须落到 `policy` 包的 4 个函数；service 内禁止出现 `Role ==` 比较（用 `arch_check.py` 卡） |
| B5 | **审计两条通道** | handler `recordAudit` 17 处（accounts/admin/categories/import/posts）；service `RecordAction` 7 处（create/update/delete post、review、ban、send-back、delete-banned）。**缺口**：`CreateCategory`/`UpdateCategory`/`DeleteCategory`/`UploadImage`/`DeleteUpload`/`CreatePost` 无任何审计 | 同类操作只有一部分留痕；审计覆盖率靠"记得加"，不靠结构保证 | 写用例在 service 事务内统一调用 `recordAudit`；handler 一行都不记 |
| B6 | **错误构造三套** | `newErr(...)` 256 处（主流）；`&ServiceError{...}` 1 处；裸 `fmt.Errorf` 4 处（其中 `parseToken` 的 "invalid token signature"、`uploadToSmms` 的 "smms upload failed" 会直落 500 且把内部措辞暴露给前端） | 裸 error 没有状态码，`StatusOf` 只能兜成 500；同一类"客户端错误"有时 400 有时 500 | 一切客户端可见错误一律 `errs.New(code, msg)`；`fmt.Errorf` 只用于内部 wrap（`%w`） |
| B7 | **查询参数解析三套** | `parseLimit`（3 处调用，含 clamp）；`ListPosts` / `ListArchivePaged` 手写 `strconv.Atoi` + 两段几乎相同的错误消息；`strconv.ParseInt(r.PathValue("revID"))` 再来一套 | `?page=abc` 的语义要靠每个 handler 各自记得拒绝；新增查询参数会自然长成第 4 套 | 一个 `query` 小包：`Int(q,key)`, `IntDefault(q,key,def)`, `Enum(q,key,allowed)`，统一 400 文案 |
| B8 | **响应形状五种** | 204（`DeletePost`）、`{"message":...}`（5 处）、`{"ok":...}`、`{"url":...}`/`{"file":...}`、裸数组 vs `PostPage` | 前端每个调用点都得记住"这个端点返回什么"；`types.ts` 的 `PostPage` 与"可能是数组"冲突 | 成功：资源对象 / 列表用 `{items,total,...}` / 删除 204；不再用 `{message}` 表达成功 |
| B9 | **迁移代码 9 段克隆** | `addAutoBackupColumns`、`addPostSlugColumn`、`addSiteURLColumn`、`addPublicReadColumn`、`addCategoryParentColumn`、`addPageSizeColumn`、`addSmtpColumns`、`addReviewColumns`、`dropScheduledAt` —— 每段 20–25 行，结构完全一致：查 gate → 判表存在 → 判列存在 → `ALTER` → `recordMigration` | 新增一列要抄 25 行；抄漏一步就是线上 500（`site_url` 坏了一个月就是这么来的） | 一个泛型 `addColumn(table, column, ddl)` + `var migrations = []Migration{{Version, Up}}` 注册表 |
| B10 | **`config.go` 四职责合一** | 1,031 行 = 环境变量解析（`Load`/`ValidateSecurity`）+ 连接与连接池 + 种子数据（`seedPresetCategories`/`seedSiteConfig`/`seedDemoData`）+ 迁移与漂移检查 | 任何一处改动都要碰同一文件；`config` 包被 service 反向 import（业务依赖配置初始化） | 拆 `config`（纯 env）/ `store`（连接 + 迁移 + 种子）|
| B11 | **漂移守卫有洞** | `checkSchemaDrift` 手写模型清单 9 个，**缺 `PostRevision`**（第 31 轮新增表）与 `SchemaMigration` | "新增字段必须写迁移"这条不变量对 `post_revisions` 不生效，等于半个盲区 | 从 `models` 包反射遍历（`models.All()` 返回全部实体），删掉手写清单 |
| B12 | **测试文件按"轮次"命名** | `services_test.go`、`services_extra_test.go`、`round19_test.go`、`services_backup_test.go`、`revoke_test.go`…26 个文件命名规则有 4 种 | 想找"审核流测试在哪"只能 grep；新增测试会继续随手命名 | 一律 `<域>_<用例>_test.go`（`review_state_test.go` 已是好样子），`round19_test.go` 拆入对应域 |
| B13 | **包级单例做依赖注入** | `handlers.Init(db, cfg, svc)` 把 `svc` 存包级变量，`WithAuth` 直接读；`router.New` 的注释还写着"db is not a parameter" | 无法并行测试（全局状态）；handler 的函数签名看不出依赖；`router.New` 的 `cfg` 只用来取 CORS，真实依赖被藏起来 | `type API struct{ svc *app.Service; cfg *config.Config }` + 方法接收者，`New(...)` 返回 `*API` |
| B14 | **注释性漂移（AI 追加式编辑的直接证据）** | `GetPostPublic` 注释写 "Increments view_count"，实现早已不增（spec 23）；`router.New` 顶部同一段注释**重复两遍**；`Register` 的 doc comment **重复两遍** | 注释开始撒谎，比没有注释更贵；也说明"改代码时只追加不清理"的习惯 | 每次改动顺手删旧注释；`doc_check.py` 之外再加一项"同一函数 doc 重复"检测 |

### 2.2 前端

| # | 现象 | 证据 | 后果 | 统一到 |
|---|---|---|---|---|
| F1 | **没有数据层，24 个页面各写一套三态机** | 每页 `useState(loading) + useState(data) + load useCallback + useEffect`；`PersonalCenterPage` 25 个 `useState` / 623 行，`AdminUsersPage` 18 个 / 535 行 | 加载态、错误态、重试、竞态防护各写一遍，质量随页面而异（P1-10 的"死重试按钮"就是这么来的） | 一个 `useQuery(fn, deps) → {data, loading, error, reload}`（约 40 行，零依赖）+ `useMutation` |
| F2 | **4 个 `fetch` 落点，3 个绕过统一层** | `core/api.ts` 内：`request`（主路径）、`importMarkdown` 裸 fetch、`downloadBackup` 裸 fetch（自己再解析一次错误 JSON）、`uploadImage` 裸 fetch（再复制一次 JSON 解析） | 401 统一跳转、429 退避、错误解析只对主路径生效；下载/上传/导入三条路径行为不一致（例如 `downloadBackup` 没有 429 重试） | 只保留 3 个基元：`request()` / `upload(formData)` / `download(url) → Blob`，其余全部基于它们 |
| F3 | **角色守卫两层重复** | `main.tsx` 的 `AdminGuarded` 已统一判定；仍有 5 个 admin 页面各自再判一次 `user?.role !== 'admin' && ...`（全仓 `role ===` 共 25 处） | 判据一旦改变（例如新增"编辑器"角色）就有 5 个地方要同步；且 `/admin/banned` 是 root-only，路由层只区分 admin|root，admin 能进页面再被 API 403 | 守卫只写在路由表；root-only 页面在路由层声明 `roles: ['root']` |
| F4 | **错误提示模板复制 54 次** | `e instanceof ApiError ? e.message : t('admin.failed')` 在 22 个文件里出现 54 次；`pushToast('error', ...)` 60+ 次 | 文案与降级策略无法统一演进 | `toastError(e, fallbackKey)` 一个函数 |
| F5 | **业务常量硬编码 8 处** | 密码最小长度 `8`：后端 3 处（注册 / 建号 / 改密）+ 前端 5 处 | 改策略要改 8 个文件 | 后端一个 `PasswordPolicyMinLen`；前端从 `/api/site-config` 或常量模块读 |
| F6 | **i18n 单文件 1,775 行 / 536 键** | `core/i18n.tsx` = 类型 + zh + en 三份平铺；`t()` 未命中时**原样返回 key** | P0-3 那类"界面显示 `statusLabel.pending`"的 bug 结构上必然复发 | 按域拆 `locales/zh/{post,admin,auth}.ts`；`t()` 未命中在 dev 打 `console.warn`，prod 回退英文 |
| F7 | **确认框两套** | `app.tsx` 的 `confirmDialog`（Promise 化）与 `AdminUsersPage` 里一处 `window.confirm` | 后者无法样式化、无法 i18n 统一 | 只用 `confirmDialog` |
| F8 | **CSS 单文件 1,848 行** | `index.css` 无作用域，`.btn` / `.admin-table` / `.post-status-*` 混排 | 样式冲突靠命名自律；页面级样式无处安放 | 短期：按域分文件 `import`；长期（可选）：CSS Modules（Vite 原生支持，零新依赖） |
| F9 | **前端零测试 vs 后端 168 个测试** | `find web/src -name "*.test.*"` → 0；`package.json` 无 test 脚本 | 唯一验证是类型 + 构建，行为回归全靠手点（P2-8 的"翻页重复追加"就是手点漏掉的） | 只给 3 个纯函数模块（`format` / `md` 清洗 / `api` 错误映射）加 vitest，先覆盖"最容易被改坏且无 UI 依赖"的部分 |

---

## 3. 架构问题（不是写法问题，是边界问题）

### A1. 依赖方向被打破（最严重）— ✅ P1 已修复

声明是 `handlers（薄） → services（厚） → models`。当时的实际 `services → net/http`：

```go
func (s *Service) ServeBackup(w http.ResponseWriter, r *http.Request, name string) error
func (s *Service) ServeUploads(w http.ResponseWriter, r *http.Request)
func (s *Service) UploadImage(ctx context.Context, r *http.Request) (string, error)  // 函数体首行 _ = ctx
```

**为什么这是架构级而不是风格级**：这条 import 让"业务层可脱离 HTTP 测试/复用"这件事永久失效；也让 `ctx` 变成装饰品（`_ = ctx` 就是在承认"我需要这个签名但用不上"）。三个端点而已，抽离成本很低，收益是把整层的边界拿回来。

**P1 修复后（2026-10-23 第 55 轮）**：

```go
func (s *Service) ResolveBackupFile(name string) (path string, size int64, err error)
func (s *Service) ResolveUploadFile(name string) (string, error)
func (s *Service) UploadImage(ctx context.Context, filename string, data []byte) (string, error)
// 出站 SM.MS 客户端搬进 internal/imghost（适配器，零领域知识）
```

`services` 包已无任何传输符号，`_ = ctx` 消失且 ctx 真正被使用。副作用：`UploadImage` 可脱离 HTTP 直接测（构造字节切片即可）。

### A2. `services.go` 是"历史沉积岩"，不是模块

| 该文件承担 | 应归属 |
|---|---|
| `Register` / `Login` / `SignToken` / `Authenticate` / `parseToken` | 认证域 |
| `CreatePost` … `DeleteBannedPermanently`（12 个方法） | 内容域 + 审核域 |
| `ListCategories` … `UpdateTag`（10 个方法） | 分类域 |
| `ListUsers` / `SetUserDisabled` | 用户域 |
| `ExportBackup` … `ListSnapshots`（8 个方法） | 备份域 |
| `ListTrash` / `RestoreTrash` / `EmptyTrash` | 回收站 |
| `UploadImage` / `ServeBackup` / `ServeUploads` … | 媒体域 + **传输层** |
| 27 个 DTO 类型散落在方法之间 | DTO 文件 |

判定证据：已抽出的 11 个文件（`views.go` 161 行、`related.go` 64 行、`revisions.go` 114 行…）全部是**后期新增功能**；未抽出的是**早期功能**。也就是说拆分依据是"新增 vs 存量"，不是"职责"。

### A3. 权限：双层校验，但"单一权威"是假的

- handler 层：41 处 `requireAdmin`/`requireRoot`（31 + 10）。
- service 层：`roles.go` 承诺统一，实际 `canReviewPost` 零调用、`canManageUser` 一处调用。
- 结果：`ReviewPost`/`BanPost`/`SendBackPost`/`DeleteBanned` 各写一遍 `switch actor.Role`，`UpdatePost` 再写一遍"admin 只能改 user 的文章"（与 `canReviewPost` 语义近似但规则不同，且没有名字）。

**这是本仓库历史上唯一产生过 P0 越权的结构**（P0-1、P0-7 都是"某处漏了一个 if"）。修法不是"更小心"，是**让手写 if 写不出来**。

### A4. 路由表不表达权限，权限藏在 handler 里

76 条路由全在一个 190 行的 `New()` 里，按注释分块；`withAuth` / `withOptionalAuth` 表达"是否登录"，但**"谁能调"要看 handler 第一行**。于是：

- 无法回答"哪些端点只有 root 能调"（要 grep 10 个文件）。
- 新增端点时"该用 requireRoot 还是 requireAdmin"没有可复制的事实来源。
- `/admin/banned` 前端路由层与后端权限层级不一致（见 F3）。

### A5. 审计是"调用方的责任"，所以必然漏

24 个写操作，两种通道，6 个缺口。审计本该是**写用例的副产品**（在同一事务里顺带落一行），现在是**写用例之后额外要做的一件事**——那它一定会被忘。

### A6. 迁移机制靠"抄上一段"，所以必然抄错

9 段克隆 + 手写漂移清单（且清单已漏 `PostRevision`）。这不是编码水平问题，是**缺少抽象**：一个 `addColumn` 泛型 + 一张注册表就能永久消灭这一类 bug。

### A7. 前端把"页面"当成了"应用"

24 个页面各自持有数据获取、加载态、错误态、权限判断。真正的应用层（数据、会话、权限）只有 `request()` 一个函数。这是典型的"页面即架构"——功能加得越多，重复越多，且**没有任何地方能回答"全站数据从哪来、失效怎么重取"**。

---

## 4. 根因：为什么 AI 会把代码写成这样

不是能力问题，是**流程缺了三样东西**：

| 缺失 | 表现 | 后果 |
|---|---|---|
| **同题唯一解的约定** | 参数解析、审计、守卫、fetch、迁移，每处都有 2–9 种写法 | 后来者（人和 AI）只能"就近模仿"，模仿对象取决于它先打开了哪个文件 |
| **机械门禁** | 只有 `go build/vet/test` 与 `doc_check.py`，没有一条规则检查"service 不许 import net/http" | 违反架构**不会被任何命令发现**，只有人读到才会发现 |
| **重构窗口** | 52 轮里每轮都是"加功能 + 修 bug"，没有一轮是"只搬代码不改行为" | 沉积岩只增不减；文档里的"架构"描述逐渐变成愿望而非事实 |

反证：本仓库凡是**建立了机械校验**的地方都没坏过——`contract_check.py` 之后没再出现跨栈字段漂移；`doc_check.py` 之后母文档没再失真；`checkSchemaDrift` 覆盖的 9 张表没再缺列（漏掉的 `post_revisions` 正好在清单外）。**结论：想让架构不腐化，得让它可被机器检查。**

---

## 5. 目标架构

### 5.1 分层与依赖方向（唯一允许的方向）

```
cmd/server/main.go                 装配：读 env → 开库 → 迁移 → new app → new httpx → ListenAndServe
        │
        ▼
internal/httpx/                    传输层（唯一允许 import net/http 的地方）
  router.go        路由表：[]route{Method, Path, Roles, Auth, Handler}
  respond.go       writeJSON / writeError / 错误映射表单点
  dto.go           请求/响应 DTO（对应前端 types.ts，contract_check 校验）
  middleware/      auth / cors / logger / recover / limiter（复用现有实现，仅搬家）
        │
        ▼
internal/app/                      业务层（禁止 import net/http）
  post/     create.go update.go review.go list.go read.go visibility.go
  auth/     session.go token.go password.go
  user/     manage.go profile.go
  taxonomy/ category.go tag.go
  backup/   export.go import.go snapshot.go autobackup.go
  media/    upload.go store.go
  audit/    record.go
        │
        ▼
internal/store/     连接 + 迁移注册表 + 事务助手（唯一碰 gorm 的地方）
internal/policy/    角色与可见性（canSeePost / canEditPost / canManageUser / canReviewPost）
internal/models/    实体 + 不变量（BeforeCreate 保持）
internal/config/    纯 env 解析（不碰 DB）
```

依赖规则：**`httpx → app → {policy, store, models}`，箭头永不反向。** `app` 与 `policy` 不得出现 `net/http`、不得出现 `gorm.DB` 之外的基础设施类型。

### 5.2 八条硬规则（每条都可被 `arch_check.py` 机械检查）

| # | 规则 | 检查方式 |
|---|---|---|
| R1 | 业务包不得出现**传输符号**（`ResponseWriter` / `*http.Request` / `ServeFile` / `NotFound` / `http.Client` / `NewRequest` / `MaxBytesReader` / `http.Handler`）。`http.StatusXxx` 整数常量不计入——它不是依赖，是数字 | `arch_check.py` 符号扫描（跳过注释行） |
| R2 | 单文件 ≤ 350 行；单函数 ≤ 60 行（存量按阶段收敛，新增即刻生效） | 行数统计 |
| R3 | DTO 只允许出现在 `httpx/dto.go` 与 `app/*/input.go`；实体只在 `models` | 包内 `type X struct` 白名单 |
| R4 | 路由权限在路由表声明（`Roles: []string{"root"}`），handler 内禁止 `requireAdmin`/`requireRoot` | grep + 表结构类型约束 |
| R5 | 审计只能由 `app/*` 的写用例调用（`audit.Record`），`httpx` 零调用 | grep |
| R6 | 客户端可见错误一律 `errs.New(status, msg)`；`fmt.Errorf` 只能 `%w` 包装内部错误 | AST 扫描 |
| R7 | 新增列 = 模型字段 + 一行迁移注册表项；漂移检查从 `models.All()` 反射遍历（禁止手写清单） | 反射清单与 `models` 计数比对 |
| R8 | 前端所有网络出口只有 `core/http.ts`（`request`/`upload`/`download`）；页面不得出现裸 `fetch` | grep `fetch(` |

### 5.3 关键取舍（不做"教科书式"重构）

- **不引入 DI 框架（wire/fx）**：`main.go` 手写装配 20 行足够，依赖注入容器对 1 vCPU 的收益为 0，认知成本为正。
- **不引入 TanStack Query**：`useQuery` 40 行手写即可覆盖本项目的 24 个页面；引依赖违背"前端 3 个包"红线。
- **不拆微服务 / 不拆 `internal/store` 为独立 repo**：单二进制是本项目的价值主张。
- **不把 `services` 包改名**（第一轮）：先在包内按域拆文件（零调用点改动、测试零修改），包级拆分留到文件拆完之后按需再做。**大爆炸重命名会让 168 个测试同时红，等于把重构风险放大 10 倍。**

---

## 6. 迁移路线（8 阶段，每阶段独立可交付、可回滚）

> 总原则：**先立规矩，再搬代码**。否则第 1 阶段搬完，第 2 阶段又长出新写法。
> 每阶段结束都必须：`go build/vet/test` + `tsc` + `vite build` + `doc_check.py` 全绿 + `arch_check.py` 违规数下降。

| 阶段 | 动作 | 涉及文件 | 验收判据 | 风险 |
|---|---|---|---|---|
| **P0 立规矩（不改行为）** | 新增 `scripts/arch_check.py`，把 R1/R2/R3/R4/R5/R8 做成检查；先以 `--baseline` 模式记录当前违规数（预计 R1=1 个包、R4=41 处、R5=17 处、R8=3 处），CI/门禁里只允许下降不允许上升 | 新增 1 脚本 + `AGENTS.md` §4 加一行 | 脚本可跑、基线数写入本报告 §7；顺手修 B11（漂移清单漏 `PostRevision`）、B14（3 处重复/错误注释） | 极低（不改业务） |
| **P1 拿回分层边界** ✅ 已完成 | 把 `ServeBackup`/`ServeUploads`/`UploadImage` 的 HTTP 部分搬到 `handlers`，service 只返回 `(path, meta, err)`；删 `net/http` import 与 `_ = ctx`；出站 SM.MS 客户端抽成 `internal/imghost` | `services.go`（3 函数）+ `handlers/admin.go`/`upload.go` + 1 个测试文件迁移 | 传输符号 12 处 → **0**；7 个行为场景冒烟一致（详见 §7.1 下注） | 中（涉及文件下载/上传，需真跑） |
| **P2 审计单点化** | handler 的 17 处 `recordAudit` 全部下沉到 service 写用例；补 6 个缺口（分类增删改、上传、删图、发文章） | 6 个 handler + 5 个 service 方法 | `grep recordAudit internal/handlers` → 0；新增 6 个审计断言测试 | 低 |
| **P3 拆 `services.go`** | 按域顺序搬文件（纯移动 + 编译）：`auth.go` → `post.go`/`moderation.go` → `taxonomy.go` → `backup.go`/`trash.go` → `media.go`；DTO 收进 `dto.go` | `internal/services/*`（仅文件级移动） | `services.go` 消失；最大文件 ≤ 400 行；168 个测试一个不改仍全绿 | 中（但纯搬移，出错即编译失败，安全） |
| **P4 路由表声明化** | 76 条路由改为 `[]route{{Method, Path, Auth, Roles, Handler}}`，权限从 handler 上移到表；删 41 处 `require*` | `router.go` + 6 个 handler | `/api/admin/banned` 等 root-only 端点由表声明；越权测试（admin→root 端点 403）全绿 | 中高（权限是安全边界，需逐条核对现有 41 处） |
| **P5 迁移注册表** | 9 段克隆 → `addColumn(table, column, ddl)` + `var migrations`；漂移检查改反射遍历 `models.All()` | `config.go` 拆出 `store/migrate.go` | 新库 + 旧库（拷 `data/*.bak` 一份）各跑一次启动，schema 一致；新增列的迁移只需 2 行 | 中（数据库，必须双库验证） |
| **P6 前端数据层** | 新增 `core/http.ts`（3 基元，替换 `api.ts` 里的 3 处裸 fetch）+ `core/useQuery.ts`；先改 3 个最复杂页面（PersonalCenter / AdminUsers / AdminPosts），验证模式后再推其余 | `api.ts` + 5–8 个页面 | 裸 `fetch` → 0；`instanceof ApiError` 54 → ≤5；页面行数下降 ≥30% | 中（页面多，需逐页冒烟） |
| **P7 i18n 与守卫收敛** | i18n 按域拆 5–8 文件；`t()` 未命中加 dev 警告；删 5 处重复 admin 守卫；统一 `confirmDialog` | `core/i18n.tsx` + 5 页面 + `main.tsx` | `i18n.tsx` ≤ 200 行；`role ===` 25 → ≤8（仅路由表与角色常量） | 低 |

**建议节奏**：P0 + P1 一轮（先把规矩和边界定住，收益最大、风险最低）；之后每轮 1 个阶段。**不建议把 P3 与 P4 放同一轮**——一个是搬文件、一个是改权限，混在一起出问题分不清是谁的。

---

## 7. 验收指标（before → after）

### 7.1 机械基线（P0 已落地，2026-10-23 实测）

`scripts/arch_check.py` 已实现 7 项计数，基线存 `scripts/arch_baseline.json`。**以下数字是脚本产出的，不是人眼数的**；与正文预估（R4=41 / R5=17）有偏差，以本表为准（R4 排除 `func require*` 定义行，R5 排除 `func recordAudit` 定义行）。

| 键 | 含义 | 基线 | 目标 | 收口阶段 |
|---|---|---|---|---|
| R1 | 业务包里的**传输符号**（`ResponseWriter` / `*http.Request` / `ServeFile` / `NotFound` / `http.Client` / `NewRequest` / `MaxBytesReader` / `http.Handler`） | **0**（P1 前 12 处 / 1 文件） | 0 | **P1 ✅** |
| R1b | 仅 import `net/http` 取状态码常量的文件（观察项，不阻断） | 11 | — | 可选（AR-1b） |
| R2.files | > 350 行的文件 | 4 | 0 | P3 |
| R2.funcs | > 60 行的函数 | 17 | 0 | P3 |
| R3 | `services` 包内 struct 类型（排除 `Service`/`ServiceError`） | 45 | 2 | P3 |
| R4 | handler 内 `require*` 调用 | 39 | 0 | P4 |
| R5 | handler 内 `recordAudit` 调用 | 18 | 0 | P2 |
| R8 | `core/api.ts` 内超出 1 处的 `fetch` | 3 | 0 | P6 |
| R9 | 单个注释块内的重复行 | 0 | 0 | 已完成 |

> **R1 口径在 P1 修正过，这不是"把门槛调低"。** 旧口径数"import net/http 的文件数"= 11，但逐个文件核对后发现：10 个文件只是用 `http.StatusBadRequest` 这类**整数常量**，1 个文件（`services.go`）才真正持有传输类型。把 10 个文件的常量换成自定义常量，等于改名——包依赖没变、可测试性没变、收益为零。新口径数的是"拿到 `*http.Request` / 往 `ResponseWriter` 写 / 持有 `http.Client`"这类**必须存在一个传输才能跑**的耦合，这才是分层被打破的定义。降幅因此是真实的 12 处 → 0，而不是 11 个文件 → 0 的乐观记分。
> 剩下的 11 个文件记为 **R1b 观察项**（只显示、永不阻断）：它说明"领域层还在用 HTTP 数字说话"，属于可选改造（AR-1b），不该阻塞主线。

> R6（错误构造）与 R7（迁移注册表）需要 AST 级检查，随 P2 / P5 一起做成规则；用正则硬凑会产出误报，反而削弱门禁可信度。
> 语义是**棘轮**：计数只许降不许升；降了要 `--update` 锁住，升了退出码 1 必须修，**不许用 `--update` 掩盖**。

### 7.2 目标指标

| 指标 | 现在 | 目标 | 测法 |
|---|---|---|---|
| `services` 里的传输符号 | 12 处 / 1 文件 | **0** ✅ | `arch_check.py` R1 |
| `services` 仅 import 状态码常量的文件 | 11 | —（观察项，可选 AR-1b） | `arch_check.py` R1b |
| 最大后端文件 | 2,767 行 | ≤ 400 行 | `wc -l` |
| `canReviewPost` 调用数 | 0 | ≥ 5 | `grep -c` |
| service 内 `Role ==` 比较 | 多处手写 | 0（全走 policy） | `arch_check.py` |
| 审计调用点 | 2 处机制 / 24 调用 / 6 缺口 | 1 处机制 / 全覆盖 | 审计覆盖率测试 |
| `require*` 调用 | 41 | 0 | `arch_check.py` |
| 迁移克隆函数 | 9 | 0（1 泛型 + 表） | `grep -c "func add.*Column"` |
| 裸 `fetch` | 4 | 1（`http.ts` 内） | `arch_check.py` |
| `instanceof ApiError` | 54 | ≤ 5 | `grep -c` |
| 前端测试文件 | 0 | ≥ 3 | `find` |
| 前端最大页面 | 623 行 | ≤ 250 行 | `wc -l` |

---

## 8. 明确不做

1. **不换栈**：`net/http` + GORM + glebarez + Preact 没有问题，问题在组织方式。
2. **不加依赖**：后端 6 个 / 前端 3 个是资产，不是债。
3. **不做"一步到位"重写**：本项目 168 个后端测试是唯一的行为保险，重写会同时失去它。
4. **不为分层而分层**：`ctx` 已经 140 处正确传播、事务只有 4 处且都用对了，这两块别动。
5. **不追求"每层一个包"**：`policy` 只有 5 个函数也值得独立成包（它是安全边界），而 `dto` 留在 `httpx` 内即可。

---

## 9. 需要用户拍板的 3 件事

| # | 问题 | 选项 | 我的建议 |
|---|---|---|---|
| D1 | 是否立刻开 P0（立规矩 + 修 3 个小缺陷） | 是 / 等 v1.0 发布后 | **是**。P0 不改行为、不动业务，却能把后续所有轮次的"新增不一致"刹住 |
| D2 | `services` 包是否最终拆成 `app/{post,auth,...}` 多包 | 只拆文件 / 也拆包 | **先只拆文件**（P3），拆包等文件拆完看测试成本再定 |
| D3 | 前端是否引入 3 个纯函数测试（vitest，1 个 devDependency） | 引入 / 不引入 | **引入**。它只测 `format`/`md`/`api`，不进产物，但能挡住"手点漏掉"的回归 |

---

## 10. 附录：本文证据的复现命令

```bash
# 分层越界（期望 P1 后为 0）
grep -rn "net/http" internal/services/*.go

# 权限 helper 是否真被使用（期望 P4 后 canReviewPost ≥ 5）
grep -rn "canReviewPost(\|canManageUser(" internal --include=*.go | grep -v "func "

# 审计双轨（期望 P2 后 handler 侧为 0）
grep -rn "recordAudit(" internal/handlers/*.go | grep -v "func recordAudit"
grep -rn "RecordAction(" internal/services/*.go | grep -v "func (s"

# 迁移克隆（期望 P5 后为 0）
grep -c "^func add.*Column\|^func drop" internal/config/config.go

# 形参命名三套（期望 B3 后只剩一种）
grep -rho "func (s \*Service) [A-Za-z]*(\(ctx context.Context, \)\?\(me\|user\|actor\) " internal/services/*.go | sed 's/.*[,(] //' | sort | uniq -c

# 前端裸 fetch（期望 P6 后只剩 http.ts 一处）
grep -rn "fetch(" web/src --include=*.ts --include=*.tsx

# 前端角色判断散落（期望 P7 后 ≤ 8）
grep -rc "role ===" web/src --include=*.tsx | awk -F: '{s+=$2} END {print s}'
```

---

> 备注：本报告只做静态取证，**没有运行中的实例**（第 52 轮遗留：服务需独立终端启动）。因此所有"行为层面"的结论均来自代码与控制流，不来自运行时观测；涉及运行时才能确认的项（如下载/上传路径的实际响应头）已在 P1 验收里明确要求冒烟。
