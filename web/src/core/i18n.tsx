// i18n entry point. The 536-key dictionary is split per domain under
// `locales/` (see ARCH_REVIEW F6); this file only composes them and exposes
// the `useI18n()` hook. `t()` warns on a missing key in dev and falls back to
// English in prod instead of echoing the raw key (the P0-3 failure mode).

import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { NavTexts, navZh, navEn } from './locales/nav'
import { AuthTexts, authZh, authEn } from './locales/auth'
import { CenterTexts, centerZh, centerEn } from './locales/center'
import { ReaderTexts, readerZh, readerEn } from './locales/reader'
import { EditorTexts, editorZh, editorEn } from './locales/editor'
import { AdminTexts, adminZh, adminEn } from './locales/admin'
import { SiteTexts, siteZh, siteEn } from './locales/site'
import { CommonTexts, commonZh, commonEn } from './locales/common'

export type Locale = 'zh' | 'en'

export type Texts = NavTexts &
  AuthTexts &
  CenterTexts &
  ReaderTexts &
  EditorTexts &
  AdminTexts &
  SiteTexts &
  CommonTexts

const zh: Texts = {
  ...navZh,
  ...authZh,
  ...centerZh,
  ...readerZh,
  ...editorZh,
  ...adminZh,
  ...siteZh,
  ...commonZh,
}

const en: Texts = {
  ...navEn,
  ...authEn,
  ...centerEn,
  ...readerEn,
  ...editorEn,
  ...adminEn,
  ...siteEn,
  ...commonEn,
}

const dicts: Record<Locale, Texts> = { zh, en }
const LOCALE_KEY = 'echo.locale'

function detect(): Locale {
  const saved = localStorage.getItem(LOCALE_KEY) as Locale | null
  if (saved === 'zh' || saved === 'en') return saved
  return navigator.language?.toLowerCase().startsWith('en') ? 'en' : 'zh'
}

type T = (key: string, vars?: Record<string, string>) => string

interface I18nValue {
  locale: Locale
  setLocale: (l: Locale) => void
  t: T
}

const I18nContext = createContext<I18nValue | null>(null)

// Look up a dotted key in one dictionary; returns undefined when missing.
function lookup(dict: Texts, key: string): string | undefined {
  const parts = key.split('.')
  let node: unknown = dict
  for (const p of parts) node = (node as Record<string, unknown>)?.[p]
  return typeof node === 'string' ? node : undefined
}

export function I18nProvider({ children }: { children: ComponentChildren }) {
  const [locale, setLocale] = useState<Locale>(detect)

  useEffect(() => {
    document.documentElement.lang = locale
  }, [locale])

  const apply = (l: Locale) => {
    localStorage.setItem(LOCALE_KEY, l)
    setLocale(l)
  }

  const t: T = (key, vars) => {
    let out = lookup(dicts[locale], key)
    if (out === undefined) {
      // Missing key: never surface the raw key to users. Warn in dev so the
      // bug is caught during development, and fall back to English in prod.
      if (import.meta.env.DEV) console.warn(`[i18n] missing key: ${key}`)
      out = lookup(dicts.en, key) ?? key
    }
    if (vars) for (const [k, v] of Object.entries(vars)) out = out.replace(`{${k}}`, v)
    return out
  }

  return (
    <I18nContext.Provider value={{ locale, setLocale: apply, t }}>
      {children}
    </I18nContext.Provider>
  )
}

export function useI18n(): I18nValue {
  const v = useContext(I18nContext)
  if (!v) throw new Error('useI18n must be used within <I18nProvider>')
  return v
}
