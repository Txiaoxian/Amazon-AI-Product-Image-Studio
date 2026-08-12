import { AlertCircle, ArrowDownRight, ArrowUpRight, Inbox, Loader2, RefreshCw } from 'lucide-react'
import type { ReactNode } from 'react'
import { formatChange, statusTone, type StatusTone } from '../../../lib/adminPresentation'
import { Button } from '../../ui/Button'

export function SectionCard({
  title,
  description,
  action,
  children,
  className = '',
}: {
  title: string
  description?: string
  action?: ReactNode
  children: ReactNode
  className?: string
}) {
  return (
    <section className={`min-w-0 max-w-full rounded-xl border border-slate-200 bg-white shadow-panel ${className}`}>
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-slate-100 px-4 py-4 sm:px-5">
        <div>
          <h2 className="text-sm font-semibold text-slate-950">{title}</h2>
          {description ? <p className="mt-1 text-xs leading-5 text-slate-500">{description}</p> : null}
        </div>
        {action}
      </header>
      <div className="p-4 sm:p-5">{children}</div>
    </section>
  )
}

export function KpiCard({
  label,
  value,
  change,
  description,
  lowerIsBetter = false,
  title,
}: {
  label: string
  value: string
  change?: number | null
  description: string
  lowerIsBetter?: boolean
  tone?: 'blue' | 'amber' | 'emerald' | 'violet' | 'rose' | 'slate'
  title?: string
}) {
  const changeText = formatChange(change, lowerIsBetter)
  const improved = change !== null && change !== undefined && (lowerIsBetter ? change < 0 : change > 0)
  const declined = change !== null && change !== undefined && (lowerIsBetter ? change > 0 : change < 0)

  return (
    <article className="relative overflow-hidden rounded-xl border border-slate-200 bg-white p-4 shadow-panel" title={title}>
      <p className="text-xs font-semibold text-slate-500">{label}</p>
      <p className="mt-3 break-words text-2xl font-bold tracking-tight text-slate-950 sm:text-[1.7rem]">{value}</p>
      <p className={`mt-2 flex min-h-5 items-center gap-1 text-xs ${improved ? 'text-emerald-700' : declined ? 'text-rose-700' : 'text-slate-500'}`}>
        {improved ? <ArrowUpRight className="h-3.5 w-3.5" /> : null}
        {declined ? <ArrowDownRight className="h-3.5 w-3.5" /> : null}
        {changeText}
      </p>
      <p className="mt-3 border-t border-slate-100 pt-3 text-xs leading-5 text-slate-500">{description}</p>
    </article>
  )
}

export function StatusBadge({ value, label, tone }: { value?: string; label: string; tone?: StatusTone }) {
  const resolvedTone = tone ?? statusTone(value ?? '')
  const classes: Record<StatusTone, string> = {
    success: 'border-emerald-200 bg-emerald-50 text-emerald-800',
    warning: 'border-amber-200 bg-amber-50 text-amber-800',
    danger: 'border-rose-200 bg-rose-50 text-rose-800',
    info: 'border-blue-200 bg-blue-50 text-blue-800',
    neutral: 'border-slate-200 bg-slate-50 text-slate-700',
  }
  return <span className={`inline-flex items-center rounded-full border px-2 py-1 text-xs font-semibold ${classes[resolvedTone]}`}>{label}</span>
}

export function LoadingState({ moduleName = '统计数据' }: { moduleName?: string }) {
  return (
    <div aria-busy="true" className="flex min-h-48 flex-col items-center justify-center rounded-lg bg-slate-50 px-6 text-center" role="status">
      <Loader2 className="h-6 w-6 animate-spin text-amazon-600" />
      <p className="mt-3 text-sm font-semibold text-slate-700">正在加载{moduleName}...</p>
      <p className="mt-1 text-xs text-slate-500">数据会按当前筛选条件汇总。</p>
    </div>
  )
}

export function EmptyState({ message, action }: { message: string; action?: ReactNode }) {
  return (
    <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border border-dashed border-slate-300 bg-slate-50/70 px-6 text-center">
      <Inbox className="h-7 w-7 text-slate-400" />
      <p className="mt-3 max-w-xl text-sm leading-6 text-slate-600">{message}</p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  )
}

export function ErrorState({ moduleName, onRetry }: { moduleName: string; onRetry: () => void }) {
  return (
    <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border border-rose-200 bg-rose-50 px-6 text-center" role="alert">
      <AlertCircle className="h-7 w-7 text-rose-600" />
      <p className="mt-3 text-sm font-semibold text-rose-900">{moduleName}暂时无法加载</p>
      <p className="mt-1 max-w-lg text-xs leading-5 text-rose-700">其他管理功能不受影响，请稍后重试。若持续出现，可记录页面和时间交给技术人员排查。</p>
      <Button className="mt-4" icon={<RefreshCw className="h-4 w-4" />} onClick={onRetry}>
        重新加载
      </Button>
    </div>
  )
}

export function ActionFeedback({ feedback }: { feedback: { message: string; tone: 'success' | 'error' } | null }) {
  if (!feedback) return null
  const className = feedback.tone === 'success'
    ? 'border-emerald-200 bg-emerald-50 text-emerald-800'
    : 'border-rose-200 bg-rose-50 text-rose-800'
  return <p aria-live="polite" className={`rounded-lg border px-3 py-2 text-sm ${className}`} role={feedback.tone === 'error' ? 'alert' : 'status'}>{feedback.message}</p>
}

export function TableShell({ children, label }: { children: ReactNode; label: string }) {
  return (
    <div aria-label={label} className="overflow-x-auto rounded-lg border border-slate-200">
      {children}
    </div>
  )
}

export const tableClassName = 'min-w-full divide-y divide-slate-200 text-left text-sm'
export const tableHeadClassName = 'bg-slate-50 text-xs font-semibold text-slate-600'
export const tableBodyClassName = 'divide-y divide-slate-100 bg-white'

export function Pagination({ pageNum, pageSize, total, onChange, disabled = false }: { pageNum: number; pageSize: number; total: number; onChange: (page: number) => void; disabled?: boolean }) {
  const pageCount = Math.max(1, Math.ceil(total / pageSize))
  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-xs text-slate-500">
      <span>共 {total.toLocaleString('zh-CN')} 条，第 {pageNum} / {pageCount} 页</span>
      <div className="flex gap-2">
        <Button disabled={disabled || pageNum <= 1} onClick={() => onChange(pageNum - 1)} variant="secondary">上一页</Button>
        <Button disabled={disabled || pageNum >= pageCount} onClick={() => onChange(pageNum + 1)} variant="secondary">下一页</Button>
      </div>
    </div>
  )
}
