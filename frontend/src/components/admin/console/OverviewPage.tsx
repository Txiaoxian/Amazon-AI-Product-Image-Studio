import { AlertTriangle, ArrowRight, Clock3, RefreshCw, Sparkles, UsersRound, WalletCards } from 'lucide-react'
import type { EChartsCoreOption } from 'echarts/core'
import { adminApi } from '../../../api/admin'
import {
  ADMIN_CHART_TOKENS,
  dimensionLabel,
  buildTaskStatusOptions,
  currencyLabel,
  errorCategory,
  formatCompactNumber,
  formatCurrency,
  formatDateLabel,
  formatDateTime,
  formatDuration,
  formatExactNumber,
  formatPercentage,
  formatPreciseCurrency,
  formatPricingCoverage,
} from '../../../lib/adminPresentation'
import type { AnalyticsCostMetric, AnalyticsRankingItem, AnalyticsTrendPoint } from '../../../types/analytics'
import { Button } from '../../ui/Button'
import { AdminChart } from './AdminChart'
import { AdminFilterBar } from './AdminFilterBar'
import { useAdminAnalyticsQuery, useAdminConsole } from './AdminConsoleContext'
import { EmptyState, ErrorState, KpiCard, LoadingState, SectionCard, TableShell, tableBodyClassName, tableClassName, tableHeadClassName } from './AdminUi'
import { useAdminData } from './useAdminData'

