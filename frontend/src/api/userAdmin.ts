import type { QueryParamRecord } from '../types/api'
import type {
  CreateUserAdminUserRequest,
  CreateUserAdminRoleRequest,
  CurrentTenantAdminResponse,
  ListUsersParams,
  ReplaceRolePermissionsRequest,
  ReplaceUserModelAccessRequest,
  ReplaceUserRolesRequest,
  UpdateCurrentTenantRequest,
  UpdateUserAdminRoleRequest,
  UpdateUserAdminUserRequest,
  UserAdminPage,
  UserAdminPermission,
  UserAdminRole,
  UserAdminUser,
  UserModelAccess,
} from '../types/userAdmin'
import type { UserId } from '../types/platform'
import { apiClient, csrfHeaders, type ApiClient } from './client'

export interface UserAdminApi {
  getCurrentTenant(): Promise<CurrentTenantAdminResponse>
  updateCurrentTenant(request: UpdateCurrentTenantRequest, csrfToken: string): Promise<CurrentTenantAdminResponse>
  listUsers(params?: ListUsersParams): Promise<UserAdminPage>
  createUser(request: CreateUserAdminUserRequest, csrfToken: string): Promise<UserAdminUser>
  getUser(userId: UserId | string): Promise<UserAdminUser>
  updateUser(userId: UserId | string, request: UpdateUserAdminUserRequest, csrfToken: string): Promise<UserAdminUser>
  disableUser(userId: UserId | string, csrfToken: string): Promise<UserAdminUser>
  enableUser(userId: UserId | string, csrfToken: string): Promise<UserAdminUser>
  replaceUserRoles(userId: UserId | string, request: ReplaceUserRolesRequest, csrfToken: string): Promise<UserAdminUser>
  getUserModelAccess(userId: UserId | string): Promise<UserModelAccess>
  replaceUserModelAccess(userId: UserId | string, request: ReplaceUserModelAccessRequest, csrfToken: string): Promise<UserModelAccess>
  listRoles(): Promise<UserAdminRole[]>
  createRole(request: CreateUserAdminRoleRequest, csrfToken: string): Promise<UserAdminRole>
  updateRole(roleId: string, request: UpdateUserAdminRoleRequest, csrfToken: string): Promise<UserAdminRole>
  deleteRole(roleId: string, csrfToken: string): Promise<void>
  replaceRolePermissions(roleId: string, request: ReplaceRolePermissionsRequest, csrfToken: string): Promise<UserAdminRole>
  listPermissions(): Promise<UserAdminPermission[]>
}

export function createUserAdminApi(client: ApiClient = apiClient): UserAdminApi {
  return {
    getCurrentTenant: () => client.get<CurrentTenantAdminResponse>('/tenants/current'),
    updateCurrentTenant: (request, csrfToken) =>
      client.patch<CurrentTenantAdminResponse>('/tenants/current', request, {
        headers: csrfHeaders(csrfToken),
      }),
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
    getUserModelAccess: (userId) => client.get<UserModelAccess>(`/users/${encodeURIComponent(userId)}/ai-access`),
    replaceUserModelAccess: (userId, request, csrfToken) =>
      client.request<UserModelAccess>(`/users/${encodeURIComponent(userId)}/ai-access`, {
        body: request,
        headers: csrfHeaders(csrfToken),
        method: 'PUT',
      }),
    listRoles: () => client.get<UserAdminRole[]>('/roles'),
    createRole: (request, csrfToken) =>
      client.post<UserAdminRole>('/roles', request, {
        headers: csrfHeaders(csrfToken),
      }),
    updateRole: (roleId, request, csrfToken) =>
      client.patch<UserAdminRole>(`/roles/${encodeURIComponent(roleId)}`, request, {
        headers: csrfHeaders(csrfToken),
      }),
    deleteRole: (roleId, csrfToken) =>
      client.delete<void>(`/roles/${encodeURIComponent(roleId)}`, {
        headers: csrfHeaders(csrfToken),
      }),
    replaceRolePermissions: (roleId, request, csrfToken) =>
      client.request<UserAdminRole>(`/roles/${encodeURIComponent(roleId)}/permissions`, {
        body: request,
        headers: csrfHeaders(csrfToken),
        method: 'PUT',
      }),
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
