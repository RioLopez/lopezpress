> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 阅读页（`/read/:id`）

> 状态：Done（代码已实现，本 spec 首次补写；第 15 轮加入 TOC；第 16 轮加入滚动高亮 + 回顶 + 灯箱）
> 路由：`/read/:id`
> 权限：登录（`Guarded`；公开阅读走 `GET /api/posts/:id/public`，见 8）
> 轮次：第 1、8、10、12、15、16 轮
> 最后更新：2026-09-21

## 1. 目标

渲染单篇文章的 Markdown 正文，提供舒适的阅读体验（字号 / 进度条 / 目录占位 / 代码复制 / 上下篇），并允许作者编辑 / 删除自己的文章。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为读者，我要舒适的正文排版与阅读进度。
  - **AC-1.1**：WHEN 文章加载完成 THEN 顶部进度条随滚动 0→100% 增长
  - **AC-1.2**：WHEN 选择字号档位（小 / 中 / 大 / 特大）THEN 正文 `.read-page--<size>` 切换且写入 `localStorage['echo.readFont']`
  - **AC-1.3**：WHEN 点「重置」THEN 字号回 `medium`
  - **AC-1.4**：WHEN 元信息栏 THEN 显示作者 / 创建时间 /（更新时）更新时间 / 字数 / 浏览量 / 预估阅读时长
- **US-2**：作为读者，我要复制代码块。
  - **AC-2.1**：WHEN 点某代码块复制按钮 THEN `navigator.clipboard`（不可用时 `execCommand` 兜底）复制成功，按钮文案变「已复制」1.5s 后还原
- **US-3**：作为读者，我要翻页读相邻文章。
  - **AC-3.1**：WHEN 点上一篇 / 下一篇 THEN 跳对应 `/read/<id>`；无相邻时按钮禁用
- **US-4**：作为作者，我要编辑 / 删除自己的文章。
  - **AC-4.1**：WHEN `user.id === post.author_id` 或 `role === admin` THEN 显示编辑 / 删除按钮，否则隐藏
  - **AC-4.2**：WHEN 删除成功 THEN toast + 回 `/`；失败 THEN 错误 toast
- **US-5**：作为读者，我要通过目录（TOC）跳转长文各章节（第 15 轮加入）。
  - **AC-5.1**：WHEN 正文有 ≥ 1 个 `##`/`###` 标题 THEN 渲染 TOC 区块（置于正文前，列出全部 `##`/`###` 标题，`###` 相对 `##` 缩进一级）
  - **AC-5.2**：WHEN 点某 TOC 项 THEN 平滑滚动到对应标题（锚点定位，不触发整页重载）
  - **AC-5.3**：WHEN 正文无 `##`/`###` 标题 THEN 不渲染 TOC 区块（不出现空目录）
  - **AC-5.4**：WHEN 两个标题文本相同 THEN 生成唯一锚点（重复名追加 `-2`/`-3`），TOC 链接与各标题 `id` 一一对应不冲突
  - **AC-5.5**：WHEN 移动端（< 768px）THEN TOC 区块折叠为可展开/收起（默认收起），桌面端默认展开
- **US-6**：作为读者，我要在滚动长文时看到当前阅读到哪个章节（滚动高亮，第 16 轮加入）。
  - **AC-6.1**：WHEN 正文含 ≥1 个可锚定标题（h2/h3）THEN 随滚动实时高亮当前所在章节对应的 TOC 项（`.read-toc-item--active`；TOC 折叠时仍可高亮打开的项）
  - **AC-6.2**：WHEN 滚动到页面顶部（无任何标题越过视口顶部）THEN 无 TOC 项被高亮（回到起始态）
- **US-7**：作为读者，我要一键回到顶部（第 16 轮加入）。
  - **AC-7.1**：WHEN 向下滚动超过一屏（> 400px）THEN 右下角浮现「回到顶部」悬浮按钮；回顶后自动隐藏
  - **AC-7.2**：WHEN 点该按钮 THEN 平滑滚动回页面顶部
- **US-8**：作为读者，我要查看被正文插图放大的清晰版（图片灯箱 + 懒加载，第 16 轮加入）。
  - **AC-8.1**：WHEN 正文有 `<img>` THEN 图片带 `loading="lazy"`（首图除外）
  - **AC-8.2**：WHEN 点某正文图片（且有效 src）THEN 打开灯箱（全屏遮罩 + 原图居中）；按 Esc 或点遮罩 THEN 关闭灯箱

## 3. 前端

- **布局**（ASCII）：

```
┌──────────────────────────────────────────┐
│ 顶栏（Layout）                           │
│ 阅读进度条（read-progress，sticky）      │
├──────────────────────────────────────────┤
│ 正文（read-page--<font>，居中 680px）    │
│  read-head：徽章 / 标题 / 元信息 / 字号  │
│  toc（可折叠，桌面默认展开）             │
│  read-body.md-body：渲染后 Markdown      │
│  read-footer：上一篇 / 下一篇 / 回列表    │
└──────────────────────────────────────────┘
```

