import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { BarChart3, Settings, UserCog } from 'lucide-react'
import { AuthStatus } from './components/auth/AuthStatus'
import { LoginPanel } from './components/auth/LoginPanel'
import { AdminObservabilitySettingsPanel } from './components/admin/AdminObservabilitySettingsPanel'
import { ProviderModelAdminPanel } from './components/admin/ProviderModelAdminPanel'
import { UserRoleAdminPanel } from './components/admin/UserRoleAdminPanel'
import { AppShell } from './components/layout/AppShell'
import { HistoryPanel } from './components/history/HistoryPanel'
import { ImageDetailModal, type ImageDetail } from './components/modals/ImageDetailModal'
import { AssetDetailModal } from './components/projects/AssetDetailModal'
import { ProjectAssetsPanel } from './components/projects/ProjectAssetsPanel'
import { ProjectManagementModal } from './components/projects/ProjectManagementModal'
import { ProjectTabs } from './components/projects/ProjectTabs'
import { ManagementMenu, type ManagementMenuItem } from './components/layout/ManagementMenu'
import { BackendControlPanel, type BackendControlPanelDraft } from './components/studio/BackendControlPanel'
import { ResultCanvas } from './components/studio/ResultCanvas'
import { GenerationHistoryRail, TaskNotifications } from './components/tasks/TaskCenter'
import { Modal } from './components/ui/Modal'
import {
  ImageTypeTabs,
  type ImageTypeActivity,
  ModelSetupBanner,
  WorkspaceOnboarding,
} from './components/studio/WorkbenchNavigation'
import { useWorkbenchModels } from './components/studio/useWorkbenchModels'
import { AuthProvider } from './hooks/AuthProvider'
import { useAuth } from './hooks/useAuth'
import { useGeneration } from './hooks/useGeneration'
import { useHistory } from './hooks/useHistory'
import { useProjectAssets } from './hooks/useProjectAssets'
import { downloadBlob } from './lib/download'
import type { AuthSession } from './types/auth'
import type { BackendHistoryItem } from './types/history'
import type { Asset, Project, ProjectMemberRole, TaskId, UserId } from './types/platform'
import {
  DEFAULT_WORKBENCH_IMAGE_TYPE,
  normalizeWorkbenchImageType,
  type AssetReferenceInput,
  type WorkbenchImageType,
  type WorkbenchReferenceInput,
  type WorkbenchTaskInput,
  type WorkbenchTaskSubmission,
} from './types/workbench'

function App() {
  return (
    <AuthProvider>
      <AppContent />
    </AuthProvider>
  )
}

function AppContent() {
  const auth = useAuth()

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

  return (
    <StudioWorkbench
      authError={auth.error}
      isAuthSubmitting={auth.isSubmitting}
      onLogout={auth.logout}
      session={auth.session}
    />
  )
}

interface StudioWorkbenchProps {
  authError: string | null
  isAuthSubmitting: boolean
  session: AuthSession
  onLogout: () => Promise<void>
}

