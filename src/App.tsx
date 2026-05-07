import { useEffect, useState } from 'react'
import type { HistoryWithImage } from './db/historyRepository'
import { AppShell } from './components/layout/AppShell'
import { HistoryPanel } from './components/history/HistoryPanel'
import { ImageDetailModal } from './components/modals/ImageDetailModal'
import { SettingsModal } from './components/modals/SettingsModal'
import { ControlPanel, type ControlPanelDraft } from './components/studio/ControlPanel'
import { ResultCanvas } from './components/studio/ResultCanvas'
import { useGeneration } from './hooks/useGeneration'
import { useHistory } from './hooks/useHistory'
import { useSettings } from './hooks/useSettings'
import { useStorageUsage } from './hooks/useStorageUsage'
import { downloadBlob } from './lib/download'
import { IMAGE_MODELS } from './providers/registry'
import type { GenerationRequest, ReferenceImageInput } from './providers/types'

function App() {
  const { settings, setSettings } = useSettings()
  const generation = useGeneration(settings)
  const history = useHistory()
  const storageUsage = useStorageUsage()
  const [isSettingsOpen, setSettingsOpen] = useState(false)
  const [isDetailOpen, setDetailOpen] = useState(false)
  const [notice, setNotice] = useState('')
  const [draft, setDraft] = useState<ControlPanelDraft | null>(null)

  useEffect(() => {
    if (generation.error) {
      setNotice(generation.error)
    }
  }, [generation.error])

  const showNotice = (message: string) => {
    setNotice(message)
  }

  const handleGenerate = async (request: GenerationRequest) => {
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
    const reference: ReferenceImageInput = {
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

  return (
    <AppShell notice={notice} onOpenSettings={() => setSettingsOpen(true)}>
      <div className="grid grid-cols-1 gap-3 sm:gap-4 xl:min-h-[calc(100dvh-112px)] xl:grid-cols-[360px_minmax(0,1fr)_340px]">
        <ControlPanel
          defaultModelId={settings.defaultModelId}
          defaultResolution={settings.defaultResolution}
          draft={draft}
          isGenerating={generation.status === 'loading'}
          onError={showNotice}
          onGenerate={handleGenerate}
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

      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setSettingsOpen(false)}
        onSave={setSettings}
        settings={settings}
      />

      <ImageDetailModal
        current={generation.current}
        isOpen={isDetailOpen}
        onClose={() => setDetailOpen(false)}
        onDownload={handleDownloadCurrent}
      />
    </AppShell>
  )
}

export default App
