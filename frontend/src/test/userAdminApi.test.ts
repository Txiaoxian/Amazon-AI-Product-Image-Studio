import { describe, expect, it, vi } from 'vitest'
import { createApiClient } from '../api/client'
import { createUserAdminApi } from '../api/userAdmin'

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_user_admin_api',
    }),
    { status },
  )
}

const role = {
  id: 'role_seller',
  tenantId: 'tenant_1',
  code: 'seller',
  name: 'Seller',
  description: 'Seller role',
  status: 'ACTIVE',
  permissions: [
    {
      id: 'permission_user_read',
      code: 'user:read',
      name: 'Read users',
      description: 'Read tenant users',
    },
  ],
  createdAt: '2026-05-21T00:00:00Z',
  updatedAt: '2026-05-21T00:00:00Z',
}

const userRecord = {
  id: 'user_2',
  tenantId: 'tenant_1',
  email: 'seller@example.com',
  displayName: 'Seller User',
  status: 'ACTIVE',
  lastLoginAt: null,
  createdAt: '2026-05-21T00:00:00Z',
  updatedAt: '2026-05-21T00:00:00Z',
  roles: [role],
}

describe('user admin API wrapper', () => {
  it('covers user, role, and permission endpoints with authenticated requests and CSRF writes', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse({
          records: [userRecord],
          total: 1,
          pageNum: 1,
          pageSize: 20,
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ ...userRecord, id: 'user_3' }, 201))
      .mockResolvedValueOnce(jsonResponse(userRecord))
      .mockResolvedValueOnce(jsonResponse({ ...userRecord, displayName: 'Seller Renamed' }))
      .mockResolvedValueOnce(jsonResponse({ ...userRecord, status: 'DISABLED' }))
      .mockResolvedValueOnce(jsonResponse({ ...userRecord, status: 'ACTIVE' }))
      .mockResolvedValueOnce(jsonResponse({ ...userRecord, roles: [] }))
      .mockResolvedValueOnce(jsonResponse([role]))
      .mockResolvedValueOnce(jsonResponse(role.permissions))
    const api = createUserAdminApi(createApiClient({ fetchImpl }))

    await expect(api.listUsers({ pageNum: 1, pageSize: 20, status: 'ACTIVE', q: 'seller' })).resolves.toMatchObject({
      records: [userRecord],
      total: 1,
    })
    await api.createUser(
      {
        email: 'new@example.com',
        displayName: 'New User',
        password: 'create-only-password',
        roleIds: ['role_seller'],
      },
      'csrf_memory_only',
    )
    await api.getUser('user_2')
    await api.updateUser('user_2', { displayName: 'Seller Renamed' }, 'csrf_memory_only')
    await api.disableUser('user_2', 'csrf_memory_only')
    await api.enableUser('user_2', 'csrf_memory_only')
    await api.replaceUserRoles('user_2', { roleIds: [] }, 'csrf_memory_only')
    await api.listRoles()
    await api.listPermissions()

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/users?pageNum=1&pageSize=20&status=ACTIVE&q=seller',
      '/api/v1/users',
      '/api/v1/users/user_2',
      '/api/v1/users/user_2',
      '/api/v1/users/user_2/disable',
      '/api/v1/users/user_2/enable',
      '/api/v1/users/user_2/roles',
      '/api/v1/roles',
      '/api/v1/permissions',
    ])
    expect(fetchImpl.mock.calls[0][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toEqual({
      email: 'new@example.com',
      displayName: 'New User',
      password: 'create-only-password',
      roleIds: ['role_seller'],
    })
    expect(JSON.parse(fetchImpl.mock.calls[3][1]?.body as string)).toEqual({ displayName: 'Seller Renamed' })
    expect(JSON.parse(fetchImpl.mock.calls[6][1]?.body as string)).toEqual({ roleIds: [] })
    for (const index of [1, 3, 4, 5, 6]) {
      expect((fetchImpl.mock.calls[index][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    }
  })

  it('covers current tenant and custom role writes with the frozen URL, method, body, and CSRF contract', async () => {
    const customRole = {
      ...role,
      id: 'role_catalog_manager',
      code: 'catalog-manager',
      name: 'Catalog Manager',
      description: 'Manage catalog workflow',
    }
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse({ id: 'tenant_1', name: 'Studio Tenant', status: 'ACTIVE' }))
      .mockResolvedValueOnce(jsonResponse({ id: 'tenant_1', name: 'Studio Team', status: 'ACTIVE' }))
      .mockResolvedValueOnce(jsonResponse(customRole, 201))
      .mockResolvedValueOnce(jsonResponse({ ...customRole, name: 'Catalog Lead', status: 'DISABLED' }))
      .mockResolvedValueOnce(jsonResponse({ ...customRole, permissions: role.permissions }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
    const api = createUserAdminApi(createApiClient({ fetchImpl }))

    await api.getCurrentTenant()
    await api.updateCurrentTenant({ name: 'Studio Team' }, 'csrf_memory_only')
    await api.createRole(
      {
        code: 'catalog-manager',
        name: 'Catalog Manager',
        description: 'Manage catalog workflow',
      },
      'csrf_memory_only',
    )
    await api.updateRole(
      'role_catalog_manager',
      {
        name: 'Catalog Lead',
        description: 'Manage catalog workflow',
        status: 'DISABLED',
      },
      'csrf_memory_only',
    )
    await api.replaceRolePermissions('role_catalog_manager', { permissionIds: ['permission_user_read'] }, 'csrf_memory_only')
    await api.deleteRole('role_catalog_manager', 'csrf_memory_only')

    expect(fetchImpl.mock.calls.map(([url, init]) => [url, init?.method])).toEqual([
      ['/api/v1/tenants/current', 'GET'],
      ['/api/v1/tenants/current', 'PATCH'],
      ['/api/v1/roles', 'POST'],
      ['/api/v1/roles/role_catalog_manager', 'PATCH'],
      ['/api/v1/roles/role_catalog_manager/permissions', 'PUT'],
      ['/api/v1/roles/role_catalog_manager', 'DELETE'],
    ])
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toEqual({ name: 'Studio Team' })
    expect(JSON.parse(fetchImpl.mock.calls[2][1]?.body as string)).toEqual({
      code: 'catalog-manager',
      name: 'Catalog Manager',
      description: 'Manage catalog workflow',
    })
    expect(JSON.parse(fetchImpl.mock.calls[3][1]?.body as string)).toEqual({
      name: 'Catalog Lead',
      description: 'Manage catalog workflow',
      status: 'DISABLED',
    })
    expect(JSON.parse(fetchImpl.mock.calls[4][1]?.body as string)).toEqual({ permissionIds: ['permission_user_read'] })
    for (const index of [1, 2, 3, 4, 5]) {
      expect((fetchImpl.mock.calls[index][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    }
  })
})
