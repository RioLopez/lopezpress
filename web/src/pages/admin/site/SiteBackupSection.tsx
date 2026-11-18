import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../../../core/i18n'
import { useApp } from '../../../core/app'
import { getAutoBackupStatus, runAutoBackup, restoreUploadsSnapshot, errMessage } from '../../../core/api'
import { fmtDate } from '../../../core/format'
import type { AutoBackupStatus, SiteConfigFull } from '../../../core/types'

interface Props {
  cfg: SiteConfigFull
  set: <K extends keyof SiteConfigFull>(k: K, v: SiteConfigFull[K]) => void
  // Bumped by the parent after a successful save, so the status block (and the
  // "snapshot now" availability) reflects what the server now has rather than
  // the values the form held when it was opened.
  refreshKey?: number
}

// Backup policy and location.
//
// Two different questions are answered here and they must not be confused:
//   - what the form will SAVE (interval, hour, retention, subfolder), and
//   - what the job actually WRITES right now (the resolved absolute path and
//     the snapshots already on disk).
// The second one comes from the server status endpoint, so an operator never
// has to guess where "data/backups" ended up relative to the working directory.
export function SiteBackupSection({ cfg, set, refreshKey = 0 }: Props) {
  const { t, locale } = useI18n()
  const { pushToast } = useApp()
  const [st, setSt] = useState<AutoBackupStatus | null>(null)
  const [busy, setBusy] = useState(false)

  const refresh = () => {
    getAutoBackupStatus()
      .then(setSt)
      .catch(() => {
        /* the policy form still works without the status block */
      })
  }

  useEffect(() => {
    refresh()
  }, [refreshKey])

  const runNow = async () => {
    setBusy(true)
    try {
      const r = await runAutoBackup()
      pushToast('info', t('site.backupRunDone', { file: r.file || '—' }))
      await refresh()
    } catch (e) {
      pushToast('error', errMessage(e, t('site.saveFailed')))
    } finally {
      setBusy(false)
    }
  }

  // Additive restore: the server copies only files that are missing from the
  // live upload directory, so repeated clicks are harmless and no picture that
  // is still in use can be overwritten.
  const restoreUploads = async () => {
    const name = st?.uploads_snapshot_name
    if (!name) return
    if (!confirm(t('site.backupRestoreConfirm'))) return
    setBusy(true)
    try {
      const r = await restoreUploadsSnapshot(name)
      pushToast('info', t('site.backupRestoreDone', { n: String(r.restored), s: String(r.skipped) }))
      await refresh()
    } catch (e) {
      pushToast('error', errMessage(e, t('site.saveFailed')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section class="settings-section">
      <h3>{t('admin.autoBackupTitle')}</h3>
      <label class="toggle">
        <input
          type="checkbox"
          checked={cfg.auto_backup}
          onInput={(e) => set('auto_backup', (e.target as HTMLInputElement).checked)}
        />
        {t('site.autoBackup')}
      </label>
      <p class="settings-hint">{t('site.autoBackupHint')}</p>

      <div class="settings-field">
        <label>{t('site.backupInterval')}</label>
        <select
          value={String(cfg.backup_interval_days || 1)}
          onChange={(e) => set('backup_interval_days', Number((e.target as HTMLSelectElement).value))}
        >
          {[1, 3, 7, 14, 30].map((n) => (
            <option key={n} value={String(n)}>{n}</option>
          ))}
        </select>
        <p class="settings-hint">{t('site.backupIntervalHint')}</p>
      </div>

      <div class="settings-field">
        <label>{t('site.backupHour')}</label>
        <select
          value={String(cfg.backup_hour ?? 0)}
          onChange={(e) => set('backup_hour', Number((e.target as HTMLSelectElement).value))}
        >
          <option value="0">{locale === 'zh' ? '不限' : 'Any'}</option>
          {Array.from({ length: 24 }, (_, h) => (
            <option key={h} value={String(h)}>{`${String(h).padStart(2, '0')}:00`}</option>
          ))}
        </select>
        <p class="settings-hint">{t('site.backupHourHint')}</p>
      </div>

      <div class="settings-field">
        <label>{t('site.backupKeep')}</label>
        <select
          value={String(cfg.backup_keep || 7)}
          onChange={(e) => set('backup_keep', Number((e.target as HTMLSelectElement).value))}
        >
          {[3, 7, 14, 30, 90].map((n) => (
            <option key={n} value={String(n)}>{n}</option>
          ))}
        </select>
        <p class="settings-hint">{t('site.backupKeepHint')}</p>
      </div>

      <div class="settings-field">
        <label>{t('site.backupSubdir')}</label>
        <input
          value={cfg.backup_subdir || ''}
          placeholder="daily/2026"
          onInput={(e) => set('backup_subdir', (e.target as HTMLInputElement).value)}
        />
        <p class="settings-hint">{t('site.backupSubdirHint')}</p>
      </div>

      <label class="toggle">
        <input
          type="checkbox"
          checked={cfg.backup_include_uploads}
          onInput={(e) => set('backup_include_uploads', (e.target as HTMLInputElement).checked)}
        />
        {t('site.backupIncludeUploads')}
      </label>
      <p class="settings-hint">{t('site.backupIncludeUploadsHint')}</p>

      {st && (
        <div class="settings-field">
          <label>{t('site.backupLocation')}</label>
          <input value={st.dir_abs || st.dir} readOnly class="media-mono" />
          <p class="settings-hint">
            {st.last_at ? `${t('site.backupLast')}: ${fmtDate(st.last_at, locale)}` : t('site.backupNever')}
            {' · '}
            {t('site.backupFiles', { n: String(st.files), u: String(st.uploads_snapshots) })}
          </p>
          <p class="settings-hint">
            {t('site.backupUploadsLocation')}: {st.uploads_dir_abs || st.uploads_dir}
          </p>
          {st.uploads_snapshot_dir_abs && (
            <p class="settings-hint">
              {t('site.backupUploadsSnapshot')}: {st.uploads_snapshot_dir_abs}
            </p>
          )}
          {st.uploads_snapshot_name && (
            <button class="btn btn-ghost btn-sm" disabled={busy} onClick={restoreUploads}>
              {t('site.backupRestoreUploads')}
            </button>
          )}
          {st.server_now && (
            <p class="settings-hint">
              {t('site.backupServerTime')}: {st.server_now} {st.server_zone}
            </p>
          )}
        </div>
      )}

      <button class="btn btn-ghost btn-sm" disabled={busy || !st?.enabled} onClick={runNow}>
        {busy ? t('common.loading') : t('site.backupRunNow')}
      </button>
    </section>
  )
}
