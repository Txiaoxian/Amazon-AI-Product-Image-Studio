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
  permissions: ['project:read', 'project:create', 'asset:read', 'asset:download'],
  csrfToken: 'csrf_from_me',
}

const projects = [
  {
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
  },
  {
    id: 'project_2',
    tenantId: 'tenant_1',
    name: 'Winter Launch',
    brand: 'Acme',
    asin: 'B000NEXT',
    site: 'US',
    notes: 'Secondary image set',
    status: 'ACTIVE',
    createdBy: 'user_1',
    createdAt: '2026-05-13T00:00:00Z',
    updatedAt: '2026-05-13T00:00:00Z',
  },
]

const referenceAsset = {
  id: 'asset_reference_1',
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
  previewUrl: '/api/v1/assets/asset_reference_1/download',
  isFavorite: false,
  createdBy: 'user_1',
  createdAt: '2026-05-12T00:00:00Z',
  updatedAt: '2026-05-12T00:00:00Z',
}

const generatedAsset = {
  id: 'asset_generated_1',
  tenantId: 'tenant_1',
  projectId: 'project_1',
  taskId: 'task_1',
  kind: 'GENERATED',
  category: 'main',
  filename: 'hero.png',
  mimeType: 'image/png',
  fileSize: 2048,
  width: 1536,
  height: 1024,
  thumbnailUrl: '/api/v1/assets/asset_generated_1/download?variant=thumb',
  previewUrl: '/api/v1/assets/asset_generated_1/download',
  isFavorite: false,
  createdBy: 'user_1',
  createdAt: '2026-05-17T00:00:04Z',
  updatedAt: '2026-05-17T00:00:04Z',
}

