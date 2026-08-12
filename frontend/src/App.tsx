import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { AuthStatus } from './components/auth/AuthStatus'
import { LoginPanel } from './components/auth/LoginPanel'
import { AppShell } from './components/layout/AppShell'
import { WorkspaceShell } from './components/layout/WorkspaceShell'
import { HistoryPanel } from './components/history/HistoryPanel'
import { ImageDetailModal, type ImageDetail } from './components/modals/ImageDetailModal'
import { AssetDetailModal } from './components/projects/AssetDetailModal'
import { ProjectAssetsPanel } from './components/projects/ProjectAssetsPanel'
import { ProjectManagementModal } from './components/projects/ProjectManagementModal'
import { BackendControlPanel, type BackendControlPanelDraft } from './components/studio/BackendControlPanel'
import { CanvasStudioLayout } from './components/studio/CanvasStudioLayout'
import { ResultCanvas, type CompareItemRef } from './components/studio/ResultCanvas'
import { TemplateLibraryPage } from './components/templates/TemplateLibraryPage'
import { TaskCenter, TaskNotifications } from './components/tasks/TaskCenter'
import { ContextDrawer } from './components/ui/ContextDrawer'
import { type ImageTypeActivity, WorkspaceOnboarding } from './components/studio/WorkbenchNavigation'
import { useWorkbenchModels } from './components/studio/useWorkbenchModels'
import { AuthProvider } from './hooks/AuthProvider'
import { useAuth } from './hooks/useAuth'
import { useGeneration } from './hooks/useGeneration'
import { useHistory } from './hooks/useHistory'
import { useProjectAssets } from './hooks/useProjectAssets'
import { downloadBlob } from './lib/download'
import type { AuthSession } from './types/auth'
import type { BackendHistoryItem } from './types/history'
import { normalizeWorkspacePathname, primaryRouteFromPathname } from './types/navigation'
import type { Asset, Project, ProjectMemberRole, TaskId, UserId } from './types/platform'
import {
  DEFAULT_WORKBENCH_IMAGE_TYPE,
  WORKBENCH_IMAGE_TYPE_OPTIONS,
  normalizeWorkbenchImageType,
  type AssetReferenceInput,
  type WorkbenchImageType,
  type WorkbenchReferenceInput,
  type WorkbenchTaskInput,
  type WorkbenchTaskSubmission,
} from './types/workbench'

const AdminConsole = lazy(() => import('./components/admin/console/AdminConsole').then((module) => ({ default: module.AdminConsole })))

function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  )
}

