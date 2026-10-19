# 三层角色 + 内容审核流 + root 封禁库

> 状态：Done（第 25 轮实现完成并全绿）
> 路由：`/admin/review`、`/admin/banned`、`/admin/users`（角色列增强）
> 权限：admin | root（封禁库仅 root）
> 轮次：第 25 轮
> 最后更新：2026-10-15

## 1. 目标

把「单管理员」模型升级为**三级角色 + 内容审核流 + root 专属封禁库**，支撑多作者站点：

- **角色**：`root`（唯一超级管理员）/ `admin`（普通管理员）/ `user`（普通作者）。
- **审核**：user 发文 → admin 或 root 审核；admin 发文 → root 审核；通过即 `published`，打回即 `rejected`（必带意见）。
- **封禁库**：root 可随时封禁任意 user 的文章；被封禁文章**仅 root 可见**（作者本人与其他 admin 都看不到），在封禁库统一管理，可「发回原作者修改 + 备注意见」或「永久删除」。

## 2. 用户故事 + 验收标准（EARS）

### 2.1 角色模型

- **US-1**：作为站点所有者，我要全库有且只有一个 root，且任何 API 都不能造出第二个 root。
  - **AC-1.1**：WHEN 调用 `POST /api/users` 或 `PUT /api/users/{id}` 且 `role=root` THEN 返回 403「the root role cannot be assigned」
  - **AC-1.2**：WHEN 启动时库内已存在 admin 且无 root THEN 迁移 `0003_root_migration` 把**最早创建**的 admin 升为 root
  - **AC-1.3**：WHEN 库内存在多个 root THEN 保留最早的一个，其余降级为 admin（保证唯一性）
  - **AC-1.4**：WHEN 全新库首启 THEN `seedDemoData` 直接创建 root（而非 admin）
- **US-2**：作为 root，我要把普通用户提升为 admin。
  - **AC-2.1**：WHEN root 对 user 行选择 admin THEN `PUT /api/users/{id}` 成功，角色变 admin
- **US-3**：作为 admin，我**不能**给任何人提权。
  - **AC-3.1**：WHEN admin 尝试把 user 改为 admin THEN 403
  - **AC-3.2**：WHEN admin 尝试修改另一个 admin 的角色 THEN 403
- **US-4**：作为 root，我要保护自己不被误操作。
  - **AC-4.1**：WHEN 任何请求试图降级 / 禁用 / 删除 root THEN 403

### 2.2 发文与审核

- **US-5**：作为 user，我发文后进入审核队列。
  - **AC-5.1**：WHEN user 调 `POST /api/posts` THEN 无论请求体 `status` 是什么（`published`/`private`/任意脏值）初始状态一律为 `pending`
  - **AC-5.2**：WHEN user 查看自己的文章列表 THEN 能看到自己的 `pending`/`rejected` 文章
- **US-6**：作为 admin，我发文后由 root 审核。
  - **AC-6.1**：WHEN admin 调 `POST /api/posts` THEN 初始状态为 `pending`
  - **AC-6.2**：WHEN admin 查看审核队列 THEN **看不到**自己或别的 admin 发的 pending（只能审 user 的发文）
- **US-7**：作为 root，我发文直接发布或私密。
  - **AC-7.1**：WHEN root 调 `POST /api/posts` 且 `status=private` THEN 保持 `private`
  - **AC-7.2**：WHEN root 不传 status THEN 默认 `published`
- **US-8**：作为审核者，我要通过或打回一篇文章。
  - **AC-8.1**：WHEN `POST /api/admin/review/{id}` 且 `decision=approve` THEN 状态变 `published`
  - **AC-8.2**：WHEN `decision=reject` 且 `note` 为空 THEN 400「打回时必须填写审核意见」
  - **AC-8.3**：WHEN `decision=reject` 且 `note` 非空 THEN 状态变 `rejected`，`review_note` 记录意见，作者收到邮件通知
  - **AC-8.4**：WHEN admin 尝试审核一篇 admin/root 发的文章 THEN 403
- **US-9**：作为 root，我要看全部 pending。
  - **AC-9.1**：WHEN root 调 `GET /api/admin/pending` THEN 返回所有 pending（含 admin 发的）

