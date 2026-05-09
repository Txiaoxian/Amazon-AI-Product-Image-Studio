import {
  TASK_SSE_EVENT_TYPES,
  type TaskSseEvent,
  type TaskSseEventType,
  type TaskSsePayload,
} from '../types/sse'
import type { ProjectId, TaskEventId, TaskId } from '../types/platform'

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

export type TaskSseEventHandler<TPayload = TaskSsePayload> = (event: TaskSseEvent<TPayload>) => void

export interface TaskSseClientOptions {
  baseUrl?: string
  eventSourceFactory?: EventSourceFactory
  lastEventId?: TaskEventId
  onError?: (error: Event | Error) => void
  onEvent?: TaskSseEventHandler
  onHeartbeat?: TaskSseEventHandler
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
  on<TPayload = unknown>(eventType: TaskSseEventType, handler: TaskSseEventHandler<TPayload>): () => void
  reconnect(): void
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

  const handleMessage = (eventType: TaskSseEventType, event: MessageEvent<string>) => {
    try {
      const nextEventId = event.lastEventId ? (event.lastEventId as TaskEventId) : undefined
      if (nextEventId) {
        lastEventId = nextEventId
      }

      const taskEvent: TaskSseEvent = {
        id: nextEventId,
        type: eventType,
        data: parseEventData(eventType, event.data),
        receivedAt: new Date().toISOString(),
      }

      if (eventType === 'HEARTBEAT') {
        lastHeartbeatAt = Date.now()
        options.onHeartbeat?.(taskEvent)
      }

      options.onEvent?.(taskEvent)
      handlers.get(eventType)?.forEach((handler) => {
        handler(taskEvent)
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
    on: <TPayload = unknown>(eventType: TaskSseEventType, handler: TaskSseEventHandler<TPayload>) => {
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

function parseEventData(eventType: TaskSseEventType, data: string): TaskSsePayload {
  if (!data) {
    return {}
  }

  try {
    return JSON.parse(data) as TaskSsePayload
  } catch {
    throw new SseMessageParseError(eventType)
  }
}
