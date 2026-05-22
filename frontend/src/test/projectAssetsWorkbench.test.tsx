import { cleanup, render, screen, waitFor } from '@testing-library/react'
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

const secondProject = {
  ...project,
  id: 'project_2',
  name: 'Winter Launch',
  asin: 'B000NEXT',
  createdAt: '2026-05-13T00:00:00Z',
  updatedAt: '2026-05-13T00:00:00Z',
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

const task = {
  id: 'task_1',
  tenantId: 'tenant_1',
  projectId: 'project_2',
  type: 'IMAGE_GENERATION',
  status: 'QUEUED',
  prompt: 'Fresh project B prompt',
  providerId: 'provider_1',
  modelId: 'model_1',
  imageType: '',
  parameters: {
    size: '1024x1024',
    quality: 'standard',
    outputFormat: 'png',
    outputCount: 1,
  },
  inputAssetIds: [],
  outputAssetIds: [],
  attempt: 1,
  maxAttempts: 3,
  queuedAt: '2026-05-17T00:00:00Z',
  startedAt: null,
  finishedAt: null,
  timeoutAt: '2026-05-17T00:30:00Z',
  errorCode: '',
  errorMessage: '',
  createdBy: 'user_1',
  createdAt: '2026-05-17T00:00:00Z',
  updatedAt: '2026-05-17T00:00:00Z',
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

  it('adds a project asset reference without downloading a legacy File payload', async () => {
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
      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '项目资产库' })).toBeInTheDocument()
    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: '当前项目' })).toHaveValue('project_1')

    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))

    expect(await screen.findByAltText('reference.png')).toBeInTheDocument()
    expect(screen.getByAltText('reference.png')).toHaveAttribute('src', '/api/v1/assets/asset_1/download')
    expect(fetchImpl.mock.calls.map(([url]) => url)).not.toContain('/api/v1/assets/asset_1/download')
  })

  it('clears project asset references after switching projects before task submission', async () => {
    const user = userEvent.setup()
    FakeEventSource.instances.length = 0
    vi.stubGlobal('EventSource', FakeEventSource)
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([project, secondProject]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([asset]))
      }
      if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_2/tasks' && init?.method === 'POST') {
        return successResponse(task, 201)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))
    expect(await screen.findByAltText('reference.png')).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText('当前项目'), 'project_2')
    expect(await screen.findByText('暂无项目资产')).toBeInTheDocument()
    expect(screen.queryByAltText('reference.png')).not.toBeInTheDocument()

    await user.type(screen.getByLabelText('提示词'), 'Fresh project B prompt')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const taskCreateCall = fetchImpl.mock.calls.find(
      ([url, init]) => url === '/api/v1/projects/project_2/tasks' && init?.method === 'POST',
    )
    expect(taskCreateCall).toBeDefined()
    expect(JSON.parse(taskCreateCall?.[1]?.body as string)).toEqual({
      type: 'IMAGE_GENERATION',
      prompt: 'Fresh project B prompt',
      providerId: 'provider_1',
      modelId: 'model_1',
      parameters: {
        size: '1024x1024',
        quality: 'standard',
        outputFormat: 'png',
        outputCount: 1,
      },
    })
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
    expect((await screen.findAllByText('暂无项目')).length).toBeGreaterThan(0)
    await user.type(screen.getByLabelText('新项目名称'), 'Summer Launch')
    await user.click(screen.getByRole('button', { name: '创建项目' }))

    expect((await screen.findAllByText('Summer Launch')).length).toBeGreaterThan(0)

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

  it('shows project load failures with a recoverable empty project state', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return errorResponse(500, 'PROJECT_LIST_FAILED', 'Project list failed.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('项目加载失败，请刷新项目列表后重试。')).toBeInTheDocument()
    expect(await screen.findByText('暂无已选项目')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '上传参考图' })).toBeDisabled()
  })

  it.each([
    [403, 'FORBIDDEN', '没有权限执行该项目资产操作。'],
    [404, 'NOT_FOUND', '项目或资产不存在，可能已被删除或无权访问。'],
    [422, 'VALIDATION_ERROR', '请求内容未通过校验，请检查项目名称或图片文件。'],
    [409, 'CONFLICT', '操作冲突，请刷新后重试。'],
  ])('keeps project metadata unchanged when project update fails with %i', async (status, code, message) => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
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
      if (url === '/api/v1/projects/project_1' && init?.method === 'PATCH') {
        return errorResponse(status, code, message)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('Hero image set')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '编辑项目 Summer Launch' }))
    await user.clear(screen.getByLabelText('项目名称'))
    await user.type(screen.getByLabelText('项目名称'), 'Renamed Launch')
    await user.click(screen.getByRole('button', { name: '保存项目' }))

    expect((await screen.findAllByText(message)).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Summer Launch').length).toBeGreaterThan(0)
    expect(screen.queryByText('Renamed Launch')).not.toBeInTheDocument()

    const updateCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects/project_1' && init?.method === 'PATCH')
    expect((updateCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
  })

  it('discards a late asset response after switching projects', async () => {
    const user = userEvent.setup()
    const firstProjectAssets = deferredResponse()
    const secondProjectAsset = {
      ...asset,
      id: 'asset_2',
      projectId: 'project_2',
      filename: 'winter.png',
      previewUrl: '/api/v1/assets/asset_2/download',
    }
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(authenticatedSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([project, secondProject]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return firstProjectAssets.promise
      }
      if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
        return successResponse(page([secondProjectAsset]))
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('option', { name: 'Winter Launch' })).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('当前项目'), 'project_2')
    expect(await screen.findByText('winter.png')).toBeInTheDocument()

    firstProjectAssets.resolve(successResponse(page([asset])))

    await waitFor(() => {
      expect(screen.getByText('winter.png')).toBeInTheDocument()
      expect(screen.queryByText('reference.png')).not.toBeInTheDocument()
    })
  })

  it('keeps existing assets visible when a filter request fails', async () => {
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
      if (url === '/api/v1/projects/project_1/assets?kind=GENERATED&category=hero&favorite=true&pageNum=1&pageSize=50') {
        return errorResponse(500, 'ASSET_FILTER_FAILED', 'Asset filter failed.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('资产类型'), 'GENERATED')
    await user.selectOptions(screen.getByLabelText('收藏'), 'true')
    await user.type(screen.getByLabelText('筛选分类'), 'hero')
    await user.click(screen.getByRole('button', { name: '筛选资产' }))

    expect(await screen.findByText('资产加载失败')).toBeInTheDocument()
    expect(screen.getAllByText('reference.png').length).toBeGreaterThan(0)
  })

  it('does not apply asset metadata edits until the backend confirms them', async () => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
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
      if (url === '/api/v1/assets/asset_1' && init?.method === 'PATCH') {
        return errorResponse(422, 'VALIDATION_ERROR', 'Invalid filename.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '查看详情 reference.png' }))
    await user.clear(screen.getByLabelText('文件名'))
    await user.type(screen.getByLabelText('文件名'), 'renamed.png')
    await user.click(screen.getByRole('button', { name: '保存元数据' }))

    expect((await screen.findAllByText('请求内容未通过校验，请检查项目名称或图片文件。')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('reference.png').length).toBeGreaterThan(0)
    expect(screen.queryByText('renamed.png')).not.toBeInTheDocument()
  })

  it('updates asset metadata through PATCH after backend confirmation', async () => {
    const user = userEvent.setup()
    const updatedAsset = {
      ...asset,
      filename: 'renamed.png',
      category: 'hero',
      isFavorite: true,
      updatedAt: '2026-05-13T00:00:00Z',
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
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([asset]))
      }
      if (url === '/api/v1/assets/asset_1' && init?.method === 'PATCH') {
        return successResponse(updatedAsset)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '查看详情 reference.png' }))
    await user.clear(screen.getByLabelText('文件名'))
    await user.type(screen.getByLabelText('文件名'), 'renamed.png')
    await user.clear(screen.getByLabelText('分类'))
    await user.type(screen.getByLabelText('分类'), 'hero')
    await user.click(screen.getByLabelText('收藏资产'))
    await user.click(screen.getByRole('button', { name: '保存元数据' }))

    expect(await screen.findByText('资产元数据已更新。')).toBeInTheDocument()
    expect((await screen.findAllByText('renamed.png')).length).toBeGreaterThan(0)

    const updateCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/assets/asset_1' && init?.method === 'PATCH')
    expect((updateCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect(JSON.parse(updateCall?.[1]?.body as string)).toEqual({
      filename: 'renamed.png',
      category: 'hero',
      isFavorite: true,
    })
  })

  it('closes the detail panel and refreshes the list after deleting the open asset', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    let listCalls = 0
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
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
        listCalls += 1
        return successResponse(page(listCalls === 1 ? [asset] : []))
      }
      if (url === '/api/v1/assets/asset_1' && init?.method === 'DELETE') {
        return successResponse({ ok: true })
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '查看详情 reference.png' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '删除' }))

    expect(await screen.findByText('项目资产已删除。')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(await screen.findByText('暂无项目资产')).toBeInTheDocument()
    expect(listCalls).toBe(2)
  })

  it('does not store project or asset master data in browser storage', async () => {
    const user = userEvent.setup()
    const localStorageSet = vi.spyOn(Storage.prototype, 'setItem')
    const sessionStorageSet = vi.spyOn(window.sessionStorage.__proto__, 'setItem')
    const indexedDbOpen = vi.spyOn(indexedDB, 'open')
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

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))

    expect(localStorageSet).not.toHaveBeenCalled()
    expect(sessionStorageSet).not.toHaveBeenCalled()
    expect(indexedDbOpen).not.toHaveBeenCalled()
  })
})

class FakeEventSource {
  static readonly instances: FakeEventSource[] = []

  readonly url: string
  readonly withCredentials: boolean
  onerror: ((event: Event) => void) | null = null
  onopen: ((event: Event) => void) | null = null

  constructor(url: string, init: EventSourceInit) {
    this.url = url
    this.withCredentials = Boolean(init.withCredentials)
    FakeEventSource.instances.push(this)
  }

  addEventListener() {}

  removeEventListener() {}

  close() {}
}

function deferredResponse() {
  let resolve: (response: Response) => void = () => {}
  const promise = new Promise<Response>((nextResolve) => {
    resolve = nextResolve
  })

  return { promise, resolve }
}
