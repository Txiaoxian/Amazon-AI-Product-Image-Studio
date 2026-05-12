import type {
  ApiErrorBody,
  ApiErrorResponse,
  ApiSuccessResponse,
  QueryParamRecord,
  QueryParamValue,
} from '../types/api'

export const DEFAULT_API_BASE_URL = '/api/v1'

export type JsonRequestBody = object

export interface ApiRequestOptions extends Omit<RequestInit, 'body'> {
  baseUrl?: string
  body?: BodyInit | JsonRequestBody
  fetchImpl?: typeof fetch
  query?: QueryParamRecord
}

export interface ApiClientConfig {
  baseUrl?: string
  fetchImpl?: typeof fetch
}

export interface ApiClient {
  request<TData>(path: string, options?: ApiRequestOptions): Promise<TData>
  raw(path: string, options?: ApiRequestOptions): Promise<Response>
  get<TData>(path: string, options?: ApiRequestOptions): Promise<TData>
  post<TData>(path: string, body?: ApiRequestOptions['body'], options?: ApiRequestOptions): Promise<TData>
  patch<TData>(path: string, body?: ApiRequestOptions['body'], options?: ApiRequestOptions): Promise<TData>
  delete<TData>(path: string, options?: ApiRequestOptions): Promise<TData>
}

export interface ApiClientErrorOptions {
  status: number
  code: string
  message: string
  requestId?: string
  details?: unknown
}

export class ApiClientError extends Error {
  readonly status: number
  readonly code: string
  readonly requestId?: string
  readonly details?: unknown

  constructor(options: ApiClientErrorOptions) {
    super(options.message)
    this.name = 'ApiClientError'
    this.status = options.status
    this.code = options.code
    this.requestId = options.requestId
    this.details = options.details
  }
}

export function isApiClientError(error: unknown): error is ApiClientError {
  return error instanceof ApiClientError
}

export function isUnauthorizedError(error: unknown): boolean {
  return isApiClientError(error) && error.status === 401
}

export const CSRF_HEADER_NAME = 'X-CSRF-Token'

export function csrfHeaders(csrfToken: string): HeadersInit {
  return {
    [CSRF_HEADER_NAME]: csrfToken,
  }
}

export function createApiClient(config: ApiClientConfig = {}): ApiClient {
  const request = <TData>(path: string, options: ApiRequestOptions = {}) =>
    apiRequest<TData>(path, {
      ...options,
      baseUrl: options.baseUrl ?? config.baseUrl,
      fetchImpl: options.fetchImpl ?? config.fetchImpl,
    })
  const raw = (path: string, options: ApiRequestOptions = {}) =>
    apiRawRequest(path, {
      ...options,
      baseUrl: options.baseUrl ?? config.baseUrl,
      fetchImpl: options.fetchImpl ?? config.fetchImpl,
    })

  return {
    request,
    raw,
    get: <TData>(path: string, options: ApiRequestOptions = {}) =>
      request<TData>(path, { ...options, method: 'GET' }),
    post: <TData>(path: string, body?: ApiRequestOptions['body'], options: ApiRequestOptions = {}) =>
      request<TData>(path, { ...options, body, method: 'POST' }),
    patch: <TData>(path: string, body?: ApiRequestOptions['body'], options: ApiRequestOptions = {}) =>
      request<TData>(path, { ...options, body, method: 'PATCH' }),
    delete: <TData>(path: string, options: ApiRequestOptions = {}) =>
      request<TData>(path, { ...options, method: 'DELETE' }),
  }
}

export const apiClient = createApiClient()

export async function apiRequest<TData>(path: string, options: ApiRequestOptions = {}): Promise<TData> {
  const response = await apiRawRequest(path, options)
  return parseApiSuccessResponse<TData>(response)
}

export async function apiRawRequest(path: string, options: ApiRequestOptions = {}): Promise<Response> {
  const {
    baseUrl = DEFAULT_API_BASE_URL,
    body: requestBody,
    fetchImpl: configuredFetch,
    query,
    ...requestInit
  } = options
  const fetchImpl = configuredFetch ?? globalThis.fetch

  if (!fetchImpl) {
    throw new ApiClientError({
      status: 0,
      code: 'FETCH_UNAVAILABLE',
      message: 'Fetch is not available in this environment.',
    })
  }

  const headers = new Headers(requestInit.headers)
  const body = normalizeRequestBody(requestBody, headers)
  const response = await fetchImpl(buildApiUrl(baseUrl, path, query), {
    ...requestInit,
    body,
    credentials: requestInit.credentials ?? 'include',
    headers,
  })

  if (!response.ok) {
    throw await parseApiErrorResponse(response)
  }

  return response
}

