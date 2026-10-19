# Echo Content API — 阶段总结与问题记录

> 更新时间：2026-09-22
> 用途：存档各轮次进度、已知风险、待解决问题，供后续接续开发参考。

---

## 一、产品定位

面向低配 VPS（1 vCPU / 1.5 GB RAM / 5 GB 磁盘）的全栈内容管理站点。
追求小而美、低资源消耗、功能深度优先于广度，视觉简约（矩形、无圆角、无阴影、不用 emoji）。

---

## 二、技术栈

| 维度 | 选型 | 说明 |
|------|------|------|
| 后端 | Go 1.22 + net/http 标准库 | 零框架，方法匹配路由 |
| 数据层 | GORM v1.25.7 + SQLite（纯 Go 驱动） | 单二进制，无 C 依赖 |
| 鉴权 | golang-jwt/v5 HS256 | JWT 无状态 |
| 前端 | Preact + Vite 5 + TypeScript | ~3KB 框架，手写 i18n/主题/路由 |
| Markdown | marked@18（同步） | DOMParser 清洗，零沙箱 |
| 依赖总量 | 后端 12 模块，前端 2 NPM 包 | 符合低资源目标 |
| 前端产物 | 133.64 kB（gzip 43.61 kB） | 极低体积 |

---

## 三、完成进度总览

| 轮次 | 范围 | 状态 | 关键成果 |
|------|------|------|---------|
| 第一轮（后端基础） | 模型 + CRUD + 鉴权 | ✅ | User/Post 模型，JWT，Post 全量 CRUD |
| 第二轮（后端增强） | 搜索分页 + 图片上传 + 测试 | ✅ | LIKE 搜索、分页、图片验证、全测试绿 |
| 第三轮（前端骨架） | 全量 SPA | ✅ | 6 页面、i18n、亮暗主题、Toast、MD 渲染、防抖 |
| 第四轮（前端增强） | 编辑器 + 列表增强 | ✅ | 双面板编辑器、自动保存、本地图床、分页防抖 |
| 第五轮（体验打磨） | 图床 + 设置页 | ✅ | SM.MS 双模、设置页、种子 Markdown、摘要优化 |
| 第六轮（界面细节） | 导航/登录/快捷键/统计 | ✅ | active 高亮、密码 show/hide、Ctrl+B/I、字数统计、空状态图标 |
| 第七轮（分类/标签） | 模型 + 后端 + 侧栏布局 | ✅ | Category/Tag 模型、many2many 关联、分类 CRUD、侧栏筛选新布局 |
| 第八轮（置顶/浏览数） | 置顶 + 浏览计数 + 上下篇 | ✅ | pinned 置顶、view_count 增量、prev/next 导航、公开阅读接口 |
| 第九轮（管理后台） | 分类/标签管理 + 导航 | ✅ | 分类/标签管理页（含计数）、Layout 管理导航入口 |
| 第十轮（阅读体验） | 字号/进度条/阅读时长/响应式 | ✅ | 字号四级、阅读进度条、WPM 估算、移动端自适应 |
| 第十一轮（管理 UI + 后端） | 用户/备份/回收站 | ✅ | 用户启用禁用、JSON 备份、软删回收站、JWT 篡改 401 修复 |
| 第十二轮（体验增强） | 搜索高亮/代码复制/批量操作 | ✅ | 搜索结果高亮、代码块复制按钮、用户批量启用禁用 |
| 第十三轮（搜索调研） | 全文搜索评估 + 测试加固 | ✅ | 评估 FTS5（纯 Go 驱动不支持，回退 LIKE）、测试库改 :memory:、LIKE 搜索补测试 |
| 第十四轮（本地存储卫生 + 表格样式） | localStorage TTL / Markdown 表格 | ✅ | draft 30 天 TTL 清理 + 保存成功即回收、宽表格横向滚动 wrapper、斑马纹、tall-table 钩子 |
| 第十五轮（阅读 TOC + 列表懒加载） | 阅读页目录 / 列表页无限滚动 | ✅ | md.ts 纯函数 buildSlugs + extractTOC、ReadPage 可折叠目录、ListPage IntersectionObserver 哨兵、bug 修复 2 处 |
| 第十六轮（归档页 + 阅读/编辑增强） | `/archive` 归档 / 滚动高亮 / 回顶 / 灯箱 / 实时字数 | ✅ | ListArchive 服务 + `GET /api/posts/archive`、ArchivePage 年月分组、TOC 滚动高亮、回到顶部、图片灯箱 + lazy、编辑器字数与阅读时长 |
| 第十七轮（定时发布 + 备份导入 + 清空回收站） | 定时发布 / 备份恢复 / 回收站批量清空 | ✅ | `scheduled_at` + 每分钟调度器、`POST /api/backup/import`（事务替换）、`POST /api/trash/empty` |
| 第十八轮（全量 spec 合规审计 + 补缺） | 12 份 spec 逐条验收核验 / 注册即登录 / 备份下载 | ✅ | 逐 AC 比对发现 2 缺口并修复：`svc.Register` 签发 JWT、`GET /api/backup/download` + Blob 下载；7 份 spec 状态转正 Done |
| 第十九轮（后台重构 + 站点配置 + 多用户准备） | 侧边栏后台 / 站点配置 / 用户CRUD / 登录日志 / 作者页 / 注册开关 | 📝 文档 | 4 项决策确认；重写 05/06/09、新增 14/15/16 共 6 份 spec（均 Draft 待实现） |
| 第二十轮（右上角用户菜单 + 个人中心） | 用户菜单 / 个人中心（我的主页·内容管理·安全中心） | ✅ 已实现 | 新增 17-user-menu / 18-personal-center 两份 spec；`UserMenu` 组件（hover+键盘+a11y+退出确认）、`/me` 个人中心三栏；后端 `GET /api/me/stats` + `ListPostsPage` 排序参数（sort/order） |
| 第二十一轮（写文章页面 · Vditor 三模式） | 三段式布局 / 分屏·IR·源码 / 工具栏·快捷键 / 拖拽粘贴上传 / 自动保存 / 字数·目录 / 封面 / 发布流程 | ✅ 已实现 | 新增 19-write-editor（取代 03）；引入 Vditor 重写 `PostEditor`；后端 `posts.cover` 字段 + `CreatePost/UpdatePost` 透传；i18n/ CSS 加三段式与移动端钩子 |
| 第二十二轮（文章可见范围：公开 / 私密，去掉草稿与定时） | 写文章页可见范围单选（默认公开）/ 内容管理可见性标签 + 内联切换（乐观更新 + 撤销）/ 访客侧仅公开 / 删除定时调度器 | ✅ 已实现 | 新增 20-visibility spec；`PostStatus = 'published' \| 'private'`；后端 `CreatePost/UpdatePost` 白名单（遗留 draft/scheduled 落回公开）、`ListPostsPage` 非作者排除他者私密、删除 `PublishDuePosts` 与 `main.go` 每分钟 ticker；i18n 增 `editor.visibility*` / `list.private` / `list.visibilityUndo`；CSS 增 `badge--private` / `post-status-private` / `.pe-visibility` |
| 第二十三轮（登录限流账号桶加固 · HANDOVER §7.2 收尾） | login_limiter identifier 提取 / 补单测 / 文档校正 | ✅ 已实现 | 核实账号级失败锁定当前即生效（`LoginInput.email` 承载 username-or-email、前端同字段发出）；限流器 identifier 加固为 `email` 优先、回退 `username`，防未来独立 `username` 字段登录致 key 取空；新增 `TestLoginLimiterUsernameFieldFallback`；校正 HANDOVER §7.2/§8 表、MEMORY、PROJECT_OVERVIEW 表述；`go build/vet/test ./...` 全绿 |
| 第二十四轮（容器化部署 · Docker Compose 生产化第一步） | deploy/docker 套件 / 根 .dockerignore / systemd 版补 /uploads 反代 | ✅ 已实现 | 多阶段 `Dockerfile.api`（golang→distroless 静态、非 root、/data 卷）+ `Dockerfile.web`（node build→nginx 托管 dist）+ compose 编排 + compose 版 `nginx.conf`（upstream api:8080、SPA 回退、gzip、补 `/uploads/` 反代）+ `api.env` 模板 + README；根 `.dockerignore` 精简 context；不改业务代码。本机验证 `CGO_ENABLED=0 GOOS=linux go build`（11.2MB 静态二进制）+ `vite build` 通过；**未 docker build**（开发机无 Docker） |
| 第二十五轮（三层角色 + 内容审核流 + root 封禁库） | 角色重构 / 审核工作台 / 封禁库 / 层级提权 | ✅ 已实现 | 见 §4.25 |

---

## 四、各轮次改动详情

### 第一轮 — 后端基础

