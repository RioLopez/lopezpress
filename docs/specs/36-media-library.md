> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# 图库、头像与账号资料策略

> 状态：Accepted（§9 五项已拍板；P0–P3 全部实现——邮箱验证见 `37-email-verification.md`）
> 路由：`/admin/media`（新增）、`/settings`（改造）
> 权限：图库 admin，头像与资料本人，配置 root
> 轮次：第 76 轮立项 · 第 77 轮实现数据层与图库 · 第 78 轮缩略图管线
> 最后更新：2026-10-24

## 0. 现状体检（代码事实，不是猜测）

| 关注点 | 现状 | 判定 |
|---|---|---|
| 上传权限 | `POST /api/upload` 为 `authRequired`，任何登录用户可传 | 可接受（作者要能插图） |
| 单文件大小 | `UPLOAD_MAX_BYTES`，默认 **2 MB**；handler 用 `MaxBytesReader` 提前截断并回 413 | 已有，但按用途未分级 |
| 格式校验 | 扩展名白名单 → `DetectContentType` → `image.DecodeConfig`（边长 ≤12000、像素 ≤4000 万） | 第 75 轮已加固 ✅ |
| 文件名 | `<unix-ms>-<8hex><ext>`，如 `1792811611984-2278c63f.png` | 不可枚举，但**暴露上传时间** |
| 元数据 | **零**。没有 DB 记录，只有文件 mtime | 缺。谁传的、IP、尺寸、hash 全无 |
| 配额 | **无**。单用户无上限、全站无上限 | 缺。爆盘风险 |
| 去重 | **无**。同一张图传十次 = 十个文件 | 缺 |
| 图库读取 | `GET /api/uploads` 为 `authRequired` → **任何登录用户可列全站图片** | **越权**（与「不是免费图床」的定位直接冲突） |
| 图库删除 | `DELETE /api/uploads/{name}` 为 `adminRoles` | 权限对，但 UI 没对齐 |
| 图库入口 | 挂在 **`/settings`（个人设置页）**，普通用户能看到列表、能看到删除按钮 → 点了必 403 | **UI 与后端权限不一致**（第 75 轮同类问题） |
| 头像 | `User.Avatar` 是**任意字符串**，`UpdateMe` 原样写入，无校验、无裁剪、无尺寸 | 缺。可被塞外链追踪像素 |
| 头像展示 | `object-fit: cover`，未强制 1:1 容器 | 部分位置可能失调 |
| 邮件能力 | SMTP + 4 类通知（登录失败/改密/异地登录/审核结果） | 有发信，**没有验证码** |
| 站点配置 | 17 个字段平铺一页，无任何图片/存储策略项 | 缺。没有留给站长调的地方 |

结论：**「图库管理」和「磁盘保护」不是两件事，是同一件事** —— 没有元数据表就没有配额、没有配额就防不住爆盘、没有归属就没有「谁的图」。

## 1. 目标与边界

**做**：
1. 上传有硬上限（单文件 / 单用户 / 全站），满了明确拒绝且不写半截文件。
2. 每一张图都有归属、时间、来源 IP、尺寸、哈希，**但这些只在后台可见，不进 URL**。
3. 图库是**站长资产视图**，只有 admin/root 能看全站；普通用户只能看自己传的。
4. 头像有专用通道：裁剪 → 压缩 → 上传 → 二次校验，展示处处 1:1。
5. 站长能决定「用户可以自己改哪些资料字段」。
6. 邮箱相关的验证规划进去（分阶段）。

**不做**（明确画线，避免范围失控）：
- 不做多用户相册/文件夹/标签分类（这是博客图库，不是网盘）。
- 不做 CDN、不做图片处理服务、不加水印。
- 不做前端图床（普通用户没有「我的图库」页面）。
- 不引入图片处理依赖（除非缩略图拍板要做，见 §9-D3）。

## 2. 用户故事 + 验收标准

