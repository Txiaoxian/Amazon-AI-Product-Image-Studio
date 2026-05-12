import { Sparkles } from 'lucide-react'
import { useEffect, useState } from 'react'
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
import type { AspectRatio, GenerationRequest, ImageCount, ImageResolution, ReferenceImageInput } from '../../providers/types'
import { Button } from '../ui/Button'
import { ImageDropzone } from './ImageDropzone'
import { PromptEditor } from './PromptEditor'

export interface ControlPanelDraft {
  prompt: string
  modelId: string
  quality: ImageResolution
  aspectRatio: AspectRatio
  imageCount?: ImageCount
  references?: ReferenceImageInput[]
}

interface ControlPanelProps {
  defaultModelId: string
  defaultResolution: ImageResolution
  isGenerating: boolean
  draft?: ControlPanelDraft | null
  referenceToAdd?: ReferenceImageInput | null
  onGenerate: (request: GenerationRequest) => Promise<void>
  onError: (message: string) => void
  onReferenceAdded?: () => void
}

export function ControlPanel({
  defaultModelId,
  defaultResolution,
  isGenerating,
  draft,
  referenceToAdd,
  onGenerate,
  onError,
  onReferenceAdded,
}: ControlPanelProps) {
  const [prompt, setPrompt] = useState('')
  const [modelId, setModelId] = useState(defaultModelId)
  const [quality, setQuality] = useState<ImageResolution>(defaultResolution)
  const [aspectRatio, setAspectRatio] = useState<AspectRatio>('1:1')
  const [imageCount, setImageCount] = useState<ImageCount>(1)
  const [references, setReferences] = useState<ReferenceImageInput[]>([])
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
      currentReferences.forEach((reference) => URL.revokeObjectURL(reference.previewUrl))
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
      currentReferences.forEach((reference) => URL.revokeObjectURL(reference.previewUrl))
      return []
    })
  }, [references.length, supportsLocalReferences])

  useEffect(() => {
    if (!referenceToAdd) {
      return
    }

    if (!supportsLocalReferences) {
      URL.revokeObjectURL(referenceToAdd.previewUrl)
      onError('当前模型不支持本地参考图，请切换模型后重试。')
      onReferenceAdded?.()
      return
    }

    setReferences((currentReferences) => [...currentReferences, referenceToAdd])
    onReferenceAdded?.()
  }, [onError, onReferenceAdded, referenceToAdd, supportsLocalReferences])

  return (
    <aside className="panel flex min-h-0 flex-col">
      <div className="border-b border-ink-200 px-4 py-3">
        <h2 className="text-sm font-semibold text-ink-900">生成参数</h2>
      </div>
      <form
        className="flex flex-1 flex-col gap-5 p-4 xl:overflow-y-auto"
        onSubmit={(event) => {
          event.preventDefault()
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
            references: supportsLocalReferences ? references.map((reference) => reference.file) : [],
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
      </form>
    </aside>
  )
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
