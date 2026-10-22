> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 外部参考项目提炼(AI_CODING_REFERENCES)

> 建立:2026-10-19 | star 数为当日 GitHub API 实时值
> 用途:记录本项目 AI 协作标准(`docs/AI_STANDARD.md`)借鉴了哪些开源方法论、抄了什么、为什么没抄剩下的。
> 原则:**只提炼可落地的流程,不搬运具体实现**——本项目是低配单机项目,不需要多 agent 编排框架。

---

## 1. 参考项目总表

| 项目 | ★ | 类型 | 我们借鉴了什么 |
|---|---|---|---|
| `github/spec-kit` | 141760 | 规范驱动开发(SDD)工具包 | 核心流程:specify → clarify → plan → tasks → implement;spec 先行再写码 |
| `bmad-code-org/BMAD-METHOD` | 54290 | 敏捷 AI 驱动开发方法论 | 角色分离思路(需求/架构/实现/验证分开推进),防 AI"一个人一口气写完" |
| `spec-kitty/spec-kitty` | 1688 | SDD + 组织治理 | "spec 管做什么、charter 管怎么做"分层——对应我们的 `AI_STANDARD.md`(流程)与 `docs/specs/`(功能)分离 |
| `johnpapa/ai-ready` | 229 | 仓库分析生成 AI 配置 | "先分析仓库再生成规则"的思路;我们手工写了 `AGENTS.md`(项目小,手工更准) |
| `Anbeeld/AGENTS.md` | 168 | 全局 agent 指令模板 | AGENTS.md 作为**单一入口文件**的形态;证据/验证优先的表述 |
| `hesreallyhim/awesome-claude-code` | 55945 | 资源总清单 | skills/hooks/子代理生态认知;本项目暂不引入,保持轻量 |
| `Meirtz/Awesome-Context-Engineering` | 3327 | 上下文工程综述 | "上下文是稀缺资源"原则 → `AI_STANDARD.md` §5 上下文纪律 |
| `Aider-AI/aider` | 49612 | 终端 pair programming | 若未来要自动化,低配机首选的轻量 agent(仅记录,未采用) |
| `stackblitz-labs/bolt.diy` | 19954 | AI 全栈应用生成 | "prompt → 文件树 diff → 运行"架构参考(仅记录,未采用) |
| `FoundationAgents/MetaGPT` / `OpenBMB/ChatDev` | 70908 / 34545 | 多智能体软件公司 | 研究向;单机单人项目**不采用**(复杂度与资源都不匹配) |

## 2. spec-kit 核心流程提炼(本项目采用的部分)

```
/specify   →  写清"做什么 + 验收标准(EARS)"        → 对应 docs/specs/ 模板
/clarify  →  结构化澄清歧义(最多 5 个关键问题)     → 对应 AI_STANDARD §3 阶段 2 等用户确认
/plan     →  技术方案 + 分步(每步可验证)           → 对应 plan 汇报
/tasks    →  把 plan 拆成可勾选任务                → 小项目口头 plan 即可,不强制文件化
/implement→  按 tasks 执行,测试同轮                → 对应 §3 阶段 3
```

未采用:spec-kit 的 CLI 工具链、分支/看板等重型机制——单人项目用文档 + PROGRESS 轮次已足够。

## 3. BMAD 角色分离提炼(采用理念,不采用框架)

BMAD 的價值不在代码,而在**强制把"想清楚"和"写代码"分开**:Analyst 问需求 → PM 定范围 → Architect 出方案 → Dev 才动手。对应到本标准:

- `02-new-feature.md` 模板强制「先 spec + plan,等确认再写码」——这就是轻量版角色分离。
- 用户(决策者)与 AI(执行者)职责写死在 `AI_STANDARD.md` §1。

## 4. AGENTS.md 规范说明

`AGENTS.md` 正成为跨工具事实标准(Claude Code / Cursor / Codex / OpenHands / Copilot 均读取)。本项目把它放在仓库根目录,内容定位「速查 + 红线」,详细流程指向 `docs/AI_STANDARD.md`——**一个入口、两级深度**,避免单文件膨胀。

## 5. 上下文工程关键原则(来自 Awesome-Context-Engineering)

1. 上下文是稀缺资源:大文件分段读、无关内容不灌入。
2. 检索优于记忆:不确定的历史结论去日志/搜索查证。
3. 结构化固化:重复三次以上的流程就该写成模板(→ `docs/prompts/`)。
4. 失败模式要记录:每个踩过的坑进 `HANDOVER.md` §7,而不是依赖 AI"下次记得"。

## 6. 明确不采用的(及原因)

| 不采用 | 原因 |
|---|---|
| 多 agent 编排框架(MetaGPT/ChatDev/AutoGen) | 1 vCPU 机器 + 单人项目,编排开销 > 收益 |
| Claude Code skills/hooks 生态 | 当前协作者是 WorkBuddy + 用户手动驱动;生态绑定降低可移植性 |
| AI 应用生成器(bolt.diy 类) | 本项目是手工精修的内容平台,不是生成式产品 |
| spec-kitty 的组织治理层 | 没有团队,治理无对象 |