- **US-1**：作为站长，我要让磁盘不会被传满，以便站点不会因为图片而宕机。
  - **AC-1.1**：WHEN 单文件超过该用途上限 THEN 413，且 `data/uploads` 不新增任何文件（含临时文件）。
  - **AC-1.2**：WHEN 某用户已用配额 THEN 再传 413，错误信息含「已用 / 上限」，其他用户不受影响。
  - **AC-1.3**：WHEN 全站已用达到上限 THEN 所有上传 413，后台顶部出现红色横幅提示清理。
  - **AC-1.4**：WHEN 站点重启 THEN 配额从 DB 汇总恢复，不做全目录扫描（扫描仅每小时对账一次）。

- **US-2**：作为站长，我要看到所有图片是谁、什么时候、多大传的，以便管理站点资产。
  - **AC-2.1**：WHEN 打开 `/admin/media` THEN 列表每行显示缩略预览、文件名、上传者、时间、大小、尺寸、被几篇文章引用。
  - **AC-2.2**：WHEN 按上传者 / 时间范围 / 大小筛选 THEN 结果正确，分页 24 条。
  - **AC-2.3**：WHEN 删除一张被文章引用的图 THEN 409 并列出引用它的文章标题，除非勾选「强制删除」。
  - **AC-2.4**：WHEN 查看图片详情 THEN 能看到原始文件名、来源 IP、sha256，但**这些信息不出现在 `src` 里**。

- **US-3**：作为作者，我要知道这张图是不是已经传过，以便不重复占用空间。
  - **AC-3.1**：WHEN 上传的图与库中某张 sha256 相同 THEN 直接复用该 URL，返回 `{url, deduped:true}`，磁盘不新增文件。
  - **AC-3.2**：WHEN 复用 THEN 前端提示「图库中已有相同图片，已直接使用」。
  - **AC-3.3**：WHEN 别人上传过同一张图、我删掉自己这条引用 THEN 文件保留（还有别的引用），我的文章破图提示正常。

- **US-4**：作为普通作者，我只能看到自己传的图，不是免费图床。
  - **AC-4.1**：WHEN 普通用户请求 `GET /api/uploads` THEN 403。
  - **AC-4.2**：WHEN 普通用户请求 `GET /api/me/uploads` THEN 只返回自己的，分页 20。
  - **AC-4.3**：WHEN 普通用户删除自己未被引用的图 THEN 204；删除已被引用的 THEN 409。
  - **AC-4.4**：WHEN 普通用户访问 `/settings` THEN 页面上没有全站图库区块。

- **US-5**：作为用户，我要上传头像并让它显示得整齐。
  - **AC-5.1**：WHEN 选择头像 THEN 出现 1:1 裁剪框，确认后前端缩放到 512×512 并转 WebP（质量 0.85）再上传。
  - **AC-5.2**：WHEN 上传的头像不是正方形或超过 512×512 或超过 512 KB THEN 400。
  - **AC-5.3**：WHEN 头像替换成功 THEN 旧头像文件被删除（仅当它属于该用户）。
  - **AC-5.4**：WHEN 任何位置展示头像 THEN 容器 `aspect-ratio:1` + `object-fit:cover`，不变形。
  - **AC-5.5**：WHEN 站长关闭「用户可改头像」THEN 普通用户看不到入口，`PUT /api/me` 带 avatar 返回 403。

- **US-6**：作为站长，我要决定用户可以自己改哪些字段。
  - **AC-6.1**：WHEN 关闭「允许改用户名」THEN 前端禁用该输入，`PUT /api/me` 带 username 返回 403（不是静默忽略）。
  - **AC-6.2**：WHEN 管理员在后台改用户资料 THEN 不受上述开关限制。
  - **AC-6.3**：WHEN 改邮箱 THEN 需要验证码（若已启用邮箱验证），管理员操作则免验证。

- **US-7**：作为站长，我要不被假邮箱烦到。
  - **AC-7.1**：WHEN 开启邮箱验证 THEN 注册必须携带有效验证码，否则 400。
  - **AC-7.2**：WHEN 同一邮箱 60 秒内重复请求验证码 THEN 429。
  - **AC-7.3**：WHEN 验证码错误 5 次 THEN 该码作废，需重新申请。

