import { useSite } from '../core/site'
import { useI18n } from '../core/i18n'
import type { FriendLink } from '../core/types'

// Site-wide footer: copyright text, ICP/license link and friend links. All
// values come from the public site config. Renders nothing extra
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
          {/* Both links used to point at the raw documents (/feed and
              /sitemap.xml), so clicking either dumped XML into the tab. Two
              different problems behind that:
                - /sitemap.xml is not a reader-facing page at all. It is a list
                  of URLs for crawlers, and it is already advertised in
                  robots.txt, which is the only place it needs to be. There is
                  nothing for a visitor to do with it, so the footer no longer
                  offers it.
                - /feed IS worth offering (it is how a reader subscribes), but
                  it is also XML by definition. The fix is not to stop linking
                  it, it is to link the explanation instead: /subscribe states
                  what the address is, makes it copyable, and sets the
                  expectation that opening it shows source.
              The net effect: neither footer link can surprise anyone with a
              screen of angle brackets. */}
          <a href="/subscribe">{t('feed.footerLink')}</a>
        </span>
        {!hasContent && <span class="footer-copy">Echo Content</span>}
      </div>
    </footer>
  )
}
