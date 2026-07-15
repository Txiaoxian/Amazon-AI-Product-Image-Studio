import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminObservabilitySettingsPanel } from '../components/admin/AdminObservabilitySettingsPanel'
import { ApiClientError } from '../api/client'
import type { AdminApi } from '../api/admin'
import type { ModelApi } from '../api/models'
import type { ProviderApi } from '../api/providers'
import type { ApiCallLog, UsageSummary, UsageSummaryQuery } from '../types/admin'
import type { ApiPage } from '../types/api'
import type { Model, Provider } from '../types/platform'

const usageSummary = {
  dimension: 'provider',
  dimensionId: 'provider_1',
  currency: 'USD',
  recordCount: 2,
  inputTokens: 12,
  outputTokens: 34,
  imageCount: 3,
  estimatedCost: '0.12000000',
  latestCreatedAt: '2026-05-18T10:00:00Z',
} as const

const tenantUsageTotal = {
  dimension: 'tenant',
  dimensionId: 'tenant_1',
  currency: 'USD',
  recordCount: 4,
  inputTokens: 120,
  outputTokens: 340,
  imageCount: 5,
  estimatedCost: '1.25000000',
  latestCreatedAt: '2026-05-18T10:00:00Z',
} as const

const eurTenantUsageTotal = {
  ...tenantUsageTotal,
  currency: 'EUR',
  recordCount: 1,
  estimatedCost: '0.75000000',
} as const

const usageRecord = {
  id: 'usage_1',
  tenantId: 'tenant_1',
  taskId: 'task_1',
  userId: 'user_1',
  projectId: 'project_1',
  providerId: 'provider_1',
  modelId: 'model_1',
  inputTokens: 12,
  outputTokens: 34,
  imageCount: 1,
  estimatedCost: '0.12000000',
  currency: 'USD',
  rawUsage: {
    apiKey: '[REDACTED]',
    safe: 'ok',
  },
  createdAt: '2026-05-18T10:00:00Z',
} as const

const secondUsageRecord = {
  ...usageRecord,
  id: 'usage_2',
  taskId: 'task_2',
  projectId: 'project_2',
  providerId: 'provider_2',
  modelId: 'model_2',
  estimatedCost: '0.24000000',
} as const

const operationLog = {
  id: 'operation_1',
  tenantId: 'tenant_1',
  actorUserId: 'user_1',
  action: 'provider.test',
  resourceType: 'provider',
  resourceId: 'provider_1',
  ip: '127.0.0.1',
  userAgent: 'test-agent',
  metadata: {
    apiKey: '[REDACTED]',
    safe: 'ok',
  },
  createdAt: '2026-05-18T10:00:00Z',
} as const

const apiCallLog = {
  id: 'api_log_1',
  tenantId: 'tenant_1',
  taskId: 'task_1',
  providerId: 'provider_1',
  modelId: 'model_1',
  status: 'SUCCESS',
  durationMs: 123,
  requestId: 'provider_request_1',
  httpStatus: 200,
  errorCode: '',
  errorMessage: '',
  redactedRequest: {
    apiKey: '[REDACTED]',
    prompt: 'safe',
  },
  redactedResponse: {
    result: 'safe',
  },
  createdAt: '2026-05-18T10:00:00Z',
} as ApiCallLog

const secondApiCallLog: ApiCallLog = {
  ...apiCallLog,
  id: 'api_log_2',
  taskId: 'task_2' as ApiCallLog['taskId'],
  requestId: 'provider_request_2',
  redactedResponse: {
    result: 'second-safe',
  },
}

const systemSettings = {
  uploadPolicy: {
    maxFileSizeBytes: 26214400,
    maxWidth: 8192,
    maxHeight: 8192,
    maxPixels: 40000000,
  },
  taskDefaults: {
    defaultProviderId: 'provider_1',
    defaultModelId: 'model_1',
  },
  taskConcurrency: {
    tenantLimit: 4,
    userLimit: 3,
    providerLimit: 2,
    modelLimit: 1,
  },
  storageRetention: {
    deletedAssetRetentionDays: 30,
  },
  storageQuota: {
    maxBytes: 1073741824,
    usedBytes: 1048576,
  },
  logRetention: {
    operationLogRetentionDays: 90,
    apiCallLogRetentionDays: 60,
    taskEventRetentionDays: null,
  },
  constraints: {
    uploadPolicy: {
      maxFileSizeBytes: { min: 1, max: 26214400 },
      maxWidth: { min: 1, max: 8192 },
      maxHeight: { min: 1, max: 8192 },
      maxPixels: { min: 1, max: 40000000 },
    },
    taskConcurrency: {
      globalCapacity: 8,
      tenantLimit: { min: 1, max: 4 },
      userLimit: { min: 1, max: 3 },
      providerLimit: { min: 1, max: 2 },
      modelLimit: { min: 1, max: 2 },
    },
    storageRetention: { min: 1, max: 3650 },
    storageQuota: { min: 1, max: 109951162777600 },
    logRetention: { min: 1, max: 3650 },
  },
}

const providers = [
  {
    id: 'provider_1',
    type: 'OPENAI',
    name: 'OpenAI 主站',
    status: 'ENABLED',
  },
  {
    id: 'provider_2',
    type: 'OPENAI_COMPATIBLE',
    name: '备用中转站',
    status: 'ENABLED',
  },
] as unknown as Provider[]

const models = [
  {
    id: 'model_1',
    providerId: 'provider_1',
    displayName: 'GPT Image 1',
    modelName: 'gpt-image-1',
    status: 'ENABLED',
  },
  {
    id: 'model_2',
    providerId: 'provider_2',
    displayName: 'GPT Image 2',
    modelName: 'gpt-image-2',
    status: 'ENABLED',
  },
] as unknown as Model[]

