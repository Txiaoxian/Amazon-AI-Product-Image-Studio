import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { assetApi as defaultAssetApi, type AssetApi, type ListAssetsParams, type UpdateAssetRequest } from '../api/assets'
import { isApiClientError } from '../api/client'
import {
  projectApi as defaultProjectApi,
  type CreateProjectRequest,
  type ListProjectMemberCandidatesParams,
  type ProjectApi,
  type ProjectMemberRequest,
  type UpdateProjectMemberRequest,
  type UpdateProjectRequest,
} from '../api/projects'
import { validateImageFile } from '../lib/file'
import type { Asset, AssetId, AssetKind, Project, ProjectId, ProjectMember, ProjectMemberCandidate, UserId } from '../types/platform'
import {
  DEFAULT_WORKBENCH_IMAGE_TYPE,
  normalizeWorkbenchImageType,
  type AssetReferenceInput,
  type WorkbenchImageType,
} from '../types/workbench'

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

export interface AssetFilters {
  category?: string
  favorite?: boolean
  kind?: AssetKind
  imageType?: WorkbenchImageType
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
  const [projectMemberStatus, setProjectMemberStatus] = useState<RemoteStatus>('idle')
  const [error, setError] = useState<string | null>(null)
  const [projectMemberError, setProjectMemberError] = useState<string | null>(null)
  const [isCreatingProject, setCreatingProject] = useState(false)
  const [isDeletingProject, setDeletingProject] = useState(false)
  const [isUpdatingProject, setUpdatingProject] = useState(false)
  const [isUpdatingAsset, setUpdatingAsset] = useState(false)
  const [isSavingProjectMember, setSavingProjectMember] = useState(false)
  const [isUploadingAsset, setUploadingAsset] = useState(false)
  const [actionAssetId, setActionAssetId] = useState<AssetId | null>(null)
  const [projectMemberActionUserId, setProjectMemberActionUserId] = useState<UserId | string | null>(null)
  const [assetFilters, setAssetFilters] = useState<AssetFilters>({})
  const [projectMembers, setProjectMembers] = useState<ProjectMember[]>([])
  const [projectMemberCandidates, setProjectMemberCandidates] = useState<ProjectMemberCandidate[]>([])
  const [projectMemberCandidateStatus, setProjectMemberCandidateStatus] = useState<RemoteStatus>('idle')
  const assetRequestVersionRef = useRef(0)
  const projectMemberRequestVersionRef = useRef(0)
  const selectedProjectIdRef = useRef<ProjectId | null>(null)

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectId) ?? null,
    [projects, selectedProjectId],
  )

  useEffect(() => {
    selectedProjectIdRef.current = selectedProjectId
  }, [selectedProjectId])

  useEffect(() => {
    projectMemberRequestVersionRef.current += 1
    setProjectMembers([])
    setProjectMemberCandidates([])
    setProjectMemberStatus('idle')
    setProjectMemberCandidateStatus('idle')
    setProjectMemberError(null)
  }, [selectedProjectId])

  const refreshProjects = useCallback(async () => {
    setProjectStatus('loading')
    setError(null)

    try {
      const page = await projectApi.list({ status: 'ACTIVE', pageNum: 1, pageSize: 50 })
      const records = sortProjects(page.records)
      setProjects(records)
      setSelectedProjectId((current) => {
        if (current && records.some((project) => project.id === current)) {
          return current
        }

        return records[0]?.id ?? null
      })
      setProjectStatus('success')
    } catch (requestError) {
      setProjectStatus('error')
      setError(getProjectAssetErrorMessage(requestError, '产品加载失败，请刷新产品列表后重试。'))
    }
  }, [projectApi])

  const refreshAssets = useCallback(
    async (projectId: ProjectId | null = selectedProjectId, filters: AssetFilters = assetFilters) => {
      const requestVersion = assetRequestVersionRef.current + 1
      assetRequestVersionRef.current = requestVersion

      if (!projectId) {
        setAssets([])
        setAssetStatus('idle')
        return
      }

      setAssetStatus('loading')
      setError(null)

      try {
        const page = await assetApi.list(projectId, buildAssetListParams(filters))
        if (requestVersion !== assetRequestVersionRef.current || selectedProjectIdRef.current !== projectId) {
          return
        }
        setAssets(page.records)
        setAssetStatus('success')
      } catch (requestError) {
        if (requestVersion !== assetRequestVersionRef.current || selectedProjectIdRef.current !== projectId) {
          return
        }
        setAssetStatus('error')
        setError(getProjectAssetErrorMessage(requestError, '无法加载产品素材，请稍后重试。'))
      }
    },
    [assetApi, assetFilters, selectedProjectId],
  )

  useEffect(() => {
    void refreshProjects()
  }, [refreshProjects])

  useEffect(() => {
    void refreshAssets(selectedProjectId, assetFilters)
  }, [assetFilters, refreshAssets, selectedProjectId])

  const createProject = useCallback(
    async (request: CreateProjectRequest): Promise<Project | null> => {
      const name = request.name.trim()
      if (!name) {
        setError('请输入产品名称。')
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
        setProjects((current) => sortProjects([project, ...current.filter((item) => item.id !== project.id)]))
        setSelectedProjectId(project.id)
        setAssets([])
        setAssetStatus('idle')
        return project
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '产品创建失败，请稍后重试。'))
        return null
      } finally {
        setCreatingProject(false)
      }
    },
    [csrfToken, projectApi],
  )

  const selectProject = useCallback((projectId: ProjectId) => {
    selectedProjectIdRef.current = projectId
    assetRequestVersionRef.current += 1
    setSelectedProjectId(projectId)
    setAssets([])
    setAssetStatus('loading')
    setError(null)
  }, [])

  const updateAssetFilters = useCallback((filters: AssetFilters) => {
    setAssetFilters(normalizeAssetFilters(filters))
    setError(null)
  }, [])

  const updateProject = useCallback(
    async (projectId: ProjectId, request: UpdateProjectRequest): Promise<Project | null> => {
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      const payload = normalizeProjectUpdateRequest(request)
      if (payload.name !== undefined && payload.name.length === 0) {
        setError('请输入产品名称。')
        return null
      }

      setUpdatingProject(true)
      setError(null)

      try {
        const updated = await projectApi.update(projectId, payload, csrfToken)
        setProjects((current) => {
          if (updated.status !== 'ACTIVE') {
            const nextProjects = sortProjects(current.filter((project) => project.id !== updated.id))
            if (selectedProjectIdRef.current === updated.id) {
              const nextSelectedProjectId = nextProjects[0]?.id ?? null
              selectedProjectIdRef.current = nextSelectedProjectId
              setSelectedProjectId(nextSelectedProjectId)
            }
            return nextProjects
          }
          return sortProjects(current.map((project) => (project.id === updated.id ? updated : project)))
        })
        return updated
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '产品更新失败，请稍后重试。'))
        return null
      } finally {
        setUpdatingProject(false)
      }
    },
    [csrfToken, projectApi],
  )

  const deleteProject = useCallback(
    async (projectId: ProjectId): Promise<boolean> => {
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return false
      }

      setDeletingProject(true)
      setError(null)

      try {
        await projectApi.delete(projectId, csrfToken)
        const remainingProjects = sortProjects(projects.filter((project) => project.id !== projectId))
        setProjects(remainingProjects)

        if (selectedProjectIdRef.current === projectId) {
          const nextProjectId = remainingProjects[0]?.id ?? null
          selectedProjectIdRef.current = nextProjectId
          assetRequestVersionRef.current += 1
          setSelectedProjectId(nextProjectId)
          setAssets([])
          setAssetStatus(nextProjectId ? 'loading' : 'idle')
        }
        return true
      } catch (requestError) {
        setError(getProjectDeleteErrorMessage(requestError))
        return false
      } finally {
        setDeletingProject(false)
      }
    },
    [csrfToken, projectApi, projects],
  )

  const refreshProjectMembers = useCallback(
    async (projectId: ProjectId | null = selectedProjectId) => {
      const requestVersion = projectMemberRequestVersionRef.current + 1
      projectMemberRequestVersionRef.current = requestVersion

      if (!projectId) {
        setProjectMembers([])
        setProjectMemberStatus('idle')
        setProjectMemberError(null)
        return
      }

      setProjectMemberStatus('loading')
      setProjectMemberError(null)

      try {
        const members = await projectApi.listMembers(projectId)
        if (requestVersion !== projectMemberRequestVersionRef.current || selectedProjectIdRef.current !== projectId) {
          return
        }
        setProjectMembers(sortMembers(members))
        setProjectMemberStatus('success')
      } catch (requestError) {
        if (requestVersion !== projectMemberRequestVersionRef.current || selectedProjectIdRef.current !== projectId) {
          return
        }
        setProjectMemberStatus('error')
        setProjectMemberError(getProjectMemberErrorMessage(requestError, '无法加载产品成员，请稍后重试。'))
      }
    },
    [projectApi, selectedProjectId],
  )

  const refreshProjectMemberCandidates = useCallback(
    async (projectId: ProjectId | null = selectedProjectId, params: ListProjectMemberCandidatesParams = {}) => {
      if (!projectId) {
        setProjectMemberCandidates([])
        setProjectMemberCandidateStatus('idle')
        return
      }

      setProjectMemberCandidateStatus('loading')
      setProjectMemberError(null)

      try {
        const candidates = await projectApi.listMemberCandidates(projectId, { pageNum: 1, pageSize: 100, ...params })
        if (selectedProjectIdRef.current !== projectId) {
          return
        }
        setProjectMemberCandidates(candidates)
        setProjectMemberCandidateStatus('success')
      } catch (requestError) {
        if (selectedProjectIdRef.current !== projectId) {
          return
        }
        setProjectMemberCandidateStatus('error')
        setProjectMemberError(getProjectMemberErrorMessage(requestError, '无法加载可添加用户，请稍后重试。'))
      }
    },
    [projectApi, selectedProjectId],
  )

  const addProjectMember = useCallback(
    async (projectId: ProjectId, request: ProjectMemberRequest): Promise<ProjectMember | null> => {
      if (!csrfToken) {
        setProjectMemberError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      const userId = String(request.userId).trim()
      if (!userId) {
        setProjectMemberError('请选择要添加的成员。')
        return null
      }

      setSavingProjectMember(true)
      setProjectMemberActionUserId(userId)
      setProjectMemberError(null)

      try {
        const member = await projectApi.addMember(projectId, { ...request, userId }, csrfToken)
        if (selectedProjectIdRef.current === projectId) {
          setProjectMembers((current) => sortMembers([member, ...current.filter((item) => item.userId !== member.userId)]))
          setProjectMemberStatus('success')
        }
        return member
      } catch (requestError) {
        setProjectMemberError(getProjectMemberErrorMessage(requestError, '产品成员添加失败，请稍后重试。'))
        return null
      } finally {
        setSavingProjectMember(false)
        setProjectMemberActionUserId(null)
      }
    },
    [csrfToken, projectApi],
  )

  const updateProjectMember = useCallback(
    async (projectId: ProjectId, userId: UserId | string, request: UpdateProjectMemberRequest): Promise<ProjectMember | null> => {
      if (!csrfToken) {
        setProjectMemberError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      setSavingProjectMember(true)
      setProjectMemberActionUserId(userId)
      setProjectMemberError(null)

      try {
        const member = await projectApi.updateMember(projectId, userId, request, csrfToken)
        if (selectedProjectIdRef.current === projectId) {
          setProjectMembers((current) => current.map((item) => (item.userId === member.userId ? member : item)))
          setProjectMemberStatus('success')
        }
        return member
      } catch (requestError) {
        setProjectMemberError(getProjectMemberErrorMessage(requestError, '产品成员更新失败，请稍后重试。'))
        return null
      } finally {
        setSavingProjectMember(false)
        setProjectMemberActionUserId(null)
      }
    },
    [csrfToken, projectApi],
  )

  const removeProjectMember = useCallback(
    async (projectId: ProjectId, userId: UserId | string): Promise<boolean> => {
      if (!csrfToken) {
        setProjectMemberError('缺少安全校验信息，请刷新页面后重试。')
        return false
      }

      setSavingProjectMember(true)
      setProjectMemberActionUserId(userId)
      setProjectMemberError(null)

      try {
        await projectApi.removeMember(projectId, userId, csrfToken)
        if (selectedProjectIdRef.current === projectId) {
          setProjectMembers((current) => current.filter((item) => item.userId !== userId))
          setProjectMemberStatus('success')
        }
        return true
      } catch (requestError) {
        setProjectMemberError(getProjectMemberErrorMessage(requestError, '产品成员移除失败，请稍后重试。'))
        return false
      } finally {
        setSavingProjectMember(false)
        setProjectMemberActionUserId(null)
      }
    },
    [csrfToken, projectApi],
  )

  const reorderProjects = useCallback(
    async (orderedProjectIds: Array<ProjectId | string>): Promise<boolean> => {
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return false
      }
      const idSet = new Set(orderedProjectIds.map(String))
      if (idSet.size !== projects.length || projects.some((project) => !idSet.has(String(project.id)))) {
        setError('产品排序数据不完整，请刷新产品列表后重试。')
        return false
      }

      const previousProjects = projects
      const optimisticProjects = orderedProjectIds
        .map((projectId, index) => {
          const project = projects.find((candidate) => String(candidate.id) === String(projectId))
          return project ? { ...project, sortOrder: (index + 1) * 10 } : null
        })
        .filter((project): project is Project => Boolean(project))

      setProjects(optimisticProjects)
      setUpdatingProject(true)
      setError(null)

      try {
        const updatedProjects = await Promise.all(
          optimisticProjects.map((project) => projectApi.update(project.id, { sortOrder: project.sortOrder }, csrfToken)),
        )
        setProjects(sortProjects(updatedProjects))
        return true
      } catch (requestError) {
        setProjects(previousProjects)
        setError(getProjectAssetErrorMessage(requestError, '产品排序保存失败，请刷新后重试。'))
        return false
      } finally {
        setUpdatingProject(false)
      }
    },
    [csrfToken, projectApi, projects],
  )

  const uploadReferences = useCallback(
    async (files: File[] | FileList, category: WorkbenchImageType = DEFAULT_WORKBENCH_IMAGE_TYPE): Promise<UploadReferenceResult> => {
      if (!selectedProjectId) {
        setError('请先选择产品。')
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
      const uploadProjectId = selectedProjectId

      try {
        const uploadedAssets: Asset[] = []
        for (const file of acceptedFiles) {
          const uploaded = await assetApi.uploadReference(
            selectedProjectId,
            {
              file,
              category,
              filename: file.name,
            },
            csrfToken,
          )
          uploadedAssets.push(uploaded)
        }

        if (selectedProjectIdRef.current !== uploadProjectId) {
          return { assets: [], skipped }
        }

        setAssets((current) =>
          filterAssetsForActiveFilters([
            ...uploadedAssets,
            ...current.filter((asset) => !uploadedAssets.some((item) => item.id === asset.id)),
          ], assetFilters),
        )
        setAssetStatus('success')
        return { assets: uploadedAssets, skipped }
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '参考图上传失败，请稍后重试。'))
        return { assets: [], skipped }
      } finally {
        setUploadingAsset(false)
      }
    },
    [assetApi, assetFilters, csrfToken, selectedProjectId],
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
        setAssets((current) => filterAssetsForActiveFilters(current.map((item) => (item.id === updated.id ? updated : item)), assetFilters))
        return updated
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '收藏状态更新失败，请稍后重试。'))
        return null
      } finally {
        setActionAssetId(null)
      }
    },
    [assetApi, assetFilters, csrfToken],
  )

  const updateAsset = useCallback(
    async (asset: Asset, request: UpdateAssetRequest): Promise<Asset | null> => {
      if (!csrfToken) {
        setError('缺少安全校验信息，请刷新页面后重试。')
        return null
      }

      const payload = normalizeAssetUpdateRequest(request)
      if (payload.filename !== undefined && payload.filename.length === 0) {
        setError('请输入资产文件名。')
        return null
      }

      setUpdatingAsset(true)
      setActionAssetId(asset.id)
      setError(null)

      try {
        const updated = await assetApi.update(asset.id, payload, csrfToken)
        setAssets((current) => filterAssetsForActiveFilters(current.map((item) => (item.id === updated.id ? updated : item)), assetFilters))
        return updated
      } catch (requestError) {
        setError(getProjectAssetErrorMessage(requestError, '资产元数据更新失败，请稍后重试。'))
        return null
      } finally {
        setUpdatingAsset(false)
        setActionAssetId(null)
      }
    },
    [assetApi, assetFilters, csrfToken],
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
    addProjectMember,
    assetFilters,
    assetStatus,
    assets,
    createProject,
    createReferenceFromAsset,
    deleteAsset,
    deleteProject,
    downloadAsset,
    error,
    isCreatingProject,
    isDeletingProject,
    isLoadingAssets: assetStatus === 'loading',
    isLoadingProjects: projectStatus === 'loading',
    isSavingProjectMember,
    isUpdatingAsset,
    isUpdatingProject,
    isUploadingAsset,
    projectMemberCandidates,
    projectMemberCandidateStatus,
    projectMemberActionUserId,
    projectMemberError,
    projectMembers,
    projectMemberStatus,
    projectStatus,
    projects,
    refreshAssets,
    refreshProjectMembers,
    refreshProjectMemberCandidates,
    refreshProjects,
    removeProjectMember,
    reorderProjects,
    selectProject,
    selectedProject,
    selectedProjectId,
    toggleFavorite,
    updateAsset,
    updateAssetFilters,
    updateProject,
    updateProjectMember,
    uploadReferences,
  }
}

