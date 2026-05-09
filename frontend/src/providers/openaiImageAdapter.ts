import { FriendlyError } from '../lib/errors'
import { base64ToBlob } from '../lib/file'
import { getImageDimensions } from '../lib/imageMeta'
import {
  DEFAULT_RELAY2_API_URL,
  isOpenAIAspectRatioOption,
  isOpenAIResolutionOption,
  RELAY2_PROXY_API_URL,
  RESOLUTION_OPTIONS,
} from '../lib/constants'
import { resizeImageBlob } from '../lib/imageResize'
import { assertOk, normalizeApiUrl } from './providerClient'
import type { AspectRatio, GeneratedImageResult, GenerationRequest, ImageResolution, ProviderAdapter, ProviderSettings } from './types'

interface OpenAIImageResponse {
  data?: Array<{
    b64_json?: string
    revised_prompt?: string
    url?: string
  }>
}

const openAIMinPixels = 655360

interface OpenAIOutputSize {
  width: number
  height: number
}

interface OpenAIImageSizePlan {
  requestSize: string
  outputSize?: OpenAIOutputSize
}

type OpenAIImageItem = { kind: 'base64'; value: string } | { kind: 'url'; value: string }

type OpenAIImageResolution = (typeof RESOLUTION_OPTIONS)[number]
const relay2DefaultApiHost = new URL(DEFAULT_RELAY2_API_URL).hostname

const resolutionBase: Record<OpenAIImageResolution, number> = {
  '1K': 1024,
  '2K': 2048,
  '4K': 3840,
}

const openAIPixelBudget: Record<OpenAIImageResolution, number> = {
  '1K': 1024 * 1024,
  '2K': 2048 * 2048,
  '4K': 3840 * 2160,
}

const amazonRatio = '1.62:1'

const amazonRatioOutputSize: Record<OpenAIImageResolution, OpenAIOutputSize> = {
  '1K': { width: 970, height: 600 },
  '2K': { width: 1940, height: 1200 },
  '4K': { width: 3104, height: 1920 },
}

const amazonRatioRequestSizeOverrides: Partial<Record<OpenAIImageResolution, OpenAIOutputSize>> = {
  '4K': amazonRatioOutputSize['4K'],
}

export function mapOpenAIImageSize(aspectRatio: AspectRatio, quality: ImageResolution): string {
  if (quality === 'auto') {
    return 'auto'
  }

  if (!isOpenAIResolutionOption(quality)) {
    throw new FriendlyError('OpenAI 不支持当前输出质量，请选择自动、1K、2K 或 4K。', 'UNSUPPORTED_IMAGE_RESOLUTION')
  }

  if (!isOpenAIAspectRatioOption(aspectRatio)) {
    throw new FriendlyError('OpenAI 图片比例参数不正确，请选择页面提供的图片比例。', 'UNSUPPORTED_IMAGE_ASPECT_RATIO')
  }

  const requestSizeOverride = getOpenAIRequestSizeOverride(aspectRatio, quality)

  if (requestSizeOverride) {
    return formatOpenAIImageSize(requestSizeOverride)
  }

  const [rawWidth, rawHeight] = aspectRatio.split(':').map(Number)
  const base = resolutionBase[quality]
  const isLandscape = rawWidth >= rawHeight
  const initialWidth = isLandscape ? base : Math.round((base * rawWidth) / rawHeight)
  const initialHeight = isLandscape ? Math.round((base * rawHeight) / rawWidth) : base
  const pixelBudget = openAIPixelBudget[quality]
  const scale = Math.min(1, Math.sqrt(pixelBudget / (initialWidth * initialHeight)))
  const [width, height] = ensureOpenAIMinimumPixels(
    floorToMultipleOfSixteen(initialWidth * scale),
    floorToMultipleOfSixteen(initialHeight * scale),
  )

  return `${width}x${height}`
}

function buildOpenAIImageSizePlan(aspectRatio: AspectRatio, quality: ImageResolution): OpenAIImageSizePlan {
  const outputSize = getOpenAIOutputSize(aspectRatio, quality)

  return {
    requestSize: mapOpenAIImageSize(aspectRatio, quality),
    outputSize,
  }
}

function getOpenAIOutputSize(aspectRatio: AspectRatio, quality: ImageResolution): OpenAIOutputSize | undefined {
  if (aspectRatio !== amazonRatio || !isOpenAIResolutionOption(quality)) {
    return undefined
  }

  return amazonRatioOutputSize[quality]
}

function getOpenAIRequestSizeOverride(aspectRatio: AspectRatio, quality: OpenAIImageResolution): OpenAIOutputSize | undefined {
  if (aspectRatio !== amazonRatio) {
    return undefined
  }

  return amazonRatioRequestSizeOverrides[quality]
}

function formatOpenAIImageSize(size: OpenAIOutputSize): string {
  return `${size.width}x${size.height}`
}

function floorToMultipleOfSixteen(value: number): number {
  return Math.max(256, Math.floor(value / 16) * 16)
}

function ceilToMultipleOfSixteen(value: number): number {
  return Math.max(256, Math.ceil(value / 16) * 16)
}

