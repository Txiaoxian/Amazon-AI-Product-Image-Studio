import { Filter, RotateCcw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { modelApi } from '../../../api/models'
import { projectApi } from '../../../api/projects'
import { providerApi } from '../../../api/providers'
import { userAdminApi } from '../../../api/userAdmin'
import { ADMIN_IMAGE_TYPE_VALUES, imageTypeLabel } from '../../../lib/adminPresentation'
import type { Model, Project, Provider } from '../../../types/platform'
import type { UserAdminUser } from '../../../types/userAdmin'
import { Button } from '../../ui/Button'
import { useAdminConsole } from './AdminConsoleContext'

interface FilterOption {
  value: string
  label: string
}

interface AdminFilterBarProps {
  statusOptions?: FilterOption[]
  showImageType?: boolean
  showUser?: boolean
  showProject?: boolean
  showProvider?: boolean
  showModel?: boolean
  children?: React.ReactNode
}

export function AdminFilterBar({
  statusOptions = [],
  showImageType = true,
  showUser = true,
  showProject = true,
  showProvider = true,
  showModel = true,
  children,
}: AdminFilterBarProps) {
  const { route, updateQuery, hasPermission } = useAdminConsole()
  const [providers, setProviders] = useState<Provider[]>([])
  const [models, setModels] = useState<Model[]>([])
  const [projects, setProjects] = useState<Project[]>([])
  const [users, setUsers] = useState<UserAdminUser[]>([])

  useEffect(() => {
    let active = true
    const load = async () => {
      const [providerPage, modelPage, projectPage, userPage] = await Promise.all([
        showProvider && (hasPermission('provider:read') || hasPermission('provider:manage'))
          ? providerApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null)
          : null,
        showModel && (hasPermission('model:read') || hasPermission('model:manage'))
          ? modelApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null)
          : null,
        showProject && hasPermission('project:read')
          ? projectApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null)
          : null,
        showUser && hasPermission('user:read')
          ? userAdminApi.listUsers({ pageNum: 1, pageSize: 100 }).catch(() => null)
          : null,
      ])
      if (!active) return
      setProviders(providerPage?.records ?? [])
      setModels(modelPage?.records ?? [])
      setProjects(projectPage?.records ?? [])
      setUsers(userPage?.records ?? [])
    }
    void load()
    return () => {
      active = false
    }
  }, [hasPermission, showModel, showProject, showProvider, showUser])

  const current = (key: string) => route.searchParams.get(key) ?? ''
  const update = (key: string, value: string) => updateQuery({ [key]: value || null, pageNum: null }, true)
  const clear = () => updateQuery({ userId: null, projectId: null, providerId: null, modelId: null, status: null, imageType: null, search: null, pageNum: null }, true)
  const hasFilters = ['userId', 'projectId', 'providerId', 'modelId', 'status', 'imageType', 'search'].some((key) => route.searchParams.has(key))

  return (
    <section aria-label="数据筛选" className="rounded-xl border border-slate-200 bg-white p-3 shadow-panel">
      <div className="flex flex-wrap items-end gap-2">
        <div className="flex min-h-10 items-center gap-2 px-1 text-xs font-semibold text-slate-600"><Filter className="h-4 w-4" />筛选</div>
        {showUser && hasPermission('user:read') ? <FilterSelect label="用户" value={current('userId')} onChange={(value) => update('userId', value)} options={users.map((user) => ({ value: user.id, label: user.displayName || user.email }))} /> : null}
        {showProject && hasPermission('project:read') ? <FilterSelect label="项目" value={current('projectId')} onChange={(value) => update('projectId', value)} options={projects.map((project) => ({ value: project.id, label: project.name }))} /> : null}
        {showProvider && (hasPermission('provider:read') || hasPermission('provider:manage')) ? <FilterSelect label="中转站" value={current('providerId')} onChange={(value) => update('providerId', value)} options={providers.map((provider) => ({ value: provider.id, label: provider.name }))} /> : null}
        {showModel && (hasPermission('model:read') || hasPermission('model:manage')) ? <FilterSelect label="模型" value={current('modelId')} onChange={(value) => update('modelId', value)} options={models.filter((model) => !current('providerId') || model.providerId === current('providerId')).map((model) => ({ value: model.id, label: model.displayName || model.modelName }))} /> : null}
        {statusOptions.length > 0 ? <FilterSelect label="状态" value={current('status')} onChange={(value) => update('status', value)} options={statusOptions} /> : null}
        {showImageType ? <FilterSelect label="图片类型" value={current('imageType')} onChange={(value) => update('imageType', value)} options={ADMIN_IMAGE_TYPE_VALUES.map((value) => ({ value, label: imageTypeLabel(value) }))} /> : null}
        {children}
        {hasFilters ? <Button className="ml-auto min-h-10" icon={<RotateCcw className="h-4 w-4" />} onClick={clear} variant="ghost">清除筛选</Button> : null}
      </div>
    </section>
  )
}

function FilterSelect({ label, value, options, onChange }: { label: string; value: string; options: FilterOption[]; onChange: (value: string) => void }) {
  const hasSelectedOption = !value || options.some((option) => String(option.value) === value)
  return (
    <label className="grid min-w-[136px] gap-1 text-[11px] font-semibold text-slate-500">
      {label}
      <select className="min-h-10 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-medium text-slate-700 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20" onChange={(event) => onChange(event.target.value)} value={value}>
        <option value="">全部{label}</option>
        {!hasSelectedOption ? <option value={value}>已选{label}（名称不可用）</option> : null}
        {options.map((option) => <option key={String(option.value)} value={String(option.value)}>{option.label || `${label}名称未设置`}</option>)}
      </select>
    </label>
  )
}
