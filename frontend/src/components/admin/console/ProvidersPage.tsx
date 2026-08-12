import { Activity, CircleDollarSign, Clock3, Pencil, Power, RefreshCw, TestTube2, Trash2, Wrench } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { adminApi } from '../../../api/admin'
import { modelApi } from '../../../api/models'
import { providerApi } from '../../../api/providers'
import { entityStatusLabel, formatCompactNumber, formatCurrency, formatDateTime, formatDuration, formatPercentage, providerTypeLabel } from '../../../lib/adminPresentation'
import type { AnalyticsProviderHealth, AnalyticsRequestsResponse } from '../../../types/analytics'
import type { Model, Provider } from '../../../types/platform'
import { Button } from '../../ui/Button'
import { ProviderModelAdminPanel } from '../ProviderModelAdminPanel'
import { useAdminConsole } from './AdminConsoleContext'
import { EmptyState, ErrorState, LoadingState, SectionCard, StatusBadge } from './AdminUi'

type ProviderPeriod = '24h' | '7d'

export function AdminProvidersPage() {
  const { session, hasPermission } = useAdminConsole()
  const [providers, setProviders] = useState<Provider[]>([])
  const [models, setModels] = useState<Model[]>([])
  const [health24h, setHealth24h] = useState<AnalyticsRequestsResponse | null>(null)
  const [health7d, setHealth7d] = useState<AnalyticsRequestsResponse | null>(null)
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [period, setPeriod] = useState<ProviderPeriod>('24h')
  const [managementOpen, setManagementOpen] = useState(false)
  const [actionProviderId, setActionProviderId] = useState<string | null>(null)
  const [actionNotice, setActionNotice] = useState('')

  const canManageProviders = hasPermission('provider:manage')
  const canManageModels = hasPermission('model:manage')
  const canReadProviders = hasPermission('provider:read') || canManageProviders
  const canReadModels = hasPermission('model:read') || canManageModels
  const canViewHealth = hasPermission('audit:read')

  const load = useCallback(async () => {
    setStatus('loading')
    const now = new Date()
    try {
      const [providerPage, modelPage, requests24h, requests7d] = await Promise.all([
        canReadProviders ? providerApi.list({ pageNum: 1, pageSize: 100 }) : Promise.resolve(null),
        canReadModels ? modelApi.list({ pageNum: 1, pageSize: 100 }) : Promise.resolve(null),
        canViewHealth ? adminApi.getAnalyticsRequests({ from: new Date(now.getTime() - 24 * 60 * 60 * 1000).toISOString(), to: now.toISOString(), granularity: 'hour', compare: false }) : Promise.resolve(null),
        canViewHealth ? adminApi.getAnalyticsRequests({ from: new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString(), to: now.toISOString(), granularity: 'day', compare: false }) : Promise.resolve(null),
      ])
      setProviders(providerPage?.records ?? []); setModels(modelPage?.records ?? []); setHealth24h(requests24h); setHealth7d(requests7d); setStatus('success')
    } catch { setStatus('error') }
  }, [canReadModels, canReadProviders, canViewHealth])

  useEffect(() => { void load() }, [load])

  const testProvider = async (provider: Provider) => {
    setActionProviderId(provider.id); setActionNotice('')
    try {
      const result = await providerApi.test(provider.id, session.csrfToken ?? '')
      setActionNotice(`${provider.name}连接测试${result.status === 'SUCCESS' ? '成功' : '失败'}，耗时${formatDuration(result.durationMs)}。`)
      await load()
    } catch { setActionNotice(`${provider.name}连接测试未完成，请检查配置后重试。`) } finally { setActionProviderId(null) }
  }

  const toggleProvider = async (provider: Provider) => {
    if (provider.status === 'ENABLED' && !window.confirm(`确定停用“${provider.name}”吗？停用后新的生图任务将不能选择该中转站。`)) return
    setActionProviderId(provider.id); setActionNotice('')
    try {
      if (provider.status === 'ENABLED') await providerApi.disable(provider.id, session.csrfToken ?? '')
      else await providerApi.enable(provider.id, session.csrfToken ?? '')
      setActionNotice(`已${provider.status === 'ENABLED' ? '停用' : '启用'}${provider.name}。`); await load()
    } catch { setActionNotice(`${provider.name}状态修改失败，请稍后重试。`) } finally { setActionProviderId(null) }
  }

  const deleteProvider = async (provider: Provider) => {
    if (!window.confirm(`确定删除“${provider.name}”吗？该操作不可撤销；存在关联模型或任务时系统会拒绝删除。`)) return
    setActionProviderId(provider.id); setActionNotice('')
    try { await providerApi.delete(provider.id, session.csrfToken ?? ''); setActionNotice(`已删除${provider.name}。`); await load() } catch { setActionNotice(`${provider.name}无法删除，请先检查关联模型和任务。`) } finally { setActionProviderId(null) }
  }

  if (status === 'loading' && providers.length === 0) return <LoadingState moduleName="中转站与模型" />
  if (status === 'error' && providers.length === 0) return <ErrorState moduleName="中转站与模型" onRetry={() => void load()} />

  const health = period === '24h' ? health24h : health7d
  const healthMap = new Map((health?.providers ?? []).map((item) => [item.providerId, item]))

  return (
    <div className="space-y-4 sm:space-y-5">
      {actionNotice ? <div aria-live="polite" className="rounded-xl border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900" role="status">{actionNotice}</div> : null}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-200 bg-white p-3 shadow-sm"><div className="inline-flex rounded-lg bg-slate-100 p-1" aria-label="健康数据时间范围"><button className={`min-h-9 rounded-md px-3 text-xs font-semibold ${period === '24h' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-600'}`} onClick={() => setPeriod('24h')} type="button">近24小时</button><button className={`min-h-9 rounded-md px-3 text-xs font-semibold ${period === '7d' ? 'bg-white text-slate-950 shadow-sm' : 'text-slate-600'}`} onClick={() => setPeriod('7d')} type="button">近7天</button></div><div className="flex gap-2"><Button disabled={status === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${status === 'loading' ? 'animate-spin' : ''}`} />} onClick={() => void load()} variant="ghost">刷新</Button>{canManageProviders || canManageModels ? <Button icon={<Wrench className="h-4 w-4" />} onClick={() => setManagementOpen(true)} variant="primary">配置中转站与模型</Button> : null}</div></div>

      {!canReadProviders ? <EmptyState message="当前账号没有查看中转站配置的权限；下方仍会展示有权限查看的模型。" /> : providers.length === 0 ? <EmptyState message="当前租户尚未配置中转站。请先新建中转站并完成连接测试，再配置可用模型。" action={canManageProviders ? <Button onClick={() => setManagementOpen(true)} variant="primary">开始配置</Button> : null} /> : (
        <section aria-label="中转站健康卡片" className="grid gap-4 xl:grid-cols-2 2xl:grid-cols-3">
          {providers.map((provider) => <ProviderHealthCard actionPending={actionProviderId === provider.id} canManage={canManageProviders} canViewHealth={canViewHealth} health={healthMap.get(provider.id)} key={provider.id} models={models.filter((model) => model.providerId === provider.id)} onDelete={() => void deleteProvider(provider)} onEdit={() => setManagementOpen(true)} onTest={() => void testProvider(provider)} onToggle={() => void toggleProvider(provider)} period={period} provider={provider} />)}
        </section>
      )}

      <SectionCard title="模型配置概览" description="模型显示名称优先；模型ID只在配置详情中查看。" action={canManageModels ? <Button icon={<Pencil className="h-4 w-4" />} onClick={() => setManagementOpen(true)} variant="secondary">编辑模型</Button> : null}>
        {!canReadModels ? <EmptyState message="当前账号没有查看模型配置的权限。" /> : models.length === 0 ? <EmptyState message="当前没有模型配置。完成中转站配置后，请添加至少一个支持图片生成或编辑的模型。" /> : <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">{models.map((model) => <article className="rounded-lg border border-slate-200 bg-slate-50 p-3" key={model.id}><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="truncate text-sm font-semibold text-slate-900">{model.displayName || model.modelName}</p><p className="mt-1 truncate text-xs text-slate-500">{model.providerName || '中转站名称不可用'}</p></div><StatusBadge label={entityStatusLabel(model.status)} value={model.status} /></div><div className="mt-3 flex flex-wrap gap-1 text-[11px] text-slate-600">{model.supportsGenerate ? <span className="rounded bg-white px-2 py-1">支持图片生成</span> : null}{model.supportsEdit ? <span className="rounded bg-white px-2 py-1">支持图片编辑</span> : null}<span className="rounded bg-white px-2 py-1">最多 {model.maxOutputCount} 张</span></div></article>)}</div>}
      </SectionCard>

      {(canManageProviders || canManageModels) ? <ProviderModelAdminPanel canManageModels={canManageModels} canManageProviders={canManageProviders} csrfToken={session.csrfToken} isOpen={managementOpen} onClose={() => { setManagementOpen(false); void load() }} /> : null}
    </div>
  )
}

function ProviderHealthCard({ provider, models, health, period, canManage, canViewHealth, actionPending, onEdit, onTest, onToggle, onDelete }: { provider: Provider; models: Model[]; health?: AnalyticsProviderHealth; period: ProviderPeriod; canManage: boolean; canViewHealth: boolean; actionPending: boolean; onEdit: () => void; onTest: () => void; onToggle: () => void; onDelete: () => void }) {
  const unavailable = canViewHealth ? '暂无' : '无查看权限'
  return <article className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-panel"><div className="border-b border-slate-100 p-4"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><p className="truncate text-base font-bold text-slate-950">{provider.name}</p><p className="mt-1 text-xs text-slate-500">{providerTypeLabel(provider.type)} · {models.length} 个模型</p></div><StatusBadge label={entityStatusLabel(provider.status)} value={provider.status} /></div></div><div className="grid grid-cols-2 gap-px bg-slate-200"><HealthMetric icon={Activity} label={`${period === '24h' ? '近24小时' : '近7天'}调用`} value={health ? formatCompactNumber(health.callCount, '次') : unavailable} /><HealthMetric icon={Activity} label="调用成功率" value={health && health.callCount > 0 ? formatPercentage(health.successRate) : unavailable} /><HealthMetric icon={Clock3} label="95%的调用耗时不超过" value={health && health.callCount > 0 ? formatDuration(health.p95DurationMs) : unavailable} /><HealthMetric icon={CircleDollarSign} label="预计费用" value={health?.costs.length ? health.costs.map((cost) => formatCurrency(cost.amount, cost.currency)).join(' / ') : unavailable} /></div><div className="p-4"><p className={`text-xs ${health?.lastFailureAt ? 'text-rose-700' : canViewHealth ? 'text-emerald-700' : 'text-slate-500'}`}>{!canViewHealth ? '需要“查看审计记录”权限才能查看运行健康数据' : health?.lastFailureAt ? `最近发生过调用失败：${formatDateTime(health.lastFailureAt)}` : '当前时间范围内未发现调用异常'}</p>{canManage ? <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4"><Button disabled={actionPending} icon={<Pencil className="h-4 w-4" />} onClick={onEdit} variant="secondary">编辑</Button><Button disabled={actionPending} icon={<TestTube2 className="h-4 w-4" />} onClick={onTest} variant="secondary">测试连接</Button><Button disabled={actionPending} icon={<Power className="h-4 w-4" />} onClick={onToggle} variant="secondary">{provider.status === 'ENABLED' ? '停用' : '启用'}</Button><Button disabled={actionPending} icon={<Trash2 className="h-4 w-4" />} onClick={onDelete} variant="danger">删除</Button></div> : null}</div></article>
}

function HealthMetric({ icon: Icon, label, value }: { icon: typeof Activity; label: string; value: string }) {
  return <div className="bg-slate-50 p-3"><div className="flex items-center gap-1.5 text-[11px] font-semibold text-slate-500"><Icon className="h-3.5 w-3.5" />{label}</div><p className="mt-1.5 break-words text-sm font-bold text-slate-900">{value}</p></div>
}
