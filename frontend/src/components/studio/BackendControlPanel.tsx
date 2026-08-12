import { RefreshCw, Sparkles } from 'lucide-react'
import { type KeyboardEvent, type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { IMAGE_COUNT_OPTIONS } from '../../lib/constants'
import {
  labelForQualityPreset,
  labelForSizePreset,
  parameterLabelsForCapabilities,
} from '../../lib/modelCapabilityPresets'
import type { Model, ProjectId } from '../../types/platform'
import {
  DEFAULT_WORKBENCH_IMAGE_TYPE,
  WORKBENCH_IMAGE_TYPE_OPTIONS,
  normalizeWorkbenchImageType,
  type AssetReferenceInput,
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
type CanvasControlSection = 'prompt' | 'references' | 'settings'

const CANVAS_CONTROL_SECTIONS: Array<{ label: string; value: CanvasControlSection }> = [
  { label: '画面', value: 'prompt' },
  { label: '参考', value: 'references' },
  { label: '参数', value: 'settings' },
]

export interface BackendControlPanelDraft {
  prompt: string
  modelId: string
  imageType?: string
  imageCount?: ImageCount
  references?: WorkbenchReferenceInput[]
}

interface BackendControlPanelProps {
  availableReferences?: WorkbenchReferenceInput[]
  draft?: BackendControlPanelDraft | null
  isGenerating: boolean
  modelStatus: WorkbenchModelStatus
  models: Model[]
  imageType?: WorkbenchImageType
  projectId?: ProjectId | null
  prompt?: string
  referenceToAdd?: WorkbenchReferenceInput | null
  editSourceReference?: AssetReferenceInput | null
  onImageTypeChange?: (imageType: WorkbenchImageType) => void
  onPromptChange?: (prompt: string) => void
  onError: (message: string) => void
  onGenerate: (request: WorkbenchTaskSubmission, workbenchInput: WorkbenchTaskInput) => Promise<void>
  onReferenceAdded?: () => void
  onEditSourceRemoved?: () => void
  onRefreshModels: () => void
  onOpenModelSettings?: () => void
  onSavePendingReferences?: (files: File[]) => Promise<WorkbenchReferenceInput[]>
  resetKey?: string | null
  variant?: 'default' | 'canvas'
}

export function BackendControlPanel({
  availableReferences = [],
  isGenerating,
  modelStatus,
  models,
  imageType: controlledImageType,
  projectId,
  prompt: controlledPrompt,
  draft,
  referenceToAdd,
  editSourceReference,
  onImageTypeChange,
  onPromptChange,
  onGenerate,
  onError,
  onRefreshModels,
  onOpenModelSettings,
  onReferenceAdded,
  onEditSourceRemoved,
  onSavePendingReferences,
  resetKey,
  variant = 'default',
}: BackendControlPanelProps) {
  const [internalPrompt, setInternalPrompt] = useState('')
  const [modelId, setModelId] = useState('')
  const [size, setSize] = useState('')
  const [quality, setQuality] = useState('')
  const [outputFormat, setOutputFormat] = useState('')
  const [internalImageType, setInternalImageType] = useState<WorkbenchImageType>(DEFAULT_WORKBENCH_IMAGE_TYPE)
  const [imageCount, setImageCount] = useState<ImageCount>(1)
  const [references, setReferences] = useState<WorkbenchReferenceInput[]>([])
  const [canvasSection, setCanvasSection] = useState<CanvasControlSection>('prompt')
  const previousResetKey = useRef(resetKey)
  const canvasTabRefs = useRef<Array<HTMLButtonElement | null>>([])
  const setImageTypeRef = useRef(onImageTypeChange ?? setInternalImageType)
  const setPromptRef = useRef(onPromptChange ?? setInternalPrompt)
  const selectedModel = useMemo(() => models.find((model) => model.id === modelId) ?? null, [modelId, models])
  const supportedSizeOptions = useMemo(() => prioritizeAutoOption(selectedModel?.supportedSizes ?? []), [selectedModel])
  const supportedQualityOptions = useMemo(() => prioritizeAutoOption(selectedModel?.supportedQualities ?? []), [selectedModel])
  const parameterLabels = useMemo(
    () => parameterLabelsForCapabilities(
      selectedModel?.supportedSizes ?? [],
      selectedModel?.supportedQualities ?? [],
      selectedModel?.providerType,
    ),
    [selectedModel],
  )
  const isSelectedModelUnavailable = modelId.length > 0 && selectedModel === null
  const imageType = controlledImageType ?? internalImageType
  const setImageType = onImageTypeChange ?? setInternalImageType
  const prompt = controlledPrompt ?? internalPrompt
  const setPrompt = onPromptChange ?? setInternalPrompt

  useEffect(() => {
    if (modelId || models.length === 0) {
      return
    }

    setModelId(models[0].id)
  }, [modelId, models])

  useEffect(() => {
    setImageTypeRef.current = onImageTypeChange ?? setInternalImageType
    setPromptRef.current = onPromptChange ?? setInternalPrompt
  }, [onImageTypeChange, onPromptChange])

  useEffect(() => {
    if (!draft) {
      return
    }

    setPromptRef.current(draft.prompt)
    setModelId(draft.modelId)
    setImageTypeRef.current(normalizeWorkbenchImageType(draft.imageType))
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

    setSize((current) => normalizeOption(current, supportedSizeOptions))
    setQuality((current) => normalizeOption(current, supportedQualityOptions))
    setOutputFormat((current) => normalizeOption(current, selectedModel.supportedOutputFormats))
    setImageCount((current) => normalizeImageCount(current, selectedModel))
  }, [selectedModel, supportedQualityOptions, supportedSizeOptions])

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
    if (variant === 'canvas' && isSelectedModelUnavailable) {
      setCanvasSection('settings')
    }
  }, [isSelectedModelUnavailable, variant])

  useEffect(() => {
    if (variant === 'canvas' && editSourceReference) {
      setCanvasSection('references')
    }
  }, [editSourceReference, variant])

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
        onError('该产品素材已在参考图中。')
        return currentReferences
      }

      return [...currentReferences, referenceToAdd]
    })
    onReferenceAdded?.()
  }, [modelId.length, models.length, onError, onReferenceAdded, referenceToAdd, selectedModel])

  const workbenchInput = selectedModel ? buildCurrentWorkbenchInput(selectedModel, references) : null

  const canSubmit = Boolean(workbenchInput) && !isSelectedModelUnavailable && !isGenerating && (!editSourceReference || selectedModel?.supportsEdit === true)
  const canvasReferenceCount = references.length + (editSourceReference ? 1 : 0)
  const canvasParameterSummary = selectedModel
    ? `${selectedModel.displayName} · ${size ? labelForSizePreset(size) : '自动比例'} · ${imageCount} 张`
    : '模型待选择'

  const handleCanvasTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>, currentIndex: number) => {
    let nextIndex: number

    if (event.key === 'ArrowRight') {
      nextIndex = (currentIndex + 1) % CANVAS_CONTROL_SECTIONS.length
    } else if (event.key === 'ArrowLeft') {
      nextIndex = (currentIndex - 1 + CANVAS_CONTROL_SECTIONS.length) % CANVAS_CONTROL_SECTIONS.length
    } else if (event.key === 'Home') {
      nextIndex = 0
    } else if (event.key === 'End') {
      nextIndex = CANVAS_CONTROL_SECTIONS.length - 1
    } else {
      return
    }

    event.preventDefault()
    setCanvasSection(CANVAS_CONTROL_SECTIONS[nextIndex].value)
    canvasTabRefs.current[nextIndex]?.focus()
  }

  if (variant === 'canvas') {
    return (
      <aside className="canvas-control-panel">
        <form
          className="canvas-control-form"
          data-can-submit={canSubmit ? 'true' : 'false'}
          id="canvas-generation-form"
          onSubmit={(event) => {
            event.preventDefault()
            if (!selectedModel) {
              setCanvasSection('settings')
              onError('请先选择可用模型。')
              return
            }
            if (!canSubmit) {
              if (isSelectedModelUnavailable) {
                setCanvasSection('settings')
                onError('所选模型当前不可用，请刷新模型后重新选择。')
              }
              return
            }
            void submitGeneration(selectedModel)
          }}
        >
          <label className="sr-only" htmlFor="workbench-image-type-accessibility">图片类型</label>
          <select
            className="sr-only"
            id="workbench-image-type-accessibility"
            onChange={(event) => setImageType(normalizeWorkbenchImageType(event.target.value))}
            tabIndex={-1}
            value={imageType}
          >
            {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>{option.label}</option>
            ))}
          </select>

          <div aria-label="创作参数分区" className="canvas-control-tabs" role="tablist">
            {CANVAS_CONTROL_SECTIONS.map((section, index) => {
              const isSelected = canvasSection === section.value
              const meta = section.value === 'references' && canvasReferenceCount > 0
                ? String(canvasReferenceCount)
                : section.value === 'settings' && !selectedModel
                  ? '待选'
                  : null

              return (
                <button
                  aria-label={section.label}
                  aria-controls={`canvas-control-panel-${section.value}`}
                  aria-selected={isSelected}
                  className={`canvas-control-tab ${isSelected ? 'is-selected' : ''}`}
                  id={`canvas-control-tab-${section.value}`}
                  key={section.value}
                  onClick={() => setCanvasSection(section.value)}
                  onKeyDown={(event) => handleCanvasTabKeyDown(event, index)}
                  ref={(element) => {
                    canvasTabRefs.current[index] = element
                  }}
                  role="tab"
                  tabIndex={isSelected ? 0 : -1}
                  type="button"
                >
                  <span>{section.label}</span>
                  {meta ? <span aria-hidden="true" className="canvas-control-tab-meta">{meta}</span> : null}
                </button>
              )
            })}
          </div>

          <div className="canvas-control-panes">
            <section
              aria-labelledby="canvas-control-tab-prompt"
              className="canvas-control-pane"
              hidden={canvasSection !== 'prompt'}
              id="canvas-control-panel-prompt"
              role="tabpanel"
              tabIndex={0}
            >
              <PromptEditor
                disabled={isGenerating}
                imageType={imageType}
                onChange={setPrompt}
                onError={onError}
                projectId={projectId}
                value={prompt}
                variant="compact"
              />

              <button
                className="canvas-parameter-summary"
                onClick={() => setCanvasSection('settings')}
                title={canvasParameterSummary}
                type="button"
              >
                <span>
                  <span className="block text-xs font-semibold text-slate-200">当前生成参数</span>
                  <span className="mt-1 block truncate text-[11px] text-slate-400">{canvasParameterSummary}</span>
                </span>
                <span className="shrink-0 text-xs font-semibold text-amazon-400">调整</span>
              </button>
            </section>

            <section
              aria-labelledby="canvas-control-tab-references"
              className="canvas-control-pane"
              hidden={canvasSection !== 'references'}
              id="canvas-control-panel-references"
              role="tabpanel"
              tabIndex={0}
            >
              {!selectedModel || selectedModel.supportsEdit || editSourceReference ? (
                <ImageDropzone
                  allowUpload={!selectedModel || selectedModel.supportsEdit}
                  availableReferences={availableReferences}
                  disabled={isGenerating || Boolean(selectedModel && !selectedModel.supportsEdit)}
                  editSourceReference={editSourceReference}
                  maxReferences={!selectedModel || selectedModel.supportsMultiReference ? undefined : 1}
                  onChange={setReferences}
                  onError={onError}
                  onEditSourceRemoved={onEditSourceRemoved}
                  references={references}
                  variant="canvas"
                />
              ) : (
                <section>
                  <h3 className="text-sm font-semibold text-white">参考素材</h3>
                  <p className="mt-2 rounded-lg border border-white/10 bg-white/[0.04] px-3 py-3 text-xs leading-5 text-slate-400">
                    当前模型不支持参考图，可在参数中切换模型。
                  </p>
                </section>
              )}

              {references.some((reference) => reference.kind === 'pending') ? (
                <Button disabled={isGenerating || !onSavePendingReferences} onClick={() => void savePendingReferences()} variant="secondary">
                  保存为产品参考图
                </Button>
              ) : null}
            </section>

            <section
              aria-labelledby="canvas-control-tab-settings"
              className="canvas-control-pane canvas-control-pane-settings"
              hidden={canvasSection !== 'settings'}
              id="canvas-control-panel-settings"
              role="tabpanel"
              tabIndex={0}
            >
              <div className="flex items-center justify-between gap-2">
                <div>
                  <h3 className="text-sm font-semibold text-white">生成参数</h3>
                  <p className="mt-1 text-[11px] text-slate-500">仅显示当前模型支持的选项</p>
                </div>
                <button
                  aria-label="刷新模型"
                  className="inline-flex items-center gap-1 text-xs font-semibold text-slate-400 hover:text-white disabled:text-slate-600"
                  disabled={modelStatus === 'loading'}
                  onClick={onRefreshModels}
                  type="button"
                >
                  <RefreshCw className={`h-3.5 w-3.5 ${modelStatus === 'loading' ? 'animate-spin' : ''}`} />
                  刷新
                </button>
              </div>

              <div className="grid gap-3">
                <div className="space-y-1.5">
                  <label className="text-xs font-semibold text-slate-400" htmlFor="backend-model-id">模型</label>
                  <select
                    className="canvas-dark-field"
                    disabled={isGenerating || modelStatus === 'loading' || models.length === 0}
                    id="backend-model-id"
                    onChange={(event) => setModelId(event.target.value)}
                    value={selectedModel?.id ?? ''}
                  >
                    {!selectedModel ? <option value="">请选择模型</option> : null}
                    {models.map((model) => (
                      <option key={model.id} value={model.id}>{model.displayName} · {model.providerName}</option>
                    ))}
                  </select>
                  {models.length === 0 ? (
                    onOpenModelSettings ? (
                      <button
                        className="inline-flex min-h-9 w-full items-center justify-center rounded-lg border border-amazon-500/35 bg-amazon-500/10 px-3 text-xs font-semibold text-amazon-400 transition hover:bg-amazon-500/15"
                        onClick={onOpenModelSettings}
                        type="button"
                      >
                        配置可用模型
                      </button>
                    ) : (
                      <p className="text-xs leading-5 text-slate-500">当前没有可用模型，请联系管理员完成配置。</p>
                    )
                  ) : null}
                </div>

              {isSelectedModelUnavailable ? (
                <div className="rounded-lg border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-xs leading-5 text-amber-200" role="alert">
                  所选模型当前不可用，请刷新模型后重新选择。
                </div>
              ) : null}

              {selectedModel ? (
                <div className="grid grid-cols-2 gap-2">
                  {supportedSizeOptions.length > 0 ? (
                    <CanvasSelectField
                      disabled={isGenerating}
                      getOptionLabel={labelForSizePreset}
                      id="image-size"
                      label={parameterLabels.sizeLabel}
                      onChange={setSize}
                      options={supportedSizeOptions}
                      value={size}
                    />
                  ) : null}
                  {supportedQualityOptions.length > 0 ? (
                    <CanvasSelectField
                      disabled={isGenerating}
                      getOptionLabel={(value) => labelForQualityPreset(value, { modelName: selectedModel.modelName })}
                      id="image-quality"
                      label={parameterLabels.qualityLabel}
                      onChange={setQuality}
                      options={supportedQualityOptions}
                      value={quality}
                    />
                  ) : null}
                  {selectedModel.supportedOutputFormats.length > 0 ? (
                    <CanvasSelectField
                      disabled={isGenerating}
                      id="image-output-format"
                      label="输出格式"
                      onChange={setOutputFormat}
                      options={selectedModel.supportedOutputFormats}
                      value={outputFormat}
                    />
                  ) : null}
                  <div className="space-y-1.5">
                    <label className="text-xs font-semibold text-slate-400" htmlFor="backend-image-count">生成张数</label>
                    <select
                      className="canvas-dark-field"
                      disabled={isGenerating || !selectedModel.supportsN || selectedModel.maxOutputCount <= 1}
                      id="backend-image-count"
                      onChange={(event) => setImageCount(Number(event.target.value) as ImageCount)}
                      value={imageCount}
                    >
                      {getImageCountOptions(selectedModel).map((option) => (
                        <option key={option} value={option}>{option} 张</option>
                      ))}
                    </select>
                  </div>
                </div>
              ) : null}
              </div>
            </section>
          </div>
        </form>
      </aside>
    )
  }

  return (
    <PanelShell
      onSubmit={() => {
        if (!selectedModel) {
          onError('请先选择可用模型。')
          return
        }

        void submitGeneration(selectedModel)
      }}
    >
      <label className="sr-only" htmlFor="workbench-image-type-accessibility">
        图片类型
      </label>
      <select
        className="sr-only"
        disabled={false}
        id="workbench-image-type-accessibility"
        onChange={(event) => setImageType(normalizeWorkbenchImageType(event.target.value))}
        value={imageType}
        tabIndex={-1}
      >
        {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>

      {selectedModel?.supportsEdit || editSourceReference ? (
        <div className="grid gap-3">
          <ImageDropzone
            allowUpload={selectedModel?.supportsEdit === true}
            availableReferences={availableReferences}
            disabled={isGenerating || selectedModel?.supportsEdit !== true}
            editSourceReference={editSourceReference}
            maxReferences={selectedModel?.supportsMultiReference ? undefined : 1}
            onChange={setReferences}
            onError={onError}
            onEditSourceRemoved={onEditSourceRemoved}
            references={references}
          />
          {references.some((reference) => reference.kind === 'pending') ? (
            <Button disabled={isGenerating || !onSavePendingReferences} onClick={() => void savePendingReferences()} variant="secondary">
              保存为产品参考图
            </Button>
          ) : null}
        </div>
      ) : null}

      <PromptEditor disabled={isGenerating} imageType={imageType} onChange={setPrompt} onError={onError} projectId={projectId} value={prompt} />

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
            {supportedSizeOptions.length > 0 ? (
              <SelectField
                disabled={isGenerating}
                getOptionLabel={labelForSizePreset}
                id="image-size"
                label={parameterLabels.sizeLabel}
                onChange={setSize}
                options={supportedSizeOptions}
                value={size}
              />
            ) : null}

            {supportedQualityOptions.length > 0 ? (
              <SelectField
                disabled={isGenerating}
                getOptionLabel={(value) => labelForQualityPreset(value, { modelName: selectedModel.modelName })}
                id="image-quality"
                label={parameterLabels.qualityLabel}
                onChange={setQuality}
                options={supportedQualityOptions}
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
        {isGenerating ? '正在提交...' : '生成图片'}
      </Button>
    </PanelShell>
  )

  async function savePendingReferences(): Promise<WorkbenchReferenceInput[]> {
    const pendingReferences = references.filter((reference) => reference.kind === 'pending')
    if (pendingReferences.length === 0) {
      return references
    }
    if (!onSavePendingReferences) {
      onError('当前产品不可保存参考图，请刷新后重试。')
      return references
    }

    const savedReferences = await onSavePendingReferences(pendingReferences.map((reference) => reference.file))
    if (savedReferences.length === 0) {
      onError('参考图保存失败，请检查图片或稍后重试。')
      return references
    }

    const nextReferences = [...references.filter((reference) => reference.kind === 'asset'), ...savedReferences]
    revokeReferencePreviewUrls(pendingReferences)
    setReferences(nextReferences)
    return nextReferences
  }

  async function submitGeneration(selectedModel: Model) {
    const referencesForSubmit = references.some((reference) => reference.kind === 'pending') ? await savePendingReferences() : references
    const nextWorkbenchInput = buildCurrentWorkbenchInput(selectedModel, referencesForSubmit)
    if (!nextWorkbenchInput) {
      onError('请先选择可用模型。')
      return
    }
    void onGenerate(
      {
        prompt,
      },
      nextWorkbenchInput,
    )
  }

  function buildCurrentWorkbenchInput(selectedModel: Model, nextReferences: WorkbenchReferenceInput[]): WorkbenchTaskInput {
    return buildWorkbenchTaskInput(selectedModel, {
      size,
      quality,
      outputFormat,
      imageType,
      imageCount,
      references: nextReferences,
    })
  }
}

interface SelectFieldProps {
  disabled: boolean
  getOptionLabel?: (value: string) => string
  id: string
  label: string
  options: string[]
  value: string
  onChange: (value: string) => void
}

function SelectField({ disabled, getOptionLabel = identityOptionLabel, id, label, options, value, onChange }: SelectFieldProps) {
  return (
    <div className="space-y-2">
      <label className="field-label" htmlFor={id}>
        {label}
      </label>
      <select className="field-input" disabled={disabled} id={id} onChange={(event) => onChange(event.target.value)} value={value}>
        {options.map((option) => (
          <option key={option} value={option}>
            {getOptionLabel(option)}
          </option>
        ))}
      </select>
    </div>
  )
}

function CanvasSelectField({ disabled, getOptionLabel = identityOptionLabel, id, label, options, value, onChange }: SelectFieldProps) {
  return (
    <label className="grid gap-1.5 text-xs font-semibold text-slate-400" htmlFor={id}>
      {label}
      <select
        className="canvas-dark-field"
        disabled={disabled}
        id={id}
        onChange={(event) => onChange(event.target.value)}
        value={value}
      >
        {options.map((option) => (
          <option key={option} value={option}>{getOptionLabel(option)}</option>
        ))}
      </select>
    </label>
  )
}

function identityOptionLabel(value: string): string {
  return value
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
        className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto p-4"
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

function prioritizeAutoOption(options: string[]): string[] {
  if (!options.includes('auto') || options[0] === 'auto') {
    return options
  }

  return ['auto', ...options.filter((option) => option !== 'auto')]
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
