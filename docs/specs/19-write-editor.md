# 写文章页面（新建 `/create` · 编辑 `/edit/:id`）

> 状态：Draft（本轮重写成标准 spec；代码一期已实现，见「已知限制」）
> 路由：`/create`（新建）、`/edit/:id`（编辑）
> 权限：登录（`Guarded`）
> 轮次：第 21 轮
> 最后更新：2026-09-22
> 取代：`specs/03-editor.md`（早期 textarea 双面板，已被本 spec 的三模式 Vditor 方案取代）
> 注：可见范围（公开/私密）与"去掉草稿/定时"见 `specs/20-visibility.md`（第 22 轮），元面板由"状态下拉"改为"可见范围单选"，发布/更新按钮文案随新建/编辑切换；`localStorage` 自动保存仍保留（属浏览器本地兜底，非"草稿状态"）。

## 1. 目标

写文章页面是 Echo 的核心创作界面，服务于**个人作者**，架构上预留多作者。设计原则：**让作者专注于“写”，而不是“操作编辑器”**。工具与选项“需要时出现，不需要时安静消失”，编辑器主体永远占据最大视觉权重。

一期实现（PC 优先、移动端结构预留）覆盖：三段式布局、三模式编辑器（分屏预览 / 即时渲染 / 纯源码）、完整工具栏、全部核心快捷键（含中文输入法让位）、三种图片上传（拖拽 / 粘贴 / 点击）、自动保存草稿、字数与阅读时长、浮动目录、元信息面板（分类 / 标签 / 状态 / 置顶 / 封面）、发布流程与反馈、移动端结构预留。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为作者，我要用最舒服的方式写 Markdown。
  - **AC-1.1**：WHEN 选择「分屏预览」THEN 左编辑右实时渲染，分割线可拖动，左右滚动双向同步
  - **AC-1.2**：WHEN 选择「即时渲染」THEN 编辑区即渲染区，光标离开行时源码标记隐藏、回到行时重现（Typora 风格）
  - **AC-1.3**：WHEN 选择「纯源码」THEN 仅显示 Markdown 源码；三模式切换内容不丢失、光标尽量保持
- **US-2**：作为作者，我要丰富的格式工具。
  - **AC-2.1**：WHEN 看工具栏 THEN 分组展示基础格式 / 标题 H1–H6 / 列表（含任务列表）/ 插入（链接·图片·表格·代码块·公式·Mermaid·脚注）/ 高级（TOC·折叠块·提示卡片）
  - **AC-2.2**：WHEN 悬停工具项 THEN 显示名称与快捷键提示
  - **AC-2.3**：WHEN 点击任务列表复选框 THEN 直接切换 `- [ ]` / `- [x]` 状态
- **US-3**：作为作者，我要熟练的快捷键。
  - **AC-3.1**：WHEN 按 Ctrl/⌘+B/I/E、⌘+K、⌘+⇧+I/K/U/O/C、⌘+⌥+1–6、⌘+Z/Y/F/S/Enter THEN 触发对应加粗/斜体/行内代码/链接/图片/代码块/列表/标题/撤销/重做/查找/保存/发布
  - **AC-3.2**：WHEN 处于中文拼音组合输入期间 THEN 组合键让位输入法，不误触格式
- **US-4**：作为作者，我要顺手地上传图片。
  - **AC-4.1**：WHEN 从文件管理器拖拽图片到编辑区 THEN 自动上传并插入 `![alt](url)`，拖拽时编辑区有高亮反馈
  - **AC-4.2**：WHEN 截图后 ⌘+V 粘贴 THEN 识别剪贴板图片并上传插入
  - **AC-4.3**：WHEN 点击工具栏图片按钮或占位区 THEN 打开本地选择器上传
  - **AC-4.4**：WHEN 上传中 THEN 插入位置显示占位（半透明轮廓 / 加载指示）；失败显示错误 + 重试，不阻断写作