function buildAssetListParams(filters: AssetFilters): ListAssetsParams {
  return {
    ...normalizeAssetFilters(filters),
    pageNum: 1,
    pageSize: 50,
  }
}

function normalizeAssetFilters(filters: AssetFilters): AssetFilters {
  return {
    category: filters.category?.trim() || undefined,
    favorite: filters.favorite,
    kind: filters.kind,
    imageType: filters.imageType,
  }
}

function normalizeProjectUpdateRequest(request: UpdateProjectRequest): UpdateProjectRequest {
  return {
    brand: request.brand?.trim(),
    asin: request.asin?.trim(),
    name: request.name?.trim(),
    notes: request.notes?.trim(),
    site: request.site?.trim(),
    status: request.status,
    sortOrder: request.sortOrder,
  }
}

function sortProjects(projects: Project[]): Project[] {
  return [...projects].sort((left, right) => {
    if (left.sortOrder !== right.sortOrder) {
      return left.sortOrder - right.sortOrder
    }
    if (left.createdAt !== right.createdAt) {
      return left.createdAt < right.createdAt ? -1 : 1
    }
    return String(left.id).localeCompare(String(right.id), 'zh-CN')
  })
}

function sortMembers(members: ProjectMember[]): ProjectMember[] {
  return [...members].sort((left, right) => {
    const leftName = left.userName || left.userEmail || String(left.userId)
    const rightName = right.userName || right.userEmail || String(right.userId)
    return leftName.localeCompare(rightName, 'zh-CN')
  })
}

