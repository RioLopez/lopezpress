import { useI18n } from '../core/i18n'
import { useNav } from '../core/router'

export function NotFoundPage() {
  const { t } = useI18n()
  const [, navigate] = useNav()
  return (
    <div class="notfound">
      <h1>404</h1>
      <p>{t('notfound.text')}</p>
      <button class="btn btn-primary" onClick={() => navigate('/')}>
        {t('notfound.home')}
      </button>
    </div>
  )
}