function page<TRecord>(records: TRecord[], options: Partial<Omit<ApiPage<TRecord>, 'records'>> = {}): ApiPage<TRecord> {
  return {
    records,
    total: options.total ?? records.length,
    pageNum: options.pageNum ?? 1,
    pageSize: options.pageSize ?? 10,
  }
}

function createMockAdminApi(overrides: Partial<AdminApi> = {}): AdminApi {
  return {
    getUsageSummary: vi.fn().mockResolvedValue(page([])),
    listUsageRecords: vi.fn().mockResolvedValue(page([])),
    listOperationLogs: vi.fn().mockResolvedValue(page([])),
    listApiCallLogs: vi.fn().mockResolvedValue(page([])),
    getApiCallLog: vi.fn().mockResolvedValue(apiCallLog),
    getSystemSettings: vi.fn().mockResolvedValue(systemSettings),
    updateSystemSettings: vi.fn().mockResolvedValue(systemSettings),
    ...overrides,
  }
}

function createMockProviderApi(): ProviderApi {
  return {
    list: vi.fn().mockResolvedValue(page(providers)),
  } as unknown as ProviderApi
}

function createMockModelApi(): ModelApi {
  return {
    list: vi.fn().mockResolvedValue(page(models)),
  } as unknown as ModelApi
}

function renderSystemSettings(adminApi: AdminApi) {
  return render(
    <AdminObservabilitySettingsPanel
      adminApi={adminApi}
      canManageSystemSettings
      canReadAudit={false}
      canReadUsage={false}
      csrfToken="csrf_memory_only"
      isOpen
      modelApi={createMockModelApi()}
      onClose={() => undefined}
      providerApi={createMockProviderApi()}
    />,
  )
}

function deferred<TValue>() {
  let resolve!: (value: TValue) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<TValue>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, reject, resolve }
}

function getBrowserStorage(prefix: string): Storage {
  return Reflect.get(globalThis, `${prefix}Storage`) as Storage
}

