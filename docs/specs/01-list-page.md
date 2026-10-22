> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 文章列表页（`/`）

> 状态：Done（代码已实现，本 spec 首次补写；第 15 轮加入懒加载；第 16 轮加入归档页）
> 路由：`/`（列表）、`/archive`（归档，US-5）
> 权限：登录（`Guarded`，无 token 重定向到 `/login`）
> 轮次：第 1–2、7、10、12、15、16 轮
> 最后更新：2026-09-21

## 1. 目标

登录作者的主入口：浏览、筛选、搜索、分页自己的文章，快速进入阅读 / 编辑 / 删除。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为作者，我要按状态 / 分类 / 标签筛选并搜索文章，以便快速定位。
  - **AC-1.1**：WHEN 搜索框输入 THEN 300ms 防抖后重新拉取列表并重置到第 1 页
  - **AC-1.2**：WHEN 选中某分类（工具栏下拉或侧栏）THEN 列表只显示该分类文章并重置到第 1 页
  - **AC-1.3**：WHEN 点击某标签 THEN 按标签名过滤；再点同一标签则取消过滤
  - **AC-1.4**：WHEN 搜索词命中标题 / 正文 / 作者名 THEN 命中片段以 `<mark class="hl">` 高亮
- **US-2**：作为作者，我要翻页浏览文章。
  - **AC-2.1**：WHEN 总条目数 > 20 THEN 显示分页（每页 20 条）；上一页 / 下一页在边界置灰
- **US-3**：作为作者，我要懒加载（无限滚动）查看后续文章（第 15 轮加入）。
  - **AC-3.1**：WHEN 页面底部哨兵元素进入视口（IntersectionObserver，阈值 0）且还有更多页 THEN 追加加载下一页（20 条/页），与已有列表拼接，不重置页面
  - **AC-3.2**：WHEN 已加载到最后一页（无更多）THEN 停止监听并显示"没有更多了"提示
  - **AC-3.3**：WHEN 搜索 / 分类 / 状态 / 标签筛选变化 THEN 重置到第 1 页并清空已加载列表（防旧数据残留）
  - **AC-3.4**：WHEN 加载下一页失败 THEN 错误 toast + 停止懒加载（保留已加载数据）；401 跳登录
  - **AC-3.5**：WHEN 加载中 THEN 底部显示加载占位（loading spinner / "加载中…"）
- **US-4**：作为作者，我要删除某文章。
  - **AC-4.1**：WHEN 点「删除」THEN 弹确认框；确认后删成功 → 成功 toast + 重新拉当前页；失败 → 错误 toast
- **US-5**：作为读者/作者，我要按年月归档浏览全部已发布文章（第 16 轮加入，独立 `/archive` 页）。
  - **AC-5.1**：WHEN 打开 `/archive` THEN 展示全部已发布文章，按「年-月」分组（组标题为本地化年月，如 `2026年9月` / `September 2026`），组内按发布时间倒序，顶部显示总篇数
  - **AC-5.2**：WHEN 无已发布文章 THEN 显示空态（区分登录/未登录）
  - **AC-5.3**：WHEN 点某文章标题 THEN 跳转 `/read/<id>`

## 3. 前端

- **布局**（ASCII）：

```
┌────────────────────────────────────────────────┐
│ 顶栏（Layout）                                 │
├────────────────────────────────────────────────┤
│ 主区 list-main                                 │
│  h1 归档                                        │
│  年月分组 × N（组标题 + 文章标题列表）          │
│  空态（暂无已发布文章 / 登录后查看）            │
└────────────────────────────────────────────────┘
（无侧栏；归档页独立，移动端单栏）
```

- **组件树**：`Layout > ArchivePage > (archive-group / post-item / empty-state)`
- **关键交互**：
  - 挂载时 `listArchive()` 拉取全部已发布文章 → 前端按 `created_at` 的「年-月」分组（`YYYY-MM`），组内按时间倒序
  - 组标题点击可折叠（可选）；文章标题点击 → `navigate('/read/<id>')`
  - 空态：区分「已登录但暂无已发布文章」/「未登录查看」
- **路由**：`/archive`，精确匹配，`Guarded`（需登录）
- **i18n 键**：`archive.*`、`post.*`、`common.*`
- **共享组件依赖**：`Layout` / `i18n` / `theme` / `Toast` / `router`

**ListPage（主列表，原有）**

- **组件树**：`Layout > ListPage > (工具栏 / post-list / pagination / lazy-load-sentinel) + Sidebar`
- **关键交互**：
  - 搜索用 `useDebounce(search, 300)`；`debouncedSearch` 变 → 重置 `page=1` + 清空已加载列表 → 触发重新加载
  - 懒加载（第 15 轮）：底部哨兵 div + `IntersectionObserver`；进入视口且还有下一页 → 追加加载 `page+1`，`items` 拼接保留（`pageData` 存已加载全部）；到底后显示"没有更多了"
  - 列表项点击 → `navigate('/read/<id>')`（SPA 内跳转，15 轮已修原生跳转 404 循环）
  - 标题 / 摘要用 `dangerouslySetInnerHTML` + `highlight(...)`（`md.ts`，`escapeHTML` + 包 `<mark>`）
  - 401 → `navigate('/login')`；其他错误 → `pushToast('error', ...)`
  - 空状态：有用户显示"暂无文章"，未登录显示"登录后查看"
