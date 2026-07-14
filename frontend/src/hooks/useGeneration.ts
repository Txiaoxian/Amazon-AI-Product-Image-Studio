import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
import type { AssetId, ProjectId, Task, TaskId, TaskStatus } from '../types/platform'
import type { ImageOutputPayload, TaskSseEvent } from '../types/sse'
import { normalizeWorkbenchImageType, type WorkbenchImageType } from '../types/workbench'
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

export interface ManagedGenerationTask {
  task: Task
  state: TaskEventState
  items: WorkbenchGeneration[]
}

export interface GenerationNotification {
  id: string
  taskId: TaskId
  projectId: ProjectId
  imageType: WorkbenchImageType
  errorCode?: string
  status: Extract<TaskStatus, 'SUCCEEDED' | 'FAILED' | 'CANCELLED' | 'TIMED_OUT'>
}

interface ManagedTaskRecord {
  task: Task
  state: TaskEventState
  selectedIndex: number
}

export interface UseGenerationOptions {
  assetApi?: AssetApi
  csrfToken?: string
  imageType?: WorkbenchImageType
  onModelInvalidated?: () => void | Promise<void>
  projectId?: ProjectId | null
  taskApi?: TaskApi
  taskSseClientFactory?: (options: TaskSseClientOptions) => TaskSseClient
}

