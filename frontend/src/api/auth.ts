import type { AuthMutationResponse, AuthSession, ChangePasswordRequest, InitAdminRequest, LoginRequest } from '../types/auth'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface AuthApi {
  initAdmin(request: InitAdminRequest): Promise<AuthSession>
  login(request: LoginRequest): Promise<AuthSession>
  logout(csrfToken: string): Promise<AuthMutationResponse>
  me(): Promise<AuthSession>
  changePassword(request: ChangePasswordRequest, csrfToken: string): Promise<AuthMutationResponse>
}

export function createAuthApi(client: ApiClient = apiClient): AuthApi {
  return {
    initAdmin: (request) => client.post<AuthSession>('/auth/init-admin', request),
    login: (request) => client.post<AuthSession>('/auth/login', request),
    logout: (csrfToken) =>
      client.post<AuthMutationResponse>('/auth/logout', undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    me: () => client.get<AuthSession>('/me'),
    changePassword: (request, csrfToken) =>
      client.patch<AuthMutationResponse>('/me/password', request, {
        headers: csrfHeaders(csrfToken),
      }),
  }
}

export const authApi = createAuthApi()
