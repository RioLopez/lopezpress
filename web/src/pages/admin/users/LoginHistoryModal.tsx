import { useI18n } from '../../../core/i18n'
import { fmtDate } from '../../../core/format'
import type { LoginLog, UserWithDisabled } from '../../../core/types'
import { Modal } from '../../../ui/Modal'

export function LoginHistoryModal({
  user,
  logs,
  loading,
  onClose,
}: {
  user: UserWithDisabled
  logs: LoginLog[]
  loading: boolean
  onClose: () => void
}) {
  const { t, locale } = useI18n()

  return (
    <Modal
      wide
      title={`${t('admin.loginHistory')} — ${user.username}`}
      onClose={onClose}
      footer={<button class="btn btn-ghost" onClick={onClose}>{t('editor.cancel')}</button>}
    >
      {loading ? (
        <p class="muted">{t('common.loading')}</p>
      ) : logs.length === 0 ? (
        <p class="muted">{t('admin.noLogs')}</p>
      ) : (
        <div class="table-scroll">
          <table class="admin-table login-log-table admin-table--cards">
            <thead>
              <tr>
                <th>{t('admin.loginTime')}</th>
                <th>{t('admin.loginIp')}</th>
                <th>{t('admin.loginOk')}</th>
                <th>UA</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id}>
                  <td class="muted" data-label={t('admin.loginTime')}>{fmtDate(l.created_at, locale)}</td>
                  <td class="muted" data-label={t('admin.loginIp')}>{l.ip}</td>
                  <td data-label={t('admin.loginOk')}>
                    <span class={`post-status post-status-${l.success ? 'published' : 'draft'}`}>
                      {l.success ? t('admin.loginOk') : t('admin.loginFail')}
                    </span>
                  </td>
                  <td class="muted ua-cell" data-label="UA" title={l.ua}>{l.ua}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  )
}
