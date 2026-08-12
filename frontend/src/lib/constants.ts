export const APP_NAME = '亚马逊 AI 商品图工作室'

export const STORAGE_LIMIT_OPTIONS = [
  { label: '200MB', value: 200 * 1024 * 1024 },
  { label: '500MB', value: 500 * 1024 * 1024 },
  { label: '1GB', value: 1024 * 1024 * 1024 },
  { label: '2GB', value: 2 * 1024 * 1024 * 1024 },
] as const

export const DEFAULT_STORAGE_LIMIT_BYTES = 1024 * 1024 * 1024

export const RESOLUTION_OPTIONS = ['1K', '2K', '4K'] as const
export const TUTUJIN_QUALITY_OPTIONS = ['low', 'medium', 'high', 'auto'] as const
export const OPENAI_RESOLUTION_OPTIONS = ['auto', ...RESOLUTION_OPTIONS] as const
export const PROVIDER_RESOLUTION_OPTIONS = {
  openai: OPENAI_RESOLUTION_OPTIONS,
  gemini: TUTUJIN_QUALITY_OPTIONS,
  relay2: OPENAI_RESOLUTION_OPTIONS,
} as const
export const RESOLUTION_LABELS = {
  auto: '自动',
  '1K': '1K',
  '2K': '2K',
  '4K': '4K',
  low: 'Low',
  medium: 'Medium',
  high: 'High',
} as const

export type ProviderResolutionKey = keyof typeof PROVIDER_RESOLUTION_OPTIONS
export type ImageResolutionValue = (typeof OPENAI_RESOLUTION_OPTIONS)[number] | (typeof TUTUJIN_QUALITY_OPTIONS)[number]
export type FixedImageResolutionValue = Exclude<ImageResolutionValue, 'auto'>

export function getResolutionOptionsForProvider(provider: ProviderResolutionKey): readonly ImageResolutionValue[] {
  return PROVIDER_RESOLUTION_OPTIONS[provider]
}

export function getDefaultResolutionForProvider(provider: ProviderResolutionKey): ImageResolutionValue {
  return provider === 'gemini' ? 'medium' : RESOLUTION_OPTIONS[0]
}

export function isResolutionOption(resolution: string, options: readonly string[]): resolution is ImageResolutionValue {
  return options.includes(resolution)
}

export function isOpenAIResolutionOption(resolution: string): resolution is (typeof RESOLUTION_OPTIONS)[number] {
  return (RESOLUTION_OPTIONS as readonly string[]).includes(resolution)
}

export function isTutujinQualityOption(quality: string): quality is (typeof TUTUJIN_QUALITY_OPTIONS)[number] {
  return (TUTUJIN_QUALITY_OPTIONS as readonly string[]).includes(quality)
}

export const IMAGE_COUNT_OPTIONS = [1, 2, 3, 4] as const

export const ASPECT_RATIO_OPTIONS = [
  '1:1',
  '1:4',
  '1:8',
  '1.62:1',
  '2:3',
  '3:2',
  '3:4',
  '4:1',
  '4:3',
  '4:5',
  '5:4',
  '8:1',
  '9:16',
  '16:9',
  '21:9',
] as const

export const TUTUJIN_SIZE_OPTIONS = [
  '1024x1024',
  '1536x1024',
  '1024x1536',
  '2048x2048',
  '2048x1152',
  '3840x2160',
  '2160x3840',
  'auto',
] as const

export const IMAGE_CANVAS_OPTIONS = [...ASPECT_RATIO_OPTIONS, ...TUTUJIN_SIZE_OPTIONS] as const

export const PROVIDER_CANVAS_OPTIONS = {
  openai: ASPECT_RATIO_OPTIONS,
  gemini: TUTUJIN_SIZE_OPTIONS,
  relay2: ASPECT_RATIO_OPTIONS,
} as const

export type ProviderCanvasKey = keyof typeof PROVIDER_CANVAS_OPTIONS
export type ImageCanvasValue = (typeof IMAGE_CANVAS_OPTIONS)[number]

export const TUTUJIN_SIZE_LABELS: Record<(typeof TUTUJIN_SIZE_OPTIONS)[number], string> = {
  '1024x1024': '1:1 · 1024x1024',
  '1536x1024': '横图 · 1536x1024',
  '1024x1536': '竖图 · 1024x1536',
  '2048x2048': '2K 方图 · 2048x2048',
  '2048x1152': '2K 横图 · 2048x1152',
  '3840x2160': '4K 横图 · 3840x2160',
  '2160x3840': '4K 竖图 · 2160x3840',
  auto: '自动',
}

export function getCanvasOptionsForProvider(provider: ProviderCanvasKey): readonly ImageCanvasValue[] {
  return PROVIDER_CANVAS_OPTIONS[provider]
}

export function getDefaultCanvasForProvider(provider: ProviderCanvasKey): ImageCanvasValue {
  return provider === 'gemini' ? '1024x1024' : ASPECT_RATIO_OPTIONS[0]
}

export function isCanvasOption(canvas: string, options: readonly string[]): canvas is ImageCanvasValue {
  return options.includes(canvas)
}

export function isOpenAIAspectRatioOption(aspectRatio: string): aspectRatio is (typeof ASPECT_RATIO_OPTIONS)[number] {
  return (ASPECT_RATIO_OPTIONS as readonly string[]).includes(aspectRatio)
}

export function isTutujinSizeOption(size: string): size is (typeof TUTUJIN_SIZE_OPTIONS)[number] {
  return (TUTUJIN_SIZE_OPTIONS as readonly string[]).includes(size)
}

export const MAX_REFERENCE_IMAGE_BYTES = 15 * 1024 * 1024
export const ALLOWED_IMAGE_TYPES = ['image/jpeg', 'image/png', 'image/webp'] as const
