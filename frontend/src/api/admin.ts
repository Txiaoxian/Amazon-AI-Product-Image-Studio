import type { QueryParamRecord } from '../types/api'
import type {
  ApiCallLog,
  ApiCallLogPage,
  ApiCallLogQuery,
  OperationLogPage,
  OperationLogQuery,
  SystemSettings,
  UpdateSystemSettingsRequest,
  UsageRecordPage,
  UsageSummaryPage,
  UsageSummaryQuery,
  AdminUsageQuery,
} from '../types/admin'
import type {
  AnalyticsDataset,
  AnalyticsExport,
  AnalyticsOverviewResponse,
  AnalyticsQuery,
  AnalyticsRequestsResponse,
  AnalyticsTaskPage,
  AnalyticsUsageResponse,
  AnalyticsUserDetailResponse,
  AnalyticsUserPage,
} from '../types/analytics'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface AdminApi {
  getAnalyticsOverview(params?: AnalyticsQuery): Promise<AnalyticsOverviewResponse>
  getAnalyticsUsage(params?: AnalyticsQuery): Promise<AnalyticsUsageResponse>
  getAnalyticsUsers(params?: AnalyticsQuery): Promise<AnalyticsUserPage>
  getAnalyticsUser(id: string, params?: AnalyticsQuery): Promise<AnalyticsUserDetailResponse>
  getAnalyticsTasks(params?: AnalyticsQuery): Promise<AnalyticsTaskPage>
  getAnalyticsRequests(params?: AnalyticsQuery): Promise<AnalyticsRequestsResponse>
  exportAnalytics(dataset: AnalyticsDataset, params?: AnalyticsQuery): Promise<AnalyticsExport>
  getUsageSummary(params?: UsageSummaryQuery): Promise<UsageSummaryPage>
  listUsageRecords(params?: AdminUsageQuery): Promise<UsageRecordPage>
  listOperationLogs(params?: OperationLogQuery): Promise<OperationLogPage>
  listApiCallLogs(params?: ApiCallLogQuery): Promise<ApiCallLogPage>
  getApiCallLog(id: ApiCallLog['id']): Promise<ApiCallLog>
  getSystemSettings(): Promise<SystemSettings>
  updateSystemSettings(request: UpdateSystemSettingsRequest, csrfToken: string): Promise<SystemSettings>
}

export function createAdminApi(client: ApiClient = apiClient): AdminApi {
  return {
    getAnalyticsOverview: (params = {}) =>
      client.get<AnalyticsOverviewResponse>('/admin/analytics/overview', { query: analyticsQuery(params) }),
    getAnalyticsUsage: (params = {}) =>
      client.get<AnalyticsUsageResponse>('/admin/analytics/usage', { query: analyticsQuery(params) }),
    getAnalyticsUsers: (params = {}) =>
      client.get<AnalyticsUserPage>('/admin/analytics/users', { query: analyticsQuery(params) }),
    getAnalyticsUser: (id, params = {}) =>
      client.get<AnalyticsUserDetailResponse>(`/admin/analytics/users/${encodeURIComponent(id)}`, {
        query: analyticsQuery(params),
      }),
    getAnalyticsTasks: (params = {}) =>
      client.get<AnalyticsTaskPage>('/admin/analytics/tasks', { query: analyticsQuery(params) }),
    getAnalyticsRequests: (params = {}) =>
      client.get<AnalyticsRequestsResponse>('/admin/analytics/requests', { query: analyticsQuery(params) }),
    exportAnalytics: async (dataset, params = {}) => {
      const response = await client.raw(`/admin/analytics/exports/${dataset}`, { query: analyticsQuery(params) })
      return {
        blob: await response.blob(),
        filename: exportFilename(response.headers.get('Content-Disposition'), dataset),
      }
    },
    getUsageSummary: (params = {}) =>
      client.get<UsageSummaryPage>('/admin/usage/summary', {
        query: usageSummaryQuery(params),
      }),
    listUsageRecords: (params = {}) =>
      client.get<UsageRecordPage>('/admin/usage/records', {
        query: usageQuery(params),
      }),
    listOperationLogs: (params = {}) =>
      client.get<OperationLogPage>('/admin/operation-logs', {
        query: operationLogQuery(params),
      }),
    listApiCallLogs: (params = {}) =>
      client.get<ApiCallLogPage>('/admin/api-call-logs', {
        query: apiCallLogQuery(params),
      }),
    getApiCallLog: (id) => client.get<ApiCallLog>(`/admin/api-call-logs/${encodeURIComponent(id)}`),
    getSystemSettings: () => client.get<SystemSettings>('/admin/system-settings'),
    updateSystemSettings: (request, csrfToken) =>
      client.patch<SystemSettings>('/admin/system-settings', request, {
        headers: csrfHeaders(csrfToken),
      }),
  }
}

function analyticsQuery(params: AnalyticsQuery): QueryParamRecord {
  return {
    from: params.from,
    to: params.to,
    granularity: params.granularity,
    compare: params.compare,
    userId: params.userId,
    projectId: params.projectId,
    providerId: params.providerId,
    modelId: params.modelId,
    status: params.status,
    imageType: params.imageType,
    groupBy: params.groupBy,
    search: params.search,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

function exportFilename(contentDisposition: string | null, dataset: AnalyticsDataset): string {
  const encoded = contentDisposition?.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  if (encoded) {
    try {
      return decodeURIComponent(encoded)
    } catch {
      // 文件名解析失败时使用稳定的中文兜底名称。
    }
  }
  const labels: Record<AnalyticsDataset, string> = {
    usage: '用量与费用.csv',
    users: '用户与活跃.csv',
    tasks: '生图任务.csv',
    requests: '模型调用.csv',
  }
  return labels[dataset]
}

function pageQuery(params: UsageSummaryQuery | AdminUsageQuery | OperationLogQuery | ApiCallLogQuery): QueryParamRecord {
  return {
    pageNum: params.pageNum,
    pageSize: params.pageSize,
    sortBy: params.sortBy,
    sortOrder: params.sortOrder,
    createdAtFrom: params.createdAtFrom,
    createdAtTo: params.createdAtTo,
  }
}

function usageQuery(params: AdminUsageQuery): QueryParamRecord {
  return {
    ...pageQuery(params),
    taskId: params.taskId,
    userId: params.userId,
    projectId: params.projectId,
    providerId: params.providerId,
    modelId: params.modelId,
  }
}

function usageSummaryQuery(params: UsageSummaryQuery): QueryParamRecord {
  return {
    ...usageQuery(params),
    dimension: params.dimension,
  }
}

function operationLogQuery(params: OperationLogQuery): QueryParamRecord {
  return {
    ...pageQuery(params),
    actorUserId: params.actorUserId,
    action: params.action,
    resourceType: params.resourceType,
    resourceId: params.resourceId,
  }
}

function apiCallLogQuery(params: ApiCallLogQuery): QueryParamRecord {
  return {
    ...usageQuery(params),
    imageType: params.imageType,
    status: params.status,
    requestId: params.requestId,
  }
}

export const adminApi = createAdminApi()
