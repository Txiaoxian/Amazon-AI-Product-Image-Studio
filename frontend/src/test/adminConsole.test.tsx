import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { adminApi } from '../api/admin'
import { modelApi } from '../api/models'
import { projectApi } from '../api/projects'
import { providerApi } from '../api/providers'
import { userAdminApi } from '../api/userAdmin'
import { AdminConsole } from '../components/admin/console/AdminConsole'
import type { AuthSession } from '../types/auth'
import type { AnalyticsOverviewResponse } from '../types/analytics'
import type { PermissionKey, RoleId, TenantId, UserId } from '../types/platform'

vi.mock('../components/admin/console/AdminChart', () => ({
  AdminChart: ({ title, description }: { title: string; description: string }) => (
    <figure aria-label={title}><figcaption>{description}</figcaption></figure>
  ),
}))

const overview: AnalyticsOverviewResponse = {
  meta: {
    from: '2026-08-04T16:00:00Z',
    to: '2026-08-11T16:00:00Z',
    timezone: 'Asia/Shanghai',
    granularity: 'day',
    costType: 'ESTIMATED',
    generatedAt: '2026-08-11T06:30:00Z',
  },
  current: {
    taskCount: 400,
    outputCount: 328,
    terminalTaskCount: 390,
    succeededTaskCount: 351,
    taskSuccessRate: 0.9,
    activeUserCount: 46,
    loginActiveUserCount: 52,
    p95DurationMs: 18_400,
  },
  previous: {
    taskCount: 350,
    outputCount: 292,
    terminalTaskCount: 340,
    succeededTaskCount: 289,
    taskSuccessRate: 0.85,
    activeUserCount: 40,
    loginActiveUserCount: 48,
    p95DurationMs: 20_000,
  },
  changes: {
    taskCount: 14.3,
    outputCount: 12.3,
    taskSuccessRate: 5.9,
    activeUserCount: 15,
    loginActiveUserCount: 8.3,
    p95DurationMs: -8,
  },
  costs: [{ currency: 'USD', amount: '3.68000000', previousAmount: '3.10000000', changePercent: 18.7, recordCount: 100, pricedRecordCount: 88, pricingCoverage: 0.88 }],
  trend: [{ bucket: '2026-08-11T00:00:00+08:00', taskCount: 58, outputCount: 47, succeededCount: 50, failedCount: 4, timedOutCount: 2, cancelledCount: 1, activeUserCount: 18, loginActiveUsers: 20 }],
  costTrend: [{ bucket: '2026-08-11T00:00:00+08:00', currency: 'USD', estimatedCost: '0.52000000' }],
  rankings: [
    { dimension: 'user', dimensionId: 'user_internal_1', name: '王小明', taskCount: 28, outputCount: 24, successRate: 0.92, costs: [] },
    { dimension: 'project', dimensionId: 'project_internal_1', name: '秋季新品', taskCount: 36, outputCount: 31, successRate: 0.91, costs: [] },
    { dimension: 'provider', dimensionId: 'provider_internal_1', name: '上海中转站', taskCount: 80, outputCount: 70, successRate: 0.94, costs: [] },
    { dimension: 'model', dimensionId: 'model_internal_1', name: '商品图模型', taskCount: 70, outputCount: 63, successRate: 0.95, costs: [] },
  ],
  errorGroups: [],
}