- `internal/models/models.go`：User（BCrypt 密码哈希）、Post（title/body/status/author_id）
- `internal/services/services.go`：Register、Login、GetUser、List/Create/Get/Update/Delete Post（权限控制）
- `internal/handlers/auth.go` + `posts.go`：HTTP 编解码薄层
- `internal/middleware/middleware.go`：JWT 鉴权、CORS、Recover、Logger、内存限流
- `internal/router/router.go`：Go 1.22 ServeMux 路由注册
- `internal/config/config.go`：env 读取 + 种子数据注入

### 第二轮 — 后端增强

- `ListPostsPage` 分页接口（`?status=&q=&page=&size=`）
  - 非 admin 只看自己文章；`q` 走 LEFT JOIN users 命中 title/body/作者名
  - 三参全空时回退旧版裸数组，保持向后兼容
- `UploadImage`：`POST /api/upload` multipart 文件上传
  - 扩展名白名单 + 2MB 双重校验（MaxBytesReader + LimitReader）
  - 随机文件名 `<unixms>-<4hex><ext>`
- `ServeUploads`：`GET /uploads/{file}` 公开回读，防路径穿越
- 配置文件：`data/uploads` 目录 + `UPLOAD_MAX_BYTES`

### 第三轮 — 前端骨架

- `main.tsx`：Provider 嵌套（Theme → I18n → Auth → App）+ wouter 路由表
- `core/router.tsx`：`useNav()` + `AppLink` 封装
- `core/i18n.tsx`：手写 zh/en Context，`t(key, vars)` 支持 `{var}` 插值
- `core/theme.tsx`：light/dark/system 三态，`data-theme` 属性，matchMedia 监听
- `core/app.tsx`：Toast 队列 + confirmDialog
- `core/auth.tsx`：JWT 自愈（启动时 GET /me 复核）
- `core/debounce.ts`：手写 `useDebounce(value, delayMs)` 防抖 hook
- `core/md.ts`：marked@18 同步解析 + DOMParser 清洗（移除 script、过滤非法 src/href）
- `pages/`：LoginPage、RegisterPage、ListPage、CreatePage、EditPage、ReadPage、NotFoundPage
- `ui/`：Layout、PostEditor（双面板编辑器 + 工具栏 + 自动保存）、Toast
- `index.css`：CSS 变量驱动亮暗主题，矩形无阴影，响应式（640px 断点）

### 第四轮 — 前端增强

- `PostEditor` 双面板：编辑/预览切换，自动保存（800ms 防抖 → `echo.draft.<id>`）
- `ImageUploader` 组件：本地图片上传（MIME 白名单 + 2MB 校验 + 状态反馈）
- `ReadPage` 改用 `renderMD(post.body)` 渲染 Markdown
- `ListPage` 切到 `listPostsPaged`，搜索/状态过滤 300ms 防抖，自动重置页码
- `index.css` 补全 `.md-body` 排版规则（标题字号阶梯、代码块、表格、图片）

### 第五轮 — 体验打磨

- `api.ts` 新增 `uploadImageSmms(file, token)` 调用 SM.MS v2 API
- `ImageUploader` 改为双模 tab：本地 / SM.MS（默认由 `localStorage['echo.hosting.token']` 决定）
- `SettingsPage` 新建：路由 `/settings`（需登录），保存图床 token 到 localStorage
- `Layout` 管理员新增「设置」导航入口
- `PostEditor` TOOLBAR 修复 tooltip key 映射 bug（`tooltipKey` 字段替代字符串 hack）
- `config.go` 种子文章正文换为完整 Markdown（标题/列表/代码块/表格/引用/水平线）
- `md.ts` 新增 `stripHTML(html)` 函数
- `ListPage` 摘要从裸字符串截断改为 `stripHTML(renderMD(body)).slice(0,120)`
- `services.go` 修复 GORM scope 污染（COUNT 后重新构建 Find 查询）

### 第六轮 — 界面细节完善

- `i18n.tsx`：清理未使用 key（`showAll`、`perPage`、`hostingSaved`），新增 `post.words`
- `Layout.tsx`：顶部导航 active 路由高亮（`nav-link--active` CSS 类）
- `LoginPage.tsx` / `RegisterPage.tsx`：密码框 SHOW/HIDE 切换按钮
- `PostEditor.tsx`：Ctrl+B（粗体）/ Ctrl+I（斜体）快捷键，工具栏 tooltip 加快捷键提示
- `md.ts`：新增 `wordCount(src)` 函数（支持中英文混合字数统计）
- `ReadPage.tsx`：meta 行追加字数统计
- `ListPage.tsx`：空状态美化（纯 CSS 文档图标 + 文案块）
- `index.css`：新增 `.empty-state`、`.empty-icon`、`.pwd-wrap`、`.nav-link--active` 等样式

### 第七轮 — 分类 / 标签 / 新布局

- `models.go`：新增 `Category`、`Tag`、`PostTag`（`post_tags` 多对多）；`Post` 增 `CategoryID`/`Tags` 关联
- `services.go`：`ListCategories` / `ListTags` / `GetRecentPosts`，分类 CRUD（`Create/Update/DeleteCategory`，删分类时解绑文章）、`ListCategoriesWithCount` / `ListTagsWithCount`
- `handlers/categories.go`：分类/标签全部 REST 端点（公开读 + 管理写，admin 角色在 handler 内校验）
- `ListPostsPage`：新增 `category` / `tag` 过滤参数
- `ui/Sidebar.tsx`：侧栏布局（分类列表 + 标签云 + 最近文章），`ListPage` 改主区 + 侧栏双列结构

### 第八轮 — 置顶 / 浏览数 / 上下篇

- `models.go`：`Post` 增 `Pinned`（置顶）、`ViewCount`（浏览数）字段
- `services.go`：`ListPostsPage` 置顶优先排序（pinned desc → created desc）；`GetPostPublic` 仅返回已发布并 `view_count + 1`；`GetPostPrevNext` 上一篇/下一篇
- `handlers/posts.go`：`GET /api/posts/{id}/nav`、`GET /api/posts/{id}/public` 路由
- `ReadPage.tsx`：置顶徽标、浏览数展示、上/下一篇导航条；编辑器新增「置顶」开关
- `router.go`：`/` 精确路径 + 未知路径 404 兜底（避免 `GET /` 子树吞掉 `/favicon.ico`）

### 第九轮 — 管理后台（分类 / 标签）

- `pages/AdminCategoriesPage.tsx`：分类 CRUD（含每分类文章数统计）
- `pages/AdminTagsPage.tsx`：标签管理（含使用计数、删除标签及关联）
- `Layout.tsx`：为 admin 角色增加「管理」导航分组，进入分类/标签管理页

### 第十轮 — 阅读体验增强

- `ReadPage.tsx`：字号四级（small/medium/large/xlarge，`localStorage['echo.readFont']` 记忆）；阅读进度条（滚动 → 0..100%）；WPM=200 估算「约 N 分钟读完」
- `types.ts`：新增 `FontSize` 类型
- `index.css`：`.read-progress` 进度条、`.font-controls` 字号切换组、`--small/--medium/--large/--xlarge` 字号变量
- 移动端（640px 断点）：侧栏收起、字号控件换行、meta 自适应

### 第十一轮 — 管理 UI + 后端（用户 / 备份 / 回收站）

- `models.go`：`User.Disabled`（管理员可禁用账号）、`Post.DeletedAt`（GORM 软删除，`Unscoped()` 查询/恢复）
- `services.go`：`ListUsers`、`SetUserDisabled`、`ExportBackup`（全库 JSON 快照，含已删文章）、`ListTrash`（7 天回收窗口内软删文章）、`RestoreTrash`
- `handlers/admin.go`：`GET /api/users`、`PUT /api/users/{id}/disabled`、`POST /api/backup`、`GET /api/trash`、`POST /api/trash/{id}/restore`，全部经 `requireAdmin` 校验
- 前端：`AdminUsersPage`（启用/禁用）、`AdminBackupPage`（导出 + 统计）、`AdminTrashPage`（一键恢复）
- **修复**：`services.go` `parseToken` 对篡改签名的 JWT 返回 `err==nil`（仅 `t.Valid==false`），导致 `Authenticate` 走不到 401 分支而 500；补 `if err==nil { err = ... }` 守卫（`TestAuthenticateRejectsTamperedToken` 由红转绿）

### 第十二轮 — 体验增强（本轮）

