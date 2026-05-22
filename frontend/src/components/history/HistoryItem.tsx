import { Download, Eye, Pencil } from 'lucide-react'
import { formatBytes } from '../../lib/storageLimit'
import type { BackendHistoryItem } from '../../types/history'

interface HistoryItemProps {
  history: BackendHistoryItem
  onView: (history: BackendHistoryItem) => void
  onEdit: (history: BackendHistoryItem) => void
  onDownload: (history: BackendHistoryItem) => void
}

export function HistoryItem({ history, onView, onEdit, onDownload }: HistoryItemProps) {
  const createdAt = new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(history.asset.createdAt))
  const previewUrl = `/api/v1/assets/${encodeURIComponent(history.asset.id)}/download`

  return (
    <article className="rounded-lg border border-ink-200 bg-white p-2">
      <div className="flex gap-3">
        <button
          className="h-20 w-20 shrink-0 overflow-hidden rounded-md bg-ink-100"
          onClick={() => onView(history)}
          type="button"
        >
          {previewUrl ? <img alt={history.asset.filename} className="h-full w-full object-cover" src={previewUrl} /> : null}
        </button>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-ink-900">{history.asset.filename}</p>
          <p className="mt-1 text-xs text-ink-500">
            {history.asset.kind} · {history.task.status}
          </p>
          <p className="mt-1 text-xs text-ink-400">
            {createdAt} · {formatBytes(history.asset.fileSize)}
          </p>
          <div className="mt-2 flex gap-1">
            <button aria-label={`查看结果 ${history.asset.filename}`} className="icon-button h-8 w-8" onClick={() => onView(history)} title="查看" type="button">
              <Eye className="h-4 w-4" />
            </button>
            <button aria-label={`再次编辑 ${history.asset.filename}`} className="icon-button h-8 w-8" onClick={() => onEdit(history)} title="再次编辑" type="button">
              <Pencil className="h-4 w-4" />
            </button>
            <button aria-label={`下载结果 ${history.asset.filename}`} className="icon-button h-8 w-8" onClick={() => onDownload(history)} title="下载" type="button">
              <Download className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </article>
  )
}
