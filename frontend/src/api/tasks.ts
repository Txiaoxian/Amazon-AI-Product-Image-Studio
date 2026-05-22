import type { ApiPage, QueryParamRecord } from '../types/api'
import type { BackendHistoryItem, ListProjectHistoryParams } from '../types/history'
import type { CreateTaskRequest, ProjectId, Task, TaskId, TaskStatus, TaskType } from '../types/platform'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface ListTasksParams {
  status?: TaskStatus
  type?: TaskType
  pageNum?: number
  pageSize?: number
}

export interface TaskApi {
  create(projectId: ProjectId | string, request: CreateTaskRequest, csrfToken: string): Promise<Task>
  list(projectId: ProjectId | string, params?: ListTasksParams): Promise<ApiPage<Task>>
  listHistory(projectId: ProjectId | string, params?: ListProjectHistoryParams): Promise<ApiPage<BackendHistoryItem>>
  get(taskId: TaskId | string): Promise<Task>
  cancel(taskId: TaskId | string, csrfToken: string): Promise<Task>
  retry(taskId: TaskId | string, csrfToken: string): Promise<Task>
}

export function createTaskApi(client: ApiClient = apiClient): TaskApi {
  return {
    create: (projectId, request, csrfToken) =>
      client.post<Task>(`/projects/${encodeURIComponent(projectId)}/tasks`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    list: (projectId, params = {}) =>
      client.get<ApiPage<Task>>(`/projects/${encodeURIComponent(projectId)}/tasks`, {
        query: taskListQuery(params),
      }),
    listHistory: (projectId, params = {}) =>
      client.get<ApiPage<BackendHistoryItem>>(`/projects/${encodeURIComponent(projectId)}/history`, {
        query: historyListQuery(params),
      }),
    get: (taskId) => client.get<Task>(`/tasks/${encodeURIComponent(taskId)}`),
    cancel: (taskId, csrfToken) =>
      client.post<Task>(`/tasks/${encodeURIComponent(taskId)}/cancel`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    retry: (taskId, csrfToken) =>
      client.post<Task>(`/tasks/${encodeURIComponent(taskId)}/retry`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
  }
}

function taskListQuery(params: ListTasksParams): QueryParamRecord {
  return {
    status: params.status,
    type: params.type,
    pageNum: params.pageNum,
    pageSize: params.pageSize,
  }
}

function historyListQuery(params: ListProjectHistoryParams): QueryParamRecord {
  return {
    pageNum: params.pageNum,
    pageSize: params.pageSize,
    kind: params.kind,
  }
}

export const taskApi = createTaskApi()