- `md.ts`：新增 `escapeHTML(s)` 与 `highlight(text, query)`（HTML 转义后用 `<mark class="hl">` 包裹匹配词，大小写不敏感）
- `ListPage.tsx`：搜索词防抖命中后，文章标题与摘要经 `dangerouslySetInnerHTML` 实时高亮
- `ReadPage.tsx`：进入文章后为每个 `pre` 代码块注入复制按钮（`navigator.clipboard` + `execCommand` 降级，1.5s 后还原），组件卸载时清理
- `api.ts`：`batchSetUserDisabled(ids, disabled)` 用 `Promise.allSettled` 扇出逐条调用，返回 `{ ok, fail }` 以支持部分失败提示（后端无批量端点，前端兼容）
- `AdminUsersPage.tsx`：新增多选列 + 全选 + 批量启用/禁用操作条；`useEffect` 在列表变化时裁剪失效选择；结果经 `batchDone` / `batchFailed` 反馈
- `i18n.tsx`：补 `admin.selectAll/selectedCount/batchEnable/batchDisable/batchConfirmEnable/batchConfirmDisable/batchDone/batchFailed` 与 `read.copyCode/codeCopied`（zh/en 同步）
- `index.css`：`mark.hl` 高亮、`.code-copy*` 复制按钮、`.admin-batchbar` / `.row-selected` 批量操作条与选中行样式（延续矩形无阴影风格）

### 第十三轮 — 全文搜索调研 + 测试加固（本轮）

- **结论：FTS5 不可行**。当前 SQLite 驱动为 `github.com/glebarez/sqlite`（纯 Go → `modernc.org/sqlite`，无 cgo），不内置 FTS5 扩展；实测 `CREATE VIRTUAL TABLE ... USING fts5(..., contentless=1)` 报 `unrecognized option: "contentless"`。已放弃 FTS5 方案，保留 `ListPostsPage` 的 LIKE 搜索（`title / body / 作者名`）。
- `services.go`：`ListPostsPage` 搜索分支保留 LIKE，注释记录「纯 Go 驱动无 FTS5，千级规模内 LIKE 可接受；若驱动支持 FTS5 可平滑替换」。
- `services_test.go`：`newTestService` 改用 `sqlite.Open(":memory:")`，规避 Windows 下 glebarez 持有临时文件句柄导致 `t.TempDir()` 清理失败的问题（`file in use`）。
- `fts_test.go`：补 LIKE 搜索行为测试——`TestSearchHitByTitleBodyAndAuthor`（正文 / 标题 / 作者名命中）与 `TestSearchSoftDeleteIsolation`（软删隔离 + 恢复回显）。

### 第十四轮 — localStorage 卫生 + Markdown 表格样式（本轮）

- **P1 localStorage TTL**：`PostEditor.tsx` 引入 `DRAFT_TTL_DAYS=30` 常量与 `pruneOldDrafts()` 扫描函数；挂载时执行全量清理；恢复 draft 时检查 `ts` 字段，过期即丢弃并回退服务端数据；卸载空编辑器状态时移除当前 draft 键。
- `CreatePage.tsx`：`createPost` 成功后移除 `echo.draft`（匿名键），保证 `/create` 始终从空白起步。
- `EditPage.tsx`：`updatePost` 成功后移除 `echo.draft.<id>`，避免再次打开时覆盖已保存版本。
- **P2 Markdown 表格样式**：
  - `md.ts`：`renderMD` 在 sanitize 阶段给每个 `<table>` 包一层 `<div class="table-scroll">`，宽表格横向滚动不破版。
  - `index.css`：`.md-body .table-scroll { overflow-x: auto }`、斑马纹 `tbody tr:nth-child(even)`、预留 `table.tall-table` 高度上限钩子（`max-height: 60vh`）。

### 第十五轮 — 阅读页 TOC + 列表页懒加载（本轮）

按「先改文档再改代码」工作流，把 spec 01 / 02 中记录的"未来方向"两项提升为正式需求并实现：

- **文档**：`docs/specs/01-list-page.md`（US-3 懒加载）、`docs/specs/02-read-page.md`（TOC）转正并加 EARS 验收标准；两份 spec 状态改 Done。
- **阅读页 TOC**：
  - `md.ts`：新增纯函数 `buildSlugs(texts[])`（无模块级可变状态，确定性 slug，重名追加 -2/-3），`extractTOC(src)` 解析 `##`/`###` 标题（跳过代码围栏）返回 `TOCEntry[]`，`renderMD` 给 h2/h3 写 id。
  - `ReadPage.tsx`：渲染可折叠 `<nav class="read-toc">`（桌面默认展开、移动端收起），点击 `scrollIntoView({behavior:'smooth'})` 跳转，`scroll-margin-top` 避让 sticky 顶栏。
  - `i18n.tsx` / `index.css`：补 TOC 文案与样式。
- **列表页懒加载**：
  - `ListPage.tsx`：原生 `IntersectionObserver` 监听底部哨兵（`rootMargin:'200px'`），触达自动追加下一页；`load(q,...,append)` 支持追加模式（pageData.items 拼接）；筛选变化重置 page=1；保留 US-2 分页按钮（上一页/下一页），nextPage 接 `loadNext`。
  - 防抖守卫 `loadNext` 用 `loading || loadingMore || error` 拦截 + `hasMoreRef`（无更多数据即停），避免哨兵持续可见时 page 失控累加。
  - `i18n.tsx` / `index.css`：补 `loadingMore`/`noMore`/`loadMore` 文案与 `.lazy-load*` 样式。
- **修复 2 个隐藏 bug**：
  1. IntersectionObserver 在哨兵持续可见（offscreen / 内容不足一屏）时无限累加 page（实测显示 `第 14 / 1 页`）→ 加 `hasMoreRef` 守卫。
  2. `renderMD` 与 `extractTOC` 共享模块级 `seen` Set，`uniqueSlug` 有副作用（每次调用 mutate Set），执行顺序不同导致锚点 id 与 TOC href 错位（h2 id=`为什么选择-preact-7`，href=`#为什么选择-preact-6`）→ 重构为纯函数 `buildSlugs`，每次调用独立且确定性。
- **验证**：`tsc --noEmit` ✅、`vite build` ✅（JS 133.64 kB）、浏览器冒烟——列表页分页 `1/1` + 哨兵"没有更多了"、阅读页 TOC `allMatch: true` 锚点全部命中。

### 第十六轮 — 归档页 + 阅读 / 编辑增强（本轮）

> 轮次编号说明：`docs/specs/01|02|03` 此前把"TOC + 懒加载"标为第 16 轮、把本轮标为第 17 轮，
> 与本文（时间线为准）相差 1。本轮已把三份 spec 的轮次号统一回退一位与 PROGRESS 对齐
> （spec 16→15、17→16），后续不再有偏差。

**1. 归档页 `/archive`（spec 01 US-5）**

- `services.go`：`ListArchive(ctx, user)` —— 全部 `published` 文章，`Preload` 作者/分类/标签，
  `Order(created_at DESC)`；非 admin 只返回本人文章；返回前把 `Body` 清空，保证归档响应轻量。
- `handlers/posts.go`：`ListArchive` handler（薄层，只做 `userFromContext` + `writeJSON`）。
- `router.go`：`GET /api/posts/archive`（`withAuth`）；字面量路径优先于 `/api/posts/{id}` 通配，
  与既有 `/api/posts/recent` 同一套路，无路由冲突。
- `services_test.go`：`TestListArchive` 覆盖"非 admin 只看自己 / admin 看全部 / 草稿不进归档"。
- `api.ts`：`listArchive()` → `request<Post[]>('/api/posts/archive')`。
- `pages/ArchivePage.tsx`（新建）：挂载拉全量 → 按 `created_at` 的 `YYYY-MM` 分组（组内时间倒序），
  组标题 `2026-09` + 条目日期 + 标题，点条目跳 `/read/<id>`；空态区分"已登录但无已发布文章 /
  未登录"；401 → 跳登录，其他错误 → toast。
- `main.tsx`：`/archive` 路由（`Guarded`）；`Layout.tsx`：顶栏新增「归档」入口（active 高亮）。
- `i18n.tsx`：新增 `archive.*`（`title/empty/emptyLoggedOut/loadFailed`）与 `nav.archive`，zh/en 齐备。
- `index.css`：复用既有 `.archive-*` 矩形无阴影样式（组标题 2px 主色下划线、`hover` 反色）。

**2. 阅读页增强（spec 02 US-6/7/8，上一轮次已落地代码，本轮补齐校验）**

- 滚动高亮：同一 `scroll` 监听里算 progress / `showTop` / `activeHeading`（rAF 节流），
  取"最后一个越过视口顶部（阈值 80px）"的 h2/h3 → TOC 项加 `.read-toc-item--active`。
- 回到顶部：`scrollTop > 400` 显示右下悬浮按钮，`scrollTo({top:0,behavior:'smooth'})`。
- 图片灯箱：正文 `img` 命令式加 `loading="lazy"`（首图除外）+ `.read-zoomable`，
  点击开遮罩原图；Esc / 点遮罩关闭，打开时锁 `body` 滚动。

**3. 编辑器实时统计（spec 03 US-5）**

- `PostEditor.tsx`：正文区下方 `.editor-statbar` 实时显示 `wordCount(body)` 字数与
  `max(1, round(n/200))` 分钟阅读时长；空正文只显字数。
- `i18n.tsx`：`editor.stats` / `editor.readTime`（zh/en）。

