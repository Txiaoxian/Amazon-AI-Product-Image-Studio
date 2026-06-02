import { RefreshCw, Sparkles } from 'lucide-react'
import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { IMAGE_COUNT_OPTIONS } from '../../lib/constants'
import type { Model } from '../../types/platform'
import {
  DEFAULT_WORKBENCH_IMAGE_TYPE,
  WORKBENCH_IMAGE_TYPE_OPTIONS,
  normalizeWorkbenchImageType,
  type WorkbenchImageType,
  type WorkbenchReferenceInput,
  type WorkbenchTaskInput,
  type WorkbenchTaskSubmission,
} from '../../types/workbench'
import { Button } from '../ui/Button'
import { ImageDropzone } from './ImageDropzone'
import { PromptEditor } from './PromptEditor'
import type { WorkbenchModelStatus } from './useWorkbenchModels'

type ImageCount = (typeof IMAGE_COUNT_OPTIONS)[number]

export interface BackendControlPanelDraft {
  prompt: string
  modelId: string
  imageType?: string
  imageCount?: ImageCount
  references?: WorkbenchReferenceInput[]
}

interface BackendControlPanelProps {
  draft?: BackendControlPanelDraft | null
  isGenerating: boolean
  modelStatus: WorkbenchModelStatus
  models: Model[]
  referenceToAdd?: WorkbenchReferenceInput | null
  onError: (message: string) => void
  onGenerate: (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput) => Promise<void>
  onReferenceAdded?: () => void
  onRefreshModels: () => void
  resetKey?: string | null
}

export function BackendControlPanel({
  isGenerating,
  modelStatus,
  models,
  draft,
  referenceToAdd,
  onGenerate,
  onError,
  onRefreshModels,
  onReferenceAdded,
  resetKey,
}: BackendControlPanelProps) {
  const [prompt, setPrompt] = useState('')
  const [modelId, setModelId] = useState('')
  const [size, setSize] = useState('')
  const [quality, setQuality] = useState('')
  const [outputFormat, setOutputFormat] = useState('')
  const [imageType, setImageType] = useState<WorkbenchImageType>(DEFAULT_WORKBENCH_IMAGE_TYPE)
  const [imageCount, setImageCount] = useState<ImageCount>(1)
  const [references, setReferences] = useState<WorkbenchReferenceInput[]>([])
  const previousResetKey = useRef(resetKey)
  const selectedModel = useMemo(() => models.find((model) => model.id === modelId) ?? null, [modelId, models])
  const isSelectedModelUnavailable = modelId.length > 0 && selectedModel === null

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
    setImageType(normalizeWorkbenchImageType(draft.imageType))
    setImageCount(draft.imageCount ?? 1)
    setReferences((currentReferences) => {
      revokeReferencePreviewUrls(currentReferences)
      return draft.references ?? []
    })
  }, [draft])

  useEffect(() => {
    if (previousResetKey.current === resetKey) {
      return
    }

    previousResetKey.current = resetKey
    setReferences((currentReferences) => {
      revokeReferencePreviewUrls(currentReferences)
      return []
    })
  }, [resetKey])

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
        imageType,
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
          },
          workbenchInput,
        )
      }}
    >
      {selectedModel?.supportsEdit ? (
        <ImageDropzone
          allowUpload={false}
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
            <div className="space-y-2">
              <label className="field-label" htmlFor="backend-image-type">
                图片类型
              </label>
              <select
                className="field-input"
                disabled={isGenerating}
                id="backend-image-type"
                onChange={(event) => setImageType(normalizeWorkbenchImageType(event.target.value))}
                value={imageType}
              >
                {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </div>

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
    imageType: WorkbenchImageType
    imageCount: ImageCount
    references: WorkbenchReferenceInput[]
  },
): WorkbenchTaskInput {
  return {
    providerId: model.providerId,
    modelId: model.id,
    imageType: state.imageType,
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
  references.forEach((reference) => {
    if (reference.kind === 'pending') {
      URL.revokeObjectURL(reference.previewUrl)
    }
  })
}
