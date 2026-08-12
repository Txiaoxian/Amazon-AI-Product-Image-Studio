import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { REMEMBERED_LOGIN_EMAIL_KEY } from '../components/auth/LoginPanel'

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
  permissions: ['audit:read', 'project:read', 'project:create'],
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

function emptyProjectPageResponse(): Response {
  return successResponse({
    records: [],
    total: 0,
    pageNum: 1,
    pageSize: 50,
  })
}

describe('auth flow', () => {
  afterEach(() => {
    cleanup()
    localStorage.clear()
    vi.unstubAllGlobals()
  })

  it('loads /me on startup and shows the current user with the existing workbench', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return emptyProjectPageResponse()
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await userEvent.setup().click(await screen.findByRole('button', { name: '打开账户菜单' }))
    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.getByText('Studio Tenant')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '从创建第一个产品开始' })).toBeInTheDocument()
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/me',
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
  })

  it('guides users to create their first project instead of showing a disabled workbench', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return emptyProjectPageResponse()
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '从创建第一个产品开始' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '生成参数' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '创建第一个产品' }))

    expect(await screen.findByRole('heading', { name: '产品中心' })).toBeInTheDocument()
    expect(await screen.findByRole('dialog', { name: '新建产品' })).toBeInTheDocument()
  })

  it('shows a login screen when unauthenticated and stores the returned CSRF token only in memory', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.')
      }
      if (url === '/api/v1/auth/captcha') {
        return successResponse({
          captchaId: 'captcha_1',
          imageUrl: '/api/v1/auth/captcha/captcha_1/image',
          expiresAt: '2026-07-13T12:00:00Z',
        }, 201)
      }
      if (url === '/api/v1/auth/login') {
        return successResponse({ ...authenticatedSession, csrfToken: 'csrf_from_login' })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return emptyProjectPageResponse()
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()

    expect(screen.queryByLabelText('租户 ID')).not.toBeInTheDocument()
    expect(await screen.findByAltText('登录验证码')).toBeInTheDocument()
    await user.type(screen.getByLabelText('邮箱'), 'admin@example.com')
    await user.type(screen.getByLabelText('密码'), 'valid-password-123')
    await user.click(screen.getByRole('button', { name: '登录' }))

    await user.click(await screen.findByRole('button', { name: '打开账户菜单' }))
    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '从创建第一个产品开始' })).toBeInTheDocument()

    const loginCall = fetchImpl.mock.calls.find(([requestUrl]) => requestUrl === '/api/v1/auth/login')
    expect(loginCall).toBeDefined()
    const [url, init] = loginCall!
    expect(url).toBe('/api/v1/auth/login')
    expect(init).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect(JSON.parse(init?.body as string)).toEqual({
      email: 'admin@example.com',
      password: 'valid-password-123',
    })
    expect(localStorage.getItem('csrf_from_login')).toBeNull()
    expect(sessionStorage.getItem('csrf_from_login')).toBeNull()
  })

  it('supports password visibility and remembers only the login email', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.')
      }
      if (url === '/api/v1/auth/captcha') {
        return successResponse({
          captchaId: 'captcha_visibility',
          imageUrl: '/api/v1/auth/captcha/captcha_visibility/image',
          expiresAt: '2026-07-13T12:00:00Z',
        }, 201)
      }
      if (url === '/api/v1/auth/login') {
        return successResponse({ ...authenticatedSession, csrfToken: 'csrf_from_login' })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return emptyProjectPageResponse()
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()
    const passwordInput = screen.getByLabelText('密码')
    expect(passwordInput).toHaveAttribute('type', 'password')

    await user.click(screen.getByRole('button', { name: '显示密码' }))
    expect(passwordInput).toHaveAttribute('type', 'text')
    expect(screen.getByRole('button', { name: '隐藏密码' })).toBeInTheDocument()

    await user.type(screen.getByLabelText('邮箱'), 'admin@example.com')
    await user.type(passwordInput, 'valid-password-123')
    await user.click(screen.getByRole('checkbox', { name: /记住账号/ }))
    await user.click(screen.getByRole('button', { name: '登录' }))

    await user.click(await screen.findByRole('button', { name: '打开账户菜单' }))
    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(localStorage.getItem(REMEMBERED_LOGIN_EMAIL_KEY)).toBe('admin@example.com')
    expect(localStorage.getItem('password')).toBeNull()
    expect(localStorage.getItem('csrf_from_login')).toBeNull()
  })

  it('prefills a remembered email without remembering the password', async () => {
    localStorage.setItem(REMEMBERED_LOGIN_EMAIL_KEY, 'remembered@example.com')
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.')
      }
      if (url === '/api/v1/auth/captcha') {
        return successResponse({
          captchaId: 'captcha_remembered',
          imageUrl: '/api/v1/auth/captcha/captcha_remembered/image',
          expiresAt: '2026-07-13T12:00:00Z',
        }, 201)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()
    expect(screen.getByLabelText('邮箱')).toHaveValue('remembered@example.com')
    expect(screen.getByLabelText('密码')).toHaveValue('')
    expect(screen.getByRole('checkbox', { name: /记住账号/ })).toBeChecked()
  })

  it('translates rate-limit login errors into simplified Chinese', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.')
      }
      if (url === '/api/v1/auth/captcha') {
        return successResponse({
          captchaId: 'captcha_rate_limit',
          imageUrl: '/api/v1/auth/captcha/captcha_rate_limit/image',
          expiresAt: '2026-07-13T12:00:00Z',
        }, 201)
      }
      if (url === '/api/v1/auth/login') {
        return errorResponse(429, 'RATE_LIMITED', 'Too many login attempts. Try again later.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.type(await screen.findByLabelText('邮箱'), 'admin@example.com')
    await user.type(screen.getByLabelText('密码'), 'invalid-password')
    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('登录尝试次数过多，请稍后再试。')
    expect(screen.queryByText('Too many login attempts. Try again later.')).not.toBeInTheDocument()
  })

  it('logs out with the in-memory CSRF token and returns to the login screen', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({ ...authenticatedSession, csrfToken: 'csrf_from_me' })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return emptyProjectPageResponse()
      }
      if (url === '/api/v1/auth/logout') {
        return successResponse({ ok: true })
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '打开账户菜单' }))
    await user.click(screen.getByRole('menuitem', { name: '退出登录' }))

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()
    const logoutCall = fetchImpl.mock.calls.find(([url]) => url === '/api/v1/auth/logout')
    expect(logoutCall?.[0]).toBe('/api/v1/auth/logout')
    expect((logoutCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
  })

  it('clears the current session when a state-changing auth request returns 401', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({ ...authenticatedSession, csrfToken: 'csrf_stale' })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return emptyProjectPageResponse()
      }
      if (url === '/api/v1/auth/logout') {
        return errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '打开账户菜单' }))
    await user.click(screen.getByRole('menuitem', { name: '退出登录' }))

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: '登录' })).toBeInTheDocument()
    })
    expect(screen.queryByText('Admin User')).not.toBeInTheDocument()
  })
})
