import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'

const baseSession = {
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
  csrfToken: 'csrf_from_me',
}

const provider = {
  id: 'provider_1',
  tenantId: 'tenant_1',
  type: 'OPENAI_COMPATIBLE',
  name: 'Secure Gateway',
  baseUrl: 'https://provider.example/v1',
  status: 'ENABLED',
  timeoutSeconds: 30,
  concurrencyLimit: 2,
  apiKeyHint: '****1234',
  apiKeyUpdatedAt: '2026-05-13T00:00:00Z',
  lastTestStatus: 'SUCCESS',
  lastTestedAt: '2026-05-13T00:01:00Z',
  createdAt: '2026-05-13T00:00:00Z',
  updatedAt: '2026-05-13T00:01:00Z',
}

const model = {
  id: 'model_1',
  tenantId: 'tenant_1',
  providerId: 'provider_1',
  providerName: 'Secure Gateway',
  modelName: 'image-model',
  displayName: 'Image Model',
  supportsGenerate: true,
  supportsEdit: true,
  supportsMultiReference: true,
  supportsN: true,
  maxOutputCount: 4,
  supportedSizes: ['1024x1024'],
  supportedQualities: ['standard'],
  supportedOutputFormats: ['png'],
  pricing: {
    currency: 'USD',
    unitPrices: {
      image: 0.04,
    },
  },
  status: 'ENABLED',
  createdAt: '2026-05-13T00:00:00Z',
  updatedAt: '2026-05-13T00:01:00Z',
}

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_admin_provider_model',
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
      requestId: 'req_admin_provider_model_error',
    }),
    { status },
  )
}

function page(records: unknown[], pageSize = 50) {
  return {
    records,
    total: records.length,
    pageNum: 1,
    pageSize,
  }
}

function getBrowserStorage(prefix: string): Storage {
  return Reflect.get(globalThis, `${prefix}Storage`) as Storage
}

describe('admin Provider and model management UI', () => {
  beforeEach(() => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  afterEach(() => {
    cleanup()
    getBrowserStorage('local').clear()
    getBrowserStorage('session').clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('hides the management entry when the user lacks Provider/model manage permissions', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['project:read'],
        })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Provider/model 管理' })).not.toBeInTheDocument()
  })

  it('submits Provider keys once and keeps only masked metadata visible after save', async () => {
    const user = userEvent.setup()
    const plainKey = 'one-time-secret-1234'
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['provider:manage', 'model:manage'],
        })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/providers?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/models?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/providers' && init?.method === 'POST') {
        return successResponse(provider, 201)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Provider/model 管理' }))
    expect(await screen.findByRole('heading', { name: 'Provider 与模型管理' })).toBeInTheDocument()

    await user.type(screen.getByLabelText('Provider 名称'), 'Secure Gateway')
    await user.type(screen.getByLabelText('Base URL'), 'https://provider.example/v1')
    await user.type(screen.getByLabelText('API Key'), plainKey)
    await user.clear(screen.getByLabelText('超时秒数'))
    await user.type(screen.getByLabelText('超时秒数'), '30')
    await user.clear(screen.getByLabelText('并发限制'))
    await user.type(screen.getByLabelText('并发限制'), '2')
    await user.click(screen.getByRole('button', { name: '保存 Provider' }))

    const createCall = await waitFor(() => {
      const call = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/providers' && init?.method === 'POST')
      expect(call).toBeDefined()
      return call
    })
    expect(JSON.parse(createCall?.[1]?.body as string)).toMatchObject({
      name: 'Secure Gateway',
      baseUrl: 'https://provider.example/v1',
      apiKey: plainKey,
    })
    expect((createCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect(await screen.findByText('****1234')).toBeInTheDocument()
    expect(screen.getByLabelText('API Key')).toHaveValue('')
    expect(screen.queryByDisplayValue(plainKey)).not.toBeInTheDocument()
    expect(getBrowserStorage('local').getItem(plainKey)).toBeNull()
    expect(getBrowserStorage('session').getItem(plainKey)).toBeNull()
  })

  it('clears an unsubmitted Provider API key when the management modal closes', async () => {
    const user = userEvent.setup()
    const plainKey = 'unsubmitted-secret-4321'
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['provider:manage', 'model:manage'],
        })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/providers?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/models?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Provider/model 管理' }))
    await user.type(await screen.findByLabelText('API Key'), plainKey)
    expect(screen.getByLabelText('API Key')).toHaveValue(plainKey)

    await user.click(screen.getByRole('button', { name: '关闭弹窗' }))
    await user.click(screen.getByRole('button', { name: 'Provider/model 管理' }))

    expect(await screen.findByLabelText('API Key')).toHaveValue('')
    expect(screen.queryByDisplayValue(plainKey)).not.toBeInTheDocument()
    expect(getBrowserStorage('local').getItem(plainKey)).toBeNull()
    expect(getBrowserStorage('session').getItem(plainKey)).toBeNull()
  })

  it('surfaces permission and validation failures as text error states', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['provider:manage', 'model:manage'],
        })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/providers?pageNum=1&pageSize=50') {
        return errorResponse(403, 'FORBIDDEN', 'Forbidden.')
      }
      if (url === '/api/v1/models?pageNum=1&pageSize=50') {
        return successResponse(page([model]))
      }
      if (url === '/api/v1/providers' && init?.method === 'POST') {
        return errorResponse(422, 'VALIDATION_ERROR', 'Invalid Provider.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: 'Provider/model 管理' }))

    expect(await screen.findByText('当前账号没有此管理权限。')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Provider 名称'), 'Broken Provider')
    await user.type(screen.getByLabelText('Base URL'), 'https://provider.example/v1')
    await user.type(screen.getByLabelText('API Key'), 'one-time-secret-0000')
    await user.click(screen.getByRole('button', { name: '保存 Provider' }))

    expect(await screen.findByText('表单内容未通过校验：Invalid Provider.')).toBeInTheDocument()
  })
})
