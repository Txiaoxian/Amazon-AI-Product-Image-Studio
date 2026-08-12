import {
  AlertCircle,
  ArrowLeftRight,
  Check,
  Columns2,
  Download,
  Heart,
  ImageIcon,
  Images,
  Info,
  Loader2,
  PencilLine,
  RotateCcw,
  X,
  XCircle,
  ZoomIn,
  ZoomOut,
} from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import type { WorkbenchGeneration } from '../../hooks/useGeneration'
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
  onRename: () => void
  onDownload: () => void
  onCancelTask?: () => void
  onRetryTask?: () => void
  onOpenAssets?: () => void
  variant?: 'default' | 'canvas'
  imageTypeLabel?: string
  comparisonSource?: CompareItemRef
}

export type CompareMode = 'source-result' | 'candidate-candidate'

export interface CompareItemRef {
  id: string
  label: string
  url: string
}

export interface CompareState {
  isActive: boolean
  left: CompareItemRef | null
  mode: CompareMode
  right: CompareItemRef | null
  split: number
  swapped: boolean
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
  onRename,
  onDownload,
  onCancelTask,
  onRetryTask,
  onOpenAssets,
  variant = 'default',
  imageTypeLabel,
  comparisonSource,
}: ResultCanvasProps) {
  if (variant === 'canvas') {
    return (
      <CanvasResultCanvas
        canCancelTask={canCancelTask}
        canRetryTask={canRetryTask}
        comparisonSource={comparisonSource}
        current={current}
        currentItems={currentItems}
        error={error}
        imageTypeLabel={imageTypeLabel}
        onCancelTask={onCancelTask}
        onDownload={onDownload}
        onOpenAssets={onOpenAssets}
        onOpenDetail={onOpenDetail}
        onRename={onRename}
        onRetryTask={onRetryTask}
        onSelect={onSelect}
        pendingTaskAction={pendingTaskAction}
        selectedIndex={selectedIndex}
        status={status}
        taskStatus={taskStatus}
      />
    )
  }

  const imageUrl = getResultImageUrl(current)
  const hasResult = Boolean(current && imageUrl)

  return (
    <section className="panel flex min-h-[360px] flex-col overflow-hidden sm:min-h-[440px] lg:h-full lg:min-h-0">
      <div className="flex shrink-0 items-center justify-between gap-3 border-b border-ink-200 px-4 py-3">
        <div className="flex min-w-0 items-center gap-2">
          <h2 className="text-sm font-semibold text-ink-900">生成结果</h2>
          {taskStatus ? (
            <span className="rounded-md bg-ink-100 px-2 py-1 text-[11px] font-semibold text-ink-600">{taskStatus}</span>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          {onOpenAssets ? (
            <button
              className="inline-flex min-h-9 items-center gap-1.5 rounded-md border border-ink-200 bg-white px-2.5 text-xs font-semibold text-ink-700 transition hover:bg-ink-50"
              onClick={onOpenAssets}
              type="button"
            >
              <Images className="h-4 w-4" />
              产品素材
            </button>
          ) : null}
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
        </div>
      </div>

      <div
        className="result-checkerboard flex min-h-0 flex-1 items-center justify-center overflow-y-auto p-3 sm:p-4"
        data-testid="result-canvas-content"
      >
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
          <div className="flex h-full min-h-0 w-full flex-col items-center justify-center gap-3">
            {status === 'error' && error ? (
              <div className="w-full max-w-lg rounded-md border border-red-200 bg-white px-3 py-2 text-sm text-red-700">{error}</div>
            ) : null}
            <button
              aria-label="打开当前结果详情"
              className="flex min-h-0 w-full max-w-full flex-1 items-center justify-center overflow-hidden rounded-lg border border-ink-200 bg-white p-2 shadow-panel"
              data-testid="result-canvas-image"
              onClick={onOpenDetail}
              type="button"
            >
              <img alt="生成结果" className="h-full max-h-full w-full max-w-full object-contain" src={imageUrl} />
            </button>
            {currentItems.length > 1 ? (
              <div className="grid w-full max-w-lg shrink-0 grid-cols-4 gap-2">
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
            <div className="flex shrink-0 flex-wrap items-center justify-center gap-2 text-xs text-ink-500">
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
            </div>
            <div className="flex shrink-0 flex-wrap justify-center gap-2 pb-1" data-testid="result-canvas-actions">
              <Button icon={<PencilLine className="h-4 w-4" />} onClick={onRename}>
                重命名
              </Button>
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

const DEMO_RESULT_CANDIDATES = [
  { label: '标准棚拍', url: '/studio-assets/demo-bottle-candidate-studio.jpg' },
  { label: '冷色展示台', url: '/studio-assets/demo-bottle-candidate-cool.jpg' },
  { label: '暖色光影', url: '/studio-assets/demo-bottle-candidate-warm.jpg' },
  { label: '生活场景', url: '/studio-assets/demo-bottle-candidate-lifestyle.jpg' },
]

const DEMO_RESULT_IMAGES = DEMO_RESULT_CANDIDATES.map((candidate) => candidate.url)

function CanvasResultCanvas({
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
  onRename,
  onDownload,
  onCancelTask,
  onRetryTask,
  onOpenAssets,
  imageTypeLabel = '图片',
  comparisonSource,
}: Omit<ResultCanvasProps, 'variant'>) {
  const [demoSelectedIndex, setDemoSelectedIndex] = useState(0)
  const [isComparing, setComparing] = useState(false)
  const [compareMode, setCompareMode] = useState<CompareMode>('candidate-candidate')
  const [candidateBIndex, setCandidateBIndex] = useState(1)
  const [comparisonSplit, setComparisonSplit] = useState(50)
  const [isCompareSwapped, setCompareSwapped] = useState(false)
  const [isFavorite, setFavorite] = useState(false)
  const [isAdopted, setAdopted] = useState(false)
  const [zoom, setZoom] = useState(100)
  const candidateHelpId = useId()
  const comparisonPointerIdRef = useRef<number | null>(null)
  const realUrls = currentItems.flatMap((item) => {
    const url = getResultImageUrl(item)
    return url ? [url] : []
  })
  const isDemo = realUrls.length === 0
  const images = isDemo ? DEMO_RESULT_IMAGES : realUrls
  const activeIndex = isDemo ? demoSelectedIndex : Math.min(selectedIndex, images.length - 1)
  const candidateItems: CompareItemRef[] = images.map((url, index) => ({
    id: isDemo ? `demo-${index + 1}` : currentItems[index]?.result.assetId ?? `candidate-${index + 1}`,
    label: isDemo ? `候选 ${index + 1} · ${DEMO_RESULT_CANDIDATES[index]?.label}` : `候选 ${index + 1}`,
    url,
  }))
  const activeItem = candidateItems[activeIndex] ?? candidateItems[0]
  const resolvedCandidateBIndex = candidateBIndex !== activeIndex && candidateItems[candidateBIndex]
    ? candidateBIndex
    : candidateItems.findIndex((_, index) => index !== activeIndex)
  const candidateBItem = resolvedCandidateBIndex >= 0 ? candidateItems[resolvedCandidateBIndex] : null
  const sourceComparisonEnabled = Boolean(comparisonSource)
  const candidateComparisonEnabled = candidateItems.length >= 2
  const hasAnyComparison = sourceComparisonEnabled || candidateComparisonEnabled
  const preferredMode: CompareMode = current?.task.type === 'IMAGE_EDIT' && sourceComparisonEnabled
    ? 'source-result'
    : 'candidate-candidate'
  const selectedModeEnabled = compareMode === 'source-result' ? sourceComparisonEnabled : candidateComparisonEnabled
  const rawLeftItem = compareMode === 'source-result' ? comparisonSource ?? null : activeItem ?? null
  const rawRightItem = compareMode === 'source-result' ? activeItem ?? null : candidateBItem
  const leftItem = isCompareSwapped ? rawRightItem : rawLeftItem
  const rightItem = isCompareSwapped ? rawLeftItem : rawRightItem
  const activeImage = activeItem?.url ?? DEMO_RESULT_IMAGES[0]
  const alternateImages = images
    .map((url, index) => ({
      index,
      label: isDemo ? DEMO_RESULT_CANDIDATES[index]?.label : undefined,
      url,
    }))
    .filter((item) => item.index !== activeIndex)
    .slice(0, 3)

  useEffect(() => {
    setFavorite(false)
    setAdopted(false)
    setZoom(100)
  }, [activeIndex, current?.result.assetId])

  useEffect(() => {
    setComparing(false)
    setComparisonSplit(50)
    setCompareSwapped(false)
    setCompareMode(current?.task.type === 'IMAGE_EDIT' && sourceComparisonEnabled ? 'source-result' : 'candidate-candidate')
  }, [comparisonSource?.id, comparisonSource?.url, current?.task.id, current?.task.type, sourceComparisonEnabled])

  useEffect(() => {
    if (isComparing && !selectedModeEnabled) setComparing(false)
  }, [isComparing, selectedModeEnabled])

  const selectImage = (index: number) => {
    if (isDemo) {
      setDemoSelectedIndex(index)
      return
    }
    onSelect(index)
  }

  const openComparison = () => {
    if (!hasAnyComparison) return
    const nextMode = preferredMode === 'source-result' && sourceComparisonEnabled
      ? 'source-result'
      : candidateComparisonEnabled ? 'candidate-candidate' : 'source-result'
    setCompareMode(nextMode)
    if (nextMode === 'candidate-candidate') {
      const nextCandidateBIndex = candidateItems.findIndex((_, index) => index !== activeIndex)
      if (nextCandidateBIndex >= 0) setCandidateBIndex(nextCandidateBIndex)
    }
    setComparisonSplit(50)
    setCompareSwapped(false)
    setComparing(true)
  }

  const compareUnavailableReason = !sourceComparisonEnabled && !candidateComparisonEnabled
    ? '至少需要两张候选图，或为编辑任务提供原图后才能对比。'
    : undefined

  const updateComparisonFromPointer = (clientX: number, element: HTMLInputElement) => {
    const bounds = element.getBoundingClientRect()
    if (bounds.width <= 0) return
    const nextSplit = Math.round(((clientX - bounds.left) / bounds.width) * 100)
    setComparisonSplit(Math.min(100, Math.max(0, nextSplit)))
  }

  return (
    <section aria-label={`${imageTypeLabel}候选`} className="canvas-result-panel">
      <h2 className="sr-only">{imageTypeLabel}候选</h2>

      <div
        className="canvas-result-content min-h-0 overflow-y-auto"
        data-testid="result-canvas-content"
      >
        <div className="canvas-result-stage-wrap">
          <div className="canvas-result-stage min-h-0 flex-1" data-testid="result-canvas-image">
            <img
              alt="生成结果"
              className="absolute inset-0 h-full w-full object-contain transition-transform duration-200"
              src={isComparing && rightItem ? rightItem.url : activeImage}
              style={{ transform: `scale(${zoom / 100})` }}
            />
            {isComparing && leftItem && rightItem ? (
              <div
                aria-hidden="true"
                className="absolute inset-0 overflow-hidden border-r border-white/90"
                style={{ clipPath: `inset(0 ${100 - comparisonSplit}% 0 0)` }}
              >
                <img
                  alt=""
                  className="absolute inset-0 h-full w-full object-contain transition-transform duration-200"
                  src={leftItem.url}
                  style={{ transform: `scale(${zoom / 100})` }}
                />
              </div>
            ) : null}
            {isComparing && leftItem && rightItem ? (
              <>
                <span className="canvas-compare-label is-left">{leftItem.label}</span>
                <span className="canvas-compare-label is-right">{rightItem.label}</span>
              </>
            ) : null}
            {isComparing && leftItem && rightItem ? (
              <>
                <input
                  aria-label="调整对比位置"
                  aria-valuetext={`${comparisonSplit}%`}
                  className="canvas-comparison-slider"
                  max="100"
                  min="0"
                  onChange={(event) => setComparisonSplit(Number(event.target.value))}
                  onKeyDown={(event) => {
                    if (event.key === 'Home') {
                      event.preventDefault()
                      setComparisonSplit(0)
                    } else if (event.key === 'End') {
                      event.preventDefault()
                      setComparisonSplit(100)
                    }
                  }}
                  onLostPointerCapture={() => {
                    comparisonPointerIdRef.current = null
                  }}
                  onPointerCancel={() => {
                    comparisonPointerIdRef.current = null
                  }}
                  onPointerDown={(event) => {
                    comparisonPointerIdRef.current = event.pointerId
                    event.currentTarget.focus()
                    event.currentTarget.setPointerCapture?.(event.pointerId)
                    updateComparisonFromPointer(event.clientX, event.currentTarget)
                    event.preventDefault()
                  }}
                  onPointerMove={(event) => {
                    if (comparisonPointerIdRef.current !== event.pointerId) return
                    updateComparisonFromPointer(event.clientX, event.currentTarget)
                  }}
                  onPointerUp={(event) => {
                    if (comparisonPointerIdRef.current !== event.pointerId) return
                    updateComparisonFromPointer(event.clientX, event.currentTarget)
                    comparisonPointerIdRef.current = null
                    if (event.currentTarget.hasPointerCapture?.(event.pointerId)) {
                      event.currentTarget.releasePointerCapture(event.pointerId)
                    }
                  }}
                  type="range"
                  value={comparisonSplit}
                />
                <span aria-hidden="true" className="canvas-comparison-control" style={{ left: `${comparisonSplit}%` }}>
                  <Columns2 className="h-4 w-4" />
                </span>
              </>
            ) : null}
            {isComparing ? (
              <div className="canvas-compare-toolbar" role="toolbar" aria-label="图片对比工具">
                <button
                  aria-pressed={compareMode === 'source-result'}
                  disabled={!sourceComparisonEnabled}
                  onClick={() => setCompareMode('source-result')}
                  title={sourceComparisonEnabled ? '对比编辑原图与当前结果' : '当前任务没有可用原图'}
                  type="button"
                >
                  原图/结果
                </button>
                <button
                  aria-pressed={compareMode === 'candidate-candidate'}
                  disabled={!candidateComparisonEnabled}
                  onClick={() => setCompareMode('candidate-candidate')}
                  title={candidateComparisonEnabled ? '对比两张候选图片' : '至少需要两张候选图'}
                  type="button"
                >
                  候选 A/B
                </button>
                {compareMode === 'candidate-candidate' ? (
                  <>
                    <label>
                      <span>候选 A</span>
                      <select aria-label="候选 A" onChange={(event) => selectImage(Number(event.target.value))} value={activeIndex}>
                        {candidateItems.map((item, index) => <option disabled={index === resolvedCandidateBIndex} key={item.id} value={index}>{item.label}</option>)}
                      </select>
                    </label>
                    <label>
                      <span>候选 B</span>
                      <select aria-label="候选 B" onChange={(event) => setCandidateBIndex(Number(event.target.value))} value={resolvedCandidateBIndex}>
                        {candidateItems.map((item, index) => <option disabled={index === activeIndex} key={item.id} value={index}>{item.label}</option>)}
                      </select>
                    </label>
                  </>
                ) : (
                  <label>
                    <span>结果</span>
                    <select aria-label="结果候选" onChange={(event) => selectImage(Number(event.target.value))} value={activeIndex}>
                      {candidateItems.map((item, index) => <option key={item.id} value={index}>{item.label}</option>)}
                    </select>
                  </label>
                )}
                <button aria-label="交换对比两侧" onClick={() => setCompareSwapped((value) => !value)} title="交换两侧" type="button">
                  <ArrowLeftRight className="h-4 w-4" />
                </button>
                <button aria-label="重置对比位置" onClick={() => setComparisonSplit(50)} title="重置为 50%" type="button">
                  <RotateCcw className="h-4 w-4" />
                </button>
                <button aria-label="退出对比" onClick={() => setComparing(false)} title="退出对比" type="button">
                  <X className="h-4 w-4" />
                </button>
              </div>
            ) : null}
            {taskStatus || isDemo ? (
              <div
                className="absolute left-3 top-3 rounded-full bg-slate-950/75 px-3 py-1.5 text-xs font-medium text-white backdrop-blur"
                title={isDemo ? '当前展示的是示例候选；点击其他缩略图可切换查看。' : undefined}
              >
                {taskStatus ? canvasTaskStatusLabel(taskStatus) : '示例候选预览'}
              </div>
            ) : null}
            {current || canCancelTask || canRetryTask ? (
              <div className="canvas-stage-task-actions">
                {current ? (
                  <>
                    <button aria-label="下载原图" onClick={onDownload} title="下载原图" type="button">
                      <Download className="h-4 w-4" />
                    </button>
                    <button aria-label="打开当前结果详情" onClick={onOpenDetail} title="查看结果详情" type="button">
                      <Info className="h-4 w-4" />
                    </button>
                  </>
                ) : null}
                {canCancelTask ? (
                  <button aria-label="取消任务" disabled={pendingTaskAction === 'cancel'} onClick={onCancelTask} title="取消任务" type="button">
                    <XCircle className="h-4 w-4" />
                  </button>
                ) : null}
                {canRetryTask ? (
                  <button aria-label="重试任务" disabled={pendingTaskAction === 'retry'} onClick={onRetryTask} title="重试任务" type="button">
                    <RotateCcw className="h-4 w-4" />
                  </button>
                ) : null}
              </div>
            ) : null}
            <div className="canvas-zoom-control" aria-label="缩放控制">
              <button aria-label="缩小画布" disabled={zoom <= 90} onClick={() => setZoom((currentZoom) => Math.max(90, currentZoom - 10))} type="button">
                <ZoomOut className="h-4 w-4" />
              </button>
              <span>{zoom}%</span>
              <button aria-label="放大画布" disabled={zoom >= 120} onClick={() => setZoom((currentZoom) => Math.min(120, currentZoom + 10))} type="button">
                <ZoomIn className="h-4 w-4" />
              </button>
            </div>
          </div>

          {alternateImages.length > 0 ? (
            <div
              aria-describedby={candidateHelpId}
              aria-label="其他候选结果"
              className="canvas-result-alternates"
              role="group"
            >
              <span className="sr-only" id={candidateHelpId}>这些缩略图是同一任务的其他候选结果，点击即可切换到主画布查看。</span>
              {alternateImages.map((item) => {
                const candidateName = isDemo
                  ? `示例候选 ${item.index + 1}：${item.label}`
                  : `第 ${item.index + 1} 张候选结果`
                const candidateHint = `${candidateName}，点击切换到主画布查看`

                return (
                  <button
                    aria-label={candidateHint}
                    className={`canvas-result-thumbnail ${item.index === activeIndex ? 'is-selected' : ''}`}
                    disabled={isComparing}
                    key={`${item.url}:${item.index}`}
                    onClick={() => selectImage(item.index)}
                    title={isComparing ? '请在对比工具栏中明确选择左右图片' : candidateHint}
                    type="button"
                  >
                    <img alt={candidateName} className="h-full w-full object-contain" src={item.url} />
                  </button>
                )
              })}
            </div>
          ) : null}
        </div>

        <div className="canvas-result-actions shrink-0" data-testid="result-canvas-actions">
          {onOpenAssets ? (
            <button onClick={onOpenAssets} type="button">
              <Images className="h-4 w-4" />
              选择素材
            </button>
          ) : null}
          <button
            aria-pressed={isComparing}
            disabled={!hasAnyComparison}
            onClick={() => isComparing ? setComparing(false) : openComparison()}
            title={compareUnavailableReason}
            type="button"
          >
            <Columns2 className="h-4 w-4" />
            对比
          </button>
          <button aria-pressed={isFavorite} onClick={() => setFavorite((currentValue) => !currentValue)} type="button">
            <Heart className={`h-4 w-4 ${isFavorite ? 'fill-current' : ''}`} />
            {isFavorite ? '已收藏' : '收藏'}
          </button>
          <button
            aria-pressed={isAdopted}
            className="is-primary"
            onClick={() => setAdopted((currentValue) => !currentValue)}
            type="button"
          >
            <Check className="h-4 w-4" />
            {isAdopted ? '已采用' : '采用'}
          </button>
          <button aria-label="重命名" disabled={!current} onClick={onRename} type="button">
            <PencilLine className="h-4 w-4" />
            编辑
          </button>
        </div>

        {!hasAnyComparison ? <p className="canvas-compare-unavailable">{compareUnavailableReason}</p> : null}

        {status === 'loading' ? (
          <div className="canvas-status-toast" role="status">
            <Loader2 className="h-4 w-4 animate-spin text-emerald-400" />
            正在生成，可继续编辑
          </div>
        ) : null}
        {status === 'error' && error ? (
          <div className="canvas-status-toast border-red-400/20 text-red-100" role="alert">
            <AlertCircle className="h-4 w-4 text-red-400" />
            {error}
          </div>
        ) : null}
      </div>
    </section>
  )
}

function canvasTaskStatusLabel(status: TaskStatus): string {
  return {
    QUEUED: '排队中',
    RUNNING: '生成中',
    SUCCEEDED: '已完成',
    FAILED: '生成失败',
    CANCELLED: '已取消',
    RETRYING: '重试中',
    TIMED_OUT: '已超时',
  }[status]
}

interface ResultThumbnailProps {
  item: WorkbenchGeneration
  index: number
  isSelected: boolean
  onSelect: (index: number) => void
}

function ResultThumbnail({ item, index, isSelected, onSelect }: ResultThumbnailProps) {
  const imageUrl = getResultImageUrl(item)

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

function getResultImageUrl(item: WorkbenchGeneration | null) {
  return item?.result.previewUrl ?? item?.result.thumbnailUrl
}

function resultKey(item: WorkbenchGeneration) {
  return item.result.assetId
}