export function AdminOverviewPage() {
  const query = useAdminAnalyticsQuery()
  const queryKey = JSON.stringify(query)
  const { data, status, reload } = useAdminData(() => adminApi.getAnalyticsOverview(query), [queryKey])
  const { hasPermission, navigate } = useAdminConsole()

  if (status === 'loading' && !data) return <LoadingState moduleName="经营总览" />
  if (status === 'error' && !data) return <ErrorState moduleName="经营总览" onRetry={() => void reload()} />
  if (!data) return null

  const primaryCost = data.costs[0]
  const coverage = weightedCoverage(data.costs)
  const taskTrendOption = buildTaskTrendOption(data.trend)
  const statusTrendOption = buildStatusTrendOption(data.trend)
  const activeTrendOption = buildActiveTrendOption(data.trend, hasPermission('audit:read'))
  const costTrendOption = buildCostTrendOption(data.costTrend)
  const costValue = data.costs.length > 0 ? data.costs.map((item) => formatCurrency(item.amount, item.currency)).join(' / ') : '暂无费用记录'

  return (
    <div className="space-y-4 sm:space-y-5">
      <AdminFilterBar statusOptions={taskStatusOptions} />

      {coverage < 1 && data.costs.length > 0 ? (
        <div className="flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-950" role="status">
          <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" />
          <div><p className="font-semibold">预计费用数据尚不完整</p><p className="mt-1 text-xs leading-5 text-amber-800">{formatPercentage(1 - coverage)} 的用量记录缺少有效定价或可靠的历史费用状态。任务与出图统计不受影响。</p></div>
        </div>
      ) : null}

      <section aria-label="核心经营指标" className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
        <KpiCard label="实际出图张数" value={formatCompactNumber(data.current.outputCount, '张')} change={data.changes.outputCount} description="周期内真正持久化的任务输出数量。" tone="blue" title={formatExactNumber(data.current.outputCount, '张')} />
        <KpiCard label="生图任务数" value={formatCompactNumber(data.current.taskCount, '个')} change={data.changes.taskCount} description="周期内创建的图片生成和编辑任务。" tone="violet" title={formatExactNumber(data.current.taskCount, '个')} />
        <KpiCard label="任务成功率" value={formatPercentage(data.current.taskSuccessRate)} change={data.changes.taskSuccessRate} description="已完成任务数 ÷ 全部终态任务数，排除进行中任务。" tone="emerald" />
        <KpiCard label="生图活跃用户" value={formatCompactNumber(data.current.activeUserCount, '人')} change={data.changes.activeUserCount} description="周期内至少创建过一个生图或编辑任务的去重用户。" tone="amber" title={formatExactNumber(data.current.activeUserCount, '人')} />
        <KpiCard label="预计费用" value={costValue} change={primaryCost?.changePercent} description={`按持久化费用精确汇总；${data.costs.length > 0 ? formatPricingCoverage(coverage) : '暂无费用记录'}。`} tone="rose" title={data.costs.map((item) => formatPreciseCurrency(item.amount, item.currency)).join(' / ')} />
        <KpiCard label="95%的任务耗时不超过" value={formatDuration(data.current.p95DurationMs)} change={data.changes.p95DurationMs} description="第95百分位任务耗时，少数最慢任务可能高于该值。" lowerIsBetter tone="slate" />
      </section>

      <div className="grid gap-4 2xl:grid-cols-2">
        <SectionCard title="每日实际出图与生图任务" description="任务量与真正落盘的图片数量使用同一时间范围。" action={<RefreshButton loading={status === 'loading'} onClick={() => void reload()} />}>
          {data.trend.length === 0 ? <EmptyState message="当前时间范围内暂无生图记录，可扩大时间范围或清除筛选。" /> : <><AdminChart title="实际出图与生图任务趋势" description="按日期展示实际出图张数和生图任务数，图表下方提供同口径数据表。" option={taskTrendOption} /><TrendTable points={data.trend} columns={['taskCount', 'outputCount']} /></>}
        </SectionCard>
        <SectionCard title="终态任务结果趋势" description="成功、失败、超时和取消分别展示，便于识别异常类型。">
          {data.trend.length === 0 ? <EmptyState message="当前时间范围内暂无终态任务。" /> : <><AdminChart title="任务结果趋势" description="按日期展示成功、失败、超时和取消任务数。" option={statusTrendOption} /><TrendTable points={data.trend} columns={['succeededCount', 'failedCount', 'timedOutCount', 'cancelledCount']} /></>}
        </SectionCard>
        <SectionCard title="预计费用趋势" description="不同币种分开展示，不进行跨币种相加。">
          {data.costTrend.length === 0 ? <EmptyState message="当前范围暂无可展示的费用记录，可检查定价配置或扩大时间范围。" /> : <><AdminChart title="预计费用趋势" description="按日期和币种展示持久化的预计费用。" option={costTrendOption} /><CostTrendTable points={data.costTrend} /></>}
        </SectionCard>
        <SectionCard title="生图活跃与登录活跃" description={hasPermission('audit:read') ? '生图活跃和成功登录活跃分开计算，避免混淆。' : '当前账号无操作审计权限，仅展示生图活跃。'}>
          {data.trend.length === 0 ? <EmptyState message="当前范围暂无活跃记录。" /> : <><AdminChart title="用户活跃趋势" description="按日期展示生图活跃用户与登录活跃用户。" option={activeTrendOption} /><TrendTable points={data.trend} columns={hasPermission('audit:read') ? ['activeUserCount', 'loginActiveUsers'] : ['activeUserCount']} /></>}
        </SectionCard>
      </div>

      <SectionCard title="用户、项目、模型和中转站排行" description="点击排行可携带当前时间范围和筛选条件继续钻取。">
        {data.rankings.length === 0 ? <EmptyState message="当前范围暂无可排行的任务记录。" /> : (
          <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-4">
            {(['user', 'project', 'provider', 'model'] as const).map((dimension) => (
              <RankingList
                dimension={dimension}
                items={data.rankings.filter((item) => item.dimension === dimension)}
                key={dimension}
                onSelect={(item) => {
                  const canOpenUser = dimension === 'user' && hasPermission('user:read')
                  const canOpenTasks = dimension === 'project' && hasPermission('audit:read')
                  const pathname = canOpenUser ? '/admin/users' : canOpenTasks ? '/admin/requests' : '/admin/usage'
                  navigate(pathname, {
                    [`${dimension}Id`]: item.dimensionId,
                    groupBy: pathname === '/admin/usage' ? dimension : null,
                  })
                }}
              />
            ))}
          </div>
        )}
      </SectionCard>

      <SectionCard title="高频异常" description="错误按中文业务类别聚合；原始错误码只在模型调用技术详情中查看。" action={hasPermission('audit:read') ? <Button icon={<ArrowRight className="h-4 w-4" />} onClick={() => navigate('/admin/requests', { tab: 'requests' })} variant="ghost">查看模型调用</Button> : null}>
        {!hasPermission('audit:read') ? <EmptyState message="需要“查看审计记录”权限才能查看模型调用异常。" /> : data.errorGroups.length === 0 ? <EmptyState message="当前时间范围内未发现模型调用异常。" /> : (
          <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {data.errorGroups.map((item) => {
              const category = errorCategory(item.errorCode)
              return <article className="rounded-lg border border-slate-200 bg-slate-50 p-3" key={item.errorCode}><div className="flex items-center justify-between gap-3"><p className="text-sm font-semibold text-slate-900">{category.label}</p><span className="rounded-full bg-white px-2 py-1 text-xs font-bold text-slate-700">影响 {formatCompactNumber(item.count, '个任务')}</span></div><p className="mt-2 text-xs leading-5 text-slate-500">{category.guidance}</p></article>
            })}
          </div>
        )}
      </SectionCard>

      <p className="flex items-center justify-end gap-1 text-[11px] text-slate-400"><Clock3 className="h-3.5 w-3.5" />数据生成于 {formatDateTime(data.meta.generatedAt)}，统计时区为北京时间。</p>
    </div>
  )
}

