> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 分类管理（`/admin/categories`）

> 状态：Done（代码已实现，本 spec 首次补写；第 18 轮逐条核验后转 Done）
> 路由：`/admin/categories`
> 权限：admin
> 轮次：第 7、9 轮
> 最后更新：2026-09-20

## 1. 目标

admin 管理文章分类：新增、重命名、删除；每行显示分类下的文章数。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要新增 / 重命名 / 删除分类。
  - **AC-1.1**：WHEN 输入名点「添加」或按 Enter THEN 创建分类 + 成功 toast + 重新拉取
  - **AC-1.2**：WHEN 点某行「重命名」THEN 该行变内联输入；Enter/保存 → 更新，Esc/取消 → 还原
  - **AC-1.3**：WHEN 点「删除」THEN 确认框（带分类名）；确认后删 + 重新拉取
- **US-2**：作为 admin，我要看到每个分类的文章数。
  - **AC-2.1**：WHEN 列表渲染 THEN 每行显示 `count`（该分类下未删文章数）

## 3. 前端

- **布局**（ASCII）：

```
┌───────────────────────────┐
│ 顶栏（admin 布局）        │
│  h1 分类管理 + 副标题      │
│  [输入框] [添加]          │
│  表格：分类 | 文章数 | 操作 │
└───────────────────────────┘
```

- **组件树**：`Layout > AdminCategoriesPage > (admin-form / admin-table)`
- **关键交互**：
  - 挂载时 `user.role === 'admin'` 才 `load()`；非 admin 显示「无管理员权限」
  - `editingId` 控制某行是否处于内联编辑态
  - 删除走 `confirmDialog`
  - 每次变更后 `load()` 重新拉（带 count）
- **路由**：`/admin/categories`
- **i18n 键**：`admin.*`（categoriesTitle / categoriesSub / add / rename / deleteConfirmCat / saved / deleted / emptyCat / postCount）
- **共享组件依赖**：`Layout` / `i18n` / `Toast` / `confirmDialog`

## 4. 后端

- **API 端点**：
  - `GET /api/categories/with-count` → `CategoryWithCount[] { id, name, count }`
  - `POST /api/categories` `{ name }` → 创建
  - `PATCH /api/categories/<id>` `{ name }` → 重命名
  - `DELETE /api/categories/<id>` → 删除（软处理，解绑文章）
- **Service 方法**：`svc.ListCategoriesWithCount` / `svc.CreateCategory` / `svc.UpdateCategory` / `svc.DeleteCategory`
- **数据模型**：`categories`（`name` 唯一），文章关联 `posts.category_id`
- **权限检查**：`requireAdmin`（所有写操作）
- **错误处理**：重名 409；不存在 404

## 5. 数据流

```
挂载 → listCategoriesWithCount → 渲染表格（含 count）
添加/重命名/删除 → createCategory/updateCategory/deleteCategory → 成功 toast + load()
```

- **可能失败点**：重名 409 / 网络失败
- **回退**：失败 toast + 保留旧数据

## 5.5 性能与资源

- **前端包体积**：复用 admin-table / admin-form，影响 < 1 kB
- **后端查询量级**：分类数（几十级）+ `count` 聚合（每个分类一次 `COUNT(*)`）
- **内存 / 磁盘**：无
- **慢点在哪**：分类数上百时 `count` 聚合 N 次查询；候选＝一次 `GROUP BY`
- **是否需重新评估**：分类 > 100 时聚合优化

## 6. 边界情况

- 无分类：空态「暂无分类」
- 重命名为空：阻止保存
- 删除有文章的分类：文章 `category_id` 置空，文章保留

## 7. 已知限制 / 未来方向

- 无分类层级（扁平）；候选＝树形分类
- 候选：分类按文章数排序 / 置顶

## 8. 相关

- 依赖 spec：`specs/06-admin-layout.md`
- 相关决策：无
