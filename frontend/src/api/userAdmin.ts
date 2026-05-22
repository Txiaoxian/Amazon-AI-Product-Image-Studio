import type { QueryParamRecord } from '../types/api'
import type {
  CreateUserAdminUserRequest,
  ListUsersParams,
  ReplaceUserRolesRequest,
  UpdateUserAdminUserRequest,
  UserAdminPage,
  UserAdminPermission,
  UserAdminRole,
  UserAdminUser,
} from '../types/userAdmin'
import type { UserId } from '../types/platform'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface UserAdminApi {
  listUsers(params?: ListUsersParams): Promise<UserAdminPage>
  createUser(request: CreateUserAdminUserRequest, csrfToken: string): Promise<UserAdminUser>
  getUser(userId: UserId | string): Promise<UserAdminUser>
  updateUser(userId: UserId | string, request: UpdateUserAdminUserRequest, csrfToken: string): Promise<UserAdminUser>
  disableUser(userId: UserId | string, csrfToken: string): Promise<UserAdminUser>
  enableUser(userId: UserId | string, csrfToken: string): Promise<UserAdminUser>
  replaceUserRoles(userId: UserId | string, request: ReplaceUserRolesRequest, csrfToken: string): Promise<UserAdminUser>
  listRoles(): Promise<UserAdminRole[]>
  listPermissions(): Promise<UserAdminPermission[]>
}

export function createUserAdminApi(client: ApiClient = apiClient): UserAdminApi {
  return {
    listUsers: (params = {}) => client.get<UserAdminPage>('/users', { query: listUsersQuery(params) }),
    createUser: (request, csrfToken) =>
      client.post<UserAdminUser>('/users', request, {
        headers: csrfHeaders(csrfToken),
      }),
    getUser: (userId) => client.get<UserAdminUser>(`/users/${encodeURIComponent(userId)}`),
    updateUser: (userId, request, csrfToken) =>
      client.patch<UserAdminUser>(`/users/${encodeURIComponent(userId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    disableUser: (userId, csrfToken) =>
      client.post<UserAdminUser>(`/users/${encodeURIComponent(userId)}/disable`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    enableUser: (userId, csrfToken) =>
      client.post<UserAdminUser>(`/users/${encodeURIComponent(userId)}/enable`, undefined, {
        headers: csrfHeaders(csrfToken),
      }),
    replaceUserRoles: (userId, request, csrfToken) =>
      client.post<UserAdminUser>(`/users/${encodeURIComponent(userId)}/roles`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    listRoles: () => client.get<UserAdminRole[]>('/roles'),
    listPermissions: () => client.get<UserAdminPermission[]>('/permissions'),
  }
}

function listUsersQuery(params: ListUsersParams): QueryParamRecord {
  return {
    pageNum: params.pageNum,
    pageSize: params.pageSize,
    status: params.status,
    q: params.q,
  }
}

export const userAdminApi = createUserAdminApi()
