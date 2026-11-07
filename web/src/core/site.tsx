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
  theme_color: '',
  home_layout: 'list',
  icp_no: '',
  friend_links: '',
  page_size: 10,
  // Until the real config arrives, assume public: a blog should not flash a
  // login redirect at a visitor while the config request is in flight.
  public_read: true,
 // Off until the server says otherwise, matching the schema default.
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

  // Appearance: the accent is one inline custom property, so it beats the
  // stylesheet without a second theme file. An empty value removes the
  // override and the built-in palette returns — that is why "clear" is a
  // meaningful choice rather than "pick the old colour from memory".
  useEffect(() => {
    applyThemeColor(config.theme_color)
  }, [config.theme_color])

  return <SiteContext.Provider value={{ config, reload }}>{children}</SiteContext.Provider>
}

// applyThemeColor derives the hover shade and the readable foreground from one
// hex. The foreground uses relative luminance rather than a fixed white, because
// a pale accent with white text is unreadable and a dark one with near-black
// text is worse.
function applyThemeColor(hex: string) {
  const root = document.documentElement
  if (!hex) {
    root.style.removeProperty('--primary')
    root.style.removeProperty('--primary-hover')
    root.style.removeProperty('--primary-fg')
    return
  }
  const m = /^#([0-9a-f]{6})$/i.exec(hex)
  if (!m) return
  const n = parseInt(m[1], 16)
  const r = (n >> 16) & 255
  const g = (n >> 8) & 255
  const b = n & 255
  const dark = (v: number) => Math.round(v * 0.86)
  root.style.setProperty('--primary', hex)
  root.style.setProperty('--primary-hover', `rgb(${dark(r)}, ${dark(g)}, ${dark(b)})`)
  const lum = (0.299 * r + 0.587 * g + 0.114 * b) / 255
  root.style.setProperty('--primary-fg', lum > 0.65 ? '#1b1a17' : '#ffffff')
}

export function useSite() {
  const v = useContext(SiteContext)
  if (!v) throw new Error('useSite must be used within <SiteProvider>')
  return v
}
