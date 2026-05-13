import type { ApiPage, QueryParamRecord } from '../types/api'
import type { Provider, ProviderId, ProviderStatus, ProviderTestResult, ProviderType } from '../types/platform'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface ListProvidersParams {
  type?: ProviderType
  status?: ProviderStatus
  pageNum?: number
  pageSize?: number
}

export interface CreateProviderRequest {
  type: ProviderType
  name: string
  baseUrl: string
  apiKey: string
  status?: ProviderStatus
  timeoutSeconds?: number
  concurrencyLimit?: number
}

export interface UpdateProviderRequest {
  name?: string
  baseUrl?: string
  apiKey?: string
  status?: ProviderStatus
  timeoutSeconds?: number
  concurrencyLimit?: number
}

export interface ProviderApi {
  list(params?: ListProvidersParams): Promise<ApiPage<Provider>>
  create(request: CreateProviderRequest, csrfToken: string): Promise<Provider>
  get(providerId: ProviderId | string): Promise<Provider>
  update(providerId: ProviderId | string, request: UpdateProviderRequest, csrfToken: string): Promise<Provider>
  delete(providerId: ProviderId | string, csrfToken: string): Promise<{ ok: boolean }>
  enable(providerId: ProviderId | string, csrfToken: string): Promise<Provider>
  disable(providerId: ProviderId | string, csrfToken: string): Promise<Provider>
  test(providerId: ProviderId | string, csrfToken: string): Promise<ProviderTestResult>
}

export function createProviderApi(client: ApiClient = apiClient): ProviderApi {
  return {
    list: (params = {}) => client.get<ApiPage<Provider>>('/providers', { query: providerListQuery(params) }),
    create: (request, csrfToken) =>
      client.post<Provider>('/providers', request, {
        headers: csrfHeaders(csrfToken),
      }),
    get: (providerId) => client.get<Provider>(`/providers/${encodeURIComponent(providerId)}`),
    update: (providerId, request, csrfToken) =>
      client.patch<Provider>(`/providers/${encodeURIComponent(providerId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    delete: (providerId, csrfToken) =>
      client.delete<{ ok: boolean }>(`/providers/${encodeURIComponent(providerId)}`, {
        headers: csrfHeaders(csrfToken),
      }),
    enable: (providerId, csrfToken) =>
      client.post<Provider>(`/providers/${encodeURIComponent(providerId)}/enable`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    disable: (providerId, csrfToken) =>
      client.post<Provider>(`/providers/${encodeURIComponent(providerId)}/disable`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    test: (providerId, csrfToken) =>
      client.post<ProviderTestResult>(`/providers/${encodeURIComponent(providerId)}/test`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
  }
}

function providerListQuery(params: ListProvidersParams): QueryParamRecord {
  return {
    type: params.type,
    status: params.status,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

export const providerApi = createProviderApi()
