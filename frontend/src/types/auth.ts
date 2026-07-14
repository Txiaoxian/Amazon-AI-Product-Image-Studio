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
  captchaId?: string
  captchaCode?: string
}

export interface CaptchaChallenge {
  captchaId: string
  imageUrl: string
  expiresAt: string
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
