# 备份管理（`/admin/backup`）

> 状态：Done
> 路由：`/admin/backup`
> 权限：admin
> 轮次：第 11、17 轮
> 最后更新：2026-09-21

## 1. 目标

admin 一键导出全站数据为 JSON 备份文件（用户 / 文章 / 分类 / 标签），并可导入以恢复数据。

## 2. 用户故事 + 验收标准（EARS）

- **US-1**：作为 admin，我要导出全站备份。
  - **AC-1.1**：WHEN 点「导出备份」THEN 后端生成 JSON 备份 + 浏览器下载 + 成功 toast
  - **AC-1.2**：WHEN 导出成功 THEN 显示最近一次备份信息（文件名 / 大小 / 各表条数统计）
  - **AC-1.3**：WHEN 导出失败 THEN 错误 toast（保留上次成功信息若有的话）
- **US-2**：作为 admin，我要从备份文件导入以恢复数据。
  - **AC-2.1**：WHEN 选择 JSON 备份文件并确认覆盖 THEN 后端事务内清空并按依赖顺序重插全部数据，成功 toast + 更新统计
  - **AC-2.2**：WHEN 未选文件 THEN 提示先选文件，不请求
  - **AC-2.3**：WHEN 确认弹窗被取消 THEN 取消导入
  - **AC-2.4**：WHEN 文件解析失败或后端返回错误 THEN 错误 toast，数据不变

## 3. 前端

- **布局**（ASCII）：

```
┌───────────────────────────┐
│ 顶栏（admin 布局）         │
│  h1 备份管理 + 副标题       │
│  [导出备份] 按钮            │
│  最近备份：文件名 大小       │
│   统计：用户 N 文章 N 分类 N 标签 N │
│  ─────────────────        │
│  h2 导入备份 + 副标题       │
│  [选择文件 ▢]  [导入备份] 按钮 │
└───────────────────────────┘
```

- **组件树**：`Layout > AdminBackupPage > (button / backup-result / backup-import)`
- **关键交互**：
  - 挂载时 `!hasToken || role !== 'admin'` 清空 `result`
  - 导出走 `exportBackup()`（Blob 下载）
  - 成功显示 `result`（filename / size / stats）
  - `fmtBytes` 把字节转 B / KB / MB
  - 导入：`fileRef` 读本地文件 → `JSON.parse` → `confirmDialog` → `importBackup(data)`；成功后刷新 result 并清空 file input
- **路由**：`/admin/backup`
- **i18n 键**：`admin.*`（backupTitle / backupSub / exportBtn / exportBusy / exportSuccess / exportFailed / lastBackup / backupStats / stUsers / stPosts / stCats / stTags / importTitle / importSub / importBtn / importBusy / importSuccess / importFailed / importConfirm / importNoFile）
- **共享组件依赖**：`Layout` / `i18n` / `Toast` / `confirmDialog`

## 4. 后端

- **API 端点**：
  - `POST /api/backup` → `BackupResult { filename, size, stats }`（后端在 `BACKUP_DIR` 拼 JSON **落盘**，只回元信息）
  - `GET /api/backup/download?file=<name>` → 备份文件正文（`Content-Disposition: attachment`，第 18 轮补实 AC-1.1 的"浏览器下载"）
  - `POST /api/backup/import` → `BackupStats { users, posts, categories, tags }`（body = 导出的 JSON 原文）
- **Service 方法**：`svc.ExportBackup`（导出 users / posts-with-softdeleted / categories / tags + password_hashes + post_tags）、`svc.ImportBackup`（事务内清空重插）、`svc.ServeBackup`（第 18 轮：按文件名安全读取并流式回写）
- **下载安防**（`svc.ServeBackup`）：拒绝含 `..` / `/` / `\` 的名字、`filepath.Base(name) != name` 即拒、`filepath.Rel` 复核解析结果仍在备份目录内；一律返回 404（不区分"名字非法"与"文件不存在"，避免探测泄露）
- **传输回路**：导出写盘 → 前端拿 `filename` 回捞 `GET /api/backup/download` → Blob → `a[download]` 触发另存
- **数据模型**：全表快照；导出含 `password_hashes` map 与 `post_tags` 关联，保证导入往返完整
- **权限检查**：`requireAdmin`
- **错误处理**：导出失败 500；导入 body 非法 400、执行失败 500

## 5. 数据流

```
导出：点导出 → exportBackup() → POST /api/backup → svc.ExportBackup → JSON 落盘
  → 回 {filename,size,stats} → downloadBackup(filename) → GET /api/backup/download?file=
  → svc.ServeBackup 流式回写 → Blob → saveBlob() 触发另存 → 显示 stats + 成功 toast
导入：选文件 → JSON.parse → confirmDialog → importBackup(data)
  → POST /api/backup/import → svc.ImportBackup（事务：清空 → 按序重插）
  → 刷新 result + 成功 toast
```

- **可能失败点**：导出过程 DB 读失败 / 网络中断；导入文件损坏 / 非本系统备份格式
- **回退**：失败 toast + 保留上次 result

## 5.5 性能与资源

- **前端包体积**：影响 < 1 kB
- **后端查询量级**：导出全表；导入先全表 Delete 再逐条 Create（事务内）
- **内存 / 磁盘**：
  - 导出文件落盘 `BACKUP_DIR`（5 GB 磁盘需留意备份累积）
  - 大站点（文章上万）导出 JSON 体积大，可能 10~100 MB
- **慢点在哪**：全表导出 + 写盘；导入逐条 Create（文章 > 1 万时明显）
- **是否需重新评估**：大站点时改流式导出 / 定时自动备份 / 批量插入导入

## 6. 边界情况

- 首次导出（无 result）：不显示"最近备份"
- 导出中：按钮「导出中…」禁用
- 导入未选文件：提示先选文件
- 导入确认取消：不请求
- 导入后：清空 file input，避免重复选择同一文件被跳过

## 7. 已知限制 / 未来方向

- 无定时自动备份；候选＝cron 每日备份 + 保留 N 份
- 备份落盘占磁盘（5 GB 需控制累积）；候选＝备份上限 + 滚动删除
- 导入为全量替换式，无差异合并 / 预览；候选＝导入前 dry-run 预览统计

## 8. 相关

- 依赖 spec：`specs/06-admin-layout.md`
- 相关决策：无
