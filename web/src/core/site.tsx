// Site config context: fetches the PUBLIC site config once on boot and
// exposes it (plus a reload) so the brand, document title and footer render
// consistently across the whole app. The fetched payload carries no secrets.

import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { getSiteConfig } from './api'
import type { SiteConfigPublic } from './types'

const DEFAULT: SiteConfigPublic = {
  site_name: 'Echo',
  site_title: 'Echo',
  site_logo: '',
  site_favicon: '',
  site_url: '',
  site_description: '',
  footer_text: '',
  icp_no: '',
  friend_links: '',
  page_size: 10,
  // Until the real config arrives, assume public: a blog should not flash a
  // login redirect at a visitor while the config request is in flight.
  public_read: true,
  // Spec 37: off until the server says otherwise, matching the schema default.
  email_verify_enabled: false,
}

interface SiteContextValue {
  config: SiteConfigPublic
  reload: () => void
}

const SiteContext = createContext<SiteContextValue | null>(null)

export function SiteProvider({ children }: { children: ComponentChildren }) {
  const [config, setConfig] = useState<SiteConfigPublic>(DEFAULT)

  const reload = () => {
    getSiteConfig()
      .then(setConfig)
      .catch(() => {
        /* site config is non-critical; keep defaults on failure */
      })
  }

  // Reflect the configured brand across the whole app: browser tab title and
  // favicon. Kept here (not in Layout) so it applies to admin pages too.
  useEffect(() => {
    if (config.site_title) document.title = config.site_title
    if (config.site_favicon) {
      let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
      if (!link) {
        link = document.createElement('link')
        link.rel = 'icon'
        document.head.appendChild(link)
      }
      link.href = config.site_favicon
    }
  }, [config.site_title, config.site_favicon])

  useEffect(() => {
    reload()
  }, [])

  return <SiteContext.Provider value={{ config, reload }}>{children}</SiteContext.Provider>
}

export function useSite() {
  const v = useContext(SiteContext)
  if (!v) throw new Error('useSite must be used within <SiteProvider>')
  return v
}
