import { Download } from 'lucide-react'
import type { CurrentGeneration } from '../../hooks/useGeneration'
import { useObjectUrl } from '../../hooks/useObjectUrl'
import { formatBytes } from '../../lib/storageLimit'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

interface ImageDetailModalProps {
  isOpen: boolean
  current: CurrentGeneration | null
  onClose: () => void
  onDownload: () => void
}

export function ImageDetailModal({ isOpen, current, onClose, onDownload }: ImageDetailModalProps) {
  const imageUrl = useObjectUrl(current?.result.blob)

  if (!current) {
    return null
  }

  const isNanoBanana = current.history.item.provider === 'gemini'
  const rows = [
    ['Prompt', current.history.item.prompt],
    ['Model', current.history.item.model],
    ['Provider', current.history.item.provider],
    [isNanoBanana ? 'Quality' : 'Resolution', current.history.item.quality],
    [isNanoBanana ? 'Size' : 'Aspect Ratio', current.history.item.aspectRatio],
    ['Image Count', `${current.history.item.imageCount ?? 1} 张`],
    ['File Size', formatBytes(current.history.item.fileSize)],
    ['Width', `${current.history.item.width}px`],
    ['Height', `${current.history.item.height}px`],
    ['Created At', new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(current.history.item.createdAt))],
    ['Duration', `${current.history.item.durationMs}ms`],
  ]

  return (
    <Modal
      footer={
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>关闭</Button>
          <Button icon={<Download className="h-4 w-4" />} onClick={onDownload} variant="primary">
            下载原图
          </Button>
        </div>
      }
      isOpen={isOpen}
      onClose={onClose}
      title="图片详情"
    >
      <div className="grid gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <div className="overflow-hidden rounded-lg border border-ink-200 bg-ink-50">
          {imageUrl ? <img alt="生成结果预览" className="h-full max-h-[440px] w-full object-contain" src={imageUrl} /> : null}
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
    </Modal>
  )
}
