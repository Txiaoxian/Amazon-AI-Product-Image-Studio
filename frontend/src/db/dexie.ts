import Dexie, { type Table } from 'dexie'
import type { HistoryItem, ImagePurpose } from '../providers/types'

export interface StoredImage {
  id: string
  blob: Blob
  mimeType: string
  size: number
  width?: number
  height?: number
  purpose: ImagePurpose
  createdAt: string
}

export interface PromptTemplate {
  id: string
  title: string
  prompt: string
  createdAt: string
  updatedAt: string
}

export class StudioDatabase extends Dexie {
  images!: Table<StoredImage, string>
  historyItems!: Table<HistoryItem, string>
  promptTemplates!: Table<PromptTemplate, string>

  constructor() {
    super('amazon-ai-product-image-studio')

    this.version(1).stores({
      images: '&id, purpose, createdAt',
      historyItems: '&id, createdAt, model, provider',
      promptTemplates: '&id, updatedAt',
    })
  }
}

export const db = new StudioDatabase()

export function createId(prefix: string): string {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}_${random}`
}
