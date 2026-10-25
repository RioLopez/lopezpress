> 类别 A · AI 指挥/约束（母文档族）— 由根目录 `AGENTS.md` 统管，新对话须先读。
>
> **⚠️ 先读 `MASTER.md`（文档总索引 + 自驱协议）**：它告诉你"有哪些文档、去哪找、怎么自己跑起来"。
> 本文件是它的硬约束附表：MASTER 管路由，本文件管红线。两者冲突时以 `docs/AI_STANDARD.md` 为准。

# AGENTS.md — AI 协作者入口

> 本文件是**任何 AI 编码助手**(Claude Code / Cursor / Codex / OpenHands / WorkBuddy)进入本仓库的第一份必读。
> 完整协作标准见 `docs/AI_STANDARD.md`;交接全貌见 `HANDOVER.md`。三者关系:本文件管"速查与红线",`AI_STANDARD.md` 管"流程与纪律",`HANDOVER.md` 管"项目现状与陷阱细节"。
> **文档两类制(2026-10-22 确立)**:全仓库文档分 A(指挥约束 AI,本文件为母文档) / B(项目参考)两类,每个文件顶部有 `> 类别 A/B` 横幅。**新对话无历史上下文**,AI 只凭本母文档 + 其指向的 Class A 文档恢复约束,不凭记忆推测,避免幻觉。

---

## 1. 项目一句话

面向低配 VPS(1 vCPU / 1.5 GB / 5 GB)的可自托管全栈内容站点:Go 单二进制 + SQLite 单文件(后端),Preact 单页(前端),前后端分离。**功能深度优先于广度**,追求极致资源占用比与可维护性。

## 2. 硬约束(违反即返工)

| 约束 | 说明 |
|------|------|
| **不换框架** | 后端 Go `net/http` + GORM + `glebarez/sqlite`(零 CGO);前端 Preact + Vite + TS。没有性能/功能瓶颈证据时,不为了"更现代"改栈 |
| **不加重依赖** | 后端 `go.mod` 直接依赖 7 个（sqlite 驱动 / jwt / uuid / x-crypto / x-image / x-time / gorm，另有 11 个 indirect）；前端 3 个运行依赖(`preact` + `wouter` + `marked`)+ 4 个 devDependency。加依赖前先问"能不能手写/标准库解决"；非加不可时写进 `docs/decisions/` 说明豁免理由（先例：0005 的 `golang.org/x/image`） |
| **低资源优先** | SQLite 连接池 4/2、限流内存态、无 Redis。**唯一的后台 goroutine 是缩略图 worker**（单协程 + 张间 150 ms），它也必须受此约束：解码上限比请求路径更严（24 MP vs 40 MP），因为后台 OOM 会杀掉整个进程 |
| **root 唯一且不可经 API 创建/提升** | 角色 `root/admin/user`;root 全库唯一,仅首启 seed + 迁移 `0003_root_migration` 产生 |
| **可见性判定只在 service 层** | `canSeePost`(`internal/services`)是文章五态 `published/private/pending/rejected/banned` 的唯一权威,handler/router 不写可见性 if |
| **权限 helper 集中在 `services/roles.go`** | 不写散落的 `me.Role != "admin"`;`canManageUser` / `canReviewPost` 等是唯一入口 |
| **错误契约用英文** | handler/service 返回给前端的 `error` 字段用英文短句;界面文案走 i18n 键(`web/src/core/i18n.tsx`,点分 key + `{var}` 插值) |
| **单对话 ≤ 30 轮** | 以"用户消息 + AI 回复"为一轮;累计接近 30 轮时,AI 必须主动总结进度/待办/未决项并建议开启新对话,不在同一对话无限延展,防上下文膨胀与幻觉。新对话重读本母文档即恢复全部约束 |

## 3. 关键陷阱(改代码前必读,详见 `HANDOVER.md` §7)

