import type {
  AssetId,
  ISODateTimeString,
  ProjectId,
  TaskEventId,
  TaskId,
  TaskStatus,
} from './platform'

export const TASK_SSE_EVENT_TYPES = [
  'TASK_QUEUED',
  'TASK_STARTED',
  'TASK_PROGRESS',
  'IMAGE_OUTPUT',
  'USAGE_RECORDED',
  'TASK_FAILED',
  'TASK_COMPLETED',
  'TASK_CANCELLED',
  'TASK_RETRIED',
  'TASK_TIMED_OUT',
  'HEARTBEAT',
] as const

export type TaskSseEventType = (typeof TASK_SSE_EVENT_TYPES)[number]

export interface TaskSseTaskPayload {
  taskId: TaskId
  projectId?: ProjectId
}

export interface TaskStatusPayload extends TaskSseTaskPayload {
  status: TaskStatus
  message?: string
  startedAt?: ISODateTimeString
  completedAt?: ISODateTimeString
}

export interface TaskProgressPayload extends TaskSseTaskPayload {
  status: TaskStatus
  progress: number
  message?: string
}

export interface ImageOutputPayload extends TaskSseTaskPayload {
  assetId: AssetId
  outputIndex: number
  thumbnailUrl?: string
  previewUrl?: string
  width: number
  height: number
  mimeType: string
}

export interface UsageRecordedPayload extends TaskSseTaskPayload {
  providerId?: string
  modelId?: string
  cost?: number
  currency?: string
}

export interface TaskFailurePayload extends TaskSseTaskPayload {
  status: TaskStatus
  errorCode: string
  message: string
}

export interface HeartbeatPayload {
  taskId?: TaskId
  projectId?: ProjectId
  timestamp?: ISODateTimeString
}

export type TaskSsePayload =
  | TaskStatusPayload
  | TaskProgressPayload
  | ImageOutputPayload
  | UsageRecordedPayload
  | TaskFailurePayload
  | HeartbeatPayload
  | Record<string, unknown>

export interface TaskSseEvent<TPayload = TaskSsePayload> {
  id?: TaskEventId
  type: TaskSseEventType
  data: TPayload
  receivedAt: ISODateTimeString
}
