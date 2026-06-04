import { useCallback, useEffect, useRef, useState } from 'react'
import { assetApi as defaultAssetApi, type AssetApi } from '../api/assets'
import { isApiClientError } from '../api/client'
import { taskApi as defaultTaskApi, type TaskApi } from '../api/tasks'
import {
  createTaskEventState,
  createTaskSseClient,
  reduceTaskEventState,
  type TaskEventState,
  type TaskSseClient,
  type TaskSseClientOptions,
} from '../lib/taskSseClient'
import type { AssetId, ProjectId, Task, TaskStatus } from '../types/platform'
import type { ImageOutputPayload } from '../types/sse'
import type { WorkbenchTaskInput, WorkbenchTaskSubmission } from '../types/workbench'

type GenerationStatus = 'idle' | 'loading' | 'success' | 'error'
type PendingTaskAction = 'cancel' | 'retry' | null

export interface BackendImageResult {
  assetId: AssetId
  previewUrl?: string
  thumbnailUrl?: string
  width: number
  height: number
  mimeType: string
  fileSize?: number
}

export interface BackendCurrentGeneration {
  kind: 'backend'
  outputIndex: number
  task: Task
  result: BackendImageResult
}

export type WorkbenchGeneration = BackendCurrentGeneration

export interface UseGenerationOptions {
  assetApi?: AssetApi
  csrfToken?: string
  onModelInvalidated?: () => void | Promise<void>
  projectId?: ProjectId | null
  taskApi?: TaskApi
  taskSseClientFactory?: (options: TaskSseClientOptions) => TaskSseClient
}

