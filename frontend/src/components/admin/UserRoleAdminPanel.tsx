import {
  AlertTriangle,
  CheckCircle2,
  Loader2,
  Layers3,
  Pencil,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { isApiClientError } from '../../api/client'
import { modelApi as defaultModelApi, type ModelApi } from '../../api/models'
import { userAdminApi as defaultUserAdminApi, type UserAdminApi } from '../../api/userAdmin'
import { permissionCopy } from '../../lib/permissionCatalog'
import { loadAllModelsForAccess } from '../../lib/userModelAccess'
import type { Model, UserId, UserStatus } from '../../types/platform'
import type {
  CurrentTenantAdminResponse,
  UserAdminPermission,
  UserAdminRole,
  UserAdminRoleStatus,
  UserAdminUser,
} from '../../types/userAdmin'
import { Button } from '../ui/Button'
import { EditorDrawer } from '../ui/EditorDrawer'
import { Modal } from '../ui/Modal'
import { UserModelAccessEditor } from './UserModelAccessEditor'

type IdentityAdminTab = 'users' | 'roles'

interface UserRoleAdminPanelProps {
  isOpen: boolean
  csrfToken?: string
  currentUserId: UserId | string
  canReadUsers: boolean
  canCreateUsers: boolean
  canUpdateUsers: boolean
  canDisableUsers: boolean
  canReadRoles: boolean
  canManageRoles: boolean
  canManageTenant: boolean
  canManageModelAccess?: boolean
  onClose: () => void
  modelApi?: ModelApi
  userAdminApi?: UserAdminApi
}

interface UserPageState {
  records: UserAdminUser[]
  total: number
  pageNum: number
  pageSize: number
}

interface CreateUserDraft {
  email: string
  displayName: string
  password: string
  roleIds: string[]
}

interface CreateRoleDraft {
  code: string
  name: string
  description: string
}

interface EditRoleDraft {
  name: string
  description: string
  status: UserAdminRoleStatus
}

const PAGE_SIZE = 10
const BUILT_IN_ROLE_CODES = new Set(['admin', 'seller', 'viewer'])

const userStatuses: Array<{ value: UserStatus | ''; label: string }> = [
  { value: '', label: '全部状态' },
  { value: 'ACTIVE', label: '启用' },
  { value: 'DISABLED', label: '禁用' },
]

export function UserRoleAdminPanel({
  isOpen,
  csrfToken,
  currentUserId,
  canReadUsers,
  canCreateUsers,
  canUpdateUsers,
  canDisableUsers,
  canReadRoles,
  canManageRoles,
  canManageTenant,
  canManageModelAccess = false,
  onClose,
  modelApi = defaultModelApi,
  userAdminApi = defaultUserAdminApi,
}: UserRoleAdminPanelProps) {
  const panelSeqRef = useRef(0)
  const usersRequestSeqRef = useRef(0)
  const rolesRequestSeqRef = useRef(0)
  const tenantRequestSeqRef = useRef(0)
  const modelAccessRequestSeqRef = useRef(0)
  const availableTabs = useMemo(
    () => getAvailableTabs({ canCreateUsers, canDisableUsers, canManageRoles, canReadRoles, canReadUsers, canUpdateUsers }),
    [canCreateUsers, canDisableUsers, canManageRoles, canReadRoles, canReadUsers, canUpdateUsers],
  )
  const [activeTab, setActiveTab] = useState<IdentityAdminTab>(availableTabs[0] ?? 'users')
  const [usersPage, setUsersPage] = useState<UserPageState>(() => emptyUserPage())
  const [roles, setRoles] = useState<UserAdminRole[]>([])
  const [permissions, setPermissions] = useState<UserAdminPermission[]>([])
  const [currentTenant, setCurrentTenant] = useState<CurrentTenantAdminResponse | null>(null)
  const [userPageNum, setUserPageNum] = useState(1)
  const [userStatus, setUserStatus] = useState<UserStatus | ''>('')
  const [userQuery, setUserQuery] = useState('')
  const [isLoadingUsers, setLoadingUsers] = useState(false)
  const [isLoadingRoles, setLoadingRoles] = useState(false)
  const [isLoadingTenant, setLoadingTenant] = useState(false)
  const [usersError, setUsersError] = useState<string | null>(null)
  const [rolesError, setRolesError] = useState<string | null>(null)
  const [tenantError, setTenantError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)
  const [createError, setCreateError] = useState<string | null>(null)
  const [createDraft, setCreateDraft] = useState<CreateUserDraft>(() => emptyCreateDraft())
  const [isCreating, setCreating] = useState(false)
  const [editingUserId, setEditingUserId] = useState<string | null>(null)
  const [editDisplayName, setEditDisplayName] = useState('')
  const [isUpdatingUser, setUpdatingUser] = useState(false)
  const [userActionId, setUserActionId] = useState<string | null>(null)
  const [roleEditUserId, setRoleEditUserId] = useState<string | null>(null)
  const [roleDraftIds, setRoleDraftIds] = useState<string[]>([])
  const [isSavingRoles, setSavingRoles] = useState(false)
  const [modelAccessUserId, setModelAccessUserId] = useState<string | null>(null)
  const [availableModels, setAvailableModels] = useState<Model[]>([])
  const [modelAccessDraftIds, setModelAccessDraftIds] = useState<string[]>([])
  const [isLoadingModelAccess, setLoadingModelAccess] = useState(false)
  const [isSavingModelAccess, setSavingModelAccess] = useState(false)
  const [modelAccessError, setModelAccessError] = useState<string | null>(null)
  const [isEditingTenant, setEditingTenant] = useState(false)
  const [tenantNameDraft, setTenantNameDraft] = useState('')
  const [isSavingTenant, setSavingTenant] = useState(false)
  const [createRoleDraft, setCreateRoleDraft] = useState<CreateRoleDraft>(() => emptyCreateRoleDraft())
  const [editingRoleId, setEditingRoleId] = useState<string | null>(null)
  const [editRoleDraft, setEditRoleDraft] = useState<EditRoleDraft | null>(null)
  const [permissionEditRoleId, setPermissionEditRoleId] = useState<string | null>(null)
  const [permissionDraftIds, setPermissionDraftIds] = useState<string[]>([])
  const [deleteConfirmRoleId, setDeleteConfirmRoleId] = useState<string | null>(null)
  const [roleMutation, setRoleMutation] = useState<string | null>(null)

  const resetTransientState = useCallback(() => {
    setNotice(null)
    setUsersError(null)
    setRolesError(null)
    setTenantError(null)
    setCreateError(null)
    setEditingUserId(null)
    setEditDisplayName('')
    setRoleEditUserId(null)
    setRoleDraftIds([])
    setModelAccessUserId(null)
    setAvailableModels([])
    setModelAccessDraftIds([])
    setLoadingModelAccess(false)
    setSavingModelAccess(false)
    setModelAccessError(null)
    setCurrentTenant(null)
    setEditingTenant(false)
    setTenantNameDraft('')
    setLoadingTenant(false)
    setCreateRoleDraft(emptyCreateRoleDraft())
    setEditingRoleId(null)
    setEditRoleDraft(null)
    setPermissionEditRoleId(null)
    setPermissionDraftIds([])
    setDeleteConfirmRoleId(null)
  }, [])

  useEffect(() => {
    if (!isOpen) {
      panelSeqRef.current += 1
      usersRequestSeqRef.current += 1
      rolesRequestSeqRef.current += 1
      tenantRequestSeqRef.current += 1
      modelAccessRequestSeqRef.current += 1
      resetTransientState()
      return
    }

    if (!availableTabs.includes(activeTab)) {
      setActiveTab(availableTabs[0] ?? 'users')
    }
  }, [activeTab, availableTabs, isOpen, resetTransientState])

  const loadUsers = useCallback(async () => {
    if (!isOpen || !canReadUsers) {
      return
    }

    const panelSeq = panelSeqRef.current
    const requestSeq = usersRequestSeqRef.current + 1
    usersRequestSeqRef.current = requestSeq
    setLoadingUsers(true)
    setUsersError(null)
    try {
      const page = await userAdminApi.listUsers({
        pageNum: userPageNum,
        pageSize: PAGE_SIZE,
        status: userStatus || undefined,
        q: userQuery.trim() || undefined,
      })
      if (usersRequestSeqRef.current !== requestSeq || panelSeqRef.current !== panelSeq || !isOpen) {
        return
      }
      setUsersPage({
        records: page.records,
        total: page.total,
        pageNum: page.pageNum,
        pageSize: page.pageSize,
      })
    } catch (error) {
      if (usersRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setUsersError(formatAdminError(error))
      }
    } finally {
      if (usersRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setLoadingUsers(false)
      }
    }
  }, [canReadUsers, isOpen, userAdminApi, userPageNum, userQuery, userStatus])

  const loadCurrentTenant = useCallback(async () => {
    if (!isOpen) {
      return
    }

    const panelSeq = panelSeqRef.current
    const requestSeq = tenantRequestSeqRef.current + 1
    tenantRequestSeqRef.current = requestSeq
    setLoadingTenant(true)
    setTenantError(null)
    try {
      const nextTenant = await userAdminApi.getCurrentTenant()
      if (tenantRequestSeqRef.current !== requestSeq || panelSeqRef.current !== panelSeq || !isOpen) {
        return
      }
      setCurrentTenant(nextTenant)
    } catch (error) {
      if (tenantRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setTenantError(formatAdminError(error))
      }
    } finally {
      if (tenantRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setLoadingTenant(false)
      }
    }
  }, [isOpen, userAdminApi])

  const loadRolesAndPermissions = useCallback(async () => {
    if (!isOpen || !canReadRoles) {
      return
    }

    const panelSeq = panelSeqRef.current
    const requestSeq = rolesRequestSeqRef.current + 1
    rolesRequestSeqRef.current = requestSeq
    setLoadingRoles(true)
    setRolesError(null)
    try {
      const [nextRoles, nextPermissions] = await Promise.all([
        userAdminApi.listRoles(),
        userAdminApi.listPermissions(),
      ])
      if (rolesRequestSeqRef.current !== requestSeq || panelSeqRef.current !== panelSeq || !isOpen) {
        return
      }
      setRoles(nextRoles)
      setPermissions(nextPermissions)
    } catch (error) {
      if (rolesRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setRolesError(formatAdminError(error))
      }
    } finally {
      if (rolesRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setLoadingRoles(false)
      }
    }
  }, [canReadRoles, isOpen, userAdminApi])

  useEffect(() => {
    if (!isOpen) {
      return
    }
    void loadCurrentTenant()
  }, [isOpen, loadCurrentTenant])

  useEffect(() => {
    if (!isOpen || activeTab !== 'users' || !canReadUsers) {
      return
    }
    void loadUsers()
  }, [activeTab, canReadUsers, isOpen, loadUsers])

  useEffect(() => {
    if (!isOpen || !canReadRoles) {
      return
    }
    void loadRolesAndPermissions()
  }, [canReadRoles, isOpen, loadRolesAndPermissions])

  const handleClose = () => {
    panelSeqRef.current += 1
    usersRequestSeqRef.current += 1
    rolesRequestSeqRef.current += 1
    tenantRequestSeqRef.current += 1
    modelAccessRequestSeqRef.current += 1
    setLoadingTenant(false)
    setCreateDraft(emptyCreateDraft())
    onClose()
  }

  const saveCurrentTenant = async () => {
    if (isSavingTenant) {
      return
    }
    if (!csrfToken || !canManageTenant) {
      setTenantError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    tenantRequestSeqRef.current += 1
    setLoadingTenant(false)
    setSavingTenant(true)
    setTenantError(null)
    setNotice(null)
    try {
      const updated = await userAdminApi.updateCurrentTenant({ name: tenantNameDraft.trim() }, csrfToken)
      setCurrentTenant(updated)
      setEditingTenant(false)
      setTenantNameDraft('')
      setNotice('租户名称已更新。')
    } catch (error) {
      setTenantError(formatAdminError(error))
    } finally {
      setSavingTenant(false)
    }
  }

  const createUser = async () => {
    if (!csrfToken) {
      setCreateError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    setCreating(true)
    setCreateError(null)
    setNotice(null)
    try {
      const request = {
        email: createDraft.email.trim(),
        displayName: createDraft.displayName.trim(),
        password: createDraft.password,
        ...(canManageRoles && createDraft.roleIds.length > 0 ? { roleIds: createDraft.roleIds } : {}),
      }
      const created = await userAdminApi.createUser(request, csrfToken)
      setCreateDraft(emptyCreateDraft())
      setNotice('用户已创建，密码输入已清空。')
      if (canReadUsers) {
        setUsersPage((current) => ({
          ...current,
          records: upsertById(current.records, created),
          total: Math.max(current.total, current.records.length + 1),
        }))
        void loadUsers()
      }
    } catch (error) {
      setCreateError(formatAdminError(error))
    } finally {
      setCreating(false)
    }
  }

  const saveUserDisplayName = async () => {
    if (!csrfToken || !editingUserId) {
      setUsersError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    setUpdatingUser(true)
    setUsersError(null)
    setNotice(null)
    try {
      const updated = await userAdminApi.updateUser(editingUserId, { displayName: editDisplayName.trim() }, csrfToken)
      setUsersPage((current) => ({
        ...current,
        records: upsertById(current.records, updated),
      }))
      setEditingUserId(null)
      setEditDisplayName('')
      setNotice('用户显示名已更新。')
    } catch (error) {
      setUsersError(formatAdminError(error))
    } finally {
      setUpdatingUser(false)
    }
  }

  const runStatusAction = async (user: UserAdminUser) => {
    if (!csrfToken) {
      setUsersError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }
    if (user.id === currentUserId) {
      setUsersError('不能禁用当前登录用户。')
      return
    }

    setUserActionId(user.id)
    setUsersError(null)
    setNotice(null)
    try {
      const updated = user.status === 'ACTIVE'
        ? await userAdminApi.disableUser(user.id, csrfToken)
        : await userAdminApi.enableUser(user.id, csrfToken)
      setUsersPage((current) => ({
        ...current,
        records: upsertById(current.records, updated),
      }))
      setNotice(updated.status === 'ACTIVE' ? '用户已启用。' : '用户已禁用。')
    } catch (error) {
      setUsersError(formatAdminError(error))
    } finally {
      setUserActionId(null)
    }
  }

  const saveUserRoles = async () => {
    if (!csrfToken || !roleEditUserId) {
      setUsersError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }
    if (!canManageRoles) {
      setUsersError('当前账号没有角色分配权限。')
      return
    }

    setSavingRoles(true)
    setUsersError(null)
    setNotice(null)
    try {
      const updated = await userAdminApi.replaceUserRoles(roleEditUserId, { roleIds: roleDraftIds }, csrfToken)
      setUsersPage((current) => ({
        ...current,
        records: upsertById(current.records, updated),
      }))
      setRoleEditUserId(null)
      setRoleDraftIds([])
      setNotice('用户角色已更新。')
    } catch (error) {
      setUsersError(formatAdminError(error))
    } finally {
      setSavingRoles(false)
    }
  }

  const startModelAccessEdit = async (user: UserAdminUser) => {
    if (!canManageModelAccess) {
      return
    }
    const panelSeq = panelSeqRef.current
    const requestSeq = modelAccessRequestSeqRef.current + 1
    modelAccessRequestSeqRef.current = requestSeq
    setModelAccessUserId(user.id)
    setAvailableModels([])
    setModelAccessDraftIds([])
    setModelAccessError(null)
    setLoadingModelAccess(true)
    try {
      const [models, access] = await Promise.all([
        loadAllModelsForAccess(modelApi),
        userAdminApi.getUserModelAccess(user.id),
      ])
      if (modelAccessRequestSeqRef.current !== requestSeq || panelSeqRef.current !== panelSeq || !isOpen) {
        return
      }
      setAvailableModels(models)
      setModelAccessDraftIds(access.modelIds)
    } catch (error) {
      if (modelAccessRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setModelAccessError(formatAdminError(error))
      }
    } finally {
      if (modelAccessRequestSeqRef.current === requestSeq && panelSeqRef.current === panelSeq) {
        setLoadingModelAccess(false)
      }
    }
  }

  const cancelModelAccessEdit = () => {
    modelAccessRequestSeqRef.current += 1
    setModelAccessUserId(null)
    setAvailableModels([])
    setModelAccessDraftIds([])
    setModelAccessError(null)
    setLoadingModelAccess(false)
  }

  const saveUserModelAccess = async () => {
    if (!csrfToken || !modelAccessUserId) {
      setModelAccessError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }
    if (!canManageModelAccess) {
      setModelAccessError('只有管理员可以分配中转站和模型。')
      return
    }

    setSavingModelAccess(true)
    setModelAccessError(null)
    setNotice(null)
    try {
      await userAdminApi.replaceUserModelAccess(modelAccessUserId, { modelIds: modelAccessDraftIds }, csrfToken)
      setNotice('用户可用模型已更新。')
      cancelModelAccessEdit()
    } catch (error) {
      setModelAccessError(formatAdminError(error))
    } finally {
      setSavingModelAccess(false)
    }
  }

  const createCustomRole = async () => {
    if (roleMutation) {
      return
    }
    if (!csrfToken) {
      setRolesError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    rolesRequestSeqRef.current += 1
    setLoadingRoles(false)
    setRoleMutation('create')
    setRolesError(null)
    setNotice(null)
    try {
      await userAdminApi.createRole({
        code: createRoleDraft.code.trim(),
        name: createRoleDraft.name.trim(),
        description: createRoleDraft.description.trim(),
      }, csrfToken)
      setCreateRoleDraft(emptyCreateRoleDraft())
      setNotice('自定义角色已创建。')
      await loadRolesAndPermissions()
    } catch (error) {
      setRolesError(formatAdminError(error))
    } finally {
      setRoleMutation(null)
    }
  }

  const saveCustomRole = async () => {
    if (roleMutation) {
      return
    }
    if (!csrfToken || !editingRoleId || !editRoleDraft) {
      setRolesError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    rolesRequestSeqRef.current += 1
    setLoadingRoles(false)
    setRoleMutation(editingRoleId)
    setRolesError(null)
    setNotice(null)
    try {
      await userAdminApi.updateRole(editingRoleId, {
        name: editRoleDraft.name.trim(),
        description: editRoleDraft.description.trim(),
        status: editRoleDraft.status,
      }, csrfToken)
      setEditingRoleId(null)
      setEditRoleDraft(null)
      setNotice('自定义角色已更新。')
      await loadRolesAndPermissions()
    } catch (error) {
      setRolesError(formatAdminError(error))
    } finally {
      setRoleMutation(null)
    }
  }

  const saveRolePermissions = async () => {
    if (roleMutation) {
      return
    }
    if (!csrfToken || !permissionEditRoleId) {
      setRolesError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    rolesRequestSeqRef.current += 1
    setLoadingRoles(false)
    setRoleMutation(permissionEditRoleId)
    setRolesError(null)
    setNotice(null)
    try {
      await userAdminApi.replaceRolePermissions(permissionEditRoleId, { permissionIds: permissionDraftIds }, csrfToken)
      setPermissionEditRoleId(null)
      setPermissionDraftIds([])
      setNotice('角色权限已更新。')
      await loadRolesAndPermissions()
    } catch (error) {
      setRolesError(formatAdminError(error))
    } finally {
      setRoleMutation(null)
    }
  }

  const deleteCustomRole = async (roleId: string) => {
    if (roleMutation) {
      return
    }
    if (!csrfToken) {
      setRolesError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    rolesRequestSeqRef.current += 1
    setLoadingRoles(false)
    setRoleMutation(roleId)
    setRolesError(null)
    setNotice(null)
    try {
      await userAdminApi.deleteRole(roleId, csrfToken)
      setDeleteConfirmRoleId(null)
      setNotice('自定义角色已删除。')
      await loadRolesAndPermissions()
    } catch (error) {
      setRolesError(formatAdminError(error))
    } finally {
      setRoleMutation(null)
    }
  }

  const totalPages = Math.max(1, Math.ceil(usersPage.total / usersPage.pageSize))
  const displayNameUser = usersPage.records.find((user) => user.id === editingUserId) ?? null
  const roleEditingUser = usersPage.records.find((user) => user.id === roleEditUserId) ?? null
  const modelAccessUser = usersPage.records.find((user) => user.id === modelAccessUserId) ?? null
  const editingRole = roles.find((role) => role.id === editingRoleId) ?? null
  const permissionEditingRole = roles.find((role) => role.id === permissionEditRoleId) ?? null

  return (
    <>
      <Modal isOpen={isOpen} maxWidthClass="max-w-6xl" onClose={handleClose} title="用户与角色管理">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="inline-flex rounded-md border border-ink-200 bg-ink-50 p-1">
            {availableTabs.includes('users') ? (
              <button className={tabClassName(activeTab === 'users')} onClick={() => setActiveTab('users')} type="button">
                用户
              </button>
            ) : null}
            {availableTabs.includes('roles') ? (
              <button className={tabClassName(activeTab === 'roles')} onClick={() => setActiveTab('roles')} type="button">
                角色与权限
              </button>
            ) : null}
          </div>

          <Button
            icon={<RefreshCw className={`h-4 w-4 ${isLoadingUsers || isLoadingRoles || isLoadingTenant ? 'animate-spin' : ''}`} />}
            onClick={() => {
              void loadCurrentTenant()
              if (activeTab === 'users') {
                void loadUsers()
              }
              if (canReadRoles) {
                void loadRolesAndPermissions()
              }
            }}
            variant="secondary"
          >
            刷新
          </Button>
        </div>

        <TenantManagementSection
          canManageTenant={canManageTenant}
          currentTenant={currentTenant}
          draftName={tenantNameDraft}
          error={tenantError}
          isEditing={isEditingTenant}
          isLoading={isLoadingTenant}
          isSaving={isSavingTenant}
          onCancel={() => {
            setEditingTenant(false)
            setTenantNameDraft('')
          }}
          onDraftNameChange={setTenantNameDraft}
          onSave={() => void saveCurrentTenant()}
          onStartEdit={() => {
            if (currentTenant) {
              setEditingTenant(true)
              setTenantNameDraft(currentTenant.name)
            }
          }}
        />

        <StatusMessage message={notice} tone="success" />

        {activeTab === 'users' && availableTabs.includes('users') ? (
          <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
            <div className="min-w-0 space-y-3">
              {canReadUsers ? (
                <>
                  <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_160px_auto]">
                    <input
                      aria-label="搜索用户"
                      className="field-input"
                      onChange={(event) => {
                        setUserQuery(event.target.value)
                        setUserPageNum(1)
                      }}
                      placeholder="搜索邮箱或显示名"
                      value={userQuery}
                    />
                    <select
                      aria-label="用户状态"
                      className="field-input"
                      onChange={(event) => {
                        setUserStatus(event.target.value as UserStatus | '')
                        setUserPageNum(1)
                      }}
                      value={userStatus}
                    >
                      {userStatuses.map((option) => (
                        <option key={option.value || 'all'} value={option.value}>
                          {option.label}
                        </option>
                      ))}
                    </select>
                    <Button disabled={isLoadingUsers} onClick={() => void loadUsers()} variant="secondary">
                      查询
                    </Button>
                  </div>

                  <StatusMessage message={usersError} tone="error" />

                  {isLoadingUsers ? (
                    <p className="rounded-md bg-ink-50 px-4 py-8 text-center text-sm text-ink-500">正在加载用户...</p>
                  ) : null}
                  {!isLoadingUsers && usersPage.records.length === 0 ? (
                    <EmptyState title="暂无用户" body="当前筛选条件下没有可见用户。" />
                  ) : null}

                  <div className="space-y-2">
                    {usersPage.records.map((user) => (
                      <UserListItem
                        actionId={userActionId}
                        canDisableUsers={canDisableUsers}
                        canManageRoles={canManageRoles}
                        canManageModelAccess={canManageModelAccess}
                        canReadRoles={canReadRoles}
                        canUpdateUsers={canUpdateUsers}
                        currentUserId={currentUserId}
                        key={user.id}
                        onEditDisplayName={(selected) => {
                          setRoleEditUserId(null)
                          cancelModelAccessEdit()
                          setEditingUserId(selected.id)
                          setEditDisplayName(selected.displayName)
                        }}
                        onStatusAction={(selected) => void runStatusAction(selected)}
                        user={user}
                        onStartRoleEdit={(selected) => {
                          setEditingUserId(null)
                          setEditDisplayName('')
                          cancelModelAccessEdit()
                          setRoleEditUserId(selected.id)
                          setRoleDraftIds(selected.roles.map((role) => role.id))
                        }}
                        onStartModelAccessEdit={(selected) => {
                          setEditingUserId(null)
                          setEditDisplayName('')
                          setRoleEditUserId(null)
                          setRoleDraftIds([])
                          void startModelAccessEdit(selected)
                        }}
                      />
                    ))}
                  </div>

                  <div className="flex items-center justify-between gap-3 text-sm text-ink-500">
                    <span>
                      第 {usersPage.pageNum} / {totalPages} 页，共 {usersPage.total} 个用户
                    </span>
                    <div className="flex gap-2">
                      <Button disabled={userPageNum <= 1 || isLoadingUsers} onClick={() => setUserPageNum((value) => Math.max(1, value - 1))}>
                        上一页
                      </Button>
                      <Button disabled={userPageNum >= totalPages || isLoadingUsers} onClick={() => setUserPageNum((value) => value + 1)}>
                        下一页
                      </Button>
                    </div>
                  </div>
                </>
              ) : (
                <PermissionHint message="当前账号没有 user:read 权限，面板不会调用用户列表接口。" />
              )}
            </div>

            {canCreateUsers ? (
              <CreateUserForm
                canManageRoles={canManageRoles}
                draft={createDraft}
                error={createError}
                isSaving={isCreating}
                onDraftChange={setCreateDraft}
                onSubmit={() => void createUser()}
                roles={canManageRoles ? roles : []}
              />
            ) : (
              <PermissionHint message="当前账号没有 user:create 权限，不能创建用户。" />
            )}
          </section>
        ) : null}

        {activeTab === 'roles' && availableTabs.includes('roles') ? (
          <RolePermissionView
            canManageRoles={canManageRoles}
            createDraft={createRoleDraft}
            deleteConfirmRoleId={deleteConfirmRoleId}
            error={rolesError}
            isLoading={isLoadingRoles}
            mutationId={roleMutation}
            onCancelDelete={() => setDeleteConfirmRoleId(null)}
            onConfirmDelete={(roleId) => void deleteCustomRole(roleId)}
            onCreateDraftChange={setCreateRoleDraft}
            onCreateRole={() => void createCustomRole()}
            onRequestDelete={setDeleteConfirmRoleId}
            onStartEdit={(role) => {
              setPermissionEditRoleId(null)
              setPermissionDraftIds([])
              setEditingRoleId(role.id)
              setEditRoleDraft({
                name: role.name,
                description: role.description,
                status: role.status,
              })
            }}
            onStartPermissions={(role) => {
              setEditingRoleId(null)
              setEditRoleDraft(null)
              setPermissionEditRoleId(role.id)
              setPermissionDraftIds((role.permissions ?? []).map((permission) => String(permission.id)))
            }}
            permissions={permissions}
            roles={roles}
          />
        ) : null}
        </div>
      </Modal>

      <EditorDrawer
        isOpen={isOpen && Boolean(displayNameUser)}
        onClose={() => {
          setEditingUserId(null)
          setEditDisplayName('')
        }}
        title="编辑用户"
      >
        {displayNameUser ? (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              void saveUserDisplayName()
            }}
          >
            <div>
              <p className="text-sm font-semibold text-ink-900">{displayNameUser.displayName}</p>
              <p className="mt-1 text-xs text-ink-500">{displayNameUser.email}</p>
            </div>
            <Field label="显示名">
              <input
                aria-label={`新的显示名 ${displayNameUser.email}`}
                autoFocus
                className="field-input"
                onChange={(event) => setEditDisplayName(event.target.value)}
                required
                value={editDisplayName}
              />
            </Field>
            <DrawerActions>
              <Button disabled={isUpdatingUser} icon={isUpdatingUser ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} type="submit" variant="primary">
                保存
              </Button>
              <Button disabled={isUpdatingUser} onClick={() => {
                setEditingUserId(null)
                setEditDisplayName('')
              }}>
                取消
              </Button>
            </DrawerActions>
          </form>
        ) : null}
      </EditorDrawer>

      <EditorDrawer
        isOpen={isOpen && Boolean(roleEditingUser)}
        onClose={() => {
          setRoleEditUserId(null)
          setRoleDraftIds([])
        }}
        title="分配用户角色"
      >
        {roleEditingUser ? (
          <div className="space-y-4">
            <div>
              <p className="text-sm font-semibold text-ink-900">{roleEditingUser.displayName}</p>
              <p className="mt-1 text-xs text-ink-500">{roleEditingUser.email}</p>
            </div>
            <RoleCheckboxes label={`角色分配 ${roleEditingUser.email}`} onChange={setRoleDraftIds} roleIds={roleDraftIds} roles={roles} />
            <DrawerActions>
              <Button disabled={isSavingRoles} icon={isSavingRoles ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} onClick={() => void saveUserRoles()} variant="primary">
                保存角色
              </Button>
              <Button disabled={isSavingRoles} onClick={() => {
                setRoleEditUserId(null)
                setRoleDraftIds([])
              }}>
                取消
              </Button>
            </DrawerActions>
          </div>
        ) : null}
      </EditorDrawer>

      <EditorDrawer isOpen={isOpen && Boolean(modelAccessUser)} onClose={cancelModelAccessEdit} title="分配可用模型" widthClass="max-w-2xl">
        {modelAccessUser ? (
          <UserModelAccessEditor
            error={modelAccessError}
            isLoading={isLoadingModelAccess}
            isSaving={isSavingModelAccess}
            modelIds={modelAccessDraftIds}
            models={availableModels}
            onCancel={cancelModelAccessEdit}
            onChange={setModelAccessDraftIds}
            onSave={() => void saveUserModelAccess()}
            userEmail={modelAccessUser.email}
          />
        ) : null}
      </EditorDrawer>

      <EditorDrawer
        isOpen={isOpen && Boolean(editingRole && editRoleDraft)}
        onClose={() => {
          setEditingRoleId(null)
          setEditRoleDraft(null)
        }}
        title="编辑角色"
      >
        {editingRole && editRoleDraft ? (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault()
              void saveCustomRole()
            }}
          >
            <div>
              <p className="text-sm font-semibold text-ink-900">{editingRole.name}</p>
              <p className="mt-1 text-xs text-ink-500">角色代码：{editingRole.code}</p>
            </div>
            <Field label="编辑角色名称">
              <input className="field-input" onChange={(event) => setEditRoleDraft({ ...editRoleDraft, name: event.target.value })} required value={editRoleDraft.name} />
            </Field>
            <Field label="编辑角色说明">
              <input className="field-input" onChange={(event) => setEditRoleDraft({ ...editRoleDraft, description: event.target.value })} value={editRoleDraft.description} />
            </Field>
            <Field label="编辑角色状态">
              <select className="field-input" onChange={(event) => setEditRoleDraft({ ...editRoleDraft, status: event.target.value as UserAdminRoleStatus })} value={editRoleDraft.status}>
                <option value="ACTIVE">启用</option>
                <option value="DISABLED">禁用</option>
              </select>
            </Field>
            <DrawerActions>
              <Button disabled={Boolean(roleMutation)} icon={roleMutation === editingRole.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} type="submit" variant="primary">
                保存角色
              </Button>
              <Button disabled={Boolean(roleMutation)} onClick={() => {
                setEditingRoleId(null)
                setEditRoleDraft(null)
              }}>
                取消
              </Button>
            </DrawerActions>
          </form>
        ) : null}
      </EditorDrawer>

      <EditorDrawer
        isOpen={isOpen && Boolean(permissionEditingRole)}
        onClose={() => {
          setPermissionEditRoleId(null)
          setPermissionDraftIds([])
        }}
        title="配置角色权限"
        widthClass="max-w-2xl"
      >
        {permissionEditingRole ? (
          <div className="space-y-4">
            <div>
              <p className="text-sm font-semibold text-ink-900">{permissionEditingRole.name}</p>
              <p className="mt-1 text-xs text-ink-500">为该角色选择可用权限，修改将在保存后生效。</p>
            </div>
            <PermissionCheckboxes onChange={setPermissionDraftIds} permissionIds={permissionDraftIds} permissions={permissions} />
            <DrawerActions>
              <Button disabled={Boolean(roleMutation)} icon={roleMutation === permissionEditingRole.id ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} onClick={() => void saveRolePermissions()} variant="primary">
                保存权限
              </Button>
              <Button disabled={Boolean(roleMutation)} onClick={() => {
                setPermissionEditRoleId(null)
                setPermissionDraftIds([])
              }}>
                取消
              </Button>
            </DrawerActions>
          </div>
        ) : null}
      </EditorDrawer>
    </>
  )
}

interface CreateUserFormProps {
  draft: CreateUserDraft
  roles: UserAdminRole[]
  canManageRoles: boolean
  isSaving: boolean
  error: string | null
  onDraftChange: (draft: CreateUserDraft | ((current: CreateUserDraft) => CreateUserDraft)) => void
  onSubmit: () => void
}

function CreateUserForm({ canManageRoles, draft, error, isSaving, onDraftChange, onSubmit, roles }: CreateUserFormProps) {
  return (
    <form
      className="space-y-3 rounded-lg border border-ink-200 bg-ink-50 p-4"
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <div>
        <h3 className="text-sm font-semibold text-ink-900">新建用户</h3>
        <p className="text-xs text-ink-500">密码只用于本次提交，成功后会清空。</p>
      </div>

      <Field label="用户邮箱">
        <input
          autoComplete="off"
          className="field-input"
          onChange={(event) => onDraftChange((current) => ({ ...current, email: event.target.value }))}
          required
          type="email"
          value={draft.email}
        />
      </Field>
      <Field label="显示名">
        <input
          className="field-input"
          onChange={(event) => onDraftChange((current) => ({ ...current, displayName: event.target.value }))}
          required
          value={draft.displayName}
        />
      </Field>
      <Field label="初始密码">
        <input
          autoComplete="new-password"
          className="field-input"
          onChange={(event) => onDraftChange((current) => ({ ...current, password: event.target.value }))}
          required
          type="password"
          value={draft.password}
        />
      </Field>

      {canManageRoles ? (
        <RoleCheckboxes
          label="创建时分配角色"
          onChange={(roleIds) => onDraftChange((current) => ({ ...current, roleIds }))}
          roleIds={draft.roleIds}
          roles={roles}
        />
      ) : (
        <p className="rounded-md border border-ink-200 bg-white px-3 py-2 text-xs text-ink-500">
          当前账号没有 role:manage 权限，将创建无角色用户。
        </p>
      )}

      <StatusMessage message={error} tone="error" />
      <Button className="w-full" disabled={isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />} type="submit" variant="primary">
        创建用户
      </Button>
    </form>
  )
}

interface UserListItemProps {
  user: UserAdminUser
  currentUserId: UserId | string
  canUpdateUsers: boolean
  canDisableUsers: boolean
  canReadRoles: boolean
  canManageRoles: boolean
  canManageModelAccess: boolean
  actionId: string | null
  onEditDisplayName: (user: UserAdminUser) => void
  onStatusAction: (user: UserAdminUser) => void
  onStartRoleEdit: (user: UserAdminUser) => void
  onStartModelAccessEdit: (user: UserAdminUser) => void
}

function UserListItem({
  actionId,
  canDisableUsers,
  canManageRoles,
  canManageModelAccess,
  canReadRoles,
  canUpdateUsers,
  currentUserId,
  onEditDisplayName,
  onStartModelAccessEdit,
  onStartRoleEdit,
  onStatusAction,
  user,
}: UserListItemProps) {
  const isCurrentUser = user.id === currentUserId
  const isAdminUser = user.roles.some((role) => role.code === 'admin')
  const isPending = actionId === user.id
  const statusActionLabel = user.status === 'ACTIVE' ? '禁用' : '启用'

  return (
    <article className="rounded-lg border border-ink-200 bg-white p-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="truncate text-sm font-semibold text-ink-900">{user.displayName}</h3>
            <StatusBadge status={user.status} />
            {isCurrentUser ? <span className="rounded-md bg-amazon-50 px-2 py-1 text-xs font-semibold text-amazon-700">当前用户</span> : null}
          </div>
          <p className="truncate text-xs text-ink-500">{user.email}</p>
          <p className="text-xs text-ink-500">角色：{formatRoleSummary(user.roles)}</p>
          <p className="text-xs text-ink-400">最后登录：{user.lastLoginAt ? formatDateTime(user.lastLoginAt) : '未登录'}</p>
        </div>

        <div className="flex flex-wrap gap-1">
          {canUpdateUsers ? (
            <button
              aria-label={`编辑显示名 ${user.email}`}
              className="icon-button h-8 w-8"
              onClick={() => onEditDisplayName(user)}
              title="编辑显示名"
              type="button"
            >
              <Pencil className="h-4 w-4" />
            </button>
          ) : null}
          {canManageRoles && canReadRoles ? (
            <button
              aria-label={`分配角色 ${user.email}`}
              className="icon-button h-8 w-8"
              onClick={() => onStartRoleEdit(user)}
              title="分配角色"
              type="button"
            >
              <ShieldCheck className="h-4 w-4" />
            </button>
          ) : null}
          {canManageModelAccess && !isAdminUser ? (
            <button
              aria-label={`分配可用模型 ${user.email}`}
              className="icon-button h-8 w-8"
              onClick={() => onStartModelAccessEdit(user)}
              title="分配可用中转站与模型"
              type="button"
            >
              <Layers3 className="h-4 w-4" />
            </button>
          ) : null}
          {canDisableUsers ? (
            <button
              aria-label={`${statusActionLabel}用户 ${user.email}`}
              className="icon-button h-8 w-8"
              disabled={isPending || isCurrentUser}
              onClick={() => onStatusAction(user)}
              title={isCurrentUser ? '不能禁用当前登录用户' : statusActionLabel}
              type="button"
            >
              {isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : user.status === 'ACTIVE' ? (
                <PowerOff className="h-4 w-4" />
              ) : (
                <Power className="h-4 w-4" />
              )}
            </button>
          ) : null}
        </div>
      </div>

      {isCurrentUser && canDisableUsers ? (
        <p className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">
          安全保护：不能禁用当前登录用户。
        </p>
      ) : null}
    </article>
  )
}

interface TenantManagementSectionProps {
  canManageTenant: boolean
  currentTenant: CurrentTenantAdminResponse | null
  draftName: string
  error: string | null
  isEditing: boolean
  isLoading: boolean
  isSaving: boolean
  onCancel: () => void
  onDraftNameChange: (value: string) => void
  onSave: () => void
  onStartEdit: () => void
}

function TenantManagementSection({
  canManageTenant,
  currentTenant,
  draftName,
  error,
  isEditing,
  isLoading,
  isSaving,
  onCancel,
  onDraftNameChange,
  onSave,
  onStartEdit,
}: TenantManagementSectionProps) {
  return (
    <section className="rounded-lg border border-ink-200 bg-ink-50 p-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-ink-900">当前租户</h3>
          {isLoading ? <p className="mt-1 text-xs text-ink-500">正在加载租户信息...</p> : null}
          {!isLoading && currentTenant ? <p className="mt-1 text-sm text-ink-700">{currentTenant.name}</p> : null}
          {!isLoading && !currentTenant && !error ? <p className="mt-1 text-xs text-ink-500">暂无租户信息。</p> : null}
        </div>
        {canManageTenant && currentTenant && !isEditing ? (
          <button
            aria-label="编辑当前租户名称"
            className="icon-button h-8 w-8"
            onClick={onStartEdit}
            title="编辑当前租户名称"
            type="button"
          >
            <Pencil className="h-4 w-4" />
          </button>
        ) : null}
      </div>

      <StatusMessage message={error} tone="error" />

      {isEditing ? (
        <form
          className="mt-3 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto_auto]"
          onSubmit={(event) => {
            event.preventDefault()
            onSave()
          }}
        >
          <input
            aria-label="当前租户名称"
            className="field-input"
            onChange={(event) => onDraftNameChange(event.target.value)}
            required
            value={draftName}
          />
          <Button disabled={isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} type="submit" variant="primary">
            保存租户名称
          </Button>
          <Button disabled={isSaving} onClick={onCancel}>
            取消
          </Button>
        </form>
      ) : null}
    </section>
  )
}

interface RolePermissionViewProps {
  roles: UserAdminRole[]
  permissions: UserAdminPermission[]
  isLoading: boolean
  error: string | null
  canManageRoles: boolean
  createDraft: CreateRoleDraft
  deleteConfirmRoleId: string | null
  mutationId: string | null
  onCreateDraftChange: (draft: CreateRoleDraft | ((current: CreateRoleDraft) => CreateRoleDraft)) => void
  onCreateRole: () => void
  onStartEdit: (role: UserAdminRole) => void
  onStartPermissions: (role: UserAdminRole) => void
  onRequestDelete: (roleId: string) => void
  onCancelDelete: () => void
  onConfirmDelete: (roleId: string) => void
}

function RolePermissionView({
  canManageRoles,
  createDraft,
  deleteConfirmRoleId,
  error,
  isLoading,
  mutationId,
  onCancelDelete,
  onConfirmDelete,
  onCreateDraftChange,
  onCreateRole,
  onRequestDelete,
  onStartEdit,
  onStartPermissions,
  permissions,
  roles,
}: RolePermissionViewProps) {
  return (
    <section className="space-y-4">
      <StatusMessage message={error} tone="error" />
      {isLoading ? <p className="rounded-md bg-ink-50 px-4 py-8 text-center text-sm text-ink-500">正在加载角色与权限...</p> : null}
      {!isLoading && roles.length === 0 ? <EmptyState title="暂无角色" body="当前租户没有可见角色。" /> : null}

      {canManageRoles ? (
        <form
          className="grid gap-3 rounded-lg border border-ink-200 bg-ink-50 p-3 md:grid-cols-3"
          onSubmit={(event) => {
            event.preventDefault()
            onCreateRole()
          }}
        >
          <Field label="角色代码">
            <input
              className="field-input"
              onChange={(event) => onCreateDraftChange((current) => ({ ...current, code: event.target.value }))}
              required
              value={createDraft.code}
            />
          </Field>
          <Field label="角色名称">
            <input
              className="field-input"
              onChange={(event) => onCreateDraftChange((current) => ({ ...current, name: event.target.value }))}
              required
              value={createDraft.name}
            />
          </Field>
          <Field label="角色说明">
            <input
              className="field-input"
              onChange={(event) => onCreateDraftChange((current) => ({ ...current, description: event.target.value }))}
              value={createDraft.description}
            />
          </Field>
          <Button className="md:col-span-3 md:w-fit" disabled={Boolean(mutationId)} icon={mutationId === 'create' ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />} type="submit" variant="primary">
            创建角色
          </Button>
        </form>
      ) : null}

      <div className="grid gap-3 lg:grid-cols-2">
        {roles.map((role) => {
          const isBuiltIn = isBuiltInRole(role)
          const isConfirmingDelete = deleteConfirmRoleId === role.id

          return (
            <article className="rounded-lg border border-ink-200 bg-white p-3" key={role.id}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-semibold text-ink-900">{role.name}</h3>
                  <span className="rounded-md bg-ink-100 px-2 py-1 text-xs font-semibold text-ink-600">{role.code}</span>
                  <StatusBadge status={role.status} />
                </div>
                {!isBuiltIn && canManageRoles ? (
                  <div className="flex gap-1">
                    <button aria-label={`编辑角色 ${role.name}`} className="icon-button h-8 w-8" disabled={Boolean(mutationId)} onClick={() => onStartEdit(role)} title="编辑角色" type="button">
                      <Pencil className="h-4 w-4" />
                    </button>
                    <button aria-label={`配置角色权限 ${role.name}`} className="icon-button h-8 w-8" disabled={Boolean(mutationId)} onClick={() => onStartPermissions(role)} title="配置角色权限" type="button">
                      <ShieldCheck className="h-4 w-4" />
                    </button>
                    <button aria-label={`删除角色 ${role.name}`} className="icon-button h-8 w-8" disabled={Boolean(mutationId)} onClick={() => onRequestDelete(role.id)} title="删除角色" type="button">
                      <Trash2 className="h-4 w-4" />
                    </button>
                  </div>
                ) : null}
              </div>
              {isBuiltIn ? <p className="mt-2 text-xs font-semibold text-ink-500">系统内置，只读</p> : null}
              {role.description ? <p className="mt-2 text-xs text-ink-500">{role.description}</p> : null}
              <div className="mt-3 flex flex-wrap gap-1">
                {(role.permissions ?? []).length > 0 ? (
                  role.permissions?.map((permission) => <PermissionPill key={String(permission.code)} permission={permission} />)
                ) : (
                  <span className="text-xs text-ink-400">未返回权限明细</span>
                )}
              </div>

              {isConfirmingDelete ? (
                <div className="mt-3 space-y-2 rounded-md border border-red-200 bg-red-50 p-3 text-xs text-red-700">
                  <p>确认删除此自定义角色？</p>
                  <div className="flex flex-wrap gap-2">
                    <Button aria-label={`确认删除角色 ${role.name}`} disabled={Boolean(mutationId)} onClick={() => onConfirmDelete(role.id)} variant="danger">
                      确认删除
                    </Button>
                    <Button disabled={Boolean(mutationId)} onClick={onCancelDelete}>取消</Button>
                  </div>
                </div>
              ) : null}
            </article>
          )
        })}
      </div>

      <div>
        <h3 className="mb-2 text-sm font-semibold text-ink-900">权限目录</h3>
        <div className="grid gap-2 md:grid-cols-2">
          {permissions.map((permission) => (
            <div className="rounded-md border border-ink-200 bg-ink-50 px-3 py-2" key={String(permission.code)}>
              <p className="text-sm font-semibold text-ink-800">{permissionCopy(permission).name}</p>
              <p className="mt-1 text-xs leading-5 text-ink-500">{permissionCopy(permission).description}</p>
              <code className="mt-1 inline-block rounded bg-white px-1.5 py-0.5 text-[11px] text-ink-500">{String(permission.code)}</code>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function PermissionCheckboxes({
  onChange,
  permissionIds,
  permissions,
}: {
  permissionIds: string[]
  permissions: UserAdminPermission[]
  onChange: (permissionIds: string[]) => void
}) {
  return (
    <fieldset className="rounded-md border border-ink-200 bg-white px-3 py-2">
      <legend className="px-1 text-xs font-semibold text-ink-600">角色权限</legend>
      {permissions.length === 0 ? <p className="text-xs text-ink-400">权限目录为空。</p> : null}
      <div className="mt-2 grid gap-2">
        {permissions.map((permission) => {
          const id = String(permission.id)
          const copy = permissionCopy(permission)
          return (
            <label className="flex items-center gap-2 text-sm text-ink-700" key={id}>
              <input
                aria-label={`角色权限 ${String(permission.code)}`}
                checked={permissionIds.includes(id)}
                onChange={(event) => {
                  if (event.target.checked) {
                    onChange([...permissionIds, id])
                  } else {
                    onChange(permissionIds.filter((permissionId) => permissionId !== id))
                  }
                }}
                type="checkbox"
              />
              <span className="min-w-0">
                <span className="block font-medium text-ink-800">{copy.name}</span>
                <span className="block text-xs text-ink-500">{String(permission.code)} · {copy.description}</span>
              </span>
            </label>
          )
        })}
      </div>
    </fieldset>
  )
}

function RoleCheckboxes({
  label,
  onChange,
  roleIds,
  roles,
}: {
  label: string
  roleIds: string[]
  roles: UserAdminRole[]
  onChange: (roleIds: string[]) => void
}) {
  return (
    <fieldset className="rounded-md border border-ink-200 bg-white px-3 py-2">
      <legend className="px-1 text-xs font-semibold text-ink-600">{label}</legend>
      {roles.length === 0 ? <p className="text-xs text-ink-400">没有可分配角色，或当前账号没有 role:read 权限。</p> : null}
      <div className="mt-2 grid gap-2">
        {roles.map((role) => (
          <label className="flex items-center gap-2 text-sm text-ink-700" key={role.id}>
            <input
              checked={roleIds.includes(role.id)}
              onChange={(event) => {
                if (event.target.checked) {
                  onChange([...roleIds, role.id])
                } else {
                  onChange(roleIds.filter((roleId) => roleId !== role.id))
                }
              }}
              type="checkbox"
            />
            <span>
              {role.name} <span className="text-xs text-ink-400">({role.code})</span>
            </span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}

function Field({ children, label }: { children: ReactNode; label: string }) {
  return (
    <label className="grid gap-1 text-sm text-ink-700">
      <span className="field-label">{label}</span>
      {children}
    </label>
  )
}

function DrawerActions({ children }: { children: ReactNode }) {
  return (
    <div className="sticky bottom-0 -mx-5 mt-6 flex flex-wrap gap-2 border-t border-ink-200 bg-white px-5 py-4">
      {children}
    </div>
  )
}

function StatusMessage({ message, tone }: { message: string | null; tone: 'error' | 'success' }) {
  if (!message) {
    return null
  }

  const classes = tone === 'error' ? 'border-red-200 bg-red-50 text-red-700' : 'border-green-200 bg-green-50 text-green-700'
  const Icon = tone === 'error' ? AlertTriangle : CheckCircle2

  return (
    <div className={`flex items-start gap-2 rounded-md border px-3 py-2 text-sm ${classes}`}>
      <Icon className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  )
}

function EmptyState({ body, title }: { title: string; body: string }) {
  return (
    <div className="rounded-md border border-dashed border-ink-200 bg-ink-50 px-4 py-8 text-center">
      <p className="text-sm font-semibold text-ink-700">{title}</p>
      <p className="mt-1 text-sm text-ink-500">{body}</p>
    </div>
  )
}

function PermissionHint({ message }: { message: string }) {
  return <div className="rounded-lg border border-ink-200 bg-ink-50 p-4 text-sm text-ink-600">{message}</div>
}

function StatusBadge({ status }: { status: string }) {
  const active = status === 'ACTIVE' || status === 'ENABLED'
  return (
    <span className={`rounded-md px-2 py-1 text-xs font-semibold ${active ? 'bg-green-50 text-green-700' : 'bg-ink-100 text-ink-600'}`}>
      {active ? '启用' : '禁用'}
    </span>
  )
}

function PermissionPill({ permission }: { permission: UserAdminPermission }) {
  const copy = permissionCopy(permission)
  return (
    <span className="rounded-md bg-amazon-50 px-2 py-1 text-xs font-semibold text-amazon-700" title={`${String(permission.code)}：${copy.description}`}>
      {copy.name}
    </span>
  )
}

function getAvailableTabs(permissions: {
  canReadUsers: boolean
  canCreateUsers: boolean
  canUpdateUsers: boolean
  canDisableUsers: boolean
  canReadRoles: boolean
  canManageRoles: boolean
}): IdentityAdminTab[] {
  const tabs: IdentityAdminTab[] = []
  if (permissions.canReadUsers || permissions.canCreateUsers || permissions.canUpdateUsers || permissions.canDisableUsers || permissions.canManageRoles) {
    tabs.push('users')
  }
  if (permissions.canReadRoles) {
    tabs.push('roles')
  }
  return tabs
}

function emptyUserPage(): UserPageState {
  return {
    records: [],
    total: 0,
    pageNum: 1,
    pageSize: PAGE_SIZE,
  }
}

function emptyCreateDraft(): CreateUserDraft {
  return {
    email: '',
    displayName: '',
    password: '',
    roleIds: [],
  }
}

function emptyCreateRoleDraft(): CreateRoleDraft {
  return {
    code: '',
    name: '',
    description: '',
  }
}

function isBuiltInRole(role: UserAdminRole): boolean {
  return BUILT_IN_ROLE_CODES.has(role.code)
}

function upsertById<TItem extends { id: string }>(items: TItem[], nextItem: TItem): TItem[] {
  const index = items.findIndex((item) => item.id === nextItem.id)
  if (index === -1) {
    return [nextItem, ...items]
  }

  const nextItems = [...items]
  nextItems[index] = nextItem
  return nextItems
}

function formatRoleSummary(roles: UserAdminRole[]): string {
  if (roles.length === 0) {
    return '无角色'
  }
  return roles.map((role) => role.name || role.code).join('、')
}

function formatAdminError(error: unknown): string {
  if (isApiClientError(error)) {
    if (error.status === 401) {
      return '登录状态已过期，请重新登录。'
    }
    if (error.status === 403) {
      return '当前账号没有此管理权限。'
    }
    if (error.status === 409) {
      return `操作冲突：${error.message}`
    }
    if (error.status === 422) {
      return `表单内容未通过校验：${error.message}`
    }
    return error.message
  }

  if (error instanceof Error) {
    return error.message
  }

  return '请求失败，请稍后重试。'
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function tabClassName(active: boolean): string {
  return `rounded px-3 py-1.5 text-sm font-semibold transition ${
    active ? 'bg-white text-ink-900 shadow-sm' : 'text-ink-500 hover:text-ink-900'
  }`
}
