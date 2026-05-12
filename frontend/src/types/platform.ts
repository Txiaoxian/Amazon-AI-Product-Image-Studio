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

export interface Project {
  id: ProjectId
  tenantId: TenantId
  name: string
  brand: string
  asin: string
  site: string
  notes: string
  status: ProjectStatus
  createdBy: UserId
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
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
  createdBy: UserId
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export type TaskType = 'IMAGE_GENERATION' | 'IMAGE_EDIT'
export type TaskStatus =
  | 'QUEUED'
  | 'RUNNING'
  | 'FAILED'
  | 'COMPLETED'
  | 'CANCELLED'
  | 'RETRYING'
  | 'TIMED_OUT'

export interface Task {
  id: TaskId
  tenantId: TenantId
  projectId: ProjectId
  type: TaskType
  status: TaskStatus
  prompt: string
  providerId: ProviderId
  modelId: ModelId
  inputAssetIds: AssetId[]
  outputAssetIds: AssetId[]
  progress?: number
  errorCode?: string
  errorMessage?: string
  createdBy: UserId
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
  startedAt?: ISODateTimeString
  completedAt?: ISODateTimeString
}

export interface CreateTaskRequest {
  type: TaskType
  prompt: string
  providerId: ProviderId
  modelId: ModelId
  referenceAssetIds?: AssetId[]
  editSourceAssetId?: AssetId
  parameters?: Record<string, unknown>
}

export type ProviderType = 'OPENAI' | 'GEMINI' | 'OPENAI_COMPATIBLE'
export type ProviderStatus = 'ENABLED' | 'DISABLED'

export interface ProviderCredentialSummary {
  isConfigured: boolean
  keyLast4?: string
  updatedAt?: ISODateTimeString
}

export interface Provider {
  id: ProviderId
  tenantId: TenantId
  name: string
  type: ProviderType
  status: ProviderStatus
  baseUrl?: string
  credential: ProviderCredentialSummary
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export type ModelStatus = 'ENABLED' | 'DISABLED'

export interface ModelCapability {
  supportsImageGeneration: boolean
  supportsImageEdit: boolean
  supportsReferenceImages: boolean
  supportedAspectRatios?: string[]
  supportedImageCounts?: number[]
  supportedResolutions?: string[]
  parameterSchema?: Record<string, unknown>
}

export interface Model {
  id: ModelId
  tenantId: TenantId
  providerId: ProviderId
  name: string
  displayName: string
  status: ModelStatus
  capability: ModelCapability
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}