describe('AdminObservabilitySettingsPanel', () => {
  afterEach(() => {
    cleanup()
    getBrowserStorage('local').clear()
    getBrowserStorage('session').clear()
    vi.restoreAllMocks()
  })

  it('hides sections and avoids API calls when the matching permission is missing', async () => {
    const adminApi = createMockAdminApi()

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(screen.queryByRole('button', { name: '用量' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '系统设置' })).not.toBeInTheDocument()
    expect(await screen.findByRole('button', { name: '操作日志' })).toBeInTheDocument()
    await waitFor(() => expect(adminApi.listOperationLogs).toHaveBeenCalledTimes(1))
    expect(adminApi.getUsageSummary).not.toHaveBeenCalled()
    expect(adminApi.listUsageRecords).not.toHaveBeenCalled()
    expect(adminApi.getSystemSettings).not.toHaveBeenCalled()
    expect(adminApi.updateSystemSettings).not.toHaveBeenCalled()
  })

  it('shows usage loading and empty states without unbounded list fetching', async () => {
    const tenantTotals = deferred<ApiPage<never>>()
    const summary = deferred<ApiPage<never>>()
    const records = deferred<ApiPage<never>>()
    const adminApi = createMockAdminApi({
      getUsageSummary: vi.fn().mockReturnValueOnce(tenantTotals.promise).mockReturnValueOnce(summary.promise),
      listUsageRecords: vi.fn().mockReturnValue(records.promise),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit={false}
        canReadUsage
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(screen.queryByRole('button', { name: '操作日志' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'API 调用日志' })).not.toBeInTheDocument()
    expect(await screen.findByText('正在加载用量数据...')).toBeInTheDocument()
    tenantTotals.resolve(page([]))
    summary.resolve(page([]))
    records.resolve(page([]))

    expect(await screen.findByText('暂无租户总览')).toBeInTheDocument()
    expect(await screen.findByText('暂无用量汇总')).toBeInTheDocument()
    expect(screen.getByText('暂无用量记录')).toBeInTheDocument()
    expect(adminApi.getUsageSummary).toHaveBeenNthCalledWith(1, expect.objectContaining({ dimension: 'tenant', pageNum: 1, pageSize: 50 }))
    expect(adminApi.getUsageSummary).toHaveBeenNthCalledWith(2, expect.objectContaining({ dimension: 'provider', pageNum: 1, pageSize: 10 }))
    expect(adminApi.listUsageRecords).toHaveBeenCalledWith(expect.objectContaining({ pageNum: 1, pageSize: 10 }))
    expect(adminApi.listOperationLogs).not.toHaveBeenCalled()
    expect(adminApi.listApiCallLogs).not.toHaveBeenCalled()
    expect(adminApi.getApiCallLog).not.toHaveBeenCalled()
  })

  it('shows usage errors and preserves paginated navigation state', async () => {
    const user = userEvent.setup()
    const adminApi = createMockAdminApi({
      getUsageSummary: vi
        .fn()
        .mockResolvedValue(page([tenantUsageTotal], { total: 1, pageNum: 1 }))
        .mockResolvedValue(page([usageSummary], { total: 1, pageNum: 1 }))
        .mockResolvedValue(page([tenantUsageTotal], { total: 1, pageNum: 1 }))
        .mockResolvedValue(page([usageSummary], { total: 1, pageNum: 1 }))
        .mockResolvedValue(page([tenantUsageTotal], { total: 1, pageNum: 1 }))
        .mockResolvedValue(page([usageSummary], { total: 1, pageNum: 1 })),
      listUsageRecords: vi
        .fn()
        .mockResolvedValueOnce(page([usageRecord], { total: 11, pageNum: 1 }))
        .mockResolvedValueOnce(page([{ ...usageRecord, id: 'usage_2' }], { total: 11, pageNum: 2 })),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit={false}
        canReadUsage
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByText('usage_1')).toBeInTheDocument()
    expect(screen.getByText(/REDACTED/)).toBeInTheDocument()
    await user.click(within(screen.getByLabelText('用量记录分页')).getByRole('button', { name: '下一页' }))
    expect(await screen.findByText('usage_2')).toBeInTheDocument()
    expect(adminApi.listUsageRecords).toHaveBeenLastCalledWith(expect.objectContaining({ pageNum: 2, pageSize: 10 }))
  })

  it('renders tenant totals, serializes usage filters, clears filters, and drills summary rows into records', async () => {
    const user = userEvent.setup()
    const getUsageSummary = vi.fn((params: UsageSummaryQuery = {}) => {
      if (params.dimension === 'tenant') {
        return Promise.resolve(page([tenantUsageTotal, eurTenantUsageTotal], { total: 2, pageNum: params.pageNum ?? 1 }))
      }
      return Promise.resolve(
        page(
          [
            {
              ...usageSummary,
              dimension: params.dimension ?? 'provider',
              dimensionId: `${params.dimension ?? 'provider'}_1`,
            },
          ],
          { total: 1, pageNum: params.pageNum ?? 1 },
        ),
      )
    })
    const listUsageRecords = vi.fn().mockResolvedValue(page([usageRecord]))
    const adminApi = createMockAdminApi({
      getUsageSummary,
      listUsageRecords,
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit={false}
        canReadUsage
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByText('租户总览')).toBeInTheDocument()
    expect(screen.getByText('1.25000000 USD')).toBeInTheDocument()
    expect(screen.getByText('0.75000000 EUR')).toBeInTheDocument()
    expect(within(screen.getByLabelText('汇总维度')).getByRole('option', { name: '租户' })).toBeInTheDocument()

    await user.type(screen.getByLabelText('开始时间'), '2026-05-18')
    await user.type(screen.getByLabelText('结束时间'), '2026-05-19')
    await user.type(screen.getByLabelText('Task ID'), 'task_1')
    await user.type(screen.getByLabelText('User ID'), 'user_1')
    await user.type(screen.getByLabelText('Project ID'), 'project_1')
    await user.type(screen.getByLabelText('Provider ID'), 'provider_1')
    await user.type(screen.getByLabelText('Model ID'), 'model_1')
    await user.click(screen.getByRole('button', { name: '应用筛选' }))

    await waitFor(() => expect(listUsageRecords).toHaveBeenCalledTimes(2))
    const appliedUsageFilters = {
      createdAtFrom: '2026-05-18',
      createdAtTo: '2026-05-19',
      taskId: 'task_1',
      userId: 'user_1',
      projectId: 'project_1',
      providerId: 'provider_1',
      modelId: 'model_1',
    }
    expect(getUsageSummary).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({
        ...appliedUsageFilters,
        dimension: 'tenant',
      }),
    )
    expect(getUsageSummary).toHaveBeenNthCalledWith(
      4,
      expect.objectContaining({
        ...appliedUsageFilters,
        dimension: 'provider',
      }),
    )
    expect(listUsageRecords).toHaveBeenLastCalledWith(
      expect.objectContaining({
        ...appliedUsageFilters,
        pageNum: 1,
      }),
    )

    await user.click(screen.getByRole('button', { name: '清空筛选' }))
    await waitFor(() => expect(listUsageRecords).toHaveBeenCalledTimes(3))
    expect(screen.getByLabelText('Task ID')).toHaveValue('')
    expect(listUsageRecords).toHaveBeenLastCalledWith(
      expect.objectContaining({
        taskId: undefined,
        userId: undefined,
        projectId: undefined,
        providerId: undefined,
        modelId: undefined,
      }),
    )

    await user.selectOptions(screen.getByLabelText('汇总维度'), 'user')
    await waitFor(() => expect(getUsageSummary).toHaveBeenCalledWith(expect.objectContaining({ dimension: 'user', pageNum: 1 })))
    await user.click(screen.getByRole('button', { name: /user_1/ }))
    await waitFor(() => expect(listUsageRecords).toHaveBeenCalledTimes(5))
    expect(screen.getByLabelText('User ID')).toHaveValue('user_1')
    expect(listUsageRecords).toHaveBeenLastCalledWith(expect.objectContaining({ userId: 'user_1', pageNum: 1 }))

    await user.selectOptions(screen.getByLabelText('汇总维度'), 'tenant')
    await waitFor(() => expect(getUsageSummary).toHaveBeenCalledWith(expect.objectContaining({ dimension: 'tenant', pageNum: 1 })))
    await user.click(screen.getAllByRole('button', { name: /tenant_1/ })[0])
    expect(JSON.stringify(listUsageRecords.mock.calls)).not.toContain('tenantId')
  })

  it('keeps the latest usage response when an older usage request resolves later', async () => {
    const user = userEvent.setup()
    const firstTenantTotals = deferred<ApiPage<UsageSummary>>()
    const firstSummary = deferred<ApiPage<UsageSummary>>()
    const firstRecords = deferred<ApiPage<typeof usageRecord>>()
    const secondTenantTotals = deferred<ApiPage<UsageSummary>>()
    const secondSummary = deferred<ApiPage<UsageSummary>>()
    const secondRecords = deferred<ApiPage<typeof secondUsageRecord>>()
    const getUsageSummary = vi
      .fn()
      .mockReturnValueOnce(firstTenantTotals.promise)
      .mockReturnValueOnce(firstSummary.promise)
      .mockReturnValueOnce(secondTenantTotals.promise)
      .mockReturnValueOnce(secondSummary.promise)
    const listUsageRecords = vi.fn().mockReturnValueOnce(firstRecords.promise).mockReturnValueOnce(secondRecords.promise)
    const adminApi = createMockAdminApi({
      getUsageSummary,
      listUsageRecords,
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit={false}
        canReadUsage
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByText('正在加载用量数据...')).toBeInTheDocument()
    await user.selectOptions(screen.getByLabelText('汇总维度'), 'user')
    expect(getUsageSummary).toHaveBeenCalledTimes(4)
    expect(listUsageRecords).toHaveBeenCalledTimes(2)

    secondTenantTotals.resolve(page([tenantUsageTotal]))
    secondSummary.resolve(page([{ ...usageSummary, dimension: 'user', dimensionId: 'user_2' }]))
    secondRecords.resolve(page([secondUsageRecord]))

    expect(await screen.findByText('usage_2')).toBeInTheDocument()
    expect(screen.getByText('user_2')).toBeInTheDocument()

    firstTenantTotals.resolve(page([eurTenantUsageTotal]))
    firstSummary.resolve(page([usageSummary]))
    firstRecords.resolve(page([usageRecord]))

    await waitFor(() => expect(screen.getByText('usage_2')).toBeInTheDocument())
    expect(screen.queryByText('usage_1')).not.toBeInTheDocument()
    expect(screen.queryByText('provider_1')).not.toBeInTheDocument()
  })

  it('surfaces usage API failures as error states', async () => {
    const adminApi = createMockAdminApi({
      getUsageSummary: vi.fn().mockRejectedValue(
        new ApiClientError({
          code: 'FORBIDDEN',
          message: 'Forbidden.',
          status: 403,
        }),
      ),
      listUsageRecords: vi.fn().mockResolvedValue(page([])),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit={false}
        canReadUsage
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByText('当前账号没有此管理权限。')).toBeInTheDocument()
  })

  it('loads operation logs, API call logs, and bounded API call detail metadata', async () => {
    const user = userEvent.setup()
    const hugeResponse = { body: 'safe '.repeat(2000), apiKey: '[REDACTED]' }
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([operationLog])),
      listApiCallLogs: vi.fn().mockResolvedValue(page([apiCallLog])),
      getApiCallLog: vi.fn().mockResolvedValue({
        ...apiCallLog,
        redactedResponse: hugeResponse,
      }),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByText('provider.test')).toBeInTheDocument()
    expect(screen.getByText(/REDACTED/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'API 调用日志' }))
    expect(await screen.findByText('api_log_1')).toBeInTheDocument()
    const detailTrigger = screen.getByRole('button', { name: '查看详情' })
    await user.click(detailTrigger)

    const detailDialog = await screen.findByRole('dialog', { name: 'API 调用详情：api_log_1' })
    expect(detailDialog).toHaveAttribute('aria-modal', 'true')
    expect(within(detailDialog).getByText(/内容已截断/)).toBeInTheDocument()
    expect(adminApi.getApiCallLog).toHaveBeenCalledWith('api_log_1')

    await user.click(within(detailDialog).getByRole('button', { name: '关闭弹窗' }))
    expect(screen.queryByRole('dialog', { name: 'API 调用详情：api_log_1' })).not.toBeInTheDocument()
    expect(detailTrigger).toHaveFocus()
  })

  it('explains recent provider failures in Chinese and keeps raw diagnostics inside the detail dialog', async () => {
    const user = userEvent.setup()
    const transportFailure: ApiCallLog = {
      ...apiCallLog,
      id: 'api_log_timeout',
      status: 'FAILURE',
      durationMs: 120005,
      httpStatus: null,
      errorCode: 'PROVIDER_TRANSPORT_ERROR',
      errorMessage: 'context deadline exceeded (Client.Timeout exceeded while awaiting headers)',
    }
    const oversizedResponse: ApiCallLog = {
      ...transportFailure,
      id: 'api_log_oversized',
      durationMs: 102054,
      httpStatus: 200,
      errorCode: 'PROVIDER_RESPONSE_TOO_LARGE',
      errorMessage: 'Provider response could not be read safely.',
    }
    const connectionClosed: ApiCallLog = {
      ...transportFailure,
      id: 'api_log_eof',
      durationMs: 105924,
      errorMessage: 'Post "https://relay.example.com/v1/images/edits": unexpected EOF',
    }
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([])),
      listApiCallLogs: vi.fn().mockResolvedValue(page([transportFailure, connectionClosed, oversizedResponse])),
      getApiCallLog: vi.fn((id) => Promise.resolve(
        id === transportFailure.id ? transportFailure : id === connectionClosed.id ? connectionClosed : oversizedResponse,
      )),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.click(await screen.findByRole('button', { name: 'API 调用日志' }))
    expect(await screen.findAllByText('中转站在超时时间内未返回响应。')).toHaveLength(1)
    expect(screen.getByText('中转站在返回响应前提前断开了连接。')).toBeInTheDocument()
    expect(screen.getByText('中转站已返回成功响应，但响应数据超过平台配置的安全接收上限。')).toBeInTheDocument()
    await user.click(within(screen.getByText('api_log_timeout').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))

    const detailDialog = await screen.findByRole('dialog', { name: 'API 调用详情：api_log_timeout' })
    expect(within(detailDialog).getByText('建议提高中转站超时时间，并降低并发后重试。')).toBeInTheDocument()
    expect(within(detailDialog).getByText(/Client\.Timeout exceeded/)).toBeInTheDocument()

    await user.click(within(detailDialog).getByRole('button', { name: '关闭弹窗' }))
    await user.click(within(screen.getByText('api_log_eof').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))
    const eofDialog = await screen.findByRole('dialog', { name: 'API 调用详情：api_log_eof' })
    expect(within(eofDialog).getByText('建议稍后重试或切换中转站；若持续出现，请中转站检查上游连接和网关超时。')).toBeInTheDocument()
    await user.click(within(eofDialog).getByRole('button', { name: '关闭弹窗' }))
    await user.click(within(screen.getByText('api_log_oversized').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))
    const oversizedDialog = await screen.findByRole('dialog', { name: 'API 调用详情：api_log_oversized' })
    expect(within(oversizedDialog).getByText(/平台会流式接收大图片/)).toBeInTheDocument()
  })

  it('keeps the detail trigger focusable while loading so focus can return when the dialog closes', async () => {
    const user = userEvent.setup()
    const detail = deferred<ApiCallLog>()
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([])),
      listApiCallLogs: vi.fn().mockResolvedValue(page([apiCallLog])),
      getApiCallLog: vi.fn().mockReturnValue(detail.promise),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.click(await screen.findByRole('button', { name: 'API 调用日志' }))
    const detailTrigger = await screen.findByRole('button', { name: '查看详情' })
    await user.click(detailTrigger)

    const detailDialog = await screen.findByRole('dialog', { name: 'API 调用详情：api_log_1' })
    expect(detailTrigger).toBeEnabled()
    await user.click(within(detailDialog).getByRole('button', { name: '关闭弹窗' }))

    expect(detailTrigger).toHaveFocus()
  })

  it('keeps the latest API call detail when older detail responses resolve later', async () => {
    const user = userEvent.setup()
    const firstDetail = deferred<ApiCallLog>()
    const secondDetail = deferred<ApiCallLog>()
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([])),
      listApiCallLogs: vi.fn().mockResolvedValue(page([apiCallLog, secondApiCallLog])),
      getApiCallLog: vi.fn((id) => (id === 'api_log_1' ? firstDetail.promise : secondDetail.promise)),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.click(await screen.findByRole('button', { name: 'API 调用日志' }))
    expect(await screen.findByText('api_log_2')).toBeInTheDocument()
    await user.click(within(screen.getByText('api_log_1').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))
    await user.click(within(screen.getByText('api_log_2').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))
    expect(adminApi.getApiCallLog).toHaveBeenCalledTimes(2)

    secondDetail.resolve(secondApiCallLog)
    expect(await screen.findByText('API 调用详情：api_log_2')).toBeInTheDocument()
    firstDetail.resolve(apiCallLog)

    await waitFor(() => expect(screen.getByText('API 调用详情：api_log_2')).toBeInTheDocument())
    expect(screen.queryByText('API 调用详情：api_log_1')).not.toBeInTheDocument()
  })

  it('keeps the latest API call detail failure when an older detail success resolves later', async () => {
    const user = userEvent.setup()
    const firstDetail = deferred<ApiCallLog>()
    const secondDetail = deferred<ApiCallLog>()
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([])),
      listApiCallLogs: vi.fn().mockResolvedValue(page([apiCallLog, secondApiCallLog])),
      getApiCallLog: vi.fn((id) => (id === 'api_log_1' ? firstDetail.promise : secondDetail.promise)),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.click(await screen.findByRole('button', { name: 'API 调用日志' }))
    expect(await screen.findByText('api_log_2')).toBeInTheDocument()
    await user.click(within(screen.getByText('api_log_1').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))
    await user.click(within(screen.getByText('api_log_2').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' }))

    secondDetail.reject(
      new ApiClientError({
        code: 'NOT_FOUND',
        message: 'Missing.',
        status: 404,
      }),
    )
    expect(await screen.findByText('记录不存在或已不可见。')).toBeInTheDocument()
    expect(within(screen.getByText('api_log_2').closest('tr') as HTMLElement).getByRole('button', { name: '查看详情' })).toBeEnabled()
    firstDetail.resolve(apiCallLog)

    await waitFor(() => expect(screen.getByText('记录不存在或已不可见。')).toBeInTheDocument())
    expect(screen.queryByText('API 调用详情：api_log_1')).not.toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: 'API 调用详情：api_log_2' })).toBeInTheDocument()
  })

  it('ignores in-flight API call detail responses after the panel closes and reopens cleanly', async () => {
    const user = userEvent.setup()
    const detail = deferred<ApiCallLog>()
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([])),
      listApiCallLogs: vi.fn().mockResolvedValue(page([apiCallLog])),
      getApiCallLog: vi.fn().mockReturnValue(detail.promise),
    })
    const { rerender } = render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.click(await screen.findByRole('button', { name: 'API 调用日志' }))
    expect(await screen.findByText('api_log_1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '查看详情' }))

    rerender(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen={false}
        onClose={() => undefined}
      />,
    )
    detail.resolve({
      ...apiCallLog,
      redactedResponse: {
        result: 'stale-after-close',
      },
    })

    rerender(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByText('api_log_1')).toBeInTheDocument()
    expect(screen.queryByText('API 调用详情：api_log_1')).not.toBeInTheDocument()
    expect(screen.queryByText('stale-after-close')).not.toBeInTheDocument()
  })

  it('clears and guards in-flight API call details when the API call logs page changes', async () => {
    const user = userEvent.setup()
    const detail = deferred<ApiCallLog>()
    const adminApi = createMockAdminApi({
      listOperationLogs: vi.fn().mockResolvedValue(page([])),
      listApiCallLogs: vi
        .fn()
        .mockResolvedValueOnce(page([apiCallLog], { total: 11, pageNum: 1 }))
        .mockResolvedValueOnce(page([secondApiCallLog], { total: 11, pageNum: 2 })),
      getApiCallLog: vi.fn().mockReturnValue(detail.promise),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings={false}
        canReadAudit
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.click(await screen.findByRole('button', { name: 'API 调用日志' }))
    expect(await screen.findByText('api_log_1')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '查看详情' }))
    await user.click(within(screen.getByLabelText('API 调用日志分页')).getByRole('button', { name: '下一页' }))
    expect(await screen.findByText('api_log_2')).toBeInTheDocument()
    detail.resolve(apiCallLog)

    await waitFor(() => expect(screen.getByText('api_log_2')).toBeInTheDocument())
    expect(screen.queryByText('API 调用详情：api_log_1')).not.toBeInTheDocument()
  })

  it('renders and PATCHes only uploadPolicy settings while preserving invalid input on backend validation errors', async () => {
    const user = userEvent.setup()
    const validationError = new ApiClientError({
      code: 'VALIDATION_ERROR',
      details: {
        field: 'uploadPolicy.maxWidth',
        max: 8192,
        min: 1,
      },
      message: '最大宽度必须在 1 到 8192 之间。',
      requestId: 'req_settings_upload',
      status: 422,
    })
    const updatedSettings = {
      ...systemSettings,
      uploadPolicy: {
        ...systemSettings.uploadPolicy,
        maxWidth: 4096,
      },
    }
    const adminApi = createMockAdminApi({
      getSystemSettings: vi.fn().mockResolvedValue({
        ...systemSettings,
        defaultProviderId: 'legacy_root_provider',
        defaultModelId: 'legacy_root_model',
        tenantConcurrency: 2,
        storageQuotaBytes: 1000,
        logRetentionDays: 7,
        orphanCleanup: { enabled: true },
        manualCleanup: { enabled: true },
        rawMinioBucket: 'product-originals',
        rawMinioObjectKey: 'tenant_1/raw.png',
        rawMinioUrl: 'http://minio.invalid/product-originals/tenant_1/raw.png',
        allowedMimeTypes: ['image/svg+xml'],
      } as unknown as typeof systemSettings),
      updateSystemSettings: vi.fn().mockRejectedValueOnce(validationError).mockResolvedValueOnce(updatedSettings),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    expect(screen.getByLabelText('最大文件字节数')).toHaveValue(26214400)
    expect(screen.getByLabelText('最大宽度')).toHaveValue(8192)
    expect(screen.getByLabelText('最大高度')).toHaveValue(8192)
    expect(screen.getByLabelText('最大像素数')).toHaveValue(40000000)
    expect(screen.getByLabelText('默认 Provider')).toHaveValue('provider_1')
    expect(screen.getByLabelText('默认模型')).toHaveValue('model_1')
    expect(screen.getByLabelText('租户并发上限')).toHaveValue(4)
    expect(screen.getByLabelText('用户并发上限')).toHaveValue(3)
    expect(screen.getByLabelText('Provider 并发上限')).toHaveValue(2)
    expect(screen.getByLabelText('模型并发上限')).toHaveValue(1)
    expect(screen.getByText(/保存后会对新开始的任务立即生效，无需重启/)).toHaveTextContent('当前全局安全容量为 8 个任务')
    expect(screen.getByLabelText('删除资产保留天数')).toHaveValue(30)
    expect(screen.getByLabelText('最大存储字节数')).toHaveValue(1073741824)
    expect(screen.getByLabelText('已用存储容量')).toHaveValue('1 MB（1,048,576 字节）')
    expect(screen.getByLabelText('已用存储容量')).toHaveAttribute('readonly')
    expect(screen.getByLabelText('操作日志保留天数')).toHaveValue(90)
    expect(screen.getByLabelText('API 调用日志保留天数')).toHaveValue(60)
    expect(screen.getByLabelText('任务事件保留天数')).toBeDisabled()
    expect(screen.getByLabelText('最大宽度')).toHaveAttribute('max', '8192')
    expect(screen.getByLabelText('租户并发上限')).toHaveAttribute('max', '4')
    expect(screen.getAllByText('允许范围：1–8,192 像素')).toHaveLength(2)
    expect(screen.queryByText('defaultProviderId')).not.toBeInTheDocument()
    expect(screen.queryByText('defaultModelId')).not.toBeInTheDocument()
    expect(screen.queryByText('tenantConcurrency')).not.toBeInTheDocument()
    expect(screen.queryByText('storageQuotaBytes')).not.toBeInTheDocument()
    expect(screen.queryByText('logRetentionDays')).not.toBeInTheDocument()
    expect(screen.queryByText('orphanCleanup')).not.toBeInTheDocument()
    expect(screen.queryByText('manualCleanup')).not.toBeInTheDocument()
    expect(screen.queryByText('rawMinioBucket')).not.toBeInTheDocument()
    expect(screen.queryByText('rawMinioObjectKey')).not.toBeInTheDocument()
    expect(screen.queryByText('rawMinioUrl')).not.toBeInTheDocument()
    expect(screen.queryByText('product-originals')).not.toBeInTheDocument()
    expect(screen.queryByText('tenant_1/raw.png')).not.toBeInTheDocument()
    expect(screen.queryByText('http://minio.invalid/product-originals/tenant_1/raw.png')).not.toBeInTheDocument()
    expect(screen.queryByText('allowedMimeTypes')).not.toBeInTheDocument()

    await user.clear(screen.getByLabelText('最大宽度'))
    await user.type(screen.getByLabelText('最大宽度'), '9000')
    await user.click(screen.getByRole('button', { name: '保存上传策略' }))

    const uploadPolicyForm = screen.getByRole('heading', { name: '上传策略' }).closest('form')
    expect(uploadPolicyForm).not.toBeNull()
    expect(await within(uploadPolicyForm!).findByText('最大宽度必须在 1 到 8192 之间。（请求标识：req_settings_upload）')).toBeInTheDocument()
    expect(screen.getByLabelText('最大宽度')).toHaveValue(9000)
    expect(screen.getByLabelText('默认 Provider')).toHaveValue('provider_1')
    expect(screen.getByLabelText('删除资产保留天数')).toHaveValue(30)
    expect(screen.queryByText('上传策略已更新。')).not.toBeInTheDocument()

    await user.clear(screen.getByLabelText('最大宽度'))
    await user.type(screen.getByLabelText('最大宽度'), '4096')
    await user.click(screen.getByRole('button', { name: '保存上传策略' }))

    expect(await screen.findByText('上传策略已更新。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).toHaveBeenLastCalledWith(
      {
        uploadPolicy: {
          maxFileSizeBytes: 26214400,
          maxWidth: 4096,
          maxHeight: 8192,
          maxPixels: 40000000,
        },
      },
      'csrf_memory_only',
    )
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('defaultProviderId')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('defaultModelId')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('tenantConcurrency')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('storageQuotaBytes')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('logRetentionDays')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('orphanCleanup')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('manualCleanup')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('rawMinio')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('allowedMimeTypes')
    expect(getBrowserStorage('local').length).toBe(0)
    expect(getBrowserStorage('session').length).toBe(0)
  })

  it('PATCHes only taskDefaults and enforces paired defaults before submit', async () => {
    const user = userEvent.setup()
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi
        .fn()
        .mockResolvedValueOnce({
          ...systemSettings,
          taskDefaults: {
            defaultProviderId: 'provider_2',
            defaultModelId: 'model_2',
          },
        })
        .mockResolvedValueOnce({
          ...systemSettings,
          taskDefaults: {
            defaultProviderId: null,
            defaultModelId: null,
          },
        }),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    await user.selectOptions(screen.getByLabelText('默认模型'), '')
    await user.click(screen.getByRole('button', { name: '保存任务默认模型' }))

    expect(await screen.findByText('默认 Provider 与默认模型必须成对选择或同时清空。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).not.toHaveBeenCalled()

    await user.selectOptions(screen.getByLabelText('默认 Provider'), 'provider_2')
    await user.selectOptions(screen.getByLabelText('默认模型'), 'model_2')
    await user.click(screen.getByRole('button', { name: '保存任务默认模型' }))

    expect(await screen.findByText('任务默认模型已更新。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).toHaveBeenCalledWith(
      {
        taskDefaults: {
          defaultProviderId: 'provider_2',
          defaultModelId: 'model_2',
        },
      },
      'csrf_memory_only',
    )
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('uploadPolicy')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('usedBytes')

    await user.selectOptions(screen.getByLabelText('默认 Provider'), '')
    await user.click(screen.getByRole('button', { name: '保存任务默认模型' }))

    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      2,
      {
        taskDefaults: {
          defaultProviderId: null,
          defaultModelId: null,
        },
      },
      'csrf_memory_only',
    )
  })

  it('PATCHes only taskConcurrency settings', async () => {
    const user = userEvent.setup()
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi.fn().mockResolvedValue({
        ...systemSettings,
        taskConcurrency: {
          tenantLimit: 3,
          userLimit: 2,
          providerLimit: 2,
          modelLimit: 1,
        },
      }),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    await user.clear(screen.getByLabelText('租户并发上限'))
    await user.type(screen.getByLabelText('租户并发上限'), '3')
    await user.clear(screen.getByLabelText('用户并发上限'))
    await user.type(screen.getByLabelText('用户并发上限'), '2')
    await user.click(screen.getByRole('button', { name: '保存并发限制' }))

    expect(await screen.findByText('并发限制已更新。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).toHaveBeenCalledWith(
      {
        taskConcurrency: {
          tenantLimit: 3,
          userLimit: 2,
          providerLimit: 2,
          modelLimit: 1,
        },
      },
      'csrf_memory_only',
    )
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('globalLimit')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('uploadPolicy')
  })

  it('keeps other unsaved groups intact and only locks the group being saved', async () => {
    const user = userEvent.setup()
    const save = deferred<typeof systemSettings>()
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi.fn().mockReturnValue(save.promise),
    })

    renderSystemSettings(adminApi)
    await screen.findByLabelText('任务事件保留天数')

    await user.clear(screen.getByLabelText('最大宽度'))
    await user.type(screen.getByLabelText('最大宽度'), '4096')
    await user.clear(screen.getByLabelText('Provider 并发上限'))
    await user.type(screen.getByLabelText('Provider 并发上限'), '1')

    const uploadSaveButton = screen.getByRole('button', { name: '保存上传策略' })
    const concurrencySaveButton = screen.getByRole('button', { name: '保存并发限制' })
    expect(uploadSaveButton).toBeEnabled()
    expect(concurrencySaveButton).toBeEnabled()

    await user.click(uploadSaveButton)
    expect(screen.getByRole('button', { name: '正在保存...' })).toBeDisabled()
    expect(concurrencySaveButton).toBeEnabled()

    save.resolve({
      ...systemSettings,
      uploadPolicy: {
        ...systemSettings.uploadPolicy,
        maxWidth: 4096,
      },
    })

    expect(await screen.findByText('上传策略已更新。')).toBeInTheDocument()
    expect(screen.getByLabelText('Provider 并发上限')).toHaveValue(1)
    expect(screen.getByRole('button', { name: '保存并发限制' })).toBeEnabled()
  })

  it('PATCHes storageRetention with a positive integer and null clear', async () => {
    const user = userEvent.setup()
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi
        .fn()
        .mockResolvedValueOnce({
          ...systemSettings,
          storageRetention: {
            deletedAssetRetentionDays: 45,
          },
        })
        .mockResolvedValueOnce({
          ...systemSettings,
          storageRetention: {
            deletedAssetRetentionDays: null,
          },
        }),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    await user.clear(screen.getByLabelText('删除资产保留天数'))
    await user.type(screen.getByLabelText('删除资产保留天数'), '45')
    await user.click(screen.getByRole('button', { name: '保存删除资产保留期' }))
    expect(await screen.findByText('删除资产保留期已更新。')).toBeInTheDocument()

    await user.click(screen.getByLabelText('启用软删除资产自动清理'))
    await user.click(screen.getByRole('button', { name: '保存删除资产保留期' }))

    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      1,
      {
        storageRetention: {
          deletedAssetRetentionDays: 45,
        },
      },
      'csrf_memory_only',
    )
    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      2,
      {
        storageRetention: {
          deletedAssetRetentionDays: null,
        },
      },
      'csrf_memory_only',
    )
  })

  it('PATCHes storageQuota maxBytes only and keeps usedBytes read-only', async () => {
    const user = userEvent.setup()
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi
        .fn()
        .mockResolvedValueOnce({
          ...systemSettings,
          storageQuota: {
            maxBytes: 2147483648,
            usedBytes: 1048576,
          },
        })
        .mockResolvedValueOnce({
          ...systemSettings,
          storageQuota: {
            maxBytes: null,
            usedBytes: 1048576,
          },
        }),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    expect(screen.getByLabelText('已用存储容量')).toHaveAttribute('readonly')
    await user.clear(screen.getByLabelText('最大存储字节数'))
    await user.type(screen.getByLabelText('最大存储字节数'), '2147483648')
    await user.click(screen.getByRole('button', { name: '保存存储配额' }))

    expect(await screen.findByText('存储配额已更新。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      1,
      {
        storageQuota: {
          maxBytes: 2147483648,
        },
      },
      'csrf_memory_only',
    )

    await user.click(screen.getByLabelText('启用租户存储配额'))
    await user.click(screen.getByRole('button', { name: '保存存储配额' }))

    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      2,
      {
        storageQuota: {
          maxBytes: null,
        },
      },
      'csrf_memory_only',
    )
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('usedBytes')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('storageQuotaBytes')
  })

  it('PATCHes only logRetention settings and clears nullable retention values', async () => {
    const user = userEvent.setup()
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi
        .fn()
        .mockResolvedValueOnce({
          ...systemSettings,
          logRetention: {
            operationLogRetentionDays: 120,
            apiCallLogRetentionDays: 30,
            taskEventRetentionDays: 14,
          },
        })
        .mockResolvedValueOnce({
          ...systemSettings,
          logRetention: {
            operationLogRetentionDays: null,
            apiCallLogRetentionDays: null,
            taskEventRetentionDays: null,
          },
        }),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    await user.clear(screen.getByLabelText('操作日志保留天数'))
    await user.type(screen.getByLabelText('操作日志保留天数'), '120')
    await user.clear(screen.getByLabelText('API 调用日志保留天数'))
    await user.type(screen.getByLabelText('API 调用日志保留天数'), '30')
    await user.click(screen.getByLabelText('自动清理任务事件'))
    await user.clear(screen.getByLabelText('任务事件保留天数'))
    await user.type(screen.getByLabelText('任务事件保留天数'), '14')
    await user.click(screen.getByRole('button', { name: '保存日志保留期' }))

    expect(await screen.findByText('日志保留期已更新。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      1,
      {
        logRetention: {
          operationLogRetentionDays: 120,
          apiCallLogRetentionDays: 30,
          taskEventRetentionDays: 14,
        },
      },
      'csrf_memory_only',
    )

    await user.click(screen.getByLabelText('自动清理操作日志'))
    await user.click(screen.getByLabelText('自动清理 API 调用日志'))
    await user.click(screen.getByLabelText('自动清理任务事件'))
    await user.click(screen.getByRole('button', { name: '保存日志保留期' }))

    expect(adminApi.updateSystemSettings).toHaveBeenNthCalledWith(
      2,
      {
        logRetention: {
          operationLogRetentionDays: null,
          apiCallLogRetentionDays: null,
          taskEventRetentionDays: null,
        },
      },
      'csrf_memory_only',
    )
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('uploadPolicy')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('storageRetention')
    expect(JSON.stringify(vi.mocked(adminApi.updateSystemSettings).mock.calls)).not.toContain('storageQuota')
  })

  it('rejects invalid logRetention input without requesting and preserves drafts after backend errors', async () => {
    const user = userEvent.setup()
    const validationError = new ApiClientError({
      code: 'VALIDATION_ERROR',
      details: { field: 'logRetention.operationLogRetentionDays', max: 3650, min: 1 },
      message: '操作日志保留天数必须在 1 到 3650 之间。',
      status: 422,
    })
    const adminApi = createMockAdminApi({
      updateSystemSettings: vi.fn().mockRejectedValue(validationError),
    })

    renderSystemSettings(adminApi)

    await screen.findByLabelText('任务事件保留天数')
    await user.clear(screen.getByLabelText('操作日志保留天数'))
    await user.type(screen.getByLabelText('操作日志保留天数'), '1.5')
    await user.click(screen.getByRole('button', { name: '保存日志保留期' }))

    expect(screen.getByLabelText('操作日志保留天数')).toHaveValue(1.5)
    expect(adminApi.updateSystemSettings).not.toHaveBeenCalled()

    await user.clear(screen.getByLabelText('操作日志保留天数'))
    await user.type(screen.getByLabelText('操作日志保留天数'), '120')
    await user.click(screen.getByRole('button', { name: '保存日志保留期' }))

    expect(await screen.findByText('操作日志保留天数必须在 1 到 3650 之间。')).toBeInTheDocument()
    expect(screen.getByLabelText('操作日志保留天数')).toHaveValue(120)
    expect(screen.getByLabelText('API 调用日志保留天数')).toHaveValue(60)
    expect(screen.getByLabelText('任务事件保留天数')).toBeDisabled()
    expect(adminApi.updateSystemSettings).toHaveBeenCalledTimes(1)
  })
})
