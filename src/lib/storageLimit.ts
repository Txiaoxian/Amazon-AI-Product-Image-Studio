export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) {
    return '0B'
  }

  const units = ['B', 'KB', 'MB', 'GB']
  const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / 1024 ** unitIndex

  const displayValue = unitIndex === 0 || value >= 10 ? value.toFixed(0) : value.toFixed(1)
  return `${displayValue.replace(/\.0$/, '')}${units[unitIndex]}`
}

export function isWithinStorageLimit(currentBytes: number, incomingBytes: number, limitBytes: number): boolean {
  return currentBytes + incomingBytes <= limitBytes
}

export function storagePercent(usedBytes: number, limitBytes: number): number {
  if (limitBytes <= 0) {
    return 0
  }

  return Math.min(100, Math.round((usedBytes / limitBytes) * 100))
}
