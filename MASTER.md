> 类别 A · 元文档（文档总索引 + 自驱协议）— 与 `AGENTS.md` 同为入口族，本文件管"有哪些文档、去哪找、我怎么自己跑起来"。

# MASTER — LopezPress 母文档

> **这是本仓库唯一的入口文档。** 用户只需说一句「去 MASTER.md 找」，任何 AI 都应当从本文件恢复全部上下文，
> 不需要用户重复解释项目是什么、文档在哪、下一步该干什么。
>
> 建立：2026-10-22（第 41 轮）｜维护者：AI 协作者，每次新增/删除文档必须同步本文件
> 与你（用户）的分工：你说目标 → 我按本文件自驱；我卡住才问你。

---

## 0. 会话启动提示词（复制即用）

> **前提认知**：模型不会「记得」，只会「读到」。对话越长，早期上下文越容易被压缩、被稀释。
> 所以本文件不依赖记忆，只依赖**三件事**：① 每次开场把协议重新读一遍；② 每个决定当场落盘；③ 用脚本校验落盘结果没失真。
> 下面三段提示词分别对应**新对话 / 长对话中途 / 收尾换对话**，直接复制给 AI 即可。

### 0.1 冷启动（新对话第一句）

```text
读 C:\Users\LopezPC\Desktop\Lopez\echo-content-api\MASTER.md，按里面的自驱协议工作。
读完先做三件事，再动手：
1) 复述 §6.5 的 9 条红线（要能看出你真读了，不是扫了一眼）
2) 报告 §9.4「当前断点」的最后更新时间与最后 commit，并和 `git log -1` 对比是否一致；不一致先查清原因
3) 判断我这句话属于 §6.1 哪个触发词，说出你选的任务分级（T0–T3）与理由
然后开始干活。每完成一件事按 §6.4 落盘，不要等我提醒。
```

### 0.2 长对话中途锚定（怕它忘的时候贴）

```text
落盘检查点。从现在起停止回忆，只按文件核对：
1) 自上个检查点以来，我们做了哪些**决定**？每一条要么进 docs/decisions/，要么进 MASTER §9.4，
   不允许"记在脑子里"
2) 改了哪些**行为**？进 PROGRESS.md + docs/TODO.md
3) MASTER §7 事实基线的数字变了没有（测试数 / 路由数 / 文档数 / 产物大小）？变了就更新
4) 跑 python scripts/doc_check.py，确认母文档与你说的内容一致
5) 列一份「还没落盘的东西」给我，然后逐条补
```

### 0.3 收尾 / 准备换新对话

```text
本轮收尾：按 §6.4 把该记的都写全（PROGRESS / TODO / HANDOVER §7 / decisions / memory），
把 MASTER §9.4「当前断点」更新成下一个 AI 能直接接手的状态（正在做什么、下一步第一件事、未决问题），
然后跑 doc_check.py + 五条门禁，把结果贴给我。确认全绿再说"可以开新对话了"。
```

---

## 1. 三句话认识这个项目

1. **是什么**：LopezPress（原 Echo Content API），面向低配 VPS（1 vCPU / 1.5 GB / 5 GB）的可自托管内容站点。
2. **怎么搭**：Go 单二进制 + SQLite 单文件（后端，`net/http` 标准库路由 + GORM + 纯 Go 驱动）+ Preact 单页（前端）。
3. **做到哪了**：41 轮开发，7 个 P0 全修，P1 仅剩 1 项架构美化；已推 GitHub 私有仓库，距 v1.0 差"生产部署端到端验证"。

---

## 2. 30 秒定位：我想干 X，该读哪份

