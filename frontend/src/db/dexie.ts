import Dexie, { type Table } from 'dexie'

export interface PromptTemplate {
  id: string
  title: string
  prompt: string
  createdAt: string
  updatedAt: string
}

export class StudioDatabase extends Dexie {
  promptTemplates!: Table<PromptTemplate, string>

  constructor() {
    super('amazon-ai-product-image-studio')

    this.version(1).stores({
      images: '&id, purpose, createdAt',
      historyItems: '&id, createdAt, model, provider',
      promptTemplates: '&id, updatedAt',
    })
    this.version(2).stores({
      images: null,
      historyItems: null,
      promptTemplates: '&id, updatedAt',
    })
  }
}

export const db = new StudioDatabase()

export function createId(prefix: string): string {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}_${random}`
}