function ensureOpenAIMinimumPixels(width: number, height: number): [number, number] {
  if (width * height >= openAIMinPixels) {
    return [width, height]
  }

  const scale = Math.sqrt(openAIMinPixels / (width * height))
  return [ceilToMultipleOfSixteen(width * scale), ceilToMultipleOfSixteen(height * scale)]
}

function extractOpenAIImageItems(payload: OpenAIImageResponse): OpenAIImageItem[] {
  const imageItems =
    payload.data
      ?.map((item): OpenAIImageItem | null => {
        if (item.b64_json) {
          return { kind: 'base64', value: item.b64_json }
        }

        if (item.url) {
          return { kind: 'url', value: item.url }
        }

        return null
      })
      .filter((item): item is OpenAIImageItem => Boolean(item)) ?? []

  if (imageItems.length === 0) {
    throw new FriendlyError('图片服务没有返回可用图片，请调整提示词或稍后重试。', 'EMPTY_IMAGE_RESPONSE')
  }

  return imageItems
}

async function resolveOpenAIImageBlobs(payload: OpenAIImageResponse): Promise<Blob[]> {
  const imageItems = extractOpenAIImageItems(payload)

  return Promise.all(
    imageItems.map((item) => {
      if (item.kind === 'base64') {
        return Promise.resolve(base64ToBlob(item.value, 'image/png'))
      }

      return downloadOpenAIImageUrl(item.value)
    }),
  )
}

async function downloadOpenAIImageUrl(url: string): Promise<Blob> {
  const response = await fetch(url)

  if (!response.ok) {
    throw new FriendlyError('图片服务返回了图片 URL，但下载图片失败，请稍后重试。', 'IMAGE_URL_DOWNLOAD_FAILED')
  }

  const blob = await response.blob()

  if (blob.size === 0) {
    throw new FriendlyError('图片服务返回了空图片文件，请稍后重试。', 'EMPTY_IMAGE_FILE')
  }

  if (blob.type && !blob.type.startsWith('image/')) {
    throw new FriendlyError('图片服务返回的 URL 不是有效图片文件。', 'INVALID_IMAGE_URL_FILE')
  }

  return blob
}

export const openaiImageAdapter: ProviderAdapter = {
  provider: 'openai',

  async generateImages(request: GenerationRequest, settings: ProviderSettings): Promise<GeneratedImageResult[]> {
    if (!settings.apiKey.trim()) {
      throw new FriendlyError('请先在设置中填写 OpenAI API Key。', 'MISSING_API_KEY')
    }

    const startedAt = performance.now()
    const apiUrl = resolveOpenAIRequestApiUrl(request, settings.apiUrl)
    const sizePlan = buildOpenAIImageSizePlan(request.aspectRatio, request.quality)
    const headers = {
      Authorization: `Bearer ${settings.apiKey.trim()}`,
    }

    const response =
      request.references.length > 0
        ? await requestEdit(apiUrl, headers, request, sizePlan.requestSize)
        : await requestGeneration(apiUrl, headers, request, sizePlan.requestSize)

    await assertOk(response)

    const payload = (await response.json()) as OpenAIImageResponse
    const durationMs = Math.round(performance.now() - startedAt)
    const blobs = await resolveOpenAIImageBlobs(payload)

    return Promise.all(
      blobs.map(async (blob) => {
        const outputBlob = sizePlan.outputSize ? await resizeImageBlob(blob, sizePlan.outputSize) : blob
        const dimensions = sizePlan.outputSize ?? (await getImageDimensions(outputBlob))

        return {
          blob: outputBlob,
          mimeType: outputBlob.type,
          width: dimensions.width,
          height: dimensions.height,
          fileSize: outputBlob.size,
          durationMs,
        }
      }),
    )
  },
}

function resolveOpenAIRequestApiUrl(request: GenerationRequest, configuredApiUrl: string): string {
  const apiUrl = normalizeApiUrl(configuredApiUrl)

  if (request.model.provider !== 'relay2') {
    return apiUrl
  }

  try {
    const url = new URL(apiUrl)

    if (url.hostname !== relay2DefaultApiHost) {
      return apiUrl
    }

    return `${RELAY2_PROXY_API_URL}${url.pathname}`.replace(/\/+$/, '')
  } catch {
    return apiUrl
  }
}

async function requestGeneration(
  apiUrl: string,
  headers: Record<string, string>,
  request: GenerationRequest,
  size: string,
): Promise<Response> {
  return fetch(`${apiUrl}/images/generations`, {
    method: 'POST',
    headers: {
      ...headers,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: request.model.model,
      prompt: request.prompt,
      size,
      n: request.imageCount,
    }),
  })
}

async function requestEdit(
  apiUrl: string,
  headers: Record<string, string>,
  request: GenerationRequest,
  size: string,
): Promise<Response> {
  const formData = new FormData()
  formData.append('model', request.model.model)
  formData.append('prompt', request.prompt)
  formData.append('size', size)
  formData.append('n', String(request.imageCount))

  request.references.forEach((file) => {
    formData.append('image[]', file, file.name)
  })

  return fetch(`${apiUrl}/images/edits`, {
    method: 'POST',
    headers,
    body: formData,
  })
}
