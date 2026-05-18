import { useCallback, useEffect, useRef, useState } from 'react'
import { assetApi as defaultAssetApi, type AssetApi } from '../api/assets'
import { isApiClientError } from '../api/client'
import { taskApi as defaultTaskApi, type TaskApi } from '../api/tasks'
import { clearHistory, deleteHistoryItem, listHistory, type HistoryWithImage } from '../db/historyRepository'
import { FriendlyError, toFriendlyError } from '../lib/errors'
import type { BackendHistoryItem } from '../types/history'
import type { Asset, AssetId, ProjectId, Task, TaskId } from '../types/platform'

interface UseHistoryOptions {
  assetApi?: AssetApi
  projectId?: ProjectId | null
  taskApi?: TaskApi
}

export function useHistory({
  assetApi = defaultAssetApi,
  projectId = null,
  taskApi = defaultTaskApi,
}: UseHistoryOptions = {}) {
  const [items, setItems] = useState<BackendHistoryItem[]>([])
  const [legacyItems, setLegacyItems] = useState<HistoryWithImage[]>([])
  const [error, setError] = useState('')
  const [legacyError, setLegacyError] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [isLegacyLoading, setLegacyLoading] = useState(true)
  const refreshVersionRef = useRef(0)

  const refresh = useCallback(async () => {
    const refreshVersion = refreshVersionRef.current + 1
    refreshVersionRef.current = refreshVersion

    if (!projectId) {
      setItems([])
      setError('')
      setIsLoading(false)
      return
    }

    setIsLoading(true)
    setError('')
    try {
      const [tasksPage, generatedPage, editedPage] = await Promise.all([
        taskApi.list(projectId, { pageNum: 1, pageSize: 50 }),
        assetApi.list(projectId, { kind: 'GENERATED', pageNum: 1, pageSize: 50 }),
        assetApi.list(projectId, { kind: 'EDITED', pageNum: 1, pageSize: 50 }),
      ])
      if (refreshVersion !== refreshVersionRef.current) {
        return
      }
      setItems(buildBackendHistoryItems(projectId, tasksPage.records, [...generatedPage.records, ...editedPage.records]))
    } catch (err) {
      if (refreshVersion !== refreshVersionRef.current) {
        return
      }
      setItems([])
      setError(getBackendHistoryErrorMessage(err, '无法加载项目历史，请稍后重试。'))
    } finally {
      if (refreshVersion === refreshVersionRef.current) {
        setIsLoading(false)
      }
    }
  }, [assetApi, projectId, taskApi])

  const refreshLegacy = useCallback(async () => {
    setLegacyLoading(true)
    setLegacyError('')
    try {
      setLegacyItems(await listHistory())
    } catch (err) {
      setLegacyError(toFriendlyError(err).message)
    } finally {
      setLegacyLoading(false)
    }
  }, [])

  const remove = useCallback(
    async (id: string) => {
      await deleteHistoryItem(id)
      await refreshLegacy()
    },
    [refreshLegacy],
  )

  const clear = useCallback(async () => {
    await clearHistory()
    await refreshLegacy()
  }, [refreshLegacy])

  const loadBackendDetail = useCallback(
    async (assetId: AssetId, taskId?: TaskId): Promise<{ asset: Asset; task: Task }> => {
      if (!projectId) {
        throw new FriendlyError('无法读取该结果，可能已被删除或无权访问。', 'BACKEND_RESULT_UNAVAILABLE')
      }

      try {
        const asset = await assetApi.get(assetId)
        const resolvedTaskId = taskId ?? asset.taskId
        if (!resolvedTaskId) {
          throw new FriendlyError('无法读取该结果，可能已被删除或无权访问。', 'BACKEND_RESULT_UNAVAILABLE')
        }
        const task = await taskApi.get(resolvedTaskId)

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
        const asset = await assetApi.get(assetId)
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

  useEffect(() => {
    void refreshLegacy()
  }, [refreshLegacy])

  return {
    downloadBackendAsset,
    ensureBackendAssetAvailable,
    items,
    error,
    isLoading,
    isLegacyLoading,
    legacyError,
    legacyItems,
    loadBackendDetail,
    refresh,
    refreshLegacy,
    remove,
    clear,
  }
}

function buildBackendHistoryItems(projectId: ProjectId, tasks: Task[], assets: Asset[]): BackendHistoryItem[] {
  const tasksById = new Map(tasks.filter((task) => task.projectId === projectId).map((task) => [task.id, task]))

  return assets
    .filter((asset) => asset.projectId === projectId && (asset.kind === 'GENERATED' || asset.kind === 'EDITED'))
    .flatMap((asset) => {
      const task = asset.taskId
        ? tasksById.get(asset.taskId)
        : tasks.find((candidate) => candidate.projectId === projectId && candidate.outputAssetIds.includes(asset.id))

      return task ? [{ asset, task }] : []
    })
    .sort((left, right) => Date.parse(right.asset.createdAt) - Date.parse(left.asset.createdAt))
}

function isVisibleBackendDetail(projectId: ProjectId, asset: Asset, task: Task): boolean {
  if (asset.projectId !== projectId || task.projectId !== projectId) {
    return false
  }

  return asset.taskId === task.id || task.outputAssetIds.includes(asset.id)
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
