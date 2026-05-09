import { describe, expect, it } from 'vitest'
import { formatBytes, isWithinStorageLimit, storagePercent } from '../lib/storageLimit'

describe('storageLimit', () => {
  it('formats byte values with readable units', () => {
    expect(formatBytes(0)).toBe('0B')
    expect(formatBytes(1024)).toBe('1KB')
    expect(formatBytes(1024 * 1024)).toBe('1MB')
  })

  it('checks whether incoming images fit the configured limit', () => {
    expect(isWithinStorageLimit(100, 50, 200)).toBe(true)
    expect(isWithinStorageLimit(100, 150, 200)).toBe(false)
  })

  it('caps usage percentage at 100', () => {
    expect(storagePercent(50, 200)).toBe(25)
    expect(storagePercent(400, 200)).toBe(100)
  })
})