function normalizeAssetUpdateRequest(request: UpdateAssetRequest): UpdateAssetRequest {
  return {
    category: request.category,
    filename: request.filename?.trim(),
    isFavorite: request.isFavorite,
  }
}

function filterAssetsForActiveFilters(assets: Asset[], filters: AssetFilters): Asset[] {
  return assets.filter((asset) => assetMatchesFilters(asset, filters))
}

function assetMatchesFilters(asset: Asset, filters: AssetFilters): boolean {
  const normalizedFilters = normalizeAssetFilters(filters)
  if (normalizedFilters.kind && asset.kind !== normalizedFilters.kind) {
    return false
  }
  if (normalizedFilters.category && asset.category !== normalizedFilters.category) {
    return false
  }
  if (normalizedFilters.favorite !== undefined && asset.isFavorite !== normalizedFilters.favorite) {
    return false
  }
  if (normalizedFilters.imageType && normalizeWorkbenchImageType(asset.category || asset.imageType) !== normalizedFilters.imageType) {
    return false
  }
  return true
}

function getProjectAssetErrorMessage(error: unknown, fallback: string): string {
  if (!isApiClientError(error)) {
    return fallback
  }

  if (error.status === 401) {
    return '登录状态已失效，请重新登录。'
  }

  if (error.status === 403) {
    return '没有权限执行该产品素材操作。'
  }

  if (error.status === 404) {
    return '产品或素材不存在，可能已被删除或无权访问。'
  }

  if (error.status === 422) {
    return '请求内容未通过校验，请检查产品名称或图片文件。'
  }

  if (error.status === 409) {
    return '操作冲突，请刷新后重试。'
  }

  return fallback
}

function getProjectMemberErrorMessage(error: unknown, fallback: string): string {
  if (!isApiClientError(error)) {
    return fallback
  }

  if (error.status === 401) {
    return '登录状态已失效，请重新登录。'
  }

  if (error.status === 403) {
    return '没有权限管理产品成员。'
  }

  if (error.status === 404) {
    return '产品或成员不存在，可能已被删除或无权访问。'
  }

  if (error.status === 409) {
    return '产品成员操作冲突，请刷新后重试。'
  }

  if (error.status === 422) {
    return '成员信息未通过校验，请选择有效用户和角色。'
  }

  return fallback
}

function getProjectDeleteErrorMessage(error: unknown): string {
  if (!isApiClientError(error)) {
    return '产品删除失败，请稍后重试。'
  }
  if (error.status === 401) {
    return '登录状态已失效，请重新登录。'
  }
  if (error.status === 403) {
    return '没有权限删除该产品。'
  }
  if (error.status === 404) {
    return '产品不存在，可能已被删除或无权访问。'
  }
  if (error.status === 409) {
    return '产品当前无法删除，请刷新后重试。'
  }
  return '产品删除失败，请稍后重试。'
}