function AppContent() {
  const auth = useAuth()
  const [pathname, setPathname] = useState(() => window.location.pathname)

  useEffect(() => {
    const handlePopState = () => setPathname(window.location.pathname)
    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  const navigate = useCallback((nextPathname: string) => {
    if (window.location.pathname === nextPathname && !window.location.search) return
    window.history.pushState({}, '', nextPathname)
    setPathname(nextPathname)
  }, [])

  if (auth.status === 'loading') {
    return (
      <AppShell notice={auth.error ?? undefined}>
        <div className="flex min-h-[calc(100dvh-180px)] items-center justify-center text-sm text-ink-500">
          正在加载登录状态...
        </div>
      </AppShell>
    )
  }

  if (auth.status !== 'authenticated' || !auth.session) {
    return (
      <AppShell>
        <LoginPanel error={auth.error} isSubmitting={auth.isSubmitting} onSubmit={auth.login} />
      </AppShell>
    )
  }

  if (pathname.startsWith('/admin')) {
    return (
      <Suspense fallback={<div aria-busy="true" className="flex min-h-screen items-center justify-center bg-slate-50 text-sm font-semibold text-slate-600" role="status">正在加载管理控制台...</div>}>
        <AdminConsole
          isAuthSubmitting={auth.isSubmitting}
          onWorkspaceNavigate={navigate}
          onLogout={auth.logout}
          session={auth.session}
        />
      </Suspense>
    )
  }

  return (
    <StudioWorkbench
      authError={auth.error}
      isAuthSubmitting={auth.isSubmitting}
      onLogout={auth.logout}
      onNavigate={navigate}
      pathname={normalizeWorkspacePathname(pathname)}
      session={auth.session}
    />
  )
}

interface StudioWorkbenchProps {
  authError: string | null
  isAuthSubmitting: boolean
  session: AuthSession
  onLogout: () => Promise<void>
  onNavigate: (pathname: string) => void
  pathname: string
}

function StudioWorkbench({ authError, isAuthSubmitting, onLogout, onNavigate, pathname, session }: StudioWorkbenchProps) {
  const projectAssets = useProjectAssets({ csrfToken: session.csrfToken })
  const { refreshProjectMemberCandidates, refreshProjectMembers } = projectAssets
  const workbenchModels = useWorkbenchModels()
  const [imageType, setImageType] = useState<WorkbenchImageType>(DEFAULT_WORKBENCH_IMAGE_TYPE)
  const history = useHistory({ csrfToken: session.csrfToken, imageType, projectId: projectAssets.selectedProjectId })
  const refreshHistory = history.refresh
  const generation = useGeneration({
    csrfToken: session.csrfToken,
    imageType,
    onModelInvalidated: workbenchModels.refreshModels,
    projectId: projectAssets.selectedProjectId,
  })
  const imageTypeActivity = useMemo(() => {
    const activity: Partial<Record<WorkbenchImageType, ImageTypeActivity>> = {}

    generation.tasks.forEach((item) => {
      if (item.task.projectId !== projectAssets.selectedProjectId) {
        return
      }

      const status = item.state.status ?? item.task.status
      const isActive = status === 'QUEUED' || status === 'RUNNING' || status === 'RETRYING'
      if (!isActive && status !== 'SUCCEEDED') {
        return
      }

      const taskImageType = normalizeWorkbenchImageType(item.task.imageType)
      const current = activity[taskImageType] ?? { activeCount: 0, completedCount: 0 }
      activity[taskImageType] = {
        activeCount: current.activeCount + (isActive ? 1 : 0),
        completedCount: current.completedCount + (status === 'SUCCEEDED' ? 1 : 0),
      }
    })

    return activity
  }, [generation.tasks, projectAssets.selectedProjectId])
  const [isDetailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<ImageDetail | null>(null)
  const [detailError, setDetailError] = useState('')
  const [isDetailLoading, setDetailLoading] = useState(false)
  const [assetDetail, setAssetDetail] = useState<Asset | null>(null)
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<BackendControlPanelDraft | null>(null)
  const [promptDrafts, setPromptDrafts] = useState<Record<string, string>>({})
  const [referenceToAdd, setReferenceToAdd] = useState<WorkbenchReferenceInput | null>(null)
  const [isWorkspaceLibraryOpen, setWorkspaceLibraryOpen] = useState(false)
  const [isTaskCenterOpen, setTaskCenterOpen] = useState(false)
  const [projectToManage, setProjectToManage] = useState<Project | null>(null)
  const [shouldOpenProjectEditor, setShouldOpenProjectEditor] = useState(false)
  const [comparisonSourceByTaskId, setComparisonSourceByTaskId] = useState<Record<string, AssetReferenceInput>>({})
  const [pendingEditSourceAssetId, setPendingEditSourceAssetId] = useState<Asset['id'] | null>(null)
  const [editSourceReference, setEditSourceReference] = useState<AssetReferenceInput | null>(null)
  const selectedProjectIdRef = useRef(projectAssets.selectedProjectId)
  const detailRequestVersionRef = useRef(0)
  const refreshedSuccessNotificationIdRef = useRef('')
  const workspaceDraftKey = `${projectAssets.selectedProjectId ?? 'none'}:${imageType}`
  const promptDraft = promptDrafts[workspaceDraftKey] ?? ''
  const updatePromptDraft = useCallback(
    (nextPrompt: string) => setPromptDrafts((currentDrafts) => ({ ...currentDrafts, [workspaceDraftKey]: nextPrompt })),
    [workspaceDraftKey],
  )

  useEffect(() => {
    if (generation.error) {
      setNotice(generation.error)
    }
  }, [generation.error])

  useEffect(() => {
    if (projectAssets.error) {
      setNotice(projectAssets.error)
    }
  }, [projectAssets.error])

  useEffect(() => {
    if (workbenchModels.error) {
      setNotice(workbenchModels.error)
    }
  }, [workbenchModels.error])

  useEffect(() => {
    selectedProjectIdRef.current = projectAssets.selectedProjectId
    detailRequestVersionRef.current += 1
    setDetail(null)
    setDetailError('')
    setDetailOpen(false)
    setPendingEditSourceAssetId(null)
    setEditSourceReference(null)
    setDraft(null)
    setReferenceToAdd(null)
    setWorkspaceLibraryOpen(false)
  }, [projectAssets.selectedProjectId])

  useEffect(() => {
    const latestSuccess = [...generation.notifications].reverse().find((notification) => notification.status === 'SUCCEEDED')
    if (!latestSuccess || latestSuccess.id === refreshedSuccessNotificationIdRef.current) {
      return
    }
    refreshedSuccessNotificationIdRef.current = latestSuccess.id
    void refreshHistory()
  }, [generation.notifications, refreshHistory])

  const showNotice = (message: string) => {
    setNotice(message)
  }

  const tenantAdmin = isTenantAdmin(session)
  const canManageProviders = tenantAdmin && hasPermission(session, 'provider:manage')
  const canManageModels = tenantAdmin && hasPermission(session, 'model:manage')
  const canReadProviders = tenantAdmin && hasPermission(session, 'provider:read')
  const canReadModels = tenantAdmin && hasPermission(session, 'model:read')
  const canReadUsage = hasPermission(session, 'usage:read')
  const canReadUsers = hasPermission(session, 'user:read')
  const canReadAudit = hasPermission(session, 'audit:read')
  const canManageSystemSettings = hasPermission(session, 'system:settings:manage')
  const canManageProjectMembers = hasPermission(session, 'project:member:manage')
  const canCreateProject = hasPermission(session, 'project:create')
  const canDeleteProject = hasPermission(session, 'project:delete')
  const canOpenAdmin = canManageProviders || canManageModels
  const canViewAnalytics = tenantAdmin && (canReadUsage || canReadUsers || canReadAudit)
  const canViewSettings = tenantAdmin && (
    canManageProviders || canManageModels || canReadProviders || canReadModels || canManageSystemSettings
  )
  const defaultAnalyticsPath = canReadUsage ? '/admin/overview'
    : canReadUsers ? '/admin/users'
      : '/admin/requests'
  const defaultSettingsPath = canManageSystemSettings ? '/admin/settings' : '/admin/providers'
  const selectedProject = projectAssets.projects.find((project) => project.id === projectAssets.selectedProjectId) ?? null
  const activeTaskCount = generation.tasks.filter((item) => {
    const status = item.state.status ?? item.task.status
    return status === 'QUEUED' || status === 'RUNNING' || status === 'RETRYING'
  }).length
  const completedImageTypeCount = new Set(
    projectAssets.assets
      .filter((asset) => asset.kind === 'GENERATED' || asset.kind === 'EDITED')
      .map((asset) => normalizeWorkbenchImageType(asset.imageType ?? asset.category)),
  ).size
  const productPreviewUrl = projectAssets.assets.find(
    (asset) => asset.kind === 'REFERENCE' && (asset.thumbnailUrl || asset.previewUrl),
  )?.thumbnailUrl ?? projectAssets.assets.find(
    (asset) => asset.kind === 'REFERENCE' && asset.previewUrl,
  )?.previewUrl
  const activePrimaryRoute = primaryRouteFromPathname(pathname)

  useEffect(() => {
    if (activePrimaryRoute === 'products') return
    setShouldOpenProjectEditor(false)
    setProjectToManage(null)
  }, [activePrimaryRoute])

  const currentComparisonSource: CompareItemRef | undefined = (() => {
    if (generation.current?.task.type !== 'IMAGE_EDIT') return undefined
    const localSource = comparisonSourceByTaskId[generation.current.task.id]
    if (localSource) return { id: localSource.assetId, label: '原图', url: localSource.previewUrl }
    const sourceAsset = projectAssets.assets.find((asset) => generation.current?.task.inputAssetIds.includes(asset.id))
    if (!sourceAsset) return undefined
    const reference = projectAssets.createReferenceFromAsset(sourceAsset)
    return { id: reference.assetId, label: '原图', url: reference.previewUrl }
  })()

  const handleGenerateTask = async (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput) => {
    const submittedEditSource = editSourceReference
    if (pendingEditSourceAssetId) {
      const isAvailable = await history.ensureBackendAssetAvailable(pendingEditSourceAssetId)
      if (!isAvailable) {
        setNotice('再次编辑所需资产不可用，请刷新历史后重试。')
        setPendingEditSourceAssetId(null)
        setEditSourceReference(null)
        return
      }
    }

    const task = await generation.generateTask(
      request,
      pendingEditSourceAssetId ? { ...workbenchInput, editSourceAssetId: pendingEditSourceAssetId } : workbenchInput,
    )

    if (task) {
      if (submittedEditSource) {
        setComparisonSourceByTaskId((sources) => ({ ...sources, [task.id]: submittedEditSource }))
      }
      setPendingEditSourceAssetId(null)
      setEditSourceReference(null)
      setNotice('任务已创建，结果会通过实时事件流更新。')
    }
  }

  const handleOpenBackendDetail = async (assetId: Asset['id'], taskId?: BackendHistoryItem['task']['id']) => {
    const detailRequestVersion = detailRequestVersionRef.current + 1
    detailRequestVersionRef.current = detailRequestVersion
    setDetail(null)
    setDetailError('')
    setDetailLoading(true)
    setDetailOpen(true)

    try {
      const backendDetail = await history.loadBackendDetail(assetId, taskId)
      if (detailRequestVersion !== detailRequestVersionRef.current || selectedProjectIdRef.current !== backendDetail.asset.projectId) {
        return
      }
      setDetail({
        kind: 'backend',
        ...backendDetail,
      })
    } catch {
      if (detailRequestVersion !== detailRequestVersionRef.current) {
        return
      }
      setDetailError('无法读取该结果，可能已被删除或无权访问。')
    } finally {
      if (detailRequestVersion === detailRequestVersionRef.current) {
        setDetailLoading(false)
      }
    }
  }

  const handleDownloadCurrent = async () => {
    if (!generation.current) {
      return
    }

    const download = await generation.downloadCurrentBackendAsset()
    if (download) {
      downloadBlob(download.blob, download.filename)
    }
  }

  const handleDownloadBackendHistory = async (item: BackendHistoryItem) => {
    const download = await history.downloadBackendAsset(item.asset)

    if (!download) {
      setNotice('结果下载失败，请稍后重试。')
      return
    }

    downloadBlob(download.blob, download.filename ?? item.asset.filename)
    setNotice('结果下载已开始。')
  }

  const handleEditBackendHistory = async (item: BackendHistoryItem) => {
    try {
      const backendDetail = await history.loadBackendDetail(item.asset.id, item.task.id)
      if (selectedProjectIdRef.current !== backendDetail.asset.projectId) {
        return
      }
      setPendingEditSourceAssetId(backendDetail.asset.id)
      setEditSourceReference(projectAssets.createReferenceFromAsset(backendDetail.asset))
      setDraft({
        prompt: backendDetail.task.prompt,
        modelId: backendDetail.task.modelId,
        imageType: backendDetail.task.imageType,
        imageCount: getTaskOutputCount(backendDetail.task),
      })
      setImageType(normalizeWorkbenchImageType(backendDetail.task.imageType))
      setNotice('已准备基于后端资产再次编辑。')
    } catch {
      setPendingEditSourceAssetId(null)
      setEditSourceReference(null)
      setNotice('再次编辑所需资产不可用，请刷新历史后重试。')
    }
  }

  const handleCreateProject = async (request: {
    name: string
    brand?: string
    asin?: string
    site?: string
    notes?: string
    status?: Project['status']
    sortOrder?: number
  }) => {
    const project = await projectAssets.createProject(request)

    if (project) {
      setNotice(`已创建产品：${project.name}`)
      setProjectToManage(null)
      setShouldOpenProjectEditor(false)
      onNavigate('/studio')
    }
  }

  const handleUpdateProject = async (
    projectId: Asset['projectId'],
    request: { name?: string; brand?: string; asin?: string; site?: string; notes?: string; status?: Project['status']; sortOrder?: number },
  ) => {
    const project = await projectAssets.updateProject(projectId, request)

    if (project) {
      setNotice(`已更新产品：${project.name}`)
    }
  }

  const handleDeleteProject = async (project: Project) => {
    if (!window.confirm(`确定删除产品“${project.name}”吗？\n删除后该产品及其素材、生成记录将不再显示，且无法在页面中撤销。`)) {
      return
    }

    const deleted = await projectAssets.deleteProject(project.id)
    if (deleted) {
      setNotice(`产品“${project.name}”已删除。`)
      setProjectToManage(null)
      setShouldOpenProjectEditor(false)
      onNavigate('/studio')
    }
  }

  const handleUploadReferences = async (files: FileList) => {
    const result = await projectAssets.uploadReferences(files, imageType)

    if (result.assets.length > 0) {
      setNotice(`已上传 ${result.assets.length} 张参考图到产品素材库。`)
    } else if (result.skipped > 0) {
      setNotice('参考图未上传，请检查图片格式和大小。')
    }
  }

  const handleDownloadAsset = async (asset: Asset) => {
    const download = await projectAssets.downloadAsset(asset)

    if (download) {
      downloadBlob(download.blob, download.filename ?? asset.filename)
      setNotice('产品素材下载已开始。')
    }
  }

  const handleUseAssetAsReference = async (asset: Asset) => {
    const reference = await projectAssets.createReferenceFromAsset(asset)

    if (reference) {
      setReferenceToAdd(reference)
      setNotice('已将产品素材加入参考图。')
    }
  }

  const handleSavePendingReferences = async (files: File[]): Promise<WorkbenchReferenceInput[]> => {
    const result = await projectAssets.uploadReferences(files, imageType)
    if (result.assets.length > 0) {
      setNotice(`已保存 ${result.assets.length} 张参考图到产品。`)
    }
    return result.assets.map((asset) => projectAssets.createReferenceFromAsset(asset))
  }

  const handleManageProject = (project?: Project) => {
    setProjectToManage(project ?? null)
    setShouldOpenProjectEditor(true)
    onNavigate('/products')
  }

  const handleUseTemplate = (nextImageType: WorkbenchImageType, nextPrompt: string) => {
    const nextKey = `${projectAssets.selectedProjectId ?? 'none'}:${nextImageType}`
    setImageType(nextImageType)
    setPromptDrafts((currentDrafts) => ({ ...currentDrafts, [nextKey]: nextPrompt }))
    setNotice('模板已套用，可在创作室继续调整。')
    onNavigate('/studio')
  }

  const refreshProjectMembersForModal = useCallback(
    (projectId: Project['id']) => {
      void refreshProjectMembers(projectId)
    },
    [refreshProjectMembers],
  )

  const refreshProjectMemberCandidatesForModal = useCallback(
    (projectId: Project['id'], q?: string) => {
      const query = q?.trim()
      void refreshProjectMemberCandidates(projectId, query ? { q: query } : undefined)
    },
    [refreshProjectMemberCandidates],
  )

  const handleDeleteAsset = async (asset: Asset) => {
    if (!window.confirm(`确定删除产品素材 ${asset.filename} 吗？`)) {
      return
    }

    const deleted = await projectAssets.deleteAsset(asset)

    if (deleted) {
      setNotice('产品素材已删除。')
      if (assetDetail?.id === asset.id) {
        setAssetDetail(null)
      }
      await projectAssets.refreshAssets()
    }
  }

  const handleUpdateAsset = async (
    asset: Asset,
    request: { filename?: string; category?: WorkbenchImageType; isFavorite?: boolean },
  ) => {
    const updated = await projectAssets.updateAsset(asset, request)

    if (updated) {
      setAssetDetail(updated)
      if (pendingEditSourceAssetId === updated.id) {
        setEditSourceReference(projectAssets.createReferenceFromAsset(updated))
      }
      setNotice('资产元数据已更新。')
      await history.refresh()
    }
  }

  const handleToggleAssetFavorite = async (asset: Asset) => {
    const updated = await projectAssets.toggleFavorite(asset)

    if (updated) {
      if (assetDetail?.id === updated.id) {
        setAssetDetail(updated)
      }
      setNotice(updated.isFavorite ? '图片已收藏。' : '已取消收藏。')
      await history.refresh()
    }
  }

  const handleAddProjectMember = async (
    projectId: Asset['projectId'],
    request: { userId: UserId | string; role: ProjectMemberRole },
  ) => {
    const member = await projectAssets.addProjectMember(projectId, request)
    if (member) {
      setNotice('产品成员已添加。')
    }
  }

  const handleUpdateProjectMember = async (
    projectId: Asset['projectId'],
    userId: UserId | string,
    request: { role: ProjectMemberRole },
  ) => {
    const member = await projectAssets.updateProjectMember(projectId, userId, request)
    if (member) {
      setNotice('产品成员已更新。')
    }
  }

  const handleRemoveProjectMember = async (projectId: Asset['projectId'], userId: UserId | string) => {
    if (!window.confirm(`确定移除产品成员 ${userId} 吗？`)) {
      return
    }

    const removed = await projectAssets.removeProjectMember(projectId, userId)
    if (removed) {
      setNotice('产品成员已移除。')
    }
  }

  const handleOpenCurrentDetail = () => {
    if (!generation.current) {
      return
    }

    void handleOpenBackendDetail(generation.current.result.assetId, generation.current.task.id)
  }

  const handleRenameCurrent = async () => {
    if (!generation.current) {
      return
    }

    try {
      const backendDetail = await history.loadBackendDetail(generation.current.result.assetId, generation.current.task.id)
      if (selectedProjectIdRef.current !== backendDetail.asset.projectId) {
        return
      }
      setAssetDetail(backendDetail.asset)
    } catch {
      setNotice('无法读取该图片，可能已被删除或无权访问。')
    }
  }

  const handleViewTask = (taskId: TaskId) => {
    const managedTask = generation.tasks.find((item) => item.task.id === taskId)
    if (!managedTask) {
      return
    }

    projectAssets.selectProject(managedTask.task.projectId)
    setImageType(normalizeWorkbenchImageType(managedTask.task.imageType))
    generation.selectTask(taskId)
  }

  const handleViewNotification = (taskId: TaskId, notificationId: string) => {
    handleViewTask(taskId)
    generation.dismissNotification(notificationId)
  }

  const handleDownloadDetail = async () => {
    if (!detail) {
      return
    }

    const download = await history.downloadBackendAsset(detail.asset)
    if (!download) {
      setNotice('结果下载失败，请稍后重试。')
      return
    }

    downloadBlob(download.blob, download.filename ?? detail.asset.filename)
  }

  return (
    <AppShell immersive notice={authError ?? notice}>
      <WorkspaceShell
        accountSlot={<AuthStatus isSubmitting={isAuthSubmitting} onLogout={onLogout} session={session} variant="compact" />}
        activeRoute={activePrimaryRoute}
        analyticsPathname={defaultAnalyticsPath}
        canViewAnalytics={canViewAnalytics}
        canViewSettings={canViewSettings}
        onNavigate={onNavigate}
        settingsPathname={defaultSettingsPath}
        surfaceMode={activePrimaryRoute === 'studio' ? 'workbench' : 'light'}
      >
        {activePrimaryRoute === 'products' ? (
          <ProjectManagementModal
            actionUserId={projectAssets.projectMemberActionUserId}
            canCreateProject={canCreateProject}
            canDeleteProject={canDeleteProject}
            canManageProjectMembers={canManageProjectMembers}
            candidateStatus={projectAssets.projectMemberCandidateStatus}
            candidates={projectAssets.projectMemberCandidates}
            error={projectAssets.error}
            initialProject={shouldOpenProjectEditor ? projectToManage : selectedProject}
            isCreatingProject={projectAssets.isCreatingProject}
            isDeletingProject={projectAssets.isDeletingProject}
            isOpen
            isSavingMember={projectAssets.isSavingProjectMember}
            isUpdatingProject={projectAssets.isUpdatingProject}
            memberError={projectAssets.projectMemberError}
            members={projectAssets.projectMembers}
            memberStatus={projectAssets.projectMemberStatus}
            onAddMember={(projectId, request) => void handleAddProjectMember(projectId, request)}
            onClose={() => onNavigate('/studio')}
            onCreateProject={handleCreateProject}
            onDeleteProject={(project) => void handleDeleteProject(project)}
            onRefreshCandidates={refreshProjectMemberCandidatesForModal}
            onRefreshMembers={refreshProjectMembersForModal}
            onRemoveMember={(projectId, userId) => void handleRemoveProjectMember(projectId, userId)}
            onSelectProject={projectAssets.selectProject}
            onUpdateMember={(projectId, userId, request) => void handleUpdateProjectMember(projectId, userId, request)}
            onUpdateProject={(projectId, request) => void handleUpdateProject(projectId, request)}
            openEditorOnMount={shouldOpenProjectEditor}
            projects={projectAssets.projects}
            variant="page"
          />
        ) : null}

        {activePrimaryRoute === 'assets' ? (
          <ProjectAssetsPanel
            actionAssetId={projectAssets.actionAssetId}
            assetFilters={projectAssets.assetFilters}
            assetStatus={projectAssets.assetStatus}
            assets={projectAssets.assets}
            error={projectAssets.error}
            isLoadingAssets={projectAssets.isLoadingAssets}
            isUploadingAsset={projectAssets.isUploadingAsset}
            onDeleteAsset={handleDeleteAsset}
            onDownloadAsset={handleDownloadAsset}
            onOpenAsset={setAssetDetail}
            onRefreshAssets={() => void projectAssets.refreshAssets()}
            onSelectProject={projectAssets.selectProject}
            onToggleFavorite={(asset) => void handleToggleAssetFavorite(asset)}
            onUpdateAssetFilters={projectAssets.updateAssetFilters}
            onUploadReferences={(files) => void handleUploadReferences(files)}
            onUseAssetAsReference={(asset) => {
              void handleUseAssetAsReference(asset).then(() => onNavigate('/studio'))
            }}
            selectedProjectId={projectAssets.selectedProjectId}
            projects={projectAssets.projects}
            variant="page"
          />
        ) : null}

        {activePrimaryRoute === 'templates' ? (
          <TemplateLibraryPage
            onNotice={setNotice}
            onSelectProject={projectAssets.selectProject}
            onUseTemplate={handleUseTemplate}
            projectId={projectAssets.selectedProjectId}
            projectName={selectedProject?.name}
            projects={projectAssets.projects}
          />
        ) : null}

        {activePrimaryRoute === 'studio' ? (
          projectAssets.isLoadingProjects ? (
            <div aria-busy="true" className="flex min-h-[calc(100dvh-68px)] items-center justify-center text-sm text-slate-300" role="status">
              正在加载产品...
            </div>
          ) : !selectedProject ? (
            <div className="workspace-page">
              <WorkspaceOnboarding
                canCreateProject={canCreateProject}
                hasLoadError={projectAssets.projectStatus === 'error'}
                onCreateProject={() => handleManageProject()}
                onRetry={() => void projectAssets.refreshProjects()}
              />
            </div>
          ) : (
            <CanvasStudioLayout
              activeTaskCount={activeTaskCount}
              activityByType={imageTypeActivity}
              completedImageTypeCount={completedImageTypeCount}
              controlPanel={
                <BackendControlPanel
                  availableReferences={projectAssets.assets
                    .filter((asset) => asset.kind === 'REFERENCE')
                    .map((asset) => projectAssets.createReferenceFromAsset(asset))}
                  draft={draft}
                  editSourceReference={editSourceReference}
                  imageType={imageType}
                  isGenerating={generation.isSubmitting}
                  modelStatus={workbenchModels.status}
                  models={workbenchModels.models}
                  onError={showNotice}
                  onEditSourceRemoved={() => {
                    setPendingEditSourceAssetId(null)
                    setEditSourceReference(null)
                    setNotice('已取消编辑原图。')
                  }}
                  onGenerate={handleGenerateTask}
                  onImageTypeChange={setImageType}
                  onPromptChange={updatePromptDraft}
                  onReferenceAdded={() => setReferenceToAdd(null)}
                  onOpenModelSettings={canOpenAdmin ? () => onNavigate('/admin/providers') : undefined}
                  onRefreshModels={() => void workbenchModels.refreshModels()}
                  onSavePendingReferences={handleSavePendingReferences}
                  prompt={promptDraft}
                  projectId={projectAssets.selectedProjectId}
                  referenceToAdd={referenceToAdd}
                  resetKey={projectAssets.selectedProjectId}
                  variant="canvas"
                />
              }
              imageType={imageType}
              isGenerating={generation.isSubmitting}
              onImageTypeChange={setImageType}
              onManageProject={handleManageProject}
              onOpenTaskCenter={() => setTaskCenterOpen(true)}
              onSaveDraft={() => setNotice('当前创作要求已保留为本页草稿。')}
              onSelectProject={projectAssets.selectProject}
              productPreviewUrl={productPreviewUrl || undefined}
              projects={projectAssets.projects}
              resultCanvas={
                <ResultCanvas
                  canCancelTask={generation.canCancelCurrentTask}
                  canRetryTask={generation.canRetryCurrentTask}
                  comparisonSource={currentComparisonSource}
                  current={generation.current}
                  currentItems={generation.currentItems}
                  error={generation.error}
                  imageTypeLabel={WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === imageType)?.label}
                  onCancelTask={() => void generation.cancelCurrentTask()}
                  onDownload={() => void handleDownloadCurrent()}
                  onOpenAssets={() => setWorkspaceLibraryOpen(true)}
                  onOpenDetail={handleOpenCurrentDetail}
                  onRename={() => void handleRenameCurrent()}
                  onRetryTask={() => void generation.retryCurrentTask()}
                  onSelect={generation.selectCurrent}
                  pendingTaskAction={generation.pendingTaskAction}
                  selectedIndex={generation.selectedIndex}
                  status={generation.status}
                  taskStatus={generation.taskState?.status}
                  variant="canvas"
                />
              }
              selectedProject={selectedProject}
            />
          )
        ) : null}
      </WorkspaceShell>

      <ContextDrawer
        description="快捷选择当前产品素材，不会遮挡或锁定创作画布。"
        isOpen={activePrimaryRoute === 'studio' && isWorkspaceLibraryOpen}
        onClose={() => setWorkspaceLibraryOpen(false)}
        title="选择素材"
      >
        <ProjectAssetsPanel
          actionAssetId={projectAssets.actionAssetId}
          assetFilters={projectAssets.assetFilters}
          assetStatus={projectAssets.assetStatus}
          assets={projectAssets.assets}
          error={projectAssets.error}
          isLoadingAssets={projectAssets.isLoadingAssets}
          isUploadingAsset={projectAssets.isUploadingAsset}
          onDeleteAsset={handleDeleteAsset}
          onDownloadAsset={handleDownloadAsset}
          onOpenAsset={setAssetDetail}
          onRefreshAssets={() => void projectAssets.refreshAssets()}
          onToggleFavorite={(asset) => void handleToggleAssetFavorite(asset)}
          onUpdateAssetFilters={projectAssets.updateAssetFilters}
          onUploadReferences={(files) => void handleUploadReferences(files)}
          onUseAssetAsReference={(asset) => void handleUseAssetAsReference(asset)}
          selectedProjectId={projectAssets.selectedProjectId}
        />
      </ContextDrawer>

      <TaskCenter
        history={
          <HistoryPanel
            embedded
            error={history.error}
            favoriteActionAssetId={history.favoriteActionAssetId}
            isLoading={history.isLoading}
            items={history.items}
            kind={history.kind}
            onDownload={(item) => void handleDownloadBackendHistory(item)}
            onEdit={(item) => {
              setTaskCenterOpen(false)
              void handleEditBackendHistory(item)
            }}
            onKindChange={history.setKind}
            onPageChange={history.setPageNum}
            onPageSizeChange={history.setPageSize}
            onRefresh={() => void history.refresh()}
            onRename={(item) => setAssetDetail(item.asset)}
            onToggleFavorite={(item) => {
              void history.toggleFavorite(item).then((updated) => {
                if (updated) {
                  setNotice(updated.isFavorite ? '图片已收藏。' : '已取消收藏。')
                  void projectAssets.refreshAssets()
                }
              })
            }}
            onView={(item) => {
              setTaskCenterOpen(false)
              void handleOpenBackendDetail(item.asset.id, item.task.id)
            }}
            pageNum={history.pageNum}
            pageSize={history.pageSize}
            total={history.total}
          />
        }
        isOpen={isTaskCenterOpen}
        onCancel={(taskId) => void generation.cancelTask(taskId)}
        onClose={() => setTaskCenterOpen(false)}
        onRetry={(taskId) => void generation.retryTask(taskId)}
        onView={(taskId) => {
          handleViewTask(taskId)
          setTaskCenterOpen(false)
        }}
        pendingTaskId={generation.pendingAction?.taskId}
        projects={projectAssets.projects}
        tasks={generation.tasks}
      />

      <TaskNotifications
        notifications={generation.notifications}
        onDismiss={generation.dismissNotification}
        onView={handleViewNotification}
        projects={projectAssets.projects}
      />

      <ImageDetailModal
        detail={detail}
        error={detailError}
        isOpen={isDetailOpen}
        isLoading={isDetailLoading}
        onClose={() => setDetailOpen(false)}
        onDownload={() => void handleDownloadDetail()}
      />

      <AssetDetailModal
        asset={assetDetail}
        isActionPending={Boolean(assetDetail && projectAssets.actionAssetId === assetDetail.id)}
        isOpen={Boolean(assetDetail)}
        onClose={() => setAssetDetail(null)}
        onDelete={(asset) => void handleDeleteAsset(asset)}
        onDownload={(asset) => void handleDownloadAsset(asset)}
        onUpdateAsset={(asset, request) => void handleUpdateAsset(asset, request)}
        onUseAsReference={(asset) => void handleUseAssetAsReference(asset)}
      />
    </AppShell>
  )
}

function getTaskOutputCount(task: BackendHistoryItem['task']): 1 | 2 | 3 | 4 {
  const outputCount = task.parameters.outputCount
  return outputCount === 1 || outputCount === 2 || outputCount === 3 || outputCount === 4 ? outputCount : 1
}

function hasPermission(session: AuthSession, permission: string): boolean {
  return session.permissions.some((candidate) => candidate === permission)
}

function isTenantAdmin(session: AuthSession): boolean {
  return session.roles.some((role) => role.code === 'admin')
}

export default App