const editedAsset = {
  ...generatedAsset,
  id: 'asset_edited_1',
  taskId: 'task_2',
  kind: 'EDITED',
  filename: 'edited.png',
  previewUrl: 'https://minio.internal/bucket/edited.png',
  thumbnailUrl: 'data:image/png;base64,unsafe-thumbnail',
  createdAt: '2026-05-18T00:00:04Z',
  updatedAt: '2026-05-18T00:00:04Z',
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

const task = {
  id: 'task_1',
  tenantId: 'tenant_1',
  projectId: 'project_1',
  type: 'IMAGE_GENERATION',
  status: 'SUCCEEDED',
  prompt: 'Clean Amazon product image',
  providerId: 'provider_1',
  modelId: 'model_1',
  imageType: 'COMPARISON',
  parameters: {
    size: '1024x1024',
    quality: 'standard',
    outputFormat: 'png',
    outputCount: 1,
  },
  inputAssetIds: ['asset_reference_1'],
  outputAssetIds: ['asset_generated_1'],
  attempt: 1,
  maxAttempts: 3,
  queuedAt: '2026-05-17T00:00:00Z',
  startedAt: '2026-05-17T00:00:01Z',
  finishedAt: '2026-05-17T00:00:04Z',
  timeoutAt: '2026-05-17T00:30:00Z',
  errorCode: '',
  errorMessage: '',
  createdBy: 'user_1',
  createdAt: '2026-05-17T00:00:00Z',
  updatedAt: '2026-05-17T00:00:04Z',
}

const editedTask = {
  ...task,
  id: 'task_2',
  type: 'IMAGE_EDIT',
  outputAssetIds: ['asset_edited_1'],
  parameters: {
    ...task.parameters,
    outputCount: 1,
    apiCall: {
      redactedRequest: 'Authorization: Bearer secret',
      redactedResponse: 'base64-image-data',
    },
  },
}

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_history_asset_source',
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
      requestId: 'req_history_asset_source_error',
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

async function openHistoryTab(user = userEvent.setup()) {
  void user
  await screen.findByRole('heading', { name: '已完成' })
}

describe('backend history asset source', () => {
  beforeEach(() => {
    FakeEventSource.instances.length = 0
  })

  afterEach(() => {
    cleanup()
    localStorage.clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('uses unified backend history as the only visible history source after legacy blob storage retirement', async () => {
    const fetchImpl = createHistoryFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openHistoryTab()

    expect(await screen.findByRole('button', { name: '查看结果 hero.png' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '查看旧本地历史' })).not.toBeInTheDocument()
    expect(screen.queryByText('旧本地历史（兼容）')).not.toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain('/api/v1/projects/project_1/history?pageNum=1&pageSize=10&imageType=MAIN')
    expect(fetchImpl.mock.calls.map(([url]) => url)).not.toContain('/api/v1/projects/project_1/tasks?pageNum=1&pageSize=50')
    expect(fetchImpl.mock.calls.map(([url]) => url)).not.toContain('/api/v1/projects/project_1/assets?kind=GENERATED&pageNum=1&pageSize=50')
    expect(fetchImpl.mock.calls.map(([url]) => url)).not.toContain('/api/v1/projects/project_1/assets?kind=EDITED&pageNum=1&pageSize=50')
  })

  it('reloads history with the selected image type instead of sharing one product-wide history', async () => {
    const user = userEvent.setup()
    const fetchImpl = createHistoryFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openHistoryTab(user)

    await user.click(screen.getByRole('tab', { name: 'A+ 图片' }))

    await waitFor(() => {
      const urls = fetchImpl.mock.calls.map(([url]) => String(url))
      expect(urls).toContain('/api/v1/projects/project_1/history?pageNum=1&pageSize=10&imageType=MAIN')
      expect(urls).toContain('/api/v1/projects/project_1/history?pageNum=1&pageSize=10&imageType=A_PLUS')
    })
  })

  it('does not request history when no project is selected', async () => {
    const fetchImpl = createHistoryFetch({
      listProjects: () => successResponse(page([])),
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '从创建第一个产品开始' })).toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url)).some((url) => url.includes('/history?'))).toBe(false)
  })

  it('shows a project-scoped empty state after switching projects without leaking the previous project history', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createHistoryFetch())

    render(<App />)
    await openHistoryTab(user)

    expect(await screen.findByRole('button', { name: '查看结果 hero.png' })).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')
    await openHistoryTab(user)

    expect(await screen.findByText('当前图片类型暂无生成记录')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '查看结果 hero.png' })).not.toBeInTheDocument()
  })

  it('ignores stale history responses from the previous project after a project switch', async () => {
    const user = userEvent.setup()
    const projectOneHistory = deferredResponse()
    vi.stubGlobal(
      'fetch',
      createHistoryFetch({
        listProjectOneHistory: () => projectOneHistory.promise,
      }),
    )

    render(<App />)

    await screen.findByLabelText('当前产品')
    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')
    await openHistoryTab(user)
    expect(await screen.findByText('当前图片类型暂无生成记录')).toBeInTheDocument()

    projectOneHistory.resolve(successResponse(page([{ asset: generatedAsset, task }])))

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: '查看结果 hero.png' })).not.toBeInTheDocument()
    })
  })

  it.each([
    [401, 'UNAUTHENTICATED', 'object_key=minio/internal', '登录状态已失效，请重新登录。'],
    [403, 'FORBIDDEN', 'Authorization: Bearer secret', '没有权限读取该产品生成记录。'],
    [404, 'NOT_FOUND', 'minio://private-bucket/object.png', '无法读取该结果，可能已被删除或无权访问。'],
  ])('maps /history %s errors to non-leaky feedback', async (status, code, backendMessage, friendlyMessage) => {
    vi.stubGlobal(
      'fetch',
      createHistoryFetch({
        listProjectOneHistory: () => errorResponse(status, code, backendMessage),
      }),
    )

    render(<App />)
    await openHistoryTab()

    expect(await screen.findByText(friendlyMessage)).toBeInTheDocument()
    expect(screen.queryByText(backendMessage)).not.toBeInTheDocument()
  })

  it('uses unified history query parameters for pagination and kind filtering', async () => {
    const user = userEvent.setup()
    const fetchImpl = createHistoryFetch({
      listProjectOneHistory: (url) => {
        if (url === '/api/v1/projects/project_1/history?pageNum=2&pageSize=10&imageType=MAIN') {
          return successResponse({
            records: [{ asset: editedAsset, task: editedTask }],
            total: 11,
            pageNum: 2,
            pageSize: 10,
          })
        }
        if (url === '/api/v1/projects/project_1/history?pageNum=1&pageSize=10&kind=EDITED&imageType=MAIN') {
          return successResponse({
            records: [{ asset: editedAsset, task: editedTask }],
            total: 1,
            pageNum: 1,
            pageSize: 10,
          })
        }

        return successResponse({
          records: [{ asset: generatedAsset, task }],
          total: 11,
          pageNum: 1,
          pageSize: 10,
        })
      },
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openHistoryTab(user)

    expect(await screen.findByRole('button', { name: '查看结果 hero.png' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '下一页历史记录' }))
    expect(await screen.findByRole('button', { name: '查看结果 edited.png' })).toBeInTheDocument()

    await user.selectOptions(screen.getByLabelText('结果类型'), 'EDITED')
    expect(await screen.findByRole('button', { name: '查看结果 edited.png' })).toBeInTheDocument()

    const urls = fetchImpl.mock.calls.map(([url]) => url)
    expect(urls).toContain('/api/v1/projects/project_1/history?pageNum=2&pageSize=10&imageType=MAIN')
    expect(urls).toContain('/api/v1/projects/project_1/history?pageNum=1&pageSize=10&kind=EDITED&imageType=MAIN')
    expect(urls).not.toContain('/api/v1/projects/project_1/assets?kind=GENERATED&pageNum=1&pageSize=50')
    expect(urls).not.toContain('/api/v1/projects/project_1/assets?kind=EDITED&pageNum=1&pageSize=50')
  })

  it('does not render unsafe fields returned by unified history', async () => {
    vi.stubGlobal(
      'fetch',
      createHistoryFetch({
        listProjectOneHistory: () =>
          successResponse(
            page([
              {
                asset: {
                  ...generatedAsset,
                  objectKey: 'tenant/project/object-key-secret.png',
                  thumbnailObjectKey: 'tenant/project/thumb-secret.png',
                  previewUrl: 'https://minio.internal/private/object-key-secret.png',
                  thumbnailUrl: 'data:image/png;base64,unsafe-history-thumbnail',
                },
                task: {
                  ...task,
                  apiCall: {
                    redactedRequest: 'Authorization: Bearer secret',
                    redactedResponse: 'base64-image-data',
                  },
                  redactedRequest: 'Cookie: session=secret',
                },
              },
            ], 10),
          ),
      }),
    )

    render(<App />)
    await openHistoryTab()

    const image = await screen.findByAltText('hero.png')
    expect(image).toHaveAttribute('src', '/api/v1/assets/asset_generated_1/download')
    expect(document.body.innerHTML).not.toContain('object-key-secret')
    expect(document.body.innerHTML).not.toContain('thumb-secret')
    expect(document.body.innerHTML).not.toContain('minio.internal')
    expect(document.body.innerHTML).not.toContain('Authorization')
    expect(document.body.innerHTML).not.toContain('Cookie')
    expect(document.body.innerHTML).not.toContain('base64-image-data')
  })

  it('keeps unavailable asset feedback non-leaky when detail loading fails', async () => {
    const user = userEvent.setup()
    vi.stubGlobal(
      'fetch',
      createHistoryFetch({
        getAsset: () => errorResponse(404, 'NOT_FOUND', 'Resource not found.'),
      }),
    )

    render(<App />)
    await openHistoryTab(user)

    await user.click(await screen.findByRole('button', { name: '查看结果 hero.png' }))

    expect(await screen.findByText('无法读取该结果，可能已被删除或无权访问。')).toBeInTheDocument()
    expect(screen.queryByText('Resource not found.')).not.toBeInTheDocument()
  })

  it('downloads backend history assets through the authorized asset endpoint and stays recoverable on failure', async () => {
    const user = userEvent.setup()
    const fetchImpl = createHistoryFetch({
      downloadAsset: () => errorResponse(404, 'NOT_FOUND', 'Resource not found.'),
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openHistoryTab(user)

    await user.click(await screen.findByRole('button', { name: '下载结果 hero.png' }))

    expect(await screen.findByText('结果下载失败，请稍后重试。')).toBeInTheDocument()
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/assets/asset_generated_1/download',
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
    expect(screen.getByRole('button', { name: '下载结果 hero.png' })).toBeEnabled()
  })

  it('re-edits with backend editSourceAssetId and blocks task creation when the source asset becomes unavailable', async () => {
    const user = userEvent.setup()
    let detailLoads = 0
    const fetchImpl = createHistoryFetch({
      getAsset: () => {
        detailLoads += 1
        return detailLoads === 1 ? successResponse(generatedAsset) : errorResponse(404, 'NOT_FOUND', 'Resource not found.')
      },
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openHistoryTab(user)

    await user.click(await screen.findByRole('button', { name: '再次编辑 hero.png' }))
    expect(await screen.findByText('已准备基于后端资产再次编辑。')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(await screen.findByText('再次编辑所需资产不可用，请刷新历史后重试。')).toBeInTheDocument()
    expect(
      fetchImpl.mock.calls.filter(([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST'),
    ).toHaveLength(0)
  })

  it('opens the current backend result with real asset and task detail data instead of a no-op', async () => {
    const user = userEvent.setup()
    const fetchImpl = createHistoryFetch({
      createTask: () =>
        successResponse({
          ...task,
          status: 'QUEUED',
          outputAssetIds: [],
        }, 201),
    })
    vi.stubGlobal('fetch', fetchImpl)
    vi.stubGlobal('EventSource', FakeEventSource)

    render(<App />)

    await user.type(await screen.findByLabelText('提示词'), 'Clean Amazon product image')
    await waitFor(() => {
      expect(screen.getByLabelText('提示词')).toHaveValue('Clean Amazon product image')
    })
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    await waitFor(() => {
      expect(FakeEventSource.instances).toHaveLength(1)
    })
    FakeEventSource.instances[0].emit('IMAGE_OUTPUT', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'RUNNING',
      attempt: 1,
      assetId: 'asset_generated_1',
      outputIndex: 0,
      previewUrl: '/api/v1/assets/asset_generated_1/download',
      width: 1536,
      height: 1024,
      mimeType: 'image/png',
      sizeBytes: 2048,
    }, 'evt_1')

    await user.click(await screen.findByRole('button', { name: '打开当前结果详情' }))

    expect((await screen.findAllByText('Clean Amazon product image')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('hero.png').length).toBeGreaterThan(0)
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain('/api/v1/assets/asset_generated_1')
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain('/api/v1/tasks/task_1')
  })

  it('submits edit tasks with backend asset identifiers instead of relying on IndexedDB blobs', async () => {
    const user = userEvent.setup()
    const fetchImpl = createHistoryFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)
    await openHistoryTab(user)

    await user.click(await screen.findByRole('button', { name: '再次编辑 hero.png' }))
    await waitFor(() => {
      expect(screen.getByLabelText('图片类型')).toHaveValue('COMPARISON')
    })
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    await waitFor(() => {
      expect(
        fetchImpl.mock.calls.some(([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST'),
      ).toBe(true)
    })

    const createCall = fetchImpl.mock.calls.find(
      ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
    )
    expect(JSON.parse(createCall?.[1]?.body as string)).toMatchObject({
      type: 'IMAGE_EDIT',
      editSourceAssetId: 'asset_generated_1',
      imageType: 'COMPARISON',
    })
  })

})

class FakeEventSource {
  static readonly instances: FakeEventSource[] = []

  readonly url: string
  readonly withCredentials: boolean
  onerror: ((event: Event) => void) | null = null
  onopen: ((event: Event) => void) | null = null
  private readonly listeners = new Map<string, Set<(event: MessageEvent<string>) => void>>()

  constructor(url: string, init: EventSourceInit) {
    this.url = url
    this.withCredentials = Boolean(init.withCredentials)
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, listener: (event: MessageEvent<string>) => void) {
    const listeners = this.listeners.get(type) ?? new Set()
    listeners.add(listener)
    this.listeners.set(type, listeners)
  }

  removeEventListener(type: string, listener: (event: MessageEvent<string>) => void) {
    this.listeners.get(type)?.delete(listener)
  }

  close() {}

  emit(type: string, data: unknown, lastEventId?: string) {
    const event = new MessageEvent<string>(type, {
      data: JSON.stringify(data),
      lastEventId,
    })
    this.listeners.get(type)?.forEach((listener) => listener(event))
  }
}

function createHistoryFetch(overrides: {
  createTask?: () => Promise<Response> | Response
  downloadAsset?: () => Promise<Response> | Response
  getAsset?: () => Promise<Response> | Response
  listProjects?: () => Promise<Response> | Response
  listProjectOneHistory?: (url: string) => Promise<Response> | Response
  listProjectTwoHistory?: (url: string) => Promise<Response> | Response
} = {}) {
  return vi.fn<typeof fetch>(async (input, init) => {
    const url = String(input)

    if (url === '/api/v1/me') {
      return successResponse(authenticatedSession)
    }
    if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
      return successResponse(page([model], 100))
    }
    if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
      return await (overrides.listProjects?.() ?? successResponse(page(projects)))
    }
    if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
      return successResponse(page([referenceAsset, generatedAsset]))
    }
    if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url.startsWith('/api/v1/projects/project_1/history?')) {
      return await (overrides.listProjectOneHistory?.(url) ?? successResponse(page([{ asset: generatedAsset, task }], 10)))
    }
    if (url.startsWith('/api/v1/projects/project_2/history?')) {
      return await (overrides.listProjectTwoHistory?.(url) ?? successResponse(page([], 10)))
    }
    if (url === '/api/v1/assets/asset_generated_1') {
      return await (overrides.getAsset?.() ?? successResponse(generatedAsset))
    }
    if (url === '/api/v1/tasks/task_1') {
      return successResponse(task)
    }
    if (url === '/api/v1/assets/asset_generated_1/download') {
      return await (
        overrides.downloadAsset?.() ??
        new Response(new Blob(['generated-bytes'], { type: 'image/png' }), {
          headers: {
            'Content-Disposition': 'attachment; filename="hero.png"',
            'Content-Type': 'image/png',
          },
        })
      )
    }
    if (url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST') {
      return await (overrides.createTask?.() ?? successResponse({ ...task, type: 'IMAGE_EDIT' }, 201))
    }

    return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
  })
}

function deferredResponse() {
  let resolve: (response: Response) => void = () => undefined
  const promise = new Promise<Response>((nextResolve) => {
    resolve = nextResolve
  })

  return { promise, resolve }
}
