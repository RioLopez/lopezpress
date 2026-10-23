> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# LopezPress（原 Echo Content API）— 阶段总结与问题记录

> 更新时间：2026-10-23（第 55 轮：P1 拿回分层边界）
> 用途：存档各轮次进度、已知风险、待解决问题，供后续接续开发参考。
> 说明：仓库早期名为 echo-content-api，2026-10-20 第 38 轮前正式更名 LopezPress；下文历史记录中偶见的旧名指同一项目。

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
| 第二十六轮（AI 协作标准落地） | AGENTS.md / AI_STANDARD / 提示词库 | ✅ 已实现 | 新增 `AGENTS.md` + `docs/AI_STANDARD.md` + `docs/prompts/`×5 + 外部方法论参考 |
| 第二十七轮（P2-8 分页双机制） | 删传统分页器，统一无限滚动 | ✅ 已实现 | 修掉「上一页重复追加第 1 页」真 bug；清死 CSS |
| 第二十八轮（内容域代码反推 + Q2） | banned 文章冻结（P1-15） | ✅ 已实现 | 作者改/删 banned 文章 403；出口收敛到封禁库 |
| 第二十九轮（用户域代码反推 + Q6） | disabled 账号鉴权门（P1-16） | ✅ 已实现 | `Authenticate` 加 disabled 403，覆盖 WithAuth/WithOptionalAuth |
| 第三十轮（系统域代码反推 + Q7） | 备份导入/恢复 root 接管（P0-7） | ✅ 已实现 | service 层 `IsRoot(actor)` + handler `requireRoot` 双闸 |
| 第三十一轮（P2-2 前端去重） | `fmtDate` ×7 收敛 + locale 修复 | ✅ 已实现 | 新建 `core/format.ts`；修英文界面日期按浏览器 locale 渲染 |
| 第三十二轮（P2-2 前端去重） | `Modal` ×5 收敛 | ✅ 已实现 | 新建 `ui/Modal.tsx`（含 `wide` 变体） |
| 第三十三轮（跨栈契约核对） | 修 `cover` 字段漂移（P2-13） | ✅ 已实现 | `CreatePostInput.Cover` 落库；新增 `scripts/contract_check.py` |
| 第三十四轮（P2-14 快照恢复 UI） | `GET /api/backup/snapshots` + 前端区块 | ✅ 已实现 | root-only 列表 + 恢复按钮 + 确认框 |
| 第三十五轮（审核状态前置） | P2-12 状态前置 + P2-15 删死导出 | ✅ 已实现 | `ReviewPost` 仅 pending/rejected 可审；`banPost` 死导出删除 |
| 第三十六轮（SM.MS 超时 + 去重） | Q11 SM.MS 请求超时真 bug + `writeError` ×16 收敛 | ✅ 已实现 | `writeInvalidBody(w)` 辅助 |
| 第三十七轮（P2-3 limit 统一） | `parseLimit` 收敛 + 表格三态 ×6 去重 | ✅ 已实现 | 新建 `handlers/query.go` / `ui/TableState.tsx` |
| 第三十八轮（文档校准 + 完成度核查） | README / AGENTS / HANDOVER / TODO / PROGRESS 对齐代码实际 + 全量门禁复跑 | ✅ 本机实测 | 见 §4.38 |
| 第三十九轮（修 P1-17 + P2-16） | root 密码离线恢复 CLI / 清 4 个无用前端依赖 | ✅ 本机实测 | 见 §4.39 |
| 第四十轮（首次推送 + 凭据台账） | github-auto-upload 技能推远程 / 建立 CREDENTIALS.md | ✅ 已推上 GitHub | 见 §4.40 |
| 第四十一轮（母文档 `MASTER.md`） | 文档总索引 + 自驱协议 + 事实基线 | ✅ 已推上 GitHub | 见 §4.41 |
| 第四十二轮（母文档可持续化） | 会话启动提示词 ×3 / 断点协议 / `doc_check.py` 机器校验 | ✅ 本机实测 | 见 §4.42 |
| 第五十三轮（架构复审） | 同题异构取证 + 目标分层 + 8 阶段路线 | ✅ 只读，未改代码 | 见 `docs/ARCH_REVIEW_2026-10-23.md` |
| 第五十四轮（P0 立规矩） | `scripts/arch_check.py` 棘轮门禁 + `models.All()` 治漂移漏表 | ✅ 门禁全绿 | 见 §4 |
| 第五十五轮（P1 分层） | service 层传输符号 12 处 → 0 + `internal/imghost` 抽出 | ✅ 门禁全绿 + 7 组真跑冒烟 | 见 §4 |

> 第 43–52 轮为功能轮次，详情见 §4 对应小节；本节自第 53 轮起只收录"改了架构或门禁"的里程碑轮。

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

### 第三十八轮 — 文档校准 + 完成度/真实性核查（本轮，2026-10-22）

**性质**：纯文档轮，不改业务代码。目的是消除"文档声称 ≠ 代码实际"的落差，把核查证据固化进文档。

**核查方法**：逐条把文档断言与代码事实对照（`go.mod` / `package.json` / `config.go` 默认值 / `router.go` 路由表 / `seedDemoData` / `api.ts` 导出清单 / `scripts/contract_check.py` 输出），并完整复跑验证门禁。不采信历史文档结论。

**修正的文档错误**

| 文件 | 原表述 | 实际 |
|---|---|---|
| README | 限流「5 rps / burst 10」 | `config.go:66-67` 默认 **20 / 50**，`.env` 与 `.env.example` 同为 20/50 |
| README | 路由「`@preact/signals` + 状态导航，非 react-router」 | 实际用 **`wouter`**（`core/router.tsx`），signals 包零 import |
| README | 演示账号角色 `admin` | `seedDemoData` 写入 **`RoleRoot`**（全库唯一 root） |
| README | 「`/logout` 只是客户端信号，需吊销请引入黑名单」 | 已实现 `token_version` 吊销 + `logout-all` |
| README | 项目结构只有 `handlers/(auth,posts)`、6 页面 | 实际 8 个 handler 文件、22 个页面、9 个公共组件 |
| AGENTS.md / HANDOVER.md | 「后端 ~12 模块、前端 2 个 npm 包」 | `go.mod` 6 直接依赖(+10 indirect)；前端实际用 3 个，另有 4 个零 import 依赖 |
| PROGRESS.md | 标题 `Echo Content API`；§三 总览停在第二十五轮；§七 基线停在 25 轮；小节号「六」重复 | 已更名、补齐 26–38 轮、基线重测、重编号 |

**新增待办**：`docs/TODO.md` **P1-17**（root 密码无恢复路径，运维缺口，需产品拍板）、**P2-16**（4 个零 import 前端依赖待清）。同时修正 3 处勾选状态与正文不符（P2-1 已修却未勾、P2-6 未修却已勾、P2-3 标未完成但正文为部分完成）。

**验证**：`go build` / `go vet` / `go test ./...` 全绿（117 个 `Test*`）；`tsc --noEmit` / `vite build` 全绿；`contract_check.py` 报方法不匹配 0 条；基线见 §七。

**遗留**：工作树相对 `v0.1.0` tag 有 20+ 文件未提交（均为 10-22 文档两类制横幅改动 + 本轮校准），需用户决定何时提交。

### 第三十九轮 — 修 P1-17（root 密码离线恢复）+ P2-16（无用前端依赖）

**背景**：第 38 轮文档核对发现两个真实缺口——root 忘密码无路可走（P1-17）、前端 4 个包零 import（P2-16）。本轮按 T2 流程先写 spec 再实现。

