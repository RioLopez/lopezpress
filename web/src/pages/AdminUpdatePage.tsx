import { useCallback, useEffect, useRef, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { applyUpdate, getUpdateStatus, rollbackUpdate, uploadUpdate, errMessage } from '../core/api'
import type { UpdateStatus } from '../core/types'

// Update center (root-only). The page is a poll target, not a command console:
// apply/rollback answer instantly and the interesting state lives in
// GET /api/admin/update/status. While a job runs (or the service is restarting
// and the API briefly stops answering), polling continues quietly until the
// new process comes back up and the refreshed status shows the new version.

// Phases between "apply" and "the process exits for systemd to restart it".
const ACTIVE_PHASES: ReadonlySet<string> = new Set([
  'checking',
  'downloading',
  'verifying',
  'snapshotting',
  'swapping',
  'restarting',
])

const PHASE_KEY: Record<string, string> = {
  checking: 'admin.phChecking',
  downloading: 'admin.phDownloading',
  verifying: 'admin.phVerifying',
  snapshotting: 'admin.phSnapshotting',
  swapping: 'admin.phSwapping',
  restarting: 'admin.phRestarting',
}

export function AdminUpdatePage() {
  const { t } = useI18n()
  const { pushToast, confirmDialog } = useApp()
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [awaitingRestart, setAwaitingRestart] = useState(false)
  const [busy, setBusy] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const [uploadSha, setUploadSha] = useState('')
  const loadRef = useRef(false)

  const fmtMB = (n?: number) => ((n ?? 0) / 1048576).toFixed(1)

  const load = useCallback(
    async (quiet: boolean) => {
      if (loadRef.current) return
      loadRef.current = true
      try {
        const s = await getUpdateStatus()
        setStatus(s)
        // The restart window is over when a live server answers again and its
        // job state is settled (the new process boots with job.phase=idle).
        if (awaitingRestart && !ACTIVE_PHASES.has(s.job.phase)) {
          setAwaitingRestart(false)
          pushToast('success', t('admin.restartDone'))
        }
      } catch (e) {
        // During a restart the API is down on purpose — swallow quietly.
        if (!quiet && !awaitingRestart) {
          pushToast('error', errMessage(e, t('admin.jobFailed')))
        }
      } finally {
        loadRef.current = false
      }
    },
    [awaitingRestart, pushToast, t],
  )

  useEffect(() => {
    load(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Poll while a job runs or while we are waiting for the restarted process.
  const jobActive = status !== null && ACTIVE_PHASES.has(status.job.phase)
  useEffect(() => {
    if (!jobActive && !awaitingRestart) return
    const iv = setInterval(() => load(true), 2500)
    return () => clearInterval(iv)
  }, [jobActive, awaitingRestart, load])

  const doApply = async () => {
    if (!(await confirmDialog(t('admin.applyConfirm')))) return
    setBusy(true)
    try {
      await applyUpdate()
      pushToast('info', t('admin.applyStarted'))
      await load(true)
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.jobFailed')))
    } finally {
      setBusy(false)
    }
  }

  // Already on the latest release: reinstall the same build (repair path for
  // damaged program files), explicitly confirmed so it cannot be hit by habit.
  const doReinstall = async () => {
    if (!(await confirmDialog(t('admin.reinstallConfirm')))) return
    setBusy(true)
    try {
      await applyUpdate(status?.latest?.tag)
      pushToast('info', t('admin.applyStarted'))
      await load(true)
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.jobFailed')))
    } finally {
      setBusy(false)
    }
  }

  const doUpload = async () => {
    if (!uploadFile) {
      pushToast('error', t('admin.uploadNoFile'))
      return
    }
    if (!(await confirmDialog(t('admin.applyConfirm')))) return
    setBusy(true)
    try {
      await uploadUpdate(uploadFile, uploadSha || undefined)
      pushToast('info', t('admin.uploadStarted'))
      setUploadFile(null)
      await load(true)
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.jobFailed')))
    } finally {
      setBusy(false)
    }
  }

  const doRollback = async (version: string) => {
    if (!(await confirmDialog(t('admin.rollbackConfirm', { version })))) return
    setBusy(true)
    try {
      await rollbackUpdate(version)
      pushToast('info', t('admin.rollbackStarted'))
      setAwaitingRestart(true)
      await load(true)
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.jobFailed')))
    } finally {
      setBusy(false)
    }
  }

  const job = status?.job
  const jobPhaseLabel = job && PHASE_KEY[job.phase] ? t(PHASE_KEY[job.phase]) : ''
  const locked = busy || jobActive || awaitingRestart

  return (
    <div class="admin-page">
      <p class="muted">{t('admin.updateSub')}</p>
      {status?.mirror && <p class="muted update-mirror">UPDATE_MIRROR: {status.mirror}</p>}

      {jobActive && job && (
        <div class="update-banner update-banner--active">
          <strong>{jobPhaseLabel}</strong>
          {job.message && <span> {job.message}</span>}
          {job.phase === 'downloading' && (job.bytes_total ?? 0) > 0 && (
            <div class="update-progress">
              <div class="update-progress-bar">
                <div
                  class="update-progress-fill"
                  style={{ width: `${Math.min(100, Math.round(((job.bytes_done ?? 0) / (job.bytes_total ?? 1)) * 100))}%` }}
                />
              </div>
              <span class="muted">{fmtMB(job.bytes_done)} / {fmtMB(job.bytes_total)} MB</span>
            </div>
          )}
        </div>
      )}
      {awaitingRestart && (
        <div class="update-banner update-banner--active">{t('admin.restartPending')}</div>
      )}
      {job && job.phase === 'failed' && (
        <div class="update-banner update-banner--error">
          <strong>{t('admin.jobFailed')}</strong>
          {job.error && <span> {job.error}</span>}
        </div>
      )}

      <div class="update-cards">
        <div class="update-card">
          <div class="update-card-title">{t('admin.currentVersion')}</div>
          <div class="update-card-version">{status?.current.version ?? '—'}</div>
          <div class="update-card-meta muted">
            {t('admin.commit')}: {status?.current.commit ?? '—'}
            {status?.current.built && <> &middot; {t('admin.builtAt')}: {status.current.built}</>}
          </div>
          {status?.current.dirty && <span class="badge badge-root">{t('admin.dirtyBadge')}</span>}
        </div>

        <div class="update-card">
          <div class="update-card-title">{t('admin.latestRelease')}</div>
          {status?.latest ? (
            <>
              <div class="update-card-version">{status.latest.tag}</div>
              <div class="update-card-meta muted">
                {t('admin.publishedAt')}: {new Date(status.latest.published_at).toLocaleString()}
              </div>
              <span class={`update-badge ${status.update_available ? 'update-badge--new' : 'update-badge--ok'}`}>
                {status.update_available ? t('admin.updateAvailable') : t('admin.upToDate')}
              </span>
              {status.latest.notes && (
                <details class="update-notes">
                  <summary class="muted">{t('admin.releaseNotes')}</summary>
                  <pre>{status.latest.notes}</pre>
                </details>
              )}
              <div class="update-card-actions">
                {status.update_available ? (
                  <button class="btn btn-primary" disabled={locked} onClick={doApply}>
                    {t('admin.applyUpdate')}
                  </button>
                ) : (
                  // Up to date: the loud button goes away; what remains is the
                  // quiet repair path — reinstalling the same build.
                  <button class="btn btn-ghost" disabled={locked} onClick={doReinstall}>
                    {t('admin.reinstallBtn')}
                  </button>
                )}
                <a class="btn btn-ghost" href={status.latest.url} target="_blank" rel="noreferrer">
                  {t('admin.viewRelease')}
                </a>
              </div>
            </>
          ) : (
            <p class="muted">{t('admin.noRelease')}</p>
          )}
        </div>
      </div>

      <div class="update-section">
        <h2>{t('admin.rollbackTitle')}</h2>
        <p class="muted">{t('admin.rollbackSub')}</p>
        {(status?.rollback.length ?? 0) === 0 ? (
          <p class="muted">{t('admin.rollbackEmpty')}</p>
        ) : (
          <div class="table-scroll">
            <table class="admin-table">
              <thead>
                <tr>
                  <th>#</th>
                  <th></th>
                  <th class="actions"></th>
                </tr>
              </thead>
              <tbody>
                {status!.rollback.map((r) => (
                  <tr key={r.version}>
                    <td>{r.version}</td>
                    <td class="muted">{r.binary && r.web ? 'binary + web' : 'binary'}</td>
                    <td class="actions">
                      <button
                        class="btn btn-ghost btn-sm"
                        disabled={locked}
                        onClick={() => doRollback(r.version)}
                      >
                        {t('admin.rollbackBtn')}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div class="update-section">
        <h2>{t('admin.uploadTitle')}</h2>
        <p class="muted">{t('admin.uploadSub')}</p>
        <div class="update-upload">
          <input
            type="file"
            accept=".tar.gz,.gz"
            disabled={locked}
            onChange={(e) => setUploadFile((e.target as HTMLInputElement).files?.[0] ?? null)}
          />
          <input
            class="update-sha"
            type="text"
            placeholder={t('admin.shaHint')}
            disabled={locked}
            value={uploadSha}
            onChange={(e) => setUploadSha((e.target as HTMLInputElement).value)}
          />
          <button class="btn btn-primary" disabled={locked || !uploadFile} onClick={doUpload}>
            {t('admin.uploadBtn')}
          </button>
        </div>
      </div>

      <div class="update-section">
        <h2>{t('admin.historyTitle')}</h2>
        {(status?.history.length ?? 0) === 0 ? (
          <p class="muted">{t('admin.historyEmpty')}</p>
        ) : (
          <div class="table-scroll">
            <table class="admin-table">
              <thead>
                <tr>
                  <th></th>
                  <th></th>
                  <th></th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {status!.history.map((h, i) => (
                  <tr key={i}>
                    <td>{h.action === 'update' ? t('admin.hisUpdate') : h.action === 'rollback' ? t('admin.hisRollback') : t('admin.hisNote')}</td>
                    <td class="muted">{h.from} → {h.to}</td>
                    <td class="muted">{new Date(h.at).toLocaleString()}</td>
                    <td>
                      <span class={`update-badge update-result--${h.result}`}>
                        {h.result === 'success'
                          ? t('admin.resultSuccess')
                          : h.result === 'failed'
                            ? t('admin.resultFailed')
                            : t('admin.resultPending')}
                      </span>
                      {h.note && <span class="muted"> — {h.note}</span>}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