| 你的目的 | 直接看 |
|---|---|
| 让 AI 接着干活 | 本文件 §6「自驱协议」→ `PROGRESS.md` 最新一轮 → `docs/TODO.md` |
| 知道"现在还有什么没做" | `docs/TODO.md`（**唯一权威待办**） |
| 知道"以前做过什么" | `PROGRESS.md`（第 1–41 轮，含每轮改了什么、怎么验的） |
| 改代码前别踩坑 | `HANDOVER.md` §0 + §7（陷阱）+ `AGENTS.md` §3 |
| 看某个页面应该长什么样 | `docs/specs/NN-*.md`（EARS 验收语法） |
| 看某个技术选型为什么这么定 | `docs/decisions/000N-*.md`（append-only，不可推翻只能 Superseded） |
| 找接口 / 端点 | `api.md` + `internal/router/router.go`（唯一事实源） |
| 找代码在哪 | `docs/PROJECT_OVERVIEW.md`（代码地图） |
| 改界面 | `docs/DESIGN_SYSTEM.md`（全站 UI 唯一标准） |
| 部署上线 | `deploy/README.md`、`deploy/LOCAL.md`、`deploy/docker/` |
| 找 token / 账号 / 密码 | `~/.workbuddy/CREDENTIALS.md`（**仓库外**，见 §5） |
| 我的工具怎么用 | 同上 §2「github-auto-upload」章节 |

---

## 3. 文档总表（按族分组，共 54 份，不含 memory 日志）

> 类别：**A = 指挥/约束 AI**（怎么干、不能干什么）；**B = 项目参考**（项目是什么样）。
> 每个文件顶部都有 `> 类别 A/B` 横幅，单独打开也能自识别。

### 3.1 入口族（3 份，缺一不可）

| 文档 | 类 | 作用 | 什么时候读 |
|---|---|---|---|
| **`MASTER.md`（本文件）** | A | 总索引 + 自驱协议 | 每次会话第一份 |
| `AGENTS.md` | A | 速查：硬约束表、六大陷阱、验证五命令、文档两类制 | 本文件之后立刻读 |
| `docs/README.md` | B | `docs/` 目录的内部索引（spec / decision 编号表、状态机、EARS 语法） | 需要动 spec/decision 时 |

### 3.2 AI 约束族（8 份，定义"怎么算干完"）

| 文档 | 类 | 作用 |
|---|---|---|
| `docs/AI_STANDARD.md` | A | **流程唯一标准**：任务分级 T0–T3、五阶段工作流、验证门禁、十条红线 R1–R10、上下文纪律、质量自查清单 |
| `docs/prompts/README.md` | A | 提示词模板库的使用说明 |
| `docs/prompts/01-continue.md` | A | 「继续」场景的标准提示词（恢复上下文 → 选下一优先级） |
| `docs/prompts/02-new-feature.md` | A | 新功能（T2）：spec → plan → 实现 |
| `docs/prompts/03-bugfix.md` | A | 修 bug：复现 → 定位 → 修复 → 回归 |
| `docs/prompts/04-audit.md` | A | 自查/审计（T3）：全栈扫描 → 分级清单 |
| `docs/prompts/05-verify-record.md` | A | 门禁 + 记录沉淀 |
| `docs/reference/AI_CODING_REFERENCES.md` | B | 外部方法论提炼（spec-kit / BMAD 等）：抄了什么、刻意没抄什么 |

### 3.3 项目事实族（8 份，回答"现在是什么样"）

| 文档 | 类 | 作用 |
|---|---|---|
| `HANDOVER.md` | A | 交接总览：技术栈真实状态、启动方式、§7 关键陷阱、未决决策、自测清单 |
| `PROGRESS.md` | B | 轮次史（第 1–41 轮）+ 已知风险表 + 编译验证基线（每次重测都更新） |
| `docs/TODO.md` | A | **唯一权威待办**：P0/P1/P2/P3 + 功能缺口 + 设计取舍 + 「实测没问题」清单 |
| `docs/ARCHITECTURE.md` | B | 架构设计与分层理由 |
| `docs/PROJECT_OVERVIEW.md` | B | 代码地图速查：路由 / API / 页面 / 改码红线 |
| `docs/BUSINESS_LOGIC.md` | B | 业务逻辑反向梳理（代码反推，非设计意图） |
| `README.md` | B | 快速上手：跑起来、演示账号、前端说明、文档导航 |
| `api.md` | B | API 参考 |