**4. 修复**

- `ArchivePage` 首版把日期渲染成 `new Date(`${g.month}-01`)` —— 组内每条都显示 `01`。
  改为用 `p.created_at` 取真实日号。
- 首版引用了不存在的 i18n key `archive.loadFailed`（会原样吐 key），已补齐 zh/en 文案。

### 第十七轮 — 定时发布 + 备份导入 + 清空回收站（本轮）

> 本轮同时把 spec 03「定时发布」从「候选」转正，并补全 spec 10「备份导入」/ spec 11「清空回收站」，
> 三份 spec 均已置 Done。

**1. 定时发布（spec 03 US-6，之前会话已落地代码，本轮补齐文档）**

- `models.go`：`Post.ScheduledAt *time.Time`（`gorm:"index"`，可空）
- `services.go`：`CreatePost`/`UpdatePost` 校验 `status=="scheduled"` 必须有 `scheduled_at`（400）；
  `PublishDuePosts` 每分钟把 `status='scheduled' AND scheduled_at<=now` 转 `published`（清空 scheduled_at）
- `cmd/server/main.go`：后台 goroutine + `time.Ticker`（每分钟）调 `PublishDuePosts`，随 `ctx.Done()` 退出
- 前端：`types.ts` 增 `scheduled` 状态 + `scheduled_at`；`PostEditor` 增定时字段（datetime-local→UTC ISO）；
  `EditPage`/`ListPage`/`ReadPage` 徽标补 scheduled；`ListPage` 状态筛选补 scheduled
- i18n：`list.scheduled`、`editor.scheduleLabel/Hint/Required` 等

**2. 备份导入（spec 10 US-2）**

- `services.go`：`ExportBackup` 增强——补导出 `password_hashes` map（`models.User.PasswordHash` 标 `json:"-"`，依赖此 map 复原）与 `post_tags` 关联表，保证导入往返完整
- `services.go`：新增 `ImportData` + `svc.ImportBackup`——事务内先按依赖顺序清空（post_tags→posts→tags→categories→users），再逐条 Create（Posts 用 `Unscoped().Create` 保留 deleted 状态；Users 回填密码哈希）；返回 `BackupStats`
- `handlers/admin.go`：`ImportBackup` handler（body 解码失败 400 "invalid backup JSON"）
- `router.go`：`POST /api/backup/import`（withAuth）
- 前端：`api.importBackup(data)`；`AdminBackupPage` 增「导入备份」区块（file input → JSON.parse → confirmDialog → import → 刷新统计 & 清空 input）；`backup-import*` CSS
- i18n：`admin.importTitle/importSub/importBtn/importBusy/importSuccess/importFailed/importConfirm/importNoFile`

**3. 清空回收站（spec 11 US-2）**

- `services.go`：`EmptyTrash`——`Unscoped().Where("deleted_at IS NOT NULL").Delete(&Post{})` 永久删除全部软删文章
- `handlers/admin.go`：`EmptyTrash` handler（返回 `{ deleted: N }`）
- `router.go`：`POST /api/trash/empty`
- 前端：`api.emptyTrash()`；`AdminTrashPage` 头部「清空回收站」（admin-head，非空才显示；confirmDialog 确认）→ toast（含删除数）→ load()
- i18n：`admin.emptyTrashBtn/emptyTrashConfirm/emptyTrashDone`

**4. 补测与修 bug（Go 工具链可用后复跑）**

- **修 `UpdatePost` 定时字段回归**：原实现为 `if post.Status == "scheduled" && in.ScheduledAt == nil` → 400，
  导致「已定时文章只改标题」这类局部编辑必然失败（客户端未重发时间戳）。改为
  `in.Status == "scheduled" && in.ScheduledAt == nil && post.ScheduledAt == nil`（仅由发布态切定时态且未带时间才算非法），
  并把「离开定时态」的陈旧时间戳清空（`else if post.Status != "scheduled" { post.ScheduledAt = nil }`）。
- **新增 4 个服务层测试**（`services_extra_test.go`）：`TestCreatePostScheduledRequiresTime`、
  `TestUpdatePostScheduledKeepsTimeOnPartialEdit`（上述回归的守卫）、`TestUpdatePostLeavingScheduledClearsTime`、
  `TestPublishDuePosts`（过去的翻 published 并清时间戳、未来的不动、第二轮返回 0）
- **编辑器 UX**：接上此前未使用的 `editor.scheduleLabel` / `editor.scheduleRequired`——
  未选时间时显示红色 `field-hint--warn` 提示而非静默阻止提交；新增 `.field-hint--warn` 样式
- `go build ./...` ✅、`go test ./...` ✅ 全绿（含上述 4 个新测试）
- `tsc --noEmit` ✅、`vite build` ✅（49 模块；JS 141.17 kB / gzip 46.21 kB，CSS 25.17 kB / gzip 5.30 kB）
- **接口冒烟**（临时库起服务）：`scheduled` 不带 `scheduled_at` → 400；创建 `2027-01-01` 定时文章 → `status=scheduled`
  且时间戳入库；仅改标题 → 仍 `scheduled` 且时间戳保留；创建 5 分钟前到期的文章 → 等一个 ticker 周期后
  自动变 `published`、`scheduled_at` 清空（调度器端到端生效）

---

### 第十八轮 — 全量 spec 合规审计 + 补缺（本轮）

> 起因：用户问「功能都按文档实现了吗」。做了一次**逐条验收比对**（不是看 spec 自己标的 Done），
> 把 12 份 spec 的全部 AC grep 到代码上核一遍。

**1. 审计结论**

- 确认合规 10/12：01 列表（含懒加载/高亮/归档）、02 阅读（含 TOC/滚动高亮/回顶/灯箱）、
  03 编辑器（含定时发布）、05 设置、06 管理布局、07 分类、08 标签、09 用户、11 回收站、12 404。
- 查出 2 处不合规，均已修复（详见下文）。
- 另发现文档自相矛盾：`docs/README.md` 索引称 12 份「全部已实现（spec 已写）」，
  但 04/05/06/07/08/09/12 自身仍标 `In Progress` → 已统一转 Done。

**2. 补缺 A：备份浏览器下载（spec 10 AC-1.1）**

原状：`ExportBackup` 把 JSON 写到**服务器磁盘**（`data/backups/`），响应体只有 `{filename,size,stats}`；
`router.go` 无任何备份下载路由；前端全仓搜不到 `Blob` / `createObjectURL` / `a.download`。
**后果：admin 在 UI 上导出拿不到文件，而导入却要求选文件——导出→导入这条往返是断的。**

