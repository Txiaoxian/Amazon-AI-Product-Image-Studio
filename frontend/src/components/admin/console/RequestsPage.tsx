import { Copy, Download, RefreshCw } from 'lucide-react'
import type { EChartsCoreOption } from 'echarts/core'
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { adminApi } from '../../../api/admin'
import { modelApi } from '../../../api/models'
import { providerApi } from '../../../api/providers'
import {
  ADMIN_CHART_TOKENS,
  apiCallStatusLabel,
  buildApiCallStatusOptions,
  buildTaskStatusOptions,
  costStatusLabel,
  errorCategory,
  formatCompactNumber,
  formatCurrency,
  formatDateLabel,
  formatDateTime,
  formatDuration,
  formatPercentage,
  imageTypeLabel,
  taskStatusLabel,
  taskTypeLabel,
} from '../../../lib/adminPresentation'
import type { AnalyticsTaskRecord } from '../../../types/analytics'
import type { ApiCallLog } from '../../../types/admin'
import type { Model, Provider } from '../../../types/platform'
import { Button } from '../../ui/Button'
import { EditorDrawer } from '../../ui/EditorDrawer'
import { AdminChart } from './AdminChart'
import { AdminFilterBar } from './AdminFilterBar'
import { useAdminAnalyticsQuery, useAdminConsole } from './AdminConsoleContext'
import { ActionFeedback, EmptyState, ErrorState, KpiCard, LoadingState, Pagination, SectionCard, StatusBadge, TableShell, tableBodyClassName, tableClassName, tableHeadClassName } from './AdminUi'
import { useAdminData } from './useAdminData'
import { useAnalyticsExport } from './useAnalyticsExport'

type RequestsTab = 'tasks' | 'requests'

export function AdminRequestsPage() {
  const { route, updateQuery } = useAdminConsole()
  const tab = route.searchParams.get('tab') === 'requests' ? 'requests' : 'tasks'
  const switchTab = (next: RequestsTab) => updateQuery({ tab: next, status: null, pageNum: null }, false)
  const handleTabKey = (event: KeyboardEvent<HTMLButtonElement>, current: RequestsTab) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const next = current === 'tasks' ? 'requests' : 'tasks'
    switchTab(next)
    document.getElementById(next === 'tasks' ? 'admin-requests-tasks-tab' : 'admin-requests-calls-tab')?.focus()
  }

  return (
    <div className="space-y-4 sm:space-y-5">
      <div className="inline-flex rounded-xl border border-slate-200 bg-white p-1 shadow-sm" role="tablist" aria-label="任务与调用视图">
        <button aria-controls="admin-requests-tasks-panel" aria-selected={tab === 'tasks'} className={`min-h-10 rounded-lg px-4 text-sm font-semibold transition ${tab === 'tasks' ? 'bg-slate-950 text-white shadow-sm' : 'text-slate-600 hover:bg-slate-50'}`} id="admin-requests-tasks-tab" onClick={() => switchTab('tasks')} onKeyDown={(event) => handleTabKey(event, 'tasks')} role="tab" tabIndex={tab === 'tasks' ? 0 : -1} type="button">生图任务</button>
        <button aria-controls="admin-requests-calls-panel" aria-selected={tab === 'requests'} className={`min-h-10 rounded-lg px-4 text-sm font-semibold transition ${tab === 'requests' ? 'bg-slate-950 text-white shadow-sm' : 'text-slate-600 hover:bg-slate-50'}`} id="admin-requests-calls-tab" onClick={() => switchTab('requests')} onKeyDown={(event) => handleTabKey(event, 'requests')} role="tab" tabIndex={tab === 'requests' ? 0 : -1} type="button">模型调用</button>
      </div>
      <div aria-labelledby={tab === 'tasks' ? 'admin-requests-tasks-tab' : 'admin-requests-calls-tab'} id={tab === 'tasks' ? 'admin-requests-tasks-panel' : 'admin-requests-calls-panel'} role="tabpanel">
        {tab === 'tasks' ? <TasksTab /> : <ModelRequestsTab />}
      </div>
    </div>
  )
}