export function useGeneration({
  assetApi = defaultAssetApi,
  csrfToken,
  onModelInvalidated,
  projectId,
  taskApi = defaultTaskApi,
  taskSseClientFactory = createTaskSseClient,
}: UseGenerationOptions = {}) {
  const [status, setStatus] = useState<GenerationStatus>('idle')
  const [error, setError] = useState('')
  const [currentItems, setCurrentItems] = useState<WorkbenchGeneration[]>([])
  const [selectedIndex, setSelectedIndex] = useState(0)
  const [currentTask, setCurrentTask] = useState<Task | null>(null)
  const [taskState, setTaskState] = useState<TaskEventState | null>(null)
  const [pendingTaskAction, setPendingTaskAction] = useState<PendingTaskAction>(null)
  const current = currentItems[selectedIndex] ?? null
  const currentTaskRef = useRef<Task | null>(null)
  const taskStateRef = useRef<TaskEventState | null>(null)
  const projectIdRef = useRef<ProjectId | null | undefined>(projectId)
  const streamRef = useRef<TaskSseClient | null>(null)
  const submitLockRef = useRef(false)

  useEffect(() => {
    projectIdRef.current = projectId
  }, [projectId])

  useEffect(() => {
    return () => {
      streamRef.current?.close()
    }
  }, [])

  useEffect(() => {
    streamRef.current?.close()
    streamRef.current = null
    currentTaskRef.current = null
    taskStateRef.current = null
    setCurrentTask(null)
    setTaskState(null)
    setPendingTaskAction(null)
    setCurrentItems((items) => {
      if (items.length > 0) {
        setSelectedIndex(0)
        setStatus('idle')
        setError('')
        return []
      }

      return items
    })
  }, [projectId])

  useEffect(() => {
    if (!currentTask || !taskState) {
      return
    }

    setCurrentItems(
      taskState.outputs
        .slice()
        .sort((left, right) => left.outputIndex - right.outputIndex)
        .map((output) => createBackendCurrentGeneration(currentTask, output)),
    )
    setSelectedIndex((index) => Math.min(index, Math.max(taskState.outputs.length - 1, 0)))

    if (!taskState.status) {
      setStatus('loading')
      return
    }

    if (isTerminalSuccess(taskState.status)) {
      setStatus('success')
      setError('')
      return
    }

    if (isTerminalFailure(taskState.status)) {
      setStatus('error')
      setError(taskState.errorMessage || terminalStatusMessage(taskState.status))
      return
    }

    setStatus('loading')
    setError('')
  }, [currentTask, taskState])

  const generateTask = useCallback(
    async (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput): Promise<Task | null> => {
      if (submitLockRef.current || hasActiveTask(currentTaskRef.current, taskStateRef.current)) {
        return null
      }
      if (!projectId) {
        setStatus('error')
        setError('请先选择项目。')
        return null
      }
      if (!csrfToken) {
        setStatus('error')
        setError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      const prompt = request.prompt.trim()
      if (!prompt) {
        setStatus('error')
        setError('请输入图片生成提示词。')
        return null
      }

      submitLockRef.current = true
      setStatus('loading')
      setError('')
      setCurrentItems([])
      setSelectedIndex(0)
      setCurrentTask(null)
      setTaskState(null)
      currentTaskRef.current = null
      taskStateRef.current = null
      streamRef.current?.close()
      streamRef.current = null

      try {
        const task = await taskApi.create(projectId, buildTaskCreateRequest(prompt, workbenchInput), csrfToken)
        const initialState = createTaskEventState(task.id, projectId)
        currentTaskRef.current = task
        taskStateRef.current = initialState
        setCurrentTask(task)
        setTaskState(initialState)
        streamRef.current = connectTaskStream({
          initialState,
          shouldAcceptEvent: () => projectIdRef.current === projectId && currentTaskRef.current?.id === task.id,
          projectId,
          task,
          taskSseClientFactory,
          onEventState: (nextState) => {
            taskStateRef.current = nextState
            setTaskState(nextState)
          },
        })
        return task
      } catch (requestError) {
        const friendlyError = getTaskCreateErrorMessage(requestError)
        setError(friendlyError)
        setStatus('error')
        if (isStaleWorkbenchSelectionError(requestError)) {
          await onModelInvalidated?.()
        }
        return null
      } finally {
        submitLockRef.current = false
      }
    },
    [csrfToken, onModelInvalidated, projectId, taskApi, taskSseClientFactory],
  )

  const cancelCurrentTask = useCallback(async (): Promise<boolean> => {
    const task = currentTaskRef.current
    if (!task || !csrfToken || pendingTaskAction) {
      return false
    }

    setPendingTaskAction('cancel')
    try {
      await taskApi.cancel(task.id, csrfToken)
      return true
    } catch (requestError) {
      setError(getTaskMutationErrorMessage(requestError, '任务取消失败，请稍后重试。'))
      return false
    } finally {
      setPendingTaskAction(null)
    }
  }, [csrfToken, pendingTaskAction, taskApi])

  const retryCurrentTask = useCallback(async (): Promise<boolean> => {
    const task = currentTaskRef.current
    if (!task || !csrfToken || pendingTaskAction) {
      return false
    }

    setPendingTaskAction('retry')
    try {
      await taskApi.retry(task.id, csrfToken)
      if (!streamRef.current) {
        const initialState = taskStateRef.current ?? createTaskEventState(task.id, task.projectId)
        streamRef.current = connectTaskStream({
          initialState,
          lastEventId: taskStateRef.current?.lastEventId,
          shouldAcceptEvent: () => projectIdRef.current === task.projectId && currentTaskRef.current?.id === task.id,
          projectId: task.projectId,
          task,
          taskSseClientFactory,
          onEventState: (nextState) => {
            taskStateRef.current = nextState
            setTaskState(nextState)
          },
        })
      }
      return true
    } catch (requestError) {
      setError(getTaskMutationErrorMessage(requestError, '任务重试失败，请稍后重试。'))
      return false
    } finally {
      setPendingTaskAction(null)
    }
  }, [csrfToken, pendingTaskAction, taskApi, taskSseClientFactory])

  const selectCurrent = useCallback((index: number) => {
    setSelectedIndex(index)
  }, [])

  const downloadCurrentBackendAsset = useCallback(async () => {
    if (!current) {
      return null
    }

    try {
      return await assetApi.download(current.result.assetId)
    } catch (requestError) {
      setError(getTaskMutationErrorMessage(requestError, '结果下载失败，请稍后重试。'))
      return null
    }
  }, [assetApi, current])

  return {
    canCancelCurrentTask: canCancelTask(taskState?.status),
    canRetryCurrentTask: canRetryTask(taskState?.status),
    cancelCurrentTask,
    current,
    currentItems,
    currentTask,
    downloadCurrentBackendAsset,
    error,
    generateTask,
    pendingTaskAction,
    retryCurrentTask,
    selectedIndex,
    selectCurrent,
    status,
    taskState,
  }
}

function connectTaskStream({
  initialState,
  lastEventId,
  onEventState,
  projectId,
  shouldAcceptEvent,
  task,
  taskSseClientFactory,
}: {
  initialState: TaskEventState
  lastEventId?: TaskEventState['lastEventId']
  onEventState: (state: TaskEventState) => void
  projectId: ProjectId
  shouldAcceptEvent: () => boolean
  task: Task
  taskSseClientFactory: (options: TaskSseClientOptions) => TaskSseClient
}): TaskSseClient {
  let latestState = initialState
  const client = taskSseClientFactory({
    lastEventId,
    onEvent: (event) => {
      if (!shouldAcceptEvent()) {
        return
      }

      latestState = reduceTaskEventState(latestState, event)
      onEventState(latestState)
    },
    projectId,
    taskId: task.id,
  })
  client.connect()
  return client
}

function buildTaskCreateRequest(prompt: string, workbenchInput: WorkbenchTaskInput) {
  return {
    type: workbenchInput.editSourceAssetId ? ('IMAGE_EDIT' as const) : ('IMAGE_GENERATION' as const),
    prompt,
    providerId: workbenchInput.providerId,
    modelId: workbenchInput.modelId,
    imageType: workbenchInput.imageType,
    ...(workbenchInput.referenceAssetIds.length > 0 ? { referenceAssetIds: workbenchInput.referenceAssetIds } : {}),
    ...(workbenchInput.editSourceAssetId ? { editSourceAssetId: workbenchInput.editSourceAssetId } : {}),
    parameters: workbenchInput.parameters,
  }
}

function createBackendCurrentGeneration(task: Task, output: ImageOutputPayload): BackendCurrentGeneration {
  return {
    kind: 'backend',
    outputIndex: output.outputIndex,
    task,
    result: {
      assetId: output.assetId,
      previewUrl: output.previewUrl,
      thumbnailUrl: output.thumbnailUrl,
      width: output.width,
      height: output.height,
      mimeType: output.mimeType,
      fileSize: output.sizeBytes,
    },
  }
}

function hasActiveTask(task: Task | null, state: TaskEventState | null): boolean {
  if (!task) {
    return false
  }

  return !state?.status || !isTerminalTaskStatus(state.status)
}

function isTerminalTaskStatus(status: TaskStatus): boolean {
  return status === 'SUCCEEDED' || status === 'FAILED' || status === 'CANCELLED' || status === 'TIMED_OUT'
}

function isTerminalSuccess(status: TaskStatus): boolean {
  return status === 'SUCCEEDED'
}

function isTerminalFailure(status: TaskStatus): boolean {
  return status === 'FAILED' || status === 'CANCELLED' || status === 'TIMED_OUT'
}

function canCancelTask(status?: TaskStatus): boolean {
  return status === 'QUEUED' || status === 'RUNNING' || status === 'RETRYING'
}

function canRetryTask(status?: TaskStatus): boolean {
  return status === 'FAILED' || status === 'CANCELLED' || status === 'TIMED_OUT'
}

function terminalStatusMessage(status: TaskStatus): string {
  switch (status) {
    case 'FAILED':
      return '任务执行失败。'
    case 'CANCELLED':
      return '任务已取消。'
    case 'TIMED_OUT':
      return '任务执行超时。'
    default:
      return ''
  }
}

function isStaleWorkbenchSelectionError(error: unknown): boolean {
  return isApiClientError(error) && error.status === 422
}

function getTaskCreateErrorMessage(error: unknown): string {
  if (isApiClientError(error)) {
    if (error.status === 401) {
      return '登录状态已失效，请重新登录。'
    }
    if (error.status === 403) {
      return '没有权限创建任务。'
    }
    if (error.status === 422) {
      return '当前模型或参数已失效，请刷新模型后重新选择。'
    }

    return error.message || '任务创建失败，请稍后重试。'
  }

  return '任务创建失败，请稍后重试。'
}

function getTaskMutationErrorMessage(error: unknown, fallback: string): string {
  if (isApiClientError(error)) {
    if (error.status === 401) {
      return '登录状态已失效，请重新登录。'
    }
    if (error.status === 403) {
      return '没有权限执行该任务操作。'
    }
    return error.message || fallback
  }

  return fallback
}
