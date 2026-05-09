import { useEffect, useState } from 'react'

export function useObjectUrl(blob?: Blob): string {
  const [url, setUrl] = useState('')

  useEffect(() => {
    if (!blob) {
      setUrl('')
      return
    }

    const nextUrl = URL.createObjectURL(blob)
    setUrl(nextUrl)

    return () => URL.revokeObjectURL(nextUrl)
  }, [blob])

  return url
}
