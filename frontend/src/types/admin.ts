import type { ApiPage } from './api'
import type {
  ISODateTimeString,
  ModelId,
  ProjectId,
  ProviderId,
  TaskId,
  TenantId,
  UserId,
} from './platform'

export type AdminSortOrder = 'asc' | 'desc'
export type UsageSummaryDimension = 'user' | 'project' | 'provider' | 'model'
export type ApiCallLogStatus = 'SUCCESS' | 'FAILURE'
export type RedactedMetadata = unknown

export interface AdminPageQuery {
  pageNum?: number
  pageSize?: number
  sortBy?: 'createdAt'
  sortOrder?: AdminSortOrder
  createdAtFrom?: string
  createdAtTo?: string
}

export interface AdminUsageQuery extends AdminPageQuery {
  taskId?: TaskId | string
  userId?: UserId | string
  projectId?: ProjectId | string
  providerId?: ProviderId | string
  modelId?: ModelId | string
}

export interface UsageSummaryQuery extends AdminUsageQuery {
  dimension?: UsageSummaryDimension
}

export interface OperationLogQuery extends AdminPageQuery {
  actorUserId?: UserId | string
  action?: string
  resourceType?: string
  resourceId?: string
}

export interface ApiCallLogQuery extends AdminUsageQuery {
  status?: ApiCallLogStatus
  requestId?: string
}

export interface UsageSummary {
  dimension: UsageSummaryDimension
  dimensionId: string
  currency: string
  recordCount: number
  inputTokens: number
  outputTokens: number
  imageCount: number
  estimatedCost: string
  latestCreatedAt: ISODateTimeString | ''
}

export interface UsageRecord {
  id: string
  tenantId: TenantId
  taskId: TaskId
  userId: UserId
  projectId: ProjectId
  providerId: ProviderId
  modelId: ModelId
  inputTokens: number
  outputTokens: number
  imageCount: number
  estimatedCost: string
  currency: string
  rawUsage: RedactedMetadata
  createdAt: ISODateTimeString
}

export interface OperationLog {
  id: string
  tenantId: TenantId
  actorUserId: UserId | null
  action: string
  resourceType: string
  resourceId: string
  ip: string
  userAgent: string
  metadata: RedactedMetadata
  createdAt: ISODateTimeString
}

export interface ApiCallLog {
  id: string
  tenantId: TenantId
  taskId: TaskId
  providerId: ProviderId
  modelId: ModelId
  status: ApiCallLogStatus
  durationMs: number
  requestId: string
  httpStatus: number | null
  errorCode: string
  errorMessage: string
  redactedRequest: RedactedMetadata
  redactedResponse: RedactedMetadata
  createdAt: ISODateTimeString
}

export interface UploadPolicySettings {
  maxFileSizeBytes: number
  maxWidth: number
  maxHeight: number
  maxPixels: number
}

export interface TaskDefaultsSettings {
  defaultProviderId: ProviderId | null
  defaultModelId: ModelId | null
}

export interface TaskConcurrencySettings {
  tenantLimit: number
  userLimit: number
  providerLimit: number
  modelLimit: number
}

export interface StorageRetentionSettings {
  deletedAssetRetentionDays: number | null
}

export interface StorageQuotaSettings {
  maxBytes: number | null
  usedBytes: number
}

export interface SystemSettings {
  uploadPolicy: UploadPolicySettings
  taskDefaults: TaskDefaultsSettings
  taskConcurrency: TaskConcurrencySettings
  storageRetention: StorageRetentionSettings
  storageQuota: StorageQuotaSettings
}

export interface UpdateSystemSettingsRequest {
  uploadPolicy?: Partial<UploadPolicySettings>
  taskDefaults?: TaskDefaultsSettings
  taskConcurrency?: Partial<TaskConcurrencySettings>
  storageRetention?: StorageRetentionSettings
  storageQuota?: Pick<StorageQuotaSettings, 'maxBytes'>
}

export type UsageSummaryPage = ApiPage<UsageSummary>
export type UsageRecordPage = ApiPage<UsageRecord>
export type OperationLogPage = ApiPage<OperationLog>
export type ApiCallLogPage = ApiPage<ApiCallLog>
