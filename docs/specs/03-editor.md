> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 编辑器（新建 `/create` · 编辑 `/edit/:id`）

> 状态：**已由 `specs/19-write-editor.md` 取代**（第 21 轮）。本 spec 为早期 textarea 双面板方案，三模式 Vditor 方案见 19。
> 路由：`/create`（新建）、`/edit/:id`（编辑）
> 权限：登录（`Guarded`）
> 轮次：第 4、5、14、16、17 轮
> 最后更新：2026-09-21

## 1. 目标

提供 Markdown 双面板编辑器（左侧编辑 / 右侧预览），支持标题、分类、标签、状态、置顶、图片插入；编辑模式自动保存草稿（带 30 天 TTL 卫生），保存后回阅读页。新建 / 编辑共用同一个 `PostEditor` 组件。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为作者，我要写 Markdown 并实时预览。
  - **AC-1.1**：WHEN 点预览按钮 THEN 右面板显示 `renderMD(body)`；点编辑按钮切回
  - **AC-1.2**：WHEN 点工具栏 H1/B/I/代码/链接/图片/引用/列表 THEN 在光标处插入对应包裹，选中文本时包裹选中内容
  - **AC-1.3**：WHEN 按 Ctrl/⌘+B / Ctrl/⌘+I THEN 加粗 / 斜体
- **US-2**：作为作者，我要插入图片。
  - **AC-2.1**：WHEN 上传成功 THEN 光标处插入 `![image](url)`（本地 `/api/upload` 或 SM.MS，双模见 8）
- **US-3**：作为作者，我要发布 / 保存文章。
  - **AC-3.1**：WHEN 标题为空 THEN 提交被阻止
  - **AC-3.2**：WHEN 新建提交成功 THEN 删匿名草稿键 `echo.draft` + 成功 toast + 跳 `/read/<id>`
  - **AC-3.3**：WHEN 编辑提交成功 THEN 删 `echo.draft.<id>` + toast + 跳 `/read/<id>`
- **US-4**：作为作者，我要编辑未提交时不丢内容。
  - **AC-4.1**：WHEN 编辑模式输入停顿 800ms THEN 自动保存草稿到 `localStorage` 并闪「已自动保存」
  - **AC-4.2**：WHEN 重新打开某文章 THEN 恢复对应草稿；若草稿 `ts` 超过 30 天 THEN 丢弃草稿回退服务端版本（decision 0002）
  - **AC-4.3**：WHEN 挂载编辑器 THEN 全量清理超过 30 天的过期 draft 键
  - **AC-4.4**：WHEN 卸载时编辑器为空（未输入）THEN 移除对应 draft 键，避免空草稿累积
- **US-5**：作为作者，我要在写作时看到字数与预估阅读时长（第 16 轮加入）。
  - **AC-5.1**：WHEN 正文变化 THEN 底部信息条实时显示 `字数` 与 `约 N 分钟读完`（复用 `wordCount`，WPM=200，最少 1 分钟）
  - **AC-5.2**：WHEN 正文为空 THEN 字数显示 0，不显示阅读时长（或显示 `-`）
- **US-6**：作为作者，我要定时发布文章（第 17 轮已实现）。
  - **AC-6.1**：WHEN 状态选择「定时」THEN 显示 `datetime-local` 时间选择器
  - **AC-6.2**：WHEN 状态为「定时」但未选时间 THEN 提交被阻止（前端 required + 后端 400）
  - **AC-6.3**：WHEN 提交（新建/编辑）状态为「定时」THEN 保存并携带 `scheduled_at`（UTC ISO8601）；列表与预估显示定时徽标
  - **AC-6.4**：WHEN 每分钟后台调度器运行且某定时文章到点（`scheduled_at <= now`）THEN 自动改为 `published` 并清空 `scheduled_at` / 出现在已发布列表

## 3. 前端

- **布局**（ASCII）：

```
┌──────────────────────────────────────────┐
│ 顶栏（Layout）                           │
│ h1：新建 / 编辑                          │
│ 工具栏：编辑|预览 切换 + 图片上传 + 已自动保存 │
├──────────────────┬───────────────────────┤
│ 编辑面板          │ 预览面板              │
│  标题            │  （renderMD 渲染）    │
│  分类 / 标签     │                       │
│  正文 textarea   │                       │
│  状态 / 定时 / 置顶 │                     │
│  [取消] [发布/保存]│                      │
└──────────────────┴───────────────────────┘
（移动端：两面板上下堆叠）
```

