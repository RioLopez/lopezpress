import { useCallback, useEffect, useRef, useState } from 'preact/hooks'
import { useApp } from '../core/app'
import { useI18n } from '../core/i18n'
import { applyUpdate, getUpdateStatus, rollbackUpdate, errMessage } from '../core/api'
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
  const loadRef = useRef(false)

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

      {jobActive && job && (
        <div class="update-banner update-banner--active">
          <strong>{jobPhaseLabel}</strong>
          {job.message && <span> {job.message}</span>}
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
                <button class="btn btn-primary" disabled={locked} onClick={doApply}>
                  {t('admin.applyUpdate')}
                </button>
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
