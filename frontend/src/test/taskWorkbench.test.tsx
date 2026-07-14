import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { BackendControlPanel } from '../components/studio/BackendControlPanel'
import type { Model } from '../types/platform'
import type { WorkbenchImageType } from '../types/workbench'

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

const secondProject = {
  ...project,
  id: 'project_2',
  name: 'Winter Launch',
  brand: 'Northwind',
  asin: 'B000SECOND',
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

    await screen.findByLabelText('当前产品')
    await user.click(screen.getByRole('button', { name: '产品素材' }))
    expect(await screen.findByRole('heading', { name: '产品素材库' })).toBeInTheDocument()
    expect(await screen.findByText('reference.png')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '作为参考图 reference.png' }))
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', {
        name: '关闭弹窗',
      }),
    )
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.selectOptions(screen.getByLabelText('图片比例'), '1536x1024')
    await user.selectOptions(screen.getByLabelText('生成质量'), 'hd')
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
    expect(stream.url).toBe('/api/v1/events/tasks')
    expect(stream.withCredentials).toBe(true)

    stream.emit(
      'TASK_STARTED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:01Z',
      },
      'evt_1',
    )
    stream.emit(
      'IMAGE_OUTPUT',
      {
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
      },
      'evt_2',
    )

    const outputImage = await screen.findByAltText('生成结果')
    expect(outputImage).toHaveAttribute('src', '/api/v1/assets/asset_output_1/download')
    expect(outputImage.getAttribute('src')).not.toMatch(/^(blob:|data:)/)
    expect(screen.getByText(/RUNNING/)).toBeInTheDocument()

    stream.emit(
      'TASK_COMPLETED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'SUCCEEDED',
        attempt: 1,
        finishedAt: '2026-05-17T00:00:04Z',
      },
      'evt_3',
    )

    expect(await screen.findByText(/SUCCEEDED/)).toBeInTheDocument()
  })

  it('uses the model configuration names for size and quality options', async () => {
    vi.stubGlobal(
      'fetch',
      createWorkbenchFetch({
        listModels: () => successResponse(
          page(
            [{
              ...model,
              providerType: 'OPENAI_COMPATIBLE',
              supportedSizes: ['auto', '1:1', '1.62:1', '16:9'],
              supportedQualities: ['auto', 'low', 'medium', 'high'],
            }],
            100,
          ),
        ),
      }),
    )

    render(<App />)

    const sizeSelect = await screen.findByLabelText('图片比例')
    const qualitySelect = screen.getByLabelText('生成质量')

    expect(within(sizeSelect).getAllByRole('option').map((option) => option.getAttribute('value'))).toEqual([
      'auto',
      '1:1',
      '1.62:1',
      '16:9',
    ])
    expect(within(qualitySelect).getAllByRole('option').map((option) => option.textContent)).toEqual([
      '自动',
      '低质量',
      '中等质量',
      '高质量',
    ])
  })

  it('defaults auto options to first while preserving later user selections', async () => {
    const user = userEvent.setup()
    let modelRequestCount = 0
    vi.stubGlobal(
      'fetch',
      createWorkbenchFetch({
        listModels: () => {
          modelRequestCount += 1
          return successResponse(
            page(
              [
                {
                  ...model,
                  supportedSizes: ['1024x1024', 'auto', '1536x1024'],
                  supportedQualities: ['medium', 'auto', 'high'],
                },
              ],
              100,
            ),
          )
        },
      }),
    )

    render(<App />)

    const sizeSelect = await screen.findByLabelText('图片比例')
    const qualitySelect = screen.getByLabelText('生成质量')

    expect(within(sizeSelect).getAllByRole('option').map((option) => option.getAttribute('value'))).toEqual([
      'auto',
      '1024x1024',
      '1536x1024',
    ])
    expect(within(qualitySelect).getAllByRole('option').map((option) => option.getAttribute('value'))).toEqual([
      'auto',
      'medium',
      'high',
    ])
    expect(sizeSelect).toHaveValue('auto')
    expect(qualitySelect).toHaveValue('auto')

    await user.selectOptions(sizeSelect, '1536x1024')
    await user.selectOptions(qualitySelect, 'high')
    await user.click(screen.getByRole('button', { name: '刷新模型' }))

    await waitFor(() => expect(modelRequestCount).toBe(2))
    expect(sizeSelect).toHaveValue('1536x1024')
    expect(qualitySelect).toHaveValue('high')
  })

  it('submits the selected ecommerce image type through the backend task API', async () => {
    const user = userEvent.setup()
    const fetchImpl = createWorkbenchFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByLabelText('当前产品')
    await user.click(screen.getByRole('tab', { name: '尺寸图' }))
    await user.type(screen.getByLabelText('提示词'), 'Create a dimension diagram')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const taskCreateCall = fetchImpl.mock.calls.find(
      ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
    )
    expect(taskCreateCall).toBeDefined()
    expect(JSON.parse(taskCreateCall?.[1]?.body as string)).toMatchObject({
      imageType: 'DIMENSION',
    })
  })

  it('keeps generation history visible while opening product assets on demand', async () => {
    const user = userEvent.setup()
    const fetchImpl = createWorkbenchFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByLabelText('当前产品')
    expect(screen.queryByRole('heading', { name: '产品素材库' })).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '生成历史' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '产品素材' }))
    expect(await screen.findByRole('heading', { name: '产品素材库' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '生成历史' })).toBeInTheDocument()
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
    expect(
      within(imageTypeSelect)
        .getAllByRole('option')
        .map((option) => option.getAttribute('value')),
    ).toEqual(['MAIN', 'A_PLUS', 'SCENE', 'DETAIL', 'DIMENSION', 'SELLING_POINT', 'PROMOTION', 'COMPARISON'])

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

  it('allows users to leave a history draft image type without being snapped back by the old draft', async () => {
    const user = userEvent.setup()
    const editDraft = {
      prompt: 'A+ draft from history',
      modelId: 'model_1',
      imageType: 'A_PLUS',
    }

    function DraftHarness() {
      const [imageType, setImageType] = useState<WorkbenchImageType>('A_PLUS')
      const [prompts, setPrompts] = useState<Record<string, string>>({})

      return (
        <BackendControlPanel
          draft={editDraft}
          imageType={imageType}
          isGenerating={false}
          modelStatus="success"
          models={[model as unknown as Model]}
          onError={vi.fn()}
          onGenerate={vi.fn(async () => {})}
          onImageTypeChange={setImageType}
          onPromptChange={(prompt) => setPrompts((current) => ({ ...current, [imageType]: prompt }))}
          onRefreshModels={vi.fn()}
          prompt={prompts[imageType] ?? ''}
        />
      )
    }

    render(<DraftHarness />)

    expect(await screen.findByLabelText('提示词')).toHaveValue('A+ draft from history')
    await user.selectOptions(screen.getByLabelText('图片类型'), 'MAIN')

    expect(screen.getByLabelText('图片类型')).toHaveValue('MAIN')
    expect(screen.getByLabelText('提示词')).toHaveValue('')
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

    await screen.findByLabelText('当前产品')
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.dblClick(screen.getByRole('button', { name: '生成图片' }))

    expect(
      fetchImpl.mock.calls.filter(
        ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
      ),
    ).toHaveLength(1)

    resolveCreate?.(successResponse(task, 201))
    await waitFor(() => {
      expect(FakeEventSource.instances).toHaveLength(1)
    })
  })

  it('allows a second image task while the first task is still running and tracks both through one global event stream', async () => {
    const user = userEvent.setup()
    let createCount = 0
    const fetchImpl = createWorkbenchFetch({
      createTask: () => {
        createCount += 1
        return successResponse(
          {
            ...task,
            id: `task_${createCount}`,
            imageType: createCount === 1 ? 'MAIN' : 'A_PLUS',
            prompt: createCount === 1 ? 'White background hero image' : 'A+ lifestyle banner',
          },
          201,
        )
      },
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await screen.findByLabelText('当前产品')
    await user.type(screen.getByLabelText('提示词'), 'White background hero image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const stream = FakeEventSource.instances[0]
    expect(stream.url).toBe('/api/v1/events/tasks')
    stream.emit(
      'TASK_STARTED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:01Z',
      },
      'evt_parallel_1',
    )

    await user.click(screen.getByRole('tab', { name: 'A+ 图片' }))
    await user.type(screen.getByLabelText('提示词'), 'A+ lifestyle banner')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(
      fetchImpl.mock.calls.filter(
        ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
      ),
    ).toHaveLength(2)
    expect(FakeEventSource.instances).toHaveLength(1)
    const historyRail = screen.getByRole('complementary', {
      name: '图片生成历史',
    })
    expect(within(historyRail).getByText('1 个进行中')).toBeInTheDocument()
    expect(within(historyRail).queryByText('White background hero image')).not.toBeInTheDocument()
  })

  it('shows independent generating and completed indicators on image type tabs', async () => {
    const user = userEvent.setup()
    let createCount = 0
    vi.stubGlobal(
      'fetch',
      createWorkbenchFetch({
        createTask: () => {
          createCount += 1
          return successResponse(
            {
              ...task,
              id: `task_${createCount}`,
              imageType: createCount === 1 ? 'MAIN' : 'A_PLUS',
            },
            201,
          )
        },
      }),
    )

    render(<App />)

    await user.type(await screen.findByLabelText('提示词'), 'White background hero image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    expect(screen.getByRole('tab', { name: '主图' })).toHaveAccessibleDescription('正在生成 1 个任务')

    await user.click(screen.getByRole('tab', { name: 'A+ 图片' }))
    await user.type(screen.getByLabelText('提示词'), 'A+ lifestyle banner')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    expect(screen.getByRole('tab', { name: 'A+ 图片' })).toHaveAccessibleDescription('正在生成 1 个任务')

    FakeEventSource.instances[0].emit(
      'TASK_COMPLETED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'SUCCEEDED',
        attempt: 1,
        finishedAt: '2026-05-17T00:00:04Z',
      },
      'evt_image_type_indicator',
    )

    await waitFor(() => {
      expect(screen.getByRole('tab', { name: '主图' })).toHaveAccessibleDescription('已完成 1 个任务')
    })
    expect(screen.getByRole('tab', { name: 'A+ 图片' })).toHaveAccessibleDescription('正在生成 1 个任务')
  })

  it('only shows image type task indicators for the selected product', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch({ projects: [project, secondProject] }))

    render(<App />)

    await user.type(await screen.findByLabelText('提示词'), 'White background hero image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    expect(screen.getByRole('tab', { name: '主图' })).toHaveAccessibleDescription('正在生成 1 个任务')

    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')
    expect(screen.getByRole('tab', { name: '主图' })).not.toHaveAccessibleDescription()
  })

  it('restores active tasks from the backend after the workbench is refreshed', async () => {
    const runningTask = {
      ...task,
      status: 'RUNNING',
      imageType: 'MAIN',
      prompt: '刷新后仍应显示的主图任务',
    }
    const fetchImpl = createWorkbenchFetch({ activeTasks: [runningTask] })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    const historyRail = await screen.findByRole('complementary', {
      name: '图片生成历史',
    })
    expect(await within(historyRail).findByText('1 个进行中')).toBeInTheDocument()
    expect(within(historyRail).getByText('刷新后仍应显示的主图任务')).toBeInTheDocument()
    expect(FakeEventSource.instances).toHaveLength(1)
    expect(FakeEventSource.instances[0].url).toBe('/api/v1/events/tasks')
  })

  it('keeps prompt drafts independent for each product image type', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    await screen.findByLabelText('当前产品')
    await user.type(screen.getByLabelText('提示词'), 'Main image draft')
    await user.click(screen.getByRole('tab', { name: '尺寸图' }))
    expect(screen.getByLabelText('提示词')).toHaveValue('')
    await user.type(screen.getByLabelText('提示词'), 'Dimension image draft')

    await user.click(screen.getByRole('tab', { name: '主图' }))
    expect(screen.getByLabelText('提示词')).toHaveValue('Main image draft')
    await user.click(screen.getByRole('tab', { name: '尺寸图' }))
    expect(screen.getByLabelText('提示词')).toHaveValue('Dimension image draft')
  })

  it('notifies when a background task finishes and can return to its product image type', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    await screen.findByLabelText('当前产品')
    await user.type(screen.getByLabelText('提示词'), 'White background hero image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    await user.click(screen.getByRole('tab', { name: 'A+ 图片' }))

    const stream = FakeEventSource.instances[0]
    stream.emit(
      'IMAGE_OUTPUT',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        assetId: 'asset_output_1',
        outputIndex: 0,
        previewUrl: '/api/v1/assets/asset_output_1/download',
        width: 1024,
        height: 1024,
        mimeType: 'image/png',
        sizeBytes: 1024,
      },
      'evt_notify_1',
    )

    stream.emit(
      'TASK_COMPLETED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'SUCCEEDED',
        attempt: 1,
        finishedAt: '2026-05-17T00:00:04Z',
      },
      'evt_notify_2',
    )

    expect(await screen.findByText('主图生成完成')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '查看主图结果' }))
    expect(screen.getByRole('tab', { name: '主图' })).toHaveAttribute('aria-selected', 'true')
    expect(await screen.findByAltText('生成结果')).toHaveAttribute('src', '/api/v1/assets/asset_output_1/download')
    expect(screen.getByTestId('result-canvas-content')).toHaveClass('min-h-0', 'overflow-y-auto')
    expect(screen.getByTestId('result-canvas-image')).toHaveClass('min-h-0', 'flex-1')
    expect(screen.getByTestId('result-canvas-actions')).toHaveClass('shrink-0')
  })

  it('explains provider quota failures instead of suggesting a blind retry', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    await user.type(await screen.findByLabelText('提示词'), 'Generate a product image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    await user.click(screen.getByRole('tab', { name: 'A+ 图片' }))

    FakeEventSource.instances[0].emit(
      'TASK_FAILED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'FAILED',
        attempt: 1,
        errorCode: 'PROVIDER_INSUFFICIENT_QUOTA',
        message: 'Provider account quota is insufficient.',
        finishedAt: '2026-05-17T00:00:04Z',
      },
      'evt_quota_failed',
    )

    expect(await screen.findByText(/中转站余额不足，请充值后再生成/)).toBeInTheDocument()
  })

  it('returns to the correct product and image type when a background task finishes elsewhere', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch({ projects: [project, secondProject] }))

    render(<App />)

    await screen.findByLabelText('当前产品')
    await user.click(screen.getByRole('tab', { name: 'A+ 图片' }))
    await user.type(screen.getByLabelText('提示词'), 'A+ banner for the first product')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    await user.selectOptions(screen.getByLabelText('当前产品'), 'project_2')

    const stream = FakeEventSource.instances[0]
    stream.emit(
      'TASK_COMPLETED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'SUCCEEDED',
        attempt: 1,
        finishedAt: '2026-05-17T00:00:04Z',
      },
      'evt_cross_product',
    )

    await user.click(await screen.findByRole('button', { name: '查看A+ 图片结果' }))
    expect(screen.getByLabelText('当前产品')).toHaveValue('project_1')
    expect(screen.getByRole('tab', { name: 'A+ 图片' })).toHaveAttribute('aria-selected', 'true')
  })

  it('uses product language and keeps assets and records out of the default generation workspace', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    expect(await screen.findByLabelText('当前产品')).toHaveValue('project_1')
    expect(screen.getByRole('button', { name: '新增产品' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '项目资产库' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '产品素材' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '生成历史' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '产品素材' }))
    expect(await screen.findByRole('heading', { name: '产品素材库' })).toBeInTheDocument()
  })

  it('renders products as top tabs with the add action at the end', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch({ projects: [project, secondProject] }))

    render(<App />)

    const productTabs = await screen.findByRole('tablist', {
      name: '产品列表',
    })
    const fixedWorkspace = screen.getByTestId('fixed-product-workspace')
    const workbench = screen.getByTestId('generation-workbench')
    expect(fixedWorkspace).toContainElement(productTabs)
    expect(fixedWorkspace).toHaveClass('lg:h-[calc(100dvh-88px)]', 'lg:overflow-hidden')
    expect(workbench).toHaveClass('lg:h-full', 'lg:min-h-0')
    expect(workbench).not.toHaveClass('lg:h-[calc(100dvh-152px)]', 'lg:min-h-[680px]')
    expect(productTabs.closest('section')).toHaveClass('sticky', 'top-16')
    expect(productTabs).toHaveAttribute('data-fill-axis', 'horizontal')
    expect(productTabs).toHaveAttribute('data-layout', 'wrapped')
    expect(productTabs).toHaveAttribute('data-scroll-behavior', 'fixed')
    expect(productTabs).toHaveClass('flex-wrap', 'overflow-visible')
    expect(productTabs).not.toHaveClass('overflow-x-auto')
    const selectedProductTab = within(productTabs).getByRole('tab', { name: /Summer Launch/ })
    const unselectedProductTab = within(productTabs).getByRole('tab', { name: /Winter Launch/ })
    expect(selectedProductTab).toHaveAttribute('aria-selected', 'true')
    expect(selectedProductTab).toHaveClass('flex-none', 'max-w-72', 'border-ink-600', 'bg-white', 'ring-1', 'ring-ink-600')
    expect(selectedProductTab).not.toHaveClass('flex-1', 'bg-ink-900', 'focus:ring-amazon-500/30')
    expect(within(selectedProductTab).getByText('当前')).toBeInTheDocument()
    expect(unselectedProductTab).toHaveClass('border-ink-200', 'bg-ink-50')
    expect(within(unselectedProductTab).queryByText('当前')).not.toBeInTheDocument()
    await user.click(unselectedProductTab)
    expect(unselectedProductTab).toHaveAttribute('aria-selected', 'true')
    expect(within(unselectedProductTab).getByText('当前')).toBeInTheDocument()
    expect(within(productTabs).getByRole('button', { name: '新增产品' })).toBeInTheDocument()
  })

  it('opens an empty create form from the product add button', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch({ projects: [project, secondProject] }))

    render(<App />)

    const productTabs = await screen.findByRole('tablist', { name: '产品列表' })
    await user.click(within(productTabs).getByRole('button', { name: '新增产品' }))

    expect(await screen.findByRole('dialog', { name: '产品管理' })).toBeInTheDocument()
    const editor = await screen.findByRole('dialog', { name: '新建产品' })
    expect(within(editor).getByLabelText('产品名称')).toHaveValue('')
    expect(screen.queryByRole('dialog', { name: '编辑产品' })).not.toBeInTheDocument()
  })

  it('places image types on the left and generation history on the right of the workbench', async () => {
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    const imageTypeNavigation = await screen.findByRole('navigation', {
      name: '商品图片类型选项',
    })
    const workbench = screen.getByTestId('generation-workbench')
    const historyRail = screen.getByRole('complementary', {
      name: '图片生成历史',
    })

    expect(imageTypeNavigation).toHaveAttribute('data-desktop-position', 'left')
    expect(imageTypeNavigation).toHaveAttribute('data-fill-axis', 'vertical')
    expect(imageTypeNavigation).toHaveAttribute('data-layout', 'content-sized')
    const imageTypeTabList = within(imageTypeNavigation).getByRole('tablist', { name: '选择图片类型' })
    const selectedImageType = within(imageTypeTabList).getByRole('tab', { name: '主图' })
    expect(selectedImageType).toHaveClass('bg-ink-100', 'border-ink-300', 'focus:ring-ink-300')
    expect(selectedImageType).not.toHaveClass('bg-amazon-500', 'lg:h-full', 'focus:ring-amazon-500/30')
    expect(historyRail).toHaveAttribute('data-desktop-position', 'right')
    expect(workbench.firstElementChild).toBe(imageTypeNavigation)
    expect(workbench.lastElementChild).toBe(historyRail)
  })

  it('selects an existing product reference above the prompt and submits its asset id', async () => {
    const user = userEvent.setup()
    const fetchImpl = createWorkbenchFetch()
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    const prompt = await screen.findByLabelText('提示词')
    const referenceButton = await screen.findByRole('button', {
      name: '选择产品参考图 reference.png',
    })
    expect(referenceButton.compareDocumentPosition(prompt) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()

    await user.click(referenceButton)
    await user.type(prompt, 'Use the selected product as the visual reference')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    await waitFor(() => {
      const createCall = fetchImpl.mock.calls.find(
        ([url, init]) => url === '/api/v1/projects/project_1/tasks' && init?.method === 'POST',
      )
      expect(JSON.parse(createCall?.[1]?.body as string)).toMatchObject({
        referenceAssetIds: ['asset_1'],
      })
    })
  })

  it('shows an in-progress task inside the right-side generation history', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    await user.type(await screen.findByLabelText('提示词'), 'A task visible in the history rail')
    await user.click(screen.getByRole('button', { name: '生成图片' }))
    await waitFor(() => {
      expect(FakeEventSource.instances).toHaveLength(1)
    })
    FakeEventSource.instances[0].emit(
      'TASK_STARTED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:01Z',
      },
      'evt_history_running',
    )

    const historyRail = screen.getByRole('complementary', {
      name: '图片生成历史',
    })
    await waitFor(() => {
      expect(within(historyRail).getByText('生成中')).toBeInTheDocument()
      expect(within(historyRail).getByText('A task visible in the history rail')).toBeInTheDocument()
    })
  })

  it('shows operation feedback as an overlay instead of occupying the sticky header', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    await user.type(await screen.findByLabelText('提示词'), 'Create a floating feedback message')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const feedback = await screen.findByText('任务已创建，结果会通过实时事件流更新。')
    expect(feedback.closest('header')).toBeNull()
    expect(feedback.closest('[data-overlay-notice="true"]')).not.toBeNull()
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

    await screen.findByLabelText('当前产品')
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

    await screen.findByLabelText('当前产品')
    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    const stream = FakeEventSource.instances[0]
    stream.emit(
      'TASK_STARTED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:01Z',
      },
      'evt_1',
    )
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

    stream.emit(
      'TASK_CANCELLED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'CANCELLED',
        attempt: 1,
        finishedAt: '2026-05-17T00:00:03Z',
      },
      'evt_2',
    )
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

    stream.emit(
      'TASK_RETRIED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RETRYING',
        attempt: 2,
        previousStatus: 'CANCELLED',
      },
      'evt_3',
    )
    expect(await screen.findByText(/RETRYING/)).toBeInTheDocument()
  })
})

function createWorkbenchFetch(
  overrides: {
    activeTasks?: Array<typeof task>
    createTask?: () => Promise<Response> | Response
    listModels?: () => Promise<Response> | Response
    projects?: Array<typeof project>
  } = {},
) {
  return vi.fn<typeof fetch>(async (input, init) => {
    const url = String(input)

    if (url === '/api/v1/me') {
      return successResponse(authenticatedSession)
    }
    if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
      return await (overrides.listModels?.() ?? successResponse(page([model], 100)))
    }
    if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
      return successResponse(page(overrides.projects ?? [project]))
    }
    if (url === '/api/v1/projects/project_1/assets?pageNum=1&pageSize=50') {
      return successResponse(page([asset]))
    }
    if (url.startsWith('/api/v1/projects/project_1/history?')) {
      return successResponse(page([]))
    }
    if (url.startsWith('/api/v1/projects/project_1/tasks?') && (!init?.method || init.method === 'GET')) {
      const status = new URL(url, 'http://localhost').searchParams.get('status')
      return successResponse(page((overrides.activeTasks ?? []).filter((candidate) => candidate.status === status)))
    }
    if (url === '/api/v1/projects/project_2/assets?pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }
    if (url.startsWith('/api/v1/projects/project_2/history?')) {
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
