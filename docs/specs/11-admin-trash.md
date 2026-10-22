> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 回收站（`/admin/trash`）

> 状态：Done
> 路由：`/admin/trash`
> 权限：admin
> 轮次：第 11、17 轮
> 最后更新：2026-09-21

## 1. 目标

admin 查看被软删的文章（回收站），并可恢复或批量永久清空；过期文章由后端清理。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要查看被删文章并恢复。
  - **AC-1.1**：WHEN 列表渲染 THEN 每行显示标题 / 作者 / 浏览量 / 删除时间
  - **AC-1.2**：WHEN 点「恢复」THEN 确认框（带标题）；确认后 `restoreTrash` + 重新拉取 + 成功 toast
  - **AC-1.3**：WHEN 回收站为空 THEN 显示「回收站是空的」
- **US-2**：作为 admin，我要清空回收站。
  - **AC-2.1**：WHEN 回收站非空 THEN 标题栏显示「清空回收站」按钮
  - **AC-2.2**：WHEN 点「清空回收站」THEN 确认框；确认后 `emptyTrash` 永久删除全部软删文章 + 成功 toast（含删除数）+ 重新拉取
  - **AC-2.3**：WHEN 确认被取消 THEN 不删除
- **US-3**：作为 admin，我要知道回收站会过期。
  - **AC-3.1**：WHEN 页面底部 THEN 显示过期提示文案（被删文章保留 N 天，到期自动清理）

## 3. 前端

- **布局**（ASCII）：

```
┌───────────────────────────┐
│ 顶栏（admin 布局）        │
│  h1 回收站      [清空回收站] │
│  副标题                     │
│  表格：标题 | 作者 | 删除时间 | 操作 │
│  （底部）过期提示          │
└───────────────────────────┘
```

- **组件树**：`Layout > AdminTrashPage > (admin-head / admin-table)`
- **关键交互**：
  - 挂载时 `user.role === 'admin'` 才 `load()`
  - 恢复走 `confirmDialog`（带标题）
  - `busyId` 锁当前行恢复按钮
  - 清空走 `confirmDialog`（通用文案）→ `emptyTrash()` → toast（`{n}` 删除数）→ `load()`
  - 空态 / 加载态 / 过期提示
- **路由**：`/admin/trash`
- **i18n 键**：`admin.*`（trashTitle / trashSub / name / deletedAt / restore / restored / emptyTrash / expiredHint / views / emptyTrashBtn / emptyTrashConfirm / emptyTrashDone）
- **共享组件依赖**：`Layout` / `i18n` / `Toast` / `confirmDialog`

## 4. 后端

- **API 端点**：
  - `GET /api/trash` → `TrashPost[] { id, title, author_id, author, view_count, deleted_at }`
  - `POST /api/trash/<id>/restore` → 恢复（`Unscoped` 清除 `deleted_at`）
  - `POST /api/trash/empty` → `{ deleted: N }`（`Unscoped` 永久删除全部软删文章）
- **Service 方法**：`svc.ListTrash` / `svc.RestoreTrash` / `svc.EmptyTrash`
- **数据模型**：`posts.deleted_at`（GORM 软删，`Unscoped()` 查询 / 恢复 / 永久删除）
- **权限检查**：`requireAdmin`
- **错误处理**：不存在 404；已过期清理后 410；清空失败 500

## 5. 数据流

```
挂载 → listTrash（Unscoped 查 deleted_at IS NOT NULL）→ 渲染表格
恢复 → restoreTrash → Unscoped 清 deleted_at → 成功 toast + load()
清空 → confirmDialog → emptyTrash → Unscoped 永久删除全部 → toast + load()
（后台：过期文章由清理任务删除，超保留期）
```

- **可能失败点**：目标已过期被清理（410）
- **回退**：失败 toast + 重新拉

## 5.5 性能与资源

- **前端包体积**：复用 admin-table，影响 < 1 kB
- **后端查询量级**：被删文章数（通常个位数~几十）；清空删全表一次
- **内存 / 磁盘**：软删文章仍占 SQLite 空间，未真正释放（靠清空 / 过期清理释放）
- **慢点在哪**：无
- **是否需重新评估**：回收站长期不清理 → 磁盘膨胀；需配清理策略

## 6. 边界情况

- 回收站空：「回收站是空的」且不显示清空按钮
- 目标已过期：恢复失败 410 + toast
- 作者字段缺失：显示 `author_id`
- 清空确认取消：不删除

## 7. 已知限制 / 未来方向

- 无过期天数可配置（当前写死）；候选＝后台设置保留天数
- 候选：被删文章详情页（只读）
- 清空为一次性全删，无二次确认步骤（仅一次 confirm）；候选＝输入确认文本

## 8. 相关

- 依赖 spec：`specs/06-admin-layout.md`、`specs/01-list-page.md`（删除入口）
- 相关决策：无
