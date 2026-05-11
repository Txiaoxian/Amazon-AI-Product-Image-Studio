import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'
import { authApi as defaultAuthApi, type AuthApi } from '../api/auth'
import { isApiClientError, isUnauthorizedError } from '../api/client'
import type { AuthSession, ChangePasswordRequest, LoginRequest } from '../types/auth'
import { AuthContext, type AuthContextValue, type AuthStatus } from './authContext'

interface AuthProviderProps {
  authApi?: AuthApi
  children: ReactNode
}

export function AuthProvider({ authApi = defaultAuthApi, children }: AuthProviderProps) {
  const [status, setStatus] = useState<AuthStatus>('loading')
  const [session, setSession] = useState<AuthSession | null>(null)
  const [csrfToken, setCsrfToken] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isSubmitting, setSubmitting] = useState(false)

  const clearError = useCallback(() => setError(null), [])

  const applySession = useCallback((nextSession: AuthSession) => {
    setSession(nextSession)
    setCsrfToken(nextSession.csrfToken ?? null)
    setStatus('authenticated')
    setError(null)
  }, [])

  const clearAuth = useCallback(() => {
    setSession(null)
    setCsrfToken(null)
    setStatus('anonymous')
  }, [])

  const refresh = useCallback(async () => {
    setStatus('loading')
    setError(null)

    try {
      const nextSession = await authApi.me()
      applySession(nextSession)
    } catch (requestError) {
      clearAuth()

      if (!isUnauthorizedError(requestError)) {
        setError(getAuthErrorMessage(requestError, '无法加载当前用户，请稍后重试。'))
      }
    }
  }, [applySession, authApi, clearAuth])

  useEffect(() => {
    let isActive = true

    setStatus('loading')
    setError(null)
    void authApi
      .me()
      .then((nextSession) => {
        if (isActive) {
          applySession(nextSession)
        }
      })
      .catch((requestError: unknown) => {
        if (!isActive) {
          return
        }

        clearAuth()
        if (!isUnauthorizedError(requestError)) {
          setError(getAuthErrorMessage(requestError, '无法加载当前用户，请稍后重试。'))
        }
      })

    return () => {
      isActive = false
    }
  }, [applySession, authApi, clearAuth])

  const login = useCallback(
    async (request: LoginRequest) => {
      setSubmitting(true)
      setError(null)

      try {
        const nextSession = await authApi.login(request)
        applySession(nextSession)
        return true
      } catch (requestError) {
        setSession(null)
        setCsrfToken(null)
        setStatus('anonymous')
        setError(getAuthErrorMessage(requestError, '登录失败，请稍后重试。'))
        return false
      } finally {
        setSubmitting(false)
      }
    },
    [applySession, authApi],
  )

  const logout = useCallback(async () => {
    if (!csrfToken) {
      clearAuth()
      return
    }

    setSubmitting(true)
    setError(null)

    try {
      await authApi.logout(csrfToken)
      clearAuth()
    } catch (requestError) {
      if (isUnauthorizedError(requestError)) {
        clearAuth()
      } else {
        setError(getAuthErrorMessage(requestError, '退出失败，请稍后重试。'))
      }
    } finally {
      setSubmitting(false)
    }
  }, [authApi, clearAuth, csrfToken])

  const changePassword = useCallback(
    async (request: ChangePasswordRequest) => {
      if (!csrfToken) {
        clearAuth()
        return false
      }

      setSubmitting(true)
      setError(null)

      try {
        await authApi.changePassword(request, csrfToken)
        return true
      } catch (requestError) {
        if (isUnauthorizedError(requestError)) {
          clearAuth()
        } else {
          setError(getAuthErrorMessage(requestError, '密码修改失败，请稍后重试。'))
        }
        return false
      } finally {
        setSubmitting(false)
      }
    },
    [authApi, clearAuth, csrfToken],
  )

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      session,
      error,
      isSubmitting,
      clearError,
      refresh,
      login,
      logout,
      changePassword,
    }),
    [changePassword, clearError, error, isSubmitting, login, logout, refresh, session, status],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

function getAuthErrorMessage(error: unknown, fallback: string): string {
  if (isApiClientError(error)) {
    return error.message
  }

  return fallback
}
