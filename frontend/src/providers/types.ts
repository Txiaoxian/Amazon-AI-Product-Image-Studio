import type { FixedImageResolutionValue, IMAGE_CANVAS_OPTIONS, IMAGE_COUNT_OPTIONS, ImageResolutionValue } from '../lib/constants'

export type ProviderId = 'openai' | 'gemini' | 'relay2'
export type ImageResolution = ImageResolutionValue
export type FixedImageResolution = FixedImageResolutionValue
export type AspectRatio = (typeof IMAGE_CANVAS_OPTIONS)[number]
export type ImageCount = (typeof IMAGE_COUNT_OPTIONS)[number]
export type ImagePurpose = 'generated' | 'reference'

export interface ImageModelConfig {
  id: string
  label: string
  provider: ProviderId
  model: string
  description: string
}

export interface ReferenceImageInput {
  file: File
  previewUrl: string
}

export interface GenerationRequest {
  prompt: string
  model: ImageModelConfig
  quality: ImageResolution
  aspectRatio: AspectRatio
  imageCount: ImageCount
  references: File[]
  referenceImageUrls: string[]
}

export interface GeneratedImageResult {
  blob: Blob
  mimeType: string
  width: number
  height: number
  fileSize: number
  durationMs: number
}

export interface HistoryItem {
  id: string
  prompt: string
  model: string
  modelLabel: string
  provider: ProviderId
  quality: ImageResolution
  aspectRatio: AspectRatio
  imageCount: ImageCount
  fileSize: number
  width: number
  height: number
  createdAt: string
  durationMs: number
  imageId: string
  referenceImageIds: string[]
}
