import { useCallback, useEffect, useState } from 'react'
import { getTotalImageBytes } from '../db/imageRepository'

export function useStorageUsage() {
  const [usedBytes, setUsedBytes] = useState(0)
  const [isLoading, setIsLoading] = useState(true)

  const refresh = useCallback(async () => {
    setIsLoading(true)
    try {
      setUsedBytes(await getTotalImageBytes())
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return {
    usedBytes,
    isLoading,
    refresh,
  }
}