**P1-17 · root 密码离线恢复（新增 spec `22-root-password-recovery.md`）**
- **为什么不做 HTTP 端点**：那等于开一条远程接管通道，直接违背 R2「root 不可经任何 API 创建/提升」。恢复动作只能落在"已经能读 DB 文件"这个既有信任边界上。
- `internal/config/recover.go`：`ResetRootPasswordForCLI(db)` 查唯一 root（`created_at ASC` 兜底多 root 脏数据）→ 生成 22 字符随机密码 → **一条 UPDATE 同时完成三件事**：换 `password_hash`、置 `must_change_password=true`、`token_version+1`（吊销旧会话——密码丢失常伴随会话被他人持有，恢复即清场）。
- `cmd/server/main.go`：新增 `-reset-root-password` 标志，在**启动 HTTP 之前**拦截并 `os.Exit`；先 `os.Stat(cfg.DBPath)` 拒绝不存在的库（否则 `InitDB` 会静默 seed 出一个假站点并在错误路径上报"重置成功"）。首启 banner 措辞从 "a new admin account" 改为 "the root account（role: root）"，与 seed 实际写入的 `RoleRoot` 一致，并提示该恢复命令。
- **回归** `internal/config/recover_test.go`（3 用例）：轮换成功（新密码 verify、旧密码失效、强制改密、`token_version` 3→4、角色与禁用状态不变）/ 只动 root（普通用户 hash 与 `token_version` 不受影响）/ 无 root 时返回 `ErrNoRootAccount` 且**不凭空造出 root 行**。
- **端到端实测**（临时库 `data/rc38.db`、端口 18081，跑完即删）：① 对不存在的库执行 → 拒绝、退出码 1、文件未被创建；② 首启 seed 拿旧密码 → 执行恢复 → 拿到新密码；③ 旧密码登录 **401**、新密码登录 **200** 且响应 `must_change_password:true`、`role:"root"`、JWT 内 `ver:1`（佐证 `token_version` 已递增）。

**P2-16 · 清理无用依赖**
- `npm uninstall @preact/signals @preact/signals-core preact-router preact-render-to-string`，`web/package.json` 依赖收敛为 `marked` / `preact` / `wouter` 三个（与文档承诺一致）。
- 反证：`tsc --noEmit` + `vite build` 全绿，产物仍是 **63 模块 / JS 209.09 kB / gzip 64.54 kB**，与清理前逐字节相同——说明这些包从未进入构建。

**验证**：`go build` / `go vet` / `go test -count=1 ./...` 全绿（config 包 3 新用例通过，测试总数 117→120）；`tsc --noEmit` / `vite build` 全绿。

**文档**：`docs/TODO.md` 勾 P1-17/P2-16 + 顶部状态同步；`README.md` 演示账号告警改为 CLI 恢复步骤并补进「运维」功能；`HANDOVER.md` §0 同步；`AGENTS.md` specs 区间 01–21→01–22；新增 spec 22。

### 第四十轮 — 首次推送到 GitHub + 凭据台账

- **推送**：用 `github-auto-upload` 技能，`doctor` 报 READY（token 属主 `RioLopez`），`sync` 因本机 `git push` 的 TLS 被墙而自动 fallback 到 **Git Data API**（`mode: api`）。push 了第 38–41 轮全部提交，远程 `main` 与本地一致，仓库确认 `private: true`。
- **技能实测坑（已记录）**：① `sync --dry-run` 实际上会真推，不能当只读预演；② `git fetch/push` 直连报 schannel `CRYPT_E_REVOCATION_OFFLINE`，换 openssl 报 `SSL_read: unexpected eof`；③ `credential.helper=manager` 链路失效（`git credential fill` 读不到凭据）。
- **凭据台账**：新建 `~/.workbuddy/CREDENTIALS.md`（**仓库外**，避免被 commit 泄露），含 GitHub token、技能用法与坑、本机工具链路径、应用凭据占位表。
- 未改业务代码，跳过代码门禁。

### 第四十一轮 — 母文档 `MASTER.md`（本文件体系的入口）

- **动机**：用户要求「一个母文档，说清创建了什么文档、每个有什么用什么功能，以后只让我去母文档找」。
- **产出** `MASTER.md`（仓库根）：§2「我想干 X 该读哪份」定位表；§3 全量文档总表（入口族 / AI 约束族 / 项目事实族 / 规格决策 / 审计 / 部署 / 记忆层，共 54 份，标注 A/B 类与"何时读"）；§4 我建过哪些资产、各自解决什么问题；§5 仓库外资产（凭据台账等）；§6 自驱协议（触发词→动作、T0–T3 分级、完成定义、记录去向、9 条硬红线、单对话 ≤30 轮）；§7 事实基线数字；§8 维护规则。
- **接线**：`AGENTS.md` 抬头改为「先读 MASTER」；`README.md` 文档导航首行指向 MASTER；`docs/README.md` 抬头声明总入口在根目录。
- 纯文档轮，跳过代码门禁（基线未变，见 §七）。

### 第四十二轮 — 母文档可持续化（提示词 + 断点协议 + 机器校验）

**问题**：`MASTER.md` 只是"当前快照"，对话一长，AI 会忘掉"做了决定要写回文档"这条纪律，母文档随即过期。用户要的是**可持续**，不是一次写对。

**思路**：模型不会「记得」，只会「读到」。所以不依赖记忆，只依赖三件事——开场重读协议、每个决定当场落盘、用脚本校验落盘结果。

**产出**
- `MASTER.md` 新增 **§0 会话启动提示词（3 段，复制即用）**：冷启动（要求先复述 9 条红线 + 核对断点 commit + 说任务分级）、长对话中途锚定（列"还没落盘清单"）、收尾换对话（写全 5 类记录 + 更新断点 + 跑门禁）。
- `MASTER.md` 新增 **§9 断点与落盘**：9.1 三条铁律（不落盘 = 没做 / 落盘到最窄文档再指路 / 每个断言带证据）；9.2 七类事件→落哪的触发清单；9.3 检查点节奏（T2 完成即全量落盘、每 5 轮检查、≥20 轮强制快照、≥30 轮停止新工作）；9.4 **当前断点表**（时间/commit/正在做/下一步/未决，交接靠它）；9.5 新会话开场 3 问自检；9.6 机器校验。
- **`scripts/doc_check.py`（新，纯标准库）**：校验 ① 文档总数与 MASTER 三处声明一致 ② 根目录/`deploy/`/`docs/` 顶层 19 个 md 逐个被点名 ③ 5 个批量目录已索引 ④ 全部 md 带 `> 类别 A/B` 横幅。**失败注入实测**：临时加一个未索引无横幅的 `TEMP_PROBE.md` → 报 3 项 FAIL、退出码 1；删除后 → PASS、退出码 0（证明不是空跑）。
- **接线**：`AGENTS.md` §4 与 `docs/AI_STANDARD.md` §4 验证门禁都加上该脚本（改文档必跑）；`AI_STANDARD` 追加修订记录。

**验证**：`doc_check.py` PASS（54 份 / 19 顶层 / 5 目录 / 54 横幅）；本轮未改代码，代码门禁沿用 §七 的全绿结果。

**为什么这次的"可持续"比之前可靠**：以前靠 AI 自觉更新文档（第 38 轮就抓到 3 处勾选与正文矛盾）；现在少了索引会**直接红**，把"靠自觉"换成了"跑一遍就知道"。

### 第四十三轮 — 浏览量去重 + 首页纯分页 + 标签改名（2026-10-22）

**起因**：用户实机看站点提出三个问题——首页无页码、浏览量可被刷新刷高、标签/分类后台太弱。用户明确要求「任何想法先共同商讨再执行」，所以先做了一轮事实核对与方案讨论（含对"按账号算浏览量"这一前提的纠正：博客读者多数匿名），逐条拍板后才动手。

**1. 浏览量去重（spec 23，关联 TODO P2-6）**

- 现状是 `GetPost` / `GetPostPublic` **每次 GET 就 `view_count + 1`**：作者自看、刷新、爬虫全算，且可被脚本刷。
- 新增 `post_views(post_id, visitor_hash, day)` + `UNIQUE(post_id, visitor_hash, day)`；`visitor_hash = sha256(ip|ua|JWT_SECRET)`，只存哈希（管理员也无法还原访客 IP/UA）。
- 新端点 `POST /api/posts/{id}/view`（`withOptionalAuth`，匿名可调）：作者本人 → `reason=author`；爬虫 UA → `reason=bot`；同日同人同篇 → `reason=duplicate`；只有 `INSERT` 真正插入（`RowsAffected==1`）才 +1。
- **读接口变纯读**：`GetPost` / `GetPostPublic` 不再改 `view_count`。
- 90 天保留期，**每进程每 UTC 日最多清理一次**（`viewPruneMu`/`viewPruneDay`）。
- **顺带修 P2-6**：新增 `internal/clientip` 包，`TRUSTED_PROXIES`（逗号分隔 IP/CIDR，默认 `127.0.0.1/32,::1/128`）。只有当 `RemoteAddr` 命中可信代理时才采信 `X-Forwarded-For`，否则忽略——伪造头不再能绕过限流/锁定/去重。`middleware.Limiter` 与 `handlers` 两处各自的 `clientIP` 全部并入该包（此前是两份会漂移的实现）。非法配置在启动时 `log.Fatalf`。

