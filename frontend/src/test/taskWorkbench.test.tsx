import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { BackendControlPanel } from '../components/studio/BackendControlPanel'
import type { Model } from '../types/platform'

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
  supportedSizes: ['1024x1024', '1536x1024'],
  supportedQualities: ['standard', 'hd'],
  supportedOutputFormats: ['png', 'jpeg'],
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
  status: 'QUEUED',
  prompt: 'Clean Amazon product image',
  providerId: 'provider_1',
  modelId: 'model_1',
  imageType: '',
  parameters: {
    size: '1536x1024',
    quality: 'hd',
    outputFormat: 'jpeg',
    outputCount: 4,
  },
  inputAssetIds: ['asset_1'],
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

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_task_workbench',
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
      requestId: 'req_task_workbench_error',
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

describe('task-backed workbench', () => {
  beforeEach(async () => {
    FakeEventSource.instances.length = 0
    vi.stubGlobal('EventSource', FakeEventSource)
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:legacy-reference'),
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

  it('submits visible backend parameters once, renders backend asset outputs, and does not call browser Provider adapters', async () => {
    const user = userEvent.setup()
    localStorage.setItem(
      'amazon-ai-product-image-studio.settings',
      JSON.stringify({
        providers: {
          openai: {
            apiUrl: 'https://legacy-provider.example/v1',
            apiKey: 'legacy-openai-key',
          },
        },
      }),
    )
    const fetchImpl = createWorkbenchFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('heading', { name: '项目资产库' })).toBeInTheDocument()
    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.selectOptions(screen.getByLabelText('尺寸'), '1536x1024')
    await user.selectOptions(screen.getByLabelText('质量'), 'hd')
    await user.selectOptions(screen.getByLabelText('输出格式'), 'jpeg')
    await user.selectOptions(screen.getByLabelText('生成张数'), '4')
    expect(screen.getByLabelText('图片类型')).toHaveValue('MAIN')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const taskCreateCall = fetchImpl.mock.calls.find(
      ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
    )
    expect(taskCreateCall).toBeDefined()
    expect(JSON.parse(taskCreateCall?.[1]?.body as string)).toEqual({
      type: 'IMAGE_GENERATION',
      prompt: 'Clean Amazon product image',
      providerId: 'provider_1',
      modelId: 'model_1',
      imageType: 'MAIN',
      referenceAssetIds: ['asset_1'],
      parameters: {
        size: '1536x1024',
        quality: 'hd',
        outputFormat: 'jpeg',
        outputCount: 4,
      },
    })

    const stream = FakeEventSource.instances[0]
    expect(stream.url).toContain('/api/v1/events/tasks?')
    expect(stream.withCredentials).toBe(true)

    stream.emit('TASK_STARTED', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'RUNNING',
      attempt: 1,
      startedAt: '2026-05-17T00:00:01Z',
    }, 'evt_1')
    stream.emit('IMAGE_OUTPUT', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'RUNNING',
      attempt: 1,
      assetId: 'asset_output_1',
      outputIndex: 0,
      previewUrl: '/api/v1/assets/asset_output_1/download',
      width: 1536,
      height: 1024,
      mimeType: 'image/png',
      sizeBytes: 2048,
    }, 'evt_2')

    const outputImage = await screen.findByAltText('生成结果')
    expect(outputImage).toHaveAttribute('src', '/api/v1/assets/asset_output_1/download')
    expect(outputImage.getAttribute('src')).not.toMatch(/^(blob:|data:)/)
    expect(screen.getByText(/RUNNING/)).toBeInTheDocument()

    stream.emit('TASK_COMPLETED', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'SUCCEEDED',
      attempt: 1,
      finishedAt: '2026-05-17T00:00:04Z',
    }, 'evt_3')

    expect(await screen.findByText(/SUCCEEDED/)).toBeInTheDocument()
  })

  it('submits the selected ecommerce image type through the backend task API', async () => {
    const user = userEvent.setup()
    const fetchImpl = createWorkbenchFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByRole('heading', { name: '项目资产库' })
    await user.type(screen.getByLabelText('提示词'), 'Create a dimension diagram')
    await user.selectOptions(screen.getByLabelText('图片类型'), 'DIMENSION')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const taskCreateCall = fetchImpl.mock.calls.find(
      ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
    )
    expect(taskCreateCall).toBeDefined()
    expect(JSON.parse(taskCreateCall?.[1]?.body as string)).toMatchObject({
      imageType: 'DIMENSION',
    })
  })

  it('normalizes an invalid edit draft image type before it can be submitted', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn(async () => {})

    render(
      <BackendControlPanel
        draft={{
          prompt: 'Draft from history',
          modelId: 'model_1',
          imageType: 'NOT_ALLOWED',
        }}
        isGenerating={false}
        modelStatus="success"
        models={[model as unknown as Model]}
        onError={vi.fn()}
        onGenerate={onGenerate}
        onRefreshModels={vi.fn()}
      />,
    )

    const imageTypeSelect = screen.getByLabelText('图片类型')
    expect(imageTypeSelect).toHaveValue('MAIN')
    expect(within(imageTypeSelect).getAllByRole('option').map((option) => option.getAttribute('value'))).toEqual([
      'MAIN',
      'A_PLUS',
      'SCENE',
      'DETAIL',
      'DIMENSION',
      'SELLING_POINT',
      'COMPARISON',
    ])

    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      {
        prompt: 'Draft from history',
      },
      expect.objectContaining({
        imageType: 'MAIN',
      }),
    )
  })

  it('prevents duplicate task creation while a submit is already in flight', async () => {
    const user = userEvent.setup()
    let resolveCreate: ((response: Response) => void) | undefined
    const createPromise = new Promise<Response>((resolve) => {
      resolveCreate = resolve
    })
    const fetchImpl = createWorkbenchFetch({
      createTask: () => createPromise,
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByRole('heading', { name: '项目资产库' })
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.dblClick(screen.getByRole('button', { name: '生成图片' }))

    expect(
      fetchImpl.mock.calls.filter(([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST'),
    ).toHaveLength(1)

    resolveCreate?.(successResponse(task, 201))
    await waitFor(() => {
      expect(FakeEventSource.instances).toHaveLength(1)
    })
  })

  it('refreshes models and requires reselection after stale task create validation fails', async () => {
    const user = userEvent.setup()
    let modelRequestCount = 0
    const fetchImpl = createWorkbenchFetch({
      listModels: () => {
        modelRequestCount += 1
        return successResponse(page(modelRequestCount === 1 ? [model] : [], 100))
      },
      createTask: () => errorResponse(422, 'VALIDATION_ERROR', 'Invalid request.'),
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByRole('heading', { name: '项目资产库' })
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await waitFor(() => {
      expect(screen.getByLabelText('提示词')).toHaveValue('Clean Amazon product image')
    })
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(await screen.findAllByText(/当前模型或参数已失效/)).not.toHaveLength(0)
    expect(await screen.findByRole('alert')).toHaveTextContent('所选模型当前不可用')
    expect(modelRequestCount).toBe(2)
  })

  it('uses task APIs for cancel and retry while waiting for SSE to advance the canonical status', async () => {
    const user = userEvent.setup()
    const fetchImpl = createWorkbenchFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByRole('heading', { name: '项目资产库' })
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const stream = FakeEventSource.instances[0]
    stream.emit('TASK_STARTED', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'RUNNING',
      attempt: 1,
      startedAt: '2026-05-17T00:00:01Z',
    }, 'evt_1')
    expect(await screen.findByText(/RUNNING/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '取消任务' }))
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/tasks/task_1/cancel',
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect(screen.getByText(/RUNNING/)).toBeInTheDocument()

    stream.emit('TASK_CANCELLED', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'CANCELLED',
      attempt: 1,
      finishedAt: '2026-05-17T00:00:03Z',
    }, 'evt_2')
    expect(await screen.findByText(/CANCELLED/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '重试任务' }))
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/tasks/task_1/retry',
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect(screen.getByText(/CANCELLED/)).toBeInTheDocument()

    stream.emit('TASK_RETRIED', {
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'RETRYING',
      attempt: 2,
      previousStatus: 'CANCELLED',
    }, 'evt_3')
    expect(await screen.findByText(/RETRYING/)).toBeInTheDocument()
  })

})

function createWorkbenchFetch(overrides: {
  createTask?: () => Promise<Response> | Response
  listModels?: () => Promise<Response> | Response
} = {}) {
  return vi.fn<typeof fetch>(async (input, init) => {
    const url = String(input)

    if (url === '/api/v1/me') {
      return successResponse(authenticatedSession)
    }
    if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
      return await (overrides.listModels?.() ?? successResponse(page([model], 100)))
    }
    if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
      return successResponse(page([project]))
    }
    if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
      return successResponse(page([asset]))
    }
    if (url === '/api/v1/projects/project_1/history?pageNum=1&pageSize=10') {
      return successResponse(page([]))
    }
    if (url === '/api/v1/assets/asset_1/download') {
      return new Response(new Blob(['reference-bytes'], { type: 'image/png' }), {
        headers: {
          'Content-Disposition': 'attachment; filename="reference.png"',
          'Content-Type': 'image/png',
        },
      })
    }
    if (url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST') {
      return await (overrides.createTask?.() ?? successResponse(task, 201))
    }
    if (url === '/api/v1/tasks/task_1/cancel' && init?.method === 'POST') {
      return successResponse({ ...task, status: 'CANCELLED' })
    }
    if (url === '/api/v1/tasks/task_1/retry' && init?.method === 'POST') {
      return successResponse({ ...task, status: 'QUEUED', attempt: 2 })
    }

    return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
  })
}