function TasksTab() {
  const { route, updateQuery } = useAdminConsole()
  const baseQuery = useAdminAnalyticsQuery()
  const pageNum = Number(route.searchParams.get('pageNum') || 1)
  const query = { ...baseQuery, pageNum, pageSize: 20 }
  const key = JSON.stringify(query)
  const { data, status, reload } = useAdminData(() => adminApi.getAnalyticsTasks(query), [key])
  const [selected, setSelected] = useState<AnalyticsTaskRecord | null>(null)
  const { exportData, exporting, feedback } = useAnalyticsExport('tasks', query)

  return (
    <div className="space-y-4">
      <AdminFilterBar statusOptions={taskStatusOptions} />
      <ActionFeedback feedback={feedback} />
      {status === 'loading' && !data ? <LoadingState moduleName="生图任务" /> : status === 'error' && !data ? <ErrorState moduleName="生图任务" onRetry={() => void reload()} /> : data ? (
        <>
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <KpiCard label="生图任务数" value={formatCompactNumber(data.summary.taskCount, '个')} description="当前筛选范围内创建的生成或编辑任务。" tone="violet" />
            <KpiCard label="实际出图张数" value={formatCompactNumber(data.summary.outputCount, '张')} description="真正持久化的任务输出数量。" tone="blue" />
            <KpiCard label="任务成功率" value={formatPercentage(data.summary.taskSuccessRate)} description="只以全部终态任务作为分母。" tone="emerald" />
            <KpiCard label="95%的任务耗时不超过" value={formatDuration(data.summary.p95DurationMs)} description="第95百分位任务执行耗时。" tone="slate" />
          </section>
          <SectionCard title="生图任务明细" description="用户、项目、中转站和模型名称优先显示；任务ID只在详情中出现。" action={<div className="flex gap-2"><Button disabled={status === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${status === 'loading' ? 'animate-spin' : ''}`} />} onClick={() => void reload()} variant="ghost">刷新</Button><Button disabled={exporting} icon={<Download className="h-4 w-4" />} onClick={() => void exportData()} variant="secondary">{exporting ? '正在导出...' : '导出中文 CSV'}</Button></div>}>
            {data.records.length === 0 ? <EmptyState message="当前时间范围和筛选条件下暂无生图任务，可扩大时间范围或清除筛选。" /> : <><TableShell label="生图任务明细"><table className={tableClassName}><thead className={tableHeadClassName}><tr><th className="px-3 py-3">创建时间</th><th className="px-3 py-3">用户 / 项目</th><th className="px-3 py-3">中转站 / 模型</th><th className="px-3 py-3">任务类型</th><th className="px-3 py-3">状态</th><th className="px-3 py-3">出图</th><th className="px-3 py-3">耗时</th><th className="px-3 py-3">预计费用</th><th className="px-3 py-3">操作</th></tr></thead><tbody className={tableBodyClassName}>{data.records.map((task) => <tr key={task.taskId}><td className="whitespace-nowrap px-3 py-3 text-xs text-slate-600">{formatDateTime(task.createdAt)}</td><td className="max-w-[220px] px-3 py-3"><p className="truncate font-semibold text-slate-900">{task.userName}</p><p className="truncate text-xs text-slate-500">{task.projectName}</p></td><td className="max-w-[220px] px-3 py-3"><p className="truncate text-sm text-slate-800">{task.providerName}</p><p className="truncate text-xs text-slate-500">{task.modelName}</p></td><td className="px-3 py-3"><p className="text-sm text-slate-800">{taskTypeLabel(task.type)}</p><p className="text-xs text-slate-500">{imageTypeLabel(task.imageType)}</p></td><td className="px-3 py-3"><StatusBadge label={taskStatusLabel(task.status)} value={task.status} /></td><td className="px-3 py-3 font-semibold text-slate-800">{formatCompactNumber(task.outputCount, '张')}</td><td className="px-3 py-3 text-slate-600">{task.durationMs > 0 ? formatDuration(task.durationMs) : '暂无'}</td><td className="px-3 py-3">{task.estimatedCost ? <><p className="whitespace-nowrap text-xs font-semibold text-slate-800">{formatCurrency(task.estimatedCost, task.currency)}</p><p className="mt-1 text-[10px] text-slate-400">{costStatusLabel(task.costStatus)}</p></> : <span className="text-xs text-slate-400">无查看权限</span>}</td><td className="px-3 py-3"><Button onClick={() => setSelected(task)} variant="secondary">查看详情</Button></td></tr>)}</tbody></table></TableShell><Pagination disabled={status === 'loading'} onChange={(page) => updateQuery({ pageNum: page }, false)} pageNum={data.pageNum} pageSize={data.pageSize} total={data.total} /></>}
          </SectionCard>
        </>
      ) : null}
      <TaskDetailDrawer onClose={() => setSelected(null)} task={selected} />
    </div>
  )
}

function ModelRequestsTab() {
  const { route, updateQuery, dateFrom, dateTo, hasPermission } = useAdminConsole()
  const baseQuery = useAdminAnalyticsQuery()
  const pageNum = Number(route.searchParams.get('pageNum') || 1)
  const query = { ...baseQuery }
  const key = JSON.stringify(query)
  const { data, status, reload } = useAdminData(() => adminApi.getAnalyticsRequests(query), [key])
  const { exportData, exporting, feedback } = useAnalyticsExport('requests', query)
  const [logs, setLogs] = useState<ApiCallLog[]>([])
  const [logTotal, setLogTotal] = useState(0)
  const [logStatus, setLogStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [selectedDetail, setSelectedDetail] = useState<ApiCallLog | null>(null)
  const [detailStatus, setDetailStatus] = useState<'idle' | 'loading' | 'success' | 'error'>('idle')
  const [detailReloadToken, setDetailReloadToken] = useState(0)
  const [logReloadToken, setLogReloadToken] = useState(0)
  const [providers, setProviders] = useState<Provider[]>([])
  const [models, setModels] = useState<Model[]>([])
  const requestSequence = useRef(0)
  const detailSequence = useRef(0)
  const isAnalyticsLoading = status === 'loading'
  const isLogLoading = logStatus === 'loading'

  useEffect(() => {
    const canReadProviders = hasPermission('provider:read') || hasPermission('provider:manage')
    const canReadModels = hasPermission('model:read') || hasPermission('model:manage')
    if (!canReadProviders && !canReadModels) return
    let active = true
    void Promise.all([canReadProviders ? providerApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null) : null, canReadModels ? modelApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null) : null]).then(([providerPage, modelPage]) => {
      if (!active) return
      setProviders(providerPage?.records ?? []); setModels(modelPage?.records ?? [])
    })
    return () => { active = false }
  }, [hasPermission])

  useEffect(() => {
    const sequence = requestSequence.current + 1
    requestSequence.current = sequence
    setLogStatus('loading')
    void adminApi.listApiCallLogs({
      pageNum, pageSize: 20, sortBy: 'createdAt', sortOrder: 'desc',
      createdAtFrom: `${dateFrom}T00:00:00+08:00`, createdAtTo: `${dateTo}T23:59:59.999+08:00`,
      userId: baseQuery.userId, projectId: baseQuery.projectId, providerId: baseQuery.providerId, modelId: baseQuery.modelId,
      imageType: baseQuery.imageType,
      status: baseQuery.status === 'SUCCESS' || baseQuery.status === 'FAILURE' ? baseQuery.status : undefined,
    }).then((page) => { if (requestSequence.current === sequence) { setLogs(page.records); setLogTotal(page.total); setLogStatus('success') } }).catch(() => { if (requestSequence.current === sequence) setLogStatus('error') })
  }, [baseQuery.imageType, baseQuery.modelId, baseQuery.projectId, baseQuery.providerId, baseQuery.status, baseQuery.userId, dateFrom, dateTo, logReloadToken, pageNum])

  useEffect(() => {
    if (!selectedId) {
      detailSequence.current += 1
      setSelectedDetail(null)
      setDetailStatus('idle')
      return
    }
    const sequence = detailSequence.current + 1
    detailSequence.current = sequence
    setSelectedDetail(null)
    setDetailStatus('loading')
    void adminApi.getApiCallLog(selectedId)
      .then((detail) => {
        if (detailSequence.current !== sequence) return
        setSelectedDetail(detail)
        setDetailStatus('success')
      })
      .catch(() => {
        if (detailSequence.current === sequence) setDetailStatus('error')
      })
  }, [detailReloadToken, selectedId])

  const providerNames = new Map(providers.map((provider) => [String(provider.id), provider.name]))
  const modelNames = new Map(models.map((model) => [String(model.id), model.displayName || model.modelName]))
  const providerNameFor = (log: ApiCallLog) => log.providerName || providerNames.get(String(log.providerId)) || '中转站名称不可用'
  const modelNameFor = (log: ApiCallLog) => log.modelName || modelNames.get(String(log.modelId)) || '模型名称不可用'
  const chartOption = buildRequestTrendOption(data?.trend ?? [])

  return (
    <div className="space-y-4">
      <AdminFilterBar statusOptions={buildApiCallStatusOptions(['SUCCESS', 'FAILURE'])} />
      <ActionFeedback feedback={feedback} />
      {status === 'loading' && !data ? <LoadingState moduleName="模型调用统计" /> : status === 'error' && !data ? <ErrorState moduleName="模型调用统计" onRetry={() => void reload()} /> : data ? (
        <>
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4"><KpiCard label="模型调用次数" value={formatCompactNumber(data.summary.callCount, '次')} description="平台通过中转站发起的全部模型调用。" tone="violet" /><KpiCard label="调用成功率" value={formatPercentage(data.summary.successRate)} description="成功模型调用数 ÷ 全部模型调用数。" tone="emerald" /><KpiCard label="一半调用耗时不超过" value={formatDuration(data.summary.p50DurationMs)} description="模型调用耗时中位数。" tone="blue" /><KpiCard label="95%的调用耗时不超过" value={formatDuration(data.summary.p95DurationMs)} description="第95百分位模型调用耗时。" tone="slate" /></section>
          <div className="grid gap-4 2xl:grid-cols-[minmax(0,1.4fr)_minmax(360px,0.6fr)]"><SectionCard title="模型调用趋势" description="成功与失败调用分开呈现，不与生图任务成功率混用。">{data.trend.length ? <AdminChart title="模型调用趋势" description="按日期展示成功和失败的模型调用次数。" option={chartOption} /> : <EmptyState message="当前范围暂无模型调用记录。" />}</SectionCard><SectionCard title="高频异常" description="按中文业务类别聚合，并统计受影响的生图任务数；原始错误码收进详情。">{data.errorGroups.length ? <div className="space-y-2">{data.errorGroups.map((item) => { const category = errorCategory(item.errorCode); return <div className="rounded-lg border border-slate-200 bg-slate-50 p-3" key={item.errorCode}><div className="flex justify-between gap-3"><p className="text-sm font-semibold text-slate-900">{category.label}</p><span className="text-xs font-bold text-rose-700">影响 {formatCompactNumber(item.count, '个任务')}</span></div><p className="mt-1 text-xs leading-5 text-slate-500">{category.guidance}</p></div> })}</div> : <EmptyState message="当前范围没有模型调用异常。" />}</SectionCard></div>
          <SectionCard title="模型调用明细" description="默认显示可理解的名称、中文状态和业务异常；技术标识和脱敏载荷位于侧边详情。" action={<div className="flex gap-2"><Button disabled={isAnalyticsLoading || logStatus === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${isAnalyticsLoading || logStatus === 'loading' ? 'animate-spin' : ''}`} />} onClick={() => { void reload(); setLogReloadToken((current) => current + 1) }} variant="ghost">刷新</Button><Button disabled={exporting} icon={<Download className="h-4 w-4" />} onClick={() => void exportData()} variant="secondary">{exporting ? '正在导出...' : '导出中文 CSV'}</Button></div>}>
            {logStatus === 'loading' ? <LoadingState moduleName="模型调用明细" /> : logStatus === 'error' ? <ErrorState moduleName="模型调用明细" onRetry={() => setLogReloadToken((current) => current + 1)} /> : logs.length === 0 ? <EmptyState message="当前时间范围和筛选条件下暂无模型调用记录。" /> : <><TableShell label="模型调用明细"><table className={tableClassName}><thead className={tableHeadClassName}><tr><th className="px-3 py-3">调用时间</th><th className="px-3 py-3">中转站 / 模型</th><th className="px-3 py-3">状态</th><th className="px-3 py-3">耗时</th><th className="px-3 py-3">异常类别</th><th className="px-3 py-3">操作</th></tr></thead><tbody className={tableBodyClassName}>{logs.map((log) => { const category = errorCategory(log.errorCode, log.errorMessage); return <tr key={log.id}><td className="whitespace-nowrap px-3 py-3 text-xs text-slate-600">{formatDateTime(log.createdAt)}</td><td className="max-w-[260px] px-3 py-3"><p className="truncate font-semibold text-slate-900">{providerNameFor(log)}</p><p className="truncate text-xs text-slate-500">{modelNameFor(log)}</p></td><td className="px-3 py-3"><StatusBadge label={apiCallStatusLabel(log.status)} value={log.status} /></td><td className="px-3 py-3 text-slate-600">{formatDuration(log.durationMs)}</td><td className="px-3 py-3 text-sm text-slate-700">{log.status === 'SUCCESS' ? '无异常' : category.label}</td><td className="px-3 py-3"><Button onClick={() => setSelectedId(log.id)} variant="secondary">查看详情</Button></td></tr>})}</tbody></table></TableShell><Pagination disabled={isLogLoading} onChange={(page) => updateQuery({ pageNum: page }, false)} pageNum={pageNum} pageSize={20} total={logTotal} /></>}
          </SectionCard>
        </>
      ) : null}
      <ApiCallDetailDrawer detail={selectedDetail} isOpen={selectedId !== null} onClose={() => setSelectedId(null)} onRetry={() => setDetailReloadToken((current) => current + 1)} providerName={selectedDetail ? providerNameFor(selectedDetail) : undefined} modelName={selectedDetail ? modelNameFor(selectedDetail) : undefined} status={detailStatus} />
    </div>
  )
}