### 3.4 规格与决策（27 份，定义"应该是什么样"和"为什么"）

| 目录 | 数量 | 作用 |
|---|---|---|
| `docs/specs/01–22` | 21 + 模板 | 按页面/功能域切分的功能规范，EARS 语法（`WHEN … THEN …`），带状态机 Draft→In Progress→Done |
| `docs/decisions/0001–0004` | 4 + 模板 | 跨页面重大决策，**append-only**：被推翻只能标 `Superseded by`，历史本身是价值 |

### 3.5 审计与归档（3 份）

| 文档 | 类 | 作用 |
|---|---|---|
| `docs/AUDIT_2026-10-15.md` | B | 最近一次全栈深度审计的完整证据链（P0-1～P0-6 的来源） |
| `docs/AUDIT_REPORT.md` | B | 10 项安全加固的落地记录 |
| `docs/archive/2026-10-14-FUNCTIONAL_AUDIT.md` | B | 首轮审计快照，**已归档，勿再当待办** |

### 3.6 部署（3 份）

| 文档 | 作用 |
|---|---|
| `deploy/README.md` | systemd + Nginx + 一键上传脚本 |
| `deploy/LOCAL.md` | 本机部署与联调 |
| `deploy/docker/README.md` | Docker Compose 方案（未在本机 docker build 过，无 Docker） |

### 3.7 记忆层（12 份，gitignored，不计入上表 54 份）

| 路径 | 作用 |
|---|---|
| `.workbuddy/memory/MEMORY.md` | 项目长期记忆：技术栈定稿、事实基线、关键陷阱（跨会话） |
| `.workbuddy/memory/YYYY-MM-DD.md` | 每日工作日志（append-only）：改了什么、验证结果、遗留 |

### 3.8 其他参考（2 份）

| 文档 | 类 | 作用 |
|---|---|---|
| `docs/DESIGN_SYSTEM.md` | B | 全站 UI 唯一标准：设计 Token、布局、组件 Do&Don't、博客排版、暗色、a11y |
| `docs/NEXT_VERSION_BACKLOG.md` | B | v0.2 规划草稿（未承诺的功能池） |

> **份数校验**：3.1(3) + 3.2(8) + 3.3(8) + 3.4(27) + 3.5(3) + 3.6(3) + 3.8(2) = **54**，与 §7 事实基线一致。
> 新增文档时改完表格请重算这一行，避免母文档自身失真。

---

## 4. 我（AI）在这个项目里建了什么，各自什么用

| 资产 | 建的时间 | 解决的问题 |
|---|---|---|
| `AGENTS.md` | 第 26 轮 | 任何 AI 工具进入仓库的第一份必读，收敛"改代码前必须知道的事" |
| `docs/AI_STANDARD.md` | 第 26 轮 | 把"怎么干活、干到什么程度算完、干完怎么沉淀"写成标准，避免每轮靠猜 |
| `docs/prompts/`（5 个模板） | 第 26 轮 | 用户复制一句话就能触发标准流程，不用每次解释 |
| `docs/reference/AI_CODING_REFERENCES.md` | 第 26 轮 | 外部方法论（spec-kit / BMAD）的取舍记录，防止日后重复调研 |
| 文档两类制（A/B 横幅） | 第 37 轮 | 让"约束 AI"和"描述项目"分开，新对话只读 A 类即可恢复纪律 |
| `docs/TODO.md` 勾选校准 | 第 38 轮 | 修掉"未修却已勾"的假阳性，恢复待办真实度 |
| `internal/config/recover.go` + spec 22 | 第 39 轮 | root 忘密码无路可走的死角（离线 CLI 恢复） |
| `scripts/contract_check.py` | 第 33 轮 | 前后端 method+path 契约自动核对，防 `cover` 那类静默漂移复发 |
| `scripts/doc_check.py` | 第 42 轮 | 机器校验母文档是否失真（份数 / 顶层文档是否被点名 / 批量目录是否索引 / A-B 横幅是否齐全），退出码非 0 即失败 |
| `~/.workbuddy/CREDENTIALS.md` | 第 40 轮 | 统一存放 token / 账号 / 工具用法（**仓库外**，防误提交泄露） |
| **`MASTER.md`（本文件）** | 第 41 轮 | 用户不用记任何路径：一句"去母文档找"即可 |

