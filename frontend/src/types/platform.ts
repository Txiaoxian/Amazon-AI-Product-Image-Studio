declare const platformIdBrand: unique symbol

export type Brand<TValue, TBrand extends string> = TValue & {
  readonly [platformIdBrand]: TBrand
}

export type TenantId = Brand<string, 'TenantId'>
export type UserId = Brand<string, 'UserId'>
export type RoleId = Brand<string, 'RoleId'>
export type ProjectId = Brand<string, 'ProjectId'>
export type AssetId = Brand<string, 'AssetId'>
export type TaskId = Brand<string, 'TaskId'>
export type TaskEventId = Brand<string, 'TaskEventId'>
export type ProviderId = Brand<string, 'ProviderId'>
export type ModelId = Brand<string, 'ModelId'>
export type PermissionKey = Brand<string, 'PermissionKey'>

export type ISODateTimeString = string

export type UserStatus = 'ACTIVE' | 'DISABLED'
export type TenantStatus = 'ACTIVE' | 'DISABLED'

export interface RoleSummary {
  id: RoleId
  code: string
  name: string
  description?: string
}

export interface TenantSummary {
  id: TenantId
  name: string
  status?: TenantStatus
  slug?: string
}

export interface CurrentUser {
  id: UserId
  email: string
  displayName: string
  status: UserStatus
  lastLoginAt?: ISODateTimeString | null
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export interface CurrentSession {
  user: CurrentUser
  tenant: TenantSummary
  roles: RoleSummary[]
  permissions: PermissionKey[]
}

export type ProjectStatus = 'ACTIVE' | 'ARCHIVED'
export type ProjectMemberRole = 'OWNER' | 'EDITOR' | 'VIEWER'

export interface Project {
  id: ProjectId
  tenantId: TenantId
  name: string
  brand: string
  asin: string
  site: string
  notes: string
  status: ProjectStatus
  sortOrder: number
  createdBy: UserId
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export interface ProjectMember {
  id: string
  tenantId: TenantId
  projectId: ProjectId
  userId: UserId
  userEmail: string
  userName: string
  userStatus: UserStatus
  role: ProjectMemberRole
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export interface ProjectMemberCandidate {
  userId: UserId
  userEmail: string
  userName: string
  status: UserStatus
}

export type AssetKind = 'REFERENCE' | 'GENERATED' | 'EDITED'
export type AssetMimeType = 'image/jpeg' | 'image/png' | 'image/webp'

export interface Asset {
  id: AssetId
  tenantId: TenantId
  projectId: ProjectId
  taskId?: TaskId
  kind: AssetKind
  category: string
  filename: string
  mimeType: AssetMimeType
  fileSize: number
  width: number
  height: number
  thumbnailUrl?: string
  previewUrl?: string
  downloadUrl?: string
  isFavorite: boolean
  imageType?: string
  createdBy: UserId
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export type TaskType = 'IMAGE_GENERATION' | 'IMAGE_EDIT'
export const TASK_STATUSES = [
  'QUEUED',
  'RUNNING',
  'SUCCEEDED',
  'FAILED',
  'CANCELLED',
  'RETRYING',
  'TIMED_OUT',
] as const
export type TaskStatus = (typeof TASK_STATUSES)[number]

export interface Task {
  id: TaskId
  tenantId: TenantId
  projectId: ProjectId
  type: TaskType
  status: TaskStatus
  prompt: string
  providerId: ProviderId
  modelId: ModelId
  imageType: string
  parameters: Record<string, unknown>
  inputAssetIds: AssetId[]
  outputAssetIds: AssetId[]
  attempt: number
  maxAttempts: number
  queuedAt: ISODateTimeString | null
  startedAt: ISODateTimeString | null
  finishedAt: ISODateTimeString | null
  timeoutAt: ISODateTimeString | null
  errorCode: string
  errorMessage: string
  createdBy: UserId
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export interface CreateTaskRequest {
  type: TaskType
  prompt: string
  providerId: ProviderId
  modelId: ModelId
  imageType?: string
  referenceAssetIds?: AssetId[]
  editSourceAssetId?: AssetId
  parameters?: Record<string, unknown>
}

export type ProviderType = 'OPENAI' | 'GEMINI' | 'OPENAI_COMPATIBLE'
export type ProviderStatus = 'ENABLED' | 'DISABLED'
export type ProviderTestStatus = 'SUCCESS' | 'FAILURE'

export interface Provider {
  id: ProviderId
  tenantId: TenantId
  type: ProviderType
  name: string
  baseUrl: string
  status: ProviderStatus
  timeoutSeconds: number
  concurrencyLimit: number
  apiKeyHint: string
  apiKeyUpdatedAt?: ISODateTimeString | null
  lastTestStatus: ProviderTestStatus | ''
  lastTestedAt?: ISODateTimeString | null
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export interface ProviderTestResult {
  status: ProviderTestStatus
  durationMs: number
  checkedAt: ISODateTimeString
  httpStatus?: number | null
  requestId: string
  message: string
}

export interface ModelPricing {
  currency: string
  unitPrices: Record<string, number>
}

export type ModelStatus = 'ENABLED' | 'DISABLED'

export interface Model {
  id: ModelId
  tenantId: TenantId
  providerId: ProviderId
  providerName: string
  providerType?: ProviderType
  modelName: string
  displayName: string
  supportsGenerate: boolean
  supportsEdit: boolean
  supportsMultiReference: boolean
  supportsN: boolean
  maxOutputCount: number
  supportedSizes: string[]
  supportedQualities: string[]
  supportedOutputFormats: string[]
  pricing: ModelPricing
  status: ModelStatus
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}