function TaskDetailDrawer({ task, onClose }: { task: AnalyticsTaskRecord | null; onClose: () => void }) {
  if (!task) return null
  const category = errorCategory(task.errorCode, task.errorMessage)
  return <EditorDrawer isOpen onClose={onClose} title="生图任务详情" widthClass="max-w-2xl"><div className="space-y-5"><div className="flex flex-wrap items-start justify-between gap-3 rounded-xl bg-slate-950 p-4 text-white"><div><p className="text-lg font-bold">{task.projectName}</p><p className="mt-1 text-sm text-slate-300">{task.userName} · {formatDateTime(task.createdAt)}</p></div><StatusBadge label={taskStatusLabel(task.status)} value={task.status} /></div><dl className="grid grid-cols-2 gap-2 sm:grid-cols-4"><DetailMetric label="任务类型" value={taskTypeLabel(task.type)} /><DetailMetric label="图片类型" value={imageTypeLabel(task.imageType)} /><DetailMetric label="实际出图" value={formatCompactNumber(task.outputCount, '张')} /><DetailMetric label="任务耗时" value={task.durationMs ? formatDuration(task.durationMs) : '暂无'} /></dl><section className="rounded-lg border border-slate-200 p-3"><h3 className="text-xs font-semibold text-slate-500">调用配置</h3><p className="mt-2 text-sm font-semibold text-slate-900">{task.providerName}</p><p className="mt-1 text-sm text-slate-600">{task.modelName}</p></section>{task.status === 'FAILED' || task.status === 'TIMED_OUT' ? <section className="rounded-lg border border-rose-200 bg-rose-50 p-3"><h3 className="text-sm font-bold text-rose-900">{category.label}</h3><p className="mt-1 text-xs leading-5 text-rose-800">{category.guidance}</p></section> : null}<section className="rounded-lg border border-slate-200 p-3"><h3 className="text-xs font-semibold text-slate-500">预计费用</h3><p className="mt-2 text-sm font-bold text-slate-900">{task.estimatedCost ? formatCurrency(task.estimatedCost, task.currency) : '当前账号无费用查看权限'}</p>{task.costStatus ? <p className="mt-1 text-xs text-slate-500">{costStatusLabel(task.costStatus)}</p> : null}</section><details className="rounded-lg border border-slate-200"><summary className="cursor-pointer px-3 py-2 text-xs font-semibold text-slate-600">技术详情</summary><div className="space-y-3 border-t border-slate-200 p-3"><TechnicalId label="任务ID" value={task.taskId} /><TechnicalId label="用户ID" value={task.userId} /><TechnicalId label="项目ID" value={task.projectId} /><TechnicalId label="中转站ID" value={task.providerId} /><TechnicalId label="模型ID" value={task.modelId} />{task.errorCode ? <div><p className="text-xs text-slate-500">原始错误码</p><code className="mt-1 block rounded bg-slate-100 px-2 py-1 text-xs">{task.errorCode}</code></div> : null}{task.errorMessage ? <div><p className="text-xs text-slate-500">脱敏错误信息</p><p className="mt-1 break-words rounded bg-slate-100 px-2 py-1 text-xs">{task.errorMessage}</p></div> : null}</div></details></div></EditorDrawer>
}

