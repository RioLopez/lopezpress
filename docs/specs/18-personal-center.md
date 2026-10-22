> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 个人中心

> 状态：Draft（第 20 轮新增）
> 路由：`/me`（个人中心，登录）
> 权限：登录（未登录访问由 `Guarded` 跳 `/login`，登录后回 `/me`）
> 轮次：第 20 轮
> 最后更新：2026-09-22

## 1. 目标

为已登录用户提供「自己的主页」：左侧三栏（我的主页 / 内容管理 / 安全中心）切换右侧内容。PC 端一期优先；移动端改为顶部横向标签，本期仅预留结构。当前假设个人中心仅展示当前登录用户自身数据（spec 20 §八.5）。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为用户，我要进入个人中心。
  - **AC-1.1**：WHEN 访问 `/me` THEN 左侧默认选中「我的主页」
  - **AC-1.2**：WHEN 点击左侧栏目 THEN 右侧内容切换，左侧高亮保持（不整体刷新）
- **US-2**：作为用户，我要看「我的主页」概览。
  - **AC-2.1**：WHEN 「我的主页」渲染 THEN 顶部信息横栏含头像、名称、总访问量、文章数
  - **AC-2.2**：WHEN 横栏下方渲染 THEN 显示注册时间、个人简介、编辑资料入口（→ `/settings`）
  - **AC-2.3**：WHEN 大屏（≥1440px）THEN 横栏左右保留约 15% 留白（中屏 8%、小屏 PC 4%、移动端 16px）
  - **AC-2.4**：WHEN 加载用户统计 THEN 调用 `GET /api/me/stats` 回填总访问量/文章数
- **US-3**：作为用户，我要浏览「我的主页」文章列表。
  - **AC-3.1**：WHEN 文章列表渲染 THEN 每篇显示标题、发布时间、浏览量，点击进 `/read/:id`
  - **AC-3.2**：WHEN 切换排序 THEN 重新请求并回到第 1 页，显示加载态；失败保留列表并提示
  - **AC-3.3**：WHEN 列表分页 THEN 默认每页 10 条，支持上一页/下一页/页码，数据端按页返回
- **US-4**：作为用户，我要管理「内容管理」中的文章。
  - **AC-4.1**：WHEN 「内容管理」渲染 THEN 列表每篇含状态徽标 + 编辑按钮（→ `/edit/:id`）
  - **AC-4.2**：WHEN 分页 THEN 同「我的主页」规则（默认 10 条 / 按发布时间倒序）
- **US-5**：作为用户，我要在「安全中心」看账号信息。
  - **AC-5.1**：WHEN 「安全中心」渲染 THEN 显示账号（邮箱，部分脱敏）与用户名
  - **AC-5.2**：WHEN 点击「显示/隐藏」THEN 邮箱在脱敏/明文间切换
  - **AC-5.3**：WHEN 一期渲染 THEN 仅展示基础账号信息，并注明更多安全能力后续开放
- **US-6**：作为用户，我要看到完整状态与边界。
  - **AC-6.1**：WHEN 统计/列表加载中 THEN 显示加载态
  - **AC-6.2**：WHEN 无文章 THEN 显示「暂无文章」/「暂无内容」
  - **AC-6.3**：WHEN 请求失败 THEN 显示错误提示 + 重试按钮

## 3. 前端

- **布局**（ASCII，PC）：

```
┌───────────────────────────────────────────────┐
│ 个人中心                                        │
├────────────┬──────────────────────────────────┤
│ 我的主页    │ [头像 名称]    总访问量  文章数    │
│ 内容管理    │ 注册时间：YYYY-MM-DD              │
│ 安全中心    │ 简介…              [编辑资料]      │
│            │ ── 我的文章 ──                     │
│            │ [排序: 时间▼ 顺序▼]                │
│            │ 标题            时间 · 浏览        │
│            │ …（分页）                           │
└────────────┴──────────────────────────────────┘
```

- **组件树**：`Layout > PersonalCenterPage`（路由 `/me`）> `MyHomepage` / `ContentManage` / `SecurityCenter`
- **关键交互**：
  - 三栏目通过 `tab` 状态切换，右侧内容条件渲染（不整体刷新）
  - 我的主页：横幅（`myStats()` + `user`）+ 排序下拉（发布时间/浏览量 × 正序/倒序，默认 发布时间倒序）+ `listPostsPaged({page,size:10,sort,order})` 分页
  - 内容管理：`listPostsPaged({page,size:10})` 默认发布时间倒序，每行状态徽标（`badge--published/draft/scheduled`）+ 编辑按钮
  - 安全中心：邮箱 `maskEmail()` 脱敏，显示/隐藏切换