export function useGeneration({
  assetApi = defaultAssetApi,
  csrfToken,
  imageType,
  onModelInvalidated,
  projectId,
  taskApi = defaultTaskApi,
  taskSseClientFactory = createTaskSseClient,
}: UseGenerationOptions = {}) {
  const [records, setRecords] = useState<ManagedTaskRecord[]>([])
  const [selectedTaskId, setSelectedTaskId] = useState<TaskId | null>(null)
  const [requestError, setRequestError] = useState('')
  const [isSubmitting, setSubmitting] = useState(false)
  const [pendingAction, setPendingAction] = useState<{
    action: Exclude<PendingTaskAction, null>
    taskId: TaskId
  } | null>(null)
  const [notifications, setNotifications] = useState<GenerationNotification[]>([])
  const recordsRef = useRef<ManagedTaskRecord[]>([])
  const streamRef = useRef<TaskSseClient | null>(null)
  const submitLockRef = useRef(false)
  const bufferedEventsRef = useRef<Map<TaskId, TaskSseEvent[]>>(new Map())

  useEffect(() => {
    recordsRef.current = records
  }, [records])

  useEffect(() => {
    return () => streamRef.current?.close()
  }, [])

  const contextRecords = useMemo(
    () =>
      records.filter(
        (record) =>
          record.task.projectId === projectId &&
          (!imageType || normalizeWorkbenchImageType(record.task.imageType) === imageType),
      ),
    [imageType, projectId, records],
  )
  const currentRecord =
    contextRecords.find((record) => record.task.id === selectedTaskId) ??
    contextRecords[contextRecords.length - 1] ??
    null
  const currentTask = currentRecord?.task ?? null
  const taskState = currentRecord?.state ?? null
  const currentItems = useMemo(() => createTaskItems(currentRecord), [currentRecord])
  const selectedIndex = Math.min(currentRecord?.selectedIndex ?? 0, Math.max(currentItems.length - 1, 0))
  const current = currentItems[selectedIndex] ?? null
  const status = getGenerationStatus(currentRecord, isSubmitting)
  const error = getGenerationError(currentRecord, requestError)
  const tasks = useMemo<ManagedGenerationTask[]>(
    () =>
      records.map((record) => ({
        task: record.task,
        state: record.state,
        items: createTaskItems(record),
      })),
    [records],
  )

  const handleGlobalTaskEvent = useCallback((event: TaskSseEvent) => {
    const taskId = getEventTaskId(event)
    if (!taskId) {
      return
    }

    const currentRecords = recordsRef.current
    const recordIndex = currentRecords.findIndex((record) => record.task.id === taskId)
    if (recordIndex === -1) {
      if (!bufferedEventsRef.current.has(taskId) && bufferedEventsRef.current.size >= MAX_BUFFERED_TASKS) {
        const oldestTaskId = bufferedEventsRef.current.keys().next().value
        if (oldestTaskId) {
          bufferedEventsRef.current.delete(oldestTaskId)
        }
      }
      const bufferedEvents = bufferedEventsRef.current.get(taskId) ?? []
      bufferedEventsRef.current.set(taskId, [...bufferedEvents.slice(-9), event])
      return
    }

    const record = currentRecords[recordIndex]
    const nextState = reduceTaskEventState(record.state, event)
    const wasTerminal = Boolean(record.state.status && isTerminalTaskStatus(record.state.status))
    const nextRecords = [...currentRecords]
    nextRecords[recordIndex] = {
      ...record,
      state: nextState,
    }
    recordsRef.current = nextRecords
    setRecords(nextRecords)

    const terminalStatus = nextState.status
    if (!wasTerminal && terminalStatus && isTerminalTaskStatus(terminalStatus)) {
      setNotifications((currentNotifications) => [
        ...currentNotifications,
        {
          id: event.id ?? `${record.task.id}:${terminalStatus}`,
          errorCode: nextState.errorCode,
          imageType: normalizeWorkbenchImageType(record.task.imageType),
          projectId: record.task.projectId,
          status: terminalStatus,
          taskId: record.task.id,
        },
      ])
    }
  }, [])

  const ensureGlobalTaskStream = useCallback(() => {
    if (streamRef.current) {
      return
    }

    const client = taskSseClientFactory({
      onEvent: handleGlobalTaskEvent,
    })
    client.connect()
    streamRef.current = client
  }, [handleGlobalTaskEvent, taskSseClientFactory])

  useEffect(() => {
    if (!csrfToken || !projectId) {
      return
    }

    let isCancelled = false
    if (taskSseClientFactory !== createTaskSseClient || typeof EventSource !== 'undefined') {
      ensureGlobalTaskStream()
    }

    const restoreActiveTasks = async () => {
      try {
        const pages = await Promise.all(
          ACTIVE_TASK_STATUSES.map((status) => taskApi.list(projectId, { pageNum: 1, pageSize: 100, status })),
        )
        if (isCancelled) {
          return
        }

        const restoredTasks = uniqueTasks(pages.flatMap((page) => page.records))
        const nextRecords = mergeRestoredTasks(recordsRef.current, restoredTasks)
        recordsRef.current = nextRecords
        setRecords(nextRecords)

        if (restoredTasks.length > 0) {
          ensureGlobalTaskStream()
        }
        restoredTasks.forEach((task) => {
          const bufferedEvents = bufferedEventsRef.current.get(task.id) ?? []
          bufferedEventsRef.current.delete(task.id)
          bufferedEvents.forEach(handleGlobalTaskEvent)
        })
      } catch (requestFailure) {
        if (!isCancelled) {
          setRequestError(getTaskRestoreErrorMessage(requestFailure))
        }
      }
    }

    void restoreActiveTasks()
    return () => {
      isCancelled = true
    }
  }, [csrfToken, ensureGlobalTaskStream, handleGlobalTaskEvent, projectId, taskApi, taskSseClientFactory])

  const generateTask = useCallback(
    async (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput): Promise<Task | null> => {
      if (submitLockRef.current) {
        return null
      }
      if (!projectId) {
        setRequestError('请先选择产品。')
        return null
      }
      if (!csrfToken) {
        setRequestError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      const prompt = request.prompt.trim()
      if (!prompt) {
        setRequestError('请输入图片生成提示词。')
        return null
      }

      submitLockRef.current = true
      setSubmitting(true)
      setRequestError('')

      try {
        const responseTask = await taskApi.create(projectId, buildTaskCreateRequest(prompt, workbenchInput), csrfToken)
        const task: Task = {
          ...responseTask,
          imageType: workbenchInput.imageType,
          prompt,
        }
        const initialState = {
          ...createTaskEventState(task.id, projectId),
          attempt: task.attempt,
          queuedAt: task.queuedAt ?? undefined,
          status: task.status,
        }
        const record: ManagedTaskRecord = {
          selectedIndex: 0,
          state: initialState,
          task,
        }
        const nextRecords = [...recordsRef.current.filter((candidate) => candidate.task.id !== task.id), record]
        recordsRef.current = nextRecords
        setRecords(nextRecords)
        setSelectedTaskId(task.id)
        ensureGlobalTaskStream()
        return task
      } catch (requestFailure) {
        setRequestError(getTaskCreateErrorMessage(requestFailure))
        if (isStaleWorkbenchSelectionError(requestFailure)) {
          await onModelInvalidated?.()
        }
        return null
      } finally {
        submitLockRef.current = false
        setSubmitting(false)
      }
    },
    [csrfToken, ensureGlobalTaskStream, onModelInvalidated, projectId, taskApi],
  )

  const cancelTask = useCallback(
    async (taskId: TaskId): Promise<boolean> => {
      if (!csrfToken || pendingAction) {
        return false
      }

      setPendingAction({ action: 'cancel', taskId })
      try {
        await taskApi.cancel(taskId, csrfToken)
        return true
      } catch (requestFailure) {
        setRequestError(getTaskMutationErrorMessage(requestFailure, '任务取消失败，请稍后重试。'))
        return false
      } finally {
        setPendingAction(null)
      }
    },
    [csrfToken, pendingAction, taskApi],
  )

  const retryTask = useCallback(
    async (taskId: TaskId): Promise<boolean> => {
      if (!csrfToken || pendingAction) {
        return false
      }

      setPendingAction({ action: 'retry', taskId })
      try {
        await taskApi.retry(taskId, csrfToken)
        ensureGlobalTaskStream()
        return true
      } catch (requestFailure) {
        setRequestError(getTaskMutationErrorMessage(requestFailure, '任务重试失败，请稍后重试。'))
        return false
      } finally {
        setPendingAction(null)
      }
    },
    [csrfToken, ensureGlobalTaskStream, pendingAction, taskApi],
  )

  const selectCurrent = useCallback(
    (index: number) => {
      if (!currentTask) {
        return
      }
      const nextRecords = recordsRef.current.map((record) =>
        record.task.id === currentTask.id ? { ...record, selectedIndex: index } : record,
      )
      recordsRef.current = nextRecords
      setRecords(nextRecords)
    },
    [currentTask],
  )

  const selectTask = useCallback((taskId: TaskId) => setSelectedTaskId(taskId), [])
  const dismissNotification = useCallback(
    (notificationId: string) =>
      setNotifications((current) => current.filter((notification) => notification.id !== notificationId)),
    [],
  )

  const downloadCurrentBackendAsset = useCallback(async () => {
    if (!current) {
      return null
    }

    try {
      return await assetApi.download(current.result.assetId)
    } catch (requestFailure) {
      setRequestError(getTaskMutationErrorMessage(requestFailure, '结果下载失败，请稍后重试。'))
      return null
    }
  }, [assetApi, current])

  return {
    activeTaskCount: records.filter((record) => !record.state.status || !isTerminalTaskStatus(record.state.status))
      .length,
    canCancelCurrentTask: canCancelTask(taskState?.status),
    canRetryCurrentTask: canRetryTask(taskState?.status),
    cancelCurrentTask: () => (currentTask ? cancelTask(currentTask.id) : Promise.resolve(false)),
    cancelTask,
    current,
    currentItems,
    currentTask,
    dismissNotification,
    downloadCurrentBackendAsset,
    error,
    generateTask,
    isSubmitting,
    notifications,
    pendingTaskAction: currentTask && pendingAction?.taskId === currentTask.id ? pendingAction.action : null,
    pendingAction,
    retryCurrentTask: () => (currentTask ? retryTask(currentTask.id) : Promise.resolve(false)),
    retryTask,
    selectedIndex,
    selectCurrent,
    selectedTaskId,
    selectTask,
    status,
    tasks,
    taskState,
  }
}

function getEventTaskId(event: TaskSseEvent): TaskId | undefined {
  return 'taskId' in event.data ? event.data.taskId : undefined
}

const ACTIVE_TASK_STATUSES = ['QUEUED', 'RUNNING', 'RETRYING'] as const satisfies readonly TaskStatus[]
const MAX_BUFFERED_TASKS = 100

function uniqueTasks(tasks: Task[]): Task[] {
  return [...new Map(tasks.map((task) => [task.id, task])).values()]
}

function mergeRestoredTasks(records: ManagedTaskRecord[], tasks: Task[]): ManagedTaskRecord[] {
  const recordsByTaskId = new Map(records.map((record) => [record.task.id, record]))

  tasks.forEach((task) => {
    const existingRecord = recordsByTaskId.get(task.id)
    if (existingRecord?.state.status && isTerminalTaskStatus(existingRecord.state.status)) {
      return
    }

    recordsByTaskId.set(task.id, {
      selectedIndex: existingRecord?.selectedIndex ?? 0,
      state: {
        ...createTaskEventState(task.id, task.projectId),
        ...existingRecord?.state,
        attempt: task.attempt,
        errorCode: task.errorCode || undefined,
        errorMessage: task.errorMessage || undefined,
        finishedAt: task.finishedAt ?? undefined,
        queuedAt: task.queuedAt ?? undefined,
        startedAt: task.startedAt ?? undefined,
        status: task.status,
      },
      task,
    })
  })

  return [...recordsByTaskId.values()].sort((left, right) => left.task.createdAt.localeCompare(right.task.createdAt))
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

function createTaskItems(record: ManagedTaskRecord | null): WorkbenchGeneration[] {
  if (!record) {
    return []
  }

  return record.state.outputs
    .slice()
    .sort((left, right) => left.outputIndex - right.outputIndex)
    .map((output) => createBackendCurrentGeneration(record.task, output))
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

function getGenerationStatus(record: ManagedTaskRecord | null, isSubmitting: boolean): GenerationStatus {
  if (!record) {
    return isSubmitting ? 'loading' : 'idle'
  }
  if (record.state.status === 'SUCCEEDED') {
    return 'success'
  }
  if (record.state.status && isTerminalFailure(record.state.status)) {
    return 'error'
  }
  return 'loading'
}

function getGenerationError(record: ManagedTaskRecord | null, requestError: string): string {
  if (requestError) {
    return requestError
  }
  if (!record?.state.status || !isTerminalFailure(record.state.status)) {
    return ''
  }
  if (record.state.errorCode === 'PROVIDER_INSUFFICIENT_QUOTA') {
    return '中转站余额不足，请充值后再生成。'
  }
  return terminalStatusMessage(record.state.status)
}

function isTerminalTaskStatus(status: TaskStatus): status is GenerationNotification['status'] {
  return status === 'SUCCEEDED' || status === 'FAILED' || status === 'CANCELLED' || status === 'TIMED_OUT'
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
      return '任务执行失败，请检查模型配置或稍后重试。'
    case 'CANCELLED':
      return '任务已取消。'
    case 'TIMED_OUT':
      return '任务执行超时，请稍后重试。'
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

function getTaskRestoreErrorMessage(error: unknown): string {
  if (isApiClientError(error)) {
    if (error.status === 401) {
      return '登录状态已失效，请重新登录。'
    }
    if (error.status === 403) {
      return '没有权限读取正在生成的任务。'
    }
    return error.message || '正在生成的任务恢复失败，请刷新页面重试。'
  }
  return '正在生成的任务恢复失败，请检查网络后刷新页面重试。'
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