## 3. 数据模型

```sql
-- 一个物理文件一行
uploads (
  id            TEXT PRIMARY KEY,          -- 与文件名前缀同源，22 字符 base62
  name          TEXT NOT NULL UNIQUE,      -- 磁盘文件名（含扩展名）
  owner_id      TEXT NOT NULL,             -- 首个上传者；历史遗留为空串
  size          INTEGER NOT NULL,
  width         INTEGER NOT NULL,
  height        INTEGER NOT NULL,
  mime          TEXT NOT NULL,
  sha256        TEXT NOT NULL,             -- 去重键（不存 md5：已可构造碰撞）
  original_name TEXT,                      -- 原始文件名，仅后台可见
  ip            TEXT,                      -- 来源 IP，仅后台可见
  ref_count     INTEGER NOT NULL DEFAULT 0,
  created_at    DATETIME,
  deleted_at    DATETIME
)
CREATE INDEX idx_uploads_owner ON uploads(owner_id);
CREATE INDEX idx_uploads_hash  ON uploads(sha256);
CREATE INDEX idx_uploads_live  ON uploads(deleted_at);

-- 谁在什么地方用了它（引用计数的事实来源）
upload_refs (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  upload_id  TEXT NOT NULL,
  post_id    TEXT,                         -- 文章引用；头像引用为空
  user_id    TEXT NOT NULL,                -- 引用者（≠ 上传者，去重场景下可能不同）
  kind       TEXT NOT NULL,                -- body | avatar | logo | favicon
  created_at DATETIME
)
CREATE UNIQUE INDEX idx_refs_unique ON upload_refs(upload_id, post_id, kind);
```

**引用怎么来**：文章保存时用正则 `\/uploads\/([A-Za-z0-9._-]+)` 扫 `body`，与上一版引用做差集（新增的加 refs、消失的减 refs）。头像/logo/favicon 在写入时直接记账。

**为什么不用 `ref_count` 单独维护**：`ref_count` 只是 `upload_refs` 的缓存列，用来避免列表页每条都 count。两者不一致时以 `upload_refs` 为准，每小时对账修正。

**历史数据**：现有 11 个文件没有记录。迁移时扫描目录，插入 `owner_id=""`、`size` 取 `stat`、`sha256` 现场算、`width/height` 用 `DecodeConfig`，标记 `legacy`。这样它们立刻纳入配额统计，但不出现在任何人的「我的上传」里。

## 4. 接口变更

| 端点 | 变更 |
|---|---|
| `POST /api/upload` | 增加配额检查、sha256 去重；返回 `{url, name, size, width, height, deduped}` |
| `GET /api/uploads` | **改 admin/root**（现状任何登录用户） |
| `GET /api/me/uploads` | 新增，登录，只返回自己的，分页 |
| `DELETE /api/uploads/{name}` | 改 admin/root；被引用时 409，`?force=1` 才强删 |
| `DELETE /api/me/uploads/{name}` | 新增，本人且未被引用 |
| `POST /api/me/avatar` | 新增，专门收头像（校验正方形 / ≤512 / ≤512 KB），写 refs(kind=avatar)，删旧文件 |
| `PUT /api/me` | 逐字段按站点开关授权；avatar 只接受 `/uploads/` 前缀或 https |
| `POST /api/auth/email-code` | 新增（注册 / 改邮箱 / 找回密码共用），限流 |
| `GET /api/admin/media/stats` | 新增，返回全站占用、Top 上传者、孤儿文件数 |

## 5. 磁盘保护算法

三层，从外到内：

1. **单文件**：按用途分级 —— 正文图 5 MB（可配 1–10）、头像 512 KB、站点 logo/favicon 1 MB。handler 层 `MaxBytesReader` 直接 413。
2. **单用户**：`SUM(size) FROM uploads WHERE owner_id=? AND deleted_at IS NULL` ≤ `user_quota_bytes`（默认 20 MB）。超了 413 + 「已用 12.4 MB / 上限 20 MB」。
3. **全站**：`SUM(size)` ≤ `site_quota_bytes`（默认 500 MB，范围 100 MB–2 GB）。90% 时后台横幅黄色告警，100% 时所有上传 413（站点只读不影响）。