1. **GORM AutoMigrate + glebarez 启动崩溃**:误判重建表时 copy-rename 的 `INSERT` 漏 NOT NULL 列(经典报错 `posts__temp.title constraint failed`)。当前修复:`config.go` 只对**不存在的表**跑 AutoMigrate,已存在表由 `runManualMigrations` 管理。**改 `internal/config/config.go` 不要回退成无条件 AutoMigrate。**
2. **读接口必须 `WithOptionalAuth`**(匿名放行、坏 token 401),勿换成 `WithAuth`。
3. **限流双轨**:通用桶 `middleware.Limiter`(按 user ID / IP,默认 20 rps / 50 burst,`.env` 可调)+ 登录专用 `LoginLimiter`(IP/账号双桶 + 5 次失败锁)。`.env` 只在进程启动时读一次,改限流值**必须重启后端**。
4. **测试 fixture 造 root/admin 必须直改 DB**(`db.Model(&models.User{}).Update("role", ...)`),这是 handlers 测试 `setup(t)` 返回 `(*services.Service, *gorm.DB)` 的原因。
5. **文件读取被审批拦截时**(曾发生在 `feed_test.go`),派 `Agent`(Explore 子代理)读取可绕过。
6. **Windows 中文编码**:读写仓库内文件一律 UTF-8;涉及本机 GBK 文件的操作先确认编码。
7. **新增模型字段必须同时写手工迁移**（第 46 轮血泪）：已存在的表不走 AutoMigrate，少一条 `ALTER TABLE` 就是运行时 500（`site_url` 曾因此坏了一个月没人发现）。启动期 `checkSchemaDrift` 会把缺列打成 `WARN`。
8. **新增通配路由先查同前缀形状**（第 48 轮崩溃）：`GET /api/posts/by-slug/{slug}` 与 `GET /api/posts/{id}/public` 在 Go 1.22 ServeMux 里互为歧义，**注册时直接 panic，进程起不来**。新永久链接端点因此放在 `/api/permalinks/{slug}`。
9. **`*int64` 不能直接进原生 SQL**（第 52 轮）：`database/sql` 没有 `*int64` 的 driver.Value，`Raw(...)` 传它会报 `unsupported type`；先解包成 `any`（nil 或值）。
10. **root 专属设置不能被 admin 的整表单保存覆盖**（第 52 轮）：站点设置是「读全量 → 改 → 整体 PUT」，admin 提交时不带 `auto_backup` 就会把它重置为 false。服务端必须显式继承原值。
11. **「缺省=关闭」的开关与「0=不限制」的数值一律继承**（第 77/79 轮）：`allow_*`、`email_verify_*`、四个配额字段都属于这一类。handler 要从**原始 map** 判断 key 是否出现，未出现则继承库值，数值再 clamp 到安全区间。
12. **图片软删不能用 `gorm.DeletedAt`**（第 77 轮）：撤下的图必须继续服务已发布文章，否则破图的是无辜的作者。`UploadFile.DeletedAt` 是普通 `*time.Time`，列表自己过滤。
13. **上传顺序固定：去重 → 配额 → 写盘**（第 77 轮）：配额在写盘前（否则留半截文件），去重在配额前（复用不占额度）。测配额时**每次必须换不同字节**，否则会先命中去重。
14. **队列待办状态放数据库**（第 78 轮）：`thumb_status` 是唯一真相，内存 channel 只做唤醒且满了就丢；只启动时扫一次会漏积压，必须有周期 sweep。
15. **换了个入口做同一件事，要回头堵旧入口**（第 79 轮）：开着邮箱验证时前端把邮箱改成只读，但 `PUT /api/auth/me` 原本仍能改 → 一次 PUT 绕过整套验证码。现在 403 + 单测。

## 4. 怎么跑 / 怎么验

```bat
LopezPress.bat            :: 一键起后端 :8080 + 前端 :5173(幂等,自动开浏览器)
LopezPress.bat status|stop|restart|dev|build|help
```

```powershell
# 验证门禁(改完代码必须全绿,缺一不可)
go build ./...                    # 后端编译
go vet ./...                      # 静态检查
go test -count=1 ./internal/...   # 单测(跳过缓存;当前 279 用例)
cd web
node node_modules/typescript/bin/tsc --noEmit   # 前端类型(.bin/tsc 是 bash 脚本,别直接喂 node)
NODE_OPTIONS= npm test -- --no-file-parallelism  # vitest(当前 54 用例);必须串行——并行时沙箱 EPERM 会静默漏掉测试文件(实测 54→32)
npm run build                     # 前端生产构建
cd ..
python scripts/doc_check.py       # 改过文档时必须跑:母文档索引/份数/横幅是否仍一致
python scripts/arch_check.py      # 改过代码时必须跑:架构债只许降不许升
python scripts/contract_check.py  # 改过路由/前端调用时必须跑:method+path 契约
python scripts/security_check.py  # 改过任何输入处理/路由/日志时必须跑:安全反模式棘轮(S1-S9)
```

> `security_check.py` 也是**棘轮**。它拦的是"同一类漏洞换个入口又长回来":
> 路由 auth 等级不自洽、handler 绕过体上限、**handler 自设上限 > 路由声明(S9)**、
> 状态转换无条件 UPDATE、字段无上限、SQL 字符串拼接、`template.HTML`、日志里的密钥。基线和 `arch_baseline.json`
> 一样只降不升。**上线级的安全结论看 `docs/SECURITY_BASELINE.md`(按攻击面组织),
> 不是看渗透报告修了几条。**

> **真机验证脚本**(需隔离实例,不是单测替代品):
> `scripts/defence_smoke.py`(39 项攻击用例) / `scripts/backup_restore_drill.py`(备份恢复三阶段)。
> 两者都用 `BASE_URL`/`*_URL` 参数化,非 0 退出即有攻击仍成立或恢复失败。

