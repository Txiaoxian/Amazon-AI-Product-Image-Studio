import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { ApiClientError } from '../api/client'
import type { ModelApi } from '../api/models'
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

const customRole: UserAdminRole = {
  ...sellerRole,
  id: 'role_catalog_manager' as UserAdminRole['id'],
  code: 'catalog-manager',
  name: 'Catalog Manager',
  description: 'Manage catalog workflow',
  permissions: [],
}

const currentTenant = {
  id: 'tenant_1',
  name: 'Studio Tenant',
  status: 'ACTIVE' as const,
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
    getCurrentTenant: vi.fn().mockResolvedValue(currentTenant),
    updateCurrentTenant: vi.fn().mockImplementation(async (request) => ({
      ...currentTenant,
      name: request.name,
    })),
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
    getUserModelAccess: vi.fn().mockResolvedValue({ userId: 'user_seller', modelIds: ['model_one'] }),
    replaceUserModelAccess: vi.fn().mockImplementation(async (userId, request) => ({
      userId,
      modelIds: request.modelIds,
    })),
    listRoles: vi.fn().mockResolvedValue([sellerRole, viewerRole]),
    listPermissions: vi.fn().mockResolvedValue(sellerRole.permissions ?? []),
    createRole: vi.fn().mockImplementation(async (request) => ({
      ...customRole,
      ...request,
    })),
    updateRole: vi.fn().mockImplementation(async (_roleId, request) => ({
      ...customRole,
      ...request,
    })),
    deleteRole: vi.fn().mockResolvedValue(undefined),
    replaceRolePermissions: vi.fn().mockImplementation(async (_roleId, request) => ({
      ...customRole,
      permissions: (sellerRole.permissions ?? []).filter((permission) => request.permissionIds.includes(permission.id)),
    })),
    ...overrides,
  }
}

const accessModels = [
  {
    id: 'model_one',
    tenantId: 'tenant_1',
    providerId: 'provider_a',
    providerName: 'Euzhi 中转站',
    modelName: 'gpt-image-2',
    displayName: 'GPT Image 2',
    supportsGenerate: true,
    supportsEdit: true,
    supportsMultiReference: true,
    supportsN: true,
    maxOutputCount: 4,
    supportedSizes: ['1024x1024'],
    supportedQualities: ['high'],
    supportedOutputFormats: ['png'],
    pricing: { currency: 'USD', unitPrices: {} },
    status: 'ENABLED',
    createdAt: '2026-05-21T00:00:00Z',
    updatedAt: '2026-05-21T00:00:00Z',
  },
  {
    id: 'model_two',
    tenantId: 'tenant_1',
    providerId: 'provider_a',
    providerName: 'Euzhi 中转站',
    modelName: 'gpt-image-2-low',
    displayName: 'GPT Image 2 快速版',
    supportsGenerate: true,
    supportsEdit: false,
    supportsMultiReference: false,
    supportsN: false,
    maxOutputCount: 1,
    supportedSizes: ['1024x1024'],
    supportedQualities: ['low'],
    supportedOutputFormats: ['png'],
    pricing: { currency: 'USD', unitPrices: {} },
    status: 'ENABLED',
    createdAt: '2026-05-21T00:00:00Z',
    updatedAt: '2026-05-21T00:00:00Z',
  },
] as const

function createMockModelApi(): ModelApi {
  return {
    list: vi.fn().mockResolvedValue(page([...accessModels], 100)),
    listEnabledCapabilities: vi.fn().mockResolvedValue([...accessModels]),
    create: vi.fn(),
    get: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    enable: vi.fn(),
    disable: vi.fn(),
  } as unknown as ModelApi
}

