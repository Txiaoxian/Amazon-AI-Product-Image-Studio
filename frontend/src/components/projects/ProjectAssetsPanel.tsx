import { Download, Eye, ImagePlus, Loader2, RefreshCw, Star, Trash2, UploadCloud } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { formatBytes } from '../../lib/storageLimit'
import type { Asset, AssetKind, ProjectId } from '../../types/platform'
import { WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../../types/workbench'
import { Button } from '../ui/Button'

interface ProjectAssetsPanelProps {
  actionAssetId: string | null
  assets: Asset[]
  error: string | null
  isLoadingAssets: boolean
  isUploadingAsset: boolean
  assetStatus: 'idle' | 'loading' | 'success' | 'error'
  selectedProjectId: ProjectId | null
  assetFilters: {
    favorite?: boolean
    kind?: AssetKind
    imageType?: WorkbenchImageType
  }
  onDeleteAsset: (asset: Asset) => void
  onDownloadAsset: (asset: Asset) => void
  onOpenAsset: (asset: Asset) => void
  onRefreshAssets: () => void
  onToggleFavorite: (asset: Asset) => void
  onUpdateAssetFilters: (filters: { favorite?: boolean; kind?: AssetKind; imageType?: WorkbenchImageType }) => void
  onUploadReferences: (files: FileList) => void
  onUseAssetAsReference: (asset: Asset) => void
}

export function ProjectAssetsPanel({
  actionAssetId,
  assets,
  error,
  isLoadingAssets,
  isUploadingAsset,
  assetStatus,
  selectedProjectId,
  assetFilters,
  onDeleteAsset,
  onDownloadAsset,
  onOpenAsset,
  onRefreshAssets,
  onToggleFavorite,
  onUpdateAssetFilters,
  onUploadReferences,
  onUseAssetAsReference,
}: ProjectAssetsPanelProps) {
  const uploadInputRef = useRef<HTMLInputElement>(null)
  const [kind, setKind] = useState<AssetKind | ''>(assetFilters.kind ?? '')
  const [favorite, setFavorite] = useState(assetFilters.favorite === undefined ? '' : String(assetFilters.favorite))
  const [imageType, setImageType] = useState<WorkbenchImageType | ''>(assetFilters.imageType ?? '')
  const referenceAssets = assets.filter((asset) => asset.kind === 'REFERENCE')

  useEffect(() => {
    setKind(assetFilters.kind ?? '')
  }, [assetFilters.kind])

  useEffect(() => {
    setFavorite(assetFilters.favorite === undefined ? '' : String(assetFilters.favorite))
  }, [assetFilters.favorite])

  useEffect(() => {
    setImageType(assetFilters.imageType ?? '')
  }, [assetFilters.imageType])

  const applyKind = (nextKind: AssetKind | '') => {
    setKind(nextKind)
    onUpdateAssetFilters({
      ...assetFilters,
      kind: nextKind || undefined,
    })
  }

  const applyFilters = () => {
    onUpdateAssetFilters({
      kind: kind || undefined,
      favorite: favorite === '' ? undefined : favorite === 'true',
      imageType: imageType || undefined,
    })
  }

  return (
    <aside className="flex min-h-0 flex-col rounded-lg border border-ink-200 bg-white">
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-ink-900">产品素材库</h2>
          <p className="text-xs text-ink-400">参考图与资产 · {assets.length} 个资产，{referenceAssets.length} 张参考图</p>
        </div>
        <button
          aria-label="刷新产品素材"
          className="icon-button"
          disabled={isLoadingAssets || !selectedProjectId}
          onClick={onRefreshAssets}
          title="刷新资产"
          type="button"
        >
          <RefreshCw className={`h-4 w-4 ${isLoadingAssets ? 'animate-spin' : ''}`} />
        </button>
      </div>

      <div className="grid gap-3 border-b border-ink-200 p-4">
        <input
          ref={uploadInputRef}
          accept="image/jpeg,image/png,image/webp"
          className="hidden"
          disabled={!selectedProjectId || isUploadingAsset}
          id="reference-asset-upload"
          multiple
          onChange={(event) => {
            if (event.target.files) {
              onUploadReferences(event.target.files)
            }
            event.target.value = ''
          }}
          type="file"
        />
        <label className="sr-only" htmlFor="reference-asset-upload">
          上传参考图
        </label>
        <Button
          className="w-full"
          disabled={!selectedProjectId || isUploadingAsset}
          icon={isUploadingAsset ? <Loader2 className="h-4 w-4 animate-spin" /> : <UploadCloud className="h-4 w-4" />}
          onClick={() => uploadInputRef.current?.click()}
          variant="primary"
        >
          {isUploadingAsset ? '上传中...' : '上传参考图'}
        </Button>

        {referenceAssets.length > 0 ? (
          <div>
            <p className="mb-2 text-xs font-semibold text-ink-500">产品参考图</p>
            <div className="grid grid-cols-3 gap-2">
              {referenceAssets.slice(0, 9).map((asset) => (
                <button
                  className="group relative flex aspect-square items-center justify-center overflow-hidden rounded-md border border-ink-200 bg-ink-100"
                  key={asset.id}
                  onClick={() => onUseAssetAsReference(asset)}
                  title="加入本次参考图"
                  type="button"
                >
                  {asset.thumbnailUrl || asset.previewUrl ? (
                    <img alt={`${asset.filename} 缩略图`} className="h-full w-full object-cover" src={asset.thumbnailUrl || asset.previewUrl} />
                  ) : (
                    <ImagePlus className="h-6 w-6 text-ink-400" />
                  )}
                  <span className="absolute inset-x-0 bottom-0 bg-ink-900/70 px-1.5 py-1 text-[11px] font-semibold text-white opacity-0 transition group-hover:opacity-100">
                    作为参考
                  </span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="rounded-md border border-dashed border-ink-300 bg-ink-50 px-3 py-3">
            <p className="text-sm font-semibold text-ink-700">暂无产品参考图</p>
            <p className="mt-1 text-xs leading-5 text-ink-400">上传参考图后，可在这里一键加入本次生图。</p>
          </div>
        )}

        {error ? (
          <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
            {error}
          </div>
        ) : null}

        <div className="flex flex-wrap gap-2">
          {[
            { value: 'REFERENCE', label: '参考图' },
            { value: 'GENERATED', label: '生成图' },
            { value: 'EDITED', label: '编辑图' },
            { value: '', label: '全部' },
          ].map((option) => (
            <button
              className={`rounded-md border px-3 py-1.5 text-xs font-semibold transition ${
                kind === option.value ? 'border-amazon-300 bg-amazon-50 text-amazon-700' : 'border-ink-200 bg-white text-ink-600 hover:bg-ink-50'
              }`}
              disabled={!selectedProjectId || isLoadingAssets}
              key={option.value}
              onClick={() => applyKind(option.value as AssetKind | '')}
              type="button"
            >
              {option.label}
            </button>
          ))}
        </div>

        <div className="grid gap-2 rounded-md border border-ink-200 bg-ink-50 p-3">
          <div className="grid gap-2 sm:grid-cols-3">
            <label className="grid gap-1 text-xs font-semibold text-ink-500">
              资产类型
              <select
                aria-label="资产类型"
                className="field-input bg-white"
                disabled={!selectedProjectId || isLoadingAssets}
                id="product-assets-kind"
                name="assetKind"
                onChange={(event) => setKind(event.target.value as AssetKind | '')}
                value={kind}
              >
                <option value="">全部</option>
                <option value="REFERENCE">参考图</option>
                <option value="GENERATED">生成图</option>
                <option value="EDITED">编辑图</option>
              </select>
            </label>
            <label className="grid gap-1 text-xs font-semibold text-ink-500">
              收藏
              <select
                aria-label="收藏"
                className="field-input bg-white"
                disabled={!selectedProjectId || isLoadingAssets}
                id="product-assets-favorite"
                name="assetFavorite"
                onChange={(event) => setFavorite(event.target.value)}
                value={favorite}
              >
                <option value="">全部</option>
                <option value="true">仅收藏</option>
                <option value="false">未收藏</option>
              </select>
            </label>
            <label className="grid gap-1 text-xs font-semibold text-ink-500">
              图片类型
              <select
                aria-label="素材图片类型"
                className="field-input bg-white"
                disabled={!selectedProjectId || isLoadingAssets}
                id="product-assets-image-type"
                name="assetImageType"
                onChange={(event) => setImageType(event.target.value as WorkbenchImageType | '')}
                value={imageType}
              >
                <option value="">全部图片类型</option>
                {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          <Button disabled={!selectedProjectId || isLoadingAssets} onClick={applyFilters} variant="secondary">
            筛选资产
          </Button>
        </div>
      </div>

      <div className="flex-1 p-3 xl:overflow-y-auto">
        {isLoadingAssets ? <p className="py-8 text-center text-sm text-ink-400">正在加载产品素材...</p> : null}

        {!isLoadingAssets && assetStatus === 'error' ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-8 text-center">
            <p className="text-sm font-medium text-red-700">资产加载失败</p>
            <p className="mt-1 text-xs text-red-600">请刷新资产列表后重试。</p>
          </div>
        ) : null}

        {!isLoadingAssets && assetStatus !== 'error' && selectedProjectId && assets.length === 0 ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
            <p className="text-sm font-medium text-ink-700">暂无产品素材</p>
            <p className="mt-1 text-xs text-ink-400">上传参考图，或完成生图任务后会显示在这里。</p>
          </div>
        ) : null}

        {!isLoadingAssets && !selectedProjectId ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
            <p className="text-sm font-medium text-ink-700">暂无已选产品</p>
            <p className="mt-1 text-xs text-ink-400">请选择或创建产品后再管理素材。</p>
          </div>
        ) : null}

        <div className="space-y-2">
          {assets.map((asset) => (
            <AssetListItem
              asset={asset}
              isPending={actionAssetId === asset.id}
              key={asset.id}
              onDelete={onDeleteAsset}
              onDownload={onDownloadAsset}
              onOpen={onOpenAsset}
              onToggleFavorite={onToggleFavorite}
              onUseAsReference={onUseAssetAsReference}
            />
          ))}
        </div>
      </div>
    </aside>
  )
}

interface AssetListItemProps {
  asset: Asset
  isPending: boolean
  onDelete: (asset: Asset) => void
  onDownload: (asset: Asset) => void
  onOpen: (asset: Asset) => void
  onToggleFavorite: (asset: Asset) => void
  onUseAsReference: (asset: Asset) => void
}

function AssetListItem({
  asset,
  isPending,
  onDelete,
  onDownload,
  onOpen,
  onToggleFavorite,
  onUseAsReference,
}: AssetListItemProps) {
  const previewUrl = asset.thumbnailUrl || asset.previewUrl
  const createdAt = new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(asset.createdAt))

  return (
    <article className="rounded-lg border border-ink-200 bg-white p-2">
      <div className="flex gap-3">
        <button
          className="h-20 w-20 shrink-0 overflow-hidden rounded-md bg-ink-100"
          onClick={() => onOpen(asset)}
          type="button"
        >
          {previewUrl ? (
            <img alt={`${asset.filename} 预览`} className="h-full w-full object-cover" src={previewUrl} />
          ) : (
            <ImagePlus className="mx-auto mt-7 h-6 w-6 text-ink-400" />
          )}
        </button>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-ink-900">{asset.filename}</p>
          <p className="mt-1 text-xs text-ink-500">
            {assetKindLabel(asset.kind)}{asset.imageType ? ` · ${imageTypeLabel(asset.imageType)}` : ''} · {asset.width} x {asset.height}
          </p>
          <p className="mt-1 text-xs text-ink-400">
            {createdAt} · {formatBytes(asset.fileSize)}
          </p>
          <div className="mt-2 flex flex-wrap gap-1">
            <button aria-label={`查看详情 ${asset.filename}`} className="icon-button h-8 w-8" onClick={() => onOpen(asset)} title="详情" type="button">
              <Eye className="h-4 w-4" />
            </button>
            <button
              aria-label={`作为参考图 ${asset.filename}`}
              className="icon-button h-8 w-8"
              disabled={isPending}
              onClick={() => onUseAsReference(asset)}
              title="作为参考图"
              type="button"
            >
              <ImagePlus className="h-4 w-4" />
            </button>
            <button
              aria-label={`${asset.isFavorite ? '取消收藏' : '收藏'} ${asset.filename}`}
              className={`icon-button h-8 w-8 ${asset.isFavorite ? 'border-amazon-400 bg-amazon-500/10 text-amazon-600' : ''}`}
              disabled={isPending}
              onClick={() => onToggleFavorite(asset)}
              title={asset.isFavorite ? '取消收藏' : '收藏'}
              type="button"
            >
              <Star className={`h-4 w-4 ${asset.isFavorite ? 'fill-current' : ''}`} />
            </button>
            <button
              aria-label={`下载 ${asset.filename}`}
              className="icon-button h-8 w-8"
              disabled={isPending}
              onClick={() => onDownload(asset)}
              title="下载"
              type="button"
            >
              <Download className="h-4 w-4" />
            </button>
            <button
              aria-label={`删除 ${asset.filename}`}
              className="icon-button h-8 w-8 hover:border-red-200 hover:bg-red-50 hover:text-red-700"
              disabled={isPending}
              onClick={() => onDelete(asset)}
              title="删除"
              type="button"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    </article>
  )
}

function imageTypeLabel(imageType: string): string {
  return WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === imageType)?.label ?? imageType
}

function assetKindLabel(kind: AssetKind): string {
  switch (kind) {
    case 'REFERENCE':
      return '参考图'
    case 'GENERATED':
      return '生成图'
    case 'EDITED':
      return '编辑图'
  }
}
