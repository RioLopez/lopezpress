> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。
>
> **总入口在仓库根 `../MASTER.md`**（文档总索引 + 自驱协议）。本文件只是 `docs/` 目录的内部索引。

# lopezpress 文档体系

> **文档两类制(2026-10-22)**:全仓库文档分 A(指挥约束 AI,母文档 `../AGENTS.md`) / B(项目参考,本目录为 B 总入口)两类。AI 流程约束以 Class A 为准;本目录是产品/工程文档索引。每个文件顶部有 `> 类别 A/B` 横幅。

本目录是项目的**规范与决策体系**，用于避免"想一出是一出、做一半忘了目标"。
它分两层：

- **`specs/`**：按页面切分的功能规范（空间维度）——"这个页面应该长什么样"
- **`decisions/`**：跨页面的重大决策记录（时间维度，append-only）——"为什么这么定"

两者与根目录的 `PROGRESS.md`（轮次日志，时间线）职责不重叠：
`PROGRESS.md` 记"什么时候做了什么"，`specs/` 记"应该是什么样"，`decisions/` 记"为什么"。

---

## 1. 体系说明

### 1.1 为什么分两层

- **spec 可反复更新**：页面行为会演进，spec 跟着改，是"活的"。
- **decision 不可变**：一个决策一旦被接受（Accepted），不删不改；推翻它要写新决策并
  把旧的标记为 `Superseded by [新决策]`。历史本身就是价值。

### 1.2 状态机

**spec 状态**（写在文件头 `> 状态：` 处）：

```
Draft → In Progress → Done
                       ↘ Deprecated（页面被移除时）
```

- `Draft`：目标已定，还没开始 / 还没确认
- `In Progress`：正在开发，按验收标准逐条打勾
- `Done`：全部验收标准达成
- `Deprecated`：页面已下线

**decision 状态**：

```
Proposed → Accepted
            ↘ Superseded by [新决策]
            ↘ Deprecated
```

### 1.3 EARS 验收语法

spec 的验收标准用 EARS（Easy Approach to Requirements Syntax）写，保证可测试、无歧义：

- `WHEN [条件] THEN [期望结果]`
- 例：`WHEN 用户点击"批量禁用"且选中 0 个用户 THEN 按钮置灰不可点`

---

## 2. 索引

### 2.1 specs/（按页面 / 按功能域）

> 状态三档：`已实现（spec 未写）`＝代码已在跑、规范待补；`已实现（spec 已写）`＝代码与规范均已就位；`已废弃`。
> 截至 2026-10-23（第 52 轮），**01–35 号 spec 全部为「已实现（spec 已写）」**。
> 注：13 号未使用（编号跳空）；21 号为事后补录（第 25 轮口述需求直接实现，事后按本规范补齐）。