### 2.3 可见性

- **US-10**：作为访客，我只能看已发布文章。
  - **AC-10.1**：WHEN 匿名访问列表/详情/feed/sitemap THEN 只见 `published`
- **US-11**：作为作者，我能看自己的受限文章，但**看不到自己被封禁的文章**。
  - **AC-11.1**：WHEN user 查看自己的 `private`/`pending`/`rejected` THEN 可见
  - **AC-11.2**：WHEN 文章的 `status=banned` THEN **作者本人也看不到**（列表过滤 + 详情 404）
- **US-12**：作为 admin，我能看所有非 banned 文章。
  - **AC-12.1**：WHEN admin 查看列表 THEN 见 public + 所有用户的 `private`/`pending`/`rejected`，**排除** `banned`
- **US-13**：作为 root，我能看一切。
  - **AC-13.1**：WHEN root 查看列表 THEN 含 `banned`

### 2.4 封禁库

- **US-14**：作为 root，我要封禁一篇文章。
  - **AC-14.1**：WHEN `POST /api/admin/ban/{id}` THEN 状态变 `banned`，从所有非 root 视野消失
  - **AC-14.2**：WHEN admin 调 ban THEN 403（root only）
- **US-15**：作为 root，我要在封禁库管理工作。
  - **AC-15.1**：WHEN `GET /api/admin/banned` THEN 返回全部 banned（含作者信息）
  - **AC-15.2**：WHEN `POST /api/admin/banned/{id}/send-back` 且 `note` 非空 THEN 状态变 `rejected`，离开封禁库，作者收到邮件
  - **AC-15.3**：WHEN `DELETE /api/admin/banned/{id}` THEN **硬删**（Unscoped），不可恢复
- **US-16**：作为 admin，我完全看不到封禁库。
  - **AC-16.1**：WHEN admin 访问 `/admin/banned` 路由 THEN 页面显示「此页面仅限超级管理员访问」
  - **AC-16.2**：WHEN admin 调 `GET /api/admin/banned` THEN 403

## 3. 前端

- **布局**：

```
审核工作台（/admin/review）              封禁库（/admin/banned，root only）
┌──────────────────────────────┐      ┌──────────────────────────────┐
│ 内容审核                      │      │ 封禁库                        │
│ 普通用户文章由管理员审核；      │      │ 被封禁文章仅超级管理员可见     │
│ 管理员文章由超级管理员审核      │      │                              │
│ 表格：标题|作者|提交时间|操作   │      │ 表格：标题|作者|封禁时间|操作   │
│  操作：通过 · 打回             │      │  操作：发回原作者 · 永久删除   │
└──────────────────────────────┘      └──────────────────────────────┘
打回弹窗：审核意见（必填 textarea）
发回弹窗：修改意见（必填 textarea）
```

- **组件树**：
  - `AdminLayout > AdminReviewPage > (admin-table / reject-modal)`
  - `AdminLayout > AdminBanVaultPage > (admin-table / send-back-modal)`
  - `AdminLayout > AdminUsersPage > (role-select 分级 / badge-root)`
- **关键交互**：
  - 审核页：`approve` 直接调 `reviewPost(id, {decision:'approve'})`；`reject` 开弹窗强制填 note 后才可提交
  - 封禁库：`send-back` 开弹窗填 note；`delete` 走 confirmDialog 二次确认
  - 用户管理：`canManage(u)` 判定——root 行禁改、admin 行仅 root 可管、user 行 admin/root 均可管；角色下拉对 admin 只给 user 选项；root 显示只读徽章
  - 后台侧栏按角色过滤：封禁库导航 `rootOnly: true`，admin 不可见
  - 编辑器对非 root 显示「本文需经审核通过后才会公开」提示
  - 个人中心对 `pending`/`rejected`/`banned` 文章禁用可见性切换，按钮降级为只读状态标签
