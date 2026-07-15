import { Download, ImagePlus, Save, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import type { Asset } from '../../types/platform'
import { DEFAULT_WORKBENCH_IMAGE_TYPE, WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../../types/workbench'
import { formatBytes } from '../../lib/storageLimit'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

interface AssetDetailModalProps {
  asset: Asset | null
  isActionPending: boolean
  isOpen: boolean
  onClose: () => void
  onDelete: (asset: Asset) => void
  onDownload: (asset: Asset) => void
  onUpdateAsset: (asset: Asset, request: { filename?: string; category?: WorkbenchImageType; isFavorite?: boolean }) => void
  onUseAsReference: (asset: Asset) => void
}

export function AssetDetailModal({
  asset,
  isActionPending,
  isOpen,
  onClose,
  onDelete,
  onDownload,
  onUpdateAsset,
  onUseAsReference,
}: AssetDetailModalProps) {
  const [draft, setDraft] = useState<{
    category: WorkbenchImageType
    filename: string
    isFavorite: boolean
  }>({
    category: DEFAULT_WORKBENCH_IMAGE_TYPE,
    filename: '',
    isFavorite: false,
  })

  useEffect(() => {
    if (!asset) {
      setDraft({ category: DEFAULT_WORKBENCH_IMAGE_TYPE, filename: '', isFavorite: false })
      return
    }

    setDraft({
      category: normalizeAssetCategory(asset.category),
      filename: editableFilename(asset.filename),
      isFavorite: asset.isFavorite,
    })
  }, [asset])

  if (!asset) {
    return null
  }

  const rows = [
    ['文件名', asset.filename],
    ['类型', asset.kind],
    ['分类', assetCategoryLabel(asset.category)],
    ['MIME', asset.mimeType],
    ['文件大小', formatBytes(asset.fileSize)],
    ['宽度', `${asset.width}px`],
    ['高度', `${asset.height}px`],
    ['收藏', asset.isFavorite ? '是' : '否'],
    ['创建时间', new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(asset.createdAt))],
    ['更新时间', new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(asset.updatedAt))],
  ]
  return (
    <Modal
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <Button onClick={onClose}>关闭</Button>
          <Button
            disabled={isActionPending}
            icon={<Trash2 className="h-4 w-4" />}
            onClick={() => onDelete(asset)}
            variant="danger"
          >
            删除
          </Button>
          <Button
            disabled={isActionPending}
            icon={<ImagePlus className="h-4 w-4" />}
            onClick={() => onUseAsReference(asset)}
          >
            作为参考图
          </Button>
          <Button
            disabled={isActionPending}
            icon={<Download className="h-4 w-4" />}
            onClick={() => onDownload(asset)}
            variant="primary"
          >
            下载
          </Button>
        </div>
      }
      isOpen={isOpen}
      onClose={onClose}
      title="资产详情"
    >
      <div className="grid gap-5 lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <div className="overflow-hidden rounded-lg border border-ink-200 bg-ink-50">
          {asset.previewUrl ? (
            <img alt={`${asset.filename} 预览`} className="h-full max-h-[440px] w-full object-contain" src={asset.previewUrl} />
          ) : null}
        </div>
        <div className="grid gap-4">
          <form
            className="grid gap-3 rounded-lg border border-ink-200 bg-ink-50 p-3"
            onSubmit={(event) => {
              event.preventDefault()
              onUpdateAsset(asset, {
                category: draft.category,
                filename: filenameWithMIMEExtension(draft.filename, asset.mimeType),
                isFavorite: draft.isFavorite,
              })
            }}
          >
            <div className="grid gap-1 text-xs font-semibold text-ink-500">
              <label htmlFor="asset-edit-filename">文件名</label>
              <div className="flex overflow-hidden rounded-md border border-ink-200 bg-white focus-within:border-amazon-500 focus-within:ring-2 focus-within:ring-amazon-500/20">
                <input
                  aria-describedby="asset-edit-filename-help"
                  className="min-w-0 flex-1 bg-white px-3 py-2 text-sm text-ink-800 outline-none disabled:cursor-not-allowed disabled:bg-ink-50"
                  disabled={isActionPending}
                  id="asset-edit-filename"
                  name="assetFilename"
                  onChange={(event) => setDraft((current) => ({ ...current, filename: editableFilename(event.target.value) }))}
                  value={draft.filename}
                />
                <span className="flex shrink-0 items-center border-l border-ink-200 bg-ink-50 px-3 text-sm font-semibold text-ink-500">
                  .{extensionForMIME(asset.mimeType)}
                </span>
              </div>
              <span className="font-normal leading-5 text-ink-400" id="asset-edit-filename-help">
                文件格式由图片实际内容决定，扩展名不可修改。
              </span>
            </div>
            <label className="grid gap-1 text-xs font-semibold text-ink-500" htmlFor="asset-edit-category">
              分类
              <select
                className="field-input bg-white"
                disabled={isActionPending}
                id="asset-edit-category"
                name="assetCategory"
                onChange={(event) => setDraft((current) => ({ ...current, category: event.target.value as WorkbenchImageType }))}
                value={draft.category}
              >
                {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex items-center gap-2 text-sm font-semibold text-ink-700" htmlFor="asset-edit-favorite">
              <input
                checked={draft.isFavorite}
                className="h-4 w-4 rounded border-ink-300 text-amazon-500 focus:ring-amazon-500/30"
                disabled={isActionPending}
                id="asset-edit-favorite"
                name="assetFavorite"
                onChange={(event) => setDraft((current) => ({ ...current, isFavorite: event.target.checked }))}
                type="checkbox"
              />
              收藏资产
            </label>
            <Button
              disabled={isActionPending || editableFilename(draft.filename).trim().length === 0}
              icon={<Save className="h-4 w-4" />}
              type="submit"
              variant="primary"
            >
              {isActionPending ? '保存中...' : '保存元数据'}
            </Button>
          </form>

          <dl className="grid gap-3">
            {rows.map(([label, value]) => (
              <div className="rounded-md border border-ink-200 bg-ink-50 px-3 py-2" key={label}>
                <dt className="text-xs font-semibold uppercase tracking-normal text-ink-400">{label}</dt>
                <dd className="mt-1 break-words text-sm leading-6 text-ink-800">{value}</dd>
              </div>
            ))}
          </dl>
        </div>
      </div>
    </Modal>
  )
}

function assetCategoryLabel(category: string) {
  const normalized = normalizeAssetCategory(category)
  return WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === normalized)?.label ?? '主图'
}

function normalizeAssetCategory(category: string): WorkbenchImageType {
  const normalized = category.trim().toUpperCase()
  return WORKBENCH_IMAGE_TYPE_OPTIONS.some((option) => option.value === normalized)
    ? (normalized as WorkbenchImageType)
    : DEFAULT_WORKBENCH_IMAGE_TYPE
}

function editableFilename(filename: string) {
  return filename.trim().replace(/\.(?:png|jpe?g|webp)$/i, '')
}

function filenameWithMIMEExtension(filename: string, mimeType: Asset['mimeType']) {
  return `${editableFilename(filename)}.${extensionForMIME(mimeType)}`
}

function extensionForMIME(mimeType: Asset['mimeType']) {
  switch (mimeType) {
    case 'image/jpeg':
      return 'jpg'
    case 'image/webp':
      return 'webp'
    default:
      return 'png'
  }
}
