import { useEffect, useState } from 'preact/hooks'
import { useI18n } from '../core/i18n'
import { useSite } from '../core/site'

// Reader-facing explanation of the site's feed, at /subscribe.
//
// Why this page exists: the footer used to link straight at /feed and
// /sitemap.xml, so "RSS" and "站点地图" both dumped raw XML into the tab. That
// looked broken, and it was the wrong offer — a sitemap is a crawler's index,
// not a reader's navigation, and a feed is a document for a program to poll.
// Neither is a page.
//
// So the footer now points here instead, and this page does the one job the
// raw XML cannot: say what the address is for, make it copyable, and set the
// expectation that opening it shows source. /feed and /sitemap.xml keep
// working unchanged for readers and crawlers — only the human entry point
// moved.
export function SubscribePage() {
  const { t } = useI18n()
  const { config } = useSite()
  const [copied, setCopied] = useState(false)

  // Absolute, and derived from site_url when the operator set one: a reader
  // pasting the address into another device needs the public URL, not this
  // browser's idea of the origin (behind a proxy those differ).
  const origin = config.site_url?.trim().replace(/\/+$/, '') || window.location.origin
  const feedUrl = `${origin}/feed`

  // Copy is a two-state button; reset it so a second copy is not silent.
  useEffect(() => {
    if (!copied) return
    const id = setTimeout(() => setCopied(false), 1800)
    return () => clearTimeout(id)
  }, [copied])

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(feedUrl)
      setCopied(true)
    } catch {
      // Clipboard API needs a secure context; over plain HTTP it is absent.
      // Select the text instead so the address is still one keystroke away.
      const el = document.getElementById('feed-url') as HTMLInputElement | null
      el?.select()
    }
  }

  return (
    <div class="subscribe-page">
      <h1 class="subscribe-title">{t('feed.title')}</h1>
      <p class="subscribe-intro">{t('feed.intro')}</p>

      {!config.public_read && <p class="subscribe-note">{t('feed.readLocked')}</p>}

      <label class="subscribe-label" for="feed-url">
        {t('feed.address')}
      </label>
      <div class="subscribe-row">
        <input id="feed-url" class="subscribe-input" readonly value={feedUrl} />
        <button class="btn btn-primary" onClick={copy}>
          {copied ? t('feed.copied') : t('feed.copy')}
        </button>
      </div>

      <h2 class="subscribe-h2">{t('feed.readersTitle')}</h2>
      <p class="subscribe-body">{t('feed.readersHint')}</p>

      <h2 class="subscribe-h2">{t('feed.noteTitle')}</h2>
      <p class="subscribe-body">{t('feed.note')}</p>
      <a class="subscribe-raw" href="/feed" target="_blank" rel="noreferrer">
        {t('feed.open')}
      </a>
    </div>
  )
}
