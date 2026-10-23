> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 封禁闭环（后台发起封禁）

> 状态：Implemented（第 48 轮）
> 关联：`docs/specs/21-roles-moderation.md`（五态与封禁库）、`docs/TODO.md` 遗留项「`POST /api/admin/ban/{id}` 无前端入口」
> 最后更新：2026-10-22

## 1. 背景

封禁相关的**后端早已完整**：`POST /api/admin/ban/{id}`、`GET /api/admin/banned`、`POST /api/admin/banned/{id}/send-back`、`DELETE /api/admin/banned/{id}`（永久删除）。

但**前端没有任何地方能发起封禁**——「封禁库」页面只能查看、退回、永久删除。结果是一个单向门：**进去的东西只能出来，永远没有新的进去**。这是典型的"后端做完了，闭环差最后一环"。

## 2. 目标

- 在后台文章管理页（`/admin/posts`）为 **root** 提供行内「封禁」按钮。
- 沿用既有语义：封禁后文章进入封禁库，**只有 root 可见**，作者与其他 admin 都看不到也改不了。
- 封禁后列表立即刷新（该行状态变为 banned，按钮消失）。

**非目标**：不改封禁的数据语义、不加"封禁理由"输入（后端 `ReviewNote` 已存在但 `BanPost` 未接收，留给后续需要时再做）。

## 3. 权限

| 角色 | 能看到封禁按钮 | 能调用接口 |
|---|---|---|
| root | ✅ | ✅（`requireRoot`） |
| admin | ❌（按钮不渲染） | ❌ 403 |
| user | ❌ | ❌ 403 |

前端隐藏只是 UX；真正的门在 handler 的 `requireRoot` 和 service 的 `canManageUser`/封禁规则里。

## 4. 用户故事 + 验收标准（EARS）

- **US-1**：作为 root，我要能在文章列表里直接把一篇内容拉进封禁库。
  - **AC-1.1**：WHEN 以 root 打开 `/admin/posts` 且某行状态不是 banned THEN 该行显示「封禁」按钮
  - **AC-1.2**：WHEN 点击「封禁」THEN 弹出确认框（"封禁后这篇文章只有你能看到"），确认后才发请求
  - **AC-1.3**：WHEN 封禁成功 THEN 提示成功、列表刷新，该行状态为 banned 且按钮消失
  - **AC-1.4**：WHEN 以 admin 打开同一页面 THEN 看不到「封禁」按钮
  - **AC-1.5**：WHEN 已是 banned 的行 THEN 不显示「封禁」按钮（幂等，不重复调用）
- **US-2**：封禁之后我要能按既有流程处理。
  - **AC-2.1**：WHEN 打开 `/admin/banned` THEN 能看到刚封禁的文章
  - **AC-2.2**：WHEN 在该页点「退回作者」THEN 文章离开封禁库、作者可修改后重投
  - **AC-2.3**：WHEN 在该页点「永久删除」THEN 二次确认后物理删除

## 5. 边界

| 情况 | 行为 |
|---|---|
| 封禁自己的文章 | 允许（root 对自己内容有完全处置权） |
| 封禁已 banned 的文章 | 后端幂等；UI 已隐藏按钮 |
| 封禁后原链接 | `/read/{uuid}` 与 `/p/{slug}` 对非 root 均 404（`canSeePost`） |
| 封禁后 sitemap/RSS | 立即消失（只列 published） |

## 6. 测试

- 服务层已有 `ban_post` 相关用例覆盖可见性与冻结规则（第 25/28 轮），本轮只加前端入口，未新增后端逻辑。
- 手测：root 在列表封禁一篇 → 该行变 banned、`/admin/banned` 可见、RSS 中消失。
