import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminObservabilitySettingsPanel } from '../components/admin/AdminObservabilitySettingsPanel'
import { ApiClientError } from '../api/client'
import type { AdminApi } from '../api/admin'
import type { ApiCallLog, UsageSummaryQuery } from '../types/admin'
import type { ApiPage } from '../types/api'

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
}

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

    expect(await screen.findByText('暂无 tenant totals')).toBeInTheDocument()
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

    expect(await screen.findByText('Tenant totals')).toBeInTheDocument()
    expect(screen.getByText('1.25000000 USD')).toBeInTheDocument()
    expect(screen.getByText('0.75000000 EUR')).toBeInTheDocument()
    expect(within(screen.getByLabelText('汇总维度')).getByRole('option', { name: '租户' })).toBeInTheDocument()

    await user.type(screen.getByLabelText('开始时间'), '2026-05-18T00:00')
    await user.type(screen.getByLabelText('结束时间'), '2026-05-19T23:59')
    await user.type(screen.getByLabelText('Task ID'), 'task_1')
    await user.type(screen.getByLabelText('User ID'), 'user_1')
    await user.type(screen.getByLabelText('Project ID'), 'project_1')
    await user.type(screen.getByLabelText('Provider ID'), 'provider_1')
    await user.type(screen.getByLabelText('Model ID'), 'model_1')
    await user.click(screen.getByRole('button', { name: '应用筛选' }))

    await waitFor(() => expect(listUsageRecords).toHaveBeenCalledTimes(2))
    const appliedUsageFilters = {
      createdAtFrom: '2026-05-18T00:00',
      createdAtTo: '2026-05-19T23:59',
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
    await user.click(screen.getByRole('button', { name: '查看详情' }))

    expect(await screen.findByText('API 调用详情：api_log_1')).toBeInTheDocument()
    expect(screen.getByText(/内容已截断/)).toBeInTheDocument()
    expect(adminApi.getApiCallLog).toHaveBeenCalledWith('api_log_1')
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
    expect(screen.queryByText('API 调用详情：api_log_2')).not.toBeInTheDocument()
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
      message: 'Invalid request.',
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
        logRetention: { days: 7 },
        logRetentionDays: 7,
        orphanCleanup: { enabled: true },
        manualCleanup: { enabled: true },
        allowedMimeTypes: ['image/svg+xml'],
      } as unknown as typeof systemSettings),
      updateSystemSettings: vi.fn().mockRejectedValueOnce(validationError).mockResolvedValueOnce(updatedSettings),
    })

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings
        canReadAudit={false}
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByLabelText('最大文件字节数')).toHaveValue(26214400)
    expect(screen.getByLabelText('最大宽度')).toHaveValue(8192)
    expect(screen.getByLabelText('最大高度')).toHaveValue(8192)
    expect(screen.getByLabelText('最大像素数')).toHaveValue(40000000)
    expect(screen.getByLabelText('默认 Provider ID')).toHaveValue('provider_1')
    expect(screen.getByLabelText('默认模型 ID')).toHaveValue('model_1')
    expect(screen.getByLabelText('租户并发上限')).toHaveValue(4)
    expect(screen.getByLabelText('用户并发上限')).toHaveValue(3)
    expect(screen.getByLabelText('Provider 并发上限')).toHaveValue(2)
    expect(screen.getByLabelText('模型并发上限')).toHaveValue(1)
    expect(screen.getByLabelText('删除资产保留天数')).toHaveValue(30)
    expect(screen.getByLabelText('最大存储字节数')).toHaveValue(1073741824)
    expect(screen.getByLabelText('已用存储字节数')).toHaveValue('1,048,576')
    expect(screen.getByLabelText('已用存储字节数')).toHaveAttribute('readonly')
    expect(screen.queryByText('defaultProviderId')).not.toBeInTheDocument()
    expect(screen.queryByText('defaultModelId')).not.toBeInTheDocument()
    expect(screen.queryByText('tenantConcurrency')).not.toBeInTheDocument()
    expect(screen.queryByText('storageQuotaBytes')).not.toBeInTheDocument()
    expect(screen.queryByText('logRetentionDays')).not.toBeInTheDocument()
    expect(screen.queryByText('orphanCleanup')).not.toBeInTheDocument()
    expect(screen.queryByText('manualCleanup')).not.toBeInTheDocument()
    expect(screen.queryByText('allowedMimeTypes')).not.toBeInTheDocument()

    await user.clear(screen.getByLabelText('最大宽度'))
    await user.type(screen.getByLabelText('最大宽度'), '9000')
    await user.click(screen.getByRole('button', { name: '保存上传策略' }))

    expect(await screen.findByText('表单内容未通过校验：Invalid request.')).toBeInTheDocument()
    expect(screen.getByLabelText('最大宽度')).toHaveValue(9000)
    expect(screen.getByLabelText('默认 Provider ID')).toHaveValue('provider_1')
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

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings
        canReadAudit={false}
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.clear(await screen.findByLabelText('默认模型 ID'))
    await user.click(screen.getByRole('button', { name: '保存任务默认模型' }))

    expect(await screen.findByText('默认 Provider ID 与默认模型 ID 必须成对填写或同时清空。')).toBeInTheDocument()
    expect(adminApi.updateSystemSettings).not.toHaveBeenCalled()

    await user.type(screen.getByLabelText('默认模型 ID'), 'model_2')
    await user.clear(screen.getByLabelText('默认 Provider ID'))
    await user.type(screen.getByLabelText('默认 Provider ID'), 'provider_2')
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

    await user.clear(screen.getByLabelText('默认 Provider ID'))
    await user.clear(screen.getByLabelText('默认模型 ID'))
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

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings
        canReadAudit={false}
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.clear(await screen.findByLabelText('租户并发上限'))
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

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings
        canReadAudit={false}
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    await user.clear(await screen.findByLabelText('删除资产保留天数'))
    await user.type(screen.getByLabelText('删除资产保留天数'), '45')
    await user.click(screen.getByRole('button', { name: '保存删除资产保留期' }))
    expect(await screen.findByText('删除资产保留期已更新。')).toBeInTheDocument()

    await user.clear(screen.getByLabelText('删除资产保留天数'))
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

    render(
      <AdminObservabilitySettingsPanel
        adminApi={adminApi}
        canManageSystemSettings
        canReadAudit={false}
        canReadUsage={false}
        csrfToken="csrf_memory_only"
        isOpen
        onClose={() => undefined}
      />,
    )

    expect(await screen.findByLabelText('已用存储字节数')).toHaveAttribute('readonly')
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

    await user.clear(screen.getByLabelText('最大存储字节数'))
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
})
