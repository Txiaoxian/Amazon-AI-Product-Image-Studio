import { useCallback, useEffect, useMemo, useState } from 'react'
import { assetApi as defaultAssetApi, type AssetApi } from '../api/assets'
import { isApiClientError } from '../api/client'
import { projectApi as defaultProjectApi, type CreateProjectRequest, type ProjectApi } from '../api/projects'
import { validateImageFile } from '../lib/file'
import type { Asset, AssetId, Project, ProjectId } from '../types/platform'
import type { AssetReferenceInput } from '../types/workbench'

type RemoteStatus = 'idle' | 'loading' | 'success' | 'error'

interface UseProjectAssetsOptions {
  assetApi?: AssetApi
  csrfToken?: string
  projectApi?: ProjectApi
}

interface UploadReferenceResult {
  assets: Asset[]
  skipped: number
}

export function useProjectAssets({
  assetApi = defaultAssetApi,
  csrfToken,
  projectApi = defaultProjectApi,
}: UseProjectAssetsOptions = {}) {
  const [projects, setProjects] = useState<Project[]>([])
  const [selectedProjectId, setSelectedProjectId] = useState<ProjectId | null>(null)
  const [assets, setAssets] = useState<Asset[]>([])
  const [projectStatus, setProjectStatus] = useState<RemoteStatus>('idle')
  const [assetStatus, setAssetStatus] = useState<RemoteStatus>('idle')
  const [error, setError] = useState<string | null>(null)
  const [isCreatingProject, setCreatingProject] = useState(false)
  const [isUploadingAsset, setUploadingAsset] = useState(false)
  const [actionAssetId, setActionAssetId] = useState<AssetId | null>(null)

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? null,
    [projects, selectedProjectId],
  )

  const refreshProjects = useCallback(async () => {
    setProjectStatus('loading')
    setError(null)

    try {
      const page = await projectApi.list({ status: 'ACTIVE', pageNum: 1, pageSize: 50 })
      setProjects(page.records)
      setSelectedProjectId((current) => {
        if (current && page.records.some((project) => project.id === current)) {
          return current
        }

        return page.records[0]?.id ?? null
      })
      setProjectStatus('success')
    } catch (requestError) {
      setProjectStatus('error')
      setError(getProjectAssetErrorMessage(requestError, '无法加载项目列表，请稍后重试。'))
    }
  }, [projectApi])

  const refreshAssets = useCallback(
    async (projectId: ProjectId | null = selectedProjectId) => {
      if (!projectId) {
        setAssets([])
        setAssetStatus('idle')
        return
      }

      setAssetStatus('loading')
      setError(null)

      try {
        const page = await assetApi.list(projectId, { pageNum: 1, pageSize: 50 })
        setAssets(page.records)
        setAssetStatus('success')
      } catch (requestError) {
        setAssetStatus('error')
        setError(getProjectAssetErrorMessage(requestError, '无法加载项目资产，请稍后重试。'))
      }
    },
    [assetApi, selectedProjectId],
  )

  useEffect(() => {
    void refreshProjects()
  }, [refreshProjects])

  useEffect(() => {
    void refreshAssets(selectedProjectId)
  }, [refreshAssets, selectedProjectId])

  const createProject = useCallback(
    async (request: CreateProjectRequest): Promise<Project | null> => {
      const name = request.name.trim()
      if (!name) {
        setError('请输入项目名称。')
        return null
      }
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      setCreatingProject(true)
      setError(null)

      try {
        const project = await projectApi.create({ ...request, name }, csrfToken)
        setProjects((current) => [project, ...current.filter((item) => item.id !== project.id)])
        setSelectedProjectId(project.id)
        setAssets([])
        setAssetStatus('idle')
        return project
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '项目创建失败，请稍后重试。'))
        return null
      } finally {
        setCreatingProject(false)
      }
    },
    [csrfToken, projectApi],
  )

  const selectProject = useCallback((projectId: ProjectId) => {
    setSelectedProjectId(projectId)
    setError(null)
  }, [])

  const uploadReferences = useCallback(
    async (files: File[] | FileList): Promise<UploadReferenceResult> => {
      if (!selectedProjectId) {
        setError('请先选择项目。')
        return { assets: [], skipped: 0 }
      }
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return { assets: [], skipped: 0 }
      }

      const acceptedFiles: File[] = []
      let skipped = 0

      Array.from(files).forEach((file) => {
        try {
          validateImageFile(file)
          acceptedFiles.push(file)
        } catch (validationError) {
          skipped += 1
          setError(validationError instanceof Error ? validationError.message : '参考图无效。')
        }
      })

      if (acceptedFiles.length === 0) {
        return { assets: [], skipped }
      }

      setUploadingAsset(true)
      setError(null)

      try {
        const uploadedAssets: Asset[] = []
        for (const file of acceptedFiles) {
          const uploaded = await assetApi.uploadReference(
            selectedProjectId,
            {
              file,
              category: 'reference',
              filename: file.name,
            },
            csrfToken,
          )
          uploadedAssets.push(uploaded)
        }

        setAssets((current) => [...uploadedAssets, ...current.filter((asset) => !uploadedAssets.some((item) => item.id === asset.id))])
        setAssetStatus('success')
        return { assets: uploadedAssets, skipped }
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '参考图上传失败，请稍后重试。'))
        return { assets: [], skipped }
      } finally {
        setUploadingAsset(false)
      }
    },
    [assetApi, csrfToken, selectedProjectId],
  )

  const toggleFavorite = useCallback(
    async (asset: Asset): Promise<Asset | null> => {
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      setActionAssetId(asset.id)
      setError(null)

      try {
        const updated = asset.isFavorite
          ? await assetApi.unfavorite(asset.id, csrfToken)
          : await assetApi.favorite(asset.id, csrfToken)
        setAssets((current) => current.map((item) => (item.id === updated.id ? updated : item)))
        return updated
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '收藏状态更新失败，请稍后重试。'))
        return null
      } finally {
        setActionAssetId(null)
      }
    },
    [assetApi, csrfToken],
  )

  const deleteAsset = useCallback(
    async (asset: Asset): Promise<boolean> => {
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return false
      }

      setActionAssetId(asset.id)
      setError(null)

      try {
        await assetApi.delete(asset.id, csrfToken)
        setAssets((current) => current.filter((item) => item.id !== asset.id))
        return true
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '资产删除失败，请稍后重试。'))
        return false
      } finally {
        setActionAssetId(null)
      }
    },
    [assetApi, csrfToken],
  )

  const downloadAsset = useCallback(
    async (asset: Asset) => {
      setActionAssetId(asset.id)
      setError(null)

      try {
        return await assetApi.download(asset.id)
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '资产下载失败，请稍后重试。'))
        return null
      } finally {
        setActionAssetId(null)
      }
    },
    [assetApi],
  )

  const createReferenceFromAsset = useCallback(
    (asset: Asset): AssetReferenceInput => {
      return {
        kind: 'asset',
        assetId: asset.id,
        filename: asset.filename,
        previewUrl: asset.previewUrl || asset.thumbnailUrl || `/api/v1/assets/${encodeURIComponent(asset.id)}/download`,
      }
    },
    [],
  )

  return {
    actionAssetId,
    assetStatus,
    assets,
    createProject,
    createReferenceFromAsset,
    deleteAsset,
    downloadAsset,
    error,
    isCreatingProject,
    isLoadingAssets: assetStatus === 'loading',
    isLoadingProjects: projectStatus === 'loading',
    isUploadingAsset,
    projectStatus,
    projects,
    refreshAssets,
    refreshProjects,
    selectProject,
    selectedProject,
    selectedProjectId,
    toggleFavorite,
    uploadReferences,
  }
}

function getProjectAssetErrorMessage(error: unknown, fallback: string): string {
  if (!isApiClientError(error)) {
    return fallback
  }

  if (error.status === 401) {
    return '登录状态已失效，请重新登录。'
  }

  if (error.status === 403) {
    return '没有权限执行该项目资产操作。'
  }

  if (error.status === 404) {
    return '项目或资产不存在，可能已被删除或无权访问。'
  }

  if (error.status === 422) {
    return '请求内容未通过校验，请检查项目名称或图片文件。'
  }

  return error.message || fallback
}
