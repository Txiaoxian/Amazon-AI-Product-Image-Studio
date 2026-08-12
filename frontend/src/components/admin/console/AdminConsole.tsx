import {
  Activity,
  BarChart3,
  ClipboardList,
  FileClock,
  Settings2,
  SlidersHorizontal,
  Users,
} from 'lucide-react'
import { lazy, Suspense, useCallback, useEffect, useMemo, useState, type ComponentType } from 'react'
import type { AuthSession } from '../../../types/auth'
import type { AnalyticsGranularity } from '../../../types/analytics'
import { primaryRouteFromPathname } from '../../../types/navigation'
import { AuthStatus } from '../../auth/AuthStatus'
import { WorkspaceShell } from '../../layout/WorkspaceShell'
import { Button } from '../../ui/Button'
import { AdminConsoleContext, type AdminRouteSnapshot } from './AdminConsoleContext'

const AdminOverviewPage = lazy(() => import('./OverviewPage').then((module) => ({ default: module.AdminOverviewPage })))
const AdminUsagePage = lazy(() => import('./UsagePage').then((module) => ({ default: module.AdminUsagePage })))
const AdminUsersPage = lazy(() => import('./UsersPage').then((module) => ({ default: module.AdminUsersPage })))
const AdminRequestsPage = lazy(() => import('./RequestsPage').then((module) => ({ default: module.AdminRequestsPage })))
const AdminProvidersPage = lazy(() => import('./ProvidersPage').then((module) => ({ default: module.AdminProvidersPage })))
const AdminAuditPage = lazy(() => import('./AuditPage').then((module) => ({ default: module.AdminAuditPage })))
const AdminSettingsPage = lazy(() => import('./SettingsPage').then((module) => ({ default: module.AdminSettingsPage })))

interface AdminConsoleProps {
  session: AuthSession
  isAuthSubmitting: boolean
  onLogout: () => Promise<void>
  onWorkspaceNavigate?: (pathname: string) => void
}

interface NavigationItem {
  path: string
  label: string
  description: string
  icon: ComponentType<{ className?: string }>
  visible: (permissions: Set<string>) => boolean
}

const navigationItems: NavigationItem[] = [
  { path: '/admin/overview', label: '经营总览', description: '核心经营指标与异常', icon: BarChart3, visible: (p) => p.has('usage:read') },
  { path: '/admin/usage', label: '用量与费用', description: '费用构成与定价覆盖', icon: Activity, visible: (p) => p.has('usage:read') },
  { path: '/admin/users', label: '用户与活跃', description: '用户表现与生命周期', icon: Users, visible: (p) => p.has('user:read') },
  { path: '/admin/requests', label: '任务与调用', description: '生图任务与模型调用', icon: ClipboardList, visible: (p) => p.has('audit:read') },
  { path: '/admin/providers', label: '中转站与模型', description: '线路健康与模型配置', icon: SlidersHorizontal, visible: (p) => p.has('provider:read') || p.has('provider:manage') || p.has('model:read') || p.has('model:manage') },
  { path: '/admin/audit', label: '操作审计', description: '谁在何时修改了什么', icon: FileClock, visible: (p) => p.has('audit:read') },
  { path: '/admin/settings', label: '系统设置', description: '上传、并发、存储与日志', icon: Settings2, visible: (p) => p.has('system:settings:manage') },
]

const pageTitles: Record<string, { title: string; subtitle: string }> = {
  '/admin/overview': { title: '经营总览', subtitle: '从出图、活跃、成功率和费用快速判断平台运行情况' },
  '/admin/usage': { title: '用量与费用', subtitle: '按用户、项目、中转站、模型或图片类型解释预计费用' },
  '/admin/users': { title: '用户与活跃', subtitle: '查看用户生图表现、生命周期和最近活跃情况' },
  '/admin/requests': { title: '任务与调用', subtitle: '区分生图任务与模型调用，定位失败和性能问题' },
  '/admin/providers': { title: '中转站与模型', subtitle: '将配置状态与近期开销、成功率和异常放在一起判断' },
  '/admin/audit': { title: '操作审计', subtitle: '优先回答谁、什么时候、修改了什么' },
  '/admin/settings': { title: '系统设置', subtitle: '用可读单位管理上传、默认模型、并发、存储和日志保留' },
}

