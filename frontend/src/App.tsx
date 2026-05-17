import { useEffect, useState } from 'react'
import { Settings } from 'lucide-react'
import type { HistoryWithImage } from './db/historyRepository'
import { AuthStatus } from './components/auth/AuthStatus'
import { LoginPanel } from './components/auth/LoginPanel'
import { ProviderModelAdminPanel } from './components/admin/ProviderModelAdminPanel'
import { AppShell } from './components/layout/AppShell'
import { HistoryPanel } from './components/history/HistoryPanel'
import { ImageDetailModal } from './components/modals/ImageDetailModal'
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
import type { GenerationRequest } from './providers/types'
import type { AuthSession } from './types/auth'
import type { Asset } from './types/platform'
import type { WorkbenchReferenceInput, WorkbenchTaskInput } from './types/workbench'

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
  const generation = useGeneration(settings)
  const history = useHistory()
  const projectAssets = useProjectAssets({ csrfToken: session.csrfToken })
  const workbenchModels = useWorkbenchModels()
  const storageUsage = useStorageUsage()
  const [isSettingsOpen, setSettingsOpen] = useState(false)
  const [isAdminOpen, setAdminOpen] = useState(false)
  const [isDetailOpen, setDetailOpen] = useState(false)
  const [assetDetail, setAssetDetail] = useState<Asset | null>(null)
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<ControlPanelDraft | null>(null)
  const [referenceToAdd, setReferenceToAdd] = useState<WorkbenchReferenceInput | null>(null)

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

  const showNotice = (message: string) => {
    setNotice(message)
  }

  const canManageProviders = hasPermission(session, 'provider:manage')
  const canManageModels = hasPermission(session, 'model:manage')
  const canOpenAdmin = canManageProviders || canManageModels

  const handleGenerate = async (request: GenerationRequest, workbenchInput: WorkbenchTaskInput) => {
    void workbenchInput
    const results = await generation.generate(request)

    if (results) {
      if (results.length < request.imageCount) {
        setNotice(`请求生成 ${request.imageCount} 张，图片服务实际返回 ${results.length} 张；已将返回的图片保存到本地历史记录。`)
      } else {
        setNotice(`已生成 ${results.length} 张图片，并保存到本地历史记录。`)
      }
      await history.refresh()
      await storageUsage.refresh()
    }
  }

  const handleView = (item: HistoryWithImage) => {
    generation.setFromHistory(item)
    setDetailOpen(true)
  }

  const handleDownloadCurrent = () => {
    if (generation.current) {
      downloadBlob(generation.current.result.blob)
    }
  }

  const handleDownloadHistory = (item: HistoryWithImage) => {
    if (!item.image) {
      showNotice('历史记录中的原图不存在，无法下载。')
      return
    }

    downloadBlob(item.image.blob)
  }

  const handleEdit = (item: HistoryWithImage) => {
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
    setDraft({
      prompt: item.item.prompt,
      modelId: model.id,
      quality: item.item.quality,
      aspectRatio: item.item.aspectRatio,
      imageCount: 1,
      references: [reference],
    })
    setNotice('已将历史图片加载为参考图，可调整提示词后再次生成。')
  }

  const handleDelete = async (item: HistoryWithImage) => {
    if (!window.confirm('确定删除这条历史记录吗？')) {
      return
    }

    await history.remove(item.item.id)
    await storageUsage.refresh()
    setNotice('历史记录已删除。')
  }

  const handleClear = async () => {
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
    setReferenceToAdd(projectAssets.createReferenceFromAsset(asset))
    setNotice('已将项目资产加入参考图。')
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
        <ControlPanel
          defaultModelId={settings.defaultModelId}
          defaultResolution={settings.defaultResolution}
          draft={draft}
          isGenerating={generation.status === 'loading'}
          modelStatus={workbenchModels.status}
          models={workbenchModels.models}
          onError={showNotice}
          onGenerate={handleGenerate}
          onRefreshModels={() => void workbenchModels.refreshModels()}
          onReferenceAdded={() => setReferenceToAdd(null)}
          referenceToAdd={referenceToAdd}
        />

        <ResultCanvas
          current={generation.current}
          currentItems={generation.currentItems}
          error={generation.error}
          onDownload={handleDownloadCurrent}
          onOpenDetail={() => setDetailOpen(true)}
          onSelect={generation.selectCurrent}
          selectedIndex={generation.selectedIndex}
          status={generation.status}
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
            isLoading={history.isLoading}
            items={history.items}
            limitBytes={settings.storageLimitBytes}
            onClear={handleClear}
            onDelete={handleDelete}
            onDownload={handleDownloadHistory}
            onEdit={handleEdit}
            onView={handleView}
            usedBytes={storageUsage.usedBytes}
          />
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
        current={generation.current}
        isOpen={isDetailOpen}
        onClose={() => setDetailOpen(false)}
        onDownload={handleDownloadCurrent}
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

function hasPermission(session: AuthSession, permission: string): boolean {
  return session.permissions.some((candidate) => candidate === permission)
}

export default App
