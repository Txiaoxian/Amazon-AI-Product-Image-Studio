import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { db } from '../db/dexie'
import { createHistoryItem } from '../db/historyRepository'
import { saveImage } from '../db/imageRepository'
import * as providerRegistry from '../providers/registry'

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
  imageType: 'MAIN',
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

describe('backend history asset source', () => {
  beforeEach(async () => {
    FakeEventSource.instances.length = 0
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:legacy-history'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })
    await db.delete()
    await db.open()
  })

  afterEach(async () => {
    cleanup()
    localStorage.clear()
    await db.delete()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('uses backend tasks and assets as the default history source while keeping legacy history explicit and collapsed', async () => {
    const generatedImage = await saveImage({
      blob: new Blob(['legacy-image'], { type: 'image/png' }),
      mimeType: 'image/png',
      purpose: 'generated',
      size: 12,
      width: 1,
      height: 1,
    })
    await createHistoryItem({
      generatedImageId: generatedImage.id,
      referenceImageIds: [],
      request: {
        prompt: 'Legacy local prompt',
        model: providerRegistry.getModelById('openai-gpt-image-2'),
        quality: '1K',
        aspectRatio: '1:1',
        imageCount: 1,
        references: [],
        referenceImageUrls: [],
      },
      result: {
        blob: generatedImage.blob,
        mimeType: generatedImage.mimeType,
        width: 1,
        height: 1,
        fileSize: generatedImage.size,
        durationMs: 42,
      },
    })
    const fetchImpl = createHistoryFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('button', { name: '查看结果 hero.png' })).toBeInTheDocument()
    expect(screen.queryByText('Legacy local prompt')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '查看旧本地历史' })).toBeInTheDocument()

    await userEvent.setup().click(screen.getByRole('button', { name: '查看旧本地历史' }))

    expect(await screen.findByText('旧本地历史（兼容）')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: '再次编辑' })).toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain('/api/v1/projects/project_1/tasks?pageNum=1&pageSize=50')
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain('/api/v1/projects/project_1/assets?kind=GENERATED&pageNum=1&pageSize=50')
    expect(fetchImpl.mock.calls.map(([url]) => url)).toContain('/api/v1/projects/project_1/assets?kind=EDITED&pageNum=1&pageSize=50')
  })

  it('shows a project-scoped empty state after switching projects without leaking the previous project history', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createHistoryFetch())

    render(<App />)

    expect(await screen.findByRole('button', { name: '查看结果 hero.png' })).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('当前项目'), 'project_2')

    expect(await screen.findByText('当前项目暂无结果历史')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '查看结果 hero.png' })).not.toBeInTheDocument()
  })

  it('ignores stale history responses from the previous project after a project switch', async () => {
    const user = userEvent.setup()
    const projectOneTasks = deferredResponse()
    const projectOneGeneratedAssets = deferredResponse()
    const projectOneEditedAssets = deferredResponse()
    vi.stubGlobal(
      'fetch',
      createHistoryFetch({
        listProjectOneEditedAssets: () => projectOneEditedAssets.promise,
        listProjectOneGeneratedAssets: () => projectOneGeneratedAssets.promise,
        listProjectOneTasks: () => projectOneTasks.promise,
      }),
    )

    render(<App />)

    await screen.findByText('Summer Launch')
    await user.selectOptions(screen.getByLabelText('当前项目'), 'project_2')
    expect(await screen.findByText('当前项目暂无结果历史')).toBeInTheDocument()

    projectOneTasks.resolve(successResponse(page([task])))
    projectOneGeneratedAssets.resolve(successResponse(page([generatedAsset])))
    projectOneEditedAssets.resolve(successResponse(page([])))

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: '查看结果 hero.png' })).not.toBeInTheDocument()
    })
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
    await user.click(screen.getByRole('button', { name: '生成图片' }))
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

    await user.click(await screen.findByRole('button', { name: '再次编辑 hero.png' }))
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
    })
  })

  it('clears backend re-edit source state after entering legacy compatibility mode and returning to backend workbench', async () => {
    const user = userEvent.setup()
    const generatedImage = await saveImage({
      blob: new Blob(['legacy-bridge-image'], { type: 'image/png' }),
      mimeType: 'image/png',
      purpose: 'generated',
      size: 19,
      width: 1,
      height: 1,
    })
    await createHistoryItem({
      generatedImageId: generatedImage.id,
      referenceImageIds: [],
      request: {
        prompt: 'Legacy bridge prompt',
        model: providerRegistry.getModelById('openai-gpt-image-2'),
        quality: '1K',
        aspectRatio: '1:1',
        imageCount: 1,
        references: [],
        referenceImageUrls: [],
      },
      result: {
        blob: generatedImage.blob,
        mimeType: generatedImage.mimeType,
        width: 1,
        height: 1,
        fileSize: generatedImage.size,
        durationMs: 42,
      },
    })
    const fetchImpl = createHistoryFetch({
      createTask: () => successResponse({ ...task, type: 'IMAGE_GENERATION' }, 201),
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await user.click(await screen.findByRole('button', { name: '再次编辑 hero.png' }))
    expect(await screen.findByText('已准备基于后端资产再次编辑。')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '查看旧本地历史' }))
    const legacyHistoryItem = (await screen.findByAltText('Legacy bridge prompt')).closest('article')
    expect(legacyHistoryItem).not.toBeNull()
    await user.click(within(legacyHistoryItem as HTMLElement).getByRole('button', { name: '再次编辑' }))
    await user.click(await screen.findByRole('button', { name: '返回后端工作台' }))

    await user.type(screen.getByLabelText('提示词'), 'Fresh backend prompt')
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
      type: 'IMAGE_GENERATION',
    })
    expect(JSON.parse(createCall?.[1]?.body as string)).not.toHaveProperty('editSourceAssetId')
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
  listProjectOneEditedAssets?: () => Promise<Response> | Response
  listProjectOneGeneratedAssets?: () => Promise<Response> | Response
  listProjectOneTasks?: () => Promise<Response> | Response
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
      return successResponse(page(projects))
    }
    if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
      return successResponse(page([referenceAsset, generatedAsset]))
    }
    if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url === '/api/v1/projects/project_1/tasks?pageNum=1&pageSize=50') {
      return await (overrides.listProjectOneTasks?.() ?? successResponse(page([task])))
    }
    if (url === '/api/v1/projects/project_2/tasks?pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url === '/api/v1/projects/project_1/assets?kind=GENERATED&pageNum=1&pageSize=50') {
      return await (overrides.listProjectOneGeneratedAssets?.() ?? successResponse(page([generatedAsset])))
    }
    if (url === '/api/v1/projects/project_1/assets?kind=EDITED&pageNum=1&pageSize=50') {
      return await (overrides.listProjectOneEditedAssets?.() ?? successResponse(page([])))
    }
    if (url === '/api/v1/projects/project_2/assets?kind=GENERATED&pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url === '/api/v1/projects/project_2/assets?kind=EDITED&pageNum=1&pageSize=50') {
      return successResponse(page([]))
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