---

## 5. 仓库外的东西（重要，别在仓库里找）

| 资产 | 位置 | 说明 |
|---|---|---|
| 凭据台账 | `C:\Users\LopezPC\.workbuddy\CREDENTIALS.md` | GitHub PAT、工具路径、坑。**故意不在任何 git 仓库内** |
| 上传技能 | `~/.workbuddy/skills/github-auto-upload` | 推送走 Git Data API（本机 `git push` 直连被墙） |
| 技能配置 | `~/.workbuddy/github-auto-upload/config.json` | 存 GitHub token（仅本账户可读） |
| 用户级记忆 | `~/.workbuddy/.../MEMORY.md` 等 | 跨项目偏好、本机环境约束 |

> 红线：**任何真实密钥不得出现在仓库内任何文件**（含 memory 日志）。要记就记进 `CREDENTIALS.md`。

---

## 6. 自驱协议（用户说一句话，我怎么自动跑起来）

### 6.1 触发词 → 动作

| 用户说 | 我做 |
|---|---|
| 「**继续**」 | 读 `PROGRESS.md` 最新一轮 → `docs/TODO.md` → 挑最高优先级未完成项 → 按 §6.2 执行 |
| 「**去母文档找**」/「**按母文档来**」 | 读本文件 → 按 §2 定位表找到对应文档 → 继续该任务 |
| 「**看看有什么问题**」 | 按 T3 走：取证（代码/日志/命令）→ 分级问题清单（P0–P3）→ 落 `docs/TODO.md` |
| 「**修 X**」 | 判分级 → 若 T2 先写 spec → 实现 → 门禁 → 记录 |
| 「**推上去**」 | 用 `github-auto-upload` 技能 `sync`（**不要**手搓 `git push`，直连不通） |
| 「**记一下**」 | 写 `.workbuddy/memory/YYYY-MM-DD.md`；属长期约定则进 `MEMORY.md`；涉密钥进 `CREDENTIALS.md` |

### 6.2 任务分级（决定要不要先写 spec）

| 级别 | 判据 | 流程 |
|---|---|---|
| T0 直接做 | 单文件、无行为变化（文案/注释/格式/删死代码） | 实现 → 门禁 → 记录 |
| T1 小改动 | 2–5 文件、行为变化一句话说清 | 口头 plan → 实现 → 门禁 + 回归 → 记录 |
| T2 功能/结构 | 新端点/新页面/改数据模型/改权限或可见性 | **先写 spec** → plan → 实现 → 门禁 + 新测试 → 记录 |
| T3 排查/审计 | 找 bug、全栈审计、性能 | 先取证 → 分级清单 → 再动手 |

### 6.3 完成定义（缺一不算完）

```bash
go build ./...      # 后端编译
go vet ./...        # 静态检查
go test -count=1 ./...   # 单测（跳过缓存）
cd web && npx tsc --noEmit   # 前端类型
npm run build       # 前端生产构建
python scripts/doc_check.py  # 改过文档时必跑：母文档索引/份数/横幅是否仍一致
```

行为改动还必须**真跑一次**并贴证据（状态码 / 日志 / 截图），不接受"应该可以"。
门禁任何一项红 → 修到绿，或明确记录"未解决 + 原因"，**不允许带病标完成**。

