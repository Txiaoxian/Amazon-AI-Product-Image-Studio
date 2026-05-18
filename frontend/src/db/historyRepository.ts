import { createId, db, type HistoryItem, type StoredImage } from './dexie'
import { deleteImages, getImage } from './imageRepository'
import type { GeneratedImageResult, GenerationRequest } from '../providers/types'

export interface HistoryWithImage {
  item: HistoryItem
  image?: StoredImage
}

export interface CreateHistoryInput {
  request: GenerationRequest
  result: GeneratedImageResult
  generatedImageId: string
  referenceImageIds: string[]
}

export async function createHistoryItem(input: CreateHistoryInput): Promise<HistoryItem> {
  const now = new Date().toISOString()
  const item: HistoryItem = {
    id: createId('hist'),
    prompt: input.request.prompt,
    model: input.request.model.model,
    modelLabel: input.request.model.label,
    provider: input.request.model.provider,
    quality: input.request.quality,
    aspectRatio: input.request.aspectRatio,
    imageCount: input.request.imageCount,
    fileSize: input.result.fileSize,
    width: input.result.width,
    height: input.result.height,
    createdAt: now,
    durationMs: input.result.durationMs,
    imageId: input.generatedImageId,
    referenceImageIds: input.referenceImageIds,
  }

  await db.historyItems.put(item)
  return item
}

export async function listHistory(): Promise<HistoryWithImage[]> {
  const items = await db.historyItems.orderBy('createdAt').reverse().toArray()
  const images = await Promise.all(items.map((item) => getImage(item.imageId)))

  return items.map((item, index) => ({
    item,
    image: images[index],
  }))
}

export async function getHistoryItem(id: string): Promise<HistoryWithImage | undefined> {
  const item = await db.historyItems.get(id)

  if (!item) {
    return undefined
  }

  return {
    item,
    image: await getImage(item.imageId),
  }
}

export async function deleteHistoryItem(id: string): Promise<void> {
  const item = await db.historyItems.get(id)

  if (!item) {
    return
  }

  await db.transaction('rw', db.historyItems, db.images, async () => {
    await db.historyItems.delete(id)
    await deleteImages([item.imageId, ...item.referenceImageIds])
  })
}

export async function clearHistory(): Promise<void> {
  await db.transaction('rw', db.historyItems, db.images, async () => {
    await db.historyItems.clear()
    await db.images.clear()
  })
}