**2. 首页纯分页（spec 24）**

- 删掉无限滚动（`IntersectionObserver` + `loadNext` + 追加合并逻辑），改为 URL 驱动的经典分页：`?page=N` 写在地址栏，刷新/分享/前进后退都保持当前页。
- **size 由服务端决定**：前端只传 `page`，不传 `size`——否则同一个链接对不同人含义不同，分享即失效。`site_configs.page_size`（新增列，手工迁移 `0005_add_page_size`，默认 10，只允许 5/10/20/30，非法值 `ClampPageSize` 回落 10）。
- 新组件 `web/src/ui/Pagination.tsx`：首页/上一页/页码窗（当前页 ±2，首末恒显，空档折成 `…`）/下一页/末页/跳转输入框。
- 筛选（关键字、状态、分类、标签）变更时回到第 1 页；深链 `?page=999` 不再报错，显示"这页没有内容"+ 回首页按钮。
- 顺带修 P1-11 遗留：计数行 `from–to` 现在用服务端实际页大小，不再写死 20。

**3. 标签改名 + 计数跳转（spec 25）**

- 后端新增 `PUT /api/tags/{id}`（`requireAdmin`，重名 409 / 空名 400 / 不存在 404，长度 >64 前置 400 避免驱动层 500）。改名只动 `tags.name`，`post_tags` 按 id 关联，文章不丢标签——这正是该端点的存在理由（此前只能删除重建）。
- 标签页补行内改名；分类/标签页的计数变成链接，点进 `/?category=id` 与 `/?tag=name`；两页各加前端本地过滤框（不新增后端搜索接口）。
- 明确**不做**：标签/分类的批量删除（数量少、误删不可恢复，属负收益）。用户坚持要的"批量"改落在"后台文章管理页"上（下一轮，spec 待写）——依据是 Halo 2.17 的批量管理是对**文章**做的，不是对标签做的。

**改动文件**：后端 `models/models.go`（`PostView`、`SiteConfig.PageSize`）、`clientip/clientip.go`（新）、`services/views.go`（新）、`services/services.go`（去自增、`ListPageSize`/`ClampPageSize`、`UpdateTag`）、`services/accounts.go`（`page_size` 映射与钳制）、`handlers/{posts,categories,accounts,auth}.go`、`router/router.go`、`config/config.go`（迁移 + `TRUSTED_PROXIES`）、`cmd/server/main.go`；前端 `pages/ListPage.tsx`（重写）、`pages/ReadPage.tsx`（上报）、`pages/AdminSitePage.tsx`、`pages/AdminTagsPage.tsx`（重写）、`pages/AdminCategoriesPage.tsx`、`ui/Pagination.tsx`（新）、`core/{api,types,i18n,site}.tsx`、`index.css`。

**验证（真机）**
- `go build ./...` / `go vet ./...` / `go test -count=1 ./...` 全绿（新增 13 个测试：views/tag 8 + clientip 5）。
- `npx tsc --noEmit` / `vite build` 全绿。
- 迁移实测：旧库 `data/app.db` 启动后自动加 `page_size=10`，`/api/site-config` 返回 `page_size:10`。
- 分页实测：`/api/posts?page=1` → `items=10 total=12 page_size=10`；`?page=2` → 2 条；`?page=9` → 0 条 + total 12（不重复、不报错）。
- 浏览量实测：首次上报 `{"counted":true,"view_count":11}` → 同 UA 再报 `{"counted":false,"reason":"duplicate"}` → `curl/8.4.0` 报 `reason=bot` → 再次 `GET` 文章 `view_count` 仍为 11（读接口确认不再自增）。
- 鉴权实测：无 token `PUT /api/tags/1` → 401。
- `scripts/contract_check.py`：方法不匹配 0；后端路由 63 → **65**，前端调用 56 → **58**。
- 注：`go test ./internal/clientip/` 在本机报 `fork/exec ...: Access is denied`（新建包首次编译的临时 exe 被环境拦截），改用 `go test -c -o bin/clientip.test.exe` + 直接运行，5 个用例全 PASS。

**文档**：新增 spec 23/24/25；`docs/TODO.md` 勾 P2-6；`MASTER.md` §3/§7 数字更新。

### 第四十四轮 — 后台文章管理页 + 批量操作 + 后台分区（2026-10-22）

**起因**：第 43 轮讨论遗留的两条用户决定——「批量我还是觉得要有」「后台分区我也要」。同时查 Halo 2.17 的发布说明后确认：**Halo 的批量管理是对【文章】做的**（批量发布/取消发布、批量设置分类/标签/作者），不是对标签做的。而我们此前**根本没有"后台文章管理"这个页面**——文章列表就是前台首页，管理按钮长在阅读界面上。所以本轮先补这个页面，批量才有正确落点。

**1. 批量操作后端（spec 26）**

- 新端点 `POST /api/admin/posts/batch`，白名单 6 个动作：`publish` / `private` / `delete` / `set_category` / `add_tags` / `remove_tags`。
- **核心设计：逐条复用 `UpdatePost` / `DeletePost`，不写第二套权限逻辑。** 角色矩阵、审核状态机、"banned 冻结"全部由单条方法继承——这是第 25 轮教训（设计了集中权限层却手写 if-else，漏一个就是 P0）的直接回应。
- **部分成功语义**：某条无权限/不存在 → 记入 `skipped` 并带英文原因，不影响其他条目；只有批量基础设施错误（空 ids / 未知动作 / >100 条 / admin 越权发布）才整体 4xx。
- `publish` 仅 root（与单条路径一致）；admin 调用整批 403，而不是返回 N 条相同 skip。
- `add_tags`/`remove_tags` 先读当前标签做集合运算再交给 `UpdatePost`（它按 `in.Tags != nil` 整体替换）；add 幂等、remove 大小写不敏感。
- 上限 100 条（单写 SQLite 上更长的批量会长时间占写锁）。

**2. 后台文章管理页 `/admin/posts`**

- 表格式列表：勾选框、标题（点击进阅读页）、状态徽标、分类、浏览量、发布时间、行内编辑/删除。
- 筛选：关键字、五态状态、分类；分页复用 `Pagination`，管理表格用固定 20 条/页（**不跟随** `site_config.page_size`——工作面板不该被站点阅读设置绑架）。
- 批量条固定在列表上方：显示已选数量 + 发布（仅 root 可见）+ 转私密 + 改分类 + 加/减标签 + 删除 + 取消选择；**每个动作都过确认弹窗**并写明"将对 N 篇执行 X"。
- 切页/换筛选会**清空选择**，避免"对看不见的行下手"。
- 编辑/删除按钮由首页搬到此处（首页按钮暂时保留，因为普通用户 role=user 目前只能从首页进入编辑，见 TODO F-3）。

**3. 后台导航分区（spec 26）**

- 侧边栏从 11 项平铺改为 **内容 / 用户 / 系统** 三组（`AdminLayout` 的 `group` 机制早就有，只是此前全填同一个 key）：
  - 内容：文章管理、内容审核、分类、标签、回收站、封禁库（root 专属）
  - 用户：用户管理
  - 系统：站点设置、备份、审计日志
- 仪表盘留在顶部不分组。

**改动文件**：后端 `services/batch.go`（新）、`services/batch_test.go`（新）、`handlers/posts.go`、`router/router.go`；前端 `pages/AdminPostsPage.tsx`（新）、`main.tsx`（路由）、`ui/AdminLayout.tsx`（NAV 分组）、`core/{api,i18n}.ts(x)`、`index.css`。