### 6.4 干完记哪里（五处，按性质选）

| 性质 | 写哪 |
|---|---|
| 功能/修复/加固 | `PROGRESS.md` 追加轮次 |
| 待办状态变化 | `docs/TODO.md` 勾销/降级/新增 |
| 新发现的坑 | `HANDOVER.md` §7 + `.workbuddy/memory/MEMORY.md` |
| 重大决策 | `docs/decisions/00NN-*.md`（append-only） |
| 每次会话 | `.workbuddy/memory/YYYY-MM-DD.md` |
| **新增/删除文档** | **本文件 §3 表格**（否则母文档就失真了） |

### 6.5 硬红线（违反即返工）

1. 不换框架、不加重依赖（后端 6 个直接依赖、前端 3 个包）
2. root 不可经任何 API 创建或提升
3. 可见性判定只在 service 层（`canSeePost` 是五态唯一权威）
4. 权限判定只用 `services/roles.go` 的集中 helper
5. 已存在的表不跑无条件 `AutoMigrate`（glebarez copy-rename 会崩）
6. 读接口 `WithOptionalAuth`、写接口 `WithAuth`
7. 错误契约用英文，界面文案走 i18n 中英双语
8. 密钥不进仓库
9. **单对话 ≤ 30 轮**：接近上限主动收尾 + 建议开新对话（新对话只读本文件即可恢复）

---

## 7. 事实基线（第 39 轮实测，改动后请重测并更新）

| 指标 | 值 |
|---|---|
| 后端规模 | 48 个 `.go`，约 11.4k 行；6 个直接依赖（+10 indirect） |
| 前端规模 | 43 个 `.ts(x)`，8,324 行；22 页面 / 9 公共组件；3 个依赖（`preact` / `wouter` / `marked`） |
| 测试 | 29 个文件 / **120** 个 `Test*`（services 65 / handlers 24 / middleware 21 / config 6 / models 4） |
| 路由 | 后端 **63** 条，前端接入 56 条，方法不匹配 **0**（`scripts/contract_check.py`） |
| 构建产物 | 63 模块；JS 209.09 kB / gzip 64.54 kB；CSS 57.64 kB / gzip 10.85 kB |
| Git | 分支 `main`，tag `v0.1.0`，远程 `RioLopez/lopezpress`（private） |
| 文档 | 仓库内 54 份（含本文件，不含 memory 日志） |

---

## 8. 这份母文档怎么维护

- **新增任何 `.md`** → 在 §3 对应族里加一行（路径 / 类 / 作用 / 何时读），否则下一轮 AI 找不到它。
- **删除或归档文档** → 从 §3 移除并在 §3.5 记一笔归档原因。
- **完成一轮开发** → 更新 §7 事实基线的数字（测试数、路由数、产物大小）。
- **协议本身变了**（新增触发词、改门禁） → 改 §6，并在 `docs/AI_STANDARD.md` 同步（两份不能互相矛盾）。
- 本文件**不写细节**，只写"去哪找"。细节永远留在各自文档里，避免两处失真。

---

## 9. 断点与落盘（防止"对话久了就忘"）

### 9.1 三条铁律

1. **不落盘 = 没做。** 对话里的结论对下一个会话不存在。任何决定/改动，写完文件才算完成。
2. **落盘到最窄的那份文档，再用母文档指路。** 细节进 spec / decision / HANDOVER，母文档只留一行索引——两处写同一件事必然打架。
3. **每个断言带证据。** 「已修」必须能指向 commit / 测试名 / 命令输出；「应该可以」不算证据（这条来自第 38 轮的教训：勾选状态曾经和正文互相矛盾）。

### 9.2 落盘触发清单（事件 → 写哪里，不许跳过）

