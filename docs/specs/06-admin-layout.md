> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 管理后台布局（左侧竖排侧边栏）

> 状态：Draft（第 19 轮重写，待实现；取代旧版顶栏 5 标签布局）
> 路由：`/admin`（仪表盘）、`/admin/*`
> 权限：登录 + admin（`Guarded` + 各页内 `user.role === 'admin'` 校验）
> 轮次：第 19 轮
> 最后更新：2026-09-22

## 1. 目标

把分散的 `/admin/*` 顶栏 5 标签，重构为**独立后台页 + 左侧竖排侧边栏**：侧边栏放站名/Logo（顶）、
导航（中）、账户信息（底）；内容区保留一条细顶栏放面包屑 + 主题/语言 + 返回内容。所有后台管理
收进侧边栏，并新增**仪表盘首页**。行业调研（AdminLTE / Ghost / WordPress / 国内 CMS 后台）均确认
CMS 类后台用左侧竖排导航最合适（功能多、导航深、扩展性强）。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要进入独立后台、左侧竖排导航。
  - **AC-1.1**：WHEN 访问 `/admin` THEN 渲染 `AdminLayout`（左侧栏：站名+Logo 顶 / 导航中 / 账户底；内容区细顶栏）
  - **AC-1.2**：WHEN 进入 `/admin/*` THEN 侧边栏常驻，不随页面切换消失
- **US-2**：作为 admin，我要用侧边栏切换管理页。
  - **AC-2.1**：WHEN 点导航项 THEN 跳转对应 `/admin/xxx`，当前项高亮（`nav-link--active`）
  - **AC-2.2**：WHEN 点账户区「返回内容」THEN `navigate('/')`
- **US-3**：作为 admin，我要仪表盘首页概览。
  - **AC-3.1**：WHEN 打开 `/admin` THEN 显示统计卡片（文章总数 / 总浏览量 / 最近文章 / 最近登录）
- **US-4**：作为非 admin，我被拦截。
  - **AC-4.1**：WHEN 非 admin 访问 `/admin/*` THEN 前端占位「无管理员权限」+ 后端 `requireAdmin` 403

## 3. 前端

- **布局**（ASCII）：

```
┌──────────┬───────────────────────────┐
│ 站名+Logo │ 细顶栏: 面包屑  主题 语言 │
│ ───────  │ ───────────────────────── │
│ 仪表盘    │ 内容区 (各 admin 页)      │
│ 内容      │  (仪表盘卡片 / 表格 / 表单)│
│  分类     │                           │
│  标签     │                           │
│ 用户管理  │                           │
│ 站点设置  │                           │
│ 备份      │                           │
│ 回收站    │                           │
│ ───────  │                           │
│ 头像 名   │                           │
│ 角色 退出 │                           │
└──────────┴───────────────────────────┘
```

- **组件树**：`AdminLayout`（sidebar + content `Outlet`）> `AdminDashboardPage` / 各 `Admin*Page`
- **侧边栏导航项**：
  - 仪表盘 `/admin`
  - 内容 → 分类 `/admin/categories`、标签 `/admin/tags`（分组可折叠）
  - 用户管理 `/admin/users`
  - 站点设置 `/admin/site`（图床为其内的一个 section，不再单列导航）
  - 备份 `/admin/backup`、回收站 `/admin/trash`
- **账户区**（侧边栏底）：头像 + 用户名 + 角色徽章 + 「返回内容」+「退出」
- **内容细顶栏**：当前页面包屑 + 主题(亮/暗) + 语言(中/EN)
- **路由**：`/admin`（仪表盘）、既有 `/admin/*` 保留；`main.tsx` 增加 `AdminLayout` 包裹
- **i18n 键**：`admin.*`、`nav.*`
- **共享组件依赖**：`i18n` / `theme` / `auth` / `Toast` / `Footer`（后台内容区底部亦可显示站点页脚）

## 4. 后端

- **API 端点**：`GET /api/admin/dashboard`（admin）→ `{ posts_total, views_total, recent_posts, recent_logins }`
- **Service 方法**：`svc.GetDashboard(ctx)`
- **数据模型**：无新增（聚合查询 `posts` / `login_log`）
- **权限检查**：`Guarded` 保证登录；各页 `user.role === 'admin'` + 后端 `requireAdmin`
- **错误处理**：非 admin → 前端占位 + 后端 403

## 5. 数据流

```
内容页点「管理」→ navigate('/admin') → AdminLayout 渲染侧边栏 + 仪表盘
  → 点导航 → navigate('/admin/xxx') → 切换各 admin 页（侧边栏常驻）
```

- **可能失败点**：无（布局 + 聚合查询）
- **回退**：非 admin → 占位文案

## 5.5 性能与资源

- **前端包体积**：`AdminLayout` + 仪表盘卡片，影响 < 2 kB
- **后端查询量级**：仪表盘 2~3 个聚合查询（COUNT posts、SUM views、最近登录 LIMIT）
- **内存 / 磁盘**：无
- **慢点在哪**：文章到万级时 COUNT/SUM 仍可接受（无 JOIN）
- **是否需重新评估**：导航项超过 10 个时分组 / 折叠（已用分组）

## 6. 边界情况

- 非 admin 访问：占位「无管理员权限」
- 未登录：`Guarded` 跳 `/login`
- 移动端（<960px）：侧边栏折叠为抽屉（汉堡），内容区全宽

## 7. 已知限制 / 未来方向

- 取代旧版「顶栏 5 标签」布局（spec 06 旧版 Done → 本版重写为侧边栏，待实现）
- 无管理后台搜索（候选，低频）
- 候选：侧边栏分组持久化折叠状态

## 8. 相关

- 依赖 spec：各 `specs/07~16`（后台子页）
- 消费方：`SiteBrand` / `Footer`（站名/Logo/页脚来自 `specs/05-settings.md`）