**验证**
- `go build` / `go vet` / `go test -count=1 ./...` 全绿（新增 5 个批量测试，总数 133 → **138**）；顺带 `gofmt` 后把 7 个**无关**的既有文件还原，保持 diff 聚焦。
- `tsc --noEmit` / `vite build` 全绿（65 模块；JS 220.38 kB / gzip 67.32 kB，CSS 59.33 kB / gzip 11.07 kB）。
- `scripts/contract_check.py`：路由 65 → **66**，前端调用 58 → **59**，方法不匹配 0。
- 真机（现签一个 root JWT 打接口）：
  - 无 token → 401；未知动作 → `400 unknown action`；空 ids → `400 ids is required`
  - `add_tags ["smoke-test"]` → `{affected:1,skipped:[]}`，再查文章标签确实多了 `smoke-test`
  - `remove_tags ["smoke-test"]` → affected 1，标签恢复原样（**验证后数据已还原**）
  - 不存在的 id → `{affected:0, skipped:[{id:"nope", reason:"post not found"}]}`（不 500）

**文档**：新增 spec 26；`docs/TODO.md` 勾 F-1/F-2；`MASTER.md` §3/§7/§9.4 更新。

### 第四十六轮 — 阅读权限开关 + 分类层级 + 预设分类树（2026-10-22）

**决策来源**：用户拍板「超级管理员可在后台配置是否需要公开或登录」「评论暂不考虑」「分类最好可以分层级并且由你预设」。

**1. 阅读权限开关（spec 27，解 TODO F-7）**

- 新增 `site_configs.public_read`（默认 true）。**前后端同时执行**：`WithOptionalAuth` 在匿名且开关关闭时直接 401（此前只有前端路由守卫拦，API 侧其实敞着，属于"锁只挂门面上"）；SPA 新增 `Readable` 守卫，仅 `/`、`/archive`、`/read/:id` 走它，写作类路由仍用 `Guarded`。
- 开关读进内存缓存（`publicRead`/`publicReadSet`），`UpdateSiteConfig` 落库后立即失效——改完不用重启。
- 匿名读者看不到任何写作/管理入口：首页「写文章」按钮改为 `{user && ...}`，阅读页的编辑/删除本就有 `canEdit` 门。
- 匿名仍能读 `GET /api/site-config`（否则 SPA 无法知道自己该不该弹登录）。

**2. 分类层级（spec 27）**

- `categories.parent_id`（自引用，**只允许两层**：父级不得再有父级）。
- 建/改分类校验：父级存在、不能是自己、不能造三级、带环检测兜底（防手改数据库导致死循环）。
- 删除分类时**子级上移到祖父级**（不级联删除结构）。
- 按父级筛选时**包含子级文章**（`categoryWithDescendants`），否则父节点看着是空的。
- 后台分类表改为树形缩进 + 父级列 + 新建/编辑时可选父级；前台侧栏按树展示（子级缩进）。

**3. 预设分类树**

- 预设写在 `internal/models/presets.go`（单一事实源，config 的首启播种与服务端的应用接口共用）：
  `技术笔记 → 后端开发/前端开发/数据库/运维部署`、`系统与网络 → Windows/Linux/macOS`、`生活随笔 → 折腾记录/读书笔记`。
- 新库首启自动播种；老库在后台点「应用预设分类」应用。
- **幂等规则**：同名已存在 → 不重建；已存在且无父级 → 只设父级（"归位"）；已有别的父级 → 不动；本身有子级的分类永不被领养（否则造出三层）。
- 实测：现有库应用后 `created=10, reparented=2`（Linux、Windows 被归位到「系统与网络」），再次应用 `created=0, reparented=0`。

**4. ⚠️ 顺带修掉一个既有隐藏 bug：保存站点设置一直 500**

- 真机验证时发现 `PUT /api/admin/site-config` 报 `SQL logic error: no such column: site_url`。
- 根因：这个库建于 `SiteURL` 字段加入模型之前，而**已存在的表永远不会被 AutoMigrate 碰**；当时没有为 `site_url` 写手工迁移，于是该列从未被加上。之前没暴露，是因为没人点过"保存设置"。
- 修复：迁移 `0008_add_site_url`。**并加了持久防线** `checkSchemaDrift`：启动时用反射把每个模型的字段与真实表结构比对，缺列就 `WARN schema drift: <表>.<列> is missing — add a hand-written migration`。以后同类问题会在启动日志里出现，而不是等到某个接口 500。

**改动文件**：后端 `models/{models,presets}.go`、`config/config.go`（迁移 0006/0007/0008 + 漂移检查 + 预设播种）、`services/{services,accounts,access,category_tree_test,presets}.go`、`handlers/{auth,categories,accounts}.go`、`router/router.go`；前端 `main.tsx`、`core/{types,site,api,i18n}.ts(x)`、`pages/{AdminCategoriesPage,AdminSitePage,ListPage}.tsx`、`ui/Sidebar.tsx`、`index.css`。

**验证**
- `go build` / `go vet` / `go test -count=1 ./...` 全绿（新增 6 个测试：匿名开关缓存失效、父级规则、删父级上移子级、父级筛选含子级、预设幂等、预设只领养孤儿）。
- `tsc --noEmit` / `vite build` 全绿（JS 223.70 kB / gzip 68.59）。
- 契约：路由 66 → **67**，前端调用 59 → **60**，不匹配 0。
- 真机：保存设置 200（此前 500）；`public_read=false` → 匿名 `/api/posts`、`/api/categories` 均 401，带 token 200；匿名读 `/api/site-config` 仍 200 且 `public_read=false`；改回 true → 匿名恢复 200；`?category=12`（系统与网络）返回其子级 Windows 下的 1 篇文章。

---



### 第四十七轮 — 读者链路补全 + 「算不算合格博客」盘点（2026-10-22）

**先纠正自己的判断**：第 45 轮的覆盖矩阵按"有没有 spec"打分，把阅读页判成大量 ❌。这轮读代码才发现——**实现度比文档度高**：阅读页早有目录（TOC）、阅读进度条、阅读时长、字数统计、图片灯箱、代码复制、上一篇/下一篇，只是从没写进 spec。属于"文档缺失"，不是"功能缺失"。

**本轮实做四件事（都是真缺）**

1. **归档分页**（TODO F-4）：旧的 `GET /api/posts/archive` 一次返回**全部** published（正文清空但三张关联全 Preload）。新增 `GET /api/posts/archive/paged?page=&size=`（默认 50 / 上限 200 / 只含 published / `body` 恒空），前端改用它 + 通用 `Pagination`；旧端点保留兼容。
2. **robots.txt**：新增 `GET /robots.txt`，声明 sitemap 绝对地址 + `Disallow: /admin`、`/api/`、`/login`；**跟随 `public_read`**——锁站时 `Disallow: /`，不向爬虫广告会 401 的 URL。
3. **列表封面**：`posts.cover` 早存在、编辑器可选，但没有任何列表渲染它（选了等于白选）。列表项加 104×70 缩略图（object-fit:cover + lazy）。
4. **筛选空态**：搜索无结果原本显示"暂无文章，点击上方按钮写第一篇"。现在区分：**有筛选** → "没有找到匹配的文章" + 「清除筛选」；**无筛选** → 原文案。

**改动文件**：后端 `services/services.go`（`ListArchivePage`）、`handlers/{posts,feed}.go`、`router/router.go`、`services/archive_test.go`（新）；前端 `pages/{ArchivePage,ListPage}.tsx`、`core/{api,i18n}.ts(x)`、`index.css`。

**验证**
- `go build` / `go vet` / `go test -count=1 ./...` 全绿（新增 1 个归档分页用例，144 → **145**）。
- `tsc --noEmit` / `vite build` 全绿（JS 224.47 kB / gzip 68.83）。
- 契约：路由 67 → **69**，前端调用 60 → **61**，不匹配 0。
- 真机：`?page=1&size=5` → 5 条 / total 12 / body 全空；`?page=3&size=5` → 2 条；`?page=abc` → 400；`/robots.txt` 含 sitemap 与三条 Disallow。

---

### 第四十八轮 — 文章永久链接（slug）+ 封禁闭环（2026-10-22）

**决策**：用户选**方案 A**——新增 `/p/{slug}`，**旧 `/read/{uuid}` 永久保留**，两个地址都有效。定时发布明确不做。

**1. 永久链接（spec 29）**

