import { beforeEach, describe, expect, it } from 'vitest'
import { db } from '../db/dexie'
import { createHistoryItem, deleteHistoryItem, listHistory } from '../db/historyRepository'
import { saveImage } from '../db/imageRepository'
import { IMAGE_MODELS } from '../providers/registry'

describe('historyRepository', () => {
  beforeEach(async () => {
    await db.delete()
    await db.open()
  })

  it('creates, lists and deletes history with linked images', async () => {
    const generated = await saveImage({
      blob: new Blob(['generated'], { type: 'image/png' }),
      mimeType: 'image/png',
      size: 9,
      width: 1024,
      height: 1024,
      purpose: 'generated',
    })

    await createHistoryItem({
      request: {
        prompt: 'white background product image',
        model: IMAGE_MODELS[0],
        quality: '1K',
        aspectRatio: '1:1',
        imageCount: 1,
        references: [],
        referenceImageUrls: [],
      },
      result: {
        blob: generated.blob,
        mimeType: 'image/png',
        width: 1024,
        height: 1024,
        fileSize: 9,
        durationMs: 1000,
      },
      generatedImageId: generated.id,
      referenceImageIds: [],
    })

    const items = await listHistory()
    expect(items).toHaveLength(1)
    expect(items[0].image?.id).toBe(generated.id)

    await deleteHistoryItem(items[0].item.id)
    expect(await listHistory()).toHaveLength(0)
    expect(await db.images.count()).toBe(0)
  })
})
