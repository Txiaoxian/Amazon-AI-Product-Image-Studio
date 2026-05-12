import { Download, Eye, ImagePlus, Loader2, RefreshCw, Star, Trash2, UploadCloud } from 'lucide-react'
import { useRef, useState } from 'react'
import { formatBytes } from '../../lib/storageLimit'
import type { Asset, Project, ProjectId } from '../../types/platform'
import { Button } from '../ui/Button'

interface ProjectAssetsPanelProps {
  actionAssetId: string | null
  assets: Asset[]
  error: string | null
  isCreatingProject: boolean
  isLoadingAssets: boolean
  isLoadingProjects: boolean
  isUploadingAsset: boolean
  projects: Project[]
  selectedProjectId: ProjectId | null
  onCreateProject: (request: { name: string; brand?: string; asin?: string; site?: string; notes?: string }) => void
  onDeleteAsset: (asset: Asset) => void
  onDownloadAsset: (asset: Asset) => void
  onOpenAsset: (asset: Asset) => void
  onRefreshAssets: () => void
  onRefreshProjects: () => void
  onSelectProject: (projectId: ProjectId) => void
  onToggleFavorite: (asset: Asset) => void
  onUploadReferences: (files: FileList) => void
  onUseAssetAsReference: (asset: Asset) => void
}

export function ProjectAssetsPanel({
  actionAssetId,
  assets,
  error,
  isCreatingProject,
  isLoadingAssets,
  isLoadingProjects,
  isUploadingAsset,
  projects,
  selectedProjectId,
  onCreateProject,
  onDeleteAsset,
  onDownloadAsset,
  onOpenAsset,
  onRefreshAssets,
  onRefreshProjects,
  onSelectProject,
  onToggleFavorite,
  onUploadReferences,
  onUseAssetAsReference,
}: ProjectAssetsPanelProps) {
  const uploadInputRef = useRef<HTMLInputElement>(null)
  const [name, setName] = useState('')
  const [brand, setBrand] = useState('')
  const [asin, setAsin] = useState('')

  const submitProject = () => {
    onCreateProject({
      name,
      brand: brand.trim() || undefined,
      asin: asin.trim() || undefined,
    })
    setName('')
    setBrand('')
    setAsin('')
  }

  return (
    <aside className="panel flex min-h-0 flex-col">
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <div className="min-w-0">
          <h2 className="text-sm font-semibold text-ink-900">项目资产库</h2>
          <p className="text-xs text-ink-400">{assets.length} 个资产</p>
        </div>
        <button
          aria-label="刷新项目资产"
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
        <div className="space-y-2">
          <div className="flex items-center justify-between gap-2">
            <label className="field-label" htmlFor="project-selector">
              当前项目
            </label>
            <button
              aria-label="刷新项目列表"
              className="text-xs font-semibold text-ink-500 hover:text-ink-900 disabled:text-ink-300"
              disabled={isLoadingProjects}
              onClick={onRefreshProjects}
              type="button"
            >
              刷新
            </button>
          </div>
          <select
            className="field-input"
            disabled={isLoadingProjects || projects.length === 0}
            id="project-selector"
            onChange={(event) => onSelectProject(event.target.value as ProjectId)}
            value={selectedProjectId ?? ''}
          >
            {projects.length === 0 ? <option value="">暂无项目</option> : null}
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>

        <form
          className="grid gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            submitProject()
          }}
        >
          <label className="field-label" htmlFor="new-project-name">
            新项目名称
          </label>
          <input
            className="field-input"
            disabled={isCreatingProject}
            id="new-project-name"
            onChange={(event) => setName(event.target.value)}
            placeholder="例如 Summer Launch"
            value={name}
          />
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
            <input
              aria-label="项目品牌"
              className="field-input"
              disabled={isCreatingProject}
              onChange={(event) => setBrand(event.target.value)}
              placeholder="品牌"
              value={brand}
            />
            <input
              aria-label="项目 ASIN"
              className="field-input"
              disabled={isCreatingProject}
              onChange={(event) => setAsin(event.target.value)}
              placeholder="ASIN"
              value={asin}
            />
          </div>
          <Button disabled={isCreatingProject || name.trim().length === 0} type="submit" variant="secondary">
            {isCreatingProject ? '创建中...' : '创建项目'}
          </Button>
        </form>

        <div>
          <label className="sr-only" htmlFor="reference-asset-upload">
            上传参考图
          </label>
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
          <Button
            className="w-full"
            disabled={!selectedProjectId || isUploadingAsset}
            icon={isUploadingAsset ? <Loader2 className="h-4 w-4 animate-spin" /> : <UploadCloud className="h-4 w-4" />}
            onClick={() => uploadInputRef.current?.click()}
            variant="primary"
          >
            {isUploadingAsset ? '上传中...' : '上传参考图'}
          </Button>
        </div>

        {error ? (
          <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
            {error}
          </div>
        ) : null}
      </div>

      <div className="flex-1 p-3 xl:overflow-y-auto">
        {isLoadingAssets ? <p className="py-8 text-center text-sm text-ink-400">正在加载项目资产...</p> : null}

        {!isLoadingAssets && selectedProjectId && assets.length === 0 ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
            <p className="text-sm font-medium text-ink-700">暂无项目资产</p>
            <p className="mt-1 text-xs text-ink-400">上传参考图后会显示在这里。</p>
          </div>
        ) : null}

        {!isLoadingAssets && !selectedProjectId ? (
          <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
            <p className="text-sm font-medium text-ink-700">暂无已选项目</p>
            <p className="mt-1 text-xs text-ink-400">创建或选择项目后可管理资产。</p>
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
  const previewUrl = asset.thumbnailUrl
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
            {asset.kind} · {asset.width} x {asset.height}
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
