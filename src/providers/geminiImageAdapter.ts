import { FriendlyError } from '../lib/errors'
import { base64ToBlob } from '../lib/file'
import { getImageDimensions } from '../lib/imageMeta'
import { isTutujinSizeOption } from '../lib/constants'
import { assertOk, normalizeApiUrl } from './providerClient'
import type { AspectRatio, GeneratedImageResult, GenerationRequest, ImageResolution, ProviderAdapter, ProviderSettings } from './types'

interface GeminiInlineData {
  data?: string
  mimeType?: string
  mime_type?: string
}

interface GeminiPart {
  text?: string
  inlineData?: GeminiInlineData
  inline_data?: GeminiInlineData
  thought?: boolean
}

interface GeminiGenerateContentResponse {
  candidates?: Array<{
    content?: {
      parts?: GeminiPart[]
    }
  }>
}

interface GeminiRequestPart {
  text?: string
  fileData?: {
    fileUri: string
    mimeType?: string
  }
}

interface GeminiImageConfig {
  aspectRatio?: string
  imageSize?: string
}

const tutujinSizeToGeminiConfig: Partial<Record<AspectRatio, GeminiImageConfig>> = {
  '1024x1024': { aspectRatio: '1:1', imageSize: '1K' },
  '1536x1024': { aspectRatio: '3:2', imageSize: '1K' },
  '1024x1536': { aspectRatio: '2:3', imageSize: '1K' },
  '2048x2048': { aspectRatio: '1:1', imageSize: '2K' },
  '2048x1152': { aspectRatio: '16:9', imageSize: '2K' },
  '3840x2160': { aspectRatio: '16:9', imageSize: '4K' },
  '2160x3840': { aspectRatio: '9:16', imageSize: '4K' },
}

const qualityToGeminiImageSize: Partial<Record<ImageResolution, string>> = {
  low: '1K',
  medium: '2K',
  high: '4K',
}

function extractNativeImageItems(payload: GeminiGenerateContentResponse): Array<{ b64Json: string; mimeType: string }> {
  const imageItems =
    payload.candidates
      ?.flatMap((candidate) => candidate.content?.parts ?? [])
      .filter((part) => !part.thought)
      .map((part) => part.inlineData ?? part.inline_data)
      .filter((inlineData): inlineData is GeminiInlineData => Boolean(inlineData?.data))
      .map((inlineData) => ({
        b64Json: inlineData.data as string,
        mimeType: inlineData.mimeType ?? inlineData.mime_type ?? 'image/png',
      })) ?? []

  if (imageItems.length === 0) {
    throw new FriendlyError('Nano Banana 没有返回可用图片，请调整提示词或稍后重试。', 'EMPTY_IMAGE_RESPONSE')
  }

  return imageItems
}

export const geminiImageAdapter: ProviderAdapter = {
  provider: 'gemini',

  async generateImages(request: GenerationRequest, settings: ProviderSettings): Promise<GeneratedImageResult[]> {
    if (!settings.apiKey.trim()) {
      throw new FriendlyError('请先在设置中填写中转站 API Key。', 'MISSING_API_KEY')
    }

    return generateTutujinNativeImages(request, settings)
  },
}

async function generateTutujinNativeImages(request: GenerationRequest, settings: ProviderSettings): Promise<GeneratedImageResult[]> {
  if (!isTutujinSizeOption(request.aspectRatio)) {
    throw new FriendlyError('Nano Banana 2 尺寸参数不正确，请选择页面提供的 WxH 尺寸。', 'UNSUPPORTED_IMAGE_SIZE')
  }

  const startedAt = performance.now()
  const apiUrl = buildNativeApiUrl(settings.apiUrl, request.model.model)
  const referenceImageUrls = normalizeReferenceImageUrls(request)
  const parts = buildNativeRequestParts(request.prompt, referenceImageUrls)
  const generationConfig = buildNativeGenerationConfig(request.aspectRatio, request.quality)
  const imageRequests = Array.from({ length: request.imageCount }, () =>
    requestNativeImage(apiUrl, settings.apiKey.trim(), parts, generationConfig),
  )
  const responses = await Promise.all(imageRequests)
  const images = responses.flatMap(extractNativeImageItems).slice(0, request.imageCount)
  const durationMs = Math.round(performance.now() - startedAt)

  return Promise.all(images.map((image) => buildResultFromBase64(image.b64Json, image.mimeType, durationMs)))
}

