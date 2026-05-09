import { formatBytes, storagePercent } from '../../lib/storageLimit'

interface StorageMeterProps {
  usedBytes: number
  limitBytes: number
}

export function StorageMeter({ usedBytes, limitBytes }: StorageMeterProps) {
  const percent = storagePercent(usedBytes, limitBytes)
  const isHigh = percent >= 90

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-xs text-ink-500">
        <span>本地存储</span>
        <span>
          {formatBytes(usedBytes)} / {formatBytes(limitBytes)}
        </span>
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-ink-100">
        <div
          className={`h-full rounded-full transition-all ${isHigh ? 'bg-red-500' : 'bg-amazon-500'}`}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  )
}