- `posts.slug`（迁移 0009，非唯一索引：去重在 service 层做，冲突时自动加 `-2`）。
- `Slugify`：小写化；保留 ASCII 字母数字与中日韩字符；其余折叠成 `-`；上限 80 字符。中文标题直接得中文链接（`/p/校园网自动登录系统`），可读且合法。
- 纯符号标题回退成文章 id，**slug 永不为空**。
- 创建时自动生成；**改标题不动链接**（永久链接的意义）；显式传 `slug` 才改。
- 新端点 `GET /api/permalinks/{slug}`；sitemap 与 RSS 优先 `/p/{slug}`，无 slug 回退 `/read/{id}`（`permalink()` 单一实现）。
- **历史文章回填**：`Service.BackfillSlugs` 在启动时补齐（幂等），本机实测 13 篇全部补齐、0 空值。放 service 而非迁移，因为 slug 生成逻辑只能在 service 里有一份。
- 前端：新页面 `SlugPage` 先解析 slug → 再交给既有 `ReadPage`；`/p/:slug` 路由；阅读页「复制永久链接」按钮（无 slug 的行不显示）。

**2. ⚠️ 踩到启动崩溃：路由歧义**

`GET /api/posts/by-slug/{slug}` 与既有 `GET /api/posts/{id}/public` 在 Go 1.22 ServeMux 里**互为歧义**（两者都能匹配 `/api/posts/by-slug/public`，且谁都不更具体），注册时直接 panic，进程起不来。改为 `GET /api/permalinks/{slug}`（移出 `/api/posts/` 子树）。**教训：新增通配路由前先想清楚同前缀的既有形状。**

**3. 封禁闭环（spec 30）**

- 后端四个封禁端点早就齐了，但**前端没有任何发起入口**——封禁库是单向门，只能出不能进。
- 后台文章管理页为 root 增加行内「封禁」按钮（admin 不渲染），带确认框；已是 banned 的行不显示。

**改动文件**：后端 `models/models.go`、`config/config.go`（迁移 0009）、`services/{services,slug,slug_test}.go`、`handlers/{posts,feed}.go`、`handlers/feed_test.go`、`router/router.go`、`cmd/server/main.go`；前端 `pages/{SlugPage,ReadPage,AdminPostsPage}.tsx`、`main.tsx`、`core/{api,types,i18n}.ts(x)`。

**验证**
- `go build` / `go vet` / `go test -count=1 ./...` 全绿（新增 5 个 slug 用例 + 1 个 sitemap 回退用例，145 → **150**）。
- `tsc --noEmit` / `vite build` 全绿（JS 225.74 kB / gzip 69.26）。
- 契约：路由 69 → **70**，前端调用 61 → **63**，不匹配 0。
- 真机：13 篇文章全部回填 slug（0 空值）；`/api/permalinks/<中文slug>` → 200；未知 slug → 404；sitemap 输出 `/p/…`；RSS `<link>` 为 `/p/校园网自动登录系统`；`/p/anything` 前端路由 200。

---

### 第四十九轮 — 买家视角审查（真装一遍）+ 修 3 个首启 bug（2026-10-22）

**方法**：不读代码，改为「买下来真装一遍」——全新 DB、空实例、按文档部署、登录、改密、备份、改密后用后台，全程记录响应。产出 `docs/PRODUCT_REVIEW.md`（10 维评分 + P0/P1 清单 + 出价建议 + 下轮 6 件事）。

**评分结论**：综合 **7/10**。阅读 9、性能 9、后台 8、写作 7、可发现性 7、首启 7、数据安全 6、部署 5、外观 4、迁移 **3**。

**只有真装才会发现的 3 个 bug（已修）**
1. **预设分类树从未生效**：全新库显示 `技术/教程/随笔` 三个平铺分类。根因是 `seedDemoData` 先建分类，导致 `seedPresetCategories` 的"空表才播种"判断被跳过——上轮的功能在真实首启里等于没上线。修：调整顺序（预设先播种）+ 示例数据复用已有分类。实测现在 12 个两层分类、示例文章落在 `前端开发/后端开发/技术笔记`。
2. **默认品牌是 `Echo`**：新站站点名/标题都是别人的品牌。改为 `LopezPress`（老库的现值属数据修正，需在站点设置里自己改）。
3. **仪表盘返回文章全文**：`recent_posts` 带完整 Markdown，首屏最重响应只渲染标题。服务层清空 `body`。

**同时确认是"设计如此、不是 bug"的**：改密前所有后台接口 403 `password change required`——`LoginPage:28` 检测到 `must_change_password` 会跳 `/change-password`，是安全门而非缺陷。

**买家视角的主要缺口（未修，已进报告）**：部署包未实测 Docker、首启密码只在 stderr（容器里易丢）、无导入器（WordPress/Markdown）、备份只有手动、无草稿态、无 OG 分享卡片、外观几乎不可定制。

**验证**：`go build` / `go test ./...` 全绿；全新实例实测站点名 `LopezPress`、12 个分类、示例文章归入子分类、列表 5.7 ms。

---

### 第五十轮 — 竞品对标缺口分析（2026-10-23）

用户要求「对比其他博客，哪些功能欠缺、哪些深度不够」。产出 `docs/COMPETITIVE_GAP.md`：对 WP / Ghost / Halo / Hugo-Hexo 做 15 个域的深度矩阵（●完整 / ◐有但浅 / ○没有），分「完全缺失 10 项」「有但浅 12 项」「我们更强 5 项」「补强顺序 9 条」。

**结论**：读者端对标 Ghost 能打 8 分，作者端 5 分，运营端（统计/SEO/迁移）4 分。拦住买家的四件事：**导入器、草稿、修订历史、编辑器工具栏**——都在"每天都用但各缺一点"的位置。

**明确建议不抄**：主题体系（代价极高且与轻量定位冲突，改为 3 套配色 + 2 种列表布局）、评论/会员/插件市场。

纯文档轮，未改代码。

---

### 第五十一轮 — 修订历史 + Markdown 批量导入（2026-10-23）

用户要求「历史版本、导入器都要有；深度不够的尽量补，但不能抬高配置要求」。

**1. 修订历史（spec 31）**
- 新表 `post_revisions`（新表走 AutoMigrate，无需手工迁移）；`UpdatePost` 在**写回之前**快照当前状态，所以"恢复"= 回到保存前。
- 每篇最多 20 份，超出淘汰最旧（1 vCPU 上不接受无限增长）。
- 恢复**只回滚标题/正文，不回滚状态**（不能借恢复绕过审核/封禁）；恢复本身也产生快照，可再回退。
- 权限复用"能否编辑该文"的判定；跨文章的 revID → 404。
- 前端：编辑页底部「历史版本」折叠面板，列表 + 恢复（二次确认）。

**2. Markdown 批量导入（spec 32）**
- `POST /api/admin/import/markdown`（multipart，最多 200 个文件 / 单文件 2 MB / 总 24 MB）。
- 支持 frontmatter `title/date/category/tags/draft`；无 frontmatter 时用首个 `# 标题`，再退到文件名；**保留原始发布时间**（否则归档时间线全变成今天）。
- 分类/标签按名复用，不造重复；每文件独立提交，单文件失败不影响其他，响应逐条给 `skipped` 原因。
- root 导入直接 published，admin 导入进待审（不破例）；`draft: true` 建成 private。
- 前端：文章管理页「导入 Markdown」按钮（多选 .md）。

**坑**：`ImportResult` 与既有备份导入的结构体重名，编译报 redeclared → 改名为 `MarkdownImportResult/MarkdownFile/MarkdownSkip`。

**验证（真机）**
- `go test ./...` 全绿（新增 7 个用例：快照内容、20 条上限、恢复不还状态、越权 403、跨文 404、frontmatter 全字段、标题回退、分类标签复用、空文件与 201 个文件）。
- `tsc` / `vite build` 全绿（JS 228.28 kB / gzip 70.17）。
- 真机：改一次正文 → 历史 1 条且内容是旧值 → 恢复 → 正文回到旧值（随后已还原到原文）；multipart 导入 1 个带 frontmatter 的 md → `created:1`；验证用的导入文章与标签已从库中清除。

---

### 第五十二轮 — 统计趋势 + 每日自动备份 + 相关文章/面包屑 + 全文档校准（2026-10-23）

