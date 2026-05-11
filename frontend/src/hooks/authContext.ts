import { createContext } from 'react'
import type { AuthSession, ChangePasswordRequest, LoginRequest } from '../types/auth'

export type AuthStatus = 'loading' | 'anonymous' | 'authenticated'

export interface AuthContextValue {
  status: AuthStatus
  session: AuthSession | null
  error: string | null
  isSubmitting: boolean
  clearError: () => void
  refresh: () => Promise<void>
  login: (request: LoginRequest) => Promise<boolean>
  logout: () => Promise<void>
  changePassword: (request: ChangePasswordRequest) => Promise<boolean>
}

export const AuthContext = createContext<AuthContextValue | null>(null)