- **组件树**：`Layout > (CreatePage | EditPage) > PostEditor > (editor-pane / preview-pane / ImageUploader / stat-bar / schedule-input)`
- **关键交互**：
  - 编辑 / 预览互斥显示，激活面板加 `editor-pane--active`
  - 自动保存防抖 800ms（`setTimeout`），仅 `id` 存在（编辑模式）才存草稿；`/create` 存匿名键 `echo.draft`
  - 工具栏 `insertAtCursor` 操作光标 + `setTimeout` 重设选区
  - `EditPage` 先 `getPost(id)` 取初始值，404 → toast + 回 `/`
  - 实时统计（第 16 轮）：正文区下方 stat-bar 显示 `wordCount(body)` 与 `max(1, round(count/200))` 分钟，空正文只显字数
  - 定时发布（第 17 轮）：状态选「定时」→ 显示 `datetime-local`；提交时 `scheduled_at` 由本地时间转 UTC ISO（`new Date(v).toISOString()`）；编辑恢复 `initialScheduledAt` 回填输入框；`scheduled_at` 一并写入草稿；未选时间时提示 `editor.scheduleRequired` 并阻止提交
- **路由**：`/create`、`/edit/:id`
- **i18n 键**：`editor.*`、`list.*`、`post.*`
- **共享组件依赖**：`Layout` / `i18n` / `md.ts`（`renderMD`）/ `ImageUploader` / `api`（`createPost`/`updatePost`/`listCategories`）

## 4. 后端

- **API 端点**：
  - `POST /api/posts` → `Post`（新建，`author_id` 由登录态填）
  - `PUT /api/posts/<id>` → `Post`（更新，需本人 / admin）
  - `GET /api/posts/<id>` → 取初始值
  - `POST /api/upload`（multipart）/ SM.MS `POST https://sm.ms/api/v2/upload` → 图片
- **Service 方法**：`svc.CreatePost` / `svc.UpdatePost` / `svc.GetPost` / `svc.PublishDuePosts`（调度器）
- **调度器**：`cmd/server/main.go` 每分钟 `time.Ticker` 调用 `PublishDuePosts`；到点（`scheduled_at <= now` 且 `status='scheduled'`）翻为 `published` 并清空 `scheduled_at`，随服务器关闭
- **数据模型**：`posts` 新增 `scheduled_at`（可空、索引、UTC），`status` 增加取值 `scheduled`；`AutoMigrate` 自动加列
- **权限**：写需本人 / admin；更新前校验归属
- **错误处理**：标题缺失 400；归属不符 403；图片超大小限制 413；`scheduled` 缺 `scheduled_at` 400

## 5. 数据流

```
输入 → title/body/status/scheduled_at ... → 防抖 800ms → localStorage 草稿（编辑模式）
提交（定时）→ 本地时间转 UTC ISO → createPost/updatePost → POST/PUT /api/posts → svc → SQLite
  → 成功：删对应 draft 键 + 跳 /read/<id>；失败：错误 toast
到点 → 每分钟调度器 PublishDuePosts → UPDATE status='published', scheduled_at=NULL
图片 → ImageUploader → /api/upload 或 SM.MS → 返回 URL → insertAtCursor
```

- **可能失败点**：`createPost`/`updatePost` 网络或校验失败（toast，不跳转，保留已输入）
- **回退**：草稿兜底（编辑模式 800ms 自动存），误关浏览器不丢最近编辑

## 5.5 性能与资源

- **前端包体积**：复用 `renderMD`；`ImageUploader` 小；影响 < 3 kB
- **后端查询量级**：新建 / 更新 O(1)；图片上传落盘 `uploads/` 或走 SM.MS 不落本地
- **内存 / 磁盘**：
  - 本地上传占用磁盘（`UPLOAD_DIR`），需 `.env` 控制 `UPLOAD_MAX_BYTES`
  - 草稿在浏览器 localStorage（decision 0002，30 天 TTL 防膨胀）
- **慢点在哪**：超大正文预览每 800ms 全量 `renderMD`（> 5 万字时明显）
- **是否需重新评估**：本地图床磁盘吃紧时切 SM.MS；正文超大时预览防抖 / 分段

## 6. 边界情况

- 空标题：阻止提交
- 草稿过期（> 30 天）：丢弃回退服务端
- 未输入即卸载：清空调错键
- 图片上传失败：toast，不阻塞正文
- SM.MS 未配置 token：回退本地上传
- 定时未选时间：前端 required 阻止 + 后端 400

## 7. 已知限制 / 未来方向

- 草稿仅存当前浏览器，不跨设备同步；候选＝后端 draft 表（大工程）
- 无封面图 / 摘要 / SEO 字段；候选＝文章卡片视觉提升
- 定时发布已实现（第 17 轮：`scheduled_at` + 每分钟调度器）；候选＝发布后回调 / 定时撤销 / 调度粒度细化
- 实时字数 / 预估阅读时长已实现（第 16 轮，复用阅读页 `wordCount`）

## 8. 相关

- 依赖 spec：`specs/02-read-page.md`（保存后跳转目标）
- 相关决策：`decisions/0002-localstorage-draft-ttl-30d.md`
- 图床双模（本地 / SM.MS）见 PROGRESS 第 5 轮说明；SM.MS token 明文存前端为已知接受项（风险 P3）
