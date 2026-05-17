import { RefreshCw, Sparkles } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { IMAGE_COUNT_OPTIONS } from '../../lib/constants'
import { IMAGE_MODELS, getModelById } from '../../providers/registry'
import type { AspectRatio, GenerationRequest, ImageCount, ImageResolution } from '../../providers/types'
import type { Model } from '../../types/platform'
import type { WorkbenchReferenceInput, WorkbenchTaskInput } from '../../types/workbench'
import { Button } from '../ui/Button'
import { ImageDropzone } from './ImageDropzone'
import { PromptEditor } from './PromptEditor'
import type { WorkbenchModelStatus } from './useWorkbenchModels'

export interface ControlPanelDraft {
  prompt: string
  modelId: string
  quality: ImageResolution
  aspectRatio: AspectRatio
  imageCount?: ImageCount
  references?: WorkbenchReferenceInput[]
}

interface ControlPanelProps {
  defaultModelId: string
  defaultResolution: ImageResolution
  isGenerating: boolean
  modelStatus: WorkbenchModelStatus
  models: Model[]
  draft?: ControlPanelDraft | null
  referenceToAdd?: WorkbenchReferenceInput | null
  onGenerate: (request: GenerationRequest, workbenchInput: WorkbenchTaskInput) => Promise<void>
  onError: (message: string) => void
  onRefreshModels: () => void
  onReferenceAdded?: () => void
}

export function ControlPanel({
  defaultModelId,
  defaultResolution,
  isGenerating,
  modelStatus,
  models,
  draft,
  referenceToAdd,
  onGenerate,
  onError,
  onRefreshModels,
  onReferenceAdded,
}: ControlPanelProps) {
  const [prompt, setPrompt] = useState('')
  const [modelId, setModelId] = useState('')
  const [size, setSize] = useState('')
  const [quality, setQuality] = useState('')
  const [outputFormat, setOutputFormat] = useState('')
  const [imageCount, setImageCount] = useState<ImageCount>(1)
  const [references, setReferences] = useState<WorkbenchReferenceInput[]>([])
  const selectedModel = useMemo(() => models.find((model) => model.id === modelId) ?? null, [modelId, models])
  const isSelectedModelUnavailable = modelId.length > 0 && selectedModel === null
  const legacyFallbackModel = getLegacyFallbackModel(modelId, defaultModelId)

  useEffect(() => {
    if (modelId || models.length === 0) {
      return
    }

    setModelId(models[0].id)
  }, [modelId, models])

  useEffect(() => {
    if (!draft) {
      return
    }

    setPrompt(draft.prompt)
    setModelId(draft.modelId)
    setImageCount(draft.imageCount ?? 1)
    setReferences((currentReferences) => {
      revokePendingReferences(currentReferences)
      return draft.references ?? []
    })
  }, [draft])

  useEffect(() => {
    if (!selectedModel) {
      return
    }

    setSize((current) => normalizeOption(current, selectedModel.supportedSizes))
    setQuality((current) => normalizeOption(current, selectedModel.supportedQualities))
    setOutputFormat((current) => normalizeOption(current, selectedModel.supportedOutputFormats))
    setImageCount((current) => normalizeImageCount(current, selectedModel))
  }, [selectedModel])

  useEffect(() => {
    if (!selectedModel || selectedModel.supportsEdit) {
      return
    }

    setReferences((currentReferences) => {
      revokePendingReferences(currentReferences)
      return []
    })
  }, [selectedModel])

  useEffect(() => {
    if (!referenceToAdd) {
      return
    }

    if (!selectedModel?.supportsEdit) {
      revokePendingReferences([referenceToAdd])
      onError('当前模型不支持参考图，请切换模型后重试。')
      onReferenceAdded?.()
      return
    }

    if (!selectedModel.supportsMultiReference && references.length > 0) {
      revokePendingReferences([referenceToAdd])
      onError('当前模型仅支持 1 张参考图。')
      onReferenceAdded?.()
      return
    }

    if (referenceToAdd.kind === 'asset' && references.some((reference) => reference.kind === 'asset' && reference.assetId === referenceToAdd.assetId)) {
      onError('该项目资产已在参考图中。')
      onReferenceAdded?.()
      return
    }

    setReferences((currentReferences) => [...currentReferences, referenceToAdd])
    onReferenceAdded?.()
  }, [onError, onReferenceAdded, referenceToAdd, references, selectedModel])

  const workbenchInput = selectedModel
    ? buildWorkbenchTaskInput(selectedModel, {
        size,
        quality,
        outputFormat,
        imageCount,
        references,
      })
    : null

  const canSubmit = Boolean(workbenchInput) && !isSelectedModelUnavailable && !isGenerating

  return (
    <aside className="panel flex min-h-0 flex-col">
      <div className="border-b border-ink-200 px-4 py-3">
        <h2 className="text-sm font-semibold text-ink-900">生成参数</h2>
      </div>
      <form
        className="flex flex-1 flex-col gap-5 p-4 xl:overflow-y-auto"
        onSubmit={(event) => {
          event.preventDefault()

          if (!workbenchInput) {
            onError('请先选择可用模型。')
            return
          }

          void onGenerate(
            {
              prompt,
              model: legacyFallbackModel,
              quality: defaultResolution,
              aspectRatio: '1:1',
              imageCount,
              references: references.flatMap((reference) => (reference.kind === 'pending' ? [reference.file] : [])),
              referenceImageUrls: [],
            },
            workbenchInput,
          )
        }}
      >
        {selectedModel?.supportsEdit ? (
          <ImageDropzone
            disabled={isGenerating}
            maxReferences={selectedModel.supportsMultiReference ? undefined : 1}
            onChange={setReferences}
            onError={onError}
            references={references}
          />
        ) : null}

        <PromptEditor disabled={isGenerating} onChange={setPrompt} onError={onError} value={prompt} />

        <section className="grid gap-4">
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2">
              <label className="field-label" htmlFor="model-id">
                模型
              </label>
              <button
                aria-label="刷新模型"
                className="inline-flex items-center gap-1 text-xs font-semibold text-ink-500 hover:text-ink-900 disabled:text-ink-300"
                disabled={modelStatus === 'loading'}
                onClick={onRefreshModels}
                type="button"
              >
                <RefreshCw className={`h-3.5 w-3.5 ${modelStatus === 'loading' ? 'animate-spin' : ''}`} />
                刷新模型
              </button>
            </div>
            <select
              className="field-input"
              disabled={isGenerating || modelStatus === 'loading' || models.length === 0}
              id="model-id"
              onChange={(event) => setModelId(event.target.value)}
              value={selectedModel?.id ?? ''}
            >
              {!selectedModel ? <option value="">请选择模型</option> : null}
              {models.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.displayName} · {model.providerName}
                </option>
              ))}
            </select>
            {selectedModel ? (
              <span className="block text-xs text-ink-400">
                {selectedModel.modelName}
                {selectedModel.supportsMultiReference ? ' · 支持多张参考图' : ''}
              </span>
            ) : null}
            {isSelectedModelUnavailable ? (
              <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm leading-6 text-amber-900" role="alert">
                所选模型当前不可用，请刷新模型后重新选择。
              </div>
            ) : null}
          </div>

          {selectedModel ? (
            <div className="grid gap-3">
              {selectedModel.supportedSizes.length > 0 ? (
                <div className="space-y-2">
                  <label className="field-label" htmlFor="image-size">
                    尺寸
                  </label>
                  <select
                    className="field-input"
                    disabled={isGenerating}
                    id="image-size"
                    onChange={(event) => setSize(event.target.value)}
                    value={size}
                  >
                    {selectedModel.supportedSizes.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>
              ) : null}

              {selectedModel.supportedQualities.length > 0 ? (
                <div className="space-y-2">
                  <label className="field-label" htmlFor="image-quality">
                    质量
                  </label>
                  <select
                    className="field-input"
                    disabled={isGenerating}
                    id="image-quality"
                    onChange={(event) => setQuality(event.target.value)}
                    value={quality}
                  >
                    {selectedModel.supportedQualities.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>
              ) : null}

              {selectedModel.supportedOutputFormats.length > 0 ? (
                <div className="space-y-2">
                  <label className="field-label" htmlFor="image-output-format">
                    输出格式
                  </label>
                  <select
                    className="field-input"
                    disabled={isGenerating}
                    id="image-output-format"
                    onChange={(event) => setOutputFormat(event.target.value)}
                    value={outputFormat}
                  >
                    {selectedModel.supportedOutputFormats.map((option) => (
                      <option key={option} value={option}>
                        {option}
                      </option>
                    ))}
                  </select>
                </div>
              ) : null}

              <div className="space-y-2">
                <label className="field-label" htmlFor="image-count">
                  生成张数
                </label>
                <select
                  className="field-input"
                  disabled={isGenerating || !selectedModel.supportsN || selectedModel.maxOutputCount <= 1}
                  id="image-count"
                  onChange={(event) => setImageCount(Number(event.target.value) as ImageCount)}
                  value={imageCount}
                >
                  {getImageCountOptions(selectedModel).map((option) => (
                    <option key={option} value={option}>
                      {option} 张
                    </option>
                  ))}
                </select>
              </div>
            </div>
          ) : null}
        </section>

        <Button className="mt-auto w-full" disabled={!canSubmit} icon={<Sparkles className="h-4 w-4" />} type="submit" variant="primary">
          {isGenerating ? '生成中...' : '生成图片'}
        </Button>
      </form>
    </aside>
  )
}