- **路由**：`/me`（登录，包 `Guarded`）；内部跳转 `/settings`、`/read/:id`、`/edit/:id`
- **i18n 键**：`center.*`（title / myHome / contentManage / security / bannerViews / bannerPosts / registeredAt / bio / editProfile / postsList / sortBy / sortTime / sortViews / orderAsc / orderDesc / empty / manageEmpty / status / edit / account / username / maskedHint / show / hide / securityEmpty / securityNote / loading / loadError / retry）
- **共享组件依赖**：`Layout` / `auth` / `i18n` / `router` / `api`（myStats, listPostsPaged）/ Toast

## 4. 后端

- **API 端点**：
  - `GET /api/me/stats`（登录）→ `{ posts_total, views_total }`：当前用户 `author_id` 的文章计数 + `SUM(view_count)`（spec 20 新增）
  - `GET /api/posts`（登录）→ 复用 `ListPostsPage`，新增 `sort=created_at|views`、`order=asc|desc` 查询参数（pinned 始终置顶）；`page`/`size` 分页，默认每页 10 条
- **Service 方法**：`svc.UserStats(ctx, user)`；`svc.ListPostsPage(ctx, user, ListPostsQuery{SortBy, Order, ...})`
- **数据模型**：读 `users`（当前用户资料）、`posts`（author_id 过滤）
- **权限检查**：`/me/stats` 与 `/posts` 均需 JWT（`withAuth`）
- **错误处理**：未登录 401；列表/统计失败返回 500（前端局部提示 + 重试）

## 5. 数据流

```
/me → 加载 user(auth context) + myStats() + listPostsPaged()
切换栏目 → 仅右侧组件切换
排序/翻页 → 重置 page=1 → 重新请求 → 回填列表
点击文章/编辑 → navigate(/read/:id | /edit/:id)
```

- **可能失败点**：统计或列表请求失败 → 保留框架，局部错误 + 重试（不影响横幅/其他栏目）
- **回退**：显示名缺失回退 `username`；头像缺失渲染首字母

## 5.5 性能与资源

- **前端包体积**：`PersonalCenterPage` 约 +3 kB（含三个栏目与分页/排序）
- **后端查询量级**：`/me/stats` 2 条聚合（COUNT + SUM，索引 `author_id`）；`/posts` 一次 COUNT + 一页 10 行（Preload Author/Category/Tags）
- **内存 / 磁盘**：无新增常驻结构
- **慢点在哪**：作者文章到万级时 `SUM(view_count)` 仍走索引聚合，可接受；列表分页已按页返回
- **是否需重新评估**：无（与现有 VPS 资源预算一致）

## 6. 边界情况

- 未登录访问 `/me`：`Guarded` 跳 `/login`，登录后回 `/me`（spec 20 §八.1）
- 加载中：横幅统计显 `–`，列表显加载态
- 空状态：我的主页「暂无文章」、内容管理「暂无内容」、安全中心「暂无账号信息」
- 错误态：网络错误显提示 + 重试；头像失败显首字母
- 名称过长：`.center-banner-name` 截断
- 权限（spec 20 §八.5）：当前仅本人数据；若未来开放他人主页，则「内容管理 / 安全中心 / 编辑资料」对访客隐藏

## 7. 已知限制 / 未来方向

- 移动端侧边栏已降级为顶部横向标签（CSS 响应式），但抽屉/吸顶交互细节未深做
- 「我的主页」与「内容管理」列表来源相同（均当前用户文章），差异在编辑按钮与状态列；后续可合并为带模式切换的单一列表
- 安全中心一期仅展示账号/用户名，候选扩展见 spec 20 §六.2（改密、绑定手机/邮箱、两步验证、登录设备、退出所有设备、注销账号）

## 8. 相关

- 依赖 spec：`specs/17-user-menu.md`（入口菜单）、`specs/15-profile-author.md`（资料编辑 `/settings`、公开作者页 `/u/:username`）、`specs/05-settings.md`（账号设置）、`specs/04-auth.md`（登录态）
- 相关决策：无