function ApiCallDetailDrawer({ detail, isOpen, status, onClose, onRetry, providerName, modelName }: { detail: ApiCallLog | null; isOpen: boolean; status: 'idle' | 'loading' | 'success' | 'error'; onClose: () => void; onRetry: () => void; providerName?: string; modelName?: string }) {
  const category = detail ? errorCategory(detail.errorCode, detail.errorMessage) : null
  return <EditorDrawer isOpen={isOpen} onClose={onClose} title="模型调用详情" widthClass="max-w-3xl">{status === 'loading' ? <LoadingState moduleName="模型调用详情" /> : status === 'error' ? <ErrorState moduleName="模型调用详情" onRetry={onRetry} /> : detail ? <div className="space-y-5"><div className="flex flex-wrap justify-between gap-3 rounded-xl bg-slate-950 p-4 text-white"><div><p className="text-lg font-bold">{providerName ?? '中转站名称不可用'}</p><p className="mt-1 text-sm text-slate-300">{modelName ?? '模型名称不可用'} · {formatDateTime(detail.createdAt)}</p></div><StatusBadge label={apiCallStatusLabel(detail.status)} value={detail.status} /></div><dl className="grid grid-cols-2 gap-2 sm:grid-cols-4"><DetailMetric label="调用耗时" value={formatDuration(detail.durationMs)} /><DetailMetric label="响应状态" value={detail.httpStatus ? `状态码 ${detail.httpStatus}` : '未收到响应'} /><DetailMetric label="调用结果" value={apiCallStatusLabel(detail.status)} /><DetailMetric label="请求标识" value={detail.requestId ? '已记录' : '未返回'} /></dl>{detail.status === 'FAILURE' && category ? <section className="rounded-lg border border-rose-200 bg-rose-50 p-3"><h3 className="text-sm font-bold text-rose-900">{category.label}</h3><p className="mt-1 text-xs leading-5 text-rose-800">{category.guidance}</p></section> : null}<details className="rounded-lg border border-slate-200"><summary className="cursor-pointer px-3 py-2 text-sm font-semibold text-slate-700">技术详情</summary><div className="space-y-3 border-t border-slate-200 p-3"><TechnicalId label="模型调用ID" value={detail.id} /><TechnicalId label="生图任务ID" value={detail.taskId} /><TechnicalId label="请求标识" value={detail.requestId || '未返回'} />{detail.errorCode ? <TechnicalId label="原始错误码" value={detail.errorCode} /> : null}{detail.errorMessage ? <div><p className="text-xs text-slate-500">脱敏错误信息</p><p className="mt-1 break-words rounded bg-slate-100 p-2 text-xs leading-5">{detail.errorMessage}</p></div> : null}<TechnicalPayload title="脱敏请求载荷" value={detail.redactedRequest} /><TechnicalPayload title="脱敏响应载荷" value={detail.redactedResponse} /></div></details></div> : null}</EditorDrawer>
}