> `arch_check.py` 是**棘轮**不是 linter:存量违规记在 `scripts/arch_baseline.json`,只拦"变多"。违规数下降后用 `--update` 锁住战果;`--baseline` 仅首次/重建时用。退出码 1 = 有规则回升,必须修,不许直接 `--update` 掩盖。

> 本机 Go 路径:`C:\Users\LopezPC\go-sdk\go\bin`(旧记录 `C:\Users\Lopez\go-install\go\bin` 已失效)。
> 长期运行服务必须独立终端启动,不要用对话内后台进程(会话结束会被回收)。

## 5. 工作流(必须遵守)

```
读上下文(HANDOVER + PROGRESS + TODO + memory)
  → 定任务(小改动直接做;功能/结构改动先写 spec + plan)
  → 实现(遵守红线,权限/可见性走 service 层)
  → 验证门禁全绿 + 行为验证
  → 记录(PROGRESS 轮次 / memory 日志 / TODO 勾销)
```

用户说「**继续**」= 读 `PROGRESS.md` 最新轮次 + `docs/TODO.md`,自主挑选下一优先级任务推进,不需要用户重复给上下文。

**单对话轮次上限**:见 §2「单对话 ≤ 30 轮」;接近上限时主动收尾并建议新对话,不要硬撑。

**提示词模板库**:`docs/prompts/`(继续轮次 / 新功能 / 修 bug / 自查 / 验证记录 五个场景,可直接粘贴)。

## 6. 文档两类制(避免幻觉的核心,2026-10-22 确立)

全仓库文档分两类。**新对话没有历史上下文**,AI 只靠本母文档 + 其指向的 Class A 文档恢复全部约束,**不凭记忆推测项目状态**,一切以文档为准。每个文件顶部都有 `> 类别 A/B` 横幅,单独打开也能自识别。

### 6.1 Class A — AI 指挥/约束(母文档族)
由**`MASTER.md` 统管**（第 41 轮起：用户只说一句「去母文档找」）。AI 进入仓库**必须依次读**：
`MASTER.md`（总索引 + 自驱协议）→ `AGENTS.md`（本文件，速查 + 红线）→ `docs/AI_STANDARD.md` → `HANDOVER.md`（§0+§7）→ `docs/TODO.md`。

| 文档 | 角色 |
|---|---|
| `MASTER.md` | **唯一入口**:文档总索引 + 自驱协议 + 事实基线 + 断点表 + 用 `doc_check.py` 校验 |
| `AGENTS.md`(本文件) | 速查卡:硬约束 + 15 条陷阱 + 验证命令 + 本两类制 |
| `docs/AI_STANDARD.md` | 五阶段工作流 / 任务分级 T0–T3 / 验证门禁 / 十条红线 R1–R10 / 上下文纪律 |
| `HANDOVER.md` | 项目现状 + 关键陷阱(§7 共 37 条,改代码前必读) |
| `docs/TODO.md` | 待办权威(AI 选任务唯一依据) |
| `docs/decisions/`(0001–0006) | 不可变跨页决策,AI 不得违反 |
| `docs/prompts/`(01–05) | 场景提示词模板(继续/新功能/修bug/自查/验证) |

### 6.2 Class B — 项目/内容/参考
产品与工程文档,AI 按需查阅,**不约束流程**。总入口 `docs/README.md`。

| 文档 | 角色 |
|---|---|
| `README.md` / `PROGRESS.md` / `api.md` | 上手 / 轮次史 / API 参考 |
| `docs/ARCHITECTURE.md` / `docs/PROJECT_OVERVIEW.md` | 架构设计 / 代码地图速查 |
| `docs/specs/`(01–37) | 页面功能规格(EARS 验收语法);36 = 图库、37 = 邮箱验证 |
| `docs/FEATURE_DESIGN_MATRIX.md` | 44 项功能 × 六维设计完整度(哪个功能只是能跑、设计没写清) |
| `docs/PRODUCT_REVIEW.md` / `docs/COMPETITIVE_GAP.md` | 买家视角评分 / 竞品对标缺口 |
| `docs/NEXT_VERSION_BACKLOG.md` | v0.2 未立项的想法池 |
| `docs/DESIGN_SYSTEM.md` | 全站 UI 标准 |
| `docs/BUSINESS_LOGIC.md` | 业务逻辑反向梳理 |
| `docs/AUDIT_REPORT.md` / `docs/AUDIT_2026-10-15.md` / `docs/archive/` | 安全/功能审计 |
| `docs/reference/AI_CODING_REFERENCES.md` | 外部方法论参考 |
| `docs/NEXT_VERSION_BACKLOG.md` | v0.2 规划草稿 |
| `deploy/**`(README/LOCAL) | 部署文档 |

> 分类口径:凡"约束 AI 怎么干/不能干"的(红线、流程、决策、陷阱、待办权威)归 A;凡"产品/工程是什么样"的(规格、架构、审计、部署、API)归 B。`docs/decisions/` 因"不可变、AI 不得违反"而归 A;`docs/specs/` 因是"页面应是什么样"而归 B。
