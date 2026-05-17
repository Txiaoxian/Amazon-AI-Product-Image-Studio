import { RefreshCw, Sparkles } from 'lucide-react'
import { type ReactNode, useEffect, useMemo, useState } from 'react'
import {
  IMAGE_COUNT_OPTIONS,
  RESOLUTION_LABELS,
  TUTUJIN_SIZE_LABELS,
  getCanvasOptionsForProvider,
  getDefaultCanvasForProvider,
  getDefaultResolutionForProvider,
  getResolutionOptionsForProvider,
  isCanvasOption,
  isResolutionOption,
} from '../../lib/constants'
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

interface SharedControlPanelProps {
  defaultModelId: string
  defaultResolution: ImageResolution
  isGenerating: boolean
  draft?: ControlPanelDraft | null
  referenceToAdd?: WorkbenchReferenceInput | null
  onError: (message: string) => void
  onReferenceAdded?: () => void
}

interface LegacyControlPanelProps extends SharedControlPanelProps {
  submissionMode?: 'legacy'
  onGenerate: (request: GenerationRequest) => Promise<void>
}

interface BackendControlPanelProps extends SharedControlPanelProps {
  submissionMode: 'backend'
  modelStatus: WorkbenchModelStatus
  models: Model[]
  onGenerate: (request: GenerationRequest, workbenchInput: WorkbenchTaskInput) => Promise<void>
  onRefreshModels: () => void
}

type ControlPanelProps = LegacyControlPanelProps | BackendControlPanelProps

export function ControlPanel(props: ControlPanelProps) {
  if (props.submissionMode === 'backend') {
    return <BackendControlPanel {...props} />
  }

  return <LegacyControlPanel {...props} />
}

