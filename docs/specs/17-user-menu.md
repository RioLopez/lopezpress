> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 右上角用户菜单

> 状态：Draft（第 20 轮新增）
> 路由：无独立页面；触发区位于内容页顶栏右上角（`Layout` > `UserMenu`）
> 权限：登录后显示菜单；未登录显示「登录 / 注册」入口
> 轮次：第 20 轮
> 最后更新：2026-09-22

## 1. 目标

在内容页右上角提供「头像 + 名称」的统一入口，聚合个人中心、账号设置、退出登录三类高频操作；同时作为未登录态的「登录 / 注册」入口。PC 端以 hover 浮层为主，移动端（本期不实现交互，仅预留结构）改用点击展开 + 底部抽屉。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为已登录用户，我要从右上角快速进入个人相关功能。
  - **AC-1.1**：WHEN 页面右上角渲染 THEN 显示当前用户头像 + 名称
  - **AC-1.2**：WHEN 鼠标悬停头像/名称区域 THEN 浮层菜单展开（含短暂关闭延迟，便于移入菜单）
  - **AC-1.3**：WHEN 鼠标移出触发区与浮层 THEN 菜单自动关闭（带延迟）
  - **AC-1.4**：WHEN 点击页面其他区域或按 Esc THEN 菜单关闭
  - **AC-1.5**：WHEN 鼠标左键点击头像/名称 THEN 进入个人中心 `/me`
- **US-2**：作为用户，我要从菜单执行三个核心操作。
  - **AC-2.1**：WHEN 菜单展开 THEN 至少包含「个人中心 / 账号设置 / 退出登录」
  - **AC-2.2**：WHEN 点击「个人中心」THEN 跳转 `/me`
  - **AC-2.3**：WHEN 点击「账号设置」THEN 跳转 `/settings`
  - **AC-2.4**：WHEN 点击「退出登录」THEN 二次确认后登出并跳回首页 `/`
- **US-3**：作为键盘用户，我要无障碍操作菜单。
  - **AC-3.1**：WHEN 触发区获得焦点 THEN 可按 Enter/Space 展开菜单
  - **AC-3.2**：WHEN 菜单展开 THEN 方向键在菜单项间移动，Enter 激活，Esc 关闭
  - **AC-3.3**：WHEN 菜单项渲染 THEN 有清晰焦点态（`:focus-visible`）
- **US-4**：作为访客，右上角不应暴露菜单。
  - **AC-4.1**：WHEN 未登录 THEN 右上角显示「登录 / 注册」并跳转 `/login`
- **US-5**：作为用户，我要看到健壮的异常态。
  - **AC-5.1**：WHEN 头像加载失败 THEN 回退为默认头像或名称首字母
  - **AC-5.2**：WHEN 名称过长 THEN 截断显示，悬停查看完整名称（`title`）

## 3. 前端

- **布局**（ASCII）：

```
┌──────────────────────────────────────────────┐
│ 品牌      导航…           主题 语言 [头像 名称] │  ← 顶栏右上角
│                                     ┌─────────┐ │
│                                     │个人中心 │ │  ← hover 浮层
│                                     │账号设置 │ │
│                                     │退出登录 │ │
│                                     └─────────┘ │
└──────────────────────────────────────────────┘
```

- **组件树**：`Layout > UserMenu`（登录）/ `LoginRegisterEntry`（未登录）
- **关键交互**：
  - hover 展开（`onMouseEnter` 立即开、`onMouseLeave` 200ms 延迟关，避免移入时闪烁）
  - 点击外部 / Esc 关闭（document 级监听，仅菜单打开时挂载）
  - 键盘：触发按钮 `Enter/Space/ArrowDown` 开菜单并把焦点移到首项；`ArrowUp/Down` 在项间循环；`Tab` 末项后关闭
  - 鼠标点击触发区 → `navigate('/me')`；键盘开菜单的同一次合成 click 被 `keyOpened` 标记吞掉，避免既开菜单又跳转
  - 退出登录：`confirm(t('menu.confirmLogout'))` 二次确认
- **路由**：无独立路由；跳转 `/me`（spec 18）、`/settings`（spec 05）、`/login`
- **i18n 键**：`menu.profile` / `menu.account` / `menu.logout` / `menu.loginOrRegister` / `menu.confirmLogout`
- **共享组件依赖**：`auth`（user/logout）、`i18n`、`router`（useNav）、`Layout`

## 4. 后端

无专属后端接口。菜单项的目标页依赖：

- `/me` → `GET /api/me/stats`（spec 18）、`GET /api/posts`（当前用户文章）
- `/settings` → `GET /api/auth/me` + `PUT /api/auth/me`（spec 15）
- 退出登录 → `POST /api/auth/logout`（无状态，仅前端丢弃 token）

## 5. 数据流

```
hover/点击触发区 → 打开 UserMenu 浮层
点击菜单项 → navigate(目标路由) / confirm → logout() → navigate('/')
```

- **可能失败点**：头像图片 404 → `<img>` onerror 回退首字母（当前用条件渲染：无 avatar 直接渲染首字母 span，规避加载失败）
- **回退**：名称缺失回退 `username`

## 5.5 性能与资源

- **前端包体积**：`UserMenu` 约 +1.5 kB（含键盘/焦点逻辑）
- **后端查询量级**：菜单本身零请求；进入 `/me` 才触发统计与列表查询（见 spec 18）
- **内存 / 磁盘**：无
- **慢点在哪**：无
- **是否需重新评估**：无

## 6. 边界情况

- 未登录：显示「登录 / 注册」入口（spec 20 §八.1）
- 头像缺失/失败：渲染首字母兜底（`.user-menu-avatar--fallback`）
- 名称过长：`max-width + text-overflow: ellipsis`，`title` 显示全称
- 浮层被父容器裁切：`.user-menu-pop` 用 `position: absolute` + `z-index: 200`，不被 `overflow:hidden` 裁切
- 移动端（<768px）：hover 不存在，触发区点击直接进 `/me`；`.user-menu-sheet` 类预留底部抽屉样式钩子（本期不实现抽屉交互）

## 7. 已知限制 / 未来方向

- 移动端底部抽屉交互本期未实现，仅预留 `.user-menu-sheet` 结构与样式钩子
- 二次确认用原生 `confirm`，后续可换为项目内 `Modal` 组件（参考 spec 09 的弹窗模式）

## 8. 相关

- 依赖 spec：`specs/18-personal-center.md`（个人中心 `/me`）、`specs/05-settings.md`（账号设置）、`specs/04-auth.md`（登录/登出）
- 相关决策：无
