import type { ApiPage, QueryParamRecord } from '../types/api'
import type { Project, ProjectId, ProjectStatus } from '../types/platform'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface ListProjectsParams {
  status?: ProjectStatus
  pageNum?: number
  pageSize?: number
}

export interface CreateProjectRequest {
  name: string
  brand?: string
  asin?: string
  site?: string
  notes?: string
  status?: ProjectStatus
}

export interface UpdateProjectRequest {
  name?: string
  brand?: string
  asin?: string
  site?: string
  notes?: string
  status?: ProjectStatus
}

export interface ProjectApi {
  list(params?: ListProjectsParams): Promise<ApiPage<Project>>
  create(request: CreateProjectRequest, csrfToken: string): Promise<Project>
  get(projectId: ProjectId | string): Promise<Project>
  update(projectId: ProjectId | string, request: UpdateProjectRequest, csrfToken: string): Promise<Project>
  delete(projectId: ProjectId | string, csrfToken: string): Promise<{ ok: boolean }>
}

export function createProjectApi(client: ApiClient = apiClient): ProjectApi {
  return {
    list: (params = {}) => client.get<ApiPage<Project>>('/projects', { query: projectListQuery(params) }),
    create: (request, csrfToken) =>
      client.post<Project>('/projects', request, {
        headers: csrfHeaders(csrfToken),
      }),
    get: (projectId) => client.get<Project>(`/projects/${encodeURIComponent(projectId)}`),
    update: (projectId, request, csrfToken) =>
      client.patch<Project>(`/projects/${encodeURIComponent(projectId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    delete: (projectId, csrfToken) =>
      client.delete<{ ok: boolean }>(`/projects/${encodeURIComponent(projectId)}`, {
        headers: csrfHeaders(csrfToken),
      }),
  }
}

function projectListQuery(params: ListProjectsParams): QueryParamRecord {
  return {
    status: params.status,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

export const projectApi = createProjectApi()