export function AdminConsole({ session, isAuthSubmitting, onLogout, onWorkspaceNavigate = (pathname) => window.location.assign(pathname) }: AdminConsoleProps) {
  const tenantAdmin = session.roles.some((role) => role.code === 'admin')
  const permissionSet = useMemo(() => new Set<string>(tenantAdmin ? session.permissions : []), [session.permissions, tenantAdmin])
  const hasPermission = useCallback((permission: string) => permissionSet.has(permission), [permissionSet])
  const availableNavigation = useMemo(() => navigationItems.filter((item) => item.visible(permissionSet)), [permissionSet])
  const [route, setRoute] = useState<AdminRouteSnapshot>(() => currentRoute())

  useEffect(() => {
    const onPopState = () => setRoute(currentRoute())
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  useEffect(() => {
    if (availableNavigation.length === 0) return
    const isRoot = route.pathname === '/admin' || route.pathname === '/admin/'
    const isKnownButForbidden = route.pathname in pageTitles && !availableNavigation.some((item) => item.path === route.pathname)
    if (!isRoot && !isKnownButForbidden) return
    const fallback = availableNavigation[0].path
    window.history.replaceState({}, '', `${fallback}?${route.searchParams.toString()}`.replace(/\?$/, ''))
    setRoute(currentRoute())
  }, [availableNavigation, route.pathname, route.searchParams])

  const updateRoute = useCallback((pathname: string, params: URLSearchParams, replace: boolean) => {
    const nextUrl = `${pathname}${params.size > 0 ? `?${params.toString()}` : ''}`
    window.history[replace ? 'replaceState' : 'pushState']({}, '', nextUrl)
    setRoute({ pathname, searchParams: params })
  }, [])

  const updateQuery = useCallback(
    (patch: Record<string, string | number | boolean | null | undefined>, replace = false) => {
      const params = new URLSearchParams(route.searchParams)
      applyQueryPatch(params, patch)
      updateRoute(route.pathname, params, replace)
    },
    [route.pathname, route.searchParams, updateRoute],
  )

  const navigate = useCallback(
    (pathname: string, queryPatch: Record<string, string | number | boolean | null | undefined> = {}) => {
      const params = new URLSearchParams()
      for (const key of ['from', 'to', 'granularity', 'compare']) {
        const value = route.searchParams.get(key)
        if (value) params.set(key, value)
      }
      applyQueryPatch(params, queryPatch)
      updateRoute(pathname, params, false)
    },
    [route.searchParams, updateRoute],
  )

  const defaults = useMemo(() => defaultDateRange(), [])
  const dateFrom = route.searchParams.get('from') ?? defaults.from
  const dateTo = route.searchParams.get('to') ?? defaults.to
  const compare = route.searchParams.get('compare') !== 'false'
  const granularity = normalizeGranularity(route.searchParams.get('granularity'))

  useEffect(() => {
    if (route.searchParams.has('from') && route.searchParams.has('to') && route.searchParams.has('granularity') && route.searchParams.has('compare')) return
    const params = new URLSearchParams(route.searchParams)
    if (!params.has('from')) params.set('from', defaults.from)
    if (!params.has('to')) params.set('to', defaults.to)
    if (!params.has('granularity')) params.set('granularity', 'day')
    if (!params.has('compare')) params.set('compare', 'true')
    updateRoute(route.pathname, params, true)
  }, [defaults.from, defaults.to, route.pathname, route.searchParams, updateRoute])

  const setDateRange = useCallback((from: string, to: string) => updateQuery({ from, to, pageNum: null }, true), [updateQuery])
  const setCompare = useCallback((value: boolean) => updateQuery({ compare: value }, true), [updateQuery])

  const contextValue = useMemo(
    () => ({
      session,
      route,
      analyticsQuery: { from: dateFrom, to: dateTo, granularity, compare },
      dateFrom,
      dateTo,
      compare,
      hasPermission,
      navigate,
      updateQuery,
      setDateRange,
      setCompare,
      onLogout,
    }),
    [compare, dateFrom, dateTo, granularity, hasPermission, navigate, onLogout, route, session, setCompare, setDateRange, updateQuery],
  )

  const activeNavigation = availableNavigation.find((item) => item.path === route.pathname)
  const pageTitle = pageTitles[route.pathname]
  const canViewPage = Boolean(activeNavigation)
  const analyticsNavigation = availableNavigation.filter((item) => !isSettingsNavigation(item.path))
  const settingsNavigation = availableNavigation.filter((item) => isSettingsNavigation(item.path))
  const canViewAnalytics = analyticsNavigation.length > 0
  const canViewSettings = settingsNavigation.length > 0
  const analyticsPathname = analyticsNavigation[0]?.path ?? '/admin/overview'
  const settingsPathname = settingsNavigation[0]?.path ?? '/admin/settings'
  const showGlobalDateFilter = primaryRouteFromPathname(route.pathname) === 'analytics'
  const contextualNavigation = showGlobalDateFilter ? analyticsNavigation : settingsNavigation
  const handleWorkspaceNavigate = (pathname: string) => {
    if (pathname.startsWith('/admin')) navigate(pathname)
    else onWorkspaceNavigate(pathname)
  }

  return (
    <AdminConsoleContext.Provider value={contextValue}>
      <WorkspaceShell
        accountSlot={<AuthStatus isSubmitting={isAuthSubmitting} onLogout={onLogout} session={session} variant="compact" />}
        activeRoute={primaryRouteFromPathname(route.pathname)}
        analyticsPathname={analyticsPathname}
        canViewAnalytics={canViewAnalytics}
        canViewSettings={canViewSettings}
        onNavigate={handleWorkspaceNavigate}
        settingsPathname={settingsPathname}
      >
        <div className="workspace-page admin-workspace-page">
          <div className="workspace-page-inner">
            <header className="workspace-page-header admin-page-header">
              <div className="min-w-[180px] flex-1">
                <p className="text-xs font-semibold text-amazon-600">{primaryRouteFromPathname(route.pathname) === 'analytics' ? '数据看板' : '平台设置'}</p>
                <h1 className="workspace-page-title">{pageTitle?.title ?? '管理控制台'}</h1>
                <p className="workspace-page-description">{pageTitle?.subtitle ?? '当前页面不存在或没有访问权限'}</p>
              </div>
              {showGlobalDateFilter ? (
                <GlobalDateFilter dateFrom={dateFrom} dateTo={dateTo} compare={compare} granularity={granularity} onDateRangeChange={setDateRange} onCompareChange={setCompare} onGranularityChange={(value) => updateQuery({ granularity: value }, true)} />
              ) : null}
            </header>

            <div className="admin-content-layout">
              <nav aria-label="管理页面导航" className="admin-secondary-nav">
                <p className="admin-secondary-nav-title">{primaryRouteFromPathname(route.pathname) === 'analytics' ? '数据分析' : '配置管理'}</p>
                <div className="grid gap-1.5">
                  {contextualNavigation.map((item) => {
                    const Icon = item.icon
                    const active = item.path === route.pathname
                    return (
                      <button
                        aria-current={active ? 'page' : undefined}
                        className={`admin-secondary-nav-item ${active ? 'is-active' : ''}`}
                        key={item.path}
                        onClick={() => navigate(item.path)}
                        type="button"
                      >
                        <Icon className="h-5 w-5 shrink-0" />
                        <span className="min-w-0">
                          <span className="block text-sm font-semibold">{item.label}</span>
                          <span className="mt-0.5 block truncate text-[11px]">{item.description}</span>
                        </span>
                      </button>
                    )
                  })}
                </div>
              </nav>

              <main className="min-w-0" id="admin-main">
                {canViewPage ? (
                  <Suspense fallback={<div aria-busy="true" className="flex min-h-64 items-center justify-center rounded-xl border border-slate-200 bg-white text-sm font-semibold text-slate-600" role="status">正在加载当前管理模块…</div>}>
                    <AdminPage pathname={route.pathname} />
                  </Suspense>
                ) : <AdminAccessState hasAnyPage={availableNavigation.length > 0} onBack={() => onWorkspaceNavigate('/studio')} />}
              </main>
            </div>
          </div>
        </div>
      </WorkspaceShell>
    </AdminConsoleContext.Provider>
  )
}

function AdminPage({ pathname }: { pathname: string }) {
  switch (pathname) {
    case '/admin/overview': return <AdminOverviewPage />
    case '/admin/usage': return <AdminUsagePage />
    case '/admin/users': return <AdminUsersPage />
    case '/admin/requests': return <AdminRequestsPage />
    case '/admin/providers': return <AdminProvidersPage />
    case '/admin/audit': return <AdminAuditPage />
    case '/admin/settings': return <AdminSettingsPage />
    default: return null
  }
}

function GlobalDateFilter({
  dateFrom,
  dateTo,
  compare,
  granularity,
  onDateRangeChange,
  onCompareChange,
  onGranularityChange,
}: {
  dateFrom: string
  dateTo: string
  compare: boolean
  granularity: AnalyticsGranularity
  onDateRangeChange: (from: string, to: string) => void
  onCompareChange: (compare: boolean) => void
  onGranularityChange: (granularity: AnalyticsGranularity) => void
}) {
  const applyPreset = (days: number) => onDateRangeChange(addDateDays(dateToInShanghai(), -(days - 1)), dateToInShanghai())
  return (
    <div className="grid w-full min-w-0 grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2 rounded-xl border border-slate-200 bg-slate-50 p-2 lg:flex lg:w-auto lg:flex-wrap" aria-label="全局时间范围">
      <div className="hidden gap-1 xl:flex">
        {[7, 30, 90].map((days) => <button className="min-h-9 rounded-lg px-2.5 text-xs font-semibold text-slate-600 hover:bg-white hover:text-slate-950 focus:outline-none focus:ring-2 focus:ring-amazon-500" key={days} onClick={() => applyPreset(days)} type="button">近{days}天</button>)}
      </div>
      <label className="sr-only" htmlFor="admin-date-from">开始日期</label>
      <input className="min-h-9 min-w-0 w-full rounded-lg border border-slate-200 bg-white px-2 text-xs text-slate-800 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20 lg:w-[126px]" id="admin-date-from" max={dateTo} onChange={(event) => onDateRangeChange(event.target.value, dateTo)} type="date" value={dateFrom} />
      <span className="text-xs text-slate-400">至</span>
      <label className="sr-only" htmlFor="admin-date-to">结束日期</label>
      <input className="min-h-9 min-w-0 w-full rounded-lg border border-slate-200 bg-white px-2 text-xs text-slate-800 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20 lg:w-[126px]" id="admin-date-to" min={dateFrom} onChange={(event) => onDateRangeChange(dateFrom, event.target.value)} type="date" value={dateTo} />
      <label className="sr-only" htmlFor="admin-granularity">时间粒度</label>
      <select className="min-h-9 min-w-0 w-full rounded-lg border border-slate-200 bg-white px-2 text-xs text-slate-700 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20 lg:w-auto" id="admin-granularity" onChange={(event) => onGranularityChange(event.target.value as AnalyticsGranularity)} value={granularity}>
        <option value="hour">按小时</option><option value="day">按天</option><option value="week">按周</option>
      </select>
      <label className="col-span-2 flex min-h-9 cursor-pointer items-center justify-center gap-2 rounded-lg px-2 text-xs font-medium text-slate-600 hover:bg-white lg:col-auto lg:justify-start"><input checked={compare} className="h-4 w-4 accent-amazon-500" onChange={(event) => onCompareChange(event.target.checked)} type="checkbox" />对比上一周期</label>
    </div>
  )
}

function AdminAccessState({ hasAnyPage, onBack }: { hasAnyPage: boolean; onBack: () => void }) {
  return (
    <div className="mx-auto mt-20 max-w-xl rounded-2xl border border-slate-200 bg-white p-8 text-center shadow-sm">
      <h2 className="text-lg font-bold text-slate-950">{hasAnyPage ? '当前页面不可访问' : '暂无管理控制台权限'}</h2>
      <p className="mt-3 text-sm leading-6 text-slate-600">{hasAnyPage ? '请从左侧选择有权限的管理模块，或联系租户管理员补充权限。' : '当前账号没有运营管理权限，可返回生图工作台继续使用。'}</p>
      <Button className="mt-5" onClick={onBack} variant="primary">返回创作室</Button>
    </div>
  )
}

function isSettingsNavigation(pathname: string) {
  return pathname === '/admin/providers' || pathname === '/admin/settings'
}

function currentRoute(): AdminRouteSnapshot {
  return { pathname: window.location.pathname, searchParams: new URLSearchParams(window.location.search) }
}

function applyQueryPatch(params: URLSearchParams, patch: Record<string, string | number | boolean | null | undefined>) {
  Object.entries(patch).forEach(([key, value]) => {
    if (value === null || value === undefined || value === '') params.delete(key)
    else params.set(key, String(value))
  })
}

function normalizeGranularity(value: string | null): AnalyticsGranularity {
  return value === 'hour' || value === 'week' ? value : 'day'
}

function defaultDateRange() {
  const to = dateToInShanghai()
  return { from: addDateDays(to, -29), to }
}

function dateToInShanghai() {
  const parts = new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Shanghai' }).formatToParts(new Date())
  const get = (type: Intl.DateTimeFormatPartTypes) => parts.find((part) => part.type === type)?.value ?? ''
  return `${get('year')}-${get('month')}-${get('day')}`
}

function addDateDays(value: string, days: number) {
  const date = new Date(`${value}T00:00:00+08:00`)
  date.setUTCDate(date.getUTCDate() + days)
  return new Intl.DateTimeFormat('en-CA', { year: 'numeric', month: '2-digit', day: '2-digit', timeZone: 'Asia/Shanghai' }).format(date)
}
