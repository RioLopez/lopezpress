import { useSite } from '../core/site'
import { useI18n } from '../core/i18n'
import type { FriendLink } from '../core/types'

// Site-wide footer: copyright text, ICP/license link and friend links. All
// values come from the public site config (spec 05). Renders nothing extra
// when fields are empty.
export function Footer() {
  const { config } = useSite()
  const { t } = useI18n()

  let links: FriendLink[] = []
  if (config.friend_links) {
    try {
      const parsed = JSON.parse(config.friend_links)
      if (Array.isArray(parsed)) links = parsed
    } catch {
      /* ignore malformed friend-links JSON */
    }
  }

  const hasContent = config.footer_text || config.icp_no || links.length > 0

  return (
    <footer class="site-footer">
      <div class="site-footer-inner">
        {config.footer_text && <span class="footer-copy">{config.footer_text}</span>}
        {config.icp_no && (
          <a class="footer-icp" href="https://beian.miit.gov.cn/" target="_blank" rel="noreferrer">
            {config.icp_no}
          </a>
        )}
        {links.length > 0 && (
          <span class="footer-links">
            {links.map((l, i) => (
              <a key={i} href={l.url} target="_blank" rel="noreferrer">
                {l.name}
              </a>
            ))}
          </span>
        )}
        <span class="footer-links">
          <a href="/feed" rel="alternate" type="application/rss+xml">{t('site.footerRss')}</a>
          <a href="/sitemap.xml" target="_blank" rel="noreferrer">{t('site.footerSitemap')}</a>
        </span>
        {!hasContent && <span class="footer-copy">Echo Content</span>}
      </div>
    </footer>
  )
}
