import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
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

const projectMemberSession = {
  ...authenticatedSession,
  permissions: [...authenticatedSession.permissions, 'project:member:manage'],
}

const projectDeleteSession = {
  ...authenticatedSession,
  permissions: [...authenticatedSession.permissions, 'project:delete'],
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

const memberCandidate = {
  userId: 'user_3',
  userEmail: 'seller@example.com',
  userName: 'Seller User',
  userStatus: 'ACTIVE',
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

async function openProductAssets(user = userEvent.setup()) {
  await user.click(await screen.findByRole('button', { name: '素材库' }))
  await screen.findByRole('heading', { name: '产品素材库' })
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
    await waitFor(() => {
      expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
        '/api/v1/me',
        '/api/v1/auth/captcha',
      ])
    })
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

    await openProductAssets(user)

    expect(await screen.findByRole('heading', { name: '产品素材库' })).toBeInTheDocument()
    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: '当前产品' })).toHaveValue('project_1')

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

    await openProductAssets(user)

    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))
    expect(await screen.findByAltText('reference.png')).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')
    await openProductAssets(user)
    expect(await screen.findByText('暂无产品素材')).toBeInTheDocument()
    expect(screen.queryByAltText('reference.png')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '创作室' }))

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
      imageType: 'MAIN',
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

    expect(await screen.findByRole('heading', { name: '从创建第一个产品开始' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '创建第一个产品' }))
    expect(await screen.findByRole('dialog', { name: '新建产品' })).toBeInTheDocument()
    await user.type(screen.getByLabelText('产品名称'), 'Summer Launch')
    await user.click(screen.getByRole('button', { name: '创建产品' }))

    expect(await screen.findByLabelText('当前产品')).toHaveValue('project_1')
    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: '产品管理' })).not.toBeInTheDocument()
    })

    await openProductAssets(user)

    const file = new File(['png-bytes'], 'upload.png', { type: 'image/png' })
    await user.upload(screen.getByLabelText('上传参考图'), file)

    expect(await screen.findByText('upload.png')).toBeInTheDocument()
    const createCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects' && init?.method === 'POST')
    const uploadCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects/project_1/assets/uploads' && init?.method === 'POST')

    expect((createCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect((uploadCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect(uploadCall?.[1]?.body).toBeInstanceOf(FormData)
    expect((uploadCall?.[1]?.body as FormData).get('file')).toBe(file)
    expect((uploadCall?.[1]?.body as FormData).get('category')).toBe('MAIN')
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

    expect(await screen.findByText('产品加载失败，请刷新产品列表后重试。')).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: '产品列表加载失败' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '重新加载产品' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '上传参考图' })).not.toBeInTheDocument()
  })

  it.each([
    [403, 'FORBIDDEN', '没有权限执行该产品素材操作。'],
    [404, 'NOT_FOUND', '产品或素材不存在，可能已被删除或无权访问。'],
    [422, 'VALIDATION_ERROR', '请求内容未通过校验，请检查产品名称或图片文件。'],
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

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    expect(await screen.findByRole('dialog', { name: '编辑产品' })).toBeInTheDocument()
    await user.clear(screen.getByLabelText('产品名称'))
    await user.type(screen.getByLabelText('产品名称'), 'Renamed Launch')
    await user.click(screen.getByRole('button', { name: '保存产品' }))

    expect((await screen.findAllByText(message)).length).toBeGreaterThan(0)
    expect(screen.getAllByText('Summer Launch').length).toBeGreaterThan(0)
    expect(screen.queryByText('Renamed Launch')).not.toBeInTheDocument()

    const updateCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects/project_1' && init?.method === 'PATCH')
    expect((updateCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
  })

  it('deletes the selected product after confirmation and switches to the next available product', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const fetchImpl = createProjectDeleteFetch(projectDeleteSession)
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    const editor = await screen.findByRole('dialog', { name: '编辑产品' })
    await user.click(within(editor).getByRole('button', { name: '删除产品' }))

    expect(window.confirm).toHaveBeenCalledWith(expect.stringContaining('Summer Launch'))
    expect(await screen.findByText('产品“Summer Launch”已删除。')).toBeInTheDocument()
    expect(screen.getByLabelText('当前产品')).toHaveValue('project_2')
    expect(within(screen.getByLabelText('当前产品')).queryByRole('option', { name: /Summer Launch/ })).not.toBeInTheDocument()

    const deleteCall = fetchImpl.mock.calls.find(
      ([url, init]) => url === '/api/v1/projects/project_1' && init?.method === 'DELETE',
    )
    expect(deleteCall).toBeDefined()
    expect((deleteCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
  })

  it('keeps the product unchanged when product deletion is not confirmed', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(false)
    const fetchImpl = createProjectDeleteFetch(projectDeleteSession)
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    const editor = await screen.findByRole('dialog', { name: '编辑产品' })
    await user.click(within(editor).getByRole('button', { name: '删除产品' }))

    expect(fetchImpl.mock.calls.some(([url, init]) => url === '/api/v1/projects/project_1' && init?.method === 'DELETE')).toBe(false)
    expect(screen.getAllByText('Summer Launch').length).toBeGreaterThan(0)
    expect(editor).toBeInTheDocument()
  })

  it('keeps the product visible and shows a safe message when product deletion is rejected', async () => {
    const user = userEvent.setup()
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const fetchImpl = createProjectDeleteFetch(
      projectDeleteSession,
      errorResponse(403, 'FORBIDDEN', 'Unsafe backend detail'),
    )
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    const editor = await screen.findByRole('dialog', { name: '编辑产品' })
    await user.click(within(editor).getByRole('button', { name: '删除产品' }))

    expect((await screen.findAllByText('没有权限删除该产品。')).length).toBeGreaterThan(0)
    expect(screen.queryByText('Unsafe backend detail')).not.toBeInTheDocument()
    expect(screen.getAllByText('Summer Launch').length).toBeGreaterThan(0)
    expect(editor).toBeInTheDocument()
  })

  it('hides product deletion from accounts without project delete permission', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createProjectDeleteFetch(authenticatedSession))

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    const editor = await screen.findByRole('dialog', { name: '编辑产品' })
    expect(within(editor).queryByRole('button', { name: '删除产品' })).not.toBeInTheDocument()
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

    await openProductAssets(user)

    await screen.findByLabelText('当前产品')
    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')
    await openProductAssets(user)
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
      if (url === '/api/v1/projects/project_1/assets?kind=GENERATED&favorite=true&imageType=MAIN&pageNum=1&pageSize=50') {
        return errorResponse(500, 'ASSET_FILTER_FAILED', 'Asset filter failed.')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await openProductAssets(user)

    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('资产类型'), 'GENERATED')
    await user.selectOptions(screen.getByLabelText('收藏'), 'true')
    await user.selectOptions(screen.getByLabelText('素材图片类型'), 'MAIN')
    await user.click(screen.getByRole('button', { name: '筛选资产' }))

    expect(await screen.findByText('资产加载失败')).toBeInTheDocument()
    expect(screen.getAllByText('reference.png').length).toBeGreaterThan(0)
  })

  it('filters favorite product assets by image type', async () => {
    const user = userEvent.setup()
    const favoriteMainAsset = {
      ...asset,
      id: 'asset_main_1',
      kind: 'GENERATED',
      imageType: 'MAIN',
      filename: 'main-favorite.png',
      isFavorite: true,
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
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([favoriteMainAsset]))
      }
      if (url === '/api/v1/projects/project_1/assets?favorite=true&imageType=MAIN&pageNum=1&pageSize=50') {
        return successResponse(page([favoriteMainAsset]))
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openProductAssets(user)

    await user.selectOptions(screen.getByLabelText('收藏'), 'true')
    await user.selectOptions(screen.getByLabelText('素材图片类型'), 'MAIN')
    await user.click(screen.getByRole('button', { name: '筛选资产' }))

    expect(await screen.findByText('main-favorite.png')).toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain(
      '/api/v1/projects/project_1/assets?favorite=true&imageType=MAIN&pageNum=1&pageSize=50',
    )
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

    await openProductAssets(user)

    await user.click(await screen.findByRole('button', { name: '查看详情 reference.png' }))
    await user.clear(screen.getByLabelText('文件名'))
    await user.type(screen.getByLabelText('文件名'), 'renamed.jpg')
    expect(screen.getByLabelText('文件名')).toHaveValue('renamed')
    expect(screen.getByText('.png')).toBeInTheDocument()
    expect(screen.getByText('文件格式由图片实际内容决定，扩展名不可修改。')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '保存元数据' }))

    expect((await screen.findAllByText('请求内容未通过校验，请检查产品名称或图片文件。')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('reference.png').length).toBeGreaterThan(0)
    expect(screen.queryByText('renamed.png')).not.toBeInTheDocument()
  })

  it('updates asset metadata through PATCH after backend confirmation', async () => {
    const user = userEvent.setup()
    const updatedAsset = {
      ...asset,
      filename: 'renamed.png',
      category: 'MAIN',
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

    await openProductAssets(user)

    await user.click(await screen.findByRole('button', { name: '查看详情 reference.png' }))
    await user.clear(screen.getByLabelText('文件名'))
    await user.type(screen.getByLabelText('文件名'), 'renamed.png')
    const categorySelect = screen.getByLabelText('分类')
    expect(categorySelect.tagName).toBe('SELECT')
    expect(within(categorySelect).getAllByRole('option')).toHaveLength(8)
    expect(within(categorySelect).queryByRole('option', { name: '未分类' })).not.toBeInTheDocument()
    expect(within(categorySelect).getByRole('option', { name: '主图' })).toHaveValue('MAIN')
    await user.selectOptions(categorySelect, 'MAIN')
    await user.click(screen.getByLabelText('收藏资产'))
    await user.click(screen.getByRole('button', { name: '保存元数据' }))

    expect(await screen.findByText('资产元数据已更新。')).toBeInTheDocument()
    expect((await screen.findAllByText('renamed.png')).length).toBeGreaterThan(0)

    const updateCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/assets/asset_1' && init?.method === 'PATCH')
    expect((updateCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect(JSON.parse(updateCall?.[1]?.body as string)).toEqual({
      filename: 'renamed.png',
      category: 'MAIN',
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

    await openProductAssets(user)

    await user.click(await screen.findByRole('button', { name: '查看详情 reference.png' }))
    expect(screen.getByRole('dialog', { name: '资产详情' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '删除' }))

    expect(await screen.findByText('产品素材已删除。')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '资产详情' })).not.toBeInTheDocument())
    expect(await screen.findByText('暂无产品素材')).toBeInTheDocument()
    expect(listCalls).toBe(2)
  })

  it('does not insert completed uploads into a project selected after upload start', async () => {
    const user = userEvent.setup()
    const upload = deferredResponse()
    const uploadedAsset = {
      ...asset,
      id: 'asset_upload',
      filename: 'upload.png',
      previewUrl: '/api/v1/assets/asset_upload/download',
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
        return successResponse(page([project, secondProject]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_1/assets/uploads' && init?.method === 'POST') {
        return upload.promise
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await openProductAssets(user)

    await screen.findByLabelText('当前产品')
    await user.upload(screen.getByLabelText('上传参考图'), new File(['png-bytes'], 'upload.png', { type: 'image/png' }))
    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')
    await openProductAssets(user)
    expect(await screen.findByText('暂无产品素材')).toBeInTheDocument()

    upload.resolve(successResponse(uploadedAsset, 201))

    await waitFor(() => {
      expect(screen.queryByText('upload.png')).not.toBeInTheDocument()
      expect(screen.getByLabelText('当前产品')).toHaveValue('project_2')
    })
  })

  it('removes an asset from a favorite-filtered list after unfavorite succeeds', async () => {
    const user = userEvent.setup()
    const favoriteAsset = {
      ...asset,
      filename: 'favorite.png',
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
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([favoriteAsset]))
      }
      if (url === '/api/v1/projects/project_1/assets?favorite=true&pageNum=1&pageSize=50') {
        return successResponse(page([favoriteAsset]))
      }
      if (url === '/api/v1/assets/asset_1/favorite' && init?.method === 'DELETE') {
        return successResponse({ ...favoriteAsset, isFavorite: false })
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await openProductAssets(user)

    expect(await screen.findByText('favorite.png')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('收藏'), 'true')
    await user.click(screen.getByRole('button', { name: '筛选资产' }))
    expect(await screen.findByRole('button', { name: '取消收藏 favorite.png' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '取消收藏 favorite.png' }))

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: '取消收藏 favorite.png' })).not.toBeInTheDocument()
    })
    expect(screen.getByText('暂无产品素材')).toBeInTheDocument()
  })

  it('moves an asset out of the current image-type list after its classification changes', async () => {
    const user = userEvent.setup()
    const categoryAsset = {
      ...asset,
      filename: 'category.png',
      category: 'MAIN',
      imageType: 'MAIN',
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
        return successResponse(page([categoryAsset]))
      }
      if (url === '/api/v1/projects/project_1/assets?imageType=MAIN&pageNum=1&pageSize=50') {
        return successResponse(page([categoryAsset]))
      }
      if (url === '/api/v1/assets/asset_1' && init?.method === 'PATCH') {
        return successResponse({ ...categoryAsset, category: 'DETAIL', imageType: 'DETAIL' })
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await openProductAssets(user)

    expect(await screen.findByText('category.png')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('素材图片类型'), 'MAIN')
    await user.click(screen.getByRole('button', { name: '筛选资产' }))
    await user.click(await screen.findByRole('button', { name: '查看详情 category.png' }))
    await user.selectOptions(screen.getByLabelText('分类'), 'DETAIL')
    await user.click(screen.getByRole('button', { name: '保存元数据' }))

    expect(await screen.findByText('资产元数据已更新。')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: '查看详情 category.png' })).not.toBeInTheDocument()
    })
    expect(screen.getByText('暂无产品素材')).toBeInTheDocument()
  })

  it('loads and writes project members through real project member APIs', async () => {
    const user = userEvent.setup()
    const member = {
      id: 'member_1',
      tenantId: 'tenant_1',
      projectId: 'project_1',
      userId: 'user_2',
      role: 'VIEWER',
      createdAt: '2026-05-12T00:00:00Z',
      updatedAt: '2026-05-12T00:00:00Z',
    }
    const addedMember = { ...member, id: 'member_2', userId: 'user_3', role: 'EDITOR' }
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(projectMemberSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_1/members' && init?.method === 'GET') {
        return successResponse([member])
      }
      if (url === '/api/v1/projects/project_1/member-candidates?pageNum=1&pageSize=100') {
        return successResponse([memberCandidate])
      }
      if (url === '/api/v1/projects/project_1/members' && init?.method === 'POST') {
        return successResponse(addedMember, 201)
      }
      if (url === '/api/v1/projects/project_1/members/user_2' && init?.method === 'PATCH') {
        return successResponse({ ...member, role: 'OWNER' })
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    expect((await screen.findAllByText('user_2')).length).toBeGreaterThan(0)
    await user.selectOptions(screen.getByLabelText('选择产品成员'), 'user_3')
    await user.selectOptions(screen.getByLabelText('成员角色'), 'EDITOR')
    await user.click(screen.getByRole('button', { name: '添加' }))
    expect((await screen.findAllByText('user_3')).length).toBeGreaterThan(0)
    await user.selectOptions(screen.getByLabelText('产品成员角色 user_2'), 'OWNER')
    expect(await screen.findByText('产品成员已更新。')).toBeInTheDocument()

    const postCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects/project_1/members' && init?.method === 'POST')
    const patchCall = fetchImpl.mock.calls.find(([url, init]) => url === '/api/v1/projects/project_1/members/user_2' && init?.method === 'PATCH')
    expect((postCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
    expect((patchCall?.[1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_from_me')
  })

  it.each([
    [401, 'AUTHENTICATION_REQUIRED', '登录状态已失效，请重新登录。'],
    [403, 'FORBIDDEN', '没有权限管理产品成员。'],
    [404, 'NOT_FOUND', '产品或成员不存在，可能已被删除或无权访问。'],
  ])('shows safe project member list errors for %i', async (status, code, message) => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(projectMemberSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_1/members' && init?.method === 'GET') {
        return errorResponse(status, code, 'Unsafe backend detail')
      }
      if (url === '/api/v1/projects/project_1/member-candidates?pageNum=1&pageSize=100') {
        return successResponse([memberCandidate])
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    expect(await screen.findByText(message)).toBeInTheDocument()
  })

  it.each([
    [409, 'CONFLICT', '产品成员操作冲突，请刷新后重试。'],
    [422, 'VALIDATION_ERROR', '成员信息未通过校验，请选择有效用户和角色。'],
  ])('shows safe project member write errors for %i', async (status, code, message) => {
    const user = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input, init) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse(projectMemberSession)
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([model], 100))
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([project]))
      }
      if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/projects/project_1/members' && init?.method === 'GET') {
        return successResponse([])
      }
      if (url === '/api/v1/projects/project_1/member-candidates?pageNum=1&pageSize=100') {
        return successResponse([memberCandidate])
      }
      if (url === '/api/v1/projects/project_1/members' && init?.method === 'POST') {
        return errorResponse(status, code, 'Unsafe backend detail')
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '产品设置' }))
    await user.selectOptions(screen.getByLabelText('选择产品成员'), 'user_3')
    await user.click(screen.getByRole('button', { name: '添加' }))

    expect(await screen.findByText(message)).toBeInTheDocument()
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

    await openProductAssets(user)

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

function createProjectDeleteFetch(session: typeof authenticatedSession, deleteResponse = successResponse({ ok: true })) {
  return vi.fn<typeof fetch>(async (input, init) => {
    const url = String(input)

    if (url === '/api/v1/me') {
      return successResponse(session)
    }
    if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
      return successResponse(page([model], 100))
    }
    if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
      return successResponse(page([project, secondProject]))
    }
    if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url.startsWith('/api/v1/projects/project_1/history?') || url.startsWith('/api/v1/projects/project_2/history?')) {
      return successResponse(page([]))
    }
    if (url.startsWith('/api/v1/projects/project_1/tasks?') || url.startsWith('/api/v1/projects/project_2/tasks?')) {
      return successResponse(page([]))
    }
    if (url === '/api/v1/projects/project_1' && init?.method === 'DELETE') {
      return deleteResponse
    }

    return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
  })
}
