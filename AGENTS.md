# AGENTS.md — AI 协作者入口

> 本文件是**任何 AI 编码助手**(Claude Code / Cursor / Codex / OpenHands / WorkBuddy)进入本仓库的第一份必读。
> 完整协作标准见 `docs/AI_STANDARD.md`;交接全貌见 `HANDOVER.md`。三者关系:本文件管"速查与红线",`AI_STANDARD.md` 管"流程与纪律",`HANDOVER.md` 管"项目现状与陷阱细节"。

---

## 1. 项目一句话

面向低配 VPS(1 vCPU / 1.5 GB / 5 GB)的可自托管全栈内容站点:Go 单二进制 + SQLite 单文件(后端),Preact 单页(前端),前后端分离。**功能深度优先于广度**,追求极致资源占用比与可维护性。

## 2. 硬约束(违反即返工)

| 约束 | 说明 |
|------|------|
| **不换框架** | 后端 Go `net/http` + GORM + `glebarez/sqlite`(零 CGO);前端 Preact + Vite + TS。没有性能/功能瓶颈证据时,不为了"更现代"改栈 |
| **不加重依赖** | 后端 ~12 个模块、前端 2 个 npm 包(`wouter` + `marked`)。加依赖前先问"能不能手写/标准库解决" |
| **低资源优先** | SQLite 连接池 4/2、限流内存态、无 Redis、无后台常驻任务。任何改动都要过"1 vCPU / 1 GB 可用内存"这关 |
| **root 唯一且不可经 API 创建/提升** | 角色 `root/admin/user`;root 全库唯一,仅首启 seed + 迁移 `0003_root_migration` 产生 |
| **可见性判定只在 service 层** | `canSeePost`(`internal/services`)是文章五态 `published/private/pending/rejected/banned` 的唯一权威,handler/router 不写可见性 if |
| **权限 helper 集中在 `services/roles.go`** | 不写散落的 `me.Role != "admin"`;`canManageUser` / `canReviewPost` 等是唯一入口 |
| **错误契约用英文** | handler/service 返回给前端的 `error` 字段用英文短句;界面文案走 i18n 键(`web/src/core/i18n.tsx`,点分 key + `{var}` 插值) |

## 3. 关键陷阱(改代码前必读,详见 `HANDOVER.md` §7)

1. **GORM AutoMigrate + glebarez 启动崩溃**:误判重建表时 copy-rename 的 `INSERT` 漏 NOT NULL 列(经典报错 `posts__temp.title constraint failed`)。当前修复:`config.go` 只对**不存在的表**跑 AutoMigrate,已存在表由 `runManualMigrations` 管理。**改 `internal/config/config.go` 不要回退成无条件 AutoMigrate。**
2. **读接口必须 `WithOptionalAuth`**(匿名放行、坏 token 401),勿换成 `WithAuth`。
3. **限流双轨**:通用桶 `middleware.Limiter`(按 user ID / IP,默认 20 rps / 50 burst,`.env` 可调)+ 登录专用 `LoginLimiter`(IP/账号双桶 + 5 次失败锁)。`.env` 只在进程启动时读一次,改限流值**必须重启后端**。
4. **测试 fixture 造 root/admin 必须直改 DB**(`db.Model(&models.User{}).Update("role", ...)`),这是 handlers 测试 `setup(t)` 返回 `(*services.Service, *gorm.DB)` 的原因。
5. **文件读取被审批拦截时**(曾发生在 `feed_test.go`),派 `Agent`(Explore 子代理)读取可绕过。
6. **Windows 中文编码**:读写仓库内文件一律 UTF-8;涉及本机 GBK 文件的操作先确认编码。

## 4. 怎么跑 / 怎么验

```bat
EchoAPI.bat            :: 一键起后端 :8080 + 前端 :5173(幂等,自动开浏览器)
EchoAPI.bat status|stop|restart|dev|build|help
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

**提示词模板库**:`docs/prompts/`(继续轮次 / 新功能 / 修 bug / 自查 / 验证记录 五个场景,可直接粘贴)。

## 6. 文档地图

| 想知道什么 | 去哪 |
|---|---|
| 项目现状 / 怎么接手 | `HANDOVER.md` |
| AI 协作流程与纪律 | `docs/AI_STANDARD.md` |
| 待办清单(唯一权威) | `docs/TODO.md` |
| 轮次开发史 | `PROGRESS.md` |
| 功能规格(EARS 语法) | `docs/specs/`(01–21) |
| 重大决策(不可变) | `docs/decisions/`(0001–0004) |
| 架构设计 | `docs/ARCHITECTURE.md` |
| 路由/API/页面速查 | `docs/PROJECT_OVERVIEW.md` |
| 审计报告(最新) | `docs/AUDIT_2026-10-15.md` |
| 外部参考项目提炼 | `docs/reference/AI_CODING_REFERENCES.md` |
| 文档体系总入口 | `docs/README.md` |
