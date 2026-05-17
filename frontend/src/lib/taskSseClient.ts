import {
  TASK_SSE_EVENT_TYPES,
  type ImageOutputPayload,
  type TaskSseEvent,
  type TaskSseEventType,
  type TaskSsePayloadMap,
  type UsageRecordedPayload,
} from '../types/sse'
import type { ISODateTimeString, ProjectId, TaskEventId, TaskId, TaskStatus } from '../types/platform'

export const DEFAULT_TASK_EVENTS_PATH = '/api/v1/events/tasks'

export interface EventSourceLike {
  readonly readyState?: number
  onerror: ((event: Event) => void) | null
  onopen: ((event: Event) => void) | null
  addEventListener(type: string, listener: (event: MessageEvent<string>) => void): void
  close(): void
  removeEventListener?(type: string, listener: (event: MessageEvent<string>) => void): void
}

export type EventSourceFactory = (url: string, init: EventSourceInit) => EventSourceLike

export interface TaskSseReconnectContext {
  lastEventId?: TaskEventId
  readyState?: number
}

export type TaskSseEventHandler<TType extends TaskSseEventType = TaskSseEventType> = (
  event: TaskSseEvent<TType>,
) => void

export interface TaskSseClientOptions {
  baseUrl?: string
  eventSourceFactory?: EventSourceFactory
  lastEventId?: TaskEventId
  onError?: (error: Event | Error) => void
  onEvent?: TaskSseEventHandler
  onHeartbeat?: TaskSseEventHandler<'HEARTBEAT'>
  onOpen?: (event: Event) => void
  onReconnect?: (context: TaskSseReconnectContext) => void
  projectId?: ProjectId
  taskId?: TaskId
  withCredentials?: boolean
}

export interface TaskSseClient {
  close(): void
  connect(): void
  getEventSource(): EventSourceLike | undefined
  getLastEventId(): TaskEventId | undefined
  getLastHeartbeatAt(): number | undefined
  on<TType extends TaskSseEventType>(eventType: TType, handler: TaskSseEventHandler<TType>): () => void
  reconnect(): void
}

export interface TaskEventState {
  attempt?: number
  errorCode?: string
  errorMessage?: string
  finishedAt?: ISODateTimeString
  lastEventId?: TaskEventId
  lastEventType?: TaskSseEventType
  message?: string
  outputs: ImageOutputPayload[]
  previousStatus?: TaskStatus
  progress?: number
  projectId?: ProjectId
  queuedAt?: ISODateTimeString
  startedAt?: ISODateTimeString
  status?: TaskStatus
  taskId?: TaskId
  usageRecords: UsageRecordedPayload[]
}

export class SseMessageParseError extends Error {
  readonly eventType: TaskSseEventType

  constructor(eventType: TaskSseEventType) {
    super(`SSE event ${eventType} did not contain valid JSON data.`)
    this.name = 'SseMessageParseError'
    this.eventType = eventType
  }
}

export function createTaskSseClient(options: TaskSseClientOptions = {}): TaskSseClient {
  let source: EventSourceLike | undefined
  let lastEventId = options.lastEventId
  let lastHeartbeatAt: number | undefined
  const listeners = new Map<TaskSseEventType, (event: MessageEvent<string>) => void>()
  const handlers = createHandlerMap()

  const connect = () => {
    if (source) {
      return
    }

    const factory = options.eventSourceFactory ?? createDefaultEventSource
    const nextSource = factory(
      buildTaskEventsUrl({
        baseUrl: options.baseUrl,
        lastEventId,
        projectId: options.projectId,
        taskId: options.taskId,
      }),
      { withCredentials: options.withCredentials ?? true },
    )

    source = nextSource
    TASK_SSE_EVENT_TYPES.forEach((eventType) => {
      const listener = (event: MessageEvent<string>) => handleMessage(eventType, event)
      listeners.set(eventType, listener)
      nextSource.addEventListener(eventType, listener)
    })

    nextSource.onopen = (event) => {
      options.onOpen?.(event)
    }

    nextSource.onerror = (event) => {
      options.onError?.(event)
      options.onReconnect?.({
        lastEventId,
        readyState: nextSource.readyState,
      })
    }
  }

  const close = () => {
    if (!source) {
      return
    }

    listeners.forEach((listener, eventType) => {
      source?.removeEventListener?.(eventType, listener)
    })
    listeners.clear()
    source.onopen = null
    source.onerror = null
    source.close()
    source = undefined
  }

  const handleMessage = <TType extends TaskSseEventType>(eventType: TType, event: MessageEvent<string>) => {
    try {
      const nextEventId = event.lastEventId ? (event.lastEventId as TaskEventId) : undefined
      if (nextEventId) {
        lastEventId = nextEventId
      }

      const taskEvent: TaskSseEvent<TType> = {
        id: nextEventId,
        type: eventType,
        data: parseEventData(eventType, event.data),
        receivedAt: new Date().toISOString(),
      }

      if (eventType === 'HEARTBEAT') {
        lastHeartbeatAt = Date.now()
        options.onHeartbeat?.(taskEvent as TaskSseEvent<'HEARTBEAT'>)
      }

      options.onEvent?.(taskEvent as TaskSseEvent)
      handlers.get(eventType)?.forEach((handler) => {
        ;(handler as TaskSseEventHandler<TType>)(taskEvent)
      })
    } catch (error) {
      options.onError?.(error instanceof Error ? error : new Error('Failed to process SSE event.'))
    }
  }

  return {
    close,
    connect,
    getEventSource: () => source,
    getLastEventId: () => lastEventId,
    getLastHeartbeatAt: () => lastHeartbeatAt,
    on: <TType extends TaskSseEventType>(eventType: TType, handler: TaskSseEventHandler<TType>) => {
      const eventHandlers = handlers.get(eventType)
      eventHandlers?.add(handler as TaskSseEventHandler)
      return () => {
        eventHandlers?.delete(handler as TaskSseEventHandler)
      }
    },
    reconnect: () => {
      close()
      connect()
    },
  }
}

