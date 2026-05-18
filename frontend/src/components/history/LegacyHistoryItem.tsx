import { Download, Eye, Pencil, Trash2 } from 'lucide-react'
import type { HistoryWithImage } from '../../db/historyRepository'
import { useObjectUrl } from '../../hooks/useObjectUrl'
import { formatBytes } from '../../lib/storageLimit'

interface LegacyHistoryItemProps {
  history: HistoryWithImage
  onView: (history: HistoryWithImage) => void
  onEdit: (history: HistoryWithImage) => void
  onDownload: (history: HistoryWithImage) => void
  onDelete: (history: HistoryWithImage) => void
}

export function LegacyHistoryItem({ history, onView, onEdit, onDownload, onDelete }: LegacyHistoryItemProps) {
  const imageUrl = useObjectUrl(history.image?.blob)
  const createdAt = new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(history.item.createdAt))

  return (
    <article className="rounded-lg border border-ink-200 bg-white p-2">
      <div className="flex gap-3">
        <button
          className="h-20 w-20 shrink-0 overflow-hidden rounded-md bg-ink-100"
          disabled={!history.image}
          onClick={() => onView(history)}
          type="button"
        >
          {imageUrl ? <img alt={history.item.prompt} className="h-full w-full object-cover" src={imageUrl} /> : null}
        </button>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-ink-900">{history.item.modelLabel}</p>
          <p className="mt-1 text-xs text-ink-500">
            {history.item.quality} · {history.item.aspectRatio}
          </p>
          <p className="mt-1 text-xs text-ink-400">
            {createdAt} · {formatBytes(history.item.fileSize)}
          </p>
          <div className="mt-2 flex gap-1">
            <button aria-label="查看历史图片" className="icon-button h-8 w-8" onClick={() => onView(history)} title="查看" type="button">
              <Eye className="h-4 w-4" />
            </button>
            <button aria-label="再次编辑" className="icon-button h-8 w-8" onClick={() => onEdit(history)} title="再次编辑" type="button">
              <Pencil className="h-4 w-4" />
            </button>
            <button aria-label="下载历史原图" className="icon-button h-8 w-8" onClick={() => onDownload(history)} title="下载" type="button">
              <Download className="h-4 w-4" />
            </button>
            <button aria-label="删除历史记录" className="icon-button h-8 w-8 hover:border-red-200 hover:bg-red-50 hover:text-red-700" onClick={() => onDelete(history)} title="删除" type="button">
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </article>
  )
}