- `services.go`：新增 `svc.ServeBackup(w, r, name)`——拒绝含 `..` / `/` / `\` 的文件名、
  要求 `filepath.Base(name) == name`、`filepath.Abs` + `filepath.Rel` 复核解析结果未逃出备份目录、
  `os.Stat` 确认存在且非目录；流式回写并设置 `Content-Disposition: attachment`。一律 404（不区分名字非法与不存在，防探测）。
- `handlers/admin.go`：`DownloadBackup`（`requireAdmin` → `svc.ServeBackup`）
- `router.go`：`GET /api/backup/download`（withAuth）
- `api.ts`：`downloadBackup(filename)`（带 Authorization + 保留 ApiError 语义）+ `saveBlob(blob, filename)`
- `AdminBackupPage.doExport()`：导出后按 `filename` 回捞 Blob 并触发另存

**3. 补缺 B：注册即登录（spec 04 AC-2.2）—— 结论修正**

> ⚠️ 首轮审计此处**判断有误**。当时只看到 `api.register()` 返回 `UserPublic`（无 token），就判定"注册后不会自动登录"。
> 实际 `core/auth.tsx` 的 `doRegister` 内部**补调了一次** `doLogin(username, password)`（后端登录支持用户名匹配），
> 功能是通的。教训：判断"某能力缺失"时要追到副作用终点，不能只看单点返回类型。

虽非 broken，但多一次往返且依赖"用户名可当邮箱用"这一巧合。既然审计到这，顺手做正：

- `services.go`：`Register` 返回类型 `models.User` → `*AuthResult`，创建后直接 `SignToken`
- `handlers/auth.go`：`Register` 直接回写 `res`（含 token / expires_at / user）
- `core/auth.tsx`：`doRegister` 用返回的 token 落 `localStorage`，删掉补调 `doLogin`
- `core/api.ts`：`register` 返回类型 `UserPublic` → `AuthResult`
- 测试：`services_test.go` 断言改用 `created.User.*`；新增 `TestRegisterIssuesUsableToken`（注册所得 token 能过 `Authenticate`）

**4. 验证**

- `go build ./...` ✅、`go vet ./...` ✅、`go test ./...` ✅ 全绿（含新增 `TestRegisterIssuesUsableToken`）
- `tsc --noEmit` ✅、`vite build` ✅（49 模块；JS 141.77 kB / gzip 46.40 kB，CSS 25.17 kB / gzip 5.30 kB）
- 接口冒烟：注册返回 207 字符 JWT 且该 token 直接 `GET /api/auth/me` 200（无需再登录）；
  导出得到 `backup-<ts>.json`（5410 B，7 个顶层键）；下载 200 且带
  `Content-Disposition: attachment`；**安防**——`../../echo-smoke.db` / `/etc/passwd` / `..%2F..%2F` / `sub/dir.json` 全 404、
  普通用户 token 403、无 token 401、不存在文件 404

---

### 第二十轮 — 右上角用户菜单 + 个人中心（本轮）

- **需求来源**：用户定稿文档《个人中心与右上角用户菜单需求》（PC 端一期必做；移动端仅预留结构）。
- **拆分文档**：按 `docs/specs/_TEMPLATE.md` 规范拆为两份标准化 spec：
  - `docs/specs/17-user-menu.md`：右上角头像+名称菜单（hover 浮层、点击外部/Esc 关闭、键盘可达、退出二次确认、未登录显「登录/注册」、移动端底部抽屉钩子）
  - `docs/specs/18-personal-center.md`：个人中心 `/me`，左侧三栏（我的主页 / 内容管理 / 安全中心）+ 右侧切换；我的主页信息横栏（头像/名称/总访问量/文章数/注册时间/简介/编辑资料）+ 文章列表（排序 created_at|views × asc|desc、默认发布时间倒序、分页 size=10）；内容管理（状态徽标 + 编辑按钮 + 分页）；安全中心（邮箱脱敏 + 显示/隐藏）；响应式（大屏 15% 留白、移动端顶部标签）
- **前端实现**：
  - `ui/UserMenu.tsx`（新）：触发区 + 浮层；`Layout` 顶栏用其替换原登出按钮，移除仅 admin 可见的「设置」nav-link（账号设置已入菜单）
  - `pages/PersonalCenterPage.tsx`（新）：`/me` 路由（`Guarded`）；i18n 新增 `menu.*` / `center.*`
  - `types.ts` `ListPostsQuery` 加 `sort`/`order`；`api.ts` `listPostsPaged` 透传、`myStats()` 调 `GET /api/me/stats`
- **后端实现**：
  - `services.ListPostsQuery` 加 `SortBy`/`Order`，`ListPostsPage` 白名单应用（`posts.pinned DESC` 置顶 + `created_at|view_count` asc/desc）
  - `services.UserStats` + `handlers.MeStats` + `GET /api/me/stats`（当前用户 `author_id` 文章计数 + `SUM(view_count)`）
- **验证**：前端 `tsc --noEmit` ✅；后端 `go build`/`go vet` 待跑（见 task 36）

---

### 第二十五轮 — 三层角色 + 内容审核流 + root 封禁库（本轮）

- **需求来源**：用户口述设计（角色三级、审核流、root 专属封禁库），核心约束：
  「root 有且只有一个，无法提升任何人为 root，但可把 user 提为 admin；admin 不能提权；
  user 发文需 admin 审核，admin 发文需 root 审核；root 可随时封禁任意 user 的文章；
  被封禁文章仅 root 可见（作者本人与其他 admin 都看不到），在封禁库统一管理，
  可删除或发回原作者修改并备注意见」。
- **模型层**（`internal/models/models.go`）：
  - 角色常量 `RoleRoot="root"` / `RoleAdmin="admin"` / `RoleUser="user"`
  - 文章状态五值 `PostPublished` / `PostPrivate` / `PostPending` / `PostRejected` / `PostBanned`
  - `Post` 新增 `ReviewNote`(text) / `ReviewedBy`(size:36) / `ReviewedAt`(*time.Time)
- **权限集中化**（`internal/services/roles.go` 新）：`IsRoot` / `IsAdminOrRoot` / `canManageUser` / `canReviewPost`，
  避免散落的 `me.Role != "admin"` 字符串比较。
- **服务层**（`services.go` / `accounts.go` / `mail.go`）：
  - `CreatePost` 按角色定初始状态：root 依请求 published/private；admin/user 一律 pending（白名单不可绕过）
  - `canSeePost` 统一可见性：published 所有人；banned **仅 root**；private/pending/rejected 作者+admin+root
  - `ListPostsPage` 四档过滤（root 全见 / admin 排除 banned / user 见 public+本人 restricted 且排除 banned / 匿名仅 published）
  - `UpdatePost`/`DeletePost` 层级编辑权限（普通 user 不能改他人；admin 只能改 user 的；root 全改）
  - `ReviewPost`（root 审 admin/root 发文，admin 只能审 user 发文，reject 必须带 note）、
    `BanPost`（root-only，不能封自己）、`SendBackPost`（banned→rejected + note + 邮件通知）、
    `ListBanned`、`ListPending`、`DeleteBannedPermanently`（Unscoped 硬删）
  - `CreateUser`/`UpdateUser`/`DeleteUser` 角色白名单：root 不可被指派/降级/删除、admin 不能提权、保留 last-admin 保护
  - 新增 `MailReviewResult` 邮件类型（approve/reject 两种正文）
- **迁移**（`config.go` `runManualMigrations`）：
  - gate `0003_root_migration`：最早创建的 admin 升 root，保证 root 唯一（多个 root 则保留最早、其余降为 admin）
  - gate `0004_add_review_columns`：给 posts 幂等补 review_note/reviewed_by/reviewed_at
  - `seedDemoData` 首启 admin 直接建为 root
- **handler 层**：`requireAdmin` 升级为 admin-or-root，新增 `requireRoot`；
  `categories.go` 7 处手写角色判断统一改 `requireAdmin`；
  新文件 `moderation.go` 六个端点（pending/review/ban/banned/send-back/delete-banned）。
- **路由**：`GET /api/admin/pending`、`POST /api/admin/review/{id}`、`POST /api/admin/ban/{id}`、
  `GET /api/admin/banned`、`POST /api/admin/banned/{id}/send-back`、`DELETE /api/admin/banned/{id}`
- **前端**：
  - `types.ts` `Role` 加 `root`；`PostStatus` 五值；`Post` 加 `review_note`/`reviewed_by`/`reviewed_at`
  - `api.ts` 新增 6 个审核/封禁调用
  - `AdminLayout` 新增「内容审核」「封禁库」导航，封禁库 `rootOnly` 对 admin 隐藏；账号区显示 root 徽章
  - 新页面 `AdminReviewPage`（审核工作台：通过/打回+意见弹窗）、`AdminBanVaultPage`（封禁库：发回+永久删除）
  - `AdminUsersPage` 层级化：`canManage()` 判定（root 可管 admin、admin 只管 user、root 行禁改）；
    角色下拉对 admin 隐藏 admin 选项；root 显示只读徽章
  - 后台各页角色门槛放宽到 admin-or-root；编辑器为非 root 显示「需审核」提示；
    个人中心对 pending/rejected/banned 禁用可见性切换并显示状态标签
  - i18n 增 `admin.roleRoot`/`statusLabel.*`/`admin.review*`/`admin.banVault*`/`editor.visibilityReviewHint`（中英）
  - CSS 增 `.post-status-pending|rejected|banned`、`.badge-root`、`.pe-review-hint`
- **测试**：
  - services 重写可见性/编辑权限系列用例（alice/bob/root 三角）
  - handlers 新增 `moderation_test.go` 7 个用例：pending 匿名 403、admin 队列不含 admin 发文、
    reject 必须带 note（400）、ban 需 root（403）、封禁库对 admin 不可见、send-back 出库并带 note、
    delete 为硬删（后续操作 404）
  - `setup` 基建改为返回 `(*services.Service, *gorm.DB)` 以便 fixture 直改角色
- **验证**：`go build`/`go vet`/`go test ./...` ✅ 全绿；`tsc --noEmit` ✅；`vite build` ✅
  （JS 206.28 kB / gzip 63.42 kB，CSS 57.29 kB / gzip 10.76 kB）

---

### 第二十六轮 — AI 协作标准落地：AGENTS.md + AI_STANDARD + 提示词库（本轮）

> 目标：把过去 25 轮 + 3 轮审计中"靠 AI 自觉"的协作经验，固化成**可执行的流程标准**，
> 让任何 AI 协作者（WorkBuddy / Claude Code / Cursor / Codex）进入仓库即按同一套标准干活。

- **新增 `AGENTS.md`（根目录）**：所有 AI 工具的第一份必读。内容 = 项目一句话 / 硬约束表 /
  六大关键陷阱 / 运行与验证命令 / 工作流骨架 / 文档地图。定位"速查 + 红线"，深度内容指向 docs。
- **新增 `docs/AI_STANDARD.md`**：协作唯一标准。核心：
  - 任务四级分类 T0–T3（直接做 / 小改动 / 功能先写 spec / 审计先取证）
  - 五阶段工作流：读上下文 → 定任务 → 实现 → 验证门禁 → 记录沉淀
  - 验证门禁：`go build` / `go vet` / `go test` / `tsc --noEmit` / `vite build` 五命令全绿
    + 行为验证贴证据，缺一不可
  - 十条编码红线 R1–R10（汇总 HANDOVER §7：AutoMigrate 陷阱、root 不可 API 创建、
    WithOptionalAuth、可见性 service 层权威、roles.go 集中、.env 改后重启、路径防逃逸、
    限流双轨、.workbuddy 不删、生产密钥）
  - 上下文工程纪律 + 交付前自查 checklist
- **新增 `docs/prompts/`（5 个场景模板）**：`01-continue`（继续轮次）/ `02-new-feature`
  （spec → plan → 实现）/ `03-bugfix`（复现 → 定位 → 修复 → 回归）/ `04-audit`
  （分级清单 → 修复）/ `05-verify-record`（门禁 + 记录）。用户可直接粘贴驱动 AI。
- **新增 `docs/reference/AI_CODING_REFERENCES.md`**：GitHub 参考项目提炼——spec-kit 的
  specify/clarify/plan/tasks 流程、BMAD 的角色分离理念、AGENTS.md 单一入口形态、
  上下文工程原则；并记录**明确不采用**的（多 agent 框架 / skills 生态 / 生成器）及原因。
- **接线**：`docs/README.md` 增 §2.5 AI 协作体系 + §4 关系表加协作维度；
  `HANDOVER.md` 抬头与 §0 增加 AI 协作者指引。
- **验证**：纯文档轮，无代码改动，跳过编译门禁（按 docs/README.md §3 约定）；
  已核对新文件与既有体系（specs/decisions/PROGRESS/TODO）无编号冲突、无职责重叠。

---

### 第二十七轮 — P2-8 分页双机制修复 + TODO 校准（本轮）

- **P2-8 修复（`web/src/pages/ListPage.tsx`）**：根因比 TODO 描述更严重——翻页 effect 是
  append 架构（`page > 1` 即追加），「上一页」按钮 `setPage(p-1)` 会把第 1 页**重复追加**进
  列表（60 条里 20 条重复，页码显示回退但内容不回退，真 bug）；且传统分页器 / 无限滚动哨兵 /
  「加载更多」按钮三个入口心智模型打架。修法：删传统分页器，统一为无限滚动（count 行显示
  进度 + 哨兵 200px lookahead 自动加载 + 「加载更多」按钮手动兜底 + 「没有更多」终态）。
  同步清理死 CSS `.pagination` / `.page-info`（PersonalCenterPage 用独立的 `.pager-*`，不受影响）。
- **TODO 校准**：核对代码实际状态后勾选 P1-2（`CreateUser` service 层 actor 纵深防御，修复轮 3 已修）、
  P2-4/5/7/10/11（修复轮 3 已修未勾）；P2-6 降级评估为「生产经 Nginx 覆盖 XFF，可信代理闸门
  暂缓」；合并重复的 P2-11 条目；更新头部状态摘要。
- **验证**：`go build` / `go vet` / `go test ./...` ✅ 全绿；`tsc --noEmit` ✅；`vite build` ✅
  （60 模块，JS 207.37 kB / gzip 63.64 kB，CSS 57.64 kB / gzip 10.85 kB）。
  UI 实机交互未验证：服务进程随会话回收（已知环境陷阱，见 MEMORY「运行方式」），
  数据流由 handler 既有测试覆盖；改动为删除破损 UI 块，保留链路未动。

---

### 第二十八轮 — 代码反推业务逻辑(内容域)+ 修复 banned 越权链 Q2（本轮）

> 方法:应用第 26 轮建立的 AI 协作标准,按 spec-kit「as-built spec」思路**从代码反推**业务逻辑,
> 再以反推结果为审计基线找偏差。反推全文:`docs/BUSINESS_LOGIC.md`。

- **新增 `docs/BUSINESS_LOGIC.md`(as-built 权威描述)**:内容域全量反推——五态状态机转换表
  (16 个 service 函数的证据链)、可见性矩阵(canSeePost vs 列表作用域 vs 队列作用域三层差异)、
  创建/编辑/删除权限模型、审核流、封禁库、列表/归档/feed。附**审计疑点清单 Q1–Q4**。
- **修复 Q2(🔴 P1,反推发现)**:作者可 `UpdatePost`/`DeletePost` 自己已 banned 的文章——违背
  "banned 仅 root 管"设计;且作者软删后 root 的 `DeleteBannedPermanently` 因软删 scope 404,
  封禁数据永久残留。修法:`UpdatePost` 对 banned 文章非 root 一律 403(root 保留编辑权);
  `DeletePost` 对 banned 文章**所有角色** 403(banned 出口收敛到 vault 的 send-back/永久删)。
  新增回归 `banned_freeze_test.go` `TestBannedPostFrozenForAuthor`(6 断言:作者编辑/删除 403、
  root 编辑放行、root 软删 403、vault 行完好、永久删仍可用)。
- **Q1 留待产品决策(已入 TODO P2-12)**:审核入口无状态前置校验——published 可被 reject 打回、
  banned 可被 approve 平反跳 published。Q3(admin 单篇读取域宽于队列域)/Q4(软删作者的 pending
  成孤儿文)记录不修。
- **验证**:`go build` / `go vet` / `go test ./...` ✅ 全绿(含新回归);`tsc --noEmit` ✅;`vite build` ✅
  (JS 207.37 kB / gzip 63.64 kB,CSS 57.64 kB / gzip 10.85 kB)。
- **下一步(反推继续)**:用户域(accounts/auth)与系统域(backup/upload/mail/限流)反推,见
  `BUSINESS_LOGIC.md` §2/§3 占位。

---

### 第二十九轮 — 代码反推业务逻辑（用户域）+ 修复 disabled 越权 Q6（本轮）

> 继续 as-built 反推（第 28 轮方法）。本轮覆盖用户域：auth/accounts/token/账号状态门。

- **`docs/BUSINESS_LOGIC.md` §2 用户域反推完成**：注册门/登录顺序（disabled 在密码校验前）、
  Token 机制（HS256 + ver claim 吊销 + legacy 兼容）、**Authenticate 四道门链**、
  账号状态 × 访问矩阵（正常/disabled/must_change_password/token 旧 四态 × Login/WithAuth/
  WithOptionalAuth）、用户管理六端点规则表（CreateUser/UpdateUser/ResetPassword/
  ChangePassword/RevokeAllSessions/DeleteUser/GetAuthor）。
- **修复 Q6（🔴 P1，反推发现）**：`Authenticate` 无 disabled 检查——Login 拦禁用账号，但已持有
  JWT（最长 24h）仍可调用所有 API（含写操作）；admin 禁用恶意账号后其会话当天仍可用。
  must_change_password 有门、disabled 没有，同类状态只做一半（与 P0-1 同构的"设计了检查漏落地"）。
  修法：`Authenticate` token_version 校验后加 `disabled → 403`，一处修覆盖两个中间件。
  新增回归 `disabled_gate_test.go` `TestAuthenticateRejectsDisabledUser`（禁用 403 / 重新启用同 token 恢复）。
- **记录不修**：Q8（UpdateUser disabled 分支不对称：admin 不能禁另一个 admin 但能启用）、
  Q9（DeleteUser last-admin 计数不含软删 admin，无恢复端点影响有限）。
- **验证**：`go build` / `go vet` / `go test ./...` ✅ 全绿（含新回归）；本轮未触及前端。
- **下一步**：系统域反推（backup/upload/mail/限流/审计日志），见 `BUSINESS_LOGIC.md` §3。

---

### 第三十轮 — 代码反推业务逻辑（系统域）+ 修复备份导入 root 接管路径 Q7（本轮）

> as-built 反推收官轮。系统域全覆盖：备份/快照/回收站/上传/邮件/限流/审计。

- **`docs/BUSINESS_LOGIC.md` §3 系统域反推完成**：备份导出（全表 + password_hashes +
  security_note + schema 门）、导入（dry-run/自动快照 keep 3/事务清表重建）、快照恢复
  （VACUUM INTO + 只读打开 + 行级复制 + 恢复前可撤销快照）、回收站（7 天窗口/410）、
  图片上传（双层大小封顶/扩展名白名单/随机文件名/SM.MS token 服务端持有）、
  邮件（best-effort + 15min throttle + 三种模板）、双层限流对照表、审计记录点清单。
- **修复 Q7（🔴 P0，反推发现）**：备份导入/恢复是**完整的 root 接管路径**——handler 仅
  requireAdmin,而导入事务 `tx.Create(&u)` 不校验角色,admin 导入把自建用户设为 root
  (或替换 root 的 password_hash)的备份 JSON 即可接管站点,直接绕过红线 R2。
  修法(三层):service 层 `ImportBackup`/`RestoreSnapshot` 加 actor 参数 + `IsRoot` 守卫
  (与 P0-1/P1-2 同构纵深防御);handler 改 `requireRoot` 透传;前端导入区对非 root
  禁用并说明(新键 `admin.importRootOnly` 中英)。回归 `backup_root_gate_test.go`
  `TestImportAndRestoreAreRootOnly`(admin 403 / root 放行 / 恶意 root 行未落库)。
- **记录不修**：Q10(上传不校验 magic bytes,靠 Content-Type 缓解)、Q11(SM.MS 无超时)、
  Q12(快照 keep=3 边缘)。
- **验证**：`go build` / `go vet` / `go test ./...` ✅ 全绿（含新回归 + 既有备份测试改签名）；
  `tsc --noEmit` ✅；`vite build` ✅（JS 207.60 kB / gzip 63.73 kB）。
- **反推完成度**：内容域/用户域/系统域三域全覆盖,Q1–Q12 疑点清单闭环（修 4 个 P0/P1,
  记录 8 个 P2/P3）。as-built 反推法三轮共抓 3 个 P0/P1,模式一致:"状态检查只落地一半"。

---

### 第三十一轮 — P2-2 前端重复代码抽取（fmtDate 去重 + locale 异构修复）（本轮）

> Q1 需产品决策暂缓；本轮做 TODO P2-2 中风险最低、收益最明确的一项。

- **新增 `web/src/core/format.ts`**：`fmtDate(iso, locale)`（全文时间戳，空值 "—"）+
  `fmtDay(iso)`（仅年月日，ISO 短格式）两个共享格式化函数。
- **去重 6 处**（审计估值 ×7，实际 6）：AdminBanVault / AdminDashboard / AdminReview /
  AdminTrash / AdminUsers 五页的 `toLocaleString` 变体 → `fmtDate`；
  PersonalCenterPage 的 `slice(0,10)` 变体 → `fmtDay`。6 页本地定义全删。
- **顺手修异构点（P2-3 边缘）**：原 5 处 `fmtDate` 都不传 locale——英文界面下日期仍按
  浏览器 locale 渲染。统一经 `useI18n` 的 `locale`（`zh-CN`/`en-US`）传入，与 ListPage 的
  localeTag 模式对齐；AdminUsersPage 4 个组件中仅用到的那个接 locale（其余还原，tsc 零告警）。
- **修正审计估值**：TODO P2-2 的「fmtDate ×7 / Modal ×5」订正为「fmtDate ×6 / Modal ×3」。
- **验证**：`tsc --noEmit` ✅；`vite build` ✅（60 模块，JS 207.62 kB / gzip 63.79 kB）。
  P2-2 余项（load ×7 / Modal ×3 / 表格三态 / 分页器 / writeError ×15）留待后续轮次。

---

### 第三十二轮 — P2-2 Modal 骨架抽取（×5）+ locale 异构收尾（本轮）

- **新增 `web/src/ui/Modal.tsx`**：共享 modal 外壳（title/onClose/children/footer/wide），
  backdrop 点击关闭、面板点击 stopPropagation 的语义由组件统一保证。
- **抽取 5 处**（审计估值 ×5 正确，第 31 轮曾误订正为 ×3）：
  AdminUsersPage×3（UserForm / ResetPassword / **LoginHistory**）、
  AdminReviewPage×1（reject）、AdminBanVaultPage×1（send-back）。
  LoginHistory 为 `modal--wide` 宽变体——靠 grep 残留验证时才暴露（前两轮侦察漏了它），
  已在 Modal 加 `wide` prop 覆盖。
- **又抓一个漏网日期函数**：AdminUsersPage LoginHistoryModal 内藏本地 `fmt`（第 31 轮因命名
  不同漏掉）；AdminAuditPage 也有一个 `fmt`——后者为**有意差异化保留**（无效日期回显原值
  而非 "—"，审计行不隐藏源数据，已加注释说明），仅补 locale 一致性。
- **估值订正**：审计「fmtDate ×7」实际正确（6 个 fmtDate + 1 个 fmt）；TODO 已改回。
- **验证**：`tsc --noEmit` ✅；`vite build` ✅（JS 206.66 kB / gzip 63.79 kB，两轮去重累计 -0.96 kB）。
  modal-backdrop  markup 现只存在于 Modal.tsx 与 index.css。

---

### 第三十三轮 — 跨栈契约核对 + 修复 cover 漂移（本轮）

> as-built 反推的跨栈延伸:后端三域已反推完,本轮核对「前端实际调用 vs 后端实际端点」——
> 这类漂移 tsc / go build 都发现不了,只能显式对照。

- **新增 `scripts/contract_check.py`**:解析 `web/src/core/api.ts` 全部 `request<T>()` 调用与
  router.go 全部路由,输出「方法不匹配」与「后端未被调用」两张清单。**结果:55 个前端调用
  method+path 全部正确**;未调用的 6 个路由中 5 个合理(health/feed/sitemap/uploads/根索引,
  非前端职责)。
- **修复契约漂移(第 33 轮新发现,已入 TODO P2-13)**:后端 `CreatePostInput` 缺 `cover` 字段,
  而 `CreatePage` 创建文章时发 cover(PostEditor 封面功能)——Go JSON decoder 静默丢弃未知字段,
  **创建时设的封面直接丢失**(仅编辑路径有 Cover)。修:struct 加 `Cover string` + `CreatePost`
  落库;回归 `post_cover_test.go` `TestCreatePostKeepsCover`(返回值 + DB 行双断言 + 空值不幻影)。
- **记录两个缺口(TODO P2-14/P2-15)**:① `POST /api/backup/restore-snapshot` 前端零调用——
  root 恢复快照只能 curl,AdminBackupPage 缺「从快照恢复」UI;② `api.ts` 的 `banPost` 死导出
  (无调用,note 参数后端也不接收)。
- **验证**:`go build` / `go vet` / `go test ./...` ✅ 全绿(含新回归);本轮未触及前端。
- **方法论沉淀**:契约核对脚本留 `scripts/`,以后每次改请求/响应结构跑一次——第 26 轮建立的
  标准 + 第 28 轮起的 as-built 反推,至此形成「反推业务逻辑 → 核对跨栈契约 → 抓漂移」完整闭环。

---

### 第三十四轮 — P2-14 快照恢复 UI（后端列表端点 + 前端区块）（本轮）

- **后端新增 `GET /api/backup/snapshots`**（root-only）：`ListSnapshots` service（读 backupDir,
  过滤 `snapshot-*.db`,按 mtime 倒序,目录不存在返空列表而非报错)+ handler `requireRoot`
  + router。与 `RestoreSnapshot` 同门（快照含全部账号与密码哈希,Q7 同款治理）。
  回归 `backup_snapshots_test.go`：admin 403 / 缺目录返空 / 非快照文件过滤 / 大小正确。
- **前端**：`api.ts` 加 `listSnapshots` / `restoreSnapshot`；`types.ts` 加 `SnapshotInfo`；
  AdminBackupPage 新增「快照恢复」区块（仅 root：文件名/大小/创建时间表格 + 每行恢复按钮 +
  `snapshotRestoreConfirm` 确认框 + 恢复后刷新列表）；i18n 11 个新键（zh/en）。
- **踩坑记录**：新 i18n 键初版命名 `admin.restoreConfirm` 与回收站既有键**撞名**（语义不同:
  回收站用 `{title}`、快照用 `{file}`）——tsc 的 duplicate identifier 报错抓住,改名
  `admin.snapshotRestoreConfirm`。加 i18n 键后必须 tsc,重名在运行期才会以文案错乱暴露。
- **验证**：`go build`/`go vet`/`go test ./...` ✅；`tsc --noEmit` ✅；`vite build` ✅
  （JS 208.99 kB / gzip 64.48 kB）；`contract_check.py`：restore-snapshot 已接入,
  未调路由仅剩 5 个合理项（health/feed/sitemap/uploads/根索引）。

### 第三十五轮 — 修 Q1 审核状态前置（P2-12）+ 删 banPost 死导出（P2-15）（本轮）

- **Q1 修复（P2-12）**：`ReviewPost` 此前对任何 status 都可操作——published 被 reject 会悄悄下线公开内容、banned 被 approve 会跳过 send-back 直接平反。修：`ReviewPost` 加状态前置，仅 `pending`/`rejected` 可审，`published`/`private`/`banned` 一律 409「post is not awaiting review」；设计取舍：不允许"重新审核已发布文章"，banned 必须走 `SendBackPost`→作者修改→重提交→再审。`BanPost` 加"已 banned 重复 ban → 409"守卫（关掉"BanPost 缺状态前置"那条）。
- **P2-15 删死导出**：`api.ts` 的 `banPost(id, note?)` 全前端零调用（service 的 `BanPost` 也不接收 note），按审计建议删除（`tsc`+`vite build` ✅）。
- **遗留产品决策（记入 TODO P2-15）**：封禁库当前**无 UI 入口可发起 ban**（`/admin/banned` 只能 send-back/删除），即"只出不进"。若需补，应在某管理页加 root-only "封禁"按钮复用 `POST /api/admin/ban/{id}`（届时重新导出 `banPost`）。待用户拍板。
- **回归** `review_state_test.go` `TestReviewPostStatePrecondition`（9 断言：published 拒 reject / private 拒审 / rejected 可再审 / banned 拒审 root+admin / 拒审后状态不变 / 重复 ban 409）。
- **验证**：`go build`/`go vet`/`go test ./...` ✅（全包）；`tsc --noEmit` ✅；`vite build` ✅（62 模块；JS 208.99 kB / gzip 64.48 kB，CSS 57.64 kB / gzip 10.85 kB）。
- **文档**：`docs/TODO.md` 勾 P2-12/P2-15 + 顶部 P2 清单同步；`docs/BUSINESS_LOGIC.md` §1.1 状态表 / §1.4 审核流 / §4 Q1 行全更新；`.workbuddy/memory/2026-10-19.md` 第 35 轮。

### 第三十六轮 — 修 Q11 SM.MS 超时（真 bug）+ P2-2 writeError 去重（本轮）

- **Q11 真 bug 修复（P3）**：`uploadToSmms` 走 `http.DefaultClient` **无超时**——SM.MS 远端挂起会占住 goroutine（且阻塞外层 `UploadImage` handler），低配 VPS 几个就紧。修：新增包级 `smmsClient = &http.Client{Timeout:10s}`，`uploadToSmms` 改用 `http.NewRequestWithContext(ctx, …)` 绑定请求 ctx（客户端断开即取消）+ 10s 硬超时兜底；`UploadImage` 透传 ctx。这是审计疑点（Q8–Q12）里唯一仍未修的真 bug（Q8/Q9 为状态不对称记录不修、Q10 magic bytes 记录不修、Q12 keep=3 为规格定稿）。
- **P2-2 writeError 去重（×16，实际比审计估值 15 多 1）**：后端 handlers 16 处完全相同的 `writeError(w, 400, "invalid request body")` 收束为 `writeInvalidBody(w)` 辅助（`handlers/auth.go`）；其余非同质消息（`backup too large` / `page, size and category must be integers` / `admin only` / `root only` 等）不动。纯机械去重，零行为变化。
- **文档修正**：`docs/BUSINESS_LOGIC.md` §3.5 原文把 magic bytes / SM.MS 超时错标为 Q11 / Q12（与 §4 表 Q10 / Q11 冲突），已订正为 Q10 / Q11 并标注 Q11 已修；§4 Q11 行更新；`docs/TODO.md` P2-2 余项去掉 writeError。
- **顺带核查**：`web/src/core/api.ts` 错误解码（`ApiError` + `data.error`）一致、无第二种写法，前端"解码错误消息 2 种"属风格差异非 bug；后端 `writeError` 形状统一（`{"error":msg}` + 正确 Content-Type），无不一致。
- **验证**：`go build` / `go vet` / `go test ./...` ✅（全包）。

### 第三十七轮 — P2-3 limit 解析统一（真改进）+ P2-2 表格三态去重（本轮）

- **P2-3 limit 解析 3 种规则 → 统一（纵深防御缺口修复）**：原三端点解析 `?limit=` 写法不一致——`auth.go`(自助登录历史) `n>0 && n<=100`、`accounts.go`(admin 登录历史) `err==nil` 无上下限、`admin.go`(操作审计) `n>0` 无上限。后两处无上限、一处无下限，纯靠 service 层 clamp 兜底（ListActions `≤0→50 />200→200`、ListUserLogins `≤0或>100→50`），handler 层无独立边界=纵深防御缺口。修：新建 `handlers/query.go` 的 `parseLimit(q, key, def, max)`（clamp 到 `[1,max]`），三处统一调用，上限与 service 对齐（ListActions 200 / 登录历史 100）；顺手删 `auth.go`/`accounts.go` 因不再使用而多余的 `strconv` import。`go build/vet/test` ✅。
- **P2-2 表格三态 ×6 去重**：新建 `web/src/ui/TableState.tsx`（`loading`/`isEmpty` → `loadingText`/`emptyText` + `children`），6 个 admin 列表页（Categories/Tags/Trash/Review/BanVault/Users）的 `loading?spinner:empty?emptyMsg:table` 三元统一收束，消除"某页漏空态"隐患。AdminTrashPage 原空态多包一层 `<div>` 已等价内联。`tsc --noEmit` ✅；`vite build` ✅（63 模块）。
- **经评估不做的余项（记入 TODO P2-2）**：`load` 回调 ×7 每个 fetch 不同数据/setter，且 Users 页在 `useEffect` 内按角色守卫后才 `load()`，抽泛型 hook 会改变行为、属过度抽象非缺陷；分页器 2 套已在 P2-8 删传统分页器后实质收敛为无限滚动单一机制。P2-3 其余 7 类（删除响应/确认框/加载态命名/useEffect 依赖/异步竞态防护/解码错误消息/品牌回退）经核查均为纯样式差异、行为已一致，不做机械统一。
- **回归** `handlers/query_test.go` `TestParseLimit`（9 断言：缺失/非数字/0/负数/超限/恰好上限/区间内/登录历史上限 100/区间内 10）。
- **验证**：`go build` / `go vet` / `go test ./...` ✅（全包）；`tsc --noEmit` ✅；`vite build` ✅（63 模块）。
- **文档**：`docs/TODO.md` 勾 P2-2（fmtDate/Modal/writeInvalidBody/TableState 全收敛）+ P2-3（limit 已统一 + 其余 7 类核查结论）+ 顶部 P2 清单同步；`.workbuddy/memory/2026-10-19.md` 第 37 轮。

---



## 五、已知风险 & 待解决问题

| 等级 | 问题 | 影响 | 状态 |
|------|------|------|------|
| 🔴 高 | `echo.draft.<id>` 不清理，localStorage 随文章增多膨胀 | 大量删除文章后本地存储不回收 | ✅ 已解决（第 14 轮 TTL + 保存回收） |
| 🔴 高 | SM.MS token 存 localStorage 明文 | 安全风险（token 权限范围可控，低危） | 计划解决：第 19 轮 spec 05 改为服务端持 token（Draft 待实现） |
| 🟡 中 | GORM `Select("COUNT(*)")` 污染后续 scope | 查询结果错误 | 已修复，需警惕类似用法 |
| 🟡 中 | 旧版 `listPosts()` 函数保留但未使用 | 死代码维护负担 | 待清理 |
| 🟡 中 | 编辑器自动保存仅 localStorage，不跨设备 | 多设备编辑无法同步草稿 | 待规划 |
| 🟢 低 | 全文搜索 LIKE `%...%`，千级以上文章变慢 | 性能瓶颈 | 纯 Go 驱动无 FTS5，暂保留 LIKE；若换支持 FTS5 的驱动再升级 |
| 🟢 低 | `JWT_SECRET` 生产环境需覆盖 | 安全 | `.env.example` 已注明 |
| 🟢 低 | `/archive` 一次性拉全部已发布文章（已剥 body，但含 author/category/tags） | 文章上万时单响应偏大 | 已知，接受（低配站规模有限）；量大时改按年分页 |

---

## 六、后续方向（优先级排序）

1. **生产部署验证** — 用 `deploy/` 脚本在真实 VPS 上端到端测试（需 1 vCPU VPS 配合；无 VPS 时先做离线 dry-run）
2. **PWA 化** — Service Worker 缓存，离线访问
3. **全文搜索升级** — 引入 SQLite FTS5 替代 LIKE `%...%`；⚠️ 当前纯 Go 驱动（`glebarez/sqlite`）无 FTS5 扩展，需先换驱动（如启用 cgo 的 `mattn/go-sqlite3`，代价：二进制 10→20 MB、VPS 需 gcc 工具链、失去纯 Go 卖点）
4. **导入体验增强** — 定时自动备份 + 保留 N 份；导入前 dry-run 预览；回收站过期天数可配置

---

## 七、编译验证基线（第二十五轮完成后）

| 检查项 | 结果 |
|--------|------|
| `tsc --noEmit` | ✅ |
| `vite build` | ✅ 60 模块；JS 206.28 kB / gzip 63.42 kB，CSS 57.29 kB / gzip 10.76 kB |
| `go build ./...` | ✅ |
| `go vet ./...` | ✅ |
| `go test ./...` | ✅ 全绿；本轮新增 `moderation_test.go` 7 个用例（pending 403 / 队列作用域 / reject 必带 note / ban 需 root / 封禁库对 admin 不可见 / send-back 出库 / 硬删） |
| 已知局限 | 未 docker build（开发机无 Docker）；未在真实 VPS 端到端跑 |
