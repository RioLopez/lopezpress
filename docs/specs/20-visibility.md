# 文章可见范围（公开 / 私密，去掉草稿与定时）

> 状态：Implemented（代码已落地，见 §7 验证记录）
> 路由：影响 `/create`、`/edit/:id`、`/center`（内容管理）、`/read/:id`、`/`（列表）
> 权限：登录（写/改）、公开（读公开文）、作者本人（读私密文）
> 轮次：第 22 轮
> 最后更新：2026-09-22
> 相关：`specs/19-write-editor.md`（写文章页元面板的"状态"下拉已被本 spec 的"可见范围"单选取代）

## 0. 读接口鉴权（回归修复，2026-09-22 追加）

主界面（列表 / 侧栏 / 归档 / 最近）必须**未登录也能读**。修复方案：

- 中间件：`handlers/auth.go` 新增 `WithOptionalAuth` —— 无 Authorization 头则匿名放行（零值 `User{}`）；**有** token 但无效/过期仍 401。写/管理接口保持 `WithAuth` 强鉴权。
- 路由：`GET /api/posts`、`/api/posts/recent`、`/api/posts/archive`、`/api/posts/{id}`、`/api/posts/{id}/nav`、`/api/categories`、`/api/tags` 改为 `withOptionalAuth`；其余写接口保留 `withAuth`。
- service 过滤语义（spec 22 定稿）：
  - **匿名（`user.ID == ""`）**：仅 `status = 'published'`（全部公开文，任何私密都不可见）。
  - **登录非 admin**：`status = 'published' OR author_id = ?`（所有公开 + 自己的私密，永不含他人私密）。
  - **admin**：全部。
- 影响范围：`ListPostsPage`（首页、内容管理）、`ListArchive`（归档页）。`ListCategories/ListTags/GetRecentPosts` 不依赖 user，纯公共读。

## 1. 目标

个人博客场景下，"草稿 + 定时发布"带来的"未发布内容怎么存、怎么恢复、列表怎么过滤、按钮怎么变、自动保存怎么算"复杂度不划算。定稿为**只有两种可见范围**：

- **公开（public）**：未登录读者也能浏览（现有 `status = "published"`）。
- **私密（private）**：只有作者本人能看到；他人访问返回 404、作者首页 / 列表对他人隐藏。

发布时选一次；内容管理里随时可改。彻底去掉"草稿"状态与"定时发布"（及其后台调度器）。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为作者，我要在写文章页只面对"公开 / 私密"一个开关，默认公开。
  - **AC-1.1**：WHEN 打开新建页 THEN 可见范围默认为「公开」，且只显示 公开/私密 两个选项（无草稿、无定时输入框）
  - **AC-1.2**：WHEN 选中「私密」THEN 提示"只有你自己能看到，其他人访问会显示 404"
- **US-2**：作为作者，我要在内容管理里一眼看出每篇的可见性，并随手切换。
  - **AC-2.1**：WHEN 内容管理列表渲染每行 THEN 公开显示绿色标签、私密显示橙色标签
  - **AC-2.2**：WHEN 点击切换按钮 THEN 乐观更新行内标签并调用 `updatePost` 持久化，**不弹确认框**，操作可撤销（短窗口内出现"撤销"）
- **US-3**：作为访客，我只能看到公开内容。
  - **AC-3.1**：WHEN 未登录读者请求 `/api/posts` THEN 列表不含任何私密文章
  - **AC-3.2**：WHEN 非作者读者请求某私密文 `/api/posts/:id` THEN 返回 404（与不存在的文章一致）
  - **AC-3.3**：WHEN 查看作者公开主页 `/u/:username` THEN 仅该作者的公开文章（GetAuthor 已有此行为）
- **US-4**：作为作者，旧的定时 / 草稿概念不应再出现。
  - **AC-4.1**：WHEN 客户端提交 `status = "draft" | "scheduled"` THEN 后端一律落回公开（白名单兜底），不报错、不产生隐藏/定时态
  - **AC-4.2**：WHEN 系统运行 THEN 无后台定时发布调度器（`PublishDuePosts` 已删除，`main.go` 不再起每分钟 ticker）