- **路由**：`/admin/review`（Admin）、`/admin/banned`（Root）
- **i18n 键**：`admin.reviewTitle/reviewSub/reviewEmpty/submittedAt/approve/reject/rejectTitle/reviewNote/reviewNotePlaceholder/reviewNoteRequired/reviewApproved/reviewRejected/rejectHint`、`admin.banVaultTitle/banVaultSub/banVaultEmpty/bannedAt/sendBack/sendBackTitle/sendBackHint/sendBackDone/deleteBanned/deleteBannedConfirm`、`admin.roleRoot/roleLocked/rootOnly`、`admin.statusLabel.{published,private,pending,rejected,banned}`、`editor.visibilityReviewHint`
- **共享组件依赖**：`AdminLayout` / `i18n` / `Toast` / `confirmDialog` / `api.*` / `.post-status-*` CSS

## 4. 后端

- **API 端点**：

| 方法 | 路径 | 角色 | 说明 |
|---|---|---|---|
| GET | `/api/admin/pending` | admin\|root | 审核队列；admin 仅见 user 发文的 pending，root 全见 |
| POST | `/api/admin/review/{id}` | admin\|root | 体 `{"decision":"approve"\|"reject","note":"..."}`；reject 必带 note |
| POST | `/api/admin/ban/{id}` | **root** | 拉入封禁库 |
| GET | `/api/admin/banned` | **root** | 封禁库列表 |
| POST | `/api/admin/banned/{id}/send-back` | **root** | 体 `{"note":"..."}`；banned→rejected + 邮件 |
| DELETE | `/api/admin/banned/{id}` | **root** | 硬删（Unscoped） |

- **Service 方法**：
  - `svc.ReviewPost(ctx, actor, id, ReviewPostInput{Action, Note}) (models.Post, error)`
  - `svc.BanPost(ctx, actor, id) (models.Post, error)`
  - `svc.SendBackPost(ctx, actor, id, note) (models.Post, error)`
  - `svc.ListBanned(ctx, actor) ([]models.Post, error)`
  - `svc.ListPending(ctx, actor) ([]models.Post, error)`
  - `svc.DeleteBannedPermanently(ctx, actor, id) error`
  - 权限 helper（`services/roles.go`）：`IsRoot` / `IsAdminOrRoot` / `canManageUser` / `canReviewPost`
- **数据模型**：
  - `users.role`：`root` | `admin` | `user`（root 全库唯一）
  - `posts.status`：`published` | `private` | `pending` | `rejected` | `banned`
  - `posts.review_note`(text) / `posts.reviewed_by`(size:36) / `posts.reviewed_at`(*time.Time)
- **权限检查**：
  - handler 层 `requireAdmin`（admin-or-root）/ `requireRoot`（root-only）作第一道防线
  - **service 层再做完整校验**（handler 可被绕过，service 是权威）
  - `CreatePost` 按角色定初始状态：root 依请求；admin/user 一律 `pending`
  - `canSeePost(user, post)` 是可见性唯一权威
- **错误处理**：
  - 403：越权（非 root 调 root 端点、admin 审 admin 文章、改 root 角色）
  - 400：reject/send-back 未带 note
  - 404：文章不存在 / `banned` 对非 root 不可见（不泄露存在性）

## 5. 数据流

```
user 发文
  → POST /api/posts（handler 只解 DTO）
  → services.CreatePost 按 role 定 status=pending
  → 落库

admin 审核
  → GET /api/admin/pending（service 按 actor.role 过滤：admin 只见 user 发文）
  → POST /api/admin/review/{id} {decision, note}
  → services.ReviewPost 校验 canReviewPost → 改 status + 写 review_note/by/at
  → MailReviewResult 邮件通知作者（best-effort，失败不阻断）

root 封禁
  → POST /api/admin/ban/{id}
  → services.BanPost（root-only）→ status=banned
  → 该文章从所有非 root 查询中消失（ListPostsPage / GetPost / FeedPosts 均不返回）

root 发回
  → POST /api/admin/banned/{id}/send-back {note}
  → status=banned→rejected，note 记录，邮件通知
```

关键链路：`ListPostsPage` 的四档过滤（root 全见 / admin 排除 banned / user 见 public+本人 restricted 且排除 banned / 匿名仅 published）与 `GetPost` 的 `canSeePost` 必须保持一致，否则会出现「列表看不到但详情能开」的越权。

