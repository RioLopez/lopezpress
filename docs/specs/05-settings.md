# 站点配置（`/admin/site`）

> 状态：Draft（第 19 轮重写，待实现；取代旧版仅 SM.MS token 的设定）
> 路由：`/admin/site`
> 权限：admin
> 轮次：第 19 轮
> 最后更新：2026-09-22

## 1. 目标

把"站点级配置"集中到后台一个页面统一管理，并落库到后端（不再存浏览器 localStorage）：
站名、浏览器标题、Logo、favicon、页脚（版权 / 备案号 / 友情链接）、图床模式与 token、
以及公开注册开关。前端通过**公共只读接口**把站名/Logo 渲染到页眉左上角、把标题渲染到浏览器
`<title>`、把页脚渲染到每个页面底部；管理员通过**受保护接口**读写完整配置（含 token / 开关）。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要集中配置站点信息，以便全站统一展示。
  - **AC-1.1**：WHEN 打开 `/admin/site` THEN 回显已保存配置（站名 / 标题 / Logo / favicon / 页脚 / 备案 / 友链 / 图床 / 注册开关）
  - **AC-1.2**：WHEN 保存 THEN `PUT /api/admin/site-config` 写入 `site_config` + 成功 toast「已保存」
  - **AC-1.3**：WHEN 校验失败（如站名超长）THEN 字段错误提示，不提交
- **US-2**：作为访客，我要看到站名/Logo/标题/页脚。
  - **AC-2.1**：WHEN 任意非登录页渲染 THEN 页眉左上角显示 `site_logo` + `site_name`；`document.title` = `site_title` + 页面后缀
  - **AC-2.2**：WHEN 任意页面底部 THEN 渲染 `Footer`（版权 + `icp_no` 链接工信部 + `friend_links`）
- **US-3**：作为 admin，我要站点级图床（取代旧 localStorage token）。
  - **AC-3.1**：WHEN 配置图床模式 `local` / `smms` + token THEN 存后端 `site_config`
  - **AC-3.2**：WHEN 编辑器上传 THEN `/api/upload` 由后端按配置决定本地落盘或代发 SM.MS（**服务端持 token，浏览器不再存 token**）
  - **AC-3.3**：WHEN 清空 token 或模式=local THEN 上传回退本地
- **US-4**：作为 admin，我要控制公开注册。
  - **AC-4.1**：WHEN 切换「允许公开注册」THEN 写入 `allow_register`；详见 `specs/16-registration.md`

## 3. 前端

- **布局**（ASCII）：

```
┌──────────────────────────────────────┐
│ 侧边栏（见 06）                       │
├──────────────────────────────────────┤
│ 站点设置 admin-site                   │
│  基本信息：站名 / 标题 / Logo / favicon│
│  页脚：版权文本 / 备案号 / 友情链接(增删)│
│  图床：模式(local/smms) / token        │
│  注册：允许公开注册(开关)             │
│  [保存]                               │
└──────────────────────────────────────┘
```

- **组件树**：`AdminLayout > AdminSitePage > (field / friend-links-editor / toggle)`
- **关键交互**：
  - 挂载 `GET /api/admin/site-config` 回显完整配置（含 token / 开关）
  - Logo / favicon 走 `/api/upload` 上传后回填路径
  - 友链为动态列表（name + url，可增删）
  - 保存即 `PUT /api/admin/site-config` + toast；`busy` 锁按钮
- **路由**：`/admin/site`（admin）
- **i18n 键**：`site.*`
- **共享组件依赖**：`AdminLayout` / `i18n` / `theme` / `auth` / `Toast` / `Footer` / `SiteBrand`

## 4. 后端

- **API 端点**：
  - `GET /api/site-config`（**公开**）→ 安全子集 `{ site_name, site_title, site_logo, site_favicon, footer_text, icp_no, friend_links:[{name,url}] }`（**不含** token / allow_register）
  - `GET /api/admin/site-config`（admin）→ 完整包含 `img_host_mode` / `img_host_token` / `allow_register`
  - `PUT /api/admin/site-config`（admin）→ 更新全部字段；校验站名/标题长度、友链 URL 合法
- **Service 方法**：`svc.GetSiteConfig(public bool)` / `svc.UpdateSiteConfig(cfg)`
- **数据模型**：`site_config`（**单行**，固定 `id = 1`）
  - `site_name` text、`site_title` text、`site_logo` text、`site_favicon` text
  - `footer_text` text、`icp_no` text、`friend_links` text（JSON 数组 `[{name,url}]`）
  - `img_host_mode` text（default `local`）、`img_host_token` text
  - `allow_register` bool（default `false`）、`updated_at` datetime
- **权限检查**：读公开子集无需登录；写需 `requireAdmin`
- **错误处理**：校验失败 400 + `message`；写非 admin 403

## 5. 数据流

```
管理员：打开 /admin/site → GET /api/admin/site-config → 回显
  → 编辑 → PUT /api/admin/site-config → 落库 site_config → toast
前端渲染：App 启动 GET /api/site-config → SiteBrand(页眉) + document.title + Footer(每页底)
编辑器上传：/api/upload（后端读 site_config.img_host_* 决定本地/SM.MS，服务端持 token）
```

- **可能失败点**：公开接口不含敏感字段（token/开关绝不下发前端）；SM.MS 代发失败回退本地 + 返回本地 URL
- **回退**：配置缺失 → 用默认值（站名 "Echo"、标题 "Echo"、模式 local）

## 5.5 性能与资源

- **前端包体积**：新增配置表单 + 友链编辑器，影响 < 3 kB
- **后端查询量级**：公开接口每次页面加载 1 行 `site_config` 读（可缓存到内存，配置变更失效）
- **内存 / 磁盘**：单行配置；图床无新增落盘（沿用 `data/uploads`）
- **慢点在哪**：无（单行）
- **是否需重新评估**：配置项增多时改 KV 表（候选）

## 6. 边界情况

- 首次启动无配置行：`config.go` 种子默认行（站名/标题 "Echo"，allow_register=false）
- 公开接口被未登录页调用：正常返回（无 token）
- Logo 未设置：页眉回退显示站名文字
- 友链为空数组：页脚不渲染友链区

## 7. 已知限制 / 未来方向

- 旧版 05（SM.MS token 存 localStorage）被本版取代；编辑器不再读 localStorage（更安全，消除 P3 风险）
- 图床代发 SM.MS 需后端联网（当前 `/api/upload` 仅本地）；本期扩展为服务端按配置路由
- 候选：配置项 KV 化、主题默认色、SEO 描述 / 关键词

## 8. 相关

- 依赖 spec：`specs/06-admin-layout.md`（后台外壳）、`specs/16-registration.md`（注册开关）
- 消费方：`Layout`/`SiteBrand`/`Footer`（前端渲染）、`UploadImage`（图床路由）、`Register`（注册开关）
