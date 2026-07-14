import { describe, expect, it, vi } from 'vitest'
import { createApiClient } from '../api/client'
import { createTaskApi } from '../api/tasks'
import type { AssetId, ModelId, ProviderId } from '../types/platform'

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_task_api',
    }),
    { status },
  )
}

const task = {
  id: 'task_1',
  tenantId: 'tenant_1',
  projectId: 'project_1',
  type: 'IMAGE_GENERATION',
  status: 'QUEUED',
  prompt: 'Create a clean ecommerce hero image.',
  providerId: 'provider_1',
  modelId: 'model_1',
  imageType: 'MAIN',
  parameters: {
    size: '1024x1024',
  },
  inputAssetIds: ['asset_reference_1'],
  outputAssetIds: [],
  attempt: 1,
  maxAttempts: 3,
  queuedAt: '2026-05-17T00:00:00Z',
  startedAt: null,
  finishedAt: null,
  timeoutAt: '2026-05-17T00:30:00Z',
  errorCode: '',
  errorMessage: '',
  createdBy: 'user_1',
  createdAt: '2026-05-17T00:00:00Z',
  updatedAt: '2026-05-17T00:00:00Z',
}

describe('task API wrappers', () => {
  it('uses authenticated task endpoints with CSRF for state-changing requests', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse({
          records: [task],
          total: 1,
          pageNum: 1,
          pageSize: 20,
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          records: [{ asset: { id: 'asset_1' }, task }],
          total: 1,
          pageNum: 2,
          pageSize: 10,
        }),
      )
      .mockResolvedValueOnce(jsonResponse(task, 201))
      .mockResolvedValueOnce(jsonResponse(task))
      .mockResolvedValueOnce(jsonResponse({ ...task, status: 'CANCELLED' }))
      .mockResolvedValueOnce(jsonResponse({ ...task, status: 'QUEUED', attempt: 2 }))
    const taskApi = createTaskApi(createApiClient({ fetchImpl }))

    await expect(taskApi.list('project_1', { status: 'QUEUED', type: 'IMAGE_GENERATION', pageNum: 1, pageSize: 20 })).resolves.toMatchObject({
      records: [task],
      total: 1,
    })
    await expect(taskApi.listHistory('project_1', { imageType: 'A_PLUS', kind: 'EDITED', pageNum: 2, pageSize: 10 })).resolves.toMatchObject({
      records: [{ asset: { id: 'asset_1' }, task }],
      total: 1,
      pageNum: 2,
      pageSize: 10,
    })
    await expect(
      taskApi.create(
        'project_1',
        {
          type: 'IMAGE_GENERATION',
          prompt: 'Create a clean ecommerce hero image.',
          providerId: 'provider_1' as ProviderId,
          modelId: 'model_1' as ModelId,
          imageType: 'MAIN',
          referenceAssetIds: ['asset_reference_1' as AssetId],
          parameters: {
            size: '1024x1024',
          },
        },
        'csrf_memory_only',
      ),
    ).resolves.toEqual(task)
    await expect(taskApi.get('task_1')).resolves.toEqual(task)
    await expect(taskApi.cancel('task_1', 'csrf_memory_only')).resolves.toMatchObject({ status: 'CANCELLED' })
    await expect(taskApi.retry('task_1', 'csrf_memory_only')).resolves.toMatchObject({
      status: 'QUEUED',
      attempt: 2,
    })

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/projects/project_1/tasks?status=QUEUED&type=IMAGE_GENERATION&pageNum=1&pageSize=20',
      '/api/v1/projects/project_1/history?pageNum=2&pageSize=10&kind=EDITED&imageType=A_PLUS',
      '/api/v1/projects/project_1/tasks',
      '/api/v1/tasks/task_1',
      '/api/v1/tasks/task_1/cancel',
      '/api/v1/tasks/task_1/retry',
    ])
    expect(fetchImpl.mock.calls[0][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect(fetchImpl.mock.calls[1][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect(fetchImpl.mock.calls[2][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'POST' }))
    expect(fetchImpl.mock.calls[3][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect(fetchImpl.mock.calls[4][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'POST' }))
    expect(fetchImpl.mock.calls[5][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'POST' }))
    expect((fetchImpl.mock.calls[2][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[4][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[5][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[2][1]?.body as string)).toEqual({
      type: 'IMAGE_GENERATION',
      prompt: 'Create a clean ecommerce hero image.',
      providerId: 'provider_1',
      modelId: 'model_1',
      imageType: 'MAIN',
      referenceAssetIds: ['asset_reference_1'],
      parameters: {
        size: '1024x1024',
      },
    })
  })

  it('propagates normalized API errors from task mutations', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: 'TASK_STATE_CONFLICT',
            message: 'Task cannot be retried.',
          },
          requestId: 'req_task_conflict',
        }),
        { status: 409 },
      ),
    )
    const taskApi = createTaskApi(createApiClient({ fetchImpl }))

    await expect(taskApi.retry('task_1', 'csrf_memory_only')).rejects.toMatchObject({
      code: 'TASK_STATE_CONFLICT',
      message: 'Task cannot be retried.',
      requestId: 'req_task_conflict',
      status: 409,
    })
  })
})
