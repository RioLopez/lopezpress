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
| **不加重依赖** | 后端 `go.mod` 直接依赖 6 个（sqlite 驱动 / jwt / uuid / x-crypto / x-time / gorm，另有 10 个 indirect）；前端 3 个 npm 包(`preact` + `wouter` + `marked`)。加依赖前先问"能不能手写/标准库解决" |
| **低资源优先** | SQLite 连接池 4/2、限流内存态、无 Redis、无后台常驻任务。任何改动都要过"1 vCPU / 1 GB 可用内存"这关 |
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

## 4. 怎么跑 / 怎么验

```bat
LopezPress.bat            :: 一键起后端 :8080 + 前端 :5173(幂等,自动开浏览器)
LopezPress.bat status|stop|restart|dev|build|help
```

```powershell
# 验证门禁(改完代码必须全绿,缺一不可)
go build ./...          # 后端编译
go vet ./...            # 静态检查
go test ./...           # 单测
cd web; npx tsc --noEmit   # 前端类型
npm run build           # 前端生产构建
```

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
由**本文件(根目录 `AGENTS.md`)统管**。AI 进入仓库**必须依次读**:`AGENTS.md`(本文件) → `docs/AI_STANDARD.md` → `HANDOVER.md`(§0+§7) → `docs/TODO.md`。

| 文档 | 角色 |
|---|---|
| `AGENTS.md`(本文件) | **母文档**:速查 + 红线 + 验证命令 + 本两类制 |
| `docs/AI_STANDARD.md` | 五阶段工作流 / 任务分级 T0–T3 / 验证门禁 / 十条红线 R1–R10 / 上下文纪律 |
| `HANDOVER.md` | 项目现状 + 关键陷阱(§7,改代码前必读) |
| `docs/TODO.md` | 待办权威(AI 选任务唯一依据) |
| `docs/decisions/`(0001–0004) | 不可变跨页决策,AI 不得违反 |
| `docs/prompts/`(01–05) | 场景提示词模板(继续/新功能/修bug/自查/验证) |

### 6.2 Class B — 项目/内容/参考
产品与工程文档,AI 按需查阅,**不约束流程**。总入口 `docs/README.md`。

| 文档 | 角色 |
|---|---|
| `README.md` / `PROGRESS.md` / `api.md` | 上手 / 轮次史 / API 参考 |
| `docs/ARCHITECTURE.md` / `docs/PROJECT_OVERVIEW.md` | 架构设计 / 代码地图速查 |
| `docs/specs/`(01–22) | 页面功能规格(EARS 验收语法) |
| `docs/DESIGN_SYSTEM.md` | 全站 UI 标准 |
| `docs/BUSINESS_LOGIC.md` | 业务逻辑反向梳理 |
| `docs/AUDIT_REPORT.md` / `docs/AUDIT_2026-10-15.md` / `docs/archive/` | 安全/功能审计 |
| `docs/reference/AI_CODING_REFERENCES.md` | 外部方法论参考 |
| `docs/NEXT_VERSION_BACKLOG.md` | v0.2 规划草稿 |
| `deploy/**`(README/LOCAL) | 部署文档 |

> 分类口径:凡"约束 AI 怎么干/不能干"的(红线、流程、决策、陷阱、待办权威)归 A;凡"产品/工程是什么样"的(规格、架构、审计、部署、API)归 B。`docs/decisions/` 因"不可变、AI 不得违反"而归 A;`docs/specs/` 因是"页面应是什么样"而归 B。
