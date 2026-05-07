export class FriendlyError extends Error {
  readonly code: string

  constructor(message: string, code = 'FRIENDLY_ERROR') {
    super(message)
    this.code = code
    this.name = 'FriendlyError'
  }
}

export class StorageLimitError extends FriendlyError {
  constructor(message = '当前本地存储空间不足，请删除部分历史记录后重试。') {
    super(message, 'STORAGE_LIMIT_EXCEEDED')
    this.name = 'StorageLimitError'
  }
}

export function toFriendlyError(error: unknown): FriendlyError {
  if (error instanceof FriendlyError) {
    return error
  }

  if (error instanceof TypeError) {
    return new FriendlyError('网络请求失败，请检查 API URL、网络连接或浏览器 CORS 限制。', 'NETWORK_ERROR')
  }

  if (error instanceof Error) {
    return new FriendlyError(error.message || '操作失败，请稍后重试。')
  }

  return new FriendlyError('操作失败，请稍后重试。')
}

export async function parseApiError(response: Response): Promise<FriendlyError> {
  let message = `请求失败，服务返回 ${response.status}。`

  try {
    const data = (await response.json()) as {
      error?: { message?: string; code?: string }
      message?: string
    }
    const remoteMessage = data.error?.message ?? data.message
    if (remoteMessage) {
      message = remoteMessage
    }
  } catch {
    const text = await response.text().catch(() => '')
    if (text) {
      message = text.slice(0, 240)
    }
  }

  if (response.status === 401 || response.status === 403) {
    return new FriendlyError('API Key 无效或没有权限，请检查设置中的密钥。', 'AUTH_ERROR')
  }

  if (response.status === 400 || response.status === 422) {
    return new FriendlyError(`参数不正确：${message}`, 'VALIDATION_ERROR')
  }

  if (response.status === 429) {
    return new FriendlyError('请求过于频繁或额度不足，请稍后重试。', 'RATE_LIMITED')
  }

  if (response.status === 502 || response.status === 503 || response.status === 504) {
    return new FriendlyError('图片中转站上游服务暂时不可用或超时，请稍后重试。', 'UPSTREAM_UNAVAILABLE')
  }

  return new FriendlyError(message, 'API_ERROR')
}