function renderPanel(props: Partial<Parameters<typeof UserRoleAdminPanel>[0]> = {}) {
  const api = props.userAdminApi ?? createMockUserAdminApi()
  render(
    <UserRoleAdminPanel
      canCreateUsers={false}
      canDisableUsers={false}
      canManageRoles={false}
      canManageTenant={false}
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

  it('shows Simplified Chinese permission names and descriptions while retaining the permission code', async () => {
    renderPanel({
      canReadRoles: true,
      userAdminApi: createMockUserAdminApi({
        listRoles: vi.fn().mockResolvedValue([sellerRole]),
        listPermissions: vi.fn().mockResolvedValue(sellerRole.permissions ?? []),
      }),
    })

    expect((await screen.findAllByText('查看用户')).length).toBeGreaterThan(0)
    expect(screen.getByText('查看当前租户的用户列表与用户详情。')).toBeInTheDocument()
    expect(screen.getAllByText('user:read').length).toBeGreaterThan(0)
  })

  it('lets an administrator assign models grouped by provider to an ordinary user', async () => {
    const user = userEvent.setup()
    const userApi = createMockUserAdminApi()
    const modelApi = createMockModelApi()
    renderPanel({
      canManageModelAccess: true,
      canReadUsers: true,
      modelApi,
      userAdminApi: userApi,
    })

    await user.click(await screen.findByRole('button', { name: '分配可用模型 seller@example.com' }))

    expect(await screen.findByRole('dialog', { name: '分配可用模型' })).toBeInTheDocument()
    expect(await screen.findByText('Euzhi 中转站')).toBeInTheDocument()
    expect(screen.getByText('GPT Image 2')).toBeInTheDocument()
    expect(screen.getByText('GPT Image 2 快速版')).toBeInTheDocument()
    expect(userApi.getUserModelAccess).toHaveBeenCalledWith('user_seller')

    await user.click(screen.getByRole('checkbox', { name: 'GPT Image 2 快速版 Euzhi 中转站' }))
    await user.click(screen.getByRole('button', { name: '保存可用模型' }))

    await waitFor(() => expect(userApi.replaceUserModelAccess).toHaveBeenCalledWith(
      'user_seller',
      { modelIds: ['model_one', 'model_two'] },
      'csrf_from_me',
    ))
    expect(await screen.findByText('用户可用模型已更新。')).toBeInTheDocument()
  })

  it('does not show model access management to an ordinary user', async () => {
    renderPanel({ canManageModelAccess: false, canReadUsers: true })

    expect(await screen.findByText('seller@example.com')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '分配可用模型 seller@example.com' })).not.toBeInTheDocument()
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

    expect(await screen.findByRole('button', { name: '打开账户菜单' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: '用户/角色管理' })).not.toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url)).some((url) => ['/api/v1/users', '/api/v1/roles', '/api/v1/permissions'].includes(url))).toBe(false)
  })

  it('App does not expose a dead console entry when role viewing is the only identity permission', async () => {
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
      return errorResponse(404, 'NOT_FOUND', `Unexpected ${url}`)
    })
    vi.stubGlobal('fetch', fetchImpl)

    render(<App />)

    expect(await screen.findByRole('button', { name: '打开账户菜单' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: '管理控制台' })).not.toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).not.toContain('/api/v1/roles')
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).not.toContain('/api/v1/permissions')
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).not.toContain('/api/v1/users')
  })

  it('App exposes the unified console entry to tenant admins with system settings permission', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          permissions: ['system:settings:manage'],
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

    const navigation = await screen.findByRole('navigation', { name: '工作区导航' })
    expect(within(navigation).getByRole('button', { name: '设置' })).toBeInTheDocument()
    expect(within(navigation).queryByRole('button', { name: '数据看板' })).not.toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: '用户/角色管理' })).not.toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).not.toContain('/api/v1/tenants/current')
  })

  it('App exposes the unified console entry to tenant admins with user read permission', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)
      if (url === '/api/v1/me') {
        return successResponse({ ...baseSession, permissions: ['user:read'] })
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

    const navigation = await screen.findByRole('navigation', { name: '工作区导航' })
    expect(within(navigation).getByRole('button', { name: '数据看板' })).toBeInTheDocument()
    expect(within(navigation).queryByRole('button', { name: '设置' })).not.toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: '用户/角色管理' })).not.toBeInTheDocument()
  })

  it('App does not treat system settings permission alone as tenant-admin identity access', async () => {
    const fetchImpl = vi.fn<typeof fetch>(async (input) => {
      const url = String(input)

      if (url === '/api/v1/me') {
        return successResponse({
          ...baseSession,
          roles: [{ id: 'role_seller', code: 'seller', name: 'Seller' }],
          permissions: ['system:settings:manage'],
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

    expect(await screen.findByRole('button', { name: '打开账户菜单' })).toBeInTheDocument()
    expect(screen.queryByRole('menuitem', { name: '管理控制台' })).not.toBeInTheDocument()
    expect(fetchImpl.mock.calls.map(([url]) => String(url))).not.toContain('/api/v1/tenants/current')
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
    expect(await screen.findByRole('dialog', { name: '编辑用户' })).toBeInTheDocument()
    await browserUser.clear(screen.getByLabelText(/新的显示名 seller@example.com/))
    await browserUser.type(screen.getByLabelText(/新的显示名 seller@example.com/), 'Seller Renamed')
    await browserUser.click(screen.getByRole('button', { name: '保存' }))
    await waitFor(() => expect(api.updateUser).toHaveBeenCalledWith('user_seller', { displayName: 'Seller Renamed' }, 'csrf_from_me'))

    await browserUser.click(screen.getByRole('button', { name: /分配角色 seller@example.com/ }))
    expect(await screen.findByRole('dialog', { name: '分配用户角色' })).toBeInTheDocument()
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
        canManageTenant={false}
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
        canManageTenant={false}
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

  it('loads the current tenant for the identity view but hides tenant editing without tenant admin settings permission', async () => {
    const api = renderPanel({ canReadUsers: true })

    expect(await screen.findByText('Studio Tenant')).toBeInTheDocument()
    expect(api.getCurrentTenant).toHaveBeenCalledTimes(1)
    expect(screen.queryByRole('button', { name: '编辑当前租户名称' })).not.toBeInTheDocument()
  })

  it('updates the current tenant name only for allowed tenant admins and does not write browser storage', async () => {
    const browserUser = userEvent.setup()
    const storageWrite = vi.spyOn(Storage.prototype, 'setItem')
    const indexedDbOpen = vi.spyOn(indexedDB, 'open')
    const api = renderPanel({ canManageTenant: true, canReadUsers: true })

    await screen.findByText('Studio Tenant')
    await browserUser.click(screen.getByRole('button', { name: '编辑当前租户名称' }))
    await browserUser.clear(screen.getByLabelText('当前租户名称'))
    await browserUser.type(screen.getByLabelText('当前租户名称'), 'Studio Team')
    await browserUser.click(screen.getByRole('button', { name: '保存租户名称' }))

    await waitFor(() => expect(api.updateCurrentTenant).toHaveBeenCalledWith({ name: 'Studio Team' }, 'csrf_from_me'))
    expect(await screen.findByText('Studio Team')).toBeInTheDocument()
    expect(storageWrite).not.toHaveBeenCalled()
    expect(indexedDbOpen).not.toHaveBeenCalled()
  })

  it('keeps built-in roles read-only and supports the custom role lifecycle with permission replacement', async () => {
    const browserUser = userEvent.setup()
    const api = createMockUserAdminApi({
      listRoles: vi.fn().mockResolvedValue([sellerRole, viewerRole, customRole]),
    })
    renderPanel({ canManageRoles: true, canReadRoles: true, userAdminApi: api })

    await browserUser.click(await screen.findByRole('button', { name: '角色与权限' }))
    expect(await screen.findByText('Catalog Manager')).toBeInTheDocument()
    expect(screen.getAllByText('系统内置，只读')).toHaveLength(2)
    expect(screen.queryByRole('button', { name: '编辑角色 Seller' })).not.toBeInTheDocument()

    await browserUser.click(screen.getByRole('button', { name: '编辑角色 Catalog Manager' }))
    expect(await screen.findByRole('dialog', { name: '编辑角色' })).toBeInTheDocument()
    await browserUser.clear(screen.getByLabelText('编辑角色名称'))
    await browserUser.type(screen.getByLabelText('编辑角色名称'), 'Catalog Lead')
    await browserUser.selectOptions(screen.getByLabelText('编辑角色状态'), 'DISABLED')
    await browserUser.click(screen.getByRole('button', { name: '保存角色' }))
    await waitFor(() => expect(api.updateRole).toHaveBeenCalledWith(
      'role_catalog_manager',
      expect.objectContaining({ name: 'Catalog Lead', status: 'DISABLED' }),
      'csrf_from_me',
    ))

    await browserUser.click(screen.getByRole('button', { name: '配置角色权限 Catalog Manager' }))
    expect(await screen.findByRole('dialog', { name: '配置角色权限' })).toBeInTheDocument()
    await browserUser.click(screen.getByLabelText('角色权限 user:read'))
    await browserUser.click(screen.getByRole('button', { name: '保存权限' }))
    await waitFor(() => expect(api.replaceRolePermissions).toHaveBeenCalledWith(
      'role_catalog_manager',
      { permissionIds: ['permission_user_read'] },
      'csrf_from_me',
    ))

    await browserUser.click(screen.getByRole('button', { name: '删除角色 Catalog Manager' }))
    expect(screen.getByText('确认删除此自定义角色？')).toBeInTheDocument()
    await browserUser.click(screen.getByRole('button', { name: '确认删除角色 Catalog Manager' }))
    await waitFor(() => expect(api.deleteRole).toHaveBeenCalledWith('role_catalog_manager', 'csrf_from_me'))
  })

  it('creates a custom role and rejects new mutations without an in-memory CSRF token', async () => {
    const browserUser = userEvent.setup()
    const api = createMockUserAdminApi({
      listRoles: vi.fn().mockResolvedValue([sellerRole]),
    })
    renderPanel({ canManageRoles: true, canReadRoles: true, csrfToken: undefined, userAdminApi: api })

    await browserUser.click(await screen.findByRole('button', { name: '角色与权限' }))
    await browserUser.type(screen.getByLabelText('角色代码'), 'catalog-manager')
    await browserUser.type(screen.getByLabelText('角色名称'), 'Catalog Manager')
    await browserUser.type(screen.getByLabelText('角色说明'), 'Manage catalog workflow')
    await browserUser.click(screen.getByRole('button', { name: '创建角色' }))

    expect(await screen.findByText('登录状态缺少 CSRF 凭据，请重新登录。')).toBeInTheDocument()
    expect(api.createRole).not.toHaveBeenCalled()
  })

  it('creates a custom role with transient form state and refreshes the role view', async () => {
    const browserUser = userEvent.setup()
    const api = createMockUserAdminApi({
      listRoles: vi.fn().mockResolvedValue([sellerRole]),
    })
    renderPanel({ canManageRoles: true, canReadRoles: true, userAdminApi: api })

    await browserUser.click(await screen.findByRole('button', { name: '角色与权限' }))
    await browserUser.type(screen.getByLabelText('角色代码'), 'catalog-manager')
    await browserUser.type(screen.getByLabelText('角色名称'), 'Catalog Manager')
    await browserUser.type(screen.getByLabelText('角色说明'), 'Manage catalog workflow')
    await browserUser.click(screen.getByRole('button', { name: '创建角色' }))

    await waitFor(() => expect(api.createRole).toHaveBeenCalledWith(
      {
        code: 'catalog-manager',
        name: 'Catalog Manager',
        description: 'Manage catalog workflow',
      },
      'csrf_from_me',
    ))
    await waitFor(() => expect(api.listRoles).toHaveBeenCalledTimes(2))
    expect(screen.getByLabelText('角色代码')).toHaveValue('')
    expect(screen.getByLabelText('角色名称')).toHaveValue('')
    expect(screen.getByLabelText('角色说明')).toHaveValue('')
  })

  it('disables custom role mutations while a role create request is pending', async () => {
    const browserUser = userEvent.setup()
    let resolveCreateRole: ((value: UserAdminRole) => void) | undefined
    const api = createMockUserAdminApi({
      listRoles: vi.fn().mockResolvedValue([sellerRole, viewerRole, customRole]),
      createRole: vi.fn().mockReturnValue(new Promise((resolve) => {
        resolveCreateRole = resolve
      })),
    })
    renderPanel({ canManageRoles: true, canReadRoles: true, userAdminApi: api })

    await browserUser.click(await screen.findByRole('button', { name: '角色与权限' }))
    await browserUser.type(screen.getByLabelText('角色代码'), 'catalog-manager')
    await browserUser.type(screen.getByLabelText('角色名称'), 'Catalog Manager')
    await browserUser.click(screen.getByRole('button', { name: '创建角色' }))

    expect(screen.getByRole('button', { name: '创建角色' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '编辑角色 Catalog Manager' })).toBeDisabled()
    expect(api.createRole).toHaveBeenCalledTimes(1)

    resolveCreateRole?.(customRole)
    await waitFor(() => expect(screen.getByRole('button', { name: '创建角色' })).toBeEnabled())
    expect(api.createRole).toHaveBeenCalledTimes(1)
  })

  it('drops a late current-tenant response after the panel closes', async () => {
    let resolveTenant: ((value: typeof currentTenant) => void) | undefined
    const api = createMockUserAdminApi({
      getCurrentTenant: vi.fn().mockReturnValue(new Promise((resolve) => {
        resolveTenant = resolve
      })),
    })
    const { rerender } = render(
      <UserRoleAdminPanel
        canCreateUsers={false}
        canDisableUsers={false}
        canManageRoles={false}
        canManageTenant={false}
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
        canManageTenant={false}
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
    resolveTenant?.(currentTenant)
    await waitFor(() => expect(screen.queryByText('Studio Tenant')).not.toBeInTheDocument())
  })
})
