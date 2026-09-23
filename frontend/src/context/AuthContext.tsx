import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from 'react'
import { ApiError, setUnauthorizedHandler } from '../api/client'
import { authApi } from '../api/endpoints'
import type { User } from '../api/types'

interface AuthContextValue {
  user: User | null
  loading: boolean
  sessionExpired: boolean
  dismissSessionExpired: () => void
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, timezone: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [sessionExpired, setSessionExpired] = useState(false)
  const userRef = useRef<User | null>(null)

  useEffect(() => {
    userRef.current = user
  }, [user])

  // A 401 on any request (not just the initial /me check) means the
  // session cookie expired or was revoked server-side. Clearing user here
  // makes App's RequireAuth redirect to /login on the next render.
  useEffect(() => {
    setUnauthorizedHandler(() => {
      if (userRef.current) setSessionExpired(true)
      setUser(null)
    })
    return () => setUnauthorizedHandler(null)
  }, [])

  useEffect(() => {
    let cancelled = false
    authApi
      .me()
      .then((u) => {
        if (!cancelled) setUser(u)
      })
      .catch(() => {
        if (!cancelled) setUser(null)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const u = await authApi.login(email, password)
    setSessionExpired(false)
    setUser(u)
  }, [])

  const register = useCallback(async (email: string, password: string, timezone: string) => {
    const u = await authApi.register(email, password, timezone)
    setSessionExpired(false)
    setUser(u)
  }, [])

  const logout = useCallback(async () => {
    try {
      await authApi.logout()
    } catch (err) {
      if (!(err instanceof ApiError)) throw err
    }
    setSessionExpired(false)
    setUser(null)
  }, [])

  const dismissSessionExpired = useCallback(() => setSessionExpired(false), [])

  return (
    <AuthContext.Provider value={{ user, loading, sessionExpired, dismissSessionExpired, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