function session(permissions: string[], roleCode = 'admin'): AuthSession {
  return {
    user: { id: 'user_admin' as UserId, email: 'admin@example.com', displayName: '运营管理员', status: 'ACTIVE', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' },
    tenant: { id: 'tenant_1' as TenantId, name: '商品图团队', status: 'ACTIVE' },
    roles: [{ id: `role_${roleCode}` as RoleId, code: roleCode, name: roleCode === 'admin' ? '管理员' : '成员' }],
    permissions: permissions as PermissionKey[],
    csrfToken: 'csrf_test',
  }
}

function openOverview() {
  window.history.replaceState({}, '', '/admin/overview?from=2026-08-05&to=2026-08-11&granularity=day&compare=true')
}

describe('新版管理控制台', () => {
  afterEach(() => {
    cleanup()
    window.history.replaceState({}, '', '/')
    vi.restoreAllMocks()
  })

  it('用简体中文、可读单位和名称优先方式展示经营总览', async () => {
    openOverview()
    const overviewSpy = vi.spyOn(adminApi, 'getAnalyticsOverview').mockResolvedValue(overview)

    render(<AdminConsole isAuthSubmitting={false} onLogout={vi.fn().mockResolvedValue(undefined)} session={session(['usage:read'])} />)

    expect(await screen.findByRole('heading', { name: '经营总览' })).toBeInTheDocument()
    expect(await screen.findByText('328张')).toBeInTheDocument()
    expect(screen.getByText('46人')).toBeInTheDocument()
    expect(screen.getByText('18.4秒')).toBeInTheDocument()
    expect(screen.getByText('3.68美元')).toBeInTheDocument()
    expect(screen.getByText('较上一周期增加 12.3%')).toBeInTheDocument()
    expect(screen.getByText(/12.0% 的用量记录缺少有效定价/)).toBeInTheDocument()
    expect(screen.getByText('上海中转站')).toBeInTheDocument()
    expect(screen.getByRole('figure', { name: '实际出图与生图任务趋势' })).toHaveTextContent('按日期展示实际出图张数和生图任务数')
    expect(document.body).not.toHaveTextContent('provider_internal_1')
    expect(document.body).not.toHaveTextContent('dimensionId')
    expect(overviewSpy).toHaveBeenCalledWith(expect.objectContaining({ from: '2026-08-05', to: '2026-08-11', granularity: 'day', compare: true }))
  })

  it('把全局时间范围写入 URL，并保持刷新可恢复的查询条件', async () => {
    openOverview()
    vi.spyOn(adminApi, 'getAnalyticsOverview').mockResolvedValue(overview)
    render(<AdminConsole isAuthSubmitting={false} onLogout={vi.fn().mockResolvedValue(undefined)} session={session(['usage:read'])} />)
    await screen.findByText('328张')

    fireEvent.change(screen.getByLabelText('开始日期'), { target: { value: '2026-08-08' } })
    fireEvent.click(screen.getByRole('checkbox', { name: '对比上一周期' }))

    await waitFor(() => {
      const params = new URLSearchParams(window.location.search)
      expect(params.get('from')).toBe('2026-08-08')
      expect(params.get('to')).toBe('2026-08-11')
      expect(params.get('compare')).toBe('false')
    })
  })

  it('按权限提供统一一级导航和分组二级导航，并阻止普通成员直接进入管理页', async () => {
    openOverview()
    vi.spyOn(adminApi, 'getAnalyticsOverview').mockResolvedValue(overview)
    vi.spyOn(providerApi, 'list').mockResolvedValue({ records: [], total: 0, pageNum: 1, pageSize: 100 })
    vi.spyOn(modelApi, 'list').mockResolvedValue({ records: [], total: 0, pageNum: 1, pageSize: 100 })
    vi.spyOn(projectApi, 'list').mockResolvedValue({ records: [], total: 0, pageNum: 1, pageSize: 100 })
    vi.spyOn(userAdminApi, 'listUsers').mockResolvedValue({ records: [], total: 0, pageNum: 1, pageSize: 100 })
    const allPermissions = ['usage:read', 'user:read', 'audit:read', 'provider:read', 'model:read', 'system:settings:manage']
    const view = render(<AdminConsole isAuthSubmitting={false} onLogout={vi.fn().mockResolvedValue(undefined)} session={session(allPermissions)} />)
    await screen.findByText('328张')

    const workspaceNavigation = screen.getByRole('navigation', { name: '工作区导航' })
    expect(within(workspaceNavigation).getByRole('button', { name: '数据看板' })).toHaveAttribute('aria-current', 'page')
    expect(within(workspaceNavigation).getByRole('button', { name: '设置' })).toBeInTheDocument()

    const analyticsNavigation = screen.getByRole('navigation', { name: '管理页面导航' })
    for (const label of ['经营总览', '用量与费用', '用户与活跃', '任务与调用', '操作审计']) {
      expect(within(analyticsNavigation).getByText(label)).toBeInTheDocument()
    }
    expect(within(analyticsNavigation).queryByText('中转站与模型')).not.toBeInTheDocument()
    expect(within(analyticsNavigation).queryByText('系统设置')).not.toBeInTheDocument()

    fireEvent.click(within(workspaceNavigation).getByRole('button', { name: '设置' }))
    expect(await screen.findByRole('heading', { name: '中转站与模型' })).toBeInTheDocument()
    const settingsNavigation = screen.getByRole('navigation', { name: '管理页面导航' })
    expect(within(settingsNavigation).getByText('中转站与模型')).toBeInTheDocument()
    expect(within(settingsNavigation).getByText('系统设置')).toBeInTheDocument()
    expect(within(settingsNavigation).queryByText('经营总览')).not.toBeInTheDocument()

    view.unmount()
    openOverview()
    render(<AdminConsole isAuthSubmitting={false} onLogout={vi.fn().mockResolvedValue(undefined)} session={session(['usage:read'], 'seller')} />)
    expect(await screen.findByRole('heading', { name: '暂无管理控制台权限' })).toBeInTheDocument()
    expect(adminApi.getAnalyticsOverview).toHaveBeenCalledTimes(1)
  })

  it('只有用户查看权限时仍可进入用户与活跃，并明确隐藏费用', async () => {
    window.history.replaceState({}, '', '/admin/users?from=2026-08-05&to=2026-08-11&granularity=day&compare=true')
    vi.spyOn(adminApi, 'getAnalyticsUsers').mockResolvedValue({
      meta: overview.meta,
      records: [{ userId: 'user_1', displayName: '王小明', email: 'wang@example.com', status: 'ACTIVE', lastLoginAt: '', activeDays: 2, taskCount: 3, outputCount: 2, successRate: 1, costs: [], lastTaskAt: '', lifecycle: 'NEW' }],
      total: 1,
      pageNum: 1,
      pageSize: 20,
    })
    vi.spyOn(projectApi, 'list').mockResolvedValue({ records: [], total: 0, pageNum: 1, pageSize: 100 })

    render(<AdminConsole isAuthSubmitting={false} onLogout={vi.fn().mockResolvedValue(undefined)} session={session(['user:read'])} />)

    expect(await screen.findByRole('heading', { name: '用户与活跃' })).toBeInTheDocument()
    expect(await screen.findByText('王小明')).toBeInTheDocument()
    expect(screen.getByText('无费用查看权限')).toBeInTheDocument()
    const navigation = screen.getByRole('navigation', { name: '管理页面导航' })
    expect(within(navigation).getByText('用户与活跃')).toBeInTheDocument()
    expect(within(navigation).queryByText('用量与费用')).not.toBeInTheDocument()
  })
})
