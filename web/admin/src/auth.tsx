import React, { createContext, useContext, useEffect, useState } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { clearAdminToken, getAdminToken, setAdminToken, UnauthorizedError, validateAdmin } from './api'

type AuthState = {
  loading: boolean
  signedIn: boolean
  mode: 'token' | 'development'
  login: (token: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [loading, setLoading] = useState(true)
  const [signedIn, setSignedIn] = useState(false)
  const [mode, setMode] = useState<'token' | 'development'>('token')

  useEffect(() => {
    const token = getAdminToken()
    validateAdmin(token).then(() => {
      setMode(token ? 'token' : 'development')
      setSignedIn(true)
    }).catch(() => {
      clearAdminToken()
      setSignedIn(false)
    }).finally(() => setLoading(false))
  }, [])

  const login = async (token: string) => {
    try {
      await validateAdmin(token.trim())
      setAdminToken(token.trim())
      setMode('token')
      setSignedIn(true)
    } catch (error) {
      if (error instanceof UnauthorizedError) throw new Error('Admin Token 不正确')
      throw error
    }
  }
  const logout = () => {
    clearAdminToken()
    setSignedIn(false)
  }

  return <AuthContext.Provider value={{ loading, signedIn, mode, login, logout }}>{children}</AuthContext.Provider>
}

export const useAuth = () => {
  const value = useContext(AuthContext)
  if (!value) throw new Error('AuthProvider is missing')
  return value
}

export function ProtectedRoute() {
  const auth = useAuth()
  const location = useLocation()
  if (auth.loading) return <div className="full-page-state"><span className="spinner"/><p>正在验证管理权限…</p></div>
  if (!auth.signedIn) return <Navigate to={`/admin/login?redirect=${encodeURIComponent(`${location.pathname}${location.search}`)}`} replace />
  return <Outlet />
}