export async function parseApiSuccessResponse<TData>(response: Response): Promise<TData> {
  if (response.status === 204) {
    return undefined as TData
  }

  const payload = await readResponsePayload(response)
  if (isApiSuccessResponse<TData>(payload.json)) {
    return payload.json.data
  }

  throw new ApiClientError({
    status: response.status,
    code: 'INVALID_RESPONSE',
    message: 'API response did not match the expected success envelope.',
    requestId: getRequestId(response, payload.json),
  })
}

export async function parseApiErrorResponse(response: Response): Promise<ApiClientError> {
  const payload = await readResponsePayload(response)
  const requestId = getRequestId(response, payload.json)

  if (isApiErrorResponse(payload.json)) {
    return new ApiClientError({
      status: response.status,
      code: payload.json.error.code,
      message: payload.json.error.message,
      requestId,
      details: payload.json.error.details,
    })
  }

  const fallback = getFallbackError(response.status)
  return new ApiClientError({
    status: response.status,
    code: fallback.code,
    message: fallback.message,
    requestId,
  })
}

export function buildApiUrl(baseUrl: string, path: string, query?: QueryParamRecord): string {
  const normalizedBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const search = buildSearchParams(query)
  return `${normalizedBase}${normalizedPath}${search ? `?${search}` : ''}`
}

function buildSearchParams(query?: QueryParamRecord): string {
  if (!query) {
    return ''
  }

  const params = new URLSearchParams()
  Object.entries(query).forEach(([key, value]) => {
    appendQueryValue(params, key, value)
  })
  return params.toString()
}

function appendQueryValue(
  params: URLSearchParams,
  key: string,
  value: QueryParamValue | QueryParamValue[],
): void {
  if (Array.isArray(value)) {
    value.forEach((item) => appendQueryValue(params, key, item))
    return
  }

  if (value === null || value === undefined) {
    return
  }

  params.append(key, String(value))
}

function normalizeRequestBody(body: ApiRequestOptions['body'], headers: Headers): BodyInit | undefined {
  if (body === undefined) {
    return undefined
  }

  if (isNativeBody(body)) {
    return body
  }

  if (!headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  return JSON.stringify(body)
}

function isNativeBody(body: ApiRequestOptions['body']): body is BodyInit {
  if (typeof body === 'string') {
    return true
  }

  if (typeof FormData !== 'undefined' && body instanceof FormData) {
    return true
  }

  if (typeof Blob !== 'undefined' && body instanceof Blob) {
    return true
  }

  if (typeof URLSearchParams !== 'undefined' && body instanceof URLSearchParams) {
    return true
  }

  if (typeof ArrayBuffer !== 'undefined' && body instanceof ArrayBuffer) {
    return true
  }

  return ArrayBuffer.isView(body)
}

interface ParsedResponsePayload {
  json?: unknown
}

async function readResponsePayload(response: Response): Promise<ParsedResponsePayload> {
  const text = await response.text().catch(() => '')

  if (!text) {
    return {}
  }

  try {
    return { json: JSON.parse(text) as unknown }
  } catch {
    return {}
  }
}

function isApiSuccessResponse<TData>(value: unknown): value is ApiSuccessResponse<TData> {
  return isRecord(value) && 'data' in value && typeof value.requestId === 'string'
}

function isApiErrorResponse(value: unknown): value is ApiErrorResponse {
  return isRecord(value) && isApiErrorBody(value.error)
}

function isApiErrorBody(value: unknown): value is ApiErrorBody {
  return (
    isRecord(value) &&
    typeof value.code === 'string' &&
    typeof value.message === 'string'
  )
}

function getRequestId(response: Response, json: unknown): string | undefined {
  if (isRecord(json) && typeof json.requestId === 'string') {
    return json.requestId
  }

  return response.headers.get('X-Request-Id') ?? undefined
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function getFallbackError(status: number): Pick<ApiClientErrorOptions, 'code' | 'message'> {
  if (status === 401) {
    return { code: 'UNAUTHENTICATED', message: 'Authentication is required.' }
  }

  if (status === 403) {
    return { code: 'FORBIDDEN', message: 'You are not allowed to perform this action.' }
  }

  if (status === 404) {
    return { code: 'NOT_FOUND', message: 'The requested resource was not found.' }
  }

  if (status === 409) {
    return { code: 'CONFLICT', message: 'The request conflicts with the current resource state.' }
  }

  if (status === 422) {
    return { code: 'VALIDATION_ERROR', message: 'The request data is invalid.' }
  }

  if (status === 429) {
    return { code: 'RATE_LIMITED', message: 'Too many requests. Please try again later.' }
  }

  if (status >= 500) {
    return { code: 'INTERNAL_ERROR', message: 'The server could not process the request.' }
  }

  return { code: 'REQUEST_FAILED', message: `Request failed with status ${status}.` }
}
