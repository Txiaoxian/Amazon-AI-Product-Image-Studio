import { useEffect, useRef, useState } from 'react'
import { Settings } from 'lucide-react'
import type { HistoryWithImage } from './db/historyRepository'
import { AuthStatus } from './components/auth/AuthStatus'
import { LoginPanel } from './components/auth/LoginPanel'
import { ProviderModelAdminPanel } from './components/admin/ProviderModelAdminPanel'
import { AppShell } from './components/layout/AppShell'
import { HistoryPanel } from './components/history/HistoryPanel'
import { LegacyHistoryPanel } from './components/history/LegacyHistoryPanel'
import { ImageDetailModal, type ImageDetail } from './components/modals/ImageDetailModal'
import { AssetDetailModal } from './components/projects/AssetDetailModal'
import { ProjectAssetsPanel } from './components/projects/ProjectAssetsPanel'
import { SettingsModal } from './components/modals/SettingsModal'
import { ControlPanel, type ControlPanelDraft } from './components/studio/ControlPanel'
import { ResultCanvas } from './components/studio/ResultCanvas'
import { useWorkbenchModels } from './components/studio/useWorkbenchModels'
import { AuthProvider } from './hooks/AuthProvider'
import { useAuth } from './hooks/useAuth'
import { useGeneration } from './hooks/useGeneration'
import { useHistory } from './hooks/useHistory'
import { useProjectAssets } from './hooks/useProjectAssets'
import { useSettings } from './hooks/useSettings'
import { useStorageUsage } from './hooks/useStorageUsage'
import { downloadBlob } from './lib/download'
import { IMAGE_MODELS } from './providers/registry'
import type { GenerationRequest, ImageCount } from './providers/types'
import type { AuthSession } from './types/auth'
import type { BackendHistoryItem } from './types/history'
import type { Asset } from './types/platform'
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
  const { settings, setSettings } = useSettings()
  const projectAssets = useProjectAssets({ csrfToken: session.csrfToken })
  const history = useHistory({ projectId: projectAssets.selectedProjectId })
  const refreshHistory = history.refresh
  const workbenchModels = useWorkbenchModels()
  const generation = useGeneration(settings, {
    csrfToken: session.csrfToken,
    onModelInvalidated: workbenchModels.refreshModels,
    projectId: projectAssets.selectedProjectId,
  })
  const storageUsage = useStorageUsage()
  const [isSettingsOpen, setSettingsOpen] = useState(false)
  const [isAdminOpen, setAdminOpen] = useState(false)
  const [isDetailOpen, setDetailOpen] = useState(false)
  const [detail, setDetail] = useState<ImageDetail | null>(null)
  const [detailError, setDetailError] = useState('')
  const [isDetailLoading, setDetailLoading] = useState(false)
  const [assetDetail, setAssetDetail] = useState<Asset | null>(null)
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<ControlPanelDraft | null>(null)
  const [referenceToAdd, setReferenceToAdd] = useState<WorkbenchReferenceInput | null>(null)
  const [workbenchMode, setWorkbenchMode] = useState<'backend' | 'legacy-history'>('backend')
  const [pendingEditSourceAssetId, setPendingEditSourceAssetId] = useState<Asset['id'] | null>(null)
  const [isLegacyHistoryVisible, setLegacyHistoryVisible] = useState(false)
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
    setWorkbenchMode('backend')
    setLegacyHistoryVisible(false)
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
  const canOpenAdmin = canManageProviders || canManageModels

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

  const handleGenerateLegacy = async (request: GenerationRequest) => {
    const results = await generation.generateLegacy(request)

    if (!results) {
      return
    }

    if (results.length < request.imageCount) {
      setNotice(`兼容模式请求生成 ${request.imageCount} 张，图片服务实际返回 ${results.length} 张；结果仍保存到旧本地历史。`)
    } else {
      setNotice(`兼容模式已生成 ${results.length} 张图片，并保存到旧本地历史。`)
    }
    await history.refreshLegacy()
    await storageUsage.refresh()
  }

  const handleViewLegacy = (item: HistoryWithImage) => {
    if (!item.image) {
      showNotice('历史记录中的原图不存在，无法查看。')
      return
    }

    generation.setFromHistory(item)
    setDetail({
      kind: 'legacy',
      current: {
        kind: 'legacy',
        history: item,
        result: {
          blob: item.image?.blob ?? new Blob(),
          mimeType: item.image?.mimeType ?? 'image/png',
          width: item.item.width,
          height: item.item.height,
          fileSize: item.item.fileSize,
          durationMs: item.item.durationMs,
        },
      },
    })
    setDetailError('')
    setDetailOpen(true)
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

    if (generation.current.kind === 'legacy') {
      downloadBlob(generation.current.result.blob)
      return
    }

    const download = await generation.downloadCurrentBackendAsset()
    if (download) {
      downloadBlob(download.blob, download.filename)
    }
  }

  const handleDownloadLegacyHistory = (item: HistoryWithImage) => {
    if (!item.image) {
      showNotice('历史记录中的原图不存在，无法下载。')
      return
    }

    downloadBlob(item.image.blob)
  }

  const handleEditLegacy = (item: HistoryWithImage) => {
    if (!item.image) {
      showNotice('历史记录中的原图不存在，无法再次编辑。')
      return
    }

    const model = IMAGE_MODELS.find((candidate) => candidate.provider === item.item.provider && candidate.model === item.item.model) ?? IMAGE_MODELS[0]
    const file = new File([item.image.blob], `reference-${item.item.id}.png`, { type: item.image.mimeType })
    const reference: WorkbenchReferenceInput = {
      kind: 'pending',
      file,
      previewUrl: URL.createObjectURL(file),
    }

    generation.setFromHistory(item)
    setPendingEditSourceAssetId(null)
    setDraft({
      prompt: item.item.prompt,
      modelId: model.id,
      quality: item.item.quality,
      aspectRatio: item.item.aspectRatio,
      imageCount: 1,
      references: [reference],
    })
    setWorkbenchMode('legacy-history')
    setNotice('已进入旧本地历史兼容模式，可调整提示词后再次生成。')
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
        quality: settings.defaultResolution,
        aspectRatio: '1:1',
        imageCount: getTaskOutputCount(backendDetail.task),
      })
      setWorkbenchMode('backend')
      setNotice('已准备基于后端资产再次编辑。')
    } catch {
      setPendingEditSourceAssetId(null)
      setNotice('再次编辑所需资产不可用，请刷新历史后重试。')
    }
  }

  const handleDeleteLegacy = async (item: HistoryWithImage) => {
    if (!window.confirm('确定删除这条历史记录吗？')) {
      return
    }

    await history.remove(item.item.id)
    await storageUsage.refresh()
    setNotice('历史记录已删除。')
  }

  const handleClearLegacy = async () => {
    if (!window.confirm('确定清空全部历史记录吗？此操作不可恢复。')) {
      return
    }

    await history.clear()
    await storageUsage.refresh()
    setNotice('全部历史记录已清空。')
  }

  const handleCreateProject = async (request: { name: string; brand?: string; asin?: string; site?: string; notes?: string }) => {
    const project = await projectAssets.createProject(request)

    if (project) {
      setNotice(`已创建项目：${project.name}`)
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
    }
  }

  const handleOpenCurrentDetail = () => {
    if (!generation.current) {
      return
    }

    if (generation.current.kind === 'legacy') {
      setDetail({
        kind: 'legacy',
        current: generation.current,
      })
      setDetailError('')
      setDetailLoading(false)
      setDetailOpen(true)
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
          <AuthStatus isSubmitting={isAuthSubmitting} onLogout={onLogout} session={session} />
        </>
      }
      notice={authError ?? notice}
      onOpenSettings={() => setSettingsOpen(true)}
    >
      <div className="grid grid-cols-1 gap-3 sm:gap-4 xl:min-h-[calc(100dvh-112px)] xl:grid-cols-[360px_minmax(0,1fr)_380px]">
        <div className="flex min-h-0 flex-col gap-3">
          {workbenchMode === 'legacy-history' ? (
            <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm leading-6 text-amber-900">
              <div className="font-semibold">旧本地历史兼容模式</div>
              <div className="mt-1 text-xs">当前再次编辑仍沿用旧本地数据源；默认新任务已切换到后端 task API。</div>
              <button
                className="mt-2 text-xs font-semibold text-amber-900 underline decoration-amber-400 underline-offset-2"
                onClick={() => {
                  setPendingEditSourceAssetId(null)
                  setWorkbenchMode('backend')
                  setDraft(null)
                }}
                type="button"
              >
                返回后端工作台
              </button>
            </div>
          ) : null}

          {workbenchMode === 'backend' ? (
            <ControlPanel
              defaultModelId={settings.defaultModelId}
              defaultResolution={settings.defaultResolution}
              draft={draft}
              isGenerating={generation.status === 'loading'}
              modelStatus={workbenchModels.status}
              models={workbenchModels.models}
              onError={showNotice}
              onGenerate={handleGenerateTask}
              onReferenceAdded={() => setReferenceToAdd(null)}
              onRefreshModels={() => void workbenchModels.refreshModels()}
              referenceToAdd={referenceToAdd}
              submissionMode="backend"
            />
          ) : (
            <ControlPanel
              defaultModelId={settings.defaultModelId}
              defaultResolution={settings.defaultResolution}
              draft={draft}
              isGenerating={generation.status === 'loading'}
              onError={showNotice}
              onGenerate={handleGenerateLegacy}
              onReferenceAdded={() => setReferenceToAdd(null)}
              referenceToAdd={referenceToAdd}
              submissionMode="legacy"
            />
          )}
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
            assets={projectAssets.assets}
            error={projectAssets.error}
            isCreatingProject={projectAssets.isCreatingProject}
            isLoadingAssets={projectAssets.isLoadingAssets}
            isLoadingProjects={projectAssets.isLoadingProjects}
            isUploadingAsset={projectAssets.isUploadingAsset}
            onCreateProject={handleCreateProject}
            onDeleteAsset={handleDeleteAsset}
            onDownloadAsset={handleDownloadAsset}
            onOpenAsset={setAssetDetail}
            onRefreshAssets={() => void projectAssets.refreshAssets()}
            onRefreshProjects={() => void projectAssets.refreshProjects()}
            onSelectProject={projectAssets.selectProject}
            onToggleFavorite={(asset) => void projectAssets.toggleFavorite(asset)}
            onUploadReferences={(files) => void handleUploadReferences(files)}
            onUseAssetAsReference={(asset) => void handleUseAssetAsReference(asset)}
            projects={projectAssets.projects}
            selectedProjectId={projectAssets.selectedProjectId}
          />

          <HistoryPanel
            error={history.error}
            isLoading={history.isLoading}
            items={history.items}
            onDownload={(item) => void handleDownloadBackendHistory(item)}
            onEdit={(item) => void handleEditBackendHistory(item)}
            onRefresh={() => void history.refresh()}
            onView={(item) => void handleOpenBackendDetail(item.asset.id, item.task.id)}
          />

          <button
            aria-label="查看旧本地历史"
            className="rounded-md border border-ink-200 bg-white px-4 py-3 text-left text-sm font-semibold text-ink-700"
            onClick={() => setLegacyHistoryVisible((visible) => !visible)}
            type="button"
          >
            查看旧本地历史
          </button>
          {isLegacyHistoryVisible ? (
            <LegacyHistoryPanel
              error={history.legacyError}
              isLoading={history.isLegacyLoading}
              items={history.legacyItems}
              limitBytes={settings.storageLimitBytes}
              onClear={handleClearLegacy}
              onDelete={handleDeleteLegacy}
              onDownload={handleDownloadLegacyHistory}
              onEdit={handleEditLegacy}
              onView={handleViewLegacy}
              usedBytes={storageUsage.usedBytes}
            />
          ) : null}
        </div>
      </div>

      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setSettingsOpen(false)}
        onSave={setSettings}
        settings={settings}
      />

      {canOpenAdmin ? (
        <ProviderModelAdminPanel
          canManageModels={canManageModels}
          canManageProviders={canManageProviders}
          csrfToken={session.csrfToken}
          isOpen={isAdminOpen}
          onClose={() => setAdminOpen(false)}
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
        onDownload={(asset) => void handleDownloadAsset(asset)}
        onUseAsReference={(asset) => void handleUseAssetAsReference(asset)}
      />
    </AppShell>
  )
}

function getTaskOutputCount(task: BackendHistoryItem['task']): ImageCount {
  const outputCount = task.parameters.outputCount
  return outputCount === 1 || outputCount === 2 || outputCount === 3 || outputCount === 4 ? outputCount : 1
}

function hasPermission(session: AuthSession, permission: string): boolean {
  return session.permissions.some((candidate) => candidate === permission)
}

export default App
