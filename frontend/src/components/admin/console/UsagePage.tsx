import { Download, Info, RefreshCw } from 'lucide-react'
import type { EChartsCoreOption } from 'echarts/core'
import { adminApi } from '../../../api/admin'
import {
  ADMIN_CHART_TOKENS,
  dimensionLabel,
  buildTaskStatusOptions,
  currencyLabel,
  formatCompactNumber,
  formatCurrency,
  formatDateLabel,
  formatExactNumber,
  formatPercentage,
  formatPreciseCurrency,
  formatPricingCoverage,
  imageTypeLabel,
} from '../../../lib/adminPresentation'
import type { AnalyticsGroup, AnalyticsUsageBreakdown } from '../../../types/analytics'
import { Button } from '../../ui/Button'
import { AdminChart } from './AdminChart'
import { AdminFilterBar } from './AdminFilterBar'
import { useAdminAnalyticsQuery, useAdminConsole } from './AdminConsoleContext'
import { ActionFeedback, EmptyState, ErrorState, KpiCard, LoadingState, SectionCard, TableShell, tableBodyClassName, tableClassName, tableHeadClassName } from './AdminUi'
import { useAdminData } from './useAdminData'
import { useAnalyticsExport } from './useAnalyticsExport'

const groups = (['provider', 'model', 'project', 'user', 'imageType'] as const).map((value) => ({
  value,
  label: `按${dimensionLabel(value)}`,
})) satisfies Array<{ value: AnalyticsGroup; label: string }>

