// Auth context: user + session state.
//
// SECURITY: the JWT is issued by the server as an HttpOnly, SameSite=Lax cookie
// (see internal/handlers/auth.go). The browser sends it automatically on
// same-origin requests, and JavaScript can NEVER read it — so an XSS bug can no
// longer walk off with the session token. This module therefore stores NO token
// in JS-accessible storage.
//
// The only thing kept in localStorage is a non-secret *marker* (`echo.session`)
// that merely records "a session exists" so the route guards can render
// instantly without waiting for /api/auth/me. It is not a credential and an
// attacker who reads it gains nothing — the real token lives in the HttpOnly
// cookie the browser refuses to expose to script.

import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { me as getMe, login as apiLogin, logout as apiLogout, register as apiRegister } from './api'
import { setTokenGetter, setOnUnauthorized, setOnPasswordChangeRequired } from './api'
import type { AuthResult, UserPublic } from './types'

// Marker only — NOT a token. Lets the guards decide instantly; never sent as a
// credential.
const SESSION_KEY = 'echo.session'

// No Authorization header: the HttpOnly cookie is sent automatically by the
// browser on same-origin requests. This must be armed at MODULE LOAD so child
// effects (which flush first) already see it.
setTokenGetter(() => '')

interface AuthContextValue {
  user: UserPublic | null
  loading: boolean
  login: (
    email: string,
    password: string,
    captcha?: { id?: string; code?: string },
  ) => Promise<UserPublic>
  register: (u: string, e: string, p: string, code?: string) => Promise<void>
  logout: () => void
  refresh: () => Promise<void>
  // adoptToken is called after a password change, which rotates the token. The
  // server already re-issued the HttpOnly cookie on the change-password
  // response, so we just re-verify the session; the argument is ignored.
  adoptToken: (token: string) => Promise<void>
  token: string
  hasToken: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ComponentChildren }) {
  const [user, setUser] = useState<UserPublic | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setOnUnauthorized(() => {
      localStorage.removeItem(SESSION_KEY)
      setUser(null)
      if (!location.pathname.startsWith('/login')) location.assign('/login')
    })
    setOnPasswordChangeRequired(() => {
      setUser((u) => (u ? { ...u, must_change_password: true } : u))
      if (!location.pathname.startsWith('/change-password')) location.assign('/change-password')
    })

    // Boot: if the marker says a session exists, verify it via /api/auth/me.
    // The HttpOnly cookie is sent automatically; we never read it from JS.
    if (!localStorage.getItem(SESSION_KEY)) {
      setLoading(false)
      return
    }
    getMe()
      .then((u: UserPublic) => setUser(u))
      .catch(() => {
        localStorage.removeItem(SESSION_KEY)
        setUser(null)
      })
      .finally(() => setLoading(false))
  }, [])

  const doLogin = async (
    email: string,
    password: string,
    captcha?: { id?: string; code?: string },
  ): Promise<UserPublic> => {
    const res: AuthResult = await apiLogin({
      email,
      password,
      captcha_id: captcha?.id,
      captcha_code: captcha?.code,
    })
    // The server set the HttpOnly cookie; we only keep the non-secret marker.
    localStorage.setItem(SESSION_KEY, '1')
    setUser(res.user)
    return res.user
  }

  const doRegister = async (u: string, e: string, p: string, code?: string): Promise<void> => {
    const res: AuthResult = await apiRegister({ username: u, email: e, password: p, code })
    localStorage.setItem(SESSION_KEY, '1')
    setUser(res.user)
  }

  const doLogout = () => {
    localStorage.removeItem(SESSION_KEY)
    setUser(null)
    apiLogout().catch(() => {})
  }

  const doRefresh = async () => {
    try {
      const u = await getMe()
      setUser(u)
    } catch {
      /* ignore: keep current user on refresh failure */
    }
  }

  // The server already rotated the HttpOnly cookie on the change-password
  // response; re-verify so the user object reflects the post-change state.
  const doAdoptToken = async (_t: string) => {
    try {
      setUser(await getMe())
    } catch {
      /* keep the current user; the next request will re-validate */
    }
  }

  const value: AuthContextValue = {
    user,
    loading,
    login: doLogin,
    register: doRegister,
    logout: doLogout,
    refresh: doRefresh,
    adoptToken: doAdoptToken,
    token: '',
    hasToken: !!localStorage.getItem(SESSION_KEY) || !!user,
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const v = useContext(AuthContext)
  if (!v) throw new Error('useAuth must be used within <AuthProvider>')
  return v
}
