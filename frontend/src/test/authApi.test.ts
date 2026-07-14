import { describe, expect, it, vi } from 'vitest'
import { createAuthApi } from '../api/auth'
import { createApiClient } from '../api/client'
import type { AuthSession } from '../types/auth'

const session = {
  user: {
    id: 'user_1',
    email: 'admin@example.com',
    displayName: 'Admin User',
    status: 'ACTIVE',
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
  },
  tenant: {
    id: 'tenant_1',
    name: 'Studio Tenant',
    status: 'ACTIVE',
  },
  roles: [],
  permissions: [],
  csrfToken: 'csrf_from_response',
} as unknown as AuthSession

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_auth_test',
    }),
    { status },
  )
}

describe('auth API', () => {
  it('creates a one-time captcha challenge', async () => {
    const challenge = {
      captchaId: 'captcha_1',
      imageUrl: '/api/v1/auth/captcha/captcha_1/image',
      expiresAt: '2026-07-13T12:00:00Z',
    }
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(challenge, 201))
    const authApi = createAuthApi(createApiClient({ fetchImpl }))

    await expect(authApi.createCaptcha()).resolves.toEqual(challenge)
    expect(fetchImpl).toHaveBeenCalledWith('/api/v1/auth/captcha', expect.objectContaining({
      credentials: 'include',
      method: 'POST',
    }))
  })

  it('posts login through /api/v1 with credentials included', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(session))
    const authApi = createAuthApi(createApiClient({ fetchImpl }))

    await expect(
      authApi.login({
        tenantId: 'tenant_1',
        email: 'admin@example.com',
        password: 'valid-password-123',
        captchaId: 'captcha_1',
        captchaCode: '1234',
      }),
    ).resolves.toEqual(session)

    const [url, init] = fetchImpl.mock.calls[0]
    expect(url).toBe('/api/v1/auth/login')
    expect(init).toEqual(
      expect.objectContaining({
        body: JSON.stringify({
          tenantId: 'tenant_1',
          email: 'admin@example.com',
          password: 'valid-password-123',
          captchaId: 'captcha_1',
          captchaCode: '1234',
        }),
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect((init?.headers as Headers).get('Content-Type')).toBe('application/json')
  })

  it('loads the current user through /api/v1/me with credentials included', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(session))
    const authApi = createAuthApi(createApiClient({ fetchImpl }))

    await expect(authApi.me()).resolves.toEqual(session)

    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/me',
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
  })

  it('sends the in-memory CSRF token on logout', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse({ ok: true }))
    const authApi = createAuthApi(createApiClient({ fetchImpl }))

    await expect(authApi.logout('csrf_memory_only')).resolves.toEqual({ ok: true })

    const [, init] = fetchImpl.mock.calls[0]
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/auth/logout',
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect((init?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
  })

  it('wraps init-admin and password change without exposing auth storage', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(session, 201))
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
    const authApi = createAuthApi(createApiClient({ fetchImpl }))

    await expect(
      authApi.initAdmin({
        tenantName: 'Studio Tenant',
        email: 'admin@example.com',
        displayName: 'Admin User',
        password: 'valid-password-123',
      }),
    ).resolves.toEqual(session)
    await expect(
      authApi.changePassword(
        {
          currentPassword: 'valid-password-123',
          newPassword: 'updated-password-123',
        },
        'csrf_memory_only',
      ),
    ).resolves.toEqual({ ok: true })

    expect(fetchImpl.mock.calls[0][0]).toBe('/api/v1/auth/init-admin')
    expect(fetchImpl.mock.calls[1][0]).toBe('/api/v1/me/password')
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
  })
})