- **路由**：`/`，精确匹配
- **i18n 键**：`list.*`、`sidebar.*`、`post.*`、`common.*`
- **共享组件依赖**：`Layout` / `i18n` / `theme` / `Toast` / `Sidebar` / `confirmDialog`

## 4. 后端

- **API 端点**：
  - `GET /api/posts?status=&q=&page=&size=&category=&tag=` → `PostPage { items, total, page, size }`
  - `GET /api/categories`、`GET /api/tags`、`GET /api/posts/recent` → 侧栏数据
  - `GET /api/posts/archive` → `Post[]`（全部已发布文章，含作者/分类，不含正文 body，按 created_at 倒序）— 归档页数据源
  - `DELETE /api/posts/<id>` → 删除（软删，进回收站）
- **Service 方法**：`svc.ListPostsPage(ctx, user, q)`、`svc.ListArchive(ctx, user)`、`svc.DeletePost(ctx, me, id)`
- **数据模型**：`posts`（含 `title`、`body`、`status`、`pinned`、`view_count`、`deleted_at`）+ `categories` / `tags` / `post_tags` 关联
- **搜索实现**：`q` 命中 `posts.title` / `posts.body` / 作者 `username`（LIKE，见 decision 0001）
- **权限**：需登录；列表只返回本人可见（作者或 admin）

## 5. 数据流

```
搜索/筛选/标签 → useDebounce + state → listPostsPaged(query, page=1)
  → GET /api/posts?... → svc.ListPostsPage → SQLite（软删过滤 + LIKE）
  → 返回 PostPage → setPageData（重置，items=[items]）
滚到底部 → IntersectionObserver 哨兵 → listPostsPaged(query, page+1)
  → 追加：items = pageData.items + 新 items（保留已加载）
  → 无更多页 → 显示"没有更多了"
删除 → deletePost(id) → 重新拉当前筛选下 page=1
归档（/archive）：listArchive() → GET /api/posts/archive → Post[] → 前端按 YYYY-MM 分组 → 渲染
```

- **可能失败点**：401（token 过期 → 跳登录）；网络失败（toast + 保留旧数据）
- **回退**：错误态不清空 `pageData`，用户可重试；懒加载失败保留已加载数据

## 5.5 性能与资源

- **前端包体积**：本页面复用 `renderMD` / `highlight` / `Sidebar`；懒加载用原生 `IntersectionObserver`（无新依赖）；对总包影响 < 6 kB；归档页为轻组件（无新依赖），额外 < 2 kB
- **后端查询量级**：每页 20 条 + total，`count + limit/offset`；懒加载每次 +20 条（千级文章内毫秒级）；归档一次全量已发布文章（不含 body，千级内 < 5 ms + 网络传输）
- **内存 / 磁盘**：`pageData` 存已加载全部（滚动越多驻留越多；单人使用 + 低资源 VPS，千级文章内存 < 1 MB）；归档页驻留全部已发布标题（千级 < 1 MB）
- **慢点在哪**：文章到 1 万时 LIKE 搜索变慢（见 decision 0001，暂不换 FTS）；懒加载每次网络请求 20 条，低资源 VPS 单次查询 < 50 ms；归档在 1 万篇时全量返回标题变大（需游标/分页）
- **是否需重新评估**：触发条件＝单页查询 > 200 ms 或文章 > 1 万；已加载内存超过 10 MB 时考虑虚拟滚动；归档 > 1 万篇时改分页

## 6. 边界情况

- 空列表：显示空状态（区分登录 / 未登录）
- 加载中：`loading` 占位
- 网络失败：保留旧数据 + 错误 toast
- 401：跳登录
- 搜索无结果：空状态
- 删除后当前页空：自动回退到上一页
- 懒加载到底：显示"没有更多了"，不再触发哨兵
- 筛选变化时懒加载中：重置到第 1 页 + 清空已加载（防旧数据残留）
- 归档：无已发布文章 → 空态；网络失败 → toast + 重试；分组为空组不渲染

## 7. 已知限制 / 未来方向

- 搜索是 LIKE，非全文（decision 0001）；候选方向＝换支持 FTS5 的驱动
- 懒加载（无限滚动）已实现（第 15 轮）；候选＝移动端加虚拟滚动（已加载内存超 10 MB 时）
- 归档页已实现（第 16 轮）；候选＝归档页分页 / 按分类归档

## 8. 相关

- 依赖 spec：`specs/06-admin-layout`（共享顶栏）、`specs/02-read-page`（点击跳转目标）
- 相关决策：`decisions/0001-pure-go-sqlite-no-fts5.md`
