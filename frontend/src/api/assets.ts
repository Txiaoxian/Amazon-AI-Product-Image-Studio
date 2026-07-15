import type { ApiPage, QueryParamRecord } from '../types/api'
import type { Asset, AssetId, AssetKind, ProjectId } from '../types/platform'
import type { WorkbenchImageType } from '../types/workbench'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface ListAssetsParams {
  kind?: AssetKind
  category?: string
  favorite?: boolean
  imageType?: WorkbenchImageType
  pageNum?: number
  pageSize?: number
}

export interface UploadReferenceAssetRequest {
  file: File
  category?: WorkbenchImageType
  filename?: string
  isFavorite?: boolean
}

export interface UpdateAssetRequest {
  category?: WorkbenchImageType
  filename?: string
  isFavorite?: boolean
}

export interface AssetDownload {
  blob: Blob
  filename?: string
}

export interface AssetApi {
  list(projectId: ProjectId | string, params?: ListAssetsParams): Promise<ApiPage<Asset>>
  uploadReference(projectId: ProjectId | string, request: UploadReferenceAssetRequest, csrfToken: string): Promise<Asset>
  get(assetId: AssetId | string): Promise<Asset>
  update(assetId: AssetId | string, request: UpdateAssetRequest, csrfToken: string): Promise<Asset>
  delete(assetId: AssetId | string, csrfToken: string): Promise<{ ok: boolean }>
  favorite(assetId: AssetId | string, csrfToken: string): Promise<Asset>
  unfavorite(assetId: AssetId | string, csrfToken: string): Promise<Asset>
  download(assetId: AssetId | string): Promise<AssetDownload>
}

export function createAssetApi(client: ApiClient = apiClient): AssetApi {
  return {
    list: (projectId, params = {}) =>
      client.get<ApiPage<Asset>>(`/projects/${encodeURIComponent(projectId)}/assets`, {
        query: assetListQuery(params),
      }),
    uploadReference: (projectId, request, csrfToken) =>
      client.post<Asset>(`/projects/${encodeURIComponent(projectId)}/assets/uploads`, buildUploadFormData(request), {
        headers: csrfHeaders(csrfToken),
      }),
    get: (assetId) => client.get<Asset>(`/assets/${encodeURIComponent(assetId)}`),
    update: (assetId, request, csrfToken) =>
      client.patch<Asset>(`/assets/${encodeURIComponent(assetId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    delete: (assetId, csrfToken) =>
      client.delete<{ ok: boolean }>(`/assets/${encodeURIComponent(assetId)}`, {
        headers: csrfHeaders(csrfToken),
      }),
    favorite: (assetId, csrfToken) =>
      client.post<Asset>(`/assets/${encodeURIComponent(assetId)}/favorite`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    unfavorite: (assetId, csrfToken) =>
      client.delete<Asset>(`/assets/${encodeURIComponent(assetId)}/favorite`, {
        headers: csrfHeaders(csrfToken),
      }),
    download: async (assetId) => {
      const response = await client.raw(`/assets/${encodeURIComponent(assetId)}/download`, {
        method: 'GET',
      })
      return {
        blob: await response.blob(),
        filename: parseContentDispositionFilename(response.headers.get('Content-Disposition')),
      }
    },
  }
}

function assetListQuery(params: ListAssetsParams): QueryParamRecord {
  return {
    kind: params.kind,
    category: params.category,
    favorite: params.favorite,
    imageType: params.imageType,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

function buildUploadFormData(request: UploadReferenceAssetRequest): FormData {
  const formData = new FormData()
  formData.append('file', request.file)
  formData.append('kind', 'REFERENCE')

  if (request.category) {
    formData.append('category', request.category)
  }
  if (request.filename) {
    formData.append('filename', request.filename)
  }
  if (request.isFavorite !== undefined) {
    formData.append('isFavorite', String(request.isFavorite))
  }

  return formData
}

function parseContentDispositionFilename(value: string | null): string | undefined {
  if (!value) {
    return undefined
  }

  const utf8Match = value.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) {
    return decodeURIComponent(utf8Match[1])
  }

  const quotedMatch = value.match(/filename="([^"]+)"/i)
  if (quotedMatch?.[1]) {
    return quotedMatch[1]
  }

  const plainMatch = value.match(/filename=([^;]+)/i)
  return plainMatch?.[1]?.trim()
}

export const assetApi = createAssetApi()
