import { describe, expect, it, vi } from 'vitest'
import { createAdminApi } from '../api/admin'
import { createApiClient } from '../api/client'

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_admin_api',
    }),
    { status },
  )
}

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
}

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
  rawUsage: { safe: 'ok' },
  createdAt: '2026-05-18T10:00:00Z',
}

const operationLog = {
  id: 'operation_1',
  tenantId: 'tenant_1',
  actorUserId: 'user_1',
  action: 'provider.test',
  resourceType: 'provider',
  resourceId: 'provider_1',
  ip: '127.0.0.1',
  userAgent: 'test-agent',
  metadata: { safe: 'ok' },
  createdAt: '2026-05-18T10:00:00Z',
}

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
  redactedRequest: { prompt: 'safe' },
  redactedResponse: { result: 'safe' },
  createdAt: '2026-05-18T10:00:00Z',
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
    deletedAssetRetentionDays: null,
  },
  storageQuota: {
    maxBytes: null,
    usedBytes: 2048,
  },
}

function page(records: unknown[], total = records.length) {
  return {
    records,
    total,
    pageNum: 1,
    pageSize: 20,
  }
}

describe('admin observability and settings API wrappers', () => {
  it('serializes usage and audit list queries through the authenticated API client', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(page([usageSummary])))
      .mockResolvedValueOnce(jsonResponse(page([usageRecord])))
      .mockResolvedValueOnce(jsonResponse(page([operationLog])))
      .mockResolvedValueOnce(jsonResponse(page([apiCallLog])))
      .mockResolvedValueOnce(jsonResponse(apiCallLog))
    const adminApi = createAdminApi(createApiClient({ fetchImpl }))

    await expect(
      adminApi.getUsageSummary({
        dimension: 'provider',
        pageNum: 1,
        pageSize: 20,
        sortBy: 'createdAt',
        sortOrder: 'desc',
        createdAtFrom: '2026-05-18',
        createdAtTo: '2026-05-19',
        providerId: 'provider_1',
      }),
    ).resolves.toMatchObject({ records: [usageSummary], total: 1 })
    await expect(adminApi.listUsageRecords({ pageNum: 2, pageSize: 10, taskId: 'task_1' })).resolves.toMatchObject({
      records: [usageRecord],
    })
    await expect(
      adminApi.listOperationLogs({
        pageNum: 1,
        pageSize: 20,
        actorUserId: 'user_1',
        action: 'provider.test',
        resourceType: 'provider',
        resourceId: 'provider_1',
      }),
    ).resolves.toMatchObject({ records: [operationLog] })
    await expect(
      adminApi.listApiCallLogs({
        pageNum: 1,
        pageSize: 20,
        projectId: 'project_1',
        userId: 'user_1',
        status: 'SUCCESS',
        requestId: 'provider_request_1',
      }),
    ).resolves.toMatchObject({ records: [apiCallLog] })
    await expect(adminApi.getApiCallLog('api_log_1/with slash')).resolves.toEqual(apiCallLog)

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/admin/usage/summary?pageNum=1&pageSize=20&sortBy=createdAt&sortOrder=desc&createdAtFrom=2026-05-18&createdAtTo=2026-05-19&providerId=provider_1&dimension=provider',
      '/api/v1/admin/usage/records?pageNum=2&pageSize=10&taskId=task_1',
      '/api/v1/admin/operation-logs?pageNum=1&pageSize=20&actorUserId=user_1&action=provider.test&resourceType=provider&resourceId=provider_1',
      '/api/v1/admin/api-call-logs?pageNum=1&pageSize=20&userId=user_1&projectId=project_1&status=SUCCESS&requestId=provider_request_1',
      '/api/v1/admin/api-call-logs/api_log_1%2Fwith%20slash',
    ])
    for (const [, init] of fetchImpl.mock.calls) {
      expect(init).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    }
  })

  it('uses GET and CSRF-protected PATCH for one system settings group at a time', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(systemSettings))
      .mockResolvedValueOnce(
        jsonResponse({
          ...systemSettings,
          uploadPolicy: {
            ...systemSettings.uploadPolicy,
            maxWidth: 4096,
          },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          ...systemSettings,
          storageQuota: {
            maxBytes: null,
            usedBytes: 2048,
          },
        }),
      )
    const adminApi = createAdminApi(createApiClient({ fetchImpl }))

    await expect(adminApi.getSystemSettings()).resolves.toEqual(systemSettings)
    await adminApi.updateSystemSettings(
      {
        uploadPolicy: {
          maxFileSizeBytes: 10485760,
          maxWidth: 4096,
          maxHeight: 4096,
          maxPixels: 16000000,
        },
      },
      'csrf_memory_only',
    )
    await adminApi.updateSystemSettings(
      {
        storageQuota: {
          maxBytes: null,
        },
      },
      'csrf_memory_only',
    )

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/admin/system-settings',
      '/api/v1/admin/system-settings',
      '/api/v1/admin/system-settings',
    ])
    expect(fetchImpl.mock.calls[0][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect(fetchImpl.mock.calls[1][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'PATCH' }))
    expect(fetchImpl.mock.calls[2][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'PATCH' }))
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[2][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toEqual({
      uploadPolicy: {
        maxFileSizeBytes: 10485760,
        maxWidth: 4096,
        maxHeight: 4096,
        maxPixels: 16000000,
      },
    })
    expect(JSON.parse(fetchImpl.mock.calls[2][1]?.body as string)).toEqual({
      storageQuota: {
        maxBytes: null,
      },
    })
    expect(fetchImpl.mock.calls[1][1]?.body).not.toContain('defaultProviderId')
    expect(fetchImpl.mock.calls[1][1]?.body).not.toContain('defaultModelId')
    expect(fetchImpl.mock.calls[1][1]?.body).not.toContain('tenantConcurrency')
    expect(fetchImpl.mock.calls[1][1]?.body).not.toContain('storageQuotaBytes')
    expect(fetchImpl.mock.calls[1][1]?.body).not.toContain('logRetentionDays')
    expect(fetchImpl.mock.calls[2][1]?.body).not.toContain('usedBytes')
  })
})