function TechnicalPayload({ title, value }: { title: string; value: unknown }) {
  return <details className="rounded border border-slate-200"><summary className="cursor-pointer px-2 py-1.5 text-xs font-semibold text-slate-600">{title}</summary><pre className="max-h-72 overflow-auto border-t border-slate-200 bg-slate-950 p-3 text-[11px] leading-5 text-slate-200">{JSON.stringify(value ?? {}, null, 2)}</pre></details>
}

function TechnicalId({ label, value }: { label: string; value: string }) {
  return <div><p className="text-xs text-slate-500">{label}</p><div className="mt-1 flex items-center gap-2"><code className="min-w-0 flex-1 truncate rounded bg-slate-100 px-2 py-1 text-xs">{value}</code><Button aria-label={`复制${label}`} icon={<Copy className="h-4 w-4" />} onClick={() => void navigator.clipboard?.writeText(value)} variant="ghost">复制</Button></div></div>
}

function DetailMetric({ label, value }: { label: string; value: string }) {
  return <div className="rounded-lg border border-slate-200 bg-slate-50 p-3"><dt className="text-[11px] font-semibold text-slate-500">{label}</dt><dd className="mt-1 text-sm font-bold text-slate-900">{value}</dd></div>
}

function buildRequestTrendOption(points: Array<{ bucket: string; callCount: number; successCount: number; failureCount: number }>): EChartsCoreOption {
  return { tooltip: { trigger: 'axis' }, legend: { bottom: 0 }, grid: { left: 46, right: 16, top: 16, bottom: 54 }, xAxis: { type: 'category', data: points.map((point) => formatDateLabel(point.bucket)) }, yAxis: { type: 'value', name: '调用次数', minInterval: 1, splitLine: { lineStyle: { color: ADMIN_CHART_TOKENS.grid } } }, series: [{ name: '成功调用', type: 'line', smooth: true, data: points.map((point) => point.successCount) }, { name: '失败调用', type: 'line', smooth: true, data: points.map((point) => point.failureCount) }] }
}

const taskStatusOptions = buildTaskStatusOptions(['SUCCEEDED', 'FAILED', 'TIMED_OUT', 'CANCELLED', 'QUEUED', 'RUNNING', 'RETRYING'])