| 发生了什么 | 必须写 |
|---|---|
| 做了技术取舍（选型、放弃某方案、改约定） | `docs/decisions/00NN-*.md`（append-only）+ `MASTER §9.4` |
| 改了行为（功能 / 修复 / 加固） | `PROGRESS.md` 新轮次 + `docs/TODO.md` 勾销 + `MASTER §7` 数字 |
| 发现新的坑 | `HANDOVER.md` §7 + `.workbuddy/memory/MEMORY.md` |
| 新增 / 删除 / 改名文档 | `MASTER §3`（并重算份数，跑 `doc_check.py`） |
| 本轮结束（无论大小） | `.workbuddy/memory/YYYY-MM-DD.md` + `MASTER §9.4` |
| 用户说了长期偏好 | `~/.workbuddy/.../MEMORY.md`（跨项目） |
| 涉及 token / 账号 / 密码 | `~/.workbuddy/CREDENTIALS.md`（**仓库外**） |

### 9.3 检查点节奏（对话越长越要密）

| 时点 | 动作 |
|---|---|
| 每完成一个 T0/T1 | 顺手落 PROGRESS 片段 + memory 一行 |
| **每完成一个 T2（写码 + 门禁）** | 全量落盘 + 更新 §9.4 + 跑 `doc_check.py` |
| 每 5 轮对话 | 主动贴 §0.2 检查点提示词，做一次"还没落盘清单" |
| 累计 ≥ 20 轮 | 强制做一次完整快照（§0.3），即使任务没完 |
| 累计 ≥ 30 轮 | **停止新工作**，收尾 + 建议开新对话（红线 9） |
| 任何一次 `git commit` 前 | 确认 §9.4「对应提交」已描述本轮主题，避免断点与提交对不上 |

### 9.4 当前断点（滚动更新，最后一个写它的人负责）

> 新会话读完本表即可接手。**时间戳与 commit 必须真实**，过期即视为母文档失真。

| 项 | 值 |
|---|---|
| 最后更新 | 2026-10-22 21:05（第 42 轮：母文档加启动提示词 + 落盘协议 + 校验脚本） |
| 对应提交 | 主题 `docs: make MASTER self-sustaining (prompts + breakpoint + doc_check)`（用 `git log --oneline` 核对，**不写 sha**，否则会陷入"记录自己"的死循环） |
| 正在做 | 无进行中任务，工作树干净 |
| 下一步第一件事 | 三选一待用户拍板：① 修 P2-6 可信代理闸门 ② 封禁库补 UI 发起入口 ③ 生产部署端到端验证 |
| 未决问题 | ① GitHub PAT 权限偏大（含 `delete_repo`/`admin:org`），建议收紧到 `repo`+`workflow` ② `POST /api/admin/ban/{id}` 无前端入口，封禁库"只出不进" ③ 未 docker build、未在真实 VPS 端到端跑 |
| 当前分支状态 | `main` 与 `origin/main` 一致；tag `v0.1.0` 之后尚未打新 tag |

### 9.5 新会话开场自检（30 秒）

1. §9.4 的「对应提交」主题，能在 `git log --oneline -20` 里找到吗？找不到 → 上一个会话没落盘，先补再干活。
2. §9.4 的「最后更新」比 `git log -1` 的提交时间旧很多 → 中间有工作没记录，先问用户。
3. `python scripts/doc_check.py` 是否为 PASS → 不是就是母文档失真，先修再干活。

### 9.6 机器校验（让"可持续"可验证，而不是靠自觉）

```bash
python scripts/doc_check.py        # 退出码 0 = 母文档与实际一致
```

它检查四件事：**① 文档总数**与 §3/§7 声明一致；**② 每个根目录 / `deploy/` / `docs/` 顶层的 `.md` 都在 §3 里被点名**；**③ 批量目录**（`specs/` `decisions/` `prompts/` `reference/` `archive/`）在 §3 中存在；**④ 每个 `.md` 顶部都有 `> 类别 A/B` 横幅**。
**新增文档却忘了改母文档 → 这个脚本会红。** 把它加进"改文档类任务"的验证门禁，母文档就不可能悄悄过期。
