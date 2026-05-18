import { Download } from 'lucide-react'
import type { LegacyCurrentGeneration } from '../../hooks/useGeneration'
import { useObjectUrl } from '../../hooks/useObjectUrl'
import { formatBytes } from '../../lib/storageLimit'
import type { Asset, Task } from '../../types/platform'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

interface ImageDetailModalProps {
  isOpen: boolean
  detail: ImageDetail | null
  error?: string
  isLoading?: boolean
  onClose: () => void
  onDownload: () => void
}

export type ImageDetail =
  | {
      kind: 'legacy'
      current: LegacyCurrentGeneration
    }
  | {
      kind: 'backend'
      asset: Asset
      task: Task
    }

export function ImageDetailModal({ isOpen, detail, error, isLoading = false, onClose, onDownload }: ImageDetailModalProps) {
  const imageUrl = useObjectUrl(detail?.kind === 'legacy' ? detail.current.result.blob : undefined)
  const backendImageUrl = detail?.kind === 'backend' ? detail.asset.previewUrl ?? detail.asset.thumbnailUrl : undefined

  const rows = detail?.kind === 'legacy' ? buildLegacyRows(detail.current) : detail?.kind === 'backend' ? buildBackendRows(detail.asset, detail.task) : []

  return (
    <Modal
      footer={
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>关闭</Button>
          <Button disabled={!detail || isLoading} icon={<Download className="h-4 w-4" />} onClick={onDownload} variant="primary">
            下载原图
          </Button>
        </div>
      }
      isOpen={isOpen}
      onClose={onClose}
      title="图片详情"
    >
      {isLoading ? <p className="py-10 text-center text-sm text-ink-400">正在加载图片详情...</p> : null}

      {!isLoading && error ? (
        <div className="rounded-md border border-red-200 bg-red-50 px-3 py-3 text-sm leading-6 text-red-700" role="alert">
          {error}
        </div>
      ) : null}

      {!isLoading && detail ? (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
          <div className="overflow-hidden rounded-lg border border-ink-200 bg-ink-50">
            {detail.kind === 'legacy' && imageUrl ? (
              <img alt="生成结果预览" className="h-full max-h-[440px] w-full object-contain" src={imageUrl} />
            ) : null}
            {detail.kind === 'backend' && backendImageUrl ? (
              <img alt={`${detail.asset.filename} 预览`} className="h-full max-h-[440px] w-full object-contain" src={backendImageUrl} />
            ) : null}
          </div>
          <dl className="grid gap-3">
            {rows.map(([label, value]) => (
              <div className="rounded-md border border-ink-200 bg-ink-50 px-3 py-2" key={label}>
                <dt className="text-xs font-semibold uppercase tracking-normal text-ink-400">{label}</dt>
                <dd className="mt-1 break-words text-sm leading-6 text-ink-800">{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      ) : null}
    </Modal>
  )
}

function buildLegacyRows(current: LegacyCurrentGeneration) {
  const isNanoBanana = current.history.item.provider === 'gemini'
  return [
    ['Prompt', current.history.item.prompt],
    ['Model', current.history.item.model],
    ['Provider', current.history.item.provider],
    [isNanoBanana ? 'Quality' : 'Resolution', current.history.item.quality],
    [isNanoBanana ? 'Size' : 'Aspect Ratio', current.history.item.aspectRatio],
    ['Image Count', `${current.history.item.imageCount ?? 1} 张`],
    ['File Size', formatBytes(current.history.item.fileSize)],
    ['Width', `${current.history.item.width}px`],
    ['Height', `${current.history.item.height}px`],
    ['Created At', formatDateTime(current.history.item.createdAt)],
    ['Duration', `${current.history.item.durationMs}ms`],
  ]
}

function buildBackendRows(asset: Asset, task: Task) {
  return [
    ['Filename', asset.filename],
    ['Prompt', task.prompt],
    ['Task Status', task.status],
    ['Task Type', task.type],
    ['Model', task.modelId],
    ['Provider', task.providerId],
    ['Asset Kind', asset.kind],
    ['File Size', formatBytes(asset.fileSize)],
    ['Width', `${asset.width}px`],
    ['Height', `${asset.height}px`],
    ['Created At', formatDateTime(asset.createdAt)],
  ]
}

function formatDateTime(value: string) {
  return new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}
