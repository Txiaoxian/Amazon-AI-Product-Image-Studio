import { fileNameTimestamp } from './imageMeta'

export function downloadBlob(blob: Blob, fileName?: string): void {
  const extension = blob.type.includes('png') ? 'png' : blob.type.includes('webp') ? 'webp' : 'jpg'
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')

  anchor.href = url
  anchor.download = fileName ?? `amazon-ai-product-image-${fileNameTimestamp()}.${extension}`
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}