用户指令：「继续补好，等都补好了就改文档、读 MASTER、改所有文档」。本轮先补三项功能，再做一次全仓库文档校准。

**1. 仪表盘统计（spec 33）**

- `GET /api/admin/dashboard` 扩展三块：`view_trend`（近 30 天逐日去重阅读，**服务端补零**）、`pending_total`、`top_posts`（published 且 view_count>0，Top5）。
- 前端仪表盘加第三张卡（待审核）、趋势图、最多阅读榜。**图表是手写内联 SVG**（30 个 `rect`），不引任何图表库——用 CSS 变量填色，暗色主题自动跟随。
- 口径写进 spec：浏览量统计的是"去重后的阅读"，不是请求数。

**2. 每日自动备份（spec 34）**

- `site_configs` 加 `auto_backup`（默认 **关**）与 `backup_keep`（默认 7，1–90），迁移 `0010_add_auto_backup`。
- `Service.MaybeAutoBackup`：`VACUUM INTO` 写 `auto-YYYYMMDD.db`；**文件名即状态**，存在就跳过（幂等）。保留份数由 `pruneAutoBackups` 执行，只删 `auto-*`。
- 触发：启动一次 + 每 30 分钟一次（命中文件时成本 = 一次 `stat`）。语义是"每天最多一次"，不是"每天准点一次"。
- `auto-*` 与 `snapshot-*` 一样可列出、可恢复（`isSnapshotName`）。
- root 专属：`GET/POST /api/admin/backup/auto`；前端站点设置加开关（仅 root 渲染），备份页显示上次快照时间 + 「立即快照」。
- **安全细节**：`UpdateSiteConfig` 收到非 root 提交时，显式继承库里的 `auto_backup`/`backup_keep`，避免 admin 保存设置把 root 的策略冲掉。

**3. 相关文章 + 面包屑（spec 35）**

- `GET /api/posts/{id}/related`（optional auth）：同分类 **或** 共享标签、仅 published、排除自己，按 `view_count DESC, created_at DESC` 取 5 条。一条原生 SQL，不在 Go 里排序。
- 阅读页底部「相关文章」（无结果不渲染），标题上方「首页 / 分类」面包屑。
- 踩坑：`category_id` 是 `*int64`，直接传 `Raw()` 报 `unsupported type *int64` → 先解包成 `any`。

**4. 数据修正**

- 3 篇历史文章 `category_id = 0`（指向不存在的分类）→ 置 `NULL`。这是无效外键，会让后台分类列空白。

**5. 全文档校准（用户要求"改所有文档"）**

| 文档 | 改了什么 |
|---|---|
| `MASTER.md` | §1/§2/§3（份数 67→70、specs 01–35、matrix 44 项）/§7 全部基线数字/§9.4 断点 |
| `AGENTS.md` | indirect 10→11、`go test -count=1`、陷阱从 6 条扩到 **10 条**（新增手工迁移/路由歧义/`*int64`/整表单覆盖） |
| `HANDOVER.md` | 新增 §7.14–7.16；§8 里 `/archive` 与草稿两行改成本轮事实 |
| `docs/TODO.md` | F-4 勾销（47 轮已修）；新增 F-8～F-13，其中 F-8/9/10/11 本轮完成 |
| `docs/FEATURE_DESIGN_MATRIX.md` | **修正一个自身错误：旧表写"33 项"漏数了 W-01～W-10，实际 44 项**；A-01 转全绿、A-10 加 34、新增 R-13 |
| `docs/COMPETITIVE_GAP.md` | 已补项目标注状态 |
| `docs/PRODUCT_REVIEW.md` | 评分与"下一轮 6 件事"按现状重排 |
| `api.md` | 新增 6 个端点 |
| `docs/PROJECT_OVERVIEW.md` | 路由数、页面数、新模块 |
| `docs/ARCHITECTURE.md` | 新表与新服务文件 |
| `docs/BUSINESS_LOGIC.md` | 自动备份/统计/相关文章的行为 |
| `README.md`、`docs/README.md`、`deploy/*` | 功能列表、spec 编号、备份说明 |

**验证**

- `go build` / `go vet` / `go test -count=1 ./...` 全绿：测试 157 → **168**（新增 10 个：自动备份 5 + 相关文章 4 + 仪表盘 1）。
- `tsc --noEmit` / `vite build` 全绿：JS 233.35 kB / gzip 71.77 kB（比上轮 +5 kB，全部来自三个新界面块，无新依赖）。
- 契约：路由 73 → **76**，前端调用 66 → **69**，方法不匹配 0。
- `doc_check.py` PASS：70 份文档、22/22 顶层被点名、70/70 横幅齐全。
- 真机：`view_trend` 30 点且末项 = 今日阅读数；`related` 对带标签文章返回 2 篇；`POST /api/admin/backup/auto` 生成 `auto-20261022.db` 并出现在快照列表；`/`、`/admin` 均 200。

---

### 第五十三轮 — 架构复审：同题异构取证 + 目标分层 + 8 阶段路线（2026-10-23）

用户指令：「把 AI 写的代码拆解，找出某功能用 A 写法、某功能用 B 写法这种不一致，找出架构设计不好的问题，整体重新规划」。本轮**只读取证，未改一行 `.go`/`.tsx`**。

- 产物：`docs/ARCH_REVIEW_2026-10-23.md`（证据链 + 目标架构 + 8 阶段迁移），`docs/TODO.md` 新增「🏗️ 架构债 AR-0～AR-8」。
- 核心判断：不是"写得烂"，是**同一个问题在不同轮次各解一遍**——9 类同题异构并存。
- 实测数字：service 层 5,242 行里 `services.go` 占 2,767 行（52.8%）；27 个 DTO 混在方法之间；`canReviewPost` **零调用**＝"集中权限层"声明不实；审计双轨导致 24 个写操作有 6 个无审计；迁移 9 段克隆；前端 0 测试 / 54 处 `instanceof ApiError` / `fetch` 4 落点。
- 最严重的边界破坏：`Service.ServeBackup/ServeUploads` 直接写 `http.ResponseWriter`，`UploadImage(ctx, r *http.Request)` 首行 `_ = ctx`。
- 目标分层 `httpx → app → {policy, store, models}` 单向，8 条规则全部可机械检查；**先立门禁再搬代码**。

### 第五十四轮 — P0：架构棘轮门禁（2026-10-23）

- 新增 `scripts/arch_check.py` + `scripts/arch_baseline.json`：7 项计数，**棘轮语义**（只拦变多；降了 `--update` 锁住；升了退出码 1）。已实测 FAIL 路径有效。
- 接入 `AGENTS.md` §4、`MASTER.md` §5/§6.3/§9.6，改代码必跑。
- 根治 1：新增 `models.All()` 作为唯一实体清单。原先 AutoMigrate 清单 11 个、`checkSchemaDrift` 手写 9 个，**漏 `PostRevision` + `SchemaMigration`** → 「新增列必须有手写迁移」对 `post_revisions` 完全失效。
- 根治 2：4 处重复 doc comment（`router.New`、`Service.Register`、`ImportBackup`、`handlers.Register`），并把「注释块内重复行」做成规则 R9（基线 0）。
- 验证：`go build/vet/test` 全绿；新库 + 旧库副本两次冒烟，无 drift 警告、11 张表齐全。

### 第五十五轮 — P1：拿回分层边界（2026-10-23）

