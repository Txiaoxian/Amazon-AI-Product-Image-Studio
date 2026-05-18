import { Trash2 } from 'lucide-react'
import type { HistoryWithImage } from '../../db/historyRepository'
import { StorageMeter } from '../ui/StorageMeter'
import { LegacyHistoryItem } from './LegacyHistoryItem'

interface LegacyHistoryPanelProps {
  items: HistoryWithImage[]
  error: string
  usedBytes: number
  limitBytes: number
  isLoading: boolean
  onView: (history: HistoryWithImage) => void
  onEdit: (history: HistoryWithImage) => void
  onDownload: (history: HistoryWithImage) => void
  onDelete: (history: HistoryWithImage) => void
  onClear: () => void
}

export function LegacyHistoryPanel({
  items,
  error,
  usedBytes,
  limitBytes,
  isLoading,
  onView,
  onEdit,
  onDownload,
  onDelete,
  onClear,
}: LegacyHistoryPanelProps) {
  return (
    <aside className="panel flex min-h-0 flex-col">
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold text-ink-900">旧本地历史（兼容）</h2>
          <p className="text-xs text-ink-400">{items.length} 条旧结果</p>
        </div>
        <button
          aria-label="清空全部历史"
          className="icon-button hover:border-red-200 hover:bg-red-50 hover:text-red-700"
          disabled={items.length === 0}
          onClick={onClear}
          title="清空全部"
          type="button"
        >
          <Trash2 className="h-4 w-4" />
        </button>
      </div>

      <div className="border-b border-ink-200 p-4">
        <StorageMeter limitBytes={limitBytes} usedBytes={usedBytes} />
      </div>

      <div className="flex-1 p-3 xl:overflow-y-auto">
        {isLoading ? <p className="py-8 text-center text-sm text-ink-400">正在读取旧本地历史...</p> : null}

        {!isLoading && items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-10 text-center">
            <p className="text-sm font-medium text-ink-700">暂无旧本地历史</p>
            <p className="mt-1 text-xs text-ink-400">这里只保留迁移前的浏览器本地记录。</p>
          </div>
        ) : null}

        {error ? (
          <div className="mb-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
            {error}
          </div>
        ) : null}

        <div className="space-y-2">
          {items.map((history) => (
            <LegacyHistoryItem
              history={history}
              key={history.item.id}
              onDelete={onDelete}
              onDownload={onDownload}
              onEdit={onEdit}
              onView={onView}
            />
          ))}
        </div>
      </div>
    </aside>
  )
}