- **US-5**：作为作者，我不怕丢内容。
  - **AC-5.1**：WHEN 停止输入 2–3 秒 THEN 防抖自动保存到 `localStorage`（标题 / 正文 / 分类 / 标签 / 状态 / 置顶 / 定时 / 封面）
  - **AC-5.2**：WHEN 挂载检测到本地未同步草稿 THEN 顶部提示「检测到未保存的草稿，是否恢复？」由作者决定
  - **AC-5.3**：WHEN 显式点「保存草稿」(⌘+S) THEN 立即保存并显示「已保存」/最后保存时间
- **US-6**：作为作者，我要实时写作指标。
  - **AC-6.1**：WHEN 正文变化 THEN 底部状态条显示字数（中英分别计）与预计阅读时长（300 字/分）
  - **AC-6.2**：WHEN 文章含多级标题 THEN 浮动目录随标题自动更新，点击跳转章节
- **US-7**：作为作者，我要完整的发布选项。
  - **AC-7.1**：WHEN 填元信息 THEN 可选分类（不存在则建新）、多标签（自动补全、可删块）、状态（草稿/已发布/定时）、置顶开关、封面图（本地上传或正文图片选）
  - **AC-7.2**：WHEN 标题为空 THEN 发布被阻止并提示
- **US-8**：作为作者，我要清晰的发布反馈。
  - **AC-8.1**：WHEN 点发布 THEN 前端校验（标题/正文/分类/定时时间）→ 通过进加载「发布中…」→ 成功提示 + 「查看文章」「返回编辑」
  - **AC-8.2**：WHEN 发布失败 THEN 显示原因 + 重试，内容不丢失
  - **AC-8.3**：WHEN 状态为草稿 THEN 主按钮文「保存草稿」；已发布编辑时文「更新」

## 3. 前端

- **布局（2026-09-22 修订：页面控制编辑器高度，三段式分行）**：

  核心问题：早期实现「标题 / 模式切换 / 发布按钮」全挤在顶栏一行，编辑器高度由内容撑开，导致下方与右侧都空、比例失衡。修订后按「页面控制编辑器，而非内容撑开页面」原则重排：

  1. 整体骨架：顶部导航固定 64px → 内容容器 `max-width: 1440px` 居中、左右 24px → 主区两列（左编辑器 `1fr`、右发布选项固定宽）→ 编辑器卡片内部「工具栏 / 编辑·预览主体（内部滚动）/ 底部状态栏 48px」。
  2. 关键 CSS：`html, body, #app, .layout--editor` 高度 100%；`main` `flex:1; min-height:0; overflow:hidden`；`.post-editor` `flex:1; min-height:0`；编辑器卡片 `flex:1` 占满剩余高度，编辑/预览/状态栏各自内部滚动，页面本身不出现纵向滚动条。
  3. 顶栏分行（对应方案 §三）：
     - 第一行：页面标题「写文章」（左）＋「发布」按钮（右，`margin-left:auto`）
     - 第二行：文章标题输入框，左列全宽，高 48px
     - 第三行：工具栏（左侧格式按钮 `flex:1`）＋ 模式切换（右侧，`margin-left:auto`）
     - 第四行：编辑器主体 ＋ 右侧发布选项（同 Grid 行）
  4. 右侧发布选项：固定宽（大屏 360 / 中屏 320 / 小屏 PC 280），`align-self:start` 顶部对齐、高度由内容决定，不拉满。
  5. 分屏比例：默认 50/50，分割线可拖 20%–80%（源码侧通常更长，可偏 55/45）；即时渲染 / 纯源码模式只显示单一编辑区、占满主体宽度。
  6. 断点：≥1440 右侧 360；1200–1439 右侧 320；1024–1199 右侧 280、工具栏允许换行；<1024 右侧掉到下方、编辑器全宽；<768 单栏、编辑/预览按钮切换（移动端结构预留，不实现交互）。
  7. 编辑路由专用：`Layout` 在 `/create`、`/edit/*` 加 `.layout--editor`，隐藏页脚、锁定视口高度，让编辑器卡片触底。