function StudioWorkbench({ authError, isAuthSubmitting, onLogout, session }: StudioWorkbenchProps) {
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
  const [isAdminOpen, setAdminOpen] = useState(false)
  const [isObservabilityAdminOpen, setObservabilityAdminOpen] = useState(false)
  const [isIdentityAdminOpen, setIdentityAdminOpen] = useState(false)
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
  const [isProjectManagementOpen, setProjectManagementOpen] = useState(false)
  const [projectToManage, setProjectToManage] = useState<Project | null>(null)
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
  const canReadUsage = hasPermission(session, 'usage:read')
  const canReadAudit = hasPermission(session, 'audit:read')
  const canManageSystemSettings = hasPermission(session, 'system:settings:manage')
  const canReadUsers = hasPermission(session, 'user:read')
  const canCreateUsers = hasPermission(session, 'user:create')
  const canUpdateUsers = hasPermission(session, 'user:update')
  const canDisableUsers = hasPermission(session, 'user:disable')
  const canReadRoles = hasPermission(session, 'role:read')
  const canManageRoles = hasPermission(session, 'role:manage')
  const canManageTenant = tenantAdmin && canManageSystemSettings
  const canManageProjectMembers = hasPermission(session, 'project:member:manage')
  const canCreateProject = hasPermission(session, 'project:create')
  const canDeleteProject = hasPermission(session, 'project:delete')
  const canOpenAdmin = canManageProviders || canManageModels
  const canOpenObservabilityAdmin = canReadUsage || canReadAudit || canManageSystemSettings
  const canOpenIdentityAdmin = canReadUsers || canCreateUsers || canUpdateUsers || canDisableUsers || canReadRoles || canManageRoles || canManageTenant

  const managementItems: ManagementMenuItem[] = [
    ...(canOpenAdmin
      ? [
          {
            description: '配置 AI 中转站、模型能力与价格信息',
            icon: Settings,
            label: '中转站与模型管理',
            onSelect: () => setAdminOpen(true),
          },
        ]
      : []),
    ...(canOpenObservabilityAdmin
      ? [
          {
            description: '查看用量、审计事件与平台运行设置',
            icon: BarChart3,
            label: '观测与设置',
            onSelect: () => setObservabilityAdminOpen(true),
          },
        ]
      : []),
    ...(canOpenIdentityAdmin
      ? [
          {
            description: '管理租户用户、角色与权限',
            icon: UserCog,
            label: '用户/角色管理',
            onSelect: () => setIdentityAdminOpen(true),
          },
        ]
      : []),
  ]

  const handleGenerateTask = async (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput) => {
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
      setProjectManagementOpen(false)
      setProjectToManage(null)
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
      setProjectManagementOpen(false)
      setProjectToManage(null)
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
    setProjectManagementOpen(true)
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
    <AppShell
      accountSlot={
        <>
          <ManagementMenu items={managementItems} />
          <AuthStatus isSubmitting={isAuthSubmitting} onLogout={onLogout} session={session} />
        </>
      }
      notice={authError ?? notice}
    >
      <div className="grid gap-2 sm:gap-3">
        {projectAssets.isLoadingProjects ? (
          <div aria-busy="true" className="panel px-5 py-16 text-center text-sm text-ink-600" role="status">
            正在加载产品...
          </div>
        ) : !projectAssets.selectedProjectId ? (
          <WorkspaceOnboarding
            canCreateProject={canCreateProject}
            hasLoadError={projectAssets.projectStatus === 'error'}
            onCreateProject={() => handleManageProject()}
            onRetry={() => void projectAssets.refreshProjects()}
          />
        ) : (
          <div
            className="grid min-h-0 gap-2 sm:gap-3 lg:h-[calc(100dvh-88px)] lg:grid-rows-[auto_minmax(0,1fr)] lg:overflow-hidden"
            data-testid="fixed-product-workspace"
          >
            <div className="grid shrink-0 gap-2 sm:gap-3">
              <ProjectTabs
                onManageProject={handleManageProject}
                onSelectProject={projectAssets.selectProject}
                projects={projectAssets.projects}
                selectedProjectId={projectAssets.selectedProjectId}
              />

              {workbenchModels.status !== 'loading' && workbenchModels.models.length === 0 ? (
                <ModelSetupBanner canManageModels={canOpenAdmin} onOpen={() => setAdminOpen(true)} />
              ) : null}
            </div>

            <div
              className="grid min-w-0 grid-cols-1 gap-2 sm:gap-3 lg:h-full lg:min-h-0 lg:grid-cols-[104px_minmax(0,1fr)_300px] lg:overflow-hidden xl:grid-cols-[112px_minmax(0,1fr)_340px]"
              data-testid="generation-workbench"
            >
              <ImageTypeTabs activityByType={imageTypeActivity} imageType={imageType} onChange={setImageType} />

              <section aria-label="图片生成工作区" className="min-h-0 min-w-0">
                <div className="grid h-full min-h-0 min-w-0 grid-cols-1 gap-2 sm:gap-3 xl:grid-cols-[minmax(300px,340px)_minmax(0,1fr)]">
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
                    onRefreshModels={() => void workbenchModels.refreshModels()}
                    onSavePendingReferences={handleSavePendingReferences}
                    referenceToAdd={referenceToAdd}
                    prompt={promptDraft}
                    resetKey={projectAssets.selectedProjectId}
                  />

                  <ResultCanvas
                    canCancelTask={generation.canCancelCurrentTask}
                    canRetryTask={generation.canRetryCurrentTask}
                    current={generation.current}
                    currentItems={generation.currentItems}
                    error={generation.error}
                    onCancelTask={() => void generation.cancelCurrentTask()}
                    onDownload={() => void handleDownloadCurrent()}
                    onOpenDetail={handleOpenCurrentDetail}
                    onRename={() => void handleRenameCurrent()}
                    onRetryTask={() => void generation.retryCurrentTask()}
                    onOpenAssets={() => setWorkspaceLibraryOpen(true)}
                    onSelect={generation.selectCurrent}
                    pendingTaskAction={generation.pendingTaskAction}
                    selectedIndex={generation.selectedIndex}
                    status={generation.status}
                    taskStatus={generation.taskState?.status}
                  />
                </div>
              </section>

              <GenerationHistoryRail
                history={
                  <HistoryPanel
                    embedded
                    error={history.error}
                    isLoading={history.isLoading}
                    items={history.items}
                    kind={history.kind}
                    onDownload={(item) => void handleDownloadBackendHistory(item)}
                    onEdit={(item) => void handleEditBackendHistory(item)}
                    onRename={(item) => setAssetDetail(item.asset)}
                    favoriteActionAssetId={history.favoriteActionAssetId}
                    onKindChange={history.setKind}
                    onPageChange={history.setPageNum}
                    onPageSizeChange={history.setPageSize}
                    onRefresh={() => void history.refresh()}
                    onToggleFavorite={(item) => {
                      void history.toggleFavorite(item).then((updated) => {
                        if (updated) {
                          setNotice(updated.isFavorite ? '图片已收藏。' : '已取消收藏。')
                          void projectAssets.refreshAssets()
                        }
                      })
                    }}
                    onView={(item) => void handleOpenBackendDetail(item.asset.id, item.task.id)}
                    pageNum={history.pageNum}
                    pageSize={history.pageSize}
                    total={history.total}
                  />
                }
                onCancel={(taskId) => void generation.cancelTask(taskId)}
                onView={handleViewTask}
                imageType={imageType}
                pendingTaskId={generation.pendingAction?.taskId}
                projectId={projectAssets.selectedProjectId}
                projects={projectAssets.projects}
                tasks={generation.tasks}
              />
            </div>
          </div>
        )}
      </div>

      {canOpenAdmin ? (
        <ProviderModelAdminPanel
          canManageModels={canManageModels}
          canManageProviders={canManageProviders}
          csrfToken={session.csrfToken}
          isOpen={isAdminOpen}
          onClose={() => setAdminOpen(false)}
        />
      ) : null}

      {canOpenObservabilityAdmin ? (
        <AdminObservabilitySettingsPanel
          canManageSystemSettings={canManageSystemSettings}
          canReadAudit={canReadAudit}
          canReadUsage={canReadUsage}
          csrfToken={session.csrfToken}
          isOpen={isObservabilityAdminOpen}
          onClose={() => setObservabilityAdminOpen(false)}
        />
      ) : null}

      {canOpenIdentityAdmin ? (
        <UserRoleAdminPanel
          canCreateUsers={canCreateUsers}
          canDisableUsers={canDisableUsers}
          canManageRoles={canManageRoles}
          canManageModelAccess={tenantAdmin}
          canManageTenant={canManageTenant}
          canReadRoles={canReadRoles}
          canReadUsers={canReadUsers}
          canUpdateUsers={canUpdateUsers}
          csrfToken={session.csrfToken}
          currentUserId={session.user.id}
          isOpen={isIdentityAdminOpen}
          onClose={() => setIdentityAdminOpen(false)}
        />
      ) : null}

      <ProjectManagementModal
        actionUserId={projectAssets.projectMemberActionUserId}
        canDeleteProject={canDeleteProject}
        canManageProjectMembers={canManageProjectMembers}
        candidateStatus={projectAssets.projectMemberCandidateStatus}
        candidates={projectAssets.projectMemberCandidates}
        error={projectAssets.error}
        initialProject={projectToManage}
        isCreatingProject={projectAssets.isCreatingProject}
        isDeletingProject={projectAssets.isDeletingProject}
        isOpen={isProjectManagementOpen}
        isSavingMember={projectAssets.isSavingProjectMember}
        isUpdatingProject={projectAssets.isUpdatingProject}
        memberError={projectAssets.projectMemberError}
        members={projectAssets.projectMembers}
        memberStatus={projectAssets.projectMemberStatus}
        onAddMember={(projectId, request) => void handleAddProjectMember(projectId, request)}
        onClose={() => setProjectManagementOpen(false)}
        onCreateProject={handleCreateProject}
        onDeleteProject={(project) => void handleDeleteProject(project)}
        onRefreshCandidates={refreshProjectMemberCandidatesForModal}
        onRefreshMembers={refreshProjectMembersForModal}
        onRemoveMember={(projectId, userId) => void handleRemoveProjectMember(projectId, userId)}
        onSelectProject={projectAssets.selectProject}
        onUpdateMember={(projectId, userId, request) => void handleUpdateProjectMember(projectId, userId, request)}
        onUpdateProject={(projectId, request) => void handleUpdateProject(projectId, request)}
        projects={projectAssets.projects}
      />

      <Modal
        isOpen={isWorkspaceLibraryOpen}
        maxWidthClass="max-w-6xl"
        onClose={() => setWorkspaceLibraryOpen(false)}
        title="产品素材"
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
      </Modal>

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