- `ServeBackup` → `ResolveBackupFile(name) (path, size, err)`；`ServeUploads` → `ResolveUploadFile(name) (path, err)`；`UploadImage(ctx, r *http.Request)` → `UploadImage(ctx, filename, data)`。设响应头、`io.Copy`、`http.ServeFile` 全部回到 handler。
- 出站 SM.MS 客户端抽出 `internal/imghost` 包（适配器，零领域知识），`services` 包至此**无任何传输符号**；`_ = ctx` 消失且 ctx 真正被使用。
- **R1 口径修正**（写在报告 §7.1，不是调低门槛）：旧口径数"import net/http 的文件数"=11，逐个核对发现 10 个只用了 `http.StatusXxx` 整数常量——换自定义常量等于改名、零收益。新口径数真正耦合传输的符号，P1 前 12 处 → **0**。剩余 11 个文件记为观察项 R1b（不阻断），语义化改造列为可选 AR-1b。
- 测试迁移：`services/upload_test.go` → `imghost/imghost_test.go`，断言不变，另加 `TestUploadTimeoutIsBounded`。
- 真跑冒烟（7 组）：上传→取回字节一致 + `image/png`；`Range` → 206；`If-Modified-Since` → 304；非图片 → 400 白名单；缺字段 → 400；未授权 → 401；`/uploads/../go.mod` 等三种穿越 → 全 404；备份下载 200 + `Content-Length` 与实际一致 + `Content-Disposition` 正确；备份名穿越三种 → 全 404。
- 顺带发现既有 bug（记为 AR-1c）：`MaxBytesReader` 在 `ParseMultipartForm` 阶段就截断，导致 413 分支是**死代码**，超限用户看到的是 400 `invalid multipart form`。P1 只搬移未改语义，保持行为一致。

### 第五十六轮 — P2：审计单点化（2026-10-23）

- 新建 `internal/services/audit.go`：`func (s *Service) audit(ctx, actor, action, target, detail string)`，是**全库唯一** `models.ActionLog` 构造点（R10 门禁拦其余文件）。配套 `WithActor`/`ActorFromContext`/`ClientIPFromContext`/`SystemActor`——「谁在动、从哪来」经 context 从 JWT 中间件传给 service，service 不再接触 `*http.Request`。
- handler 侧 18 处 `recordAudit` 全部下沉为 service 写方法内的 `s.audit(...)`；`recordAudit` 与 `RecordAction` 双轨删除，统一 35 处 `s.audit` 调用。
- 8 个审计缺口全部补齐：分类增删改 / 上传 / 删图 / 发文章 / 预设分类 / 恢复修订 / 批量操作 / 导入 Markdown。
- 审计 best-effort（插入失败仅 `slog.Warn`，不破坏主操作）；刻意不审 Login/Register/UpdateMe（Login 已有 `login_log`），改密（admin 强制 + 自助）例外都审。
- 回归 `internal/services/audit_test.go` 3 用例：AC-1 调用写用例即产生审计行；AC-2 显式 actor 优先于 context；AC-3 无请求身份（cron）也写行且 actor/IP 为空。
- 验证：`go build/vet/test` 全绿（8 个测试包）；`arch_check.py` R5 18→0、R10=0、R3 45→44，`--update` 锁定基线；`docs/TODO.md` AR-2 勾选完成。

### 第五十七轮 — P3：拆 services.go（2026-10-23）

- **纯移动，不改行为**：`services.go`（2,671 行）消失，拆成 15 个域文件（`service.go` 核心 / `dto.go` 25 个 DTO / `auth.go` / `posts.go` / `posts_list.go` / `posts_edit.go` / `moderation.go` / `tags.go` / `taxonomy.go` / `nav.go` / `users.go` / `backup.go` / `restore.go` / `trash.go` / `media.go`）。函数体零改动，用一次性脚本按行号精确切分 + gofmt，`go build/vet/test` 全绿即证明无符号丢失。
- 顺带拆掉另两个超长文件：`accounts.go`（749→313）→ `site_config.go`（站点配置+登录记录）+ `accounts.go`（用户 CRUD）+ `accounts_profile.go`（资料/作者/仪表盘）；`mail.go`（420→257）→ `mail.go`（高层）+ `mail_smtp.go`（SMTP 底层协议）。
- 结果：services 包最大文件 314 行，**全部 ≤350 行**；`arch_check` **R2.files 4→1**（仅剩 `config.go` 1023 行，属 config 包，不在 AR-3 范围）。R1b 11→26 为观察项（文件增多自然增加「import net/http 的文件数」，只显示不阻断）。
- 顺手修 `dto.go` 一处错位注释（`UserPublic` 前误挂的 `CategoryWithCount` 注释）。
- 验证：`go build/vet/test` 全绿（172 个 `Test*` 零改动）；`doc_check` PASS；`arch_check` R2.files 4→1 并 `--update` 锁定；`docs/TODO.md` AR-3 勾选完成。

### 第五十八轮 — P4：路由表声明化（2026-10-23）

- **路由表集中声明**：`internal/router/router.go` 的 76 条路由改成 `var routes = []route{{method, path, auth, roles, handler}}`，按域分组。`auth` 三档（`authPublic`/`authRequired`/`authOptional`），`roles` 用 `adminRoles`（admin|root）/ `rootRoles`（root）；`/`、`/health`、login（带 loginLimiter）保留为特殊注册。
- **权限上移**：新增 `handlers.RequireRole(roles...)` 中间件（context 读 actor 角色，放行或 403），删 handlers 里 30 处 `requireAdmin` + 9 处 `requireRoot` 及两个定义——**R4 39→0**。root-only 报 `"root only"`、其余报 `"admin only"`（消息不变）。9 个 root-only 端点（ban/banned/send-back/delete-banned/import-backup/restore-snapshot/snapshots/auto-backup）由表声明。
- **纵深防御不变**：service 层 `IsRoot`/`IsAdminOrRoot`/`canReviewPost`/`canManageUser` 仍逐用例校验；`UpdateSiteConfig` 的「非 root 不能改 AutoBackup」字段级授权保留在 handler（非路由守卫）。现有越权测试（`TestBanPostRequiresRoot`/`TestListBannedHiddenFromAdmin` 等）不改仍全绿；新增 `guard_test.go` 6 用例锁定 `RequireRole`。
- **顺带修 `contract_check.py`**：解析路由表格式 + 两处既有误报（`importMarkdown` 裸 fetch 默认 GET、`archive/paged` 嵌套反引号），「方法不匹配」恢复 0。
- 验证：`go build/vet/test` 全绿（173 个 `Test*`）；`contract_check` 方法不匹配 0；`doc_check` PASS；`arch_check` **R4 39→0、R2.funcs 17→16** 并 `--update` 锁定；`docs/TODO.md` AR-4 勾选完成。

---

### 第五十九轮 — P5：迁移注册表（2026-10-23）

- **迁移从「9 段克隆」收敛为「1 张表 + 1 个泛型」**：新建 `internal/config/migrate.go`，`var migrations = []migration{{version, up}}` 有序注册表（10 条）；`runManualMigrations` 循环调用，`apply` 集中门禁（原每个函数各抄一遍的「查 gate → 判表 → 判列 → ALTER → record」删除）。8 个 `addXxxColumn` 收敛成泛型 `ensureColumns(model, cols, after...)`（`after` 跑幂等 index/backfill）；`dropScheduledAt`/`promoteFirstAdminToRoot` 保留具名 `up`（去掉各自 gate+record）。`config.go` 1023 → 557 行（迁移代码拆出），`migrate.go` 250 行。
- **新增 `arch_check.py` R7 门禁**：`add*Column` 克隆函数计数（目标 0，已归零锁基线）。漂移检查 `checkSchemaDrift` 本就遍历 `models.All()`（P0 已改），无手写清单。
- **顺带修一个真 bug**：双库冒烟（新库 + 拷贝 `app.db.bak-20261014` 旧库各跑一次 `InitDB`）发现 `users.token_version` **自 v0.1.0 起就在模型里、却从未写过迁移**，旧库登录/登出会 `no such column` 500（漂移检查 WARN 早已提示）。加 `0011_add_token_version` 一条注册表项即修复——正是「新增列只需 2 行」的实证。冒烟后新库=旧库=89 列、零缺列、零真类型差异（`VARCHAR≡TEXT`、`bool→numeric≡INTEGER` 均为 SQLite 亲和性别名，良性）。
- 验证：`go build/vet/test` 全绿（**176 个 `Test*`**：`mig_test.go` 重写 `TestDropScheduledAt` 纯 up + 新增 `TestApplyMigrationGate`/`TestRunManualMigrationsFresh`/`TestEnsureColumns`）；`arch_check` R7=0 并 `--update` 锁定；`docs/TODO.md` AR-5 勾选完成。

---

### 第六十轮 — P6：前端数据层（2026-10-23）