| 编号 | 页面 / 功能域 | 路由 | 状态 | spec 文件 |
|------|------|------|------|-----------|
| 01 | 文章列表 + 归档 | `/`、`/archive` | 已实现（spec 已写） | `specs/01-list-page.md` |
| 02 | 阅读页 | `/read/:id` | 已实现（spec 已写） | `specs/02-read-page.md` |
| 03 | 编辑器（旧版，已被 19 取代） | `/create`, `/edit/:id` | 已实现（spec 已写） | `specs/03-editor.md` |
| 04 | 登录/注册 | `/login`, `/register` | 已实现（spec 已写） | `specs/04-auth.md` |
| 05 | 站点设置 | `/settings` | 已实现（spec 已写） | `specs/05-settings.md` |
| 06 | 管理后台布局 | `/admin/*` | 已实现（spec 已写） | `specs/06-admin-layout.md` |
| 07 | 分类管理 | `/admin/categories` | 已实现（spec 已写） | `specs/07-admin-categories.md` |
| 08 | 标签管理 | `/admin/tags` | 已实现（spec 已写） | `specs/08-admin-tags.md` |
| 09 | 用户管理（第 25 轮扩为三层角色） | `/admin/users` | 已实现（spec 已写） | `specs/09-admin-users.md` |
| 10 | 备份管理 | `/admin/backup` | 已实现（spec 已写） | `specs/10-admin-backup.md` |
| 11 | 回收站 | `/admin/trash` | 已实现（spec 已写） | `specs/11-admin-trash.md` |
| 12 | 404 页 | 兜底路由 | 已实现（spec 已写） | `specs/12-not-found.md` |
| 14 | 登录日志 | `/admin/users/{id}/logins` | 已实现（spec 已写） | `specs/14-login-log.md` |
| 15 | 作者主页 | `/u/:username` | 已实现（spec 已写） | `specs/15-profile-author.md` |
| 16 | 注册开关 | `/register` | 已实现（spec 已写） | `specs/16-registration.md` |
| 17 | 右上角用户菜单 | 全局 | 已实现（spec 已写） | `specs/17-user-menu.md` |
| 18 | 个人中心 | `/me` | 已实现（spec 已写） | `specs/18-personal-center.md` |
| 19 | 写文章编辑器（取代 03） | `/create`, `/edit/:id` | 已实现（spec 已写） | `specs/19-write-editor.md` |
| 20 | 文章可见范围（第 25 轮扩为五值状态） | `/create`, `/edit/:id`, `/me` | 已实现（spec 已写） | `specs/20-visibility.md` |
| 21 | 三层角色 + 内容审核 + root 封禁库 | `/admin/review`, `/admin/banned` | 已实现（spec 已写） | `specs/21-roles-moderation.md` |
| 22 | root 密码离线恢复 | CLI | 已实现（spec 已写） | `specs/22-root-password-recovery.md` |
| 23 | 浏览量指纹去重 + 上报接口 | `POST /api/posts/{id}/view` | 已实现 | `specs/23-post-views.md` |
| 24 | 首页纯分页 + 每页条数配置 | `/` | 已实现 | `specs/24-pagination.md` |
| 25 | 标签改名 + 计数跳转 | `/admin/tags` | 已实现 | `specs/25-tag-rename.md` |
| 26 | 后台文章管理 + 批量操作 | `/admin/posts` | 已实现 | `specs/26-admin-posts-batch.md` |
| 27 | 公开阅读开关 + 两层分类树 | `/admin/site`, `/admin/categories` | 已实现 | `specs/27-read-access-and-category-tree.md` |
| 28 | 读者链路（归档分页/robots/封面/空态） | `/archive`, `/` | 已实现 | `specs/28-reader-flows.md` |
| 29 | 文章永久链接（slug） | `/p/:slug` | 已实现 | `specs/29-permalinks.md` |
| 30 | 封禁闭环（列表内发起封禁） | `/admin/posts` | 已实现 | `specs/30-ban-closure.md` |
| 31 | 修订历史与回滚 | `/edit/:id` | 已实现 | `specs/31-revisions.md` |
| 32 | Markdown 批量导入 | `/admin/posts` | 已实现 | `specs/32-markdown-import.md` |
| 33 | 仪表盘统计（趋势/待审/Top5） | `/admin` | 已实现 | `specs/33-dashboard-insights.md` |
| 34 | 每日自动备份 | `/admin/backup` | 已实现 | `specs/34-auto-backup.md` |
| 35 | 相关文章 + 面包屑 | `/read/:id` | 已实现 | `specs/35-related-and-breadcrumb.md` |

> 模板：`specs/_TEMPLATE.md`。新页面加编号 + 一行索引。

### 2.2 decisions/（跨页面决策，append-only）

| 编号 | 决策 | 状态 | 文件 |
|------|------|------|------|
| 0001 | 纯 Go SQLite 驱动不支持 FTS5，搜索保留 LIKE | Accepted | `decisions/0001-pure-go-sqlite-no-fts5.md` |
| 0002 | 编辑器草稿 localStorage TTL 30 天 + 保存后回收 | Accepted | `decisions/0002-localstorage-draft-ttl-30d.md` |
| 0003 | Token 存储写死 localStorage（Bearer 方案） | Accepted | `decisions/0003-token-storage.md` |
| 0004 | CORS 白名单模式（受限模式不回显未知 origin） | Accepted | `decisions/0004-cors.md` |

> 模板：`decisions/_TEMPLATE.md`。新决策顺延编号。

### 2.3 设计系统（跨全站 UI 标准）

| 文档 | 作用 | 文件 |
|------|------|------|
| 设计系统与美学规范 v1 | 全站 UI 唯一标准：设计哲学 / 参考标杆 / 设计 Token / 布局 / 组件 Do&Don't / 博客排版 / 暗色 / a11y / 自查清单 | `DESIGN_SYSTEM.md` |