export function buildTaskEventsUrl(options: {
  baseUrl?: string
  lastEventId?: TaskEventId
  projectId?: ProjectId
  taskId?: TaskId
} = {}): string {
  const params = new URLSearchParams()

  if (options.projectId) {
    params.set('projectId', options.projectId)
  }

  if (options.taskId) {
    params.set('taskId', options.taskId)
  }

  if (options.lastEventId) {
    params.set('lastEventId', options.lastEventId)
  }

  const baseUrl = options.baseUrl ?? DEFAULT_TASK_EVENTS_PATH
  const query = params.toString()
  return query ? `${baseUrl}?${query}` : baseUrl
}

export function createTaskEventState(taskId?: TaskId, projectId?: ProjectId): TaskEventState {
  return {
    outputs: [],
    projectId,
    taskId,
    usageRecords: [],
  }
}

export function reduceTaskEventState(state: TaskEventState, event: TaskSseEvent): TaskEventState {
  if (event.type === 'HEARTBEAT') {
    return state
  }

  if (state.taskId && event.data.taskId !== state.taskId) {
    return state
  }

  if (state.projectId && event.data.projectId && event.data.projectId !== state.projectId) {
    return state
  }

  const nextState: TaskEventState = {
    ...state,
    attempt: event.data.attempt,
    lastEventId: event.id ?? state.lastEventId,
    lastEventType: event.type,
    projectId: event.data.projectId ?? state.projectId,
    status: event.data.status,
    taskId: event.data.taskId,
  }

  switch (event.type) {
    case 'TASK_QUEUED':
      return {
        ...nextState,
        errorCode: undefined,
        errorMessage: undefined,
        queuedAt: event.data.queuedAt,
      }
    case 'TASK_STARTED':
      return {
        ...nextState,
        errorCode: undefined,
        errorMessage: undefined,
        startedAt: event.data.startedAt,
      }
    case 'TASK_PROGRESS':
      return {
        ...nextState,
        message: event.data.message,
        progress: event.data.progress,
      }
    case 'IMAGE_OUTPUT':
      return {
        ...nextState,
        outputs: upsertImageOutput(state.outputs, event.data),
      }
    case 'USAGE_RECORDED':
      return {
        ...nextState,
        usageRecords: upsertUsageRecord(state.usageRecords, event.data),
      }
    case 'TASK_FAILED':
      return {
        ...nextState,
        errorCode: event.data.errorCode,
        errorMessage: event.data.message,
      }
    case 'TASK_COMPLETED':
      return {
        ...nextState,
        errorCode: undefined,
        errorMessage: undefined,
        finishedAt: event.data.finishedAt,
        status: 'SUCCEEDED',
      }
    case 'TASK_CANCELLED':
      return {
        ...nextState,
        errorCode: undefined,
        errorMessage: undefined,
        finishedAt: event.data.finishedAt,
      }
    case 'TASK_RETRIED':
      return {
        ...nextState,
        errorCode: event.data.errorCode,
        errorMessage: event.data.message,
        previousStatus: event.data.previousStatus,
      }
    case 'TASK_TIMED_OUT':
      return {
        ...nextState,
        errorCode: event.data.errorCode,
        errorMessage: event.data.message,
        finishedAt: event.data.finishedAt,
      }
  }
}

function createHandlerMap(): Map<TaskSseEventType, Set<TaskSseEventHandler>> {
  const handlers = new Map<TaskSseEventType, Set<TaskSseEventHandler>>()
  TASK_SSE_EVENT_TYPES.forEach((eventType) => {
    handlers.set(eventType, new Set())
  })
  return handlers
}

function createDefaultEventSource(url: string, init: EventSourceInit): EventSourceLike {
  if (typeof EventSource === 'undefined') {
    throw new Error('EventSource is not available in this environment.')
  }

  return new EventSource(url, init)
}

function parseEventData<TType extends TaskSseEventType>(
  eventType: TType,
  data: string,
): TaskSsePayloadMap[TType] {
  if (!data && eventType === 'HEARTBEAT') {
    return {} as TaskSsePayloadMap[TType]
  }

  try {
    return JSON.parse(data) as TaskSsePayloadMap[TType]
  } catch {
    throw new SseMessageParseError(eventType)
  }
}

function upsertImageOutput(outputs: ImageOutputPayload[], output: ImageOutputPayload): ImageOutputPayload[] {
  const existingIndex = outputs.findIndex((item) => item.assetId === output.assetId || item.outputIndex === output.outputIndex)
  if (existingIndex === -1) {
    return [...outputs, output]
  }

  return outputs.map((item, index) => (index === existingIndex ? output : item))
}

function upsertUsageRecord(
  usageRecords: UsageRecordedPayload[],
  usageRecord: UsageRecordedPayload,
): UsageRecordedPayload[] {
  const existingIndex = usageRecords.findIndex((item) => item.usageRecordId === usageRecord.usageRecordId)
  if (existingIndex === -1) {
    return [...usageRecords, usageRecord]
  }

  return usageRecords.map((item, index) => (index === existingIndex ? usageRecord : item))
}