- **组件树**：`Layout > ReadPage > (read-head / toc / read-body / read-footer / lightbox / back-to-top)`
- **关键交互**：
  - `getPost(id)` 成功 → 渲染；`404` → toast「未找到」+ 回 `/`；其他 → toast「加载失败」+ 回 `/`（SPA 跳转，15 轮已修）
  - 正文 `renderMD` 经 DOMParser 消毒；渲染后给每个 `h2`/`h3` 生成 slug（重复名追加 `-2`/`-3`）并写入 `id`，与 TOC 条目一一对应；代码块**命令式**注入复制按钮（`useEffect` 包裹每个 `pre` 进 `.code-copy-holder`，卸载时移除）；正文图片**命令式**加 `loading="lazy"`（首个除外）+ 可点击打开灯箱（`useEffect` 统一捕获）
  - TOC 由正文 Markdown 解析 `##`/`###` 标题生成；点击平滑滚动到锚点（`scrollIntoView` + `scroll-behavior`），`###` 相对 `##` 缩进一级；无 `##`/`###` 则整块不渲染
  - 滚动高亮（第 16 轮）：`scroll` 监听计算当前越过视口顶部的最近 h2/h3 标题 id → 对应 TOC 项加 `.read-toc-item--active`
  - 回到顶部（第 16 轮）：`scroll` 监听 `scrollTop > 400` 显示右下悬浮按钮；点击 `scrollTo({top:0,behavior:'smooth'})`
  - 灯箱（第 16 轮）：点正文图 → `lightboxSrc` state 打开遮罩 + 原图；Esc / 点遮罩关闭；滚动锁
  - 字号 / 进度条 / 代码复制 / TOC 展开态 / 滚动高亮 / 回顶 / 灯箱均为前端本地状态，不触发网络请求
- **路由**：`/read/:id`，参数 `id`
- **i18n 键**：`read.*`、`post.*`、`list.*`、`common.*`
- **共享组件依赖**：`Layout` / `i18n` / `theme` / `Toast` / `md.ts`（`renderMD` / `wordCount`）

## 4. 后端

- **API 端点**：
  - `GET /api/posts/<id>` → `Post`（含作者 / 分类 / 标签），404 若不存在或不可见
  - `GET /api/posts/<id>/nav` → `PostNav { prev, next }`（按发布时间排序的相邻文章）
  - `DELETE /api/posts/<id>` → 软删（进回收站）
  - `GET /api/posts/<id>/public` → 公开只读版本（未来"游客可读"用，当前阅读页仍走登录态 `getPost`）
- **Service 方法**：`svc.GetPost` / `svc.GetPostPrevNext` / `svc.DeletePost`
- **数据模型**：`posts`（`body`、`pinned`、`view_count`、`created_at`、`updated_at`、`deleted_at`）
- **权限**：查看需本人或 admin；删除需作者 / admin
- **错误处理**：404＝文章不存在或已软删 / 不可见

## 5. 数据流

```
进入 /read/<id> → getPost(id) → 渲染正文（h2/h3 生成 id）→ 解析 TOC
  └ getPostPrevNext(id) → 上/下篇（失败静默）
滚动 → onScroll 计算 progress（本地）
点字号/复制/TOC → 仅 localStorage / clipboard / 锚点（本地）
点编辑 → navigate('/edit/<id>')；点删除 → deletePost → 回 '/'
```

- **可能失败点**：`getPost` 404（跳回列表）；`getPostPrevNext` 失败（静默，上/下篇显示为无）
- **回退**：导航接口失败不影响正文展示；正文无 h2/h3 时 TOC 不渲染（非错误）

## 5.5 性能与资源

- **前端包体积**：复用 `renderMD`；代码复制命令式注入无额外依赖；TOC/滚动高亮/回顶/灯箱均原生 API（`IntersectionObserver` 不引入、滚动高亮用 `scroll` 事件节流），无新依赖；影响 < 5 kB
- **后端查询量级**：单篇文章 + 1 次相邻查询，O(1)
- **内存 / 磁盘**：纯读；浏览数 `view_count` 每次打开 +1（写量低）；TOC 解析结果仅驻留内存（文章标题数量级，< 10 kB）；灯箱复用原文 URL，不额外加载副本（首图无 lazy）
- **慢点在哪**：长文章 `renderMD` 一次渲染（无虚拟滚动，正文 > 5 万行时卡顿）；滚动高亮在长文 + 高频滚动下多次 `getBoundingClientRect`（已节流，^60ms）
- **是否需重新评估**：正文极大（> 10 万字）时考虑分段渲染；滚动高亮在超长文仍卡顿则换 `IntersectionObserver`

## 6. 边界情况

- 404 / 不可见：toast + 回列表
- 加载中：占位
- 空正文：显示「正文为空」
- 非作者且非 admin：隐藏编辑 / 删除
- 代码块：无 `code` 子节点时用 `pre.textContent`
- 正文只有 `#`（h1）无 `##`/`###`：不渲染 TOC，无滚动高亮目标
- 重复标题文本：锚点去重（`-2`/`-3` 后缀）
- 滚动高亮：无标题越过视口顶部 → 无 active 项；多标题同屏 → 高亮最近越过的那个
- 回顶按钮：`scrollTop <= 400` 隐藏；移动端右下方悬浮不遮挡正文
- 灯箱：图片 src 为空/无效 → 不可点击；灯箱打开时滚动锁定，Esc/点遮罩关闭；路由跳转/卸载 → 重置灯箱状态

## 7. 已知限制 / 未来方向

- TOC 只解析 `##`/`###`（第 15 轮已实现）；候选＝识别 `####` 及更深层级
- 滚动高亮已实现（第 16 轮）；候选＝移动端简化 / `IntersectionObserver` 重构
- 图片 lazy + 灯箱已实现（第 16 轮）；候选＝灯箱左右切换（多图）、图片标题
- 无 SEO meta（`<title>` / Open Graph）；候选＝后端注入 3 行 meta
- 候选：阅读页"回到顶部"悬浮按钮（第 16 轮已实现）

## 8. 相关

- 依赖 spec：`specs/03-editor.md`（编辑跳转目标）
- 相关决策：无
