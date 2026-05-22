import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { ApiClientError } from '../api/client'
import type { UserAdminApi } from '../api/userAdmin'
import { UserRoleAdminPanel } from '../components/admin/UserRoleAdminPanel'
import type { UserAdminRole, UserAdminUser } from '../types/userAdmin'

const baseSession = {
  user: {
    id: 'user_admin',
    email: 'admin@example.com',
    displayName: 'Admin User',
    status: 'ACTIVE',
    createdAt: '2026-05-21T00:00:00Z',
    updatedAt: '2026-05-21T00:00:00Z',
  },
  tenant: {
    id: 'tenant_1',
    name: 'Studio Tenant',
    status: 'ACTIVE',
  },
  roles: [
    {
      id: 'role_admin',
      code: 'admin',
      name: '管理员',
    },
  ],
  csrfToken: 'csrf_from_me',
}

const sellerRole: UserAdminRole = {
  id: 'role_seller' as UserAdminRole['id'],
  tenantId: 'tenant_1' as UserAdminRole['tenantId'],
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

const viewerRole: UserAdminRole = {
  ...sellerRole,
  id: 'role_viewer' as UserAdminRole['id'],
  code: 'viewer',
  name: 'Viewer',
  description: 'Viewer role',
}

const currentUser: UserAdminUser = {
  id: 'user_admin' as UserAdminUser['id'],
  tenantId: 'tenant_1' as UserAdminUser['tenantId'],
  email: 'admin@example.com',
  displayName: 'Admin User',
  status: 'ACTIVE',
  lastLoginAt: null,
  createdAt: '2026-05-21T00:00:00Z',
  updatedAt: '2026-05-21T00:00:00Z',
  roles: [sellerRole],
}

const managedUser: UserAdminUser = {
  id: 'user_seller' as UserAdminUser['id'],
  tenantId: 'tenant_1' as UserAdminUser['tenantId'],
  email: 'seller@example.com',
  displayName: 'Seller User',
  status: 'ACTIVE',
  lastLoginAt: '2026-05-20T00:00:00Z',
  createdAt: '2026-05-21T00:00:00Z',
  updatedAt: '2026-05-21T00:00:00Z',
  roles: [sellerRole],
}

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_user_role_admin',
    }),
    { status },
  )
}

function errorResponse(status: number, code: string, message: string): Response {
  return new Response(
    JSON.stringify({
      error: {
        code,
        message,
      },
      requestId: 'req_user_role_admin_error',
    }),
    { status },
  )
}

function page(records: unknown[], pageSize = 10) {
  return {
    records,
    total: records.length,
    pageNum: 1,
    pageSize,
  }
}

function createMockUserAdminApi(overrides: Partial<UserAdminApi> = {}): UserAdminApi {
  return {
    listUsers: vi.fn().mockResolvedValue(page([currentUser, managedUser])),
    createUser: vi.fn().mockImplementation(async (request) => ({
      ...managedUser,
      id: 'user_created',
      email: request.email,
      displayName: request.displayName,
      roles: request.roleIds?.length ? [sellerRole] : [],
    })),
    getUser: vi.fn().mockResolvedValue(managedUser),
    updateUser: vi.fn().mockImplementation(async (_userId, request) => ({
      ...managedUser,
      displayName: request.displayName ?? managedUser.displayName,
    })),
    disableUser: vi.fn().mockResolvedValue({ ...managedUser, status: 'DISABLED' }),
    enableUser: vi.fn().mockResolvedValue({ ...managedUser, status: 'ACTIVE' }),
    replaceUserRoles: vi.fn().mockImplementation(async (_userId, request) => ({
      ...managedUser,
      roles: [sellerRole, viewerRole].filter((role) => request.roleIds.includes(role.id)),
    })),
    listRoles: vi.fn().mockResolvedValue([sellerRole, viewerRole]),
    listPermissions: vi.fn().mockResolvedValue(sellerRole.permissions ?? []),
    ...overrides,
  }
}