function LegacyControlPanel({
  defaultModelId,
  defaultResolution,
  isGenerating,
  draft,
  referenceToAdd,
  onGenerate,
  onError,
  onReferenceAdded,
}: LegacyControlPanelProps) {
  const [prompt, setPrompt] = useState('')
  const [modelId, setModelId] = useState(defaultModelId)
  const [quality, setQuality] = useState<ImageResolution>(defaultResolution)
  const [aspectRatio, setAspectRatio] = useState<AspectRatio>('1:1')
  const [imageCount, setImageCount] = useState<ImageCount>(1)
  const [references, setReferences] = useState<WorkbenchReferenceInput[]>([])
  const [referenceImageUrlsText, setReferenceImageUrlsText] = useState('')
  const selectedModel = getModelById(modelId)
  const usesReferenceImageUrls = selectedModel.provider === 'gemini'
  const supportsLocalReferences = selectedModel.provider !== 'gemini'
  const resolutionOptions = getResolutionOptionsForProvider(selectedModel.provider)
  const canvasOptions = getCanvasOptionsForProvider(selectedModel.provider)
  const normalizedQuality = isResolutionOption(quality, resolutionOptions) ? quality : getDefaultResolutionForProvider(selectedModel.provider)
  const normalizedAspectRatio = isCanvasOption(aspectRatio, canvasOptions) ? aspectRatio : getDefaultCanvasForProvider(selectedModel.provider)
  const qualityLabel = usesReferenceImageUrls ? '质量' : '分辨率'
  const canvasLabel = usesReferenceImageUrls ? '尺寸' : '图片比例'

  useEffect(() => {
    if (!draft) {
      return
    }

    setPrompt(draft.prompt)
    setModelId(draft.modelId)
    setQuality(draft.quality)
    setAspectRatio(draft.aspectRatio)
    setImageCount(draft.imageCount ?? 1)
    setReferences((currentReferences) => {
      revokeReferencePreviewUrls(currentReferences)
      return draft.references ?? []
    })
    setReferenceImageUrlsText('')
  }, [draft])

  useEffect(() => {
    if (!isResolutionOption(quality, resolutionOptions)) {
      setQuality(getDefaultResolutionForProvider(selectedModel.provider))
    }
  }, [quality, resolutionOptions, selectedModel.provider])

  useEffect(() => {
    if (!isCanvasOption(aspectRatio, canvasOptions)) {
      setAspectRatio(getDefaultCanvasForProvider(selectedModel.provider))
    }
  }, [aspectRatio, canvasOptions, selectedModel.provider])

  useEffect(() => {
    if (supportsLocalReferences || references.length === 0) {
      return
    }

    setReferences((currentReferences) => {
      revokeReferencePreviewUrls(currentReferences)
      return []
    })
  }, [references.length, supportsLocalReferences])

  useEffect(() => {
    if (!referenceToAdd) {
      return
    }

    if (!supportsLocalReferences) {
      revokeReferencePreviewUrls([referenceToAdd])
      onError('当前模型不支持本地参考图，请切换模型后重试。')
      onReferenceAdded?.()
      return
    }

    setReferences((currentReferences) => {
      if (
        referenceToAdd.kind === 'asset' &&
        currentReferences.some((reference) => reference.kind === 'asset' && reference.assetId === referenceToAdd.assetId)
      ) {
        revokeReferencePreviewUrls([referenceToAdd])
        onError('该项目资产已在参考图中。')
        return currentReferences
      }

      return [...currentReferences, referenceToAdd]
    })
    onReferenceAdded?.()
  }, [onError, onReferenceAdded, referenceToAdd, supportsLocalReferences])

  return (
    <PanelShell
      onSubmit={() => {
        const referenceImageUrls = usesReferenceImageUrls ? parseReferenceImageUrls(referenceImageUrlsText, onError) : []

        if (referenceImageUrls === null) {
          return
        }

        void onGenerate({
          prompt,
          model: selectedModel,
          quality: normalizedQuality,
          aspectRatio: normalizedAspectRatio,
          imageCount,
          references: supportsLocalReferences ? getLegacyReferenceFiles(references) : [],
          referenceImageUrls,
        })
      }}
    >
      {usesReferenceImageUrls ? (
        <section className="space-y-2">
          <label className="field-label" htmlFor="reference-image-urls">
            参考图 URL
          </label>
          <textarea
            className="field-input min-h-24 resize-y"
            disabled={isGenerating}
            id="reference-image-urls"
            onChange={(event) => setReferenceImageUrlsText(event.target.value)}
            placeholder="https://example.com/reference.png"
            value={referenceImageUrlsText}
          />
          <span className="block text-xs text-ink-400">Nano Banana 2 中转站图生图使用公开 HTTPS URL，每行 1 张，最多 4 张。</span>
        </section>
      ) : supportsLocalReferences ? (
        <ImageDropzone disabled={isGenerating} onChange={setReferences} onError={onError} references={references} />
      ) : null}

      <PromptEditor disabled={isGenerating} onChange={setPrompt} onError={onError} value={prompt} />

      <section className="grid gap-4">
        <div className="space-y-2">
          <label className="field-label" htmlFor="model-id">
            模型
          </label>
          <select className="field-input" disabled={isGenerating} id="model-id" onChange={(event) => setModelId(event.target.value)} value={modelId}>
            {IMAGE_MODELS.map((model) => (
              <option key={model.id} value={model.id}>
                {model.label}
              </option>
            ))}
          </select>
          <span className="block text-xs text-ink-400">{selectedModel.description}</span>
        </div>

        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div className="space-y-2">
            <label className="field-label" htmlFor="image-resolution">
              {qualityLabel}
            </label>
            <select
              className="field-input"
              disabled={isGenerating}
              id="image-resolution"
              onChange={(event) => setQuality(event.target.value as ImageResolution)}
              value={normalizedQuality}
            >
              {resolutionOptions.map((option) => (
                <option key={option} value={option}>
                  {RESOLUTION_LABELS[option]}
                </option>
              ))}
            </select>
          </div>

          <div className="space-y-2">
            <label className="field-label" htmlFor="image-aspect-ratio">
              {canvasLabel}
            </label>
            <select
              className="field-input"
              disabled={isGenerating}
              id="image-aspect-ratio"
              onChange={(event) => setAspectRatio(event.target.value as AspectRatio)}
              value={normalizedAspectRatio}
            >
              {canvasOptions.map((option) => (
                <option key={option} value={option}>
                  {usesReferenceImageUrls ? TUTUJIN_SIZE_LABELS[option as keyof typeof TUTUJIN_SIZE_LABELS] : option}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="space-y-2">
          <label className="field-label" htmlFor="image-count">
            生成张数
          </label>
          <select
            className="field-input"
            disabled={isGenerating}
            id="image-count"
            onChange={(event) => setImageCount(Number(event.target.value) as ImageCount)}
            value={imageCount}
          >
            {IMAGE_COUNT_OPTIONS.map((option) => (
              <option key={option} value={option}>
                {option} 张
              </option>
            ))}
          </select>
          <span className="block text-xs text-ink-400">前端产品层限制为 1-4 张，接口会保留 imageCount 参数。</span>
        </div>
      </section>

      <Button className="mt-auto w-full" disabled={isGenerating} icon={<Sparkles className="h-4 w-4" />} type="submit" variant="primary">
        {isGenerating ? '生成中...' : '生成图片'}
      </Button>
    </PanelShell>
  )
}

function BackendControlPanel({
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
}: BackendControlPanelProps) {
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
      revokeReferencePreviewUrls(currentReferences)
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
      revokeReferencePreviewUrls(currentReferences)
      return []
    })
  }, [selectedModel])

  useEffect(() => {
    if (!referenceToAdd) {
      return
    }

    if (!selectedModel && modelId.length === 0 && models.length > 0) {
      return
    }

    if (!selectedModel?.supportsEdit) {
      revokeReferencePreviewUrls([referenceToAdd])
      onError('当前模型不支持参考图，请切换模型后重试。')
      onReferenceAdded?.()
      return
    }

    setReferences((currentReferences) => {
      if (!selectedModel.supportsMultiReference && currentReferences.length > 0) {
        revokeReferencePreviewUrls([referenceToAdd])
        onError('当前模型仅支持 1 张参考图。')
        return currentReferences
      }

      if (
        referenceToAdd.kind === 'asset' &&
        currentReferences.some((reference) => reference.kind === 'asset' && reference.assetId === referenceToAdd.assetId)
      ) {
        revokeReferencePreviewUrls([referenceToAdd])
        onError('该项目资产已在参考图中。')
        return currentReferences
      }

      return [...currentReferences, referenceToAdd]
    })
    onReferenceAdded?.()
  }, [modelId.length, models.length, onError, onReferenceAdded, referenceToAdd, selectedModel])

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
    <PanelShell
      onSubmit={() => {
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
            references: getLegacyReferenceFiles(references),
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
            <label className="field-label" htmlFor="backend-model-id">
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
            id="backend-model-id"
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
              <SelectField disabled={isGenerating} id="image-size" label="尺寸" onChange={setSize} options={selectedModel.supportedSizes} value={size} />
            ) : null}

            {selectedModel.supportedQualities.length > 0 ? (
              <SelectField
                disabled={isGenerating}
                id="image-quality"
                label="质量"
                onChange={setQuality}
                options={selectedModel.supportedQualities}
                value={quality}
              />
            ) : null}

            {selectedModel.supportedOutputFormats.length > 0 ? (
              <SelectField
                disabled={isGenerating}
                id="image-output-format"
                label="输出格式"
                onChange={setOutputFormat}
                options={selectedModel.supportedOutputFormats}
                value={outputFormat}
              />
            ) : null}

            <div className="space-y-2">
              <label className="field-label" htmlFor="backend-image-count">
                生成张数
              </label>
              <select
                className="field-input"
                disabled={isGenerating || !selectedModel.supportsN || selectedModel.maxOutputCount <= 1}
                id="backend-image-count"
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
    </PanelShell>
  )
}

