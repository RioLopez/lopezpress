> 类别 B · 项目/内容/参考 — 产品与工程文档，AI 按需查阅，不约束流程。

# Spec 34 — 每日自动备份

> 状态：Implemented（第 52 轮）｜关联：`docs/COMPETITIVE_GAP.md`

## 1. 问题

第 49 轮买家审查给"数据安全"6 分、给"部署"5 分，原因之一是：**备份只有一个按钮，点不点靠自觉**。个人博客最常见的丢数据场景不是硬盘坏，而是"改错了、想回到上周"。

## 2. 目标

在**不引入 cron、不引入外部依赖、不改变低配定位**的前提下，让每个站点每天自动有一份可回滚的数据库快照。

## 3. 设计决定

| 决定 | 理由 |
|---|---|
| 用 `VACUUM INTO` 生成快照 | SQLite 原生、原子、一致；和导入前快照走同一条路径 |
| 判断"今天备过没"用文件名，而不是调度器 | 进程重启、机器休眠都不会漏；最坏情况是晚几小时 |
| 文件名 `auto-YYYYMMDD.db`，与导入快照 `snapshot-*` 分开 | 否则上午做过一次导入，当天自动备份就会被误判为"已完成" |
| 默认**关闭** | 升级老库时不能突然开始写文件；开关是运营者的决定 |
| 保留份数 1–90，越界回落到 7 | `keep = 0` 等于"删光所有备份"，不允许 |
| root 专属 | 备份策略影响整个安装，不属于普通管理员 |

## 4. 行为

- **WHEN** `auto_backup = false` **THEN** 不做任何事，不产生文件。
- **WHEN** `auto_backup = true` 且今天（UTC）的快照不存在 **THEN** 写入 `auto-YYYYMMDD.db`，并按 `backup_keep` 淘汰更旧的 `auto-*`。
- **WHEN** 今天的快照已存在 **THEN** 直接返回空文件名（幂等，可重复调用）。
- **WHEN** 非 root 调用自动备份端点 **THEN** 403。
- **WHEN** 普通管理员保存站点设置 **THEN** 服务端保留已有的 `auto_backup` / `backup_keep`（不允许被覆盖）。
- **WHEN** 淘汰最旧快照 **THEN** 只删 `auto-*`，绝不碰 `snapshot-*`。
- **WHEN** 列出/恢复快照 **THEN** `auto-*` 与 `snapshot-*` 同样可见、可恢复。

## 5. 触发时机

1. 进程启动时检查一次；
2. 之后每 30 分钟检查一次（命中已有文件时成本是一次 `stat`）。

不是"每天 03:00 准时"，而是"每天最多一次，进程活着就会补上"。

## 6. 边界

| 场景 | 结果 |
|---|---|
| 磁盘满 / 目录不可写 | 记 `WARN auto backup failed: ...`，不阻塞启动、不影响请求 |
| 两个进程同时跑 | `VACUUM INTO` 目标已存在会失败，天然互斥 |
| `keep = 3`，已有 5 份旧的 | 运行后剩 3 份，含今天那份 |

## 7. 验收

| 编号 | 条件 | 证据 |
|---|---|---|
| AC-1 | 关闭时不产生文件 | `TestAutoBackupOffWritesNothing` |
| AC-2 | 同一天只生成一份 | `TestAutoBackupOncePerDay` |
| AC-3 | 按 keep 淘汰最旧 | `TestAutoBackupPrunesOldSnapshots` |
| AC-4 | 不删导入前快照 | `TestAutoBackupPruneIgnoresImportSnapshots` |
| AC-5 | keep 不可能是 0 | `TestClampBackupKeep` |
| AC-6 | 真机可生成、可列出 | 第 52 轮：`POST /api/admin/backup/auto` → `auto-20261022.db`，快照列表同时含 `auto-*` 与 `snapshot-*` |
