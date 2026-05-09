import { useCallback, useEffect, useState } from 'react'
import { clearHistory, deleteHistoryItem, listHistory, type HistoryWithImage } from '../db/historyRepository'
import { toFriendlyError } from '../lib/errors'

export function useHistory() {
  const [items, setItems] = useState<HistoryWithImage[]>([])
  const [error, setError] = useState('')
  const [isLoading, setIsLoading] = useState(true)

  const refresh = useCallback(async () => {
    setIsLoading(true)
    setError('')
    try {
      setItems(await listHistory())
    } catch (err) {
      setError(toFriendlyError(err).message)
    } finally {
      setIsLoading(false)
    }
  }, [])

  const remove = useCallback(
    async (id: string) => {
      await deleteHistoryItem(id)
      await refresh()
    },
    [refresh],
  )

  const clear = useCallback(async () => {
    await clearHistory()
    await refresh()
  }, [refresh])

  useEffect(() => {
    void refresh()
  }, [refresh])

  return {
    items,
    error,
    isLoading,
    refresh,
    remove,
    clear,
  }
}