```text
写文章                                    [发布]
[文章标题输入框................................]
[H1 H2 B I ...                        分屏|IR|源码]
[编辑区            | 预览区]            [发布选项侧栏]
[字数 阅读时长      已保存 21:08        取消]
```

- **组件树**：`Layout > (CreatePage | EditPage) > PostEditor(Vditor) > (pe-topbar / pe-grid[pe-editor + pe-meta] / pe-statline)`
- **编辑器选型**：**Vditor**（`npm i vditor`）。理由：原生三模式（split/ir/sv）切换无感，覆盖 AC-1；36+ 工具栏项可裁剪；支持拖拽 + 剪贴板粘贴上传 + 实时进度；基于 TS、不绑定框架，以原生实例挂入 Preact（无需 wrapper）。备选 Bytemd（更轻但 IR 弱）、TUI Editor（体积大、MD 控制弱）。
- **关键交互**：
  - 三模式用 Vditor `mode: 'sv' | 'ir' | 'wysiwyg'`（分屏用 `vditor` 的 `cache`+`toolbar`；分屏预览以 `preview.render` 并排渲染，自管滚动同步）
  - 图片上传走自定义 `upload.handler`：复用 `uploadImage(file)`（服务端 `/api/upload`，带 JWT），返回 `{code:0, data:{succMap:{name:url}}}`；拖拽/粘贴由 Vditor 接管
  - 自动保存防抖 2.5s，存 `localStorage` 键 `echo.draft[.id]`（沿用 decision 0002 的 TTL 思路，扩展存元信息）
  - 字数统计用 `wordCount`（md.ts，已排除语法标记），阅读时长 `max(1, round(n/300))` 分钟
  - 目录用 Vditor `outline` 或自定义 `extractTOC`（md.ts）浮动渲染
  - 发布按钮文案随状态切换：`draft`→保存草稿、`published/scheduled`→发布/更新
- **路由**：`/create`、`/edit/:id`
- **i18n 键**：`editor.*`（含新增 `modeSplit/modeIR/modeSource/cover/coverUpload/coverFromBody/coverClear/restoreDraft/restoreDraftConfirm/restoreDraftDiscard/draftSaved/savedAt/publishing/publishUpdate/saveDraftBtn/toc/metaPanel`）
- **共享依赖**：`Layout` / `i18n` / `md.ts`（`wordCount`/`extractTOC`）/ `api`（`createPost`/`updatePost`/`listCategories`/`uploadImage`）

## 4. 后端

- **API 端点**：
  - `POST /api/posts` → `Post`（新建，`author_id` 由登录态填；新增 `cover`）
  - `PUT /api/posts/<id>` → `Post`（更新，需本人/admin；`cover` 非空时覆盖）
  - `GET /api/posts/<id>` → 取初始值
  - `POST /api/upload`（multipart）→ 图片 URL（服务端路由本地/SM.MS）
- **数据模型**：`posts` 新增 `cover`（`varchar(512)`，可空）；`AutoMigrate` 自动加列
- **权限**：写需本人/admin；更新前校验归属
- **错误处理**：标题缺失 400；归属不符 403；图片超限 413；定时缺 `scheduled_at` 400

## 5. 数据流

```
输入 → title/body/status/category/tags/pinned/scheduled_at/cover
  → 防抖 2.5s → localStorage 草稿（含元信息）
提交 → createPost/updatePost → /api/posts → svc → SQLite（cover 落库）
  → 成功：删对应 draft 键 + 跳 /read/<id>；失败：错误 toast，内容保留
图片 → Vditor upload.handler → uploadImage → /api/upload → URL → 插入
```

- **可能失败点**：`createPost`/`updatePost` 网络或校验失败（toast，不跳）；图片上传失败（占位 + 重试）
- **回退**：草稿兜底，误关浏览器不丢最近编辑

