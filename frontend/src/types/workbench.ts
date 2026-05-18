import type { AssetId, ModelId, ProviderId } from './platform'

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
