export interface ApiSuccessResponse<TData> {
  data: TData
  requestId: string
}

export interface ApiPage<TRecord> {
  records: TRecord[]
  total: number
  pageNum: number
  pageSize: number
}

export type ApiPaginatedResponse<TRecord> = ApiSuccessResponse<ApiPage<TRecord>>

export interface ApiErrorBody {
  code: string
  message: string
  details?: unknown
}

export interface ApiErrorResponse {
  error: ApiErrorBody
  requestId?: string
}

export type ApiResponse<TData> = ApiSuccessResponse<TData> | ApiErrorResponse

export type QueryParamValue = string | number | boolean | null | undefined
export type QueryParamRecord = Record<string, QueryParamValue | QueryParamValue[]>
