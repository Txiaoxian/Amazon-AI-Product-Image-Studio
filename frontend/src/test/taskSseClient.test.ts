import { describe, expect, it, vi } from 'vitest'
import {
  createTaskSseClient,
  type EventSourceFactory,
  type EventSourceLike,
} from '../lib/taskSseClient'
import type { ProjectId, TaskEventId, TaskId } from '../types/platform'
import type { TaskSseEvent } from '../types/sse'

class FakeEventSource implements EventSourceLike {
  readonly url: string
  readonly init: EventSourceInit
  readyState = 0
  onerror: ((event: Event) => void) | null = null
  onopen: ((event: Event) => void) | null = null
  private readonly listeners = new Map<string, Set<(event: MessageEvent<string>) => void>>()

  constructor(url: string, init: EventSourceInit) {
    this.url = url
    this.init = init
  }

  addEventListener(type: string, listener: (event: MessageEvent<string>) => void): void {
    const listeners = this.listeners.get(type) ?? new Set()
    listeners.add(listener)
    this.listeners.set(type, listeners)
  }

  removeEventListener(type: string, listener: (event: MessageEvent<string>) => void): void {
    this.listeners.get(type)?.delete(listener)
  }

  close(): void {
    this.readyState = 2
  }

  emit(type: string, data: unknown, lastEventId?: string): void {
    const event = new MessageEvent<string>(type, {
      data: JSON.stringify(data),
      lastEventId,
    })
    this.listeners.get(type)?.forEach((listener) => {
      listener(event)
    })
  }

  fail(): void {
    this.onerror?.(new Event('error'))
  }
}

function createFakeFactory() {
  const sources: FakeEventSource[] = []
  const factory = vi.fn<EventSourceFactory>((url, init) => {
    const source = new FakeEventSource(url, init)
    sources.push(source)
    return source
  })

  return { factory, sources }
}

describe('task SSE client', () => {
  it('opens EventSource with credentials and dispatches task events', () => {
    const { factory, sources } = createFakeFactory()
    const received: TaskSseEvent[] = []
    const startedHandler = vi.fn()
    const client = createTaskSseClient({
      eventSourceFactory: factory,
      lastEventId: 'evt_old' as TaskEventId,
      onEvent: (event) => received.push(event),
      projectId: 'project_1' as ProjectId,
      taskId: 'task_1' as TaskId,
    })
    client.on('TASK_STARTED', startedHandler)

    client.connect()

    expect(factory).toHaveBeenCalledOnce()
    expect(sources[0].init).toEqual({ withCredentials: true })
    expect(new URL(sources[0].url, 'https://studio.test').searchParams.get('lastEventId')).toBe('evt_old')
    expect(new URL(sources[0].url, 'https://studio.test').searchParams.get('projectId')).toBe('project_1')
    expect(new URL(sources[0].url, 'https://studio.test').searchParams.get('taskId')).toBe('task_1')

    sources[0].emit(
      'TASK_STARTED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:00Z',
      },
      'evt_1',
    )

    expect(client.getLastEventId()).toBe('evt_1')
    expect(received).toHaveLength(1)
    expect(received[0]).toMatchObject({
      id: 'evt_1',
      type: 'TASK_STARTED',
      data: {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:00Z',
      },
    })
    expect(startedHandler).toHaveBeenCalledWith(expect.objectContaining({ type: 'TASK_STARTED' }))
  })

  it('tracks heartbeat events and exposes reconnect context', () => {
    const { factory, sources } = createFakeFactory()
    const heartbeatHandler = vi.fn()
    const reconnectHandler = vi.fn()
    const client = createTaskSseClient({
      eventSourceFactory: factory,
      onHeartbeat: heartbeatHandler,
      onReconnect: reconnectHandler,
    })

    client.connect()
    sources[0].emit('HEARTBEAT', {})
    sources[0].fail()

    expect(client.getLastEventId()).toBeUndefined()
    expect(client.getLastHeartbeatAt()).toEqual(expect.any(Number))
    expect(heartbeatHandler).toHaveBeenCalledWith(
      expect.objectContaining({
        type: 'HEARTBEAT',
      }),
    )
    expect(reconnectHandler).toHaveBeenCalledWith({
      lastEventId: undefined,
      readyState: 0,
    })
  })

  it('reconnects with the recorded last event id query fallback', () => {
    const { factory, sources } = createFakeFactory()
    const client = createTaskSseClient({ eventSourceFactory: factory })

    client.connect()
    sources[0].emit(
      'TASK_COMPLETED',
      {
        taskId: 'task_1',
        projectId: 'project_1',
        status: 'SUCCEEDED',
        attempt: 1,
        finishedAt: '2026-05-17T00:01:00Z',
      },
      'evt_done',
    )
    client.reconnect()

    expect(sources).toHaveLength(2)
    expect(sources[0].readyState).toBe(2)
    expect(new URL(sources[1].url, 'https://studio.test').searchParams.get('lastEventId')).toBe('evt_done')
  })
})