> 改任何界面前先读 `DESIGN_SYSTEM.md`；其中 Token 大多已在 `web/src/index.css`（现代设计系统层 + 响应式规范层）落地。

### 2.4 问题清单与归档

| 文档 | 作用 | 文件 |
|---|---|---|
| 待办清单（**唯一权威**） | P1/P2/P3 + 功能缺口 + 设计取舍 + 实测无问题项 | `TODO.md` |
| 安全审计报告 | 10 项安全加固的落地记录（2026-09-24 轮） | `AUDIT_REPORT.md` |
| 代码地图 / 接手速查 | 路由 / API 端点 / 页面 / 改码红线 | `PROJECT_OVERVIEW.md` |
| 首轮功能审计快照 | **已归档**，结论已全部落入 TODO.md；勿再当待办 | `archive/2026-10-14-FUNCTIONAL_AUDIT.md` |

### 2.5 AI 协作体系（2026-10-19 建立）

| 文档 | 作用 | 文件 |
|---|---|---|
| AGENTS 入口（**所有 AI 工具第一份必读**） | 速查 + 红线 + 验证命令 | 根目录 `../AGENTS.md` |
| AI 协作标准（**流程唯一标准**） | 五阶段工作流 / 任务分级 / 验证门禁 / 十条红线 / 上下文纪律 | `AI_STANDARD.md` |
| 提示词模板库 | 继续 / 新功能 / 修 bug / 自查 / 验证记录 五个场景 | `prompts/` |
| 外部参考提炼 | GitHub 参考项目（spec-kit、BMAD 等）借鉴了什么、没抄什么 | `reference/AI_CODING_REFERENCES.md` |

---

## 3. 工作流

核心原则：**先改文档，再改代码。** 新想法一律先进 spec / decision，不直接动代码。

```
新想法 / 新需求
   │
   ├─ 是"某个页面的行为调整"？ ──→ 改对应 spec（specs/NN-xxx.md）
   │
   └─ 是"跨页面的重大决策"？ ──→ 新建 decision（decisions/00NN-xxx.md）
                                    │
                                    └─ 若影响某些 spec → 更新那些 spec 的"相关决策"
   │
   ▼
确认目标（spec 状态 Draft → In Progress）
   │
   ▼
开发（按 spec 验收标准逐条实现 + 打勾）
   │
   ▼
验证（改代码轮：go build / go test / tsc / vite build 全绿；纯文档轮：跳过）
   │
   ▼
收尾（spec 状态 → Done；跨页决策 → Accepted）
```

### 3.1 判断"是 spec 还是 decision"

| 信号 | 归类 |
|------|------|
| 只改一个页面 / 一个组件的交互、文案、布局 | spec |
| 影响多个页面 / 技术选型 / 数据结构 / 不可逆 | decision |
| 例："阅读页加个目录 TOC" | spec 02 |
| 例："阅读页字号默认值 medium → large" | spec 02（单页 + 可逆，不是 decision） |
| 例："搜索从 LIKE 换 FTS、要不要换驱动" | decision |

### 3.2 不要做的事

- **不要**在没改 spec 的情况下改代码（除非是纯 bug 修复，bug 修完补一行到 spec 的"边界情况"）
- **不要**删旧 decision（被推翻就标 `Superseded`）
- **不要**一次性把 12 个 spec 全写完然后放着不更新——**做到哪写到哪**，spec 跟随代码演进

---

## 4. 与 PROGRESS.md 的关系

| 文件 | 维度 | 内容 | 更新时机 |
|------|------|------|----------|
| `PROGRESS.md` | 时间线 | 第 N 轮做了什么 | 每轮收尾 |
| `docs/specs/` | 空间 | 每个页面应该长什么样 | 改动该页面时 |
| `docs/decisions/` | 决策 | 跨页重大决策 + 理由 | 做决策时 |
| `AGENTS.md` + `docs/AI_STANDARD.md` | 协作 | AI 怎么干活、怎么算完 | 流程缺口出现时（append 条款） |

三者互相引用：spec 的"相关决策"指向 decision；decision 的"影响 spec"指向 spec；
PROGRESS 轮次可注明"见 spec 02 / decision 0001"。
