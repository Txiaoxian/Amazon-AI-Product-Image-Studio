import { createId, db, type ImagePurpose, type StoredImage } from './dexie'

export interface SaveImageInput {
  blob: Blob
  mimeType: string
  size: number
  purpose: ImagePurpose
  width?: number
  height?: number
}

export async function saveImage(input: SaveImageInput): Promise<StoredImage> {
  const image: StoredImage = {
    id: createId(input.purpose === 'generated' ? 'img' : 'ref'),
    blob: input.blob,
    mimeType: input.mimeType,
    size: input.size,
    width: input.width,
    height: input.height,
    purpose: input.purpose,
    createdAt: new Date().toISOString(),
  }

  await db.images.put(image)
  return image
}

export async function getImage(imageId: string): Promise<StoredImage | undefined> {
  return db.images.get(imageId)
}

export async function getImages(imageIds: string[]): Promise<StoredImage[]> {
  if (imageIds.length === 0) {
    return []
  }

  const images = await db.images.bulkGet(imageIds)
  return images.filter((image): image is StoredImage => Boolean(image))
}

export async function deleteImages(imageIds: string[]): Promise<void> {
  if (imageIds.length > 0) {
    await db.images.bulkDelete(imageIds)
  }
}

export async function getTotalImageBytes(): Promise<number> {
  const images = await db.images.toArray()
  return images.reduce((total, image) => total + image.size, 0)
}

export async function clearImages(): Promise<void> {
  await db.images.clear()
}
