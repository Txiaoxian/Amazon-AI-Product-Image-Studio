import { ChevronLeft, ChevronRight, RefreshCw } from 'lucide-react'
import type { BackendHistoryItem, HistoryKind } from '../../types/history'
import { HistoryItem } from './HistoryItem'

interface HistoryPanelProps {
  embedded?: boolean
  items: BackendHistoryItem[]
  error: string
  isLoading: boolean
  kind?: HistoryKind
  onView: (history: BackendHistoryItem) => void
  onEdit: (history: BackendHistoryItem) => void
  onRename: (history: BackendHistoryItem) => void
  onDownload: (history: BackendHistoryItem) => void
  onToggleFavorite: (history: BackendHistoryItem) => void
  favoriteActionAssetId?: string | null
  onKindChange: (kind?: HistoryKind) => void
  onPageChange: (pageNum: number) => void
  onPageSizeChange: (pageSize: number) => void
  onRefresh: () => void
  pageNum: number
  pageSize: number
  total: number
}

export function HistoryPanel({
  embedded = false,
  items,
  error,
  isLoading,
  kind,
  onView,
  onEdit,
  onRename,
  onDownload,
  onToggleFavorite,
  favoriteActionAssetId = null,
  onKindChange,
  onPageChange,
  onPageSizeChange,
  onRefresh,
  pageNum,
  pageSize,
  total,
}: HistoryPanelProps) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const canGoPrevious = pageNum > 1 && !isLoading
  const canGoNext = pageNum < totalPages && !isLoading

  return (
    <section className={`${embedded ? 'flex min-h-0 flex-col bg-white' : 'panel flex min-h-0 flex-col'}`}>
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold text-ink-900">{embedded ? '已完成' : '历史记录'}</h2>
          <p className="text-xs text-ink-400">{total} 条结果</p>
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

      <div className="grid gap-2 border-b border-ink-200 px-4 py-3">
        <label className="grid gap-1 text-xs font-medium text-ink-500">
          结果类型
          <select
            className="h-9 rounded-md border border-ink-200 bg-white px-2 text-sm text-ink-700 focus:border-amazon-500 focus:outline-none focus:ring-2 focus:ring-amazon-500/20"
            disabled={isLoading}
            id="history-kind-filter"
            name="historyKind"
            onChange={(event) => onKindChange(toHistoryKind(event.target.value))}
            value={kind ?? 'ALL'}
          >
            <option value="ALL">全部结果</option>
            <option value="GENERATED">生成结果</option>
            <option value="EDITED">编辑结果</option>
          </select>
        </label>
        <div className="flex items-center justify-between gap-2">
          <label className="flex items-center gap-2 text-xs font-medium text-ink-500">
            每页
            <select
              className="h-8 rounded-md border border-ink-200 bg-white px-2 text-sm text-ink-700 focus:border-amazon-500 focus:outline-none focus:ring-2 focus:ring-amazon-500/20"
              disabled={isLoading}
              id="history-page-size"
              name="historyPageSize"
              onChange={(event) => onPageSizeChange(Number(event.target.value))}
              value={pageSize}
            >
              <option value={10}>10</option>
              <option value={20}>20</option>
              <option value={50}>50</option>
            </select>
          </label>
          <div className="flex items-center gap-2">
            <button
              aria-label="上一页历史记录"
              className="icon-button h-8 w-8"
              disabled={!canGoPrevious}
              onClick={() => onPageChange(pageNum - 1)}
              title="上一页"
              type="button"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <span className="min-w-16 text-center text-xs text-ink-500">
              {pageNum} / {totalPages}
            </span>
            <button
              aria-label="下一页历史记录"
              className="icon-button h-8 w-8"
              disabled={!canGoNext}
              onClick={() => onPageChange(pageNum + 1)}
              title="下一页"
              type="button"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>

      <div className="flex-1 p-3 xl:overflow-y-auto">
        {isLoading ? <p className="py-8 text-center text-sm text-ink-400">正在读取历史记录...</p> : null}

        {!isLoading && items.length === 0 ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-10 text-center">
            <p className="text-sm font-medium text-ink-700">当前图片类型暂无生成记录</p>
            <p className="mt-1 text-xs text-ink-400">切换左侧类型可查看对应结果。</p>
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
              onRename={onRename}
              onToggleFavorite={onToggleFavorite}
              onView={onView}
              isFavoritePending={favoriteActionAssetId === history.asset.id}
            />
          ))}
        </div>
      </div>
    </section>
  )
}

function toHistoryKind(value: string): HistoryKind | undefined {
  return value === 'GENERATED' || value === 'EDITED' ? value : undefined
}