async function requestNativeImage(
  apiUrl: string,
  apiKey: string,
  parts: GeminiRequestPart[],
  generationConfig: { responseModalities: string[]; imageConfig?: GeminiImageConfig },
): Promise<GeminiGenerateContentResponse> {
  const response = await fetch(apiUrl, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      contents: [
        {
          parts,
        },
      ],
      generationConfig,
    }),
  })

  await assertOk(response)
  return (await response.json()) as GeminiGenerateContentResponse
}

function buildNativeApiUrl(apiUrl: string, model: string): string {
  const normalizedApiUrl = normalizeApiUrl(apiUrl)

  try {
    const url = new URL(normalizedApiUrl)
    const pathname = url.pathname.replace(/\/+$/, '').replace(/\/v1$/, '')
    url.pathname = `${pathname}/v1beta/models/${encodeURIComponent(model)}:generateContent`
    url.search = ''
    url.hash = ''
    return url.toString()
  } catch {
    return `${normalizedApiUrl.replace(/\/v1$/, '')}/v1beta/models/${encodeURIComponent(model)}:generateContent`
  }
}

function buildNativeRequestParts(prompt: string, referenceImageUrls: string[]): GeminiRequestPart[] {
  return [
    { text: prompt },
    ...referenceImageUrls.map((url) => ({
      fileData: {
        fileUri: url,
        mimeType: inferImageMimeType(url),
      },
    })),
  ]
}

function buildNativeGenerationConfig(
  size: AspectRatio,
  quality: ImageResolution,
): { responseModalities: string[]; imageConfig?: GeminiImageConfig } {
  const imageConfig = size === 'auto' ? buildAutoImageConfig(quality) : tutujinSizeToGeminiConfig[size]

  return {
    responseModalities: ['TEXT', 'IMAGE'],
    ...(imageConfig && Object.keys(imageConfig).length > 0 ? { imageConfig } : {}),
  }
}

function buildAutoImageConfig(quality: ImageResolution): GeminiImageConfig | undefined {
  const imageSize = qualityToGeminiImageSize[quality]
  return imageSize ? { imageSize } : undefined
}

function inferImageMimeType(url: string): string | undefined {
  const pathname = safeUrlPathname(url).toLowerCase()

  if (pathname.endsWith('.jpg') || pathname.endsWith('.jpeg')) {
    return 'image/jpeg'
  }

  if (pathname.endsWith('.png')) {
    return 'image/png'
  }

  if (pathname.endsWith('.webp')) {
    return 'image/webp'
  }

  return undefined
}

function safeUrlPathname(url: string): string {
  try {
    return new URL(url).pathname
  } catch {
    return ''
  }
}

async function buildResultFromBase64(base64: string, mimeType: string, durationMs: number): Promise<GeneratedImageResult> {
  const blob = base64ToBlob(base64, mimeType)
  const dimensions = await getImageDimensions(blob)

  return {
    blob,
    mimeType: blob.type,
    width: dimensions.width,
    height: dimensions.height,
    fileSize: blob.size,
    durationMs,
  }
}

function normalizeReferenceImageUrls(request: GenerationRequest): string[] {
  if (request.references.length > 0) {
    throw new FriendlyError('Nano Banana 2 中转站图生图需要公开 HTTPS 图片 URL，请在参考图 URL 中填写图片链接。', 'UNSUPPORTED_LOCAL_REFERENCE_IMAGE')
  }

  const referenceImageUrls = request.referenceImageUrls.map((url) => url.trim()).filter(Boolean)

  if (referenceImageUrls.length > 4) {
    throw new FriendlyError('Nano Banana 2 最多支持 4 张参考图 URL。', 'TOO_MANY_REFERENCE_IMAGES')
  }

  const invalidUrl = referenceImageUrls.find((url) => !/^https:\/\//i.test(url))

  if (invalidUrl) {
    throw new FriendlyError(`参考图 URL 必须是公开 HTTPS 链接：${invalidUrl}`, 'INVALID_REFERENCE_IMAGE_URL')
  }

  return referenceImageUrls
}
