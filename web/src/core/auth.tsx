// Auth context: token + user, persisted to localStorage so the SPA can
// survive a reload and re-verify via GET /api/auth/me on boot.

import { createContext } from 'preact'
import { useContext, useEffect, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import { me as getMe, login as apiLogin, logout as apiLogout, register as apiRegister } from './api'
import { setTokenGetter, setOnUnauthorized } from './api'
import type { AuthResult, UserPublic } from './types'

const TOKEN_KEY = 'echo.token'

// The token getter must be armed at MODULE LOAD, not inside the provider's
// useEffect: child components' effects flush before the provider's, so a
// getter installed there would leave first-render requests (e.g.
// /api/me/stats on /me) without an Authorization header → 401 →
// onUnauthorized → a hard refresh of any authed page bounces to /login.
setTokenGetter(() => localStorage.getItem(TOKEN_KEY) || '')

interface AuthContextValue {
  user: UserPublic | null
  loading: boolean
  login: (email: string, password: string) => Promise<UserPublic>
  // code is only used while the site has email verification switched on
  // (spec 37, experimental); pass undefined otherwise.
  register: (u: string, e: string, p: string, code?: string) => Promise<void>
  logout: () => void
  refresh: () => Promise<void>
  // adoptToken swaps in a server-issued token (e.g. the fresh one returned by
  // a password change, which also bumps the token version and so revokes the
  // previous token). It persists the token and re-verifies the user.
  adoptToken: (token: string) => Promise<void>
  token: string
  hasToken: boolean
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ComponentChildren }) {
  const [user, setUser] = useState<UserPublic | null>(null)
  const [loading, setLoading] = useState(true)

  // On mount: re-verify the stored token (if any) via GET /api/auth/me.
  // (The token getter itself is armed at module load — see above.)
  useEffect(() => {
    // Global 401 handling: a non-auth endpoint rejecting our token means it
    // expired — clear the session and bounce to login (unless already there).
    setOnUnauthorized(() => {
      localStorage.removeItem(TOKEN_KEY)
      setUser(null)
      if (!location.pathname.startsWith('/login')) location.assign('/login')
    })
    const t = localStorage.getItem(TOKEN_KEY)
    if (!t) {
      setLoading(false)
      return
    }
    getMe()
      .then((u: UserPublic) => setUser(u))
      .catch(() => {
        localStorage.removeItem(TOKEN_KEY)
        setUser(null)
      })
      .finally(() => setLoading(false))
  }, [])

  const doLogin = async (email: string, password: string): Promise<UserPublic> => {
    const res: AuthResult = await apiLogin({ email, password })
    localStorage.setItem(TOKEN_KEY, res.token)
    setUser(res.user)
    return res.user
  }

  const doRegister = async (u: string, e: string, p: string, code?: string): Promise<void> => {
    // The register endpoint issues a JWT itself (spec 04 AC-2.2), so the new
    // account is signed in immediately without a second login round trip.
    const res: AuthResult = await apiRegister({ username: u, email: e, password: p, code })
    localStorage.setItem(TOKEN_KEY, res.token)
    setUser(res.user)
  }

  const doLogout = () => {
    localStorage.removeItem(TOKEN_KEY)
    setUser(null)
    // Best-effort: call server logout (stateless, mostly a no-op signal).
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

  // Replace the stored token with a server-issued one and re-verify, so a
  // session that just rotated its token (password change) keeps working and
  // the user object reflects the post-change state (e.g. cleared flag).
  const doAdoptToken = async (t: string) => {
    localStorage.setItem(TOKEN_KEY, t)
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
    token: user ? localStorage.getItem(TOKEN_KEY) || '' : '',
    hasToken: !!localStorage.getItem(TOKEN_KEY),
  }

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const v = useContext(AuthContext)
  if (!v) throw new Error('useAuth must be used within <AuthProvider>')
  return v
}
