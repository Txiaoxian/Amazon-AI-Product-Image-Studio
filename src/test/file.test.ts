import { describe, expect, it } from 'vitest'
import { base64ToBlob, blobToBase64, validateImageFile } from '../lib/file'

describe('file utilities', () => {
  it('converts blobs to base64 and back', async () => {
    const source = new Blob(['hello'], { type: 'image/png' })
    const base64 = await blobToBase64(source)
    const restored = base64ToBlob(base64, 'image/png')

    expect(restored.type).toBe('image/png')
    expect(await readBlobText(restored)).toBe('hello')
  })

  it('rejects unsupported reference files', () => {
    const file = new File(['text'], 'notes.txt', { type: 'text/plain' })
    expect(() => validateImageFile(file)).toThrow('仅支持')
  })
})

async function readBlobText(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error)
    reader.readAsText(blob)
  })
}
