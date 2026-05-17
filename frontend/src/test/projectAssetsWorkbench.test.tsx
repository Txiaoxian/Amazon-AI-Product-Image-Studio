import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
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
  permissions: ['project:read', 'project:create', 'asset:read', 'asset:upload', 'asset:update', 'asset:delete', 'asset:download'],
  csrfToken: 'csrf_from_me',
}

const project = {
  id: 'project_1',
  tenantId: 'tenant_1',
  name: 'Summer Launch',
  brand: 'Acme',
  asin: 'B000TEST',
  site: 'US',
  notes: 'Hero image set',
  status: 'ACTIVE',
  createdBy: 'user_1',
  createdAt: '2026-05-12T00:00:00Z',
  updatedAt: '2026-05-12T00:00:00Z',
}

const asset = {
  id: 'asset_1',
  tenantId: 'tenant_1',
  projectId: 'project_1',
  kind: 'REFERENCE',
  category: 'reference',
  filename: 'reference.png',
  mimeType: 'image/png',
  fileSize: 68,
  width: 2,
  height: 2,
  thumbnailUrl: '',
  previewUrl: '/api/v1/assets/asset_1/download',
  isFavorite: false,
  createdBy: 'user_1',
  createdAt: '2026-05-12T00:00:00Z',
  updatedAt: '2026-05-12T00:00:00Z',
}

const model = {
  id: 'model_1',
  tenantId: 'tenant_1',
  providerId: 'provider_1',
  providerName: 'Studio Provider',
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
  createdAt: '2026-05-12T00:00:00Z',
  updatedAt: '2026-05-12T00:00:00Z',
}

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_project_assets_workbench',
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
      requestId: 'req_project_assets_workbench_error',
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

describe('project asset workbench', () => {
  beforeEach(() => {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:reference-asset'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })
  })

  afterEach(() => {
    cleanup()
    localStorage.clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('does not show project asset operations while unauthenticated', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(errorResponse(401, 'AUTHENTICATION_REQUIRED', 'Authentication is required.'))
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '登录' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '项目资产库' })).not.toBeInTheDocument()
    expect(fetchImpl).toHaveBeenCalledTimes(1)
  })

  it('loads project assets and downloads the legacy payload when selecting an asset reference', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([asset]))
      }
      if (url === '/api/v1/assets/asset_1/download') {
        return new Response(new Blob(['reference-bytes'], { type: 'image/png' }), {
          headers: {
            'Content-Disposition': 'attachment; filename="reference.png"',
            'Content-Type': 'image/png',
          },
        })
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '项目资产库' })).toBeInTheDocument()
    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: '当前项目' })).toHaveValue('project_1')

    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))

    expect(await screen.findByAltText('reference.png')).toBeInTheDocument()
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/assets/asset_1/download',
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
  })

  it('creates a project and uploads a reference image with the in-memory CSRF token', async () => {
    const user = userEvent.setup()
    const uploadedAsset = {
      ...asset,
      id: 'asset_2',
      filename: 'upload.png',
      previewUrl: '/api/v1/assets/asset_2/download',
      isFavorite: true,
    }
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects' && init?.method === 'POST') {
        return successResponse(project, 201)
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_1/assets/uploads' && init?.method === 'POST') {
        return successResponse(uploadedAsset, 201)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '项目资产库' })).toBeInTheDocument()
    await user.type(screen.getByLabelText('新项目名称'), 'Summer Launch')
    await user.click(screen.getByRole('button', { name: '创建项目' }))

    expect(await screen.findByText('Summer Launch')).toBeInTheDocument()

    const file = new File(['png-bytes'], 'upload.png', { type: 'image/png' })
    await user.upload(screen.getByLabelText('上传参考图'), file)

    expect(await screen.findByText('upload.png')).toBeInTheDocument()
    const createCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects' && init?.method === 'POST')
    const uploadCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects/project_1/assets/uploads' && init?.method === 'POST')

    expect((createCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect((uploadCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect(uploadCall?.[1]?.body).toBeInstanceOf(FormData)
    expect((uploadCall?.[1]?.body as FormData).get('file')).toBe(file)
  })
})
