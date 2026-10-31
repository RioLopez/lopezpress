// i18n entry point. The 536-key dictionary is split per domain under
// `locales/` (see ARCH_REVIEW F6); this file only composes them and exposes
// the `useI18n()` hook. `t()` warns on a missing key in dev and falls back to
// English in prod instead of echoing the raw key.

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
import { FeedTexts, feedZh, feedEn } from './locales/feed'
import { CommonTexts, commonZh, commonEn } from './locales/common'
import { setErrorTranslator } from './http'

export type Locale = 'zh' | 'en'

// Status codes whose cause is mechanical: the server's English text ("file too
// large", "internal error") tells the reader nothing they can act on, so the
// UI supplies its own line in the active language. Deliberately narrow — a 400
// from the password policy must keep its server text, because that text names
// the rule that failed.
const ERROR_TEXT_KEYS: Record<number, string> = {
  413: 'common.tooLarge',
  429: 'common.tooManyRequests',
  500: 'common.serverError',
  503: 'common.unavailable',
}

export type Texts = NavTexts &
  AuthTexts &
  CenterTexts &
  ReaderTexts &
  EditorTexts &
  AdminTexts &
  SiteTexts &
  FeedTexts &
  CommonTexts

const zh: Texts = {
  ...navZh,
  ...authZh,
  ...centerZh,
  ...readerZh,
  ...editorZh,
  ...adminZh,
  ...siteZh,
  ...feedZh,
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
  ...feedEn,
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

  // Teach errMessage() the active language, so a toast raised from any of the
  // ~60 call sites reads in the same language as the page around it.
  useEffect(() => {
    setErrorTranslator((status) => {
      const key = ERROR_TEXT_KEYS[status]
      return key ? lookup(dicts[locale], key) : undefined
    })
    return () => setErrorTranslator(null)
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