function getLegacyFallbackModel(modelId: string, defaultModelId: string) {
  return IMAGE_MODELS.find((model) => model.id === modelId) ?? getModelById(defaultModelId)
}

function normalizeOption(current: string, options: string[]): string {
  if (options.length === 0) {
    return ''
  }

  return options.includes(current) ? current : options[0]
}

function normalizeImageCount(current: ImageCount, model: Model): ImageCount {
  if (!model.supportsN || model.maxOutputCount <= 1) {
    return 1
  }

  return getImageCountOptions(model).includes(current) ? current : 1
}

function getImageCountOptions(model: Model): ImageCount[] {
  if (!model.supportsN || model.maxOutputCount <= 1) {
    return [1]
  }

  return IMAGE_COUNT_OPTIONS.filter((option) => option <= model.maxOutputCount)
}

function buildWorkbenchTaskInput(
  model: Model,
  state: {
    size: string
    quality: string
    outputFormat: string
    imageCount: ImageCount
    references: WorkbenchReferenceInput[]
  },
): WorkbenchTaskInput {
  return {
    providerId: model.providerId,
    modelId: model.id,
    referenceAssetIds: state.references.flatMap((reference) => (reference.kind === 'asset' ? [reference.assetId] : [])),
    parameters: {
      ...(state.size ? { size: state.size } : {}),
      ...(state.quality ? { quality: state.quality } : {}),
      ...(state.outputFormat ? { outputFormat: state.outputFormat } : {}),
      outputCount: state.imageCount,
    },
  }
}

function revokePendingReferences(references: WorkbenchReferenceInput[]) {
  references.forEach((reference) => {
    if (reference.kind === 'pending') {
      URL.revokeObjectURL(reference.previewUrl)
    }
  })
}
