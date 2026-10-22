> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 标签管理（`/admin/tags`）

> 状态：Done（代码已实现，本 spec 首次补写；第 18 轮逐条核验后转 Done）
> 路由：`/admin/tags`
> 权限：admin
> 轮次：第 7、9 轮
> 最后更新：2026-09-20

## 1. 目标

admin 查看与删除文章标签（标签在文章编辑时自动创建），每行显示关联文章数。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要查看 / 删除标签。
  - **AC-1.1**：WHEN 列表渲染 THEN 每行显示 `name`（chip 样式）+ `count`（关联文章数）
  - **AC-1.2**：WHEN 点「删除」THEN 确认框（带标签名）；确认后删 + 解绑文章 + 重新拉取
- **US-2**：作为 admin，我要知道标签从哪来。
  - **AC-2.1**：WHEN 没有独立"新建标签"入口 THEN 标签只能由文章编辑页打标签自动产生（见 spec 03）

## 3. 前端

- **布局**（ASCII）：

```
┌───────────────────────────┐
│ 顶栏（admin 布局）        │
│  h1 标签管理 + 副标题      │
│  表格：标签 | 文章数 | 操作 │
└───────────────────────────┘
```

- **组件树**：`Layout > AdminTagsPage > admin-table`
- **关键交互**：
  - 挂载时 `user.role === 'admin'` 才 `load()`
  - 标签用 `.post-tag-chip` 渲染
  - 删除走 `confirmDialog`，带标签名
  - 无"新建"入口（标签随文章自动产生）
- **路由**：`/admin/tags`
- **i18n 键**：`admin.*`（tagsTitle / tagsSub / deleteConfirmTag / deleted / emptyTag / postCount）
- **共享组件依赖**：`Layout` / `i18n` / `Toast` / `confirmDialog`

## 4. 后端

- **API 端点**：
  - `GET /api/tags/with-count` → `TagWithCount[] { id, name, count }`
  - `DELETE /api/tags/<id>` → 删除（软处理，解绑 `post_tags`）
- **Service 方法**：`svc.ListTagsWithCount` / `svc.DeleteTag`
- **数据模型**：`tags`（`name` 唯一）+ 多对多 `post_tags`
- **权限检查**：`requireAdmin`（写操作）
- **错误处理**：不存在 404

## 5. 数据流

```
挂载 → listTagsWithCount → 渲染表格（含 count）
删除 → deleteTag → 解绑 post_tags → 成功 toast + load()
```

- **可能失败点**：网络失败
- **回退**：失败 toast + 保留旧数据

## 5.5 性能与资源

- **前端包体积**：复用 admin-table，影响 < 1 kB
- **后端查询量级**：标签数（几十级）+ 每标签 `COUNT(*)`
- **内存 / 磁盘**：无
- **慢点在哪**：标签多时 N 次聚合；候选＝一次 `GROUP BY`
- **是否需重新评估**：标签 > 200 时聚合优化

## 6. 边界情况

- 无标签：空态「暂无标签」
- 删除有关联文章的标签：仅解绑，文章保留

## 7. 已知限制 / 未来方向

- 无独立"新建标签"入口（随文章产生）；候选＝手动批量建标签
- 候选：标签云大小按文章数渲染（侧栏已有，管理页可选）

## 8. 相关

- 依赖 spec：`specs/06-admin-layout.md`、`specs/03-editor.md`（标签来源）
- 相关决策：无
