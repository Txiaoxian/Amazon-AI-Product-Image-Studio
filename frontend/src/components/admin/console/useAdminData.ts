import { useCallback, useEffect, useEffectEvent, useRef, useState } from 'react'

export type AdminDataStatus = 'loading' | 'success' | 'error'

export function useAdminData<T>(loader: () => Promise<T>, dependencies: readonly unknown[]) {
  const requestSequence = useRef(0)
  const [data, setData] = useState<T | null>(null)
  const [status, setStatus] = useState<AdminDataStatus>('loading')
  const [reloadToken, setReloadToken] = useState(0)
  const dependencyKey = JSON.stringify(dependencies)

  const performLoad = useEffectEvent(async () => {
    const sequence = requestSequence.current + 1
    requestSequence.current = sequence
    setStatus('loading')
    try {
      const next = await loader()
      if (requestSequence.current !== sequence) return
      setData(next)
      setStatus('success')
    } catch {
      if (requestSequence.current !== sequence) return
      setStatus('error')
    }
  })

  const reload = useCallback(() => setReloadToken((current) => current + 1), [])

  useEffect(() => {
    void dependencyKey
    void reloadToken
    void performLoad()
    return () => {
      requestSequence.current += 1
    }
  }, [dependencyKey, reloadToken])

  return { data, status, reload }
}
