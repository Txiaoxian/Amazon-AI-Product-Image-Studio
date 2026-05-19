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
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface AdminApi {
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
    status: params.status,
    requestId: params.requestId,
  }
}

export const adminApi = createAdminApi()