const taskStatusOptions = buildTaskStatusOptions(['SUCCEEDED', 'FAILED', 'TIMED_OUT', 'CANCELLED', 'QUEUED', 'RUNNING', 'RETRYING'])

function buildTaskTrendOption(points: AnalyticsTrendPoint[]): EChartsCoreOption {
  return baseTrendOption(points, [
    { name: '实际出图张数', type: 'bar', data: points.map((point) => point.outputCount), itemStyle: { borderRadius: [4, 4, 0, 0] } },
    { name: '生图任务数', type: 'line', smooth: true, symbolSize: 7, data: points.map((point) => point.taskCount) },
  ], '数量')
}

function buildStatusTrendOption(points: AnalyticsTrendPoint[]): EChartsCoreOption {
  return baseTrendOption(points, [
    { name: '成功', type: 'line', smooth: true, data: points.map((point) => point.succeededCount) },
    { name: '失败', type: 'line', smooth: true, data: points.map((point) => point.failedCount) },
    { name: '超时', type: 'line', smooth: true, data: points.map((point) => point.timedOutCount) },
    { name: '取消', type: 'line', smooth: true, data: points.map((point) => point.cancelledCount) },
  ], '任务数')
}

function buildActiveTrendOption(points: AnalyticsTrendPoint[], includeLogin: boolean): EChartsCoreOption {
  const series: EChartsCoreOption[] = [{ name: '生图活跃用户', type: 'line', areaStyle: { opacity: 0.08 }, smooth: true, data: points.map((point) => point.activeUserCount) }]
  if (includeLogin) series.push({ name: '登录活跃用户', type: 'line', smooth: true, data: points.map((point) => point.loginActiveUsers) })
  return baseTrendOption(points, series, '人数')
}

function buildCostTrendOption(points: Array<{ bucket: string; currency: string; estimatedCost: string }>): EChartsCoreOption {
  const buckets = [...new Set(points.map((point) => point.bucket))].sort()
  const currencies = [...new Set(points.map((point) => point.currency))].sort()
  return {
    tooltip: { trigger: 'axis', valueFormatter: (value: unknown) => String(value) },
    legend: { bottom: 0, data: currencies.map((currency) => `${currencyLabel(currency)}预计费用`) },
    grid: { left: 52, right: 18, top: 20, bottom: 54 },
    xAxis: { type: 'category', data: buckets.map(formatDateLabel), axisLabel: { color: ADMIN_CHART_TOKENS.axis } },
    yAxis: { type: 'value', name: '预计费用', axisLabel: { color: ADMIN_CHART_TOKENS.axis }, splitLine: { lineStyle: { color: ADMIN_CHART_TOKENS.grid } } },
    series: currencies.map((currency) => ({ name: `${currencyLabel(currency)}预计费用`, type: 'line', smooth: true, data: buckets.map((bucket) => Number(points.find((point) => point.bucket === bucket && point.currency === currency)?.estimatedCost ?? 0)) })),
  }
}

function baseTrendOption(points: AnalyticsTrendPoint[], series: EChartsCoreOption[], yName: string): EChartsCoreOption {
  return {
    tooltip: { trigger: 'axis' },
    legend: { bottom: 0 },
    grid: { left: 48, right: 18, top: 20, bottom: 54 },
    xAxis: { type: 'category', data: points.map((point) => formatDateLabel(point.bucket)), axisLabel: { color: ADMIN_CHART_TOKENS.axis } },
    yAxis: { type: 'value', name: yName, minInterval: 1, axisLabel: { color: ADMIN_CHART_TOKENS.axis }, splitLine: { lineStyle: { color: ADMIN_CHART_TOKENS.grid } } },
    series,
  }
}

