import { ALLOWED_IMAGE_TYPES, MAX_REFERENCE_IMAGE_BYTES } from './constants'
import { FriendlyError } from './errors'

export function validateImageFile(file: File): void {
  if (!ALLOWED_IMAGE_TYPES.includes(file.type as (typeof ALLOWED_IMAGE_TYPES)[number])) {
    throw new FriendlyError('仅支持 JPG、PNG、WebP 格式的参考图。', 'INVALID_FILE_TYPE')
  }

  if (file.size > MAX_REFERENCE_IMAGE_BYTES) {
    throw new FriendlyError('单张参考图不能超过 15MB。', 'FILE_TOO_LARGE')
  }
}
