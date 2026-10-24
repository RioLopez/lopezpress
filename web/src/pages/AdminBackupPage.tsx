import { useEffect, useRef, useState } from 'preact/hooks'
import { useAuth } from '../core/auth'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { exportBackup, downloadBackup, saveBlob, importBackup, listSnapshots, restoreSnapshot, getAutoBackupStatus, runAutoBackup, errMessage } from '../core/api'
import type { BackupResult, BackupStats, SnapshotInfo } from '../core/types'
import { fmtDate } from '../core/format'
import { isRoot } from '../core/roles'

export function AdminBackupPage() {
  const { user } = useAuth()
  const { pushToast, confirmDialog } = useApp()
  const { t, locale } = useI18n()
  const [result, setResult] = useState<BackupResult | null>(null)
  const [busy, setBusy] = useState(false)
  const [importBusy, setImportBusy] = useState(false)
  const [snapshots, setSnapshots] = useState<SnapshotInfo[]>([])
  const [restoreBusy, setRestoreBusy] = useState<string | null>(null)
  // Spec 34: daily-snapshot state ("did last night run?"). Root-only.
  const [auto, setAuto] = useState<{ enabled: boolean; keep: number; last_at: string; files: number } | null>(null)
  const [autoBusy, setAutoBusy] = useState(false)
  const fileRef = useRef<HTMLInputElement | null>(null)

  // Snapshots are root-only (whole-database artifacts, as-built audit Q7).
  useEffect(() => {
    if (isRoot(user?.role)) {
      listSnapshots().then(setSnapshots).catch(() => setSnapshots([]))
      getAutoBackupStatus().then(setAuto).catch(() => setAuto(null))
    }
  }, [user])

  const fmtBytes = (n: number) => {
    if (n < 1024) return `${n} B`
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
    return `${(n / 1024 / 1024).toFixed(1)} MB`
  }

  const runSnapshotNow = async () => {
    setAutoBusy(true)
    try {
      const r = await runAutoBackup()
      if (r.file) {
        pushToast('success', t('admin.autoBackupRunDone', { file: r.file }))
      } else {
        // Empty name = today's file already exists; that is success, not a
        // failure, so say so instead of showing nothing happened.
        pushToast('success', t('admin.autoBackupOn'))
      }
      setAuto(await getAutoBackupStatus())
      setSnapshots(await listSnapshots())
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setAutoBusy(false)
    }
  }

  const doExport = async () => {
    setBusy(true)
    try {
      const r = await exportBackup()
      // The server writes the JSON to its own disk; fetch it back so the
      // admin actually receives a file (spec 10 AC-1.1).
      const blob = await downloadBackup(r.filename)
      saveBlob(blob, r.filename)
      setResult(r)
      pushToast('success', t('admin.exportSuccess'))
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.exportFailed')))
    } finally {
      setBusy(false)
    }
  }

  const doImport = async () => {
    const file = fileRef.current?.files?.[0]
    if (!file) {
      pushToast('error', t('admin.importNoFile'))
      return
    }

    // Read + parse the file, then hand the parsed object to the API.
    let data: unknown
    try {
      data = JSON.parse(await file.text())
    } catch {
      pushToast('error', t('admin.importFailed'))
      return
    }

    setImportBusy(true)
    try {
      // Step 1: dry run. The server validates the payload (schema, users
      // present, a root account) and returns the counts without writing
      // anything, so the operator confirms against real numbers instead of a
      // filename. An empty `{}` is rejected here with a 400.
      const preview = await importBackup(data, true)
      const ok = await confirmDialog(
        t('admin.importPreview', {
          users: String(preview.users),
          posts: String(preview.posts),
          cats: String(preview.categories),
          tags: String(preview.tags),
        }),
      )
      if (!ok) return

      // Step 2: apply. api.ts adds confirm="REPLACE-ALL-DATA" — the server
      // rejects an apply without it.
      const stats: BackupStats = await importBackup(data)
      pushToast('success', t('admin.importSuccess'))
      // Refresh the result panel to reflect the restored row counts.
      setResult({ filename: file.name, size: file.size, stats })
      // Clear the file input so the same file can be chosen again.
      if (fileRef.current) fileRef.current.value = ''
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.importFailed')))
    } finally {
      setImportBusy(false)
    }
  }

  const doRestore = async (file: string) => {
    if (!(await confirmDialog(t('admin.snapshotRestoreConfirm', { file })))) return
    setRestoreBusy(file)
    try {
      await restoreSnapshot(file)
      pushToast('success', t('admin.restoreSuccess'))
      // The restore auto-snapshotted the pre-restore state, so the list
      // gains an entry; refresh to show it.
      listSnapshots().then(setSnapshots).catch(() => {})
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.restoreFailed')))
    } finally {
      setRestoreBusy(null)
    }
  }

  return (
    <div class="admin-page">
      <h1>{t('admin.backupTitle')}</h1>
      <p class="muted">{t('admin.backupSub')}</p>

      <div class="backup-warning">{t('admin.backupSecurityNote')}</div>

      {/* Export writes a JSON dump that embeds bcrypt password hashes, and
          download streams any file in the backup dir (including the raw .db
          snapshots). Both are root-only on the server, so the button is only
          rendered for root instead of offering a control that 403s. */}
      {isRoot(user?.role) ? (
        <button class="btn btn-primary" onClick={doExport} disabled={busy}>
          {busy ? t('admin.exportBusy') : t('admin.exportBtn')}
        </button>
      ) : (
        <p class="muted">{t('admin.exportRootOnly')}</p>
      )}

      {isRoot(user?.role) && result && (
        <div class="backup-result">
          <div class="backup-result-title">{t('admin.lastBackup')}</div>
          <div class="backup-result-file">
            <span>{result.filename}</span>
            <span class="muted">{fmtBytes(result.size)}</span>
          </div>
          <div class="backup-stats">
            <span class="muted">{t('admin.backupStats')}</span>
            <span>{result.stats.users} {t('admin.stUsers')}</span>
            <span>{result.stats.posts} {t('admin.stPosts')}</span>
            <span>{result.stats.categories} {t('admin.stCats')}</span>
            <span>{result.stats.tags} {t('admin.stTags')}</span>
          </div>
        </div>
      )}

      <div class="backup-import">
        <h2>{t('admin.importTitle')}</h2>
        <p class="muted">{t('admin.importSub')}</p>
        {/* Import/restore replaces the whole database including the users
            table, so it is root-only (as-built audit Q7). Admins see the
            section disabled rather than hidden, so the gate is explicit. */}
        {isRoot(user?.role) ? (
          <div class="backup-import-row">
            <input ref={fileRef} type="file" accept="application/json,.json" />
            <button class="btn btn-danger" onClick={doImport} disabled={importBusy}>
              {importBusy ? t('admin.importBusy') : t('admin.importBtn')}
            </button>
          </div>
        ) : (
          <p class="muted">{t('admin.importRootOnly')}</p>
        )}
      </div>

      {isRoot(user?.role) && (
        <div class="backup-import">
          <h2>{t('admin.autoBackupTitle')}</h2>
          <p class="muted">
            {auto?.enabled ? t('admin.autoBackupOn') : t('admin.autoBackupOff')}
            {' · '}
            {auto?.last_at
              ? `${t('admin.autoBackupLast')}: ${fmtDate(auto.last_at, locale)}`
              : t('admin.autoBackupNever')}
            {auto ? ` · ${auto.keep} ${t('site.backupKeep')}` : ''}
          </p>
          <button class="btn btn-ghost btn-sm" disabled={autoBusy} onClick={runSnapshotNow}>
            {autoBusy ? '…' : t('admin.autoBackupRun')}
          </button>
        </div>
      )}

      {isRoot(user?.role) && (
        <div class="backup-import">
          <h2>{t('admin.snapshotsTitle')}</h2>
          <p class="muted">{t('admin.snapshotsSub')}</p>
          {snapshots.length === 0 ? (
            <p class="muted">{t('admin.snapshotsEmpty')}</p>
          ) : (
            <div class="table-scroll">
              <table class="admin-table">
                <thead>
                  <tr>
                    <th>{t('admin.snapshotsName')}</th>
                    <th>{t('admin.snapshotsSize')}</th>
                    <th>{t('admin.snapshotsTime')}</th>
                    <th class="actions"></th>
                  </tr>
                </thead>
                <tbody>
                  {snapshots.map((s) => (
                    <tr key={s.name}>
                      <td>{s.name}</td>
                      <td class="muted">{fmtBytes(s.size)}</td>
                      <td class="muted">{fmtDate(s.mod_time, locale)}</td>
                      <td class="actions">
                        <button
                          class="btn btn-danger btn-sm"
                          disabled={restoreBusy === s.name}
                          onClick={() => doRestore(s.name)}
                        >
                          {restoreBusy === s.name ? '…' : t('admin.restoreBtn')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}
