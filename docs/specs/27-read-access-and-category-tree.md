> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 阅读权限开关 + 分类层级（含预设分类树）

> 状态：In Progress（第 46 轮实现）
> 决策来源：用户 2026-10-22 拍板「超级管理员可在后台配置是否需要公开或登录」「分类最好可以分层级并且由你预设」
> 关联：`docs/specs/07-admin-categories.md`（本文件扩展其数据模型）、`docs/FEATURE_DESIGN_MATRIX.md` R-10 / A-06
> 最后更新：2026-10-22

## 1. 背景

**阅读权限**：前台所有路由被 `Guarded` 包住，匿名访客一律被弹去 `/login`；但 API 侧读接口用的是 `withOptionalAuth`，**匿名其实读得到**。也就是"锁只挂在门面上，后窗是开的"——既是产品定位含糊，也是信息泄露面。本 spec 把它变成**管理员可配置的单一开关**，并且前后端一致执行。

**分类层级**：分类是单层平铺，只能把 `Linux` / `PHP` / `技术` / `教程` 并列，无法表达"系统 → Linux"这种归属。且新站没有默认分类结构，全空。

## 2. 目标与非目标

**目标**

- `site_config.public_read`（默认 **true**）：true = 匿名可读；false = 必须登录才能读。**服务端与前端同时执行**。
- 分类支持**一级父级**（`parent_id` 自引用，只允许两层：父级不能再有父级）。
- 提供**预设分类树**，一键应用；对同名已存在分类**不重建、只归位**（非破坏性）。
- 后台分类管理支持选择/更换父级，展示树形缩进；前台侧栏按树展示。

**非目标**

- 不做无限层级（三层以上会让侧栏、面包屑、筛选处处变形；两层足够覆盖个人博客）。
- 不做分类排序权重（按名称排序即可）。
- 不做拖拽排序（点击选择父级已经够用，减少实现风险）。

## 3. 数据模型与迁移

```
site_configs.public_read  INTEGER DEFAULT 1     -- 迁移 0006
categories.parent_id      INTEGER NULL          -- 迁移 0007，index
```

两张表都已存在，**必须手工 `ALTER TABLE ADD COLUMN`**（陷阱 1：已存在的表不能交给 AutoMigrate），由 `schema_migrations` 门控，并对旧行做 backfill（`public_read` 置 1）。

## 4. API 契约

| 端点 | 变化 |
|---|---|
| `GET /api/site-config` | 公开载荷新增 `public_read` |
| `PUT /api/admin/site-config` | 接受 `public_read`，落库后**立即失效**服务端缓存 |
| `GET /api/categories` | 每项新增 `parent_id`（null 表示顶级） |
| `GET /api/categories/with-count` | 同上 |
| `POST /api/categories` | 接受 `parent_id` |
| `PUT /api/categories/{id}` | 接受 `parent_id`（可改父级；`null` 表示提升为顶级） |
| `POST /api/admin/categories/preset` | **新增**：应用预设分类树，返回 `{created: N, reparented: M}` |

### 匿名读拦截

当 `public_read=false` 时，所有 `withOptionalAuth` 读端点对匿名请求返回 **401**（与"未登录"同语义），由 `WithOptionalAuth` 统一执行，不在每个 handler 里重复判断。

## 5. 预设分类树

```
技术笔记
  ├ 后端开发
  ├ 前端开发
  ├ 数据库
  └ 运维部署
系统与网络
  ├ Windows
  ├ Linux
  └ macOS
生活随笔
  ├ 折腾记录
  └ 读书笔记
```

应用规则（幂等）：
1. 父级按名称查找，不存在则创建；
2. 子级按名称查找：不存在则创建并挂到父级；**已存在且当前无父级** → 只设置 `parent_id`（不重建、不改名）；
3. 已存在且**已有别的父级** → 不动（尊重人工调整）；
4. 重复执行不产生重复行。

## 6. 用户故事 + 验收标准（EARS）

- **US-1**：作为超级管理员，我要决定站点是公开还是仅登录可见。
  - **AC-1.1**：WHEN `public_read=true` 且未登录 THEN 前台首页/归档/文章页/作者页正常渲染，不跳登录
  - **AC-1.2**：WHEN `public_read=false` 且未登录 THEN 前端跳 `/login`，且直接请求 `/api/posts`、`/api/posts/{id}`、`/api/categories` 返回 401
  - **AC-1.3**：WHEN 管理员在站点设置切换该开关并保存 THEN 无需重启，下一次请求即生效
  - **AC-1.4**：WHEN `public_read=true` THEN 匿名访客**看不到**任何写作/管理入口（写文章、编辑、删除、后台）
- **US-2**：作为站主，我要给分类分父子。
  - **AC-2.1**：WHEN 新建/编辑分类并选择父级 THEN 列表以缩进形式展示层级
  - **AC-2.2**：WHEN 选择自己或自己的后代作为父级 THEN 拒绝（400 `category cannot be its own parent`），不死循环
  - **AC-2.3**：WHEN 父级已有父级 THEN 拒绝（400 `only two levels are supported`）
  - **AC-2.4**：WHEN 删除一个有子级的分类 THEN 子级上移到被删分类的父级（不连带删除）
  - **AC-2.5**：WHEN 前台按父级分类筛选 THEN 结果包含其所有子级分类下的文章
- **US-3**：作为新站主，我要一个开箱可用的分类结构。
  - **AC-3.1**：WHEN 首次启动且分类表为空 THEN 自动写入预设树
  - **AC-3.2**：WHEN 在后台点「应用预设分类」 THEN 返回创建/归位数量，重复点击不产生重复分类
  - **AC-3.3**：WHEN 已有同名分类且无父级 THEN 仅设置父级，不重建

## 7. 边界与错误

| 情况 | 行为 |
|---|---|
| `public_read` 缺失（旧备份导入） | 视为 true（公开） |
| 切换开关后旧 JWT 仍有效 | 有效——开关只管"匿名能否读"，不吊销会话 |
| 父级 id 不存在 | 400 `parent category not found` |
| 两层限制 | 父级自身有 `parent_id` → 400 |
| 删除顶级分类且有子级 | 子级变顶级 |
| 预设分类与手工分类同名但层级不同 | 不覆盖人工调整 |
| 侧栏无分类 | 显示 "—"（沿用现有空态） |

## 8. 测试要点

- `public_read=false` 时匿名 `GET /api/posts` 401、带 token 200；
- 切换开关后同一进程内立即生效（缓存失效）；
- 分类环路拒绝、三层拒绝；
- 删除父级后子级 `parent_id` 变为祖父级；
- 父级筛选包含子级文章；
- 预设应用两次，第二次 `created=0` 且总数不变。