## 5.5 性能与资源

- **前端包体积**：新增两个页面 + i18n 文案，JS 由 ~131 kB 增至 206.28 kB（gzip 63.42 kB），CSS 57.29 kB（gzip 10.76 kB）。两个新页面均为薄表格页，无重组件。
- **后端查询量级**：`ListPending`/`ListBanned` 都是 `WHERE status IN (...)` + Preload(Author)，行数级 = 待审/封禁文章数（小型站几十条级）。`ListPostsPage` 多一个 `status` 条件，不增加索引压力。
- **内存 / 磁盘预算**：无新增常驻结构；`review_note` 为 text 列，仅在被打回/发回时写入。
- **慢点在哪**：`ListPending` 未加复合索引，pending 过千时全表扫描——当前规模可接受；量大时加 `(status, created_at)` 索引。
- **是否需重新评估**：无。未引入新依赖、新服务、新后台任务。

## 6. 边界情况

- **空数据**：审核队列空 → 「暂无待审核文章」；封禁库空 → 「封禁库为空」
- **加载中**：两页均有 `loading` 态
- **网络失败**：统一 toast `admin.failed`；审核页失败不关弹窗，允许重试
- **404 / 403**：banned 对非 root 返回 404（与不存在一致）；admin 访问封禁库页显示「仅限超级管理员」
- **并发 / 重复提交**：`busyId` 单行锁定，防止同一行重复提交审核；`ReviewPost` 对非 pending 状态拒绝（幂等保护）
- **迁移幂等**：`0003_root_migration` 与 `0004_add_review_columns` 均有 `schema_migrations` 门控，重复启动不重跑
- **多 root 兜底**：迁移发现多个 root 时保留最早、其余降 admin，保证唯一性不变式

## 7. 已知限制 / 未来方向

- **未写独立前端 spec 流程**：本轮由口述需求直接实现，本文件为事后补录（规则以本文档 + HANDOVER §5.3/§7.7 为准）。
- **审核无批量操作**：一次只能审一篇；多篇排队时效率低。候选方向：批量通过（勾选多行）。
- **banned 文章无申诉通道**：作者看不到自己的 banned 文章，也无法申诉。候选方向：root 发回时即视为申诉结果；或增加「作者可查看自己被封禁文章（只读）」的可配置项。
- **审核意见仅邮件通知**：未做站内通知/消息中心。候选方向：加 `notifications` 表。
- **封禁库无搜索/筛选**：文章量大时只能翻页。候选方向：按作者/时间筛选。
- **root 密码恢复**：root 唯一且不可经 API 修改角色，若 root 账号被禁/遗忘密码，只能直改数据库（当前无自助恢复路径）。

## 8. 相关

- 依赖 spec：`specs/09-admin-users.md`（用户管理，本轮扩展为三层角色）、`specs/20-visibility.md`（可见性，本轮扩为五值状态）、`specs/14-login-log.md`
- 相关决策：`decisions/0001-*`（纯 Go SQLite）、`decisions/0004-*`（CORS）
- 实现位置：
  - `internal/models/models.go`（角色/状态常量 + Post 审核字段）
  - `internal/services/roles.go`（权限 helper）
  - `internal/services/services.go`（`CreatePost`/`canSeePost`/`ListPostsPage`/`ReviewPost`/`BanPost`/`SendBackPost`/`ListBanned`/`ListPending`/`DeleteBannedPermanently`）
  - `internal/services/accounts.go`（用户管理角色白名单）
  - `internal/services/mail.go`（`MailReviewResult`）
  - `internal/config/config.go`（迁移 `0003_root_migration` / `0004_add_review_columns`）
  - `internal/handlers/moderation.go`（6 个端点）
  - `internal/handlers/admin.go`（`requireAdmin` 升级 + `requireRoot`）
  - `web/src/pages/AdminReviewPage.tsx`、`web/src/pages/AdminBanVaultPage.tsx`
  - `web/src/ui/AdminLayout.tsx`（导航分级）
  - `web/src/pages/AdminUsersPage.tsx`（层级化管理）
