import { useCallback, useEffect, useRef, useState } from 'react'
import { assetApi as defaultAssetApi, type AssetApi } from '../api/assets'
import { isApiClientError } from '../api/client'
import { taskApi as defaultTaskApi, type TaskApi } from '../api/tasks'
import { FriendlyError } from '../lib/errors'
import type { BackendHistoryItem, HistoryKind } from '../types/history'
import type { Asset, AssetId, ProjectId, Task, TaskId } from '../types/platform'

interface UseHistoryOptions {
  assetApi?: AssetApi
  projectId?: ProjectId | null
  taskApi?: TaskApi
}

const DEFAULT_HISTORY_PAGE_SIZE = 10

export function useHistory({
  assetApi = defaultAssetApi,
  projectId = null,
  taskApi = defaultTaskApi,
}: UseHistoryOptions = {}) {
  const [items, setItems] = useState<BackendHistoryItem[]>([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [pageNum, setPageNum] = useState(1)
  const [pageSize, setPageSizeState] = useState(DEFAULT_HISTORY_PAGE_SIZE)
  const [kind, setKindState] = useState<HistoryKind | undefined>(undefined)
  const [total, setTotal] = useState(0)
  const refreshVersionRef = useRef(0)
  const projectIdRef = useRef<ProjectId | null | undefined>(undefined)

  useEffect(() => {
    if (projectIdRef.current === projectId) {
      return
    }

    projectIdRef.current = projectId
    refreshVersionRef.current += 1
    setItems([])
    setError('')
    setTotal(0)
    setIsLoading(false)
    setPageNum(1)
  }, [projectId])

  const refresh = useCallback(async () => {
    const refreshVersion = refreshVersionRef.current + 1
    refreshVersionRef.current = refreshVersion

    if (!projectId) {
      setItems([])
      setError('')
      setTotal(0)
      setIsLoading(false)
      return
    }

    setIsLoading(true)
    setError('')
    setItems([])
    try {
      const historyPage = await taskApi.listHistory(projectId, { pageNum, pageSize, kind })
      if (refreshVersion !== refreshVersionRef.current) {
        return
      }
      setItems(historyPage.records.map(sanitizeBackendHistoryItem).filter((item) => isVisibleHistoryItem(projectId, item)))
      setTotal(historyPage.total)
    } catch (err) {
      if (refreshVersion !== refreshVersionRef.current) {
        return
      }
      setItems([])
      setTotal(0)
      setError(getBackendHistoryErrorMessage(err, '无法加载项目历史，请稍后重试。'))
    } finally {
      if (refreshVersion === refreshVersionRef.current) {
        setIsLoading(false)
      }
    }
  }, [kind, pageNum, pageSize, projectId, taskApi])

  const loadBackendDetail = useCallback(
    async (assetId: AssetId, taskId?: TaskId): Promise<{ asset: Asset; task: Task }> => {
      if (!projectId) {
        throw new FriendlyError('无法读取该结果，可能已被删除或无权访问。', 'BACKEND_RESULT_UNAVAILABLE')
      }

      try {
        const asset = sanitizeAsset(await assetApi.get(assetId))
        const resolvedTaskId = taskId ?? asset.taskId
        if (!resolvedTaskId) {
          throw new FriendlyError('无法读取该结果，可能已被删除或无权访问。', 'BACKEND_RESULT_UNAVAILABLE')
        }
        const task = sanitizeTask(await taskApi.get(resolvedTaskId))

        if (!isVisibleBackendDetail(projectId, asset, task)) {
          throw new FriendlyError('无法读取该结果，可能已被删除或无权访问。', 'BACKEND_RESULT_UNAVAILABLE')
        }

        return { asset, task }
      } catch (err) {
        if (err instanceof FriendlyError) {
          throw err
        }

        throw new FriendlyError(getBackendHistoryErrorMessage(err, '无法读取该结果，可能已被删除或无权访问。'), 'BACKEND_RESULT_UNAVAILABLE')
      }
    },
    [assetApi, projectId, taskApi],
  )

  const ensureBackendAssetAvailable = useCallback(
    async (assetId: AssetId): Promise<boolean> => {
      if (!projectId) {
        return false
      }

      try {
        const asset = sanitizeAsset(await assetApi.get(assetId))
        return asset.projectId === projectId
      } catch {
        return false
      }
    },
    [assetApi, projectId],
  )

  const downloadBackendAsset = useCallback(
    async (asset: Asset) => {
      if (!projectId || asset.projectId !== projectId) {
        return null
      }

      try {
        return await assetApi.download(asset.id)
      } catch {
        return null
      }
    },
    [assetApi, projectId],
  )

  useEffect(() => {
    void refresh()
  }, [refresh])

  const setKind = useCallback((nextKind: HistoryKind | undefined) => {
    setKindState(nextKind)
    setPageNum(1)
  }, [])

  const setPageSize = useCallback((nextPageSize: number) => {
    setPageSizeState(nextPageSize)
    setPageNum(1)
  }, [])

  return {
    downloadBackendAsset,
    ensureBackendAssetAvailable,
    items,
    error,
    isLoading,
    kind,
    loadBackendDetail,
    pageNum,
    pageSize,
    refresh,
    setKind,
    setPageNum,
    setPageSize,
    total,
  }
}

function sanitizeBackendHistoryItem(item: BackendHistoryItem): BackendHistoryItem {
  return {
    asset: sanitizeAsset(item.asset),
    task: sanitizeTask(item.task),
  }
}

function sanitizeAsset(asset: Asset): Asset {
  const previewUrl = backendAssetDownloadUrl(asset.id)

  return {
    id: asset.id,
    tenantId: asset.tenantId,
    projectId: asset.projectId,
    taskId: asset.taskId,
    kind: asset.kind,
    category: asset.category,
    filename: asset.filename,
    mimeType: asset.mimeType,
    fileSize: asset.fileSize,
    width: asset.width,
    height: asset.height,
    thumbnailUrl: previewUrl,
    previewUrl,
    downloadUrl: undefined,
    isFavorite: asset.isFavorite,
    createdBy: asset.createdBy,
    createdAt: asset.createdAt,
    updatedAt: asset.updatedAt,
  }
}

function sanitizeTask(task: Task): Task {
  return {
    id: task.id,
    tenantId: task.tenantId,
    projectId: task.projectId,
    type: task.type,
    status: task.status,
    prompt: task.prompt,
    providerId: task.providerId,
    modelId: task.modelId,
    imageType: task.imageType,
    parameters: sanitizeTaskParameters(task.parameters),
    inputAssetIds: task.inputAssetIds,
    outputAssetIds: task.outputAssetIds,
    attempt: task.attempt,
    maxAttempts: task.maxAttempts,
    queuedAt: task.queuedAt,
    startedAt: task.startedAt,
    finishedAt: task.finishedAt,
    timeoutAt: task.timeoutAt,
    errorCode: task.errorCode,
    errorMessage: task.errorMessage,
    createdBy: task.createdBy,
    createdAt: task.createdAt,
    updatedAt: task.updatedAt,
  }
}

function backendAssetDownloadUrl(assetId: AssetId): string {
  return `/api/v1/assets/${encodeURIComponent(assetId)}/download`
}

function sanitizeTaskParameters(parameters: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(parameters).filter(([key]) => !isUnsafeHistoryParameterKey(key)))
}

function isUnsafeHistoryParameterKey(key: string): boolean {
  const normalizedKey = key.toLowerCase()
  return (
    normalizedKey === 'apicall' ||
    normalizedKey.includes('authorization') ||
    normalizedKey.includes('base64') ||
    normalizedKey.includes('cookie') ||
    normalizedKey.includes('redacted') ||
    (normalizedKey.includes('object') && normalizedKey.includes('key'))
  )
}

function isVisibleBackendDetail(projectId: ProjectId, asset: Asset, task: Task): boolean {
  if (asset.projectId !== projectId || task.projectId !== projectId) {
    return false
  }

  return asset.taskId === task.id || task.outputAssetIds.includes(asset.id)
}

function isVisibleHistoryItem(projectId: ProjectId, item: BackendHistoryItem): boolean {
  return item.asset.projectId === projectId && item.task.projectId === projectId && (item.asset.kind === 'GENERATED' || item.asset.kind === 'EDITED')
}

function getBackendHistoryErrorMessage(error: unknown, fallback: string): string {
  if (!isApiClientError(error)) {
    return fallback
  }

  if (error.status === 401) {
    return '登录状态已失效，请重新登录。'
  }

  if (error.status === 403) {
    return '没有权限读取该项目历史。'
  }

  if (error.status === 404) {
    return '无法读取该结果，可能已被删除或无权访问。'
  }

  return fallback
}
