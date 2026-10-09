// Raw, dependency-free persistence for the dashboard session. Kept out of
// React so the API client can read the token without a circular import.

import type { SessionUser } from '../api/types'

const TOKEN_KEY = 'flowhub.token'
const USER_KEY = 'flowhub.user'
const ACT_AS_TENANT_KEY = 'flowhub.actAsTenant'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function getUser(): SessionUser | null {
  const raw = localStorage.getItem(USER_KEY)
  if (!raw) return null
  try {
    return JSON.parse(raw) as SessionUser
  } catch {
    return null
  }
}

export function setUser(user: SessionUser): void {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearSession(): void {
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
  localStorage.removeItem(ACT_AS_TENANT_KEY)
}

export function getActAsTenant(): number | null {
  const raw = localStorage.getItem(ACT_AS_TENANT_KEY)
  if (!raw) return null
  const n = Number(raw)
  return Number.isInteger(n) && n > 0 ? n : null
}

export function setActAsTenant(id: number | null): void {
  if (id === null) {
    localStorage.removeItem(ACT_AS_TENANT_KEY)
  } else {
    localStorage.setItem(ACT_AS_TENANT_KEY, String(id))
  }
}