## 5.5 性能与资源

- **前端包体积**：Vditor 约 +600KB JS（gzip ~180KB）+ CSS ~30KB，一次性加载；编辑器按需仅在 `/create` `/edit` 加载（动态 `import` 可进一步减小首屏）
- **后端**：新建/更新 O(1)；上传落盘 `uploads/` 或走 SM.MS
- **慢点**：超大正文 IR 渲染；Vditor 自带分块/懒渲染缓解
- **是否需重新评估**：极低配机（i3-4160/8GB）浏览器跑 Vditor 流畅；若卡顿可切纯源码默认模式

## 6. 边界情况

- 空标题：阻止提交并高亮
- 草稿恢复：作者确认，不强制
- 图片上传失败：占位 + 重试，不阻断正文
- 封面为空：不更新（保留原封面）
- 定时未选时间：前端 required 阻止 + 后端 400
- 移动端：不依赖 hover，改用点击切换（结构已预留）

## 7. 已知限制 / 未来方向

- 一期已实现：三模式、工具栏、快捷键、拖拽/粘贴/点击上传、自动保存、字数/阅读时长、浮动目录、元信息面板（含封面）、发布流程、移动端结构预留（CSS 钩子，未做独立底部抽屉组件）
- **2026-09-22 布局修订（§3）**：按「页面控制编辑器高度」原则重排——顶栏拆为「页面标题+发布按钮 / 标题输入 / 工具栏+模式切换」三行；编辑器卡片占满视口剩余高度、内部滚动、页面无纵向滚动条；右侧发布选项固定宽（360/320/280）顶部对齐；编辑路由（`/create`、`/edit/*`）锁定视口并隐藏页脚。
- **2026-09-22 高度链修复（§3.2）**：根因是 `.pe-grid` 用了 `align-items:start`，grid 行高由最高列（右侧 meta）决定，meta 比编辑区高时整行被撑高、编辑区拿不到剩余高度，textarea 的 `height:100%` 又塌成 `min-height`。修复：
  1. `.pe-grid` 改 `grid-template-rows:100%`（行高固定为 `.post-editor` 的剩余高度），不再由内容撑开。
  2. `.pe-editor`（左列，编辑器卡片）加 `height:100%; overflow:hidden; min-height:0`，把卡片裁切到行高内；`textarea`/`preview`/`ir` 各自 `height:100%` + 内部 `overflow-y:auto`（`min-height:200px` 去掉，让 grid 拉伸主导）。
  3. `.pe-meta`（右列）`align-self:start` 顶部对齐、高度由内容决定，**不**拉满行高（下方留白是预期行为，不为对齐而拉长）。
  4. 编辑路由下 `.post-editor .pe-grid { height:100% }` 保证 grid 填满 `.post-editor`（flex 列子项）。
  5. 状态栏 `.pe-statline` 保持 `flex:0 0 48px` 固定在卡片底部，`flex-shrink:0` 防压缩。
  6. <1024 窄屏：grid 退化为 `auto auto` 行（meta 掉到下方），`main--editor` 改 `overflow:auto` 允许页面纵向滚动（移动端结构预留，不实现交互）。
- 未做：移动端独立底部抽屉交互、封面「从正文图片选择」的拾取 UI（当前为本地上传）、Mermaid/数学公式依赖 Vditor 内置渲染（需确保 CDN/离线资源）
- 封面图仅本地上传路径；SM.MS 封面未单独处理
- 多作者：当前单人，协作权限未启用（架构已预留）

## 8. 相关

- 依赖 spec：`specs/02-read-page.md`（保存后跳转）、`specs/15-profile-author.md`（作者页）、`specs/20-personal-center.md`（内容管理入口）
- 相关决策：`decisions/0002-localstorage-draft-ttl-30d.md`
- 图床双模（本地/SM.MS）见 PROGRESS 第 5 轮；选型建议见本文 §3
