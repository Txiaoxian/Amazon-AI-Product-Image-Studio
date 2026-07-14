import Dexie, { type Table } from 'dexie'
import type { WorkbenchImageType } from '../types/workbench'

export interface PromptTemplate {
  id: string
  imageType: WorkbenchImageType
  title: string
  prompt: string
  createdAt: string
  updatedAt: string
}

export class StudioDatabase extends Dexie {
  promptTemplates!: Table<PromptTemplate, string>

  constructor(databaseName = 'amazon-ai-product-image-studio') {
    super(databaseName)

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
    this.version(3)
      .stores({
        promptTemplates: '&id, imageType, updatedAt',
      })
      .upgrade((transaction) =>
        transaction
          .table<PromptTemplate, string>('promptTemplates')
          .toCollection()
          .modify((template) => {
            template.imageType = template.imageType ?? 'MAIN'
          }),
      )
  }
}

export const db = new StudioDatabase()

export function createId(prefix: string): string {
  const random = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`
  return `${prefix}_${random}`
}
