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
  status: TaskStatus
  attempt: number
}

export interface TaskQueuedPayload extends TaskSseTaskPayload {
  status: 'QUEUED'
  queuedAt: ISODateTimeString
}

export interface TaskStartedPayload extends TaskSseTaskPayload {
  status: 'RUNNING'
  startedAt: ISODateTimeString
}

export interface TaskProgressPayload extends TaskSseTaskPayload {
  status: 'RUNNING'
  progress: number
  message?: string
}

export interface ImageOutputPayload extends TaskSseTaskPayload {
  status: TaskStatus
  assetId: AssetId
  outputIndex: number
  thumbnailUrl?: string
  previewUrl?: string
  width: number
  height: number
  mimeType: string
  sourceTaskId?: TaskId
  assetKind?: string
  sizeBytes?: number
  providerIndex?: unknown
}

export interface UsageRecordedPayload extends TaskSseTaskPayload {
  usageRecordId: string
  inputTokens: number
  outputTokens: number
  imageCount: number
  estimatedCost: string
  currency: string
}

export interface TaskFailurePayload extends TaskSseTaskPayload {
  status: 'FAILED'
  errorCode: string
  message: string
}

export interface TaskCompletedPayload extends TaskSseTaskPayload {
  status: 'SUCCEEDED'
  finishedAt: ISODateTimeString
}

export interface TaskCancelledPayload extends TaskSseTaskPayload {
  status: 'CANCELLED'
  finishedAt: ISODateTimeString
}

export interface TaskRetriedPayload extends TaskSseTaskPayload {
  status: 'RETRYING'
  previousStatus?: TaskStatus
  errorCode?: string
  message?: string
}

export interface TaskTimedOutPayload extends TaskSseTaskPayload {
  status: 'TIMED_OUT'
  finishedAt: ISODateTimeString
  errorCode: string
  message: string
}

export type HeartbeatPayload = Record<string, never>

export interface TaskSsePayloadMap {
  TASK_QUEUED: TaskQueuedPayload
  TASK_STARTED: TaskStartedPayload
  TASK_PROGRESS: TaskProgressPayload
  IMAGE_OUTPUT: ImageOutputPayload
  USAGE_RECORDED: UsageRecordedPayload
  TASK_FAILED: TaskFailurePayload
  TASK_COMPLETED: TaskCompletedPayload
  TASK_CANCELLED: TaskCancelledPayload
  TASK_RETRIED: TaskRetriedPayload
  TASK_TIMED_OUT: TaskTimedOutPayload
  HEARTBEAT: HeartbeatPayload
}

export type TaskSsePayload = TaskSsePayloadMap[TaskSseEventType]

export type TaskSseEvent<TType extends TaskSseEventType = TaskSseEventType> = {
  [K in TType]: {
    id?: TaskEventId
    type: K
    data: TaskSsePayloadMap[K]
    receivedAt: ISODateTimeString
  }
}[TType]
