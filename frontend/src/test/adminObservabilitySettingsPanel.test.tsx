import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminObservabilitySettingsPanel } from '../components/admin/AdminObservabilitySettingsPanel'
import { ApiClientError } from '../api/client'
import type { AdminApi } from '../api/admin'
import type { ApiCallLog } from '../types/admin'
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
    const summary = deferred<ApiPage<never>>()
    const records = deferred<ApiPage<never>>()
    const adminApi = createMockAdminApi({
      getUsageSummary: vi.fn().mockReturnValue(summary.promise),
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
    summary.resolve(page([]))
    records.resolve(page([]))

    expect(await screen.findByText('暂无用量汇总')).toBeInTheDocument()
    expect(screen.getByText('暂无用量记录')).toBeInTheDocument()
    expect(adminApi.getUsageSummary).toHaveBeenCalledWith(expect.objectContaining({ pageNum: 1, pageSize: 10 }))
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
        .mockResolvedValue(page([usageSummary], { total: 1, pageNum: 1 }))
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
    await user.click(within(screen.getByLabelText('用量记录分页')).getByRole('button', { name: '下一页' }))
    expect(await screen.findByText('usage_2')).toBeInTheDocument()
    expect(adminApi.listUsageRecords).toHaveBeenLastCalledWith(expect.objectContaining({ pageNum: 2, pageSize: 10 }))
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
      uploadPolicy: {
        ...systemSettings.uploadPolicy,
        maxWidth: 4096,
      },
    }
    const adminApi = createMockAdminApi({
      getSystemSettings: vi.fn().mockResolvedValue({
        ...systemSettings,
        defaultProviderId: 'provider_1',
        defaultModelId: 'model_1',
        tenantConcurrency: 2,
        storageQuotaBytes: 1000,
        logRetentionDays: 7,
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
    expect(screen.queryByText('defaultProviderId')).not.toBeInTheDocument()
    expect(screen.queryByText('defaultModelId')).not.toBeInTheDocument()
    expect(screen.queryByText('tenantConcurrency')).not.toBeInTheDocument()
    expect(screen.queryByText('storageQuotaBytes')).not.toBeInTheDocument()
    expect(screen.queryByText('logRetentionDays')).not.toBeInTheDocument()

    await user.clear(screen.getByLabelText('最大宽度'))
    await user.type(screen.getByLabelText('最大宽度'), '9000')
    await user.click(screen.getByRole('button', { name: '保存上传策略' }))

    expect(await screen.findByText('表单内容未通过校验：Invalid request.')).toBeInTheDocument()
    expect(screen.getByLabelText('最大宽度')).toHaveValue(9000)
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
    expect(getBrowserStorage('local').length).toBe(0)
    expect(getBrowserStorage('session').length).toBe(0)
  })
})