function TrendTable({ points, columns }: { points: AnalyticsTrendPoint[]; columns: Array<keyof AnalyticsTrendPoint> }) {
  const labels: Partial<Record<keyof AnalyticsTrendPoint, string>> = { taskCount: '生图任务数', outputCount: '实际出图张数', succeededCount: '成功', failedCount: '失败', timedOutCount: '超时', cancelledCount: '取消', activeUserCount: '生图活跃用户', loginActiveUsers: '登录活跃用户' }
  return (
    <details className="mt-3 rounded-lg border border-slate-200"><summary className="cursor-pointer px-3 py-2 text-xs font-semibold text-slate-600">查看同口径数据表</summary><TableShell label="趋势明细"><table className={tableClassName}><thead className={tableHeadClassName}><tr><th className="px-3 py-2">日期</th>{columns.map((column) => <th className="px-3 py-2" key={column}>{labels[column]}</th>)}</tr></thead><tbody className={tableBodyClassName}>{points.map((point) => <tr key={point.bucket}><td className="px-3 py-2 font-medium text-slate-800">{formatDateLabel(point.bucket)}</td>{columns.map((column) => <td className="px-3 py-2 text-slate-600" key={column}>{formatExactNumber(Number(point[column]))}</td>)}</tr>)}</tbody></table></TableShell></details>
  )
}

function CostTrendTable({ points }: { points: Array<{ bucket: string; currency: string; estimatedCost: string }> }) {
  return <details className="mt-3 rounded-lg border border-slate-200"><summary className="cursor-pointer px-3 py-2 text-xs font-semibold text-slate-600">查看费用数据表</summary><TableShell label="预计费用趋势明细"><table className={tableClassName}><thead className={tableHeadClassName}><tr><th className="px-3 py-2">日期</th><th className="px-3 py-2">币种</th><th className="px-3 py-2">预计费用</th></tr></thead><tbody className={tableBodyClassName}>{points.map((point) => <tr key={`${point.bucket}-${point.currency}`}><td className="px-3 py-2">{formatDateLabel(point.bucket)}</td><td className="px-3 py-2">{currencyLabel(point.currency)}</td><td className="px-3 py-2 font-semibold" title={formatPreciseCurrency(point.estimatedCost, point.currency)}>{formatCurrency(point.estimatedCost, point.currency)}</td></tr>)}</tbody></table></TableShell></details>
}

function RankingList({ dimension, items, onSelect }: { dimension: string; items: AnalyticsRankingItem[]; onSelect: (item: AnalyticsRankingItem) => void }) {
  const Icon = dimension === 'user' ? UsersRound : dimension === 'provider' ? WalletCards : Sparkles
  return <section className="rounded-xl border border-slate-200 bg-slate-50/70 p-3"><div className="mb-2 flex items-center gap-2"><Icon className="h-4 w-4 text-blue-600" /><h3 className="text-xs font-bold text-slate-800">{dimensionLabel(dimension)}排行</h3></div>{items.length === 0 ? <p className="py-8 text-center text-xs text-slate-500">暂无排行数据</p> : <ol className="space-y-1">{items.map((item, index) => <li key={item.dimensionId}><button className="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left hover:bg-white focus:outline-none focus:ring-2 focus:ring-amazon-500" onClick={() => onSelect(item)} type="button"><span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-xs font-bold ${index < 3 ? 'bg-blue-600 text-white' : 'bg-slate-200 text-slate-600'}`}>{index + 1}</span><span className="min-w-0 flex-1"><span className="block truncate text-xs font-semibold text-slate-900">{item.name || `${dimensionLabel(dimension)}名称未设置`}</span><span className="mt-0.5 block text-[11px] text-slate-500">{formatCompactNumber(item.taskCount, '个任务')} · {formatCompactNumber(item.outputCount, '张出图')} · 任务成功率 {formatPercentage(item.successRate)}</span>{item.costs.length > 0 ? <span className="mt-0.5 block truncate text-[11px] text-slate-500">预计费用 {item.costs.map((cost) => formatCurrency(cost.amount, cost.currency)).join(' / ')}</span> : null}</span><ArrowRight className="h-3.5 w-3.5 text-slate-400" /></button></li>)}</ol>}</section>
}

function RefreshButton({ loading, onClick }: { loading: boolean; onClick: () => void }) {
  return <Button disabled={loading} icon={<RefreshCw className={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />} onClick={onClick} variant="ghost">刷新</Button>
}

function weightedCoverage(costs: AnalyticsCostMetric[]): number {
  const records = costs.reduce((total, item) => total + item.recordCount, 0)
  const priced = costs.reduce((total, item) => total + item.pricedRecordCount, 0)
  return records > 0 ? priced / records : 1
}