**对账**：每小时（复用现有后台任务）比对 `data/uploads` 实际字节数与 DB 汇总，差异 >5% 则重算并写审计。这样即使有人绕过 API 直接拷文件进目录，也能发现。

**为什么不做 `statfs`**：Windows 开发机与 Linux VPS 的系统调用不同，要为它加 build tag 或新依赖；本项目所有写入都经过 API，DB 汇总足以守住。真到磁盘告急是运维层面的问题，用监控解决更合适。

## 6. 链接与隐私

**结论：不加密，用不可枚举的随机 ID。**

- 加密（AES 之类）是可逆的，还得管密钥、还得改 URL 才能换密钥 —— 收益为零。
- 需要的只是「外人看不出含义 + 猜不到」：`crypto/rand` 取 16 字节 → base62 → 22 字符。
- 新文件名形如 `k7Qm2xR9tLp4vB8nW1sY6z.png`。不含时间、不含用户、不含原文件名。
- **老文件不改名**：已发布文章里写死了 `1792811611984-2278c63f.png`，改名就是全站破图。新格式只用于新上传。
- 元数据（谁传的、IP、原文件名）**只在 `/admin/media` 详情里出现**，`src` 永远是干净的 URL。

顺带修掉一个隐性问题：现在的文件名带 unix 毫秒，任何人看到链接就知道这张图是几点传的。

## 7. 头像流程

```
点击头像 → 选文件（限 5 MB，仅 image/*）
   → 前端 canvas 裁剪成 1:1（可缩放/平移）
   → 缩放到 512×512 → toBlob('image/webp', 0.85)  ← 通常 30–80 KB
   → POST /api/me/avatar
   → 后端：扩展名 + magic + DecodeConfig，校验正方形、≤512、≤512 KB
   → 存 uploads/avatars/<id>.webp，写 refs(kind=avatar)，删旧头像文件
   → 返回新 URL，前端刷新
```

**为什么后端不自己裁**：头像裁剪的核心是「人来决定构图」，本来就得有交互，放到浏览器是最自然的。后端的职责是**拒绝不合规的东西**，不是修图。（第 78 轮引入了 `golang.org/x/image`，所以「没有好用的缩放算法」这个理由已经不成立了 —— 但那是给后台缩略图队列用的，不在请求路径上，头像仍然由前端裁。）

**兼容策略**：如果站长关掉用户改头像，入口消失；管理员在后台仍然可以给用户设头像（走同一个端点，跳过开关）。老用户的外链头像保持可显示，但 `PUT /api/me` 不再接受新的外链（除非站长开着「允许外链头像」）。

## 8. 站点设置重组

现在 17 个字段平铺一页，加完这轮会到 30+，必须分组（每组一个 tab）：

| 组 | 字段 |
|---|---|
| **站点信息** | site_name、site_title、site_logo、site_favicon、site_url、footer_text、icp_no、friend_links |
| **阅读与注册** | public_read、allow_register、page_size、注册方式（开放/邀请码/关闭）、require_email_verified |
| **图片与存储** | img_host_mode、img_host_token、upload_max_image、upload_max_avatar、user_quota_bytes、site_quota_bytes、allow_user_delete_upload、allow_external_avatar |
| **邮件与通知** | smtp_host/port/user/pass、4 个通知开关、测试发信 |
| **数据与备份** | auto_backup、backup_keep、导入 / 导出 / 快照（root） |
| **安全** | 登录限流参数、token 有效期、是否允许用户改用户名/邮箱/昵称/简介/头像（5 个开关） |

「安全」组里的 5 个开关就是 §2-US-6 的字段级授权开关，默认值建议：用户名 **否**、邮箱 **是**、昵称 **是**、简介 **是**、头像 **是**。

## 9. 已拍板（第 77 轮）

