import { Copy, Download, Plus, RefreshCw, Search, ShieldCheck, UserRound } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import type { EChartsCoreOption } from 'echarts/core'
import { adminApi } from '../../../api/admin'
import { userAdminApi } from '../../../api/userAdmin'
import {
  ADMIN_CHART_TOKENS,
  buildEntityStatusOptions,
  entityStatusLabel,
  formatCompactNumber,
  formatCurrency,
  formatDateLabel,
  formatDateTime,
  formatExactNumber,
  formatPercentage,
  lifecycleLabel,
  roleLabel,
} from '../../../lib/adminPresentation'
import type { AnalyticsUserDetailResponse, AnalyticsUserRecord } from '../../../types/analytics'
import type { UserAdminRole, UserAdminUser } from '../../../types/userAdmin'
import { Button } from '../../ui/Button'
import { EditorDrawer } from '../../ui/EditorDrawer'
import { AdminChart } from './AdminChart'
import { AdminFilterBar } from './AdminFilterBar'
import { useAdminAnalyticsQuery, useAdminConsole } from './AdminConsoleContext'
import { ActionFeedback, EmptyState, ErrorState, LoadingState, Pagination, SectionCard, StatusBadge, TableShell, tableBodyClassName, tableClassName, tableHeadClassName } from './AdminUi'
import { useAdminData } from './useAdminData'
import { useAnalyticsExport } from './useAnalyticsExport'

