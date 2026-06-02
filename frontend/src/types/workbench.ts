import type { AssetId, ModelId, ProviderId } from './platform'

export const WORKBENCH_IMAGE_TYPE_OPTIONS = [
  { value: 'MAIN', label: '主图' },
  { value: 'A_PLUS', label: 'A+ 图片' },
  { value: 'SCENE', label: '场景图' },
  { value: 'DETAIL', label: '细节图' },
  { value: 'DIMENSION', label: '尺寸图' },
  { value: 'SELLING_POINT', label: '卖点图' },
  { value: 'COMPARISON', label: '对比图' },
] as const

export type WorkbenchImageType = (typeof WORKBENCH_IMAGE_TYPE_OPTIONS)[number]['value']

export const DEFAULT_WORKBENCH_IMAGE_TYPE: WorkbenchImageType = 'MAIN'

export function normalizeWorkbenchImageType(value?: string | null): WorkbenchImageType {
  const normalized = value?.trim().toUpperCase()
  return WORKBENCH_IMAGE_TYPE_OPTIONS.some((option) => option.value === normalized)
    ? (normalized as WorkbenchImageType)
    : DEFAULT_WORKBENCH_IMAGE_TYPE
}

export interface AssetReferenceInput {
  kind: 'asset'
  assetId: AssetId
  filename: string
  previewUrl: string
}

export interface PendingReferenceInput {
  kind: 'pending'
  file: File
  previewUrl: string
}

export type WorkbenchReferenceInput = AssetReferenceInput | PendingReferenceInput

export interface WorkbenchTaskInput {
  providerId: ProviderId
  modelId: ModelId
  imageType: WorkbenchImageType
  referenceAssetIds: AssetId[]
  editSourceAssetId?: AssetId
  parameters: {
    size?: string
    quality?: string
    outputFormat?: string
    outputCount: number
  }
}

export interface WorkbenchTaskSubmission {
  prompt: string
}