export function AdminUsagePage() {
  const baseQuery = useAdminAnalyticsQuery()
  const { route, updateQuery } = useAdminConsole()
  const groupBy = normalizeGroup(route.searchParams.get('groupBy'))
  const query = { ...baseQuery, groupBy }
  const queryKey = JSON.stringify(query)
  const { data, status, reload } = useAdminData(() => adminApi.getAnalyticsUsage(query), [queryKey])
  const { exportData, exporting, feedback } = useAnalyticsExport('usage', query)

  if (status === 'loading' && !data) return <LoadingState moduleName="用量与费用" />
  if (status === 'error' && !data) return <ErrorState moduleName="用量与费用" onRetry={() => void reload()} />
  if (!data) return null

  const coverage = weightedCoverage(data.costs)
  const totalRecords = data.costs.reduce((total, item) => total + item.recordCount, 0)
  const primaryUnitCost = data.unitCosts[0]
  const breakdowns = data.breakdowns.map((row) => groupBy === 'imageType' ? { ...row, name: imageTypeLabel(row.dimensionId) } : row)
  const breakdownOption = buildBreakdownOption(breakdowns)
  const costTrendOption = buildCostTrendOption(data.costTrend)

  return (
    <div className="space-y-4 sm:space-y-5">
      <AdminFilterBar statusOptions={taskStatusOptions}>
        <label className="grid min-w-[140px] gap-1 text-[11px] font-semibold text-slate-500">分组方式<select className="min-h-10 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-medium text-slate-700 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20" onChange={(event) => updateQuery({ groupBy: event.target.value }, true)} value={groupBy}>{groups.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>
      </AdminFilterBar>

      <ActionFeedback feedback={feedback} />

      {coverage < 1 && totalRecords > 0 ? <div className="flex items-start gap-3 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3"><Info className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" /><div><p className="text-sm font-semibold text-amber-950">有 {formatPercentage(1 - coverage)} 的用量记录缺少有效定价</p><p className="mt-1 text-xs leading-5 text-amber-800">预计费用只汇总已持久化的金额，不使用当前模型价格倒算历史记录。</p></div></div> : null}

      <section aria-label="费用摘要" className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <KpiCard label="预计费用" value={data.costs.length ? data.costs.map((item) => formatCurrency(item.amount, item.currency)).join(' / ') : '暂无'} change={data.costs[0]?.changePercent} description="不同币种分别汇总，绝不跨币种相加。" tone="rose" title={data.costs.map((item) => formatPreciseCurrency(item.amount, item.currency)).join(' / ')} />
        <KpiCard label="实际出图张数" value={formatCompactNumber(data.outputCount, '张')} description="当前筛选范围内真正持久化的输出数量。" tone="blue" title={formatExactNumber(data.outputCount, '张')} />
        <KpiCard label="单张预计费用" value={primaryUnitCost?.available ? data.unitCosts.map((item) => formatCurrency(item.amount, item.currency)).join(' / ') : '暂无'} description="各币种预计费用分别除以实际出图张数；无出图时不计算。" tone="violet" title={data.unitCosts.filter((item) => item.available).map((item) => formatPreciseCurrency(item.amount, item.currency)).join(' / ')} />
        <KpiCard label="定价覆盖率" value={totalRecords ? formatPercentage(coverage) : '暂无'} description={`${formatCompactNumber(totalRecords, '条')}用量记录；费用计算状态由后端持久化。`} tone={coverage >= 0.95 ? 'emerald' : 'amber'} />
      </section>

      <div className="grid gap-4 2xl:grid-cols-2">
        <SectionCard title={`${dimensionLabel(groupBy)}费用与出图构成`} description="图表和下方明细表使用完全相同的筛选与分组口径。" action={<Button disabled={status === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${status === 'loading' ? 'animate-spin' : ''}`} />} onClick={() => void reload()} variant="ghost">刷新</Button>}>
          {breakdowns.length === 0 ? <EmptyState message="当前时间范围和筛选条件下暂无用量记录，可扩大时间范围或清除筛选。" /> : <AdminChart title={`${dimensionLabel(groupBy)}费用构成`} description={`按${dimensionLabel(groupBy)}展示预计费用，不同币种分开显示。`} option={breakdownOption} height={320} />}
        </SectionCard>
        <SectionCard title="预计费用趋势" description="按时间和币种展示已持久化的预计费用。">
          {data.costTrend.length === 0 ? <EmptyState message="当前范围暂无费用趋势数据。" /> : <AdminChart title="预计费用趋势" description="按日期和币种展示预计费用变化。" option={costTrendOption} height={320} />}
        </SectionCard>
      </div>

      <SectionCard title="同口径明细" description="表格是图表的文字替代，也便于核对计费用量、出图和费用。" action={<Button disabled={exporting} icon={<Download className="h-4 w-4" />} onClick={() => void exportData()} variant="secondary">{exporting ? '正在导出...' : '导出中文 CSV'}</Button>}>
        {breakdowns.length === 0 ? <EmptyState message="当前筛选条件下没有可导出的用量明细。" /> : (
          <TableShell label="用量与费用明细">
            <table className={tableClassName}>
              <thead className={tableHeadClassName}><tr><th className="px-3 py-3">{dimensionLabel(groupBy)}</th><th className="px-3 py-3">计费用量记录</th><th className="px-3 py-3">输入计费用量</th><th className="px-3 py-3">输出计费用量</th><th className="px-3 py-3">计费图片</th><th className="px-3 py-3">实际出图</th><th className="px-3 py-3">预计费用</th><th className="px-3 py-3">定价覆盖率</th></tr></thead>
              <tbody className={tableBodyClassName}>{breakdowns.map((row) => <tr key={row.dimensionId}><td className="max-w-[260px] px-3 py-3"><p className="truncate font-semibold text-slate-900">{row.name || `${dimensionLabel(groupBy)}名称未设置`}</p></td><td className="px-3 py-3 text-slate-600" title={formatExactNumber(row.recordCount, '条')}>{formatCompactNumber(row.recordCount, '条')}</td><td className="px-3 py-3 text-slate-600" title={formatExactNumber(row.inputTokens)}>{formatCompactNumber(row.inputTokens)} <span className="text-[10px] text-slate-400">Token</span></td><td className="px-3 py-3 text-slate-600" title={formatExactNumber(row.outputTokens)}>{formatCompactNumber(row.outputTokens)} <span className="text-[10px] text-slate-400">Token</span></td><td className="px-3 py-3 text-slate-600">{formatCompactNumber(row.billedImageCount, '张')}</td><td className="px-3 py-3 font-semibold text-slate-800">{formatCompactNumber(row.outputCount, '张')}</td><td className="px-3 py-3"><CostList row={row} /></td><td className="px-3 py-3 text-slate-600">{row.costs.length ? formatPercentage(weightedCoverage(row.costs)) : '暂无'}</td></tr>)}</tbody>
            </table>
          </TableShell>
        )}
      </SectionCard>
    </div>
  )
}

function CostList({ row }: { row: AnalyticsUsageBreakdown }) {
  if (row.costs.length === 0) return <span className="text-xs text-slate-400">暂无费用</span>
  return <div className="space-y-1">{row.costs.map((cost) => <p className="whitespace-nowrap text-xs font-semibold text-slate-800" key={cost.currency} title={`${formatPreciseCurrency(cost.amount, cost.currency)}；${formatPricingCoverage(cost.pricingCoverage)}`}>{formatCurrency(cost.amount, cost.currency)}</p>)}</div>
}

function buildBreakdownOption(rows: AnalyticsUsageBreakdown[]): EChartsCoreOption {
  const currencies = [...new Set(rows.flatMap((row) => row.costs.map((cost) => cost.currency)))].sort()
  return {
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    legend: { bottom: 0 },
    grid: { left: 32, right: 24, top: 16, bottom: 58, containLabel: true },
    xAxis: { type: 'value', name: '预计费用', splitLine: { lineStyle: { color: ADMIN_CHART_TOKENS.grid } } },
    yAxis: { type: 'category', data: rows.slice(0, 10).map((row) => row.name || '名称未设置'), axisLabel: { width: 112, overflow: 'truncate' } },
    series: currencies.map((currency) => ({ name: currencyLabel(currency), type: 'bar', stack: currency, data: rows.slice(0, 10).map((row) => Number(row.costs.find((cost) => cost.currency === currency)?.amount ?? 0)), itemStyle: { borderRadius: [0, 4, 4, 0] } })),
  }
}

function buildCostTrendOption(points: Array<{ bucket: string; currency: string; estimatedCost: string }>): EChartsCoreOption {
  const buckets = [...new Set(points.map((point) => point.bucket))].sort()
  const currencies = [...new Set(points.map((point) => point.currency))].sort()
  return {
    tooltip: { trigger: 'axis' }, legend: { bottom: 0 }, grid: { left: 48, right: 18, top: 16, bottom: 54 },
    xAxis: { type: 'category', data: buckets.map(formatDateLabel), axisLabel: { color: ADMIN_CHART_TOKENS.axis } },
    yAxis: { type: 'value', name: '预计费用', splitLine: { lineStyle: { color: ADMIN_CHART_TOKENS.grid } } },
    series: currencies.map((currency) => ({ name: currencyLabel(currency), type: 'line', smooth: true, areaStyle: { opacity: 0.05 }, data: buckets.map((bucket) => Number(points.find((point) => point.bucket === bucket && point.currency === currency)?.estimatedCost ?? 0)) })),
  }
}

function normalizeGroup(value: string | null): AnalyticsGroup {
  return groups.some((group) => group.value === value) ? value as AnalyticsGroup : 'provider'
}

function weightedCoverage(costs: Array<{ recordCount: number; pricedRecordCount: number }>) {
  const total = costs.reduce((sum, item) => sum + item.recordCount, 0)
  return total > 0 ? costs.reduce((sum, item) => sum + item.pricedRecordCount, 0) / total : 1
}

const taskStatusOptions = buildTaskStatusOptions(['SUCCEEDED', 'FAILED', 'TIMED_OUT', 'CANCELLED'])