## 3. 前端

- **组件树**：`PostEditor`（`ui/PostEditor.tsx`）元面板；`PersonalCenterPage > ContentManage`（`pages/PersonalCenterPage.tsx`）列表。
- **关键交互**：
  - 元面板"可见范围"单选卡（公开=绿点 / 私密=橙点），默认公开。
  - 列表行：可见性 badge（`badge--published` 绿 / `badge--private` 橙）+ 切换按钮 + 撤销条。
  - 阅读页 / 列表页：仅私密时显示橙色"私密"徽标（公开不再显式徽标，避免噪音）。
- **i18n 键**：`editor.visibility*`、`list.private`、`list.setPublic`、`list.setPrivate`、`list.visibilityUpdated`、`list.visibilityUndo`。
- **类型**：`PostStatus = 'published' | 'private'`（`core/types.ts`，移除 `draft`/`scheduled`）。

## 4. 后端

- **Service 方法**：
  - `CreatePost`：可见范围白名单 `""/"published"/"private"`，未知值（含遗留 draft/scheduled）一律落回 `published`。
  - `UpdatePost`：仅接受 `published`/`private`，空值保留原可见性（部分编辑）；`post.ScheduledAt = nil`。
  - `ListPostsPage`：非管理员额外 `WHERE posts.status <> 'private' OR posts.author_id = ?`，他者私密行不进列表。
  - `GetPostPublic` / `GetAuthor`：维持"仅返回 published"，私密对访客 404。
  - **删除 `PublishDuePosts`**（定时发布调度方法）。
- **数据模型**：复用 `posts.status` 列，无需迁移（值域收缩为 published/private）。
- **错误处理**：404（访客看私密/不存在）、403（非作者改他人文章）不变。

## 5. 数据流

```
写文章页 选可见范围(默认公开) → 发布按钮(新建"发布"/编辑"更新") → CreatePost/UpdatePost(白名单) → status 入库
内容管理 点切换 → 乐观翻 badge → updatePost(status=新值) → 成功保留 / 失败回滚 → 出现"撤销"(6s) → 撤销再调 updatePost(旧值)
访客访问 私密文 → GetPostPublic/GetAuthor/ListPostsPage 过滤 → 404 / 不出现
```

## 5.5 性能与资源

- 前端包体积：净增 ~1 kB JS（可见性单选 + 列表切换逻辑），CSS 数百字节；对 131kB 基线影响可忽略。
- 后端：列表查询多一条 OR 过滤（索引覆盖），规模不变；删除每分钟 ticker，CPU 略降。
- 无新增常驻结构、无上传/备份变化。

## 6. 边界情况

- **空数据**：内容管理空态不变。
- **遗留数据**：库中若有历史 `draft`/`scheduled` 行——`ListPostsPage` 对其作者本人仍可见（非 private），对外部访客被 `status='published'` 门槛挡住；新写入不再产生这两种值。
- **并发切换**：乐观更新 + 服务端最终一致；重复点以最后一次 `updatePost` 为准。
- **撤销窗口**：6s 内未撤销则提示消失；撤销按钮只在最近一次切换可撤销时出现。

## 7. 已知限制 / 未来方向

- `ScheduledAt` 列与前端 `scheduled_at` 字段仍保留在 DTO（向后兼容旧响应），但不再被任何 UI 写入；后续可整体裁剪。
- 若未来要恢复"定时"，需新增 `scheduled` 值域 + 后台调度器，并重新引入 UI 时间选择。

## 8. 相关

- 依赖 spec：`specs/19-write-editor.md`（元面板）、`specs/18-personal-center.md`（内容管理）。
- 验证记录（2026-09-22）：`go build ./...`、`go vet`、`go test ./...` 全绿（services 4.6s）；前端 `tsc --noEmit` 与 `vite build` 全绿；后端二进制已重建并重启（:8080），Vite 重启（:5173）。