export function AdminUsersPage() {
  const { route, updateQuery, hasPermission, session } = useAdminConsole()
  const baseQuery = useAdminAnalyticsQuery()
  const pageNum = Number(route.searchParams.get('pageNum') || 1)
  const search = route.searchParams.get('search') ?? ''
  const query = { ...baseQuery, pageNum, pageSize: 20, search: search || undefined }
  const queryKey = JSON.stringify(query)
  const { data, status, reload } = useAdminData(() => adminApi.getAnalyticsUsers(query), [queryKey])
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const canViewCosts = hasPermission('usage:read')
  const { exportData, exporting, feedback } = useAnalyticsExport('users', query)

  if (status === 'loading' && !data) return <LoadingState moduleName="用户与活跃" />
  if (status === 'error' && !data) return <ErrorState moduleName="用户与活跃" onRetry={() => void reload()} />
  if (!data) return null

  return (
    <div className="space-y-4 sm:space-y-5">
      <AdminFilterBar showUser={false} statusOptions={buildEntityStatusOptions(['ACTIVE', 'DISABLED'])}>
        <label className="grid min-w-[210px] flex-1 gap-1 text-[11px] font-semibold text-slate-500 sm:max-w-xs">搜索用户<span className="relative"><Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" /><input className="min-h-10 w-full rounded-lg border border-slate-200 bg-white pl-8 pr-3 text-xs font-medium text-slate-700 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20" onChange={(event) => updateQuery({ search: event.target.value || null, pageNum: null }, true)} placeholder="输入名称或邮箱" value={search} /></span></label>
      </AdminFilterBar>

      <ActionFeedback feedback={feedback} />

      <SectionCard title="用户表现与生命周期" description="“生图活跃”以统计周期内是否创建任务判定；最近登录仅作为独立参考。" action={<div className="flex flex-wrap gap-2"><Button disabled={status === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${status === 'loading' ? 'animate-spin' : ''}`} />} onClick={() => void reload()} variant="ghost">刷新</Button><Button disabled={exporting} icon={<Download className="h-4 w-4" />} onClick={() => void exportData()} variant="secondary">{exporting ? '正在导出...' : '导出中文 CSV'}</Button>{hasPermission('user:create') ? <Button icon={<Plus className="h-4 w-4" />} onClick={() => setCreateOpen(true)} variant="primary">新建用户</Button> : null}</div>}>
        {data.records.length === 0 ? <EmptyState message="当前筛选条件下没有用户记录，可清除筛选或调整时间范围。" /> : (
          <>
            <TableShell label="用户与活跃明细">
              <table className={tableClassName}>
                <thead className={tableHeadClassName}><tr><th className="px-3 py-3">用户</th><th className="px-3 py-3">生命周期</th><th className="px-3 py-3">最近登录</th><th className="px-3 py-3">活跃天数</th><th className="px-3 py-3">任务 / 出图</th><th className="px-3 py-3">任务成功率</th><th className="px-3 py-3">预计费用</th><th className="px-3 py-3">最近生图</th><th className="px-3 py-3">操作</th></tr></thead>
                <tbody className={tableBodyClassName}>{data.records.map((user) => <UserRow canViewCosts={canViewCosts} key={user.userId} user={user} onOpen={() => setSelectedUserId(user.userId)} />)}</tbody>
              </table>
            </TableShell>
            <Pagination disabled={status === 'loading'} onChange={(next) => updateQuery({ pageNum: next }, false)} pageNum={data.pageNum} pageSize={data.pageSize} total={data.total} />
          </>
        )}
      </SectionCard>

      <UserDetailDrawer baseQueryKey={JSON.stringify(baseQuery)} canDisable={hasPermission('user:disable')} canManageRoles={hasPermission('role:manage')} canViewCosts={canViewCosts} csrfToken={session.csrfToken ?? ''} onChanged={() => void reload()} onClose={() => setSelectedUserId(null)} userId={selectedUserId} />
      <CreateUserDrawer csrfToken={session.csrfToken ?? ''} isOpen={createOpen} onClose={() => setCreateOpen(false)} onCreated={() => { setCreateOpen(false); void reload() }} />
    </div>
  )
}

function UserRow({ user, canViewCosts, onOpen }: { user: AnalyticsUserRecord; canViewCosts: boolean; onOpen: () => void }) {
  return <tr><td className="max-w-[250px] px-3 py-3"><div className="flex items-center gap-2"><span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-blue-50 text-blue-700"><UserRound className="h-4 w-4" /></span><span className="min-w-0"><span className="block truncate font-semibold text-slate-900">{user.displayName || '用户名称未设置'}</span><span className="block truncate text-xs text-slate-500">{user.email}</span></span></div></td><td className="px-3 py-3"><StatusBadge label={lifecycleLabel(user.lifecycle)} tone={lifecycleTone(user.lifecycle)} /></td><td className="whitespace-nowrap px-3 py-3 text-xs text-slate-600">{formatDateTime(user.lastLoginAt)}</td><td className="px-3 py-3 text-slate-600">{formatCompactNumber(user.activeDays, '天')}</td><td className="px-3 py-3"><p className="font-semibold text-slate-800" title={formatExactNumber(user.taskCount, '个任务')}>{formatCompactNumber(user.taskCount, '个')}</p><p className="text-xs text-slate-500" title={formatExactNumber(user.outputCount, '张出图')}>{formatCompactNumber(user.outputCount, '张出图')}</p></td><td className="px-3 py-3 font-semibold text-slate-800">{user.taskCount > 0 ? formatPercentage(user.successRate) : '暂无'}</td><td className="px-3 py-3"><CostValues canView={canViewCosts} costs={user.costs} /></td><td className="whitespace-nowrap px-3 py-3 text-xs text-slate-600">{formatDateTime(user.lastTaskAt)}</td><td className="px-3 py-3"><Button onClick={onOpen} variant="secondary">查看用户</Button></td></tr>
}

function UserDetailDrawer({ userId, baseQueryKey, csrfToken, canDisable, canManageRoles, canViewCosts, onClose, onChanged }: { userId: string | null; baseQueryKey: string; csrfToken: string; canDisable: boolean; canManageRoles: boolean; canViewCosts: boolean; onClose: () => void; onChanged: () => void }) {
  const sequenceRef = useRef(0)
  const [analytics, setAnalytics] = useState<AnalyticsUserDetailResponse | null>(null)
  const [adminUser, setAdminUser] = useState<UserAdminUser | null>(null)
  const [roles, setRoles] = useState<UserAdminRole[]>([])
  const [roleIds, setRoleIds] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  const [saving, setSaving] = useState(false)
  const [reloadToken, setReloadToken] = useState(0)
  const [actionFeedback, setActionFeedback] = useState<{ message: string; tone: 'success' | 'error' } | null>(null)

  useEffect(() => {
    if (!userId) {
      sequenceRef.current += 1
      return
    }
    const sequence = sequenceRef.current + 1
    sequenceRef.current = sequence
    const baseQuery = JSON.parse(baseQueryKey) as ReturnType<typeof useAdminAnalyticsQuery>
    setLoading(true); setError(false); setAnalytics(null); setAdminUser(null)
    void Promise.all([adminApi.getAnalyticsUser(userId, baseQuery), userAdminApi.getUser(userId), userAdminApi.listRoles().catch(() => [])])
      .then(([analyticsResponse, userResponse, roleResponse]) => {
        if (sequenceRef.current !== sequence) return
        setAnalytics(analyticsResponse); setAdminUser(userResponse); setRoles(roleResponse); setRoleIds(userResponse.roles.map((role) => role.id))
      })
      .catch(() => { if (sequenceRef.current === sequence) setError(true) })
      .finally(() => { if (sequenceRef.current === sequence) setLoading(false) })
  }, [baseQueryKey, reloadToken, userId])

  const toggleStatus = async () => {
    if (!adminUser) return
    setSaving(true); setActionFeedback(null)
    try {
      const updated = adminUser.status === 'ACTIVE' ? await userAdminApi.disableUser(adminUser.id, csrfToken) : await userAdminApi.enableUser(adminUser.id, csrfToken)
      setAdminUser(updated); setActionFeedback({ message: `用户已${updated.status === 'ACTIVE' ? '启用' : '停用'}。`, tone: 'success' }); onChanged()
    } catch {
      setActionFeedback({ message: '用户状态修改失败，现有状态未改变，请稍后重试。', tone: 'error' })
    } finally { setSaving(false) }
  }

  const saveRoles = async () => {
    if (!adminUser) return
    setSaving(true); setActionFeedback(null)
    try {
      const updated = await userAdminApi.replaceUserRoles(adminUser.id, { roleIds }, csrfToken)
      setAdminUser(updated); setActionFeedback({ message: '用户角色已保存。', tone: 'success' }); onChanged()
    } catch {
      setActionFeedback({ message: '用户角色保存失败，原有角色未改变，请稍后重试。', tone: 'error' })
    } finally { setSaving(false) }
  }

  const chartOption = useMemo<EChartsCoreOption>(() => ({
    tooltip: { trigger: 'axis' }, legend: { bottom: 0 }, grid: { left: 42, right: 16, top: 16, bottom: 52 },
    xAxis: { type: 'category', data: analytics?.trend.map((point) => formatDateLabel(point.bucket)) ?? [] }, yAxis: { type: 'value', minInterval: 1, name: '数量', splitLine: { lineStyle: { color: ADMIN_CHART_TOKENS.grid } } },
    series: [{ name: '生图任务', type: 'line', smooth: true, data: analytics?.trend.map((point) => point.taskCount) ?? [] }, { name: '实际出图', type: 'bar', data: analytics?.trend.map((point) => point.outputCount) ?? [] }],
  }), [analytics])

  return (
    <EditorDrawer isOpen={userId !== null} onClose={onClose} title="用户详情" widthClass="max-w-3xl">
      {loading ? <LoadingState moduleName="用户详情" /> : error ? <ErrorState moduleName="用户详情" onRetry={() => setReloadToken((current) => current + 1)} /> : analytics && adminUser ? (
        <div className="space-y-5">
          <section className="rounded-xl bg-slate-950 p-4 text-white"><div className="flex flex-wrap items-start justify-between gap-3"><div><p className="text-lg font-bold">{analytics.user.displayName || '用户名称未设置'}</p><p className="mt-1 text-sm text-slate-300">{analytics.user.email}</p><div className="mt-3 flex flex-wrap gap-2"><StatusBadge label={entityStatusLabel(adminUser.status)} value={adminUser.status} /><StatusBadge label={lifecycleLabel(analytics.user.lifecycle)} tone={lifecycleTone(analytics.user.lifecycle)} /></div></div>{canDisable ? <Button disabled={saving} onClick={() => void toggleStatus()} variant={adminUser.status === 'ACTIVE' ? 'danger' : 'secondary'}>{adminUser.status === 'ACTIVE' ? '停用用户' : '启用用户'}</Button> : null}</div></section>
          <ActionFeedback feedback={actionFeedback} />
          <section className="grid grid-cols-2 gap-2 sm:grid-cols-4"><MiniMetric label="活跃天数" value={formatCompactNumber(analytics.user.activeDays, '天')} /><MiniMetric label="生图任务" value={formatCompactNumber(analytics.user.taskCount, '个')} /><MiniMetric label="实际出图" value={formatCompactNumber(analytics.user.outputCount, '张')} /><MiniMetric label="任务成功率" value={analytics.user.taskCount ? formatPercentage(analytics.user.successRate) : '暂无'} /></section>
          <section><h3 className="text-sm font-bold text-slate-900">个人生图趋势</h3>{analytics.trend.length ? <AdminChart title="个人生图趋势" description="按日期展示该用户的生图任务和实际出图。" option={chartOption} height={260} /> : <EmptyState message="该用户在当前范围暂无生图记录。" />}</section>
          <div className="grid gap-4 lg:grid-cols-2"><BreakdownList title="常用项目" rows={analytics.projects} /><BreakdownList title="常用模型" rows={analytics.models} /></div>
          <section><h3 className="mb-2 text-sm font-bold text-slate-900">预计费用</h3><CostValues canView={canViewCosts} costs={analytics.user.costs} /></section>
          <section><h3 className="mb-2 text-sm font-bold text-slate-900">角色与权限</h3><div className="space-y-2 rounded-lg border border-slate-200 p-3">{roles.map((role) => <label className="flex min-h-10 items-start gap-3 rounded-lg p-2 hover:bg-slate-50" key={role.id}><input checked={roleIds.includes(role.id)} className="mt-1 h-4 w-4 accent-amazon-500" disabled={!canManageRoles || saving} onChange={(event) => setRoleIds((current) => event.target.checked ? [...current, role.id] : current.filter((id) => id !== role.id))} type="checkbox" /><span><span className="block text-sm font-semibold text-slate-800">{role.name || roleLabel(role.code)}</span><span className="block text-xs leading-5 text-slate-500">{role.description || '该角色暂无说明。'}</span></span></label>)}{canManageRoles ? <Button disabled={saving} icon={<ShieldCheck className="h-4 w-4" />} onClick={() => void saveRoles()} variant="primary">保存角色</Button> : null}</div></section>
          {analytics.auditVisible ? <section><h3 className="mb-2 text-sm font-bold text-slate-900">最近失败任务</h3>{analytics.failedTasks.length ? <div className="space-y-2">{analytics.failedTasks.map((task) => <article className="rounded-lg border border-rose-100 bg-rose-50 p-3" key={task.taskId}><div className="flex justify-between gap-3"><p className="text-sm font-semibold text-rose-900">{task.projectName} · {task.modelName}</p><span className="text-xs text-rose-700">{formatDateTime(task.createdAt)}</span></div><p className="mt-1 text-xs text-rose-700">任务失败；可到“任务与调用”查看中文异常分类和技术详情。</p></article>)}</div> : <p className="text-sm text-slate-500">当前范围内没有失败任务。</p>}</section> : null}
          <details className="rounded-lg border border-slate-200"><summary className="cursor-pointer px-3 py-2 text-xs font-semibold text-slate-600">技术详情</summary><div className="border-t border-slate-200 p-3"><p className="text-xs text-slate-500">用户ID（用于技术排查）</p><div className="mt-1 flex items-center gap-2"><code className="min-w-0 flex-1 truncate rounded bg-slate-100 px-2 py-1 text-xs">{analytics.user.userId}</code><Button aria-label="复制用户ID" icon={<Copy className="h-4 w-4" />} onClick={() => void navigator.clipboard?.writeText(analytics.user.userId)} variant="ghost">复制</Button></div></div></details>
        </div>
      ) : null}
    </EditorDrawer>
  )
}

function CreateUserDrawer({ isOpen, csrfToken, onClose, onCreated }: { isOpen: boolean; csrfToken: string; onClose: () => void; onCreated: () => void }) {
  const [roles, setRoles] = useState<UserAdminRole[]>([])
  const [email, setEmail] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [roleIds, setRoleIds] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!isOpen) return
    void userAdminApi.listRoles().then(setRoles).catch(() => setError('角色列表暂时无法加载，仍可创建不分配角色的用户。'))
  }, [isOpen])

  const submit = async (event: FormEvent) => {
    event.preventDefault(); setError('')
    if (!email.trim() || !displayName.trim() || password.length < 12) { setError('请填写名称和邮箱，并设置至少12位的初始密码。'); return }
    setSaving(true)
    try {
      await userAdminApi.createUser({ email: email.trim(), displayName: displayName.trim(), password, roleIds }, csrfToken)
      setEmail(''); setDisplayName(''); setPassword(''); setRoleIds([]); onCreated()
    } catch { setError('用户创建失败，请检查邮箱是否已存在以及输入内容是否符合要求。') } finally { setSaving(false) }
  }

  return <EditorDrawer isOpen={isOpen} onClose={onClose} title="新建用户"><form className="space-y-4" onSubmit={(event) => void submit(event)}><p className="rounded-lg bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-800">用户创建后可立即登录。初始密码不会在创建后再次显示，请通过安全渠道交付。</p>{error ? <p className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-800" role="alert">{error}</p> : null}<label className="grid gap-1.5 text-xs font-semibold text-slate-600">用户名称<input autoComplete="name" className="field-input" onChange={(event) => setDisplayName(event.target.value)} placeholder="例如：王小明" value={displayName} /></label><label className="grid gap-1.5 text-xs font-semibold text-slate-600">登录邮箱<input autoComplete="email" className="field-input" onChange={(event) => setEmail(event.target.value)} placeholder="name@example.com" type="email" value={email} /></label><label className="grid gap-1.5 text-xs font-semibold text-slate-600">初始密码<input autoComplete="new-password" className="field-input" minLength={12} onChange={(event) => setPassword(event.target.value)} placeholder="至少12位" type="password" value={password} /></label><fieldset><legend className="text-xs font-semibold text-slate-600">分配角色（可选）</legend><div className="mt-2 space-y-1 rounded-lg border border-slate-200 p-2">{roles.map((role) => <label className="flex min-h-10 items-center gap-2 rounded p-2 hover:bg-slate-50" key={role.id}><input checked={roleIds.includes(role.id)} className="h-4 w-4 accent-amazon-500" onChange={(event) => setRoleIds((current) => event.target.checked ? [...current, role.id] : current.filter((id) => id !== role.id))} type="checkbox" /><span className="text-sm text-slate-700">{role.name || roleLabel(role.code)}</span></label>)}</div></fieldset><div className="flex justify-end gap-2 border-t border-slate-200 pt-4"><Button disabled={saving} onClick={onClose} variant="secondary">取消</Button><Button disabled={saving} icon={<Plus className="h-4 w-4" />} type="submit" variant="primary">{saving ? '创建中...' : '创建用户'}</Button></div></form></EditorDrawer>
}

function CostValues({ costs, canView = true }: { costs: Array<{ currency: string; amount: string }>; canView?: boolean }) {
  if (!canView) return <span className="text-xs text-slate-400">无费用查看权限</span>
  return costs.length ? <div className="space-y-1">{costs.map((cost) => <p className="whitespace-nowrap text-xs font-semibold text-slate-800" key={cost.currency}>{formatCurrency(cost.amount, cost.currency)}</p>)}</div> : <span className="text-xs text-slate-400">暂无费用</span>
}

function BreakdownList({ title, rows }: { title: string; rows: AnalyticsUserDetailResponse['projects'] }) {
  return <section><h3 className="mb-2 text-sm font-bold text-slate-900">{title}</h3><div className="space-y-2 rounded-lg border border-slate-200 p-3">{rows.length ? rows.slice(0, 5).map((row) => <div className="flex items-center justify-between gap-3" key={row.dimensionId}><p className="truncate text-sm font-medium text-slate-700">{row.name}</p><span className="shrink-0 text-xs text-slate-500">{formatCompactNumber(row.recordCount, '次')}</span></div>) : <p className="text-xs text-slate-500">当前范围暂无记录。</p>}</div></section>
}

function MiniMetric({ label, value }: { label: string; value: string }) {
  return <article className="rounded-lg border border-slate-200 bg-slate-50 p-3"><p className="text-[11px] font-semibold text-slate-500">{label}</p><p className="mt-1 text-lg font-bold text-slate-900">{value}</p></article>
}

function lifecycleTone(value: string): 'success' | 'warning' | 'info' | 'neutral' {
  if (value === 'NEW') return 'info'
  if (value === 'RETURNING') return 'success'
  if (value === 'RESURRECTED') return 'warning'
  return 'neutral'
}