| # | 问题 | 决定 | 落地方式 |
|---|---|---|---|
| D1 | 普通用户能不能删自己的图 | **能删，但不是真删**：用户看到「已撤下、别人再也用不了」，字节仍留盘上，管理员在后台能看到并执行真实删除（哪怕已被文章引用） | `softDeleteUpload` 只写 `deleted_at`/`deleted_by`，并把 `live_key` 轮换成 `d:<id>`；`hardDeleteUpload` 仅 admin/root |
| D2 | 去重怎么做 | **B：物理合并 + 引用计数**，并且**所有权优先于省空间** | `live_key` 唯一索引 + `upload_refs` 台账 + `ref_count` 缓存列 |
| D3 | 要不要缩略图 | **要做**，且**必须排队慢慢来**，不能一把上传就把 CPU 打满 | `internal/services/thumbs.go`：单 goroutine + 有界 channel + 每张之间 150 ms，见 §9.1 |
| D4 | 邮箱验证做到哪一步 | **全套**（注册验证码 / 改邮箱 / 找回密码），但**定位为测试功能**：默认关闭、界面标注「测试」、管理员可开可关 | 尚未实现，见 §10 P3 |
| D5 | 头像 | **强制 1:1 裁剪，用户可微调构图** | `web/src/ui/AvatarCropper.tsx`：拖动 + 缩放 1–3x + 边界钳制，输出 512×512 WebP |

### 9.1 「删除」与「侵权」是同一件事

站长原话：*「用户删了，哪怕我们是假删除，至少也不能再给别的用户使用了，这个是所有权的问题」*。这条不能靠流程约束（「记得判断 deleted_at」），必须由数据结构保证：

- `upload_files.live_key` 唯一索引。活着时它是 sha256，软删时改成 `d:<id>`。
- 于是「去重查询」这个**唯一**的复用入口，结构上永远查不到已撤下的图 —— 不需要任何一层记得再判断一次。
- 老文件用 `legacy:<name>`，根本不参与去重。
- 文件继续由 `GET /uploads/{name}` 提供：已发布文章里的图不能因为上传者撤下而破图，那是让文章作者替别人的决定受罚。

### 9.2 缩略图管线（第 78 轮）

设计要点，按重要性排序：

1. **队列在数据库里，不在内存里。** `upload_files.thumb_status` 是唯一的待办真相：`pending` → `ready` / `skip` / `failed`。channel 只是唤醒提示，容量 32，满了直接丢 —— 因为行还是 pending，下次启动会补。进程被 kill 不会丢任务。
2. **单消费者 + 节流。** 一个 goroutine 一次解一张，之间睡 150 ms。1C 机器上并发解码 8 张 1200 万像素的图，等于把 HTTP 服务饿死。
3. **规格**：长边 480 px（卡片约 200 CSS px，覆盖 2x 屏），CatmullRom 重采样；不透明输出 JPEG q82，有透明输出 PNG（JPEG 会把透明像素变黑）。
4. **宁可不做也不做坏**：三种情况直接放过，返回 `skip` —— 长边 ≤480 且 ≤96 KB（原图已经更划算）、重编码后反而更大、格式解不开。前端一律 `thumb_url || url` 回落，所以「没有缩略图」永远不会表现成「破图」。
5. **解码上限比上传上限更严**：上传允许 40 MP，worker 只解到 24 MP。后台 goroutine 在 1.5 GB 机器上 OOM 会直接杀掉整个进程。
6. **依赖**：新增 `golang.org/x/image`（`draw` 缩放 + `webp` 解码）。这是官方扩展库、纯 Go、无 CGO；顺带修掉一个真问题 —— 之前 WebP 拿不到宽高（标准库没有解码器），图库里一直显示 `—`。
7. **硬删连带删缩略图**：缩略图是「已不存在的字节」派生的，留着没有意义。软删保留（原图还在服务已发布文章）。

实测（第 78 轮隔离冒烟 23/23）：900×600 噪声 PNG 1.62 MB → 480×320 JPEG 73 KB（**22×**）；三张图上传后队列里还有 2 张 pending（证明没有在请求路径里同步生成）；200×150 纯色图判 `skip`；重启后遗留的 pending 行在启动扫描中被补跑成 `ready`。

