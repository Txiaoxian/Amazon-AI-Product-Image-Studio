import type { CurrentSession } from './platform'

export interface InitAdminRequest {
  tenantName: string
  email: string
  displayName: string
  password: string
}

export interface LoginRequest {
  tenantId?: string
  email: string
  password: string
}

export interface ChangePasswordRequest {
  currentPassword: string
  newPassword: string
}

export interface AuthSession extends CurrentSession {
  csrfToken?: string
}

export interface AuthMutationResponse {
  ok: boolean
}
