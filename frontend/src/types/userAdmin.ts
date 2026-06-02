import type { ApiPage } from './api'
import type {
  ISODateTimeString,
  PermissionKey,
  RoleId,
  TenantId,
  TenantStatus,
  UserId,
  UserStatus,
} from './platform'

export type UserAdminRoleStatus = 'ACTIVE' | 'DISABLED'

export interface UserAdminPermission {
  id: PermissionKey | string
  code: PermissionKey | string
  name: string
  description: string
}

export interface UserAdminRole {
  id: RoleId
  tenantId: TenantId
  code: string
  name: string
  description: string
  status: UserAdminRoleStatus
  permissions?: UserAdminPermission[]
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
}

export interface UserAdminUser {
  id: UserId
  tenantId: TenantId
  email: string
  displayName: string
  status: UserStatus
  lastLoginAt: ISODateTimeString | null
  createdAt: ISODateTimeString
  updatedAt: ISODateTimeString
  roles: UserAdminRole[]
}

export interface CurrentTenantAdminResponse {
  id: TenantId
  name: string
  status: TenantStatus
}

export interface ListUsersParams {
  pageNum?: number
  pageSize?: number
  status?: UserStatus
  q?: string
}

export interface CreateUserAdminUserRequest {
  email: string
  displayName: string
  password: string
  roleIds?: Array<RoleId | string>
}

export interface UpdateUserAdminUserRequest {
  displayName?: string
}

export interface ReplaceUserRolesRequest {
  roleIds: Array<RoleId | string>
}

export interface UpdateCurrentTenantRequest {
  name: string
}

export interface CreateUserAdminRoleRequest {
  code: string
  name: string
  description: string
  status?: UserAdminRoleStatus
}

export interface UpdateUserAdminRoleRequest {
  name?: string
  description?: string
  status?: UserAdminRoleStatus
}

export interface ReplaceRolePermissionsRequest {
  permissionIds: Array<PermissionKey | string>
}

export type UserAdminPage = ApiPage<UserAdminUser>
