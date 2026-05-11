import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App'

const authenticatedSession = {
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
  roles: [
    {
      id: 'role_admin',
      code: 'admin',
      name: '管理员',
    },
  ],
  permissions: ['audit:read'],
  csrfToken: 'csrf_from_me',
}

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_auth_flow',
    }),
    { status },
  )
}

function errorResponse(status: number, code: string, message: string): Response {
  return new Response(
    JSON.stringify({
      error: {
        code,
        message,
      },
      requestId: 'req_auth_flow_error',
    }),
    { status },
  )
}

describe('auth flow', () => {
  afterEach(() => {
    cleanup()
    localStorage.clear()
    vi.unstubAllGlobals()
  })

  it('loads /me on startup and shows the current user with the existing workbench', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(successResponse(authenticatedSession))
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.getByText('Studio Tenant')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '生成参数' })).toBeInTheDocument()
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/me',
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
  })

  it('shows a login screen when unauthenticated and stores the returned CSRF token only in memory', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.'))
      .mockResolvedValueOnce(successResponse({ ...authenticatedSession, csrfToken: 'csrf_from_login' }))
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()

    await user.type(screen.getByLabelText('租户 ID'), 'tenant_1')
    await user.type(screen.getByLabelText('邮箱'), 'admin@example.com')
    await user.type(screen.getByLabelText('密码'), 'valid-password-123')
    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '生成参数' })).toBeInTheDocument()

    const [url, init] = fetchImpl.mock.calls[1]
    expect(url).toBe('/api/v1/auth/login')
    expect(init).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect(JSON.parse(init?.body as string)).toEqual({
      tenantId: 'tenant_1',
      email: 'admin@example.com',
      password: 'valid-password-123',
    })
    expect(localStorage.getItem('csrf_from_login')).toBeNull()
    expect(sessionStorage.getItem('csrf_from_login')).toBeNull()
  })

  it('logs out with the in-memory CSRF token and returns to the login screen', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(successResponse({ ...authenticatedSession, csrfToken: 'csrf_from_me' }))
      .mockResolvedValueOnce(successResponse({ ok: true }))
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '退出' }))

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()
    expect(fetchImpl.mock.calls[1][0]).toBe('/api/v1/auth/logout')
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
  })

  it('clears the current session when a state-changing auth request returns 401', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(successResponse({ ...authenticatedSession, csrfToken: 'csrf_stale' }))
      .mockResolvedValueOnce(errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.'))
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '退出' }))

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: '登录' })).toBeInTheDocument()
    })
    expect(screen.queryByText('Admin User')).not.toBeInTheDocument()
  })
})