function renderPanel(props: Partial<Parameters<typeof UserRoleAdminPanel>[0]> = {}) {
  const api = props.userAdminApi ?? createMockUserAdminApi()
  render(
    <UserRoleAdminPanel
      canCreateUsers={false}
      canDisableUsers={false}
      canManageRoles={false}
      canReadRoles={false}
      canReadUsers={false}
      canUpdateUsers={false}
      csrfToken="csrf_from_me"
      currentUserId="user_admin"
      isOpen
      onClose={vi.fn()}
      userAdminApi={api}
      {...props}
    />,
  )
  return api
}

function getBrowserStorage(prefix: string): Storage {
  return Reflect.get(globalThis, `${prefix}Storage`) as Storage
}

describe('user and role admin UI', () => {
  afterEach(() => {
    cleanup()
    getBrowserStorage('local').clear()
    getBrowserStorage('session').clear()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('App hides the entry for users without identity permissions and does not call user admin APIs', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['project:read'],
        })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([]))
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '用户/角色管理' })).not.toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url)).some((url) => ['/api/v1/users', '/api/v1/roles', '/api/v1/permissions'].includes(url))).toBe(false)
  })

  it('App allows role:read users to view roles without calling /users', async () => {
    const browserUser = userEvent.setup()
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['role:read'],
        })
      }
      if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/models?enabled=true&capability=generate&pageNum=1&pageSize=100') {
        return successResponse(page([]))
      }
      if (url === '/api/v1/roles') {
        return successResponse([sellerRole])
      }
      if (url === '/api/v1/permissions') {
        return successResponse(sellerRole.permissions)
      }

      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    await browserUser.click(await screen.findByRole('button', { name: '用户/角色管理' }))
    expect(await screen.findByRole('heading', { name: '用户与角色管理' })).toBeInTheDocument()
    expect(await screen.findByText('Seller')).toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).toContain('/api/v1/roles')
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).toContain('/api/v1/permissions')
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).not.toContain('/api/v1/users')
  })

  it('only user:read users can view the user list without create, disable, or role assignment controls', async () => {
    const api = renderPanel({ canReadUsers: true })

    expect(await screen.findByText('Seller User')).toBeInTheDocument()
    expect(api.listUsers).toHaveBeenCalled()
    expect(api.listRoles).not.toHaveBeenCalled()
    expect(api.listPermissions).not.toHaveBeenCalled()
    expect(screen.queryByRole('button', { name: '创建用户' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /禁用用户 seller@example.com/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /分配角色 seller@example.com/ })).not.toBeInTheDocument()
  })

  it('creates a user without roleIds when role:manage is absent and clears the password input without storage writes', async () => {
    const browserUser = userEvent.setup()
    const localSetItem = vi.spyOn(Storage.prototype, 'setItem')
    const indexedDbOpen = vi.spyOn(indexedDB, 'open')
    const api = renderPanel({ canCreateUsers: true })
    const password = 'create-only-password-123'

    await browserUser.type(screen.getByLabelText('用户邮箱'), 'new@example.com')
    await browserUser.type(screen.getByLabelText('显示名'), 'New User')
    await browserUser.type(screen.getByLabelText('初始密码'), password)
    await browserUser.click(screen.getByRole('button', { name: '创建用户' }))

    await waitFor(() => expect(api.createUser).toHaveBeenCalled())
    expect(api.createUser).toHaveBeenCalledWith(
      {
        email: 'new@example.com',
        displayName: 'New User',
        password,
      },
      'csrf_from_me',
    )
    expect(screen.getByLabelText('初始密码')).toHaveValue('')
    expect(screen.queryByDisplayValue(password)).not.toBeInTheDocument()
    expect(localSetItem).not.toHaveBeenCalled()
    expect(indexedDbOpen).not.toHaveBeenCalled()
    expect(getBrowserStorage('local').getItem(password)).toBeNull()
    expect(getBrowserStorage('session').getItem(password)).toBeNull()
  })

  it('creates a user with roleIds when role:manage is present', async () => {
    const browserUser = userEvent.setup()
    const api = renderPanel({ canCreateUsers: true, canManageRoles: true, canReadRoles: true })

    await screen.findByLabelText(/Seller/)
    await browserUser.type(screen.getByLabelText('用户邮箱'), 'new@example.com')
    await browserUser.type(screen.getByLabelText('显示名'), 'New User')
    await browserUser.type(screen.getByLabelText('初始密码'), 'create-only-password-123')
    await browserUser.click(screen.getByLabelText(/Seller/))
    await browserUser.click(screen.getByRole('button', { name: '创建用户' }))

    await waitFor(() => expect(api.createUser).toHaveBeenCalled())
    expect(api.createUser).toHaveBeenCalledWith(
      expect.objectContaining({
        roleIds: ['role_seller'],
      }),
      'csrf_from_me',
    )
  })

  it('updates displayName with user:update and replaces roles with role:manage', async () => {
    const browserUser = userEvent.setup()
    const api = renderPanel({ canManageRoles: true, canReadRoles: true, canReadUsers: true, canUpdateUsers: true })

    await screen.findByText('Seller User')
    await browserUser.click(screen.getByRole('button', { name: /编辑显示名 seller@example.com/ }))
    await browserUser.clear(screen.getByLabelText(/新的显示名 seller@example.com/))
    await browserUser.type(screen.getByLabelText(/新的显示名 seller@example.com/), 'Seller Renamed')
    await browserUser.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(api.updateUser).toHaveBeenCalledWith('user_seller', { displayName: 'Seller Renamed' }, 'csrf_from_me'))

    await browserUser.click(screen.getByRole('button', { name: /分配角色 seller@example.com/ }))
    await browserUser.click(screen.getAllByLabelText(/Viewer/).at(-1) as HTMLElement)
    await browserUser.click(screen.getByRole('button', { name: '保存角色' }))
    await waitFor(() => expect(api.replaceUserRoles).toHaveBeenCalledWith('user_seller', { roleIds: ['role_seller', 'role_viewer'] }, 'csrf_from_me'))
  })

  it('allows user:update displayName editing without disable or role assignment controls', async () => {
    const browserUser = userEvent.setup()
    const api = renderPanel({ canReadUsers: true, canUpdateUsers: true })

    await screen.findByText('Seller User')
    expect(screen.queryByRole('button', { name: /禁用用户 seller@example.com/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /分配角色 seller@example.com/ })).not.toBeInTheDocument()

    await browserUser.click(screen.getByRole('button', { name: /编辑显示名 seller@example.com/ }))
    await browserUser.clear(screen.getByLabelText(/新的显示名 seller@example.com/))
    await browserUser.type(screen.getByLabelText(/新的显示名 seller@example.com/), 'Seller Update Only')
    await browserUser.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => expect(api.updateUser).toHaveBeenCalledWith('user_seller', { displayName: 'Seller Update Only' }, 'csrf_from_me'))
  })

  it('uses disable and enable endpoints and disables the current user action', async () => {
    const browserUser = userEvent.setup()
    const api = createMockUserAdminApi({
      listUsers: vi.fn().mockResolvedValue(page([
        currentUser,
        managedUser,
        { ...managedUser, id: 'user_disabled', email: 'disabled@example.com', displayName: 'Disabled User', status: 'DISABLED' },
      ])),
    })
    renderPanel({ canDisableUsers: true, canReadUsers: true, userAdminApi: api })

    expect(await screen.findByText('Seller User')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /禁用用户 admin@example.com/ })).toBeDisabled()

    await browserUser.click(screen.getByRole('button', { name: /禁用用户 seller@example.com/ }))
    await waitFor(() => expect(api.disableUser).toHaveBeenCalledWith('user_seller', 'csrf_from_me'))

    await browserUser.click(screen.getByRole('button', { name: /启用用户 disabled@example.com/ }))
    await waitFor(() => expect(api.enableUser).toHaveBeenCalledWith('user_disabled', 'csrf_from_me'))
  })

  it('does not call /roles or /permissions when role:read is absent', async () => {
    const api = renderPanel({ canCreateUsers: true, canManageRoles: true })

    await screen.findByText('没有可分配角色，或当前账号没有 role:read 权限。')
    expect(api.listRoles).not.toHaveBeenCalled()
    expect(api.listPermissions).not.toHaveBeenCalled()
  })

  it('shows backend 403, 409, and 422 errors without treating them as success', async () => {
    const browserUser = userEvent.setup()
    const api = createMockUserAdminApi({
      createUser: vi
        .fn()
        .mockRejectedValueOnce(new ApiClientError({ status: 422, code: 'VALIDATION_ERROR', message: 'Password is too weak.' })),
      disableUser: vi
        .fn()
        .mockRejectedValueOnce(new ApiClientError({ status: 409, code: 'LAST_ADMIN', message: 'Cannot disable last admin.' })),
      replaceUserRoles: vi
        .fn()
        .mockRejectedValueOnce(new ApiClientError({ status: 403, code: 'FORBIDDEN', message: 'Forbidden.' })),
    })
    renderPanel({ canCreateUsers: true, canDisableUsers: true, canManageRoles: true, canReadRoles: true, canReadUsers: true, userAdminApi: api })

    await screen.findByText('Seller User')
    await browserUser.type(screen.getByLabelText('用户邮箱'), 'new@example.com')
    await browserUser.type(screen.getByLabelText('显示名'), 'New User')
    await browserUser.type(screen.getByLabelText('初始密码'), 'weak')
    await browserUser.click(screen.getByRole('button', { name: '创建用户' }))
    expect(await screen.findByText('表单内容未通过校验：Password is too weak.')).toBeInTheDocument()

    await browserUser.click(screen.getByRole('button', { name: /禁用用户 seller@example.com/ }))
    expect(await screen.findByText('操作冲突：Cannot disable last admin.')).toBeInTheDocument()

    await browserUser.click(screen.getByRole('button', { name: /分配角色 seller@example.com/ }))
    await browserUser.click(screen.getAllByLabelText(/Viewer/).at(-1) as HTMLElement)
    await browserUser.click(screen.getByRole('button', { name: '保存角色' }))
    expect(await screen.findByText('当前账号没有此管理权限。')).toBeInTheDocument()
  })

  it('does not render unsafe response fields or write stale responses after close', async () => {
    let resolveUsers: ((value: ReturnType<typeof page>) => void) | undefined
    const unsafeUser = {
      ...managedUser,
      passwordHash: 'hash_should_not_render',
      authorization: 'Bearer should-not-render',
    }
    const api = createMockUserAdminApi({
      listUsers: vi.fn().mockReturnValue(new Promise((resolve) => {
        resolveUsers = resolve
      })),
    })
    const { rerender } = render(
      <UserRoleAdminPanel
        canCreateUsers={false}
        canDisableUsers={false}
        canManageRoles={false}
        canReadRoles={false}
        canReadUsers
        canUpdateUsers={false}
        csrfToken="csrf_from_me"
        currentUserId="user_admin"
        isOpen
        onClose={vi.fn()}
        userAdminApi={api}
      />,
    )

    rerender(
      <UserRoleAdminPanel
        canCreateUsers={false}
        canDisableUsers={false}
        canManageRoles={false}
        canReadRoles={false}
        canReadUsers
        canUpdateUsers={false}
        csrfToken="csrf_from_me"
        currentUserId="user_admin"
        isOpen={false}
        onClose={vi.fn()}
        userAdminApi={api}
      />,
    )
    resolveUsers?.(page([unsafeUser]))
    await waitFor(() => expect(screen.queryByText('Seller User')).not.toBeInTheDocument())
    expect(screen.queryByText('hash_should_not_render')).not.toBeInTheDocument()
    expect(screen.queryByText('Bearer should-not-render')).not.toBeInTheDocument()
  })
})
