import { useState } from 'preact/hooks'
import { useApp } from '../../../core/app'
import { useI18n } from '../../../core/i18n'
import { passwordProblem } from '../../../core/password'
import { resetPassword, errMessage } from '../../../core/api'
import type { UserWithDisabled } from '../../../core/types'
import { Modal } from '../../../ui/Modal'

export function ResetPasswordModal({
  user,
  onClose,
  onDone,
}: {
  user: UserWithDisabled
  onClose: () => void
  onDone: () => void
}) {
  const { pushToast, confirmDialog } = useApp()
  const { t } = useI18n()
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    const problem = passwordProblem(password)
    if (problem) {
      pushToast('error', t(problem === 'long' ? 'auth.passwordTooLong' : 'auth.passwordTooShort'))
      return
    }
    if (!(await confirmDialog(t('admin.resetPwdConfirm', { t: user.username })))) return
    setBusy(true)
    try {
      await resetPassword(user.id, password)
      pushToast('success', t('admin.saved'))
      onDone()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={`${t('admin.resetPwdTitle')} — ${user.username}`}
      onClose={onClose}
      footer={
        <>
          <button class="btn btn-ghost" onClick={onClose}>{t('editor.cancel')}</button>
          <button class="btn btn-primary" disabled={busy} onClick={submit}>
            {busy ? '…' : t('editor.save')}
          </button>
        </>
      }
    >
      <div class="settings-field">
        <label>{t('admin.newPassword')}</label>
        <input
          type="text"
          placeholder={t('admin.newPwdPlaceholder')}
          value={password}
          onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
        />
      </div>
    </Modal>
  )
}
