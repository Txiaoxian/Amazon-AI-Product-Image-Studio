import type { ApiPage, QueryParamRecord } from '../types/api'
import type { Project, ProjectId, ProjectMember, ProjectMemberCandidate, ProjectMemberRole, ProjectStatus, UserId } from '../types/platform'
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
  sortOrder?: number
}

export interface UpdateProjectRequest {
  name?: string
  brand?: string
  asin?: string
  site?: string
  notes?: string
  status?: ProjectStatus
  sortOrder?: number
}

export interface ListProjectMemberCandidatesParams {
  q?: string
  pageNum?: number
  pageSize?: number
}

export interface ProjectMemberRequest {
  userId: UserId | string
  role: ProjectMemberRole
}

export interface UpdateProjectMemberRequest {
  role: ProjectMemberRole
}

export interface ProjectApi {
  list(params?: ListProjectsParams): Promise<ApiPage<Project>>
  create(request: CreateProjectRequest, csrfToken: string): Promise<Project>
  get(projectId: ProjectId | string): Promise<Project>
  update(projectId: ProjectId | string, request: UpdateProjectRequest, csrfToken: string): Promise<Project>
  delete(projectId: ProjectId | string, csrfToken: string): Promise<{ ok: boolean }>
  listMembers(projectId: ProjectId | string): Promise<ProjectMember[]>
  listMemberCandidates(projectId: ProjectId | string, params?: ListProjectMemberCandidatesParams): Promise<ProjectMemberCandidate[]>
  addMember(projectId: ProjectId | string, request: ProjectMemberRequest, csrfToken: string): Promise<ProjectMember>
  updateMember(
    projectId: ProjectId | string,
    userId: UserId | string,
    request: UpdateProjectMemberRequest,
    csrfToken: string,
  ): Promise<ProjectMember>
  removeMember(projectId: ProjectId | string, userId: UserId | string, csrfToken: string): Promise<{ ok: boolean }>
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
    listMembers: (projectId) => client.get<ProjectMember[]>(`/projects/${encodeURIComponent(projectId)}/members`),
    listMemberCandidates: (projectId, params = {}) =>
      client.get<ProjectMemberCandidate[]>(`/projects/${encodeURIComponent(projectId)}/member-candidates`, {
        query: memberCandidateQuery(params),
      }),
    addMember: (projectId, request, csrfToken) =>
      client.post<ProjectMember>(`/projects/${encodeURIComponent(projectId)}/members`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    updateMember: (projectId, userId, request, csrfToken) =>
      client.patch<ProjectMember>(`/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    removeMember: (projectId, userId, csrfToken) =>
      client.delete<{ ok: boolean }>(`/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(userId)}`, {
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

function memberCandidateQuery(params: ListProjectMemberCandidatesParams): QueryParamRecord {
  return {
    q: params.q,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

export const projectApi = createProjectApi()
