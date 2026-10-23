> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 后台文章管理页 + 批量操作

> 状态：In Progress（第 44 轮实现）
> 关联：`docs/specs/24-pagination.md`（复用其分页）、`docs/specs/21-roles-moderation.md`（权限与五态）
> 参考：Halo 2.17 的「文章批量管理」（批量发布/取消发布、批量设置分类/标签/作者）——**批量对象是文章，不是标签**
> 最后更新：2026-10-22

## 1. 背景

第 43 轮之前，这个站**没有"后台文章管理"这个页面**：

- 文章列表就是前台首页本身，登录后在每条文章下面挂着「编辑 / 删除」；
- 后台有 10 个页面（仪表盘/审核/封禁库/用户/分类/标签/站点/备份/审计/回收站），唯独缺文章；
- 于是"管理内容"和"阅读内容"挤在同一个界面，既不好读，也不好管。

这同时解释了用户为什么觉得"后台很乱"：不是菜单太多，而是**职责错位**——管理动作散落在前台，后台只剩外围配置。

## 2. 目标与非目标

**目标**

- 新增 `/admin/posts`：表格式文章管理，含状态/分类/标签徽标、浏览量、创建时间、行内编辑/删除。
- 支持筛选：关键字、状态（五态）、分类；分页复用 spec 24 的服务端页大小。
- 支持**批量操作**：发布、撤销发布（转 private）、改分类、加标签、移标签、删除（进回收站）。
- 批量执行采用**部分成功**语义：逐条复用现有单条 service 方法，权限/可见性规则不重写；失败的条目回落为 `skipped` 并带原因，不因一条 403 让整批失败。
- 后台导航按「内容 / 用户 / 系统」分组（10 个平铺改分组）。

**非目标**

- 不做批量硬删除（软删进回收站已有独立页面，硬删仅限封禁库的 root 操作）。
- 不做批量改作者（单实例单作者场景无实际用途）。
- 不把前台首页的编辑/删除按钮删掉——普通用户（role=user）目前只能从首页进入编辑，删掉会让其无处可去（另立待办 F-3，先给按钮加"仅作者/管理员可见"的门）。

## 3. API 契约

### `POST /api/admin/posts/batch`

- 鉴权：`withAuth` + `requireAdmin`。
- 请求：

```json
{ "ids": ["uuid1", "uuid2"], "action": "publish" }
{ "ids": ["uuid1"], "action": "set_category", "category_id": 3 }
{ "ids": ["uuid1"], "action": "add_tags", "tags": ["Go", "SQLite"] }
```

- `action` 白名单：`publish` | `private` | `delete` | `set_category` | `add_tags` | `remove_tags`。
- `ids`：1–100 条（超出 400）。
- 响应 `200`：

```json
{ "affected": 2, "skipped": [{ "id": "uuid3", "reason": "you can only edit your own posts" }] }
```

- 错误：`400` 空 ids / 未知 action / ids 超限；`403` 非管理员；`500`

## 4. 权限矩阵（沿用单条操作，不新增规则）

| action | 谁能做 |
|---|---|
| `publish` | **仅 root**（admin 发布需走审核端点 `POST /api/admin/review/{id}`）；admin 调用整批 → 403 |
| `private` | 作者本人、admin、root（按 `UpdatePost` 既有规则） |
| `delete` | 作者本人、admin、root（软删 → 回收站） |
| `set_category` | 同 `private` |
| `add_tags` / `remove_tags` | 同 `private` |

## 5. 用户故事 + 验收标准（EARS）

- **US-1**：作为站主，我要在一个页面里看到并管理所有文章。
  - **AC-1.1**：WHEN 打开 `/admin/posts` THEN 默认按创建时间倒序显示第 1 页，每页条数沿用站点设置
  - **AC-1.2**：WHEN 选择状态筛选 THEN 列表只显示该状态，且回到第 1 页
  - **AC-1.3**：WHEN 点击某行的「编辑」THEN 进入 `/edit/{id}`
- **US-2**：作为站主，我要一次处理多篇。
  - **AC-2.1**：WHEN 勾选若干行并执行批量操作 THEN 每篇按单条规则处理，响应给出 `affected` 与 `skipped`
  - **AC-2.2**：WHEN 批量删除 THEN 文章进入回收站（软删），可在 `/admin/trash` 恢复
  - **AC-2.3**：WHEN 批量操作会影响 N 篇 THEN 执行前弹出确认框，明确写出 N 与动作
  - **AC-2.4**：WHEN 混合选中（部分无权操作）THEN 有权的成功、无权的跳过并显示原因
- **US-3**：作为管理员，我一眼能分辨状态。
  - **AC-3.1**：WHEN 列表渲染 THEN 每行显示状态徽标（published/private/pending/rejected/banned），已发布不显示徽标以减噪

## 6. 边界与错误

| 情况 | 行为 |
|---|---|
| ids 为空 | 400 `ids is required` |
| ids > 100 | 400 `too many ids (max 100)` |
| 未知 action | 400 `unknown action` |
| 文章不存在 / 已删 | 该条跳过，reason `post not found` |
| admin 调 `publish` | 整批 403 `only the root can publish posts` |
| `set_category` 缺 category_id | 400 `category_id is required` |
| `add_tags` 但标签已存在 | 幂等（合集去重），不报错 |

## 7. 实现要点

- **不写第二套权限逻辑**：批量实现逐条调用 `UpdatePost` / `DeletePost`，权限、审核状态机、banned 冻结全部自动继承——这是本轮最重要的设计决定（第 25 轮的教训：设计了集中权限层却手写 if-else，漏一个就是 P0）。
- `add_tags` / `remove_tags` 需要先读当前标签做集合运算，再交给 `UpdatePost`（它按 `in.Tags != nil` 整体替换）。
- 单条失败只记录不中断：`skipped` 里带上 `StatusOf(err)` 对应的英文 message。
- 前端批量条固定在列表上方，勾选数实时显示；操作走确认弹窗。

## 8. 测试要点

- root 批量 publish → affected=N，状态变 published；
- admin 批量 publish → 403；
- admin 批量删除他人文章（作者是 root）→ 跳过该条，不报 500；
- 批量 add_tags 幂等（重复加同一标签不产生重复关联）；
- ids 超 100 → 400。
