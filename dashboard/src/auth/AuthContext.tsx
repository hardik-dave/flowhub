import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'
import { dash } from '../api/client'
import type { SessionUser } from '../api/types'
import * as storage from './storage'

interface AuthState {
  user: SessionUser | null
  token: string | null
  isPlatformAdmin: boolean
  actAsTenant: number | null
  login: (username: string, password: string) => Promise<void>
  logout: () => Promise<void>
  setActAsTenant: (id: number | null) => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUserState] = useState<SessionUser | null>(() => storage.getUser())
  const [token, setTokenState] = useState<string | null>(() => storage.getToken())
  const [actAsTenant, setActAsState] = useState<number | null>(() => storage.getActAsTenant())

  const login = useCallback(async (username: string, password: string) => {
    const res = await dash.login(username, password)
    storage.setToken(res.session_token)
    storage.setUser(res.user)
    setTokenState(res.session_token)
    setUserState(res.user)
  }, [])

  const logout = useCallback(async () => {
    try {
      await dash.logout()
    } catch {
      // Signing out is best-effort: clear the local session regardless.
    }
    storage.clearSession()
    setTokenState(null)
    setUserState(null)
    setActAsState(null)
  }, [])

  const setActAsTenant = useCallback((id: number | null) => {
    storage.setActAsTenant(id)
    setActAsState(id)
  }, [])

  const value = useMemo<AuthState>(
    () => ({
      user,
      token,
      isPlatformAdmin: user?.role === 'SUPERADMIN',
      actAsTenant,
      login,
      logout,
      setActAsTenant,
    }),
    [user, token, actAsTenant, login, logout, setActAsTenant],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider')
  return ctx
}
