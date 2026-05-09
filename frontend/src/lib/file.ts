import { ALLOWED_IMAGE_TYPES, MAX_REFERENCE_IMAGE_BYTES } from './constants'
import { FriendlyError } from './errors'

export async function blobToBase64(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => {
      const result = String(reader.result ?? '')
      resolve(result.includes(',') ? result.split(',')[1] : result)
    }
    reader.onerror = () => reject(new FriendlyError('读取图片文件失败，请重新选择图片。'))
    reader.readAsDataURL(blob)
  })
}

export function base64ToBlob(base64: string, mimeType: string): Blob {
  const binary = window.atob(base64)
  const bytes = new Uint8Array(binary.length)

  for (let index = 0; index < binary.length; index += 1) {
    bytes[index] = binary.charCodeAt(index)
  }

  return new Blob([bytes], { type: mimeType })
}

export function validateImageFile(file: File): void {
  if (!ALLOWED_IMAGE_TYPES.includes(file.type as (typeof ALLOWED_IMAGE_TYPES)[number])) {
    throw new FriendlyError('仅支持 JPG、PNG、WebP 格式的参考图。', 'INVALID_FILE_TYPE')
  }

  if (file.size > MAX_REFERENCE_IMAGE_BYTES) {
    throw new FriendlyError('单张参考图不能超过 15MB。', 'FILE_TOO_LARGE')
  }
}

export function buildObjectUrl(blob?: Blob): string {
  return blob ? URL.createObjectURL(blob) : ''
}
