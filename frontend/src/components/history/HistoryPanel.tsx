import { RefreshCw } from 'lucide-react'
import type { BackendHistoryItem } from '../../types/history'
import { HistoryItem } from './HistoryItem'

interface HistoryPanelProps {
  items: BackendHistoryItem[]
  error: string
  isLoading: boolean
  onView: (history: BackendHistoryItem) => void
  onEdit: (history: BackendHistoryItem) => void
  onDownload: (history: BackendHistoryItem) => void
  onRefresh: () => void
}

export function HistoryPanel({
  items,
  error,
  isLoading,
  onView,
  onEdit,
  onDownload,
  onRefresh,
}: HistoryPanelProps) {
  return (
    <aside className="panel flex min-h-0 flex-col">
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold text-ink-900">历史记录</h2>
          <p className="text-xs text-ink-400">{items.length} 条结果</p>
        </div>
        <button
          aria-label="刷新历史记录"
          className="icon-button"
          disabled={isLoading}
          onClick={onRefresh}
          title="刷新历史"
          type="button"
        >
          <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="flex-1 p-3 xl:overflow-y-auto">
        {isLoading ? <p className="py-8 text-center text-sm text-ink-400">正在读取历史记录...</p> : null}

        {!isLoading && items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-10 text-center">
            <p className="text-sm font-medium text-ink-700">当前项目暂无结果历史</p>
            <p className="mt-1 text-xs text-ink-400">生成或编辑成功后会显示后端资产结果。</p>
          </div>
        ) : null}

        {error ? (
          <div className="mb-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
            {error}
          </div>
        ) : null}

        <div className="space-y-2">
          {items.map((history) => (
            <HistoryItem
              history={history}
              key={history.asset.id}
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
