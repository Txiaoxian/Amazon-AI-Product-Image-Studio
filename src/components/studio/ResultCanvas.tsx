import { AlertCircle, Download, ImageIcon, Info, Loader2 } from 'lucide-react'
import type { CurrentGeneration } from '../../hooks/useGeneration'
import { useObjectUrl } from '../../hooks/useObjectUrl'
import { formatBytes } from '../../lib/storageLimit'
import { Button } from '../ui/Button'

interface ResultCanvasProps {
  status: 'idle' | 'loading' | 'success' | 'error'
  error?: string
  current: CurrentGeneration | null
  currentItems: CurrentGeneration[]
  selectedIndex: number
  onSelect: (index: number) => void
  onOpenDetail: () => void
  onDownload: () => void
}

export function ResultCanvas({
  status,
  error,
  current,
  currentItems,
  selectedIndex,
  onSelect,
  onOpenDetail,
  onDownload,
}: ResultCanvasProps) {
  const imageUrl = useObjectUrl(current?.result.blob)

  return (
    <section className="panel flex min-h-[360px] flex-col overflow-hidden sm:min-h-[440px] xl:min-h-[520px]">
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <h2 className="text-sm font-semibold text-ink-900">生成结果</h2>
        {current ? (
          <div className="flex items-center gap-2">
            <button aria-label="查看参数详情" className="icon-button" onClick={onOpenDetail} title="查看参数" type="button">
              <Info className="h-4 w-4" />
            </button>
            <button aria-label="下载原图" className="icon-button" onClick={onDownload} title="下载原图" type="button">
              <Download className="h-4 w-4" />
            </button>
          </div>
        ) : null}
      </div>

      <div className="flex flex-1 items-center justify-center bg-[linear-gradient(45deg,#f8fafc_25%,transparent_25%),linear-gradient(-45deg,#f8fafc_25%,transparent_25%),linear-gradient(45deg,transparent_75%,#f8fafc_75%),linear-gradient(-45deg,transparent_75%,#f8fafc_75%)] bg-[length:24px_24px] bg-[position:0_0,0_12px,12px_-12px,-12px_0] p-3 sm:p-4">
        {status === 'idle' ? (
          <div className="max-w-sm text-center">
            <ImageIcon className="mx-auto h-12 w-12 text-ink-300" />
            <p className="mt-4 text-sm font-medium text-ink-700">等待生成结果</p>
            <p className="mt-1 text-sm text-ink-400">上传参考图或直接输入提示词后开始生成。</p>
          </div>
        ) : null}

        {status === 'loading' ? (
          <div className="max-w-sm text-center">
            <Loader2 className="mx-auto h-12 w-12 animate-spin text-amazon-500" />
            <p className="mt-4 text-sm font-medium text-ink-700">正在生成图片</p>
            <p className="mt-1 text-sm text-ink-400">请保持页面打开，完成后会自动保存到历史记录。</p>
          </div>
        ) : null}

        {status === 'error' ? (
          <div className="max-w-md rounded-lg border border-red-200 bg-white p-5 text-center">
            <AlertCircle className="mx-auto h-10 w-10 text-red-500" />
            <p className="mt-3 text-sm font-semibold text-ink-900">生成失败</p>
            <p className="mt-2 text-sm leading-6 text-ink-500">{error || '操作失败，请稍后重试。'}</p>
          </div>
        ) : null}

        {status === 'success' && current && imageUrl ? (
          <div className="flex h-full w-full flex-col items-center justify-center gap-4">
            <button className="max-h-full max-w-full overflow-hidden rounded-lg border border-ink-200 bg-white p-2 shadow-panel" onClick={onOpenDetail} type="button">
              <img alt="生成结果" className="max-h-[52dvh] w-auto object-contain sm:max-h-[64vh]" src={imageUrl} />
            </button>
            {currentItems.length > 1 ? (
              <div className="grid w-full max-w-lg grid-cols-4 gap-2">
                {currentItems.map((item, index) => (
                  <ResultThumbnail
                    index={index}
                    isSelected={index === selectedIndex}
                    item={item}
                    key={item.history.item.id}
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
              <span>·</span>
              <span>{formatBytes(current.result.fileSize)}</span>
              <span>·</span>
              <span>{current.result.durationMs}ms</span>
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              <Button icon={<Info className="h-4 w-4" />} onClick={onOpenDetail}>
                查看参数
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

interface ResultThumbnailProps {
  item: CurrentGeneration
  index: number
  isSelected: boolean
  onSelect: (index: number) => void
}

function ResultThumbnail({ item, index, isSelected, onSelect }: ResultThumbnailProps) {
  const imageUrl = useObjectUrl(item.result.blob)

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
