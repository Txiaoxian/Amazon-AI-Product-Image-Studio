import { describe, expect, it } from 'vitest'
import { parseApiError } from '../lib/errors'

describe('parseApiError', () => {
  it('maps upstream gateway failures to a friendly retryable message', async () => {
    const error = await parseApiError(
      new Response('<html><h1>502 Bad Gateway</h1></html>', {
        status: 502,
        headers: { 'Content-Type': 'text/html' },
      }),
    )

    expect(error.code).toBe('UPSTREAM_UNAVAILABLE')
    expect(error.message).toBe('图片中转站上游服务暂时不可用或超时，请稍后重试。')
  })
})