interface SelectFieldProps {
  disabled: boolean
  id: string
  label: string
  options: string[]
  value: string
  onChange: (value: string) => void
}

function SelectField({ disabled, id, label, options, value, onChange }: SelectFieldProps) {
  return (
    <div className="space-y-2">
      <label className="field-label" htmlFor={id}>
        {label}
      </label>
      <select className="field-input" disabled={disabled} id={id} onChange={(event) => onChange(event.target.value)} value={value}>
        {options.map((option) => (
          <option key={option} value={option}>
            {option}
          </option>
        ))}
      </select>
    </div>
  )
}

interface PanelShellProps {
  children: ReactNode
  onSubmit: () => void
}

function PanelShell({ children, onSubmit }: PanelShellProps) {
  return (
    <aside className="panel flex min-h-0 flex-col">
      <div className="border-b border-ink-200 px-4 py-3">
        <h2 className="text-sm font-semibold text-ink-900">生成参数</h2>
      </div>
      <form
        className="flex flex-1 flex-col gap-5 p-4 xl:overflow-y-auto"
        onSubmit={(event) => {
          event.preventDefault()
          onSubmit()
        }}
      >
        {children}
      </form>
    </aside>
  )
}

function getLegacyFallbackModel(modelId: string, defaultModelId: string) {
  return IMAGE_MODELS.find((model) => model.id === modelId) ?? getModelById(defaultModelId)
}

function getLegacyReferenceFiles(references: WorkbenchReferenceInput[]): File[] {
  return references.map((reference) => (reference.kind === 'asset' ? reference.legacyFile : reference.file))
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

function revokeReferencePreviewUrls(references: WorkbenchReferenceInput[]) {
  references.forEach((reference) => URL.revokeObjectURL(reference.previewUrl))
}

function parseReferenceImageUrls(input: string, onError: (message: string) => void): string[] | null {
  const urls = input
    .split(/\r?\n/)
    .map((url) => url.trim())
    .filter(Boolean)

  if (urls.length > 4) {
    onError('Nano Banana 2 最多支持 4 张参考图 URL。')
    return null
  }

  const invalidUrl = urls.find((url) => !/^https:\/\/\S+$/i.test(url))

  if (invalidUrl) {
    onError(`参考图 URL 必须是公开 HTTPS 链接：${invalidUrl}`)
    return null
  }

  return urls
}
