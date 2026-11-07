import { useState } from 'preact/hooks'
import { useApp } from '../../../core/app'
import { useAuth } from '../../../core/auth'
import { useI18n } from '../../../core/i18n'
import { passwordProblem } from '../../../core/password'
import { createUser, errMessage } from '../../../core/api'
import { isRoot } from '../../../core/roles'
import { Modal } from '../../../ui/Modal'

export function CreateUserModal({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: () => void
}) {
  const { pushToast } = useApp()
  const { t } = useI18n()
  const { user } = useAuth()
  const isRootUser = isRoot(user?.role)
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('user')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    if (!username.trim() || !email.trim() || !password) {
      pushToast('error', t('auth.empty'))
      return
    }
    const problem = passwordProblem(password)
    if (problem) {
      pushToast('error', t(problem === 'long' ? 'auth.passwordTooLong' : 'auth.passwordTooShort'))
      return
    }
    setBusy(true)
    try {
      await createUser({ username: username.trim(), email: email.trim(), password, role })
      pushToast('success', t('admin.saved'))
      onCreated()
    } catch (e) {
      pushToast('error', errMessage(e, t('admin.failed')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={t('admin.newUser')}
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
        <label>{t('admin.newUsername')}</label>
        <input value={username} onInput={(e) => setUsername((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-field">
        <label>{t('admin.newEmail')}</label>
        <input type="email" value={email} onInput={(e) => setEmail((e.target as HTMLInputElement).value)} />
      </div>
      <div class="settings-field">
        <label>{t('admin.newPassword')}</label>
        <input
          type="text"
          value={password}
          onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
        />
      </div>
      <div class="settings-field">
        <label>{t('admin.newRole')}</label>
        <select value={role} onChange={(e) => setRole((e.target as HTMLSelectElement).value)}>
          <option value="user">{t('admin.roleUser')}</option>
          {/* Only root may provision admins; admins create plain users. */}
          {isRootUser && <option value="admin">{t('admin.roleAdmin')}</option>}
        </select>
      </div>
    </Modal>
  )
}
