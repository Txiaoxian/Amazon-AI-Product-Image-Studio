import { AlertCircle, Download, ImageIcon, Info, Loader2, RotateCcw, XCircle } from 'lucide-react'
import type { WorkbenchGeneration } from '../../hooks/useGeneration'
import { useObjectUrl } from '../../hooks/useObjectUrl'
import { formatBytes } from '../../lib/storageLimit'
import type { TaskStatus } from '../../types/platform'
import { Button } from '../ui/Button'

interface ResultCanvasProps {
  status: 'idle' | 'loading' | 'success' | 'error'
  error?: string
  current: WorkbenchGeneration | null
  currentItems: WorkbenchGeneration[]
  selectedIndex: number
  taskStatus?: TaskStatus
  pendingTaskAction?: 'cancel' | 'retry' | null
  canCancelTask?: boolean
  canRetryTask?: boolean
  onSelect: (index: number) => void
  onOpenDetail: () => void
  onDownload: () => void
  onCancelTask?: () => void
  onRetryTask?: () => void
}

export function ResultCanvas({
  status,
  error,
  current,
  currentItems,
  selectedIndex,
  taskStatus,
  pendingTaskAction,
  canCancelTask,
  canRetryTask,
  onSelect,
  onOpenDetail,
  onDownload,
  onCancelTask,
  onRetryTask,
}: ResultCanvasProps) {
  const imageUrl = useResultImageUrl(current)
  const hasResult = Boolean(current && imageUrl)

  return (
    <section className="panel flex min-h-[360px] flex-col overflow-hidden sm:min-h-[440px] xl:min-h-[520px]">
      <div className="flex items-center justify-between gap-3 border-b border-ink-200 px-4 py-3">
        <div className="flex min-w-0 items-center gap-2">
          <h2 className="text-sm font-semibold text-ink-900">生成结果</h2>
          {taskStatus ? (
            <span className="rounded-md bg-ink-100 px-2 py-1 text-[11px] font-semibold text-ink-600">{taskStatus}</span>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          {canCancelTask ? (
            <button
              aria-label="取消任务"
              className="icon-button"
              disabled={pendingTaskAction === 'cancel'}
              onClick={onCancelTask}
              title="取消任务"
              type="button"
            >
              <XCircle className="h-4 w-4" />
            </button>
          ) : null}
          {canRetryTask ? (
            <button
              aria-label="重试任务"
              className="icon-button"
              disabled={pendingTaskAction === 'retry'}
              onClick={onRetryTask}
              title="重试任务"
              type="button"
            >
              <RotateCcw className="h-4 w-4" />
            </button>
          ) : null}
          {current ? (
            <>
              <button aria-label="查看结果详情" className="icon-button" onClick={onOpenDetail} title="查看详情" type="button">
                <Info className="h-4 w-4" />
              </button>
            </>
          ) : null}
          {current?.kind === 'legacy' ? (
            <>
              <button aria-label="下载原图" className="icon-button" onClick={onDownload} title="下载原图" type="button">
                <Download className="h-4 w-4" />
              </button>
            </>
          ) : null}
        </div>
      </div>

      <div className="flex flex-1 items-center justify-center bg-[linear-gradient(45deg,#f8fafc_25%,transparent_25%),linear-gradient(-45deg,#f8fafc_25%,transparent_25%),linear-gradient(45deg,transparent_75%,#f8fafc_75%),linear-gradient(-45deg,transparent_75%,#f8fafc_75%)] bg-[length:24px_24px] bg-[position:0_0,0_12px,12px_-12px,-12px_0] p-3 sm:p-4">
        {status === 'idle' ? (
          <div className="max-w-sm text-center">
            <ImageIcon className="mx-auto h-12 w-12 text-ink-300" />
            <p className="mt-4 text-sm font-medium text-ink-700">等待生成结果</p>
            <p className="mt-1 text-sm text-ink-400">上传参考图或直接输入提示词后开始生成。</p>
          </div>
        ) : null}

        {status === 'loading' && !hasResult ? (
          <div className="max-w-sm text-center">
            <Loader2 className="mx-auto h-12 w-12 animate-spin text-amazon-500" />
            <p className="mt-4 text-sm font-medium text-ink-700">后台任务处理中</p>
            <p className="mt-1 text-sm text-ink-400">结果会通过实时事件流自动更新到这里。</p>
          </div>
        ) : null}

        {status === 'error' && !hasResult ? (
          <div className="max-w-md rounded-lg border border-red-200 bg-white p-5 text-center">
            <AlertCircle className="mx-auto h-10 w-10 text-red-500" />
            <p className="mt-3 text-sm font-semibold text-ink-900">生成失败</p>
            <p className="mt-2 text-sm leading-6 text-ink-500">{error || '操作失败，请稍后重试。'}</p>
          </div>
        ) : null}

        {hasResult && current && imageUrl ? (
          <div className="flex h-full w-full flex-col items-center justify-center gap-4">
            {status === 'error' && error ? (
              <div className="w-full max-w-lg rounded-md border border-red-200 bg-white px-3 py-2 text-sm text-red-700">{error}</div>
            ) : null}
            <button
              aria-label="打开当前结果详情"
              className="max-h-full max-w-full overflow-hidden rounded-lg border border-ink-200 bg-white p-2 shadow-panel"
              onClick={onOpenDetail}
              type="button"
            >
              <img alt="生成结果" className="max-h-[52dvh] w-auto object-contain sm:max-h-[64vh]" src={imageUrl} />
            </button>
            {currentItems.length > 1 ? (
              <div className="grid w-full max-w-lg grid-cols-4 gap-2">
                {currentItems.map((item, index) => (
                  <ResultThumbnail
                    index={index}
                    isSelected={index === selectedIndex}
                    item={item}
                    key={resultKey(item)}
                    onSelect={onSelect}
                  />
                ))}
              </div>
            ) : null}
            <div className="flex flex-wrap items-center justify-center gap-2 text-xs text-ink-500">
              {currentItems.length > 1 ? (
                <>
                  <span>第 {selectedIndex + 1} / {currentItems.length} 张</span>
                  <span>·</span>
                </>
              ) : null}
              <span>{current.result.width} x {current.result.height}</span>
              {current.result.fileSize ? (
                <>
                  <span>·</span>
                  <span>{formatBytes(current.result.fileSize)}</span>
                </>
              ) : null}
              {current.kind === 'legacy' ? (
                <>
                  <span>·</span>
                  <span>{current.result.durationMs}ms</span>
                </>
              ) : null}
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              {current.kind === 'legacy' ? (
                <Button icon={<Info className="h-4 w-4" />} onClick={onOpenDetail}>
                  查看参数
                </Button>
              ) : null}
              <Button icon={<Download className="h-4 w-4" />} onClick={onDownload} variant="primary">
                下载原图
              </Button>
            </div>
          </div>
        ) : null}
      </div>
    </section>
  )
}

interface ResultThumbnailProps {
  item: WorkbenchGeneration
  index: number
  isSelected: boolean
  onSelect: (index: number) => void
}

function ResultThumbnail({ item, index, isSelected, onSelect }: ResultThumbnailProps) {
  const imageUrl = useResultImageUrl(item)

  return (
    <button
      aria-label={`查看第 ${index + 1} 张结果`}
      className={`aspect-square overflow-hidden rounded-md border bg-white p-1 transition ${
        isSelected ? 'border-amazon-500 ring-2 ring-amazon-500/25' : 'border-ink-200 hover:border-ink-300'
      }`}
      onClick={() => onSelect(index)}
      type="button"
    >
      {imageUrl ? <img alt={`第 ${index + 1} 张生成结果`} className="h-full w-full object-cover" src={imageUrl} /> : null}
    </button>
  )
}

function useResultImageUrl(item: WorkbenchGeneration | null) {
  const legacyImageUrl = useObjectUrl(item?.kind === 'legacy' ? item.result.blob : undefined)

  if (!item) {
    return undefined
  }

  return item.kind === 'legacy' ? legacyImageUrl : item.result.previewUrl ?? item.result.thumbnailUrl
}

function resultKey(item: WorkbenchGeneration) {
  return item.kind === 'legacy' ? item.history.item.id : item.result.assetId
}
