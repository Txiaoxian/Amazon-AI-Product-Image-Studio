import { useEffect, useRef, useState } from 'react'
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
import { BackendControlPanel, type BackendControlPanelDraft } from './components/studio/BackendControlPanel'
import { ResultCanvas } from './components/studio/ResultCanvas'
import { useWorkbenchModels } from './components/studio/useWorkbenchModels'
import { AuthProvider } from './hooks/AuthProvider'
import { useAuth } from './hooks/useAuth'
import { useGeneration } from './hooks/useGeneration'
import { useHistory } from './hooks/useHistory'
import { useProjectAssets } from './hooks/useProjectAssets'
import { downloadBlob } from './lib/download'
import type { AuthSession } from './types/auth'
import type { BackendHistoryItem } from './types/history'
import type { Asset, ProjectMemberRole, UserId } from './types/platform'
import type { WorkbenchReferenceInput, WorkbenchTaskInput, WorkbenchTaskSubmission } from './types/workbench'

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
  const history = useHistory({ projectId: projectAssets.selectedProjectId })
  const refreshHistory = history.refresh
  const workbenchModels = useWorkbenchModels()
  const generation = useGeneration({
    csrfToken: session.csrfToken,
    onModelInvalidated: workbenchModels.refreshModels,
    projectId: projectAssets.selectedProjectId,
  })
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
  const [referenceToAdd, setReferenceToAdd] = useState<WorkbenchReferenceInput | null>(null)
  const [pendingEditSourceAssetId, setPendingEditSourceAssetId] = useState<Asset['id'] | null>(null)
  const selectedProjectIdRef = useRef(projectAssets.selectedProjectId)
  const detailRequestVersionRef = useRef(0)

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
    setDraft(null)
    setReferenceToAdd(null)
  }, [projectAssets.selectedProjectId])

  useEffect(() => {
    if (generation.taskState?.status === 'SUCCEEDED') {
      void refreshHistory()
    }
  }, [generation.taskState?.status, refreshHistory])

  const showNotice = (message: string) => {
    setNotice(message)
  }

  const canManageProviders = hasPermission(session, 'provider:manage')
  const canManageModels = hasPermission(session, 'model:manage')
  const canReadUsage = hasPermission(session, 'usage:read')
  const canReadAudit = hasPermission(session, 'audit:read')
  const canManageSystemSettings = hasPermission(session, 'system:settings:manage')
  const canReadUsers = hasPermission(session, 'user:read')
  const canCreateUsers = hasPermission(session, 'user:create')
  const canUpdateUsers = hasPermission(session, 'user:update')
  const canDisableUsers = hasPermission(session, 'user:disable')
  const canReadRoles = hasPermission(session, 'role:read')
  const canManageRoles = hasPermission(session, 'role:manage')
  const canManageProjectMembers = hasPermission(session, 'project:member:manage')
  const canOpenAdmin = canManageProviders || canManageModels
  const canOpenObservabilityAdmin = canReadUsage || canReadAudit || canManageSystemSettings
  const canOpenIdentityAdmin = canReadUsers || canCreateUsers || canUpdateUsers || canDisableUsers || canReadRoles || canManageRoles

  const handleGenerateTask = async (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput) => {
    if (pendingEditSourceAssetId) {
      const isAvailable = await history.ensureBackendAssetAvailable(pendingEditSourceAssetId)
      if (!isAvailable) {
        setNotice('再次编辑所需资产不可用，请刷新历史后重试。')
        setPendingEditSourceAssetId(null)
        return
      }
    }

    const task = await generation.generateTask(
      request,
      pendingEditSourceAssetId ? { ...workbenchInput, editSourceAssetId: pendingEditSourceAssetId } : workbenchInput,
    )

    if (task) {
      setPendingEditSourceAssetId(null)
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
      setDraft({
        prompt: backendDetail.task.prompt,
        modelId: backendDetail.task.modelId,
        imageCount: getTaskOutputCount(backendDetail.task),
      })
      setNotice('已准备基于后端资产再次编辑。')
    } catch {
      setPendingEditSourceAssetId(null)
      setNotice('再次编辑所需资产不可用，请刷新历史后重试。')
    }
  }

  const handleCreateProject = async (request: { name: string; brand?: string; asin?: string; site?: string; notes?: string }) => {
    const project = await projectAssets.createProject(request)

    if (project) {
      setNotice(`已创建项目：${project.name}`)
    }
  }

  const handleUpdateProject = async (
    projectId: Asset['projectId'],
    request: { name?: string; brand?: string; asin?: string; site?: string; notes?: string },
  ) => {
    const project = await projectAssets.updateProject(projectId, request)

    if (project) {
      setNotice(`已更新项目：${project.name}`)
    }
  }

  const handleUploadReferences = async (files: FileList) => {
    const result = await projectAssets.uploadReferences(files)

    if (result.assets.length > 0) {
      setNotice(`已上传 ${result.assets.length} 张参考图到项目资产库。`)
    } else if (result.skipped > 0) {
      setNotice('参考图未上传，请检查图片格式和大小。')
    }
  }

  const handleDownloadAsset = async (asset: Asset) => {
    const download = await projectAssets.downloadAsset(asset)

    if (download) {
      downloadBlob(download.blob, download.filename ?? asset.filename)
      setNotice('项目资产下载已开始。')
    }
  }

  const handleUseAssetAsReference = async (asset: Asset) => {
    const reference = await projectAssets.createReferenceFromAsset(asset)

    if (reference) {
      setReferenceToAdd(reference)
      setNotice('已将项目资产加入参考图。')
    }
  }

  const handleDeleteAsset = async (asset: Asset) => {
    if (!window.confirm(`确定删除项目资产 ${asset.filename} 吗？`)) {
      return
    }

    const deleted = await projectAssets.deleteAsset(asset)

    if (deleted) {
      setNotice('项目资产已删除。')
      if (assetDetail?.id === asset.id) {
        setAssetDetail(null)
      }
      await projectAssets.refreshAssets()
    }
  }

  const handleUpdateAsset = async (
    asset: Asset,
    request: { filename?: string; category?: string; isFavorite?: boolean },
  ) => {
    const updated = await projectAssets.updateAsset(asset, request)

    if (updated) {
      setAssetDetail(updated)
      setNotice('资产元数据已更新。')
    }
  }

  const handleAddProjectMember = async (
    projectId: Asset['projectId'],
    request: { userId: UserId | string; role: ProjectMemberRole },
  ) => {
    const member = await projectAssets.addProjectMember(projectId, request)
    if (member) {
      setNotice('项目成员已添加。')
    }
  }

  const handleUpdateProjectMember = async (
    projectId: Asset['projectId'],
    userId: UserId | string,
    request: { role: ProjectMemberRole },
  ) => {
    const member = await projectAssets.updateProjectMember(projectId, userId, request)
    if (member) {
      setNotice('项目成员已更新。')
    }
  }

  const handleRemoveProjectMember = async (projectId: Asset['projectId'], userId: UserId | string) => {
    if (!window.confirm(`确定移除项目成员 ${userId} 吗？`)) {
      return
    }

    const removed = await projectAssets.removeProjectMember(projectId, userId)
    if (removed) {
      setNotice('项目成员已移除。')
    }
  }

  const handleOpenCurrentDetail = () => {
    if (!generation.current || generation.current.kind !== 'backend') {
      return
    }

    void handleOpenBackendDetail(generation.current.result.assetId, generation.current.task.id)
  }

  const handleDownloadDetail = async () => {
    if (detail?.kind === 'backend') {
      const download = await history.downloadBackendAsset(detail.asset)
      if (!download) {
        setNotice('结果下载失败，请稍后重试。')
        return
      }

      downloadBlob(download.blob, download.filename ?? detail.asset.filename)
      return
    }

    await handleDownloadCurrent()
  }

  return (
    <AppShell
      accountSlot={
        <>
          {canOpenAdmin ? (
            <button
              aria-label="Provider/model 管理"
              className="inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-ink-200 bg-white px-3 py-2 text-sm font-semibold text-ink-700 transition hover:bg-ink-50 hover:text-ink-900 focus:outline-none focus:ring-2 focus:ring-amazon-500/30"
              onClick={() => setAdminOpen(true)}
              title="Provider/model 管理"
              type="button"
            >
              <Settings className="h-4 w-4" />
              <span className="hidden sm:inline">Provider/model 管理</span>
            </button>
          ) : null}
          {canOpenObservabilityAdmin ? (
            <button
              aria-label="观测与设置"
              className="inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-ink-200 bg-white px-3 py-2 text-sm font-semibold text-ink-700 transition hover:bg-ink-50 hover:text-ink-900 focus:outline-none focus:ring-2 focus:ring-amazon-500/30"
              onClick={() => setObservabilityAdminOpen(true)}
              title="观测与设置"
              type="button"
            >
              <BarChart3 className="h-4 w-4" />
              <span className="hidden sm:inline">观测与设置</span>
            </button>
          ) : null}
          {canOpenIdentityAdmin ? (
            <button
              aria-label="用户/角色管理"
              className="inline-flex min-h-9 items-center justify-center gap-2 rounded-md border border-ink-200 bg-white px-3 py-2 text-sm font-semibold text-ink-700 transition hover:bg-ink-50 hover:text-ink-900 focus:outline-none focus:ring-2 focus:ring-amazon-500/30"
              onClick={() => setIdentityAdminOpen(true)}
              title="用户/角色管理"
              type="button"
            >
              <UserCog className="h-4 w-4" />
              <span className="hidden sm:inline">用户/角色管理</span>
            </button>
          ) : null}
          <AuthStatus isSubmitting={isAuthSubmitting} onLogout={onLogout} session={session} />
        </>
      }
      notice={authError ?? notice}
    >
      <div className="grid grid-cols-1 gap-3 sm:gap-4 xl:min-h-[calc(100dvh-112px)] xl:grid-cols-[360px_minmax(0,1fr)_380px]">
        <div className="flex min-h-0 flex-col gap-3">
          <BackendControlPanel
            draft={draft}
            isGenerating={generation.status === 'loading'}
            modelStatus={workbenchModels.status}
            models={workbenchModels.models}
            onError={showNotice}
            onGenerate={handleGenerateTask}
            onReferenceAdded={() => setReferenceToAdd(null)}
            onRefreshModels={() => void workbenchModels.refreshModels()}
            referenceToAdd={referenceToAdd}
            resetKey={projectAssets.selectedProjectId ?? 'no-project'}
          />
        </div>

        <ResultCanvas
          canCancelTask={generation.canCancelCurrentTask}
          canRetryTask={generation.canRetryCurrentTask}
          current={generation.current}
          currentItems={generation.currentItems}
          error={generation.error}
          onCancelTask={() => void generation.cancelCurrentTask()}
          onDownload={() => void handleDownloadCurrent()}
          onOpenDetail={handleOpenCurrentDetail}
          onRetryTask={() => void generation.retryCurrentTask()}
          onSelect={generation.selectCurrent}
          pendingTaskAction={generation.pendingTaskAction}
          selectedIndex={generation.selectedIndex}
          status={generation.status}
          taskStatus={generation.taskState?.status}
        />

        <div className="flex min-h-0 flex-col gap-3">
          <ProjectAssetsPanel
            actionAssetId={projectAssets.actionAssetId}
            assetFilters={projectAssets.assetFilters}
            assetStatus={projectAssets.assetStatus}
            assets={projectAssets.assets}
            canManageProjectMembers={canManageProjectMembers}
            error={projectAssets.error}
            isCreatingProject={projectAssets.isCreatingProject}
            isLoadingAssets={projectAssets.isLoadingAssets}
            isLoadingProjects={projectAssets.isLoadingProjects}
            isSavingProjectMember={projectAssets.isSavingProjectMember}
            isUpdatingProject={projectAssets.isUpdatingProject}
            isUploadingAsset={projectAssets.isUploadingAsset}
            onAddProjectMember={(projectId, request) => void handleAddProjectMember(projectId, request)}
            onCreateProject={handleCreateProject}
            onDeleteAsset={handleDeleteAsset}
            onDownloadAsset={handleDownloadAsset}
            onOpenAsset={setAssetDetail}
            onRefreshAssets={() => void projectAssets.refreshAssets()}
            onRefreshProjectMembers={(projectId) => void projectAssets.refreshProjectMembers(projectId)}
            onRefreshProjects={() => void projectAssets.refreshProjects()}
            onRemoveProjectMember={(projectId, userId) => void handleRemoveProjectMember(projectId, userId)}
            onSelectProject={projectAssets.selectProject}
            onToggleFavorite={(asset) => void projectAssets.toggleFavorite(asset)}
            onUpdateAssetFilters={projectAssets.updateAssetFilters}
            onUpdateProjectMember={(projectId, userId, request) => void handleUpdateProjectMember(projectId, userId, request)}
            onUpdateProject={(projectId, request) => void handleUpdateProject(projectId, request)}
            onUploadReferences={(files) => void handleUploadReferences(files)}
            onUseAssetAsReference={(asset) => void handleUseAssetAsReference(asset)}
            projectMemberActionUserId={projectAssets.projectMemberActionUserId}
            projectMemberError={projectAssets.projectMemberError}
            projectMembers={projectAssets.projectMembers}
            projectMemberStatus={projectAssets.projectMemberStatus}
            projectStatus={projectAssets.projectStatus}
            projects={projectAssets.projects}
            selectedProject={projectAssets.selectedProject}
            selectedProjectId={projectAssets.selectedProjectId}
          />

          <HistoryPanel
            error={history.error}
            isLoading={history.isLoading}
            items={history.items}
            kind={history.kind}
            onDownload={(item) => void handleDownloadBackendHistory(item)}
            onEdit={(item) => void handleEditBackendHistory(item)}
            onKindChange={history.setKind}
            onPageChange={history.setPageNum}
            onPageSizeChange={history.setPageSize}
            onRefresh={() => void history.refresh()}
            onView={(item) => void handleOpenBackendDetail(item.asset.id, item.task.id)}
            pageNum={history.pageNum}
            pageSize={history.pageSize}
            total={history.total}
          />
        </div>
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
          canReadRoles={canReadRoles}
          canReadUsers={canReadUsers}
          canUpdateUsers={canUpdateUsers}
          csrfToken={session.csrfToken}
          currentUserId={session.user.id}
          isOpen={isIdentityAdminOpen}
          onClose={() => setIdentityAdminOpen(false)}
        />
      ) : null}

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

export default App