## 10. 分批计划

| 批次 | 内容 | 为什么这个顺序 |
|---|---|---|
| **P0** | ① `GET /api/uploads` 收归 admin ② SettingsPage 图库对非 admin 不渲染 ③ avatar 字段校验 ④ 元数据表 + 迁移回填 + 三层配额 | 前三条是**现有越权**，第四条是**爆盘**，都不该等 |
| **P1** | ⑤ `/admin/media` 图库页 ⑥ sha256 去重 + 引用计数 + 文章引用扫描 ⑦ 新 URL 格式 ⑧ 我的上传 | 有了元数据才谈得上管理 |
| **P2** | ⑨ 头像裁剪流程 ⑩ 资料字段权限矩阵 ⑪ 站点设置分组 | 纯体验 + 站长控制 |
| **P3** | ⑫ 邮箱验证 / 邀请码 | 依赖 SMTP 送达质量，独立推进 |

## 5.5 性能与资源

- **前端包体积**：图库页约 +8 kB（列表 + 筛选），头像裁剪复用 canvas 不引库，约 +4 kB。
- **后端查询量级**：图库列表 24 行 + 一次 count；配额检查是单行聚合（`uploads` 加索引后 <1 ms）；对账每小时一次全目录 stat。
- **内存 / 磁盘**：上传仍全量读入内存（受单文件上限约束，最大 5 MB × 并发数）；磁盘由配额封顶。
- **慢点在哪**：① 图库列表已改走缩略图（第 78 轮），原图只在详情弹窗加载 ② 文章保存时扫 `body` 提取引用（10 万字符正则，可接受）③ 对账时全目录 `stat`（1 万文件约 1–2 s，夜间跑）④ 缩略图队列会占 CPU，但被单消费者 + 150 ms 间隔压住了峰值。
- **是否需重新评估**：图片超过 1 万张时，一次启动扫描最多排 200 张（`thumbResumeBatch`），剩下的靠后续上传触发；超过 2 万张时 `SUM(size)` 也该改成增量维护。

## 6. 边界情况

- **并发上传同一张图**：两个请求同时算 sha256 都没命中 → 用 `sha256` 唯一索引兜底，冲突时回读已有行复用。
- **删图但有文章引用**：409 列标题；强制删除后文章保留文字，图片 404（前端显示破图占位）。
- **配额刚好卡在边界**：先写临时文件再 `rename`，落盘后更新 DB；DB 更新失败则删临时文件，不留半截。
- **SMTP 没配**：邮箱验证相关端点返回 400「邮件服务未配置」，注册方式自动回退为「管理员创建」。
- **历史遗留文件**：`owner_id=""`，任何人的「我的上传」都看不到，只在管理员图库显示为 `legacy`。

## 7. 已知限制 / 未来方向

- 引用扫描是**正则匹配正文**，`![alt](/uploads/x.png)` 会命中，但手写 HTML 变体、外链转内链等不会被追踪；这是刻意选择（简单、无解析器依赖）。
- 不做图片格式转换（上传什么存什么），首屏流量靠缩略图 + `lazy` 兜。
- WebP 的上传校验仍以魔数为准：`golang.org/x/image/webp` 是解码器不是格式警察，而且它拒绝动画 WebP，用它做门槛会把合法的动态图挡在门外。代价是 WebP 不做像素上限检查（尺寸仍由上传字节上限约束）。
- 邀请码方案未设计（D4 若选邀请码再补）。

## 8. 相关

- 依赖：`specs/07-media-upload`（若存在）、`specs/34-auto-backup`
- 相关决策：第 75 轮「按资源而不是按接口定权限」——本轮 P0-① 就是这条的延续
- 代码锚点：`internal/services/media.go` 的 `UploadImage` / `ListLocalUploads` / `DeleteLocalUpload`；`internal/handlers/upload.go`；`web/src/pages/SettingsPage.tsx` 的 `ImageLibrary`
