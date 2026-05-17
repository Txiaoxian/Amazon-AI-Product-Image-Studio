import type { ApiPage, QueryParamRecord } from '../types/api'
import type { Model, ModelId, ModelPricing, ModelStatus, ProviderId } from '../types/platform'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export type ModelCapabilityFilter = 'generate' | 'edit'

export interface ListModelsParams {
  providerId?: ProviderId | string
  status?: ModelStatus
  enabled?: boolean
  capability?: ModelCapabilityFilter
  pageNum?: number
  pageSize?: number
}

export interface CreateModelRequest {
  providerId: ProviderId | string
  modelName: string
  displayName: string
  supportsGenerate: boolean
  supportsEdit: boolean
  supportsMultiReference: boolean
  supportsN: boolean
  maxOutputCount: number
  supportedSizes: string[]
  supportedQualities: string[]
  supportedOutputFormats: string[]
  pricing: ModelPricing
  status?: ModelStatus
}

export type UpdateModelRequest = Partial<CreateModelRequest>

export interface ModelApi {
  list(params?: ListModelsParams): Promise<ApiPage<Model>>
  listEnabledCapabilities(capability?: ModelCapabilityFilter): Promise<Model[]>
  create(request: CreateModelRequest, csrfToken: string): Promise<Model>
  get(modelId: ModelId | string): Promise<Model>
  update(modelId: ModelId | string, request: UpdateModelRequest, csrfToken: string): Promise<Model>
  delete(modelId: ModelId | string, csrfToken: string): Promise<{ ok: boolean }>
  enable(modelId: ModelId | string, csrfToken: string): Promise<Model>
  disable(modelId: ModelId | string, csrfToken: string): Promise<Model>
}

export function createModelApi(client: ApiClient = apiClient): ModelApi {
  return {
    list: (params = {}) => client.get<ApiPage<Model>>('/models', { query: modelListQuery(params) }),
    listEnabledCapabilities: async (capability = 'generate') => {
      const page = await client.get<ApiPage<Model>>('/models', {
        query: modelListQuery({ enabled: true, capability, pageNum: 1, pageSize: 100 }),
      })
      return page.records
    },
    create: (request, csrfToken) =>
      client.post<Model>('/models', request, {
        headers: csrfHeaders(csrfToken),
      }),
    get: (modelId) => client.get<Model>(`/models/${encodeURIComponent(modelId)}`),
    update: (modelId, request, csrfToken) =>
      client.patch<Model>(`/models/${encodeURIComponent(modelId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    delete: (modelId, csrfToken) =>
      client.delete<{ ok: boolean }>(`/models/${encodeURIComponent(modelId)}`, {
        headers: csrfHeaders(csrfToken),
      }),
    enable: (modelId, csrfToken) =>
      client.post<Model>(`/models/${encodeURIComponent(modelId)}/enable`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    disable: (modelId, csrfToken) =>
      client.post<Model>(`/models/${encodeURIComponent(modelId)}/disable`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
  }
}

function modelListQuery(params: ListModelsParams): QueryParamRecord {
  return {
    providerId: params.providerId,
    status: params.status,
    enabled: params.enabled,
    capability: params.capability,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

export const modelApi = createModelApi()
