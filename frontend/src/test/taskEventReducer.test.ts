import { describe, expect, it } from 'vitest'
import { createTaskEventState, reduceTaskEventState } from '../lib/taskSseClient'
import {
  TASK_STATUSES,
  type AssetId,
  type ProjectId,
  type TaskEventId,
  type TaskId,
} from '../types/platform'
import type { TaskSseEvent, TaskSsePayloadMap } from '../types/sse'

function event<TType extends TaskSseEvent['type']>(
  type: TType,
  data: TaskSsePayloadMap[TType],
  id = `evt_${type}` as TaskEventId,
): TaskSseEvent<TType> {
  return {
    id,
    type,
    data,
    receivedAt: '2026-05-17T00:00:00Z',
  }
}

describe('task event reducer', () => {
  it('uses the canonical backend task statuses without transitional COMPLETED', () => {
    expect(TASK_STATUSES).toEqual([
      'QUEUED',
      'RUNNING',
      'SUCCEEDED',
      'FAILED',
      'CANCELLED',
      'RETRYING',
      'TIMED_OUT',
    ])
    expect(TASK_STATUSES).not.toContain('COMPLETED')
  })

  it('reduces queued, started, progress, output, usage, and completion events', () => {
    const initial = createTaskEventState('task_1' as TaskId)
    const queued = reduceTaskEventState(
      initial,
      event('TASK_QUEUED', {
        taskId: 'task_1' as TaskId,
        projectId: 'project_1' as ProjectId,
        status: 'QUEUED',
        attempt: 1,
        queuedAt: '2026-05-17T00:00:00Z',
      }),
    )
    const started = reduceTaskEventState(
      queued,
      event('TASK_STARTED', {
        taskId: 'task_1' as TaskId,
        projectId: 'project_1' as ProjectId,
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:00:05Z',
      }),
    )
    const progressed = reduceTaskEventState(
      started,
      event('TASK_PROGRESS', {
        taskId: 'task_1' as TaskId,
        projectId: 'project_1' as ProjectId,
        status: 'RUNNING',
        attempt: 1,
        progress: 60,
        message: 'Rendering.',
      }),
    )
    const withOutput = reduceTaskEventState(
      progressed,
      event('IMAGE_OUTPUT', {
        taskId: 'task_1' as TaskId,
        projectId: 'project_1' as ProjectId,
        status: 'RUNNING',
        attempt: 1,
        assetId: 'asset_1' as AssetId,
        outputIndex: 0,
        previewUrl: '/api/v1/assets/asset_1/download',
        thumbnailUrl: '',
        width: 1024,
        height: 1024,
        mimeType: 'image/png',
      }),
    )
    const withUsage = reduceTaskEventState(
      withOutput,
      event('USAGE_RECORDED', {
        taskId: 'task_1' as TaskId,
        projectId: 'project_1' as ProjectId,
        status: 'RUNNING',
        attempt: 1,
        usageRecordId: 'usage_1',
        inputTokens: 12,
        outputTokens: 34,
        imageCount: 1,
        estimatedCost: '0.04000000',
        currency: 'USD',
      }),
    )
    const completed = reduceTaskEventState(
      withUsage,
      event('TASK_COMPLETED', {
        taskId: 'task_1' as TaskId,
        projectId: 'project_1' as ProjectId,
        status: 'SUCCEEDED',
        attempt: 1,
        finishedAt: '2026-05-17T00:00:10Z',
      }),
    )

    expect(completed).toMatchObject({
      taskId: 'task_1',
      projectId: 'project_1',
      status: 'SUCCEEDED',
      attempt: 1,
      progress: 60,
      message: 'Rendering.',
      queuedAt: '2026-05-17T00:00:00Z',
      startedAt: '2026-05-17T00:00:05Z',
      finishedAt: '2026-05-17T00:00:10Z',
      lastEventId: 'evt_TASK_COMPLETED',
      lastEventType: 'TASK_COMPLETED',
    })
    expect(completed.outputs).toEqual([
      expect.objectContaining({
        assetId: 'asset_1',
        outputIndex: 0,
        previewUrl: '/api/v1/assets/asset_1/download',
      }),
    ])
    expect(completed.usageRecords).toEqual([
      expect.objectContaining({
        usageRecordId: 'usage_1',
        estimatedCost: '0.04000000',
        currency: 'USD',
      }),
    ])
  })

  it('reduces failed, cancelled, retried, and timed-out events', () => {
    const initial = createTaskEventState('task_2' as TaskId)
    const failed = reduceTaskEventState(
      initial,
      event('TASK_FAILED', {
        taskId: 'task_2' as TaskId,
        status: 'FAILED',
        attempt: 1,
        errorCode: 'EXECUTION_FAILED',
        message: 'Provider failed.',
      }),
    )
    const retried = reduceTaskEventState(
      failed,
      event('TASK_RETRIED', {
        taskId: 'task_2' as TaskId,
        status: 'RETRYING',
        attempt: 2,
        previousStatus: 'FAILED',
      }),
    )
    const cancelled = reduceTaskEventState(
      retried,
      event('TASK_CANCELLED', {
        taskId: 'task_2' as TaskId,
        status: 'CANCELLED',
        attempt: 2,
        finishedAt: '2026-05-17T00:01:00Z',
      }),
    )
    const timedOut = reduceTaskEventState(
      cancelled,
      event('TASK_TIMED_OUT', {
        taskId: 'task_2' as TaskId,
        status: 'TIMED_OUT',
        attempt: 2,
        finishedAt: '2026-05-17T00:02:00Z',
        errorCode: 'TASK_TIMED_OUT',
        message: 'Task execution timed out.',
      }),
    )

    expect(failed).toMatchObject({
      status: 'FAILED',
      errorCode: 'EXECUTION_FAILED',
      errorMessage: 'Provider failed.',
    })
    expect(retried).toMatchObject({
      status: 'RETRYING',
      attempt: 2,
      previousStatus: 'FAILED',
    })
    expect(cancelled).toMatchObject({
      status: 'CANCELLED',
      finishedAt: '2026-05-17T00:01:00Z',
    })
    expect(timedOut).toMatchObject({
      status: 'TIMED_OUT',
      errorCode: 'TASK_TIMED_OUT',
      errorMessage: 'Task execution timed out.',
      finishedAt: '2026-05-17T00:02:00Z',
    })
  })

  it('ignores heartbeat and events from a different project context', () => {
    const initial = createTaskEventState('task_3' as TaskId, 'project_1' as ProjectId)
    const heartbeat = reduceTaskEventState(
      initial,
      event('HEARTBEAT', {}),
    )
    const crossProject = reduceTaskEventState(
      heartbeat,
      event('TASK_STARTED', {
        taskId: 'task_3' as TaskId,
        projectId: 'project_2' as ProjectId,
        status: 'RUNNING',
        attempt: 1,
        startedAt: '2026-05-17T00:03:00Z',
      }),
    )

    expect(heartbeat).toBe(initial)
    expect(crossProject).toBe(initial)
  })
})
