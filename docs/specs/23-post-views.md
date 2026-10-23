> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 浏览量去重与上报接口

> 状态：In Progress（第 43 轮实现）
> 关联：`docs/TODO.md` P2-6（可信代理闸门，本轮顺带修）
> 最后更新：2026-10-22

## 1. 背景

现状（第 43 轮实测）：`GetPost`（`services.go:647`）与 `GetPostPublic`（`services.go:741`）在**每一次读取**时执行 `view_count = view_count + 1`。后果：

- 同一访客刷新页面即累加，后退再进也累加；
- 作者自己预览、管理员在后台点开文章也累加；
- 搜索引擎爬虫每抓一次累加一次；
- 任何人写个循环脚本可把任意文章刷到任意数字。

浏览量是站点唯一的量化指标（`/api/me/stats`、仪表盘都用它），错了就是**整站数据不可信**。同时 `X-Forwarded-For` 被无条件信任（`middleware/limiter.go:178`、`handlers/auth.go:114`），意味着攻击者只要伪造该头就能绕过一切基于 IP 的判定——去重指纹同样依赖它，所以两件事必须一起做。

## 2. 目标与非目标

**目标**

- 浏览量按「**访客 × 文章 × 自然日**」去重：同一人在同一天内反复看同一篇只算一次。
- 作者本人访问不计；常见爬虫/命令行工具 UA 不计。
- 计数逻辑从「读接口的副作用」改成「**显式上报**」，读接口变纯读。
- 只有来自**可信代理**的 `X-Forwarded-For` 才被采信。

**非目标**

- 不做实时在线人数、不做 PV/UV 报表页面（后续可用 `post_views` 表派生）。
- 不做跨天累计的「独立访客总数」（当前口径是「每日独立访客累加」，与主流博客一致）。
- 不做历史数据的追溯修正（旧 `view_count` 保留原值，从本 spec 上线时刻起准确）。

## 3. 数据模型

```
post_views
  id            INTEGER PK AUTOINCREMENT
  post_id       TEXT    NOT NULL  (index)
  visitor_hash  TEXT    NOT NULL  (64 hex)
  day           TEXT    NOT NULL  ('YYYY-MM-DD'，UTC)
  created_at    DATETIME
  UNIQUE(post_id, visitor_hash, day)
```

- `visitor_hash = sha256(ip + "|" + ua + "|" + JWT_SECRET)`，只存哈希，不存原始 IP/UA。
- 唯一索引是去重的**唯一依据**：靠 `INSERT ... ON CONFLICT DO NOTHING` 的 `RowsAffected` 判定是否首次。
- 表为**新建表**，走 AutoMigrate 的「仅创建不存在的表」路径，无需手工迁移。
- 保留期 90 天，惰性清理（每进程每天最多清一次）。

## 4. API 契约

### 4.1 `POST /api/posts/{id}/view`

- 鉴权：`withOptionalAuth`（匿名可调；带了有效 token 就能识别「作者本人」）。
- 请求体：空（不接受任何客户端传入的 ID/IP/时间）。
- 响应：

```json
{ "counted": true,  "view_count": 42 }
{ "counted": false, "view_count": 42, "reason": "duplicate" }
```

`reason` 取值：`duplicate` / `author` / `bot`。

### 4.2 读接口语义变更

- `GET /api/posts/{id}`（auth）：**不再**修改 `view_count`。
- `GET /api/posts/{id}/public`（公开）：**不再**修改 `view_count`。
- 两者返回的 `view_count` 是当前库中的值，不包含本次访问。

## 5. 可信代理（P2-6）

新增环境变量 `TRUSTED_PROXIES`：逗号分隔的 IP / CIDR 白名单，例如 `127.0.0.1/32,::1/128,10.0.0.0/8`。

- 为空时默认 `127.0.0.1/32,::1/128`（兼容「nginx 与后端同机」这一最常见部署）。
- 判定顺序：`RemoteAddr` 的 IP 在白名单内 → 采信 `X-Forwarded-For` 最左一跳；否则**忽略** `X-Forwarded-For`，一律用 `RemoteAddr`。
- 同时修正 `middleware/limiter.go` 与 `handlers` 的 IP 提取，统一走一个实现，杜绝两处漂移。

## 6. 用户故事 + 验收标准（EARS）

- **US-1**：作为站主，我希望浏览量反映真实阅读人数，不被我自己和爬虫污染。
  - **AC-1.1**：WHEN 同一 IP + 同一 UA 在同一 UTC 自然日内第二次上报同一篇文章 THEN 返回 `counted=false, reason="duplicate"`，且 `view_count` 不变
  - **AC-1.2**：WHEN 上报者是该文章作者 THEN 返回 `counted=false, reason="author"`，且 `view_count` 不变
  - **AC-1.3**：WHEN `User-Agent` 命中爬虫特征（bot/spider/crawler/slurp/headless/curl/wget/python-requests 等，大小写不敏感）THEN 返回 `counted=false, reason="bot"`，且 `view_count` 不变
  - **AC-1.4**：WHEN 次日（UTC 日期变化）同一访客再次阅读 THEN 重新计数一次
- **US-2**：作为读者，我刷新页面不应该让数字虚涨。
  - **AC-2.1**：WHEN 前端打开文章详情 THEN 只在挂载时上报一次，刷新等价于「同一天同一人」→ 不再累加
- **US-3**：作为运维，部署在 nginx 后面时 IP 仍然真实。
  - **AC-3.1**：WHEN `RemoteAddr` 不在 `TRUSTED_PROXIES` 内且请求伪造了 `X-Forwarded-For` THEN 该头被忽略，按 `RemoteAddr` 计算
  - **AC-3.2**：WHEN `RemoteAddr` 在 `TRUSTED_PROXIES` 内 THEN 采用 `X-Forwarded-For` 最左一跳

## 7. 边界与错误

| 情况 | 行为 |
|---|---|
| 文章不存在 | 404 `post not found` |
| 文章非 `published`（private/pending/rejected/banned） | 404（与公开读取一致，不泄露存在性） |
| 数据库插入失败 | 500，不影响读接口（上报是独立请求） |
| 高频上报 | 由通用限流器兜底；`INSERT OR IGNORE` 不产生写放大 |
| 90 天前记录 | 每次进程启动后首次上报时删除一次 |

## 8. 测试要点

- 同一天重复上报两次 → 第二次 `counted=false`；
- 换 `day`（注入时钟）→ 重新计数；
- 作者本人上报 → 不计；
- `curl/8.0` UA → 不计；
- 不可见状态文章 → 404；
- `TRUSTED_PROXIES` 未命中时忽略 XFF（表驱动）。

## 9. 已知取舍

- 用自然日（UTC）而非滑动 24 小时窗口：实现简单、可索引、跨日误差最多一次，对博客足够。
- 指纹含 `JWT_SECRET`：防止外部用相同算法离线枚举 IP 反推哈希。
- 不记录 `post_id + ip` 明文：即使是管理员也无法从表里还原访客网络地址。
