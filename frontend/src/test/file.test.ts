import { describe, expect, it } from 'vitest'
import { MAX_REFERENCE_IMAGE_BYTES } from '../lib/constants'
import { validateImageFile } from '../lib/file'

describe('file utilities', () => {
  it('rejects unsupported reference files', () => {
    const file = new File(['text'], 'notes.txt', { type: 'text/plain' })
    expect(() => validateImageFile(file)).toThrow('仅支持')
  })

  it('rejects oversized reference files', () => {
    const file = new File([new Uint8Array(MAX_REFERENCE_IMAGE_BYTES + 1)], 'large.png', { type: 'image/png' })
    expect(() => validateImageFile(file)).toThrow('单张参考图不能超过')
  })
})
