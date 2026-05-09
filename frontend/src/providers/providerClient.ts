import { parseApiError } from '../lib/errors'

export function normalizeApiUrl(apiUrl: string): string {
  const trimmedUrl = apiUrl.trim().replace(/\/+$/, '')

  if (!trimmedUrl) {
    return ''
  }

  try {
    const url = new URL(trimmedUrl)

    if (url.hostname === 'api.tutujin.app') {
      url.hostname = 'api.tutujin.com'
    }

    const pathname = url.pathname.replace(/\/+$/, '')
    const basePath = pathname.endsWith('/images/generations')
      ? pathname.slice(0, -'/images/generations'.length).replace(/\/+$/, '')
      : pathname

    url.pathname = basePath.endsWith('/v1') ? basePath : `${basePath}/v1`
    url.search = ''
    url.hash = ''

    return url.toString().replace(/\/+$/, '')
  } catch {
    const baseUrl = trimmedUrl.endsWith('/images/generations')
      ? trimmedUrl.slice(0, -'/images/generations'.length).replace(/\/+$/, '')
      : trimmedUrl

    return /(^|\/)v1$/.test(baseUrl) ? baseUrl : `${baseUrl}/v1`
  }
}

export async function assertOk(response: Response): Promise<void> {
  if (!response.ok) {
    throw await parseApiError(response)
  }
}