- **唯一网络出口 `core/http.ts`**：`ApiError` + `setTokenGetter`/`setOnUnauthorized` + 单 `fetch`（`fetchRaw`，含 429 退避）+ `request`/`upload`/`download` 三基元 + `errMessage`/`errStatus`（全库仅此 2 处 `instanceof ApiError`）。`api.ts` 改成纯类型化 facade：删内部 3 处裸 `fetch`（`importMarkdown`/`downloadBackup`/`uploadImage` 改走 `upload`/`download`），re-export `ApiError`/`setTokenGetter`/`setOnUnauthorized`/`errMessage`/`errStatus` 保持 `auth.tsx` 与页面兼容。`downloadBackup` 因此第一次拿到 429 退避（之前是手抄 fetch 副本漏了）。
- **新增 `core/useQuery.ts`**：`useQuery(fn, deps) → {data,loading,error,reload,setData}`（`setData` 供乐观更新）+ `useMutation`（busy 标志、rethrow 保留调用方错误处理）。零依赖、含竞态防护。
- **错误提取去重**：23 个文件里 54 处 `e instanceof ApiError ? e.message : t('…')` 统一为 `errMessage(e, t('…'))`，`e.status === 401/404` 统一为 `errStatus(e) === …`，复合三元（改密 401 分支）一并收敛。
- **旗舰页示范**：`PersonalCenterPage` 623 → **52 行**——三个区块拆到 `pages/personal/{MyHomepage,ContentManage,SecurityCenter}.tsx`（共享 `Pager`+`PAGE_SIZE`），列表三态机换成 `useQuery`（乐观更新走 `setData`）。
- 验证：`tsc --noEmit` + `vite build` 全绿（产物 233.35→231.99 kB）；`arch_check` **R8 3→0**（`fetch` 只剩 `http.ts` 一处）并 `--update` 锁定；`contract_check` 方法不匹配 0（顺带修解析器识别 `upload()`/`download()`，3 端点恢复）；`docs/TODO.md` AR-6 勾选完成。

---

### 第六十一轮 — P6 收尾：useQuery 铺开到管理页（2026-10-23）

- **扩展 `useQuery` 成「读三态机」的 superset**（向后兼容，旗舰页零改动）：新增 `UseQueryOptions{enabled, onError}` 和 awaitable `reload`（`() => Promise<void>`）。`enabled=false` 时不发请求（覆盖管理页的 role guard）；`onError` 覆盖「toast-on-error」形态（区别于旗舰页的 error 态 + retry）；`reload` 直接 fetch 并返回 Promise，`await load()` → `await reload()` 语义保留。内部用 `fnRef`/`optsRef` 持有最新闭包，`reload` 身份稳定。
- **迁移 `AdminPostsPage`（353→343 行）**：`load` 回调三态机 → `useQuery(enabled:isAdmin, onError:toast)`；4 处 mutation 后 `await load()` → `await reload()`；清空选择 effect 依赖去掉 `load`。
- **迁移 `AdminUsersPage`（525→520 行）**：同上；`users = data ?? []`（原初始 `[]`），清空选择 effect 与 `allSelected` 依赖从 `users` 改 `data`（避免 `data ?? []` 每次 render 新引用导致 effect 空转）。
- 验证：`tsc --noEmit` + `vite build` 全绿（产物 231.99→232.18 kB，useQuery 扩展的极小增量）；`arch_check` R8=0 不变、`contract_check` 方法不匹配 0；`instanceof ApiError` 代码仍 2 处、裸 `fetch` 仍唯一（`http.ts` 内）。
- **剩余**：还有 10 个页面手写三态机——6 个管理页（`AdminBanVault`/`AdminCategories`/`AdminDashboard`/`AdminReview`/`AdminTags`/`AdminTrash`，与 `AdminUsers` 同构：`load`+toast+`TableState`）+ 4 个读者页（`Archive`/`Author`/`List`/`Read`，形态更复杂：`List` 有 401 跳转、`Read` 有并行请求）。下一增量批量铺开。

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

## 七、编译验证基线（第 39 轮重测，2026-10-22）

| 检查项 | 结果（本机实测，命令与退出码可复现） |
|--------|------|
| `go build ./...` | ✅ exit 0 |
| `go vet ./...` | ✅ exit 0 |
| `go test -count=1 ./...` | ✅ 全绿（services 8.1s / handlers 7.3s / config 1.5s / middleware 1.8s / models 1.1s；`cmd/server`、`router`、`clientip` 见下注） |
| `npx tsc --noEmit` | ✅ exit 0 |
| `vite build` | ✅ 64 模块；JS 213.68 kB / gzip 65.72 kB，CSS 58.61 kB / gzip 10.97 kB（2.59s） |
| 规模基线 | 后端 54 个 `.go`（含 4 个本轮新增）/ 12,446 行；前端 44 个 `.ts(x)` / 8,629 行 / 22 页面；测试 31 个文件 / **133** 个 `Test*` 函数 |
| 依赖基线 | 后端 6 个直接依赖；前端 3 个（`preact` / `wouter` / `marked`） |
| 契约基线 | 后端 **65** 条路由，前端调用 **58** 条，`scripts/contract_check.py` 报「方法不匹配：无」 |
| 文档一致性 | `scripts/doc_check.py` PASS：**57** 份文档、19 个顶层文档全部被母文档点名、5 个批量目录已索引、57/57 带 A/B 横幅 |
| 已知局限 | 未 docker build（开发机无 Docker）；未在真实 VPS 端到端跑；UI 实机交互未验证（后端进程会被会话回收）。`go test ./internal/clientip/` 本机报 `fork/exec: Access is denied`，需 `go test -c -o bin/clientip.test.exe` 后再跑 |

---

## 八、版本化与工程底座（2026-10-20）

用户决定第一阶段到此为止、进入真实测试/beta 阶段，并以 **`v0.1.0`** 作为起始版本号（0.x = 接口可能变更，给自己留"改主意"余地）。

- `git init -b main`；本仓库本地身份 `Lopez` / `lopez@local`（**未改全局配置**）。
- 新增 `.gitattributes`：仓库级 `core.autocrlf=false` + `* text=auto eol=lf`，`.sh` 强制 LF、`.bat/.ps1` 保留 CRLF——解决 Windows 开发 → Linux 容器构建的行尾坑（否则 shebang 带 `\r` 容器起不来）。
- 新增 `.editorconfig`：Go=tab，前端/JSON/YAML/MD=2 空格，UTF-8，LF。
- 补 `.gitignore`：`logs/` `*.log` `backend.log` `web/uploads/` `.workbuddy/`。
- 首次提交 `8458e36`（170 文件，无真实密钥 / 无构建产物）；打 annotated tag **`v0.1.0`**（msg: stage 1 complete, enter testing, 0.x APIs may change）。
- 验证：`git ls-files --eol deploy/*.sh` = `i/lf w/lf`；工作树 clean；两个 `*.env` 部署模板（仅 `CHANGE_ME` 占位符）随库提交，真实密钥走运行时注入或被忽略的 `.env`。
- **远程已配置**（第 39 轮核实，本条此前写的"无远程"已过期）：`origin` = `https://github.com/RioLopez/lopezpress.git`（私有仓库），本地 `main` 追踪 `origin/main`，远程 HEAD 与本地一致（`17d7760`）。本机 `git fetch/push` 走 schannel 报 `CRYPT_E_REVOCATION_OFFLINE`、走 openssl 又遇 `credential.helper=manager` 链路失效，需用 GCM 里存的令牌临时拼 `http.extraheader` 才能联网；`api.github.com` 直连可用，`github.com:443` 的 git TLS 被网络阻断。**2026-10-22 已用 `github-auto-upload` 技能（`push_mode=auto` → 自动 fallback 到 Git Data API）推送成功，远程 `main` = 本地 `5c23676`，仓库确认为 private。** 结论：本机后续推送**优先走该技能**，不要手搓 `git push`（直连不通）。凭据记录在仓库外的 `~/.workbuddy/CREDENTIALS.md`。

**版本号策略**：beta 阶段 `0.x.y`（如 0.1.0 → 0.2.0 …）；临近生产升 `1.0.0-beta.N` → `1.0.0-rc.N` → `1.0.0`。每次发版打 `vX.Y.Z` tag，发版前跑全量验证门禁（go build/vet/test + tsc + vite build）。
