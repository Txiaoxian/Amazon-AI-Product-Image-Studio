import { FileClock, RefreshCw, Search } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { adminApi } from '../../../api/admin'
import { ADMIN_AUDIT_ACTION_VALUES, ADMIN_AUDIT_RESOURCE_VALUES, auditActionLabel, auditResourceLabel, buildAuditActionOptions, buildAuditResourceOptions, formatDateTime } from '../../../lib/adminPresentation'
import type { OperationLog } from '../../../types/admin'
import { Button } from '../../ui/Button'
import { EditorDrawer } from '../../ui/EditorDrawer'
import { useAdminConsole } from './AdminConsoleContext'
import { EmptyState, ErrorState, LoadingState, Pagination, SectionCard, TableShell, tableBodyClassName, tableClassName, tableHeadClassName } from './AdminUi'

export function AdminAuditPage() {
  const { route, updateQuery, dateFrom, dateTo } = useAdminConsole()
  const pageNum = Number(route.searchParams.get('pageNum') || 1)
  const action = route.searchParams.get('action') ?? ''
  const resourceType = route.searchParams.get('resourceType') ?? ''
  const [records, setRecords] = useState<OperationLog[]>([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [selected, setSelected] = useState<OperationLog | null>(null)
  const requestSequence = useRef(0)

  const load = () => {
    const sequence = requestSequence.current + 1
    requestSequence.current = sequence
    setStatus('loading')
    void adminApi.listOperationLogs({ pageNum, pageSize: 20, sortBy: 'createdAt', sortOrder: 'desc', createdAtFrom: `${dateFrom}T00:00:00+08:00`, createdAtTo: `${dateTo}T23:59:59.999+08:00`, action: action || undefined, resourceType: resourceType || undefined })
      .then((page) => { if (requestSequence.current === sequence) { setRecords(page.records); setTotal(page.total); setStatus('success') } })
      .catch(() => { if (requestSequence.current === sequence) setStatus('error') })
  }

  useEffect(load, [action, dateFrom, dateTo, pageNum, resourceType])

  return (
    <div className="space-y-4 sm:space-y-5">
      <section aria-label="审计筛选" className="flex flex-wrap items-end gap-2 rounded-xl border border-slate-200 bg-white p-3 shadow-sm">
        <div className="flex min-h-10 items-center gap-2 px-1 text-xs font-semibold text-slate-600"><Search className="h-4 w-4" />筛选</div>
        <FilterSelect label="操作类型" value={action} onChange={(value) => updateQuery({ action: value || null, pageNum: null }, true)} options={actionOptions} />
        <FilterSelect label="资源类型" value={resourceType} onChange={(value) => updateQuery({ resourceType: value || null, pageNum: null }, true)} options={resourceOptions} />
        {(action || resourceType) ? <Button className="ml-auto" onClick={() => updateQuery({ action: null, resourceType: null, pageNum: null }, true)} variant="ghost">清除筛选</Button> : null}
      </section>

      <SectionCard title="操作审计记录" description="优先展示谁、什么时候、修改了什么；IP、资源ID和脱敏元数据收进技术详情。" action={<Button disabled={status === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${status === 'loading' ? 'animate-spin' : ''}`} />} onClick={load} variant="ghost">刷新</Button>}>
        {status === 'loading' && records.length === 0 ? <LoadingState moduleName="操作审计" /> : status === 'error' && records.length === 0 ? <ErrorState moduleName="操作审计" onRetry={load} /> : records.length === 0 ? <EmptyState message="当前时间范围和筛选条件下暂无操作记录，可扩大时间范围或清除筛选。" /> : <><TableShell label="操作审计明细"><table className={tableClassName}><thead className={tableHeadClassName}><tr><th className="px-3 py-3">操作者</th><th className="px-3 py-3">操作时间</th><th className="px-3 py-3">操作摘要</th><th className="px-3 py-3">影响对象</th><th className="px-3 py-3">操作</th></tr></thead><tbody className={tableBodyClassName}>{records.map((record) => <tr key={record.id}><td className="max-w-[220px] px-3 py-3"><p className="truncate font-semibold text-slate-900">{auditActorName(record)}</p>{record.actorEmail && record.actorEmail !== record.actorName ? <p className="truncate text-xs text-slate-500">{record.actorEmail}</p> : null}</td><td className="whitespace-nowrap px-3 py-3 text-xs text-slate-600">{formatDateTime(record.createdAt)}</td><td className="px-3 py-3 text-sm font-semibold text-slate-800">{auditActionLabel(record.action)}</td><td className="px-3 py-3 text-sm text-slate-600">{auditResourceLabel(record.resourceType)}</td><td className="px-3 py-3"><Button onClick={() => setSelected(record)} variant="secondary">查看详情</Button></td></tr>)}</tbody></table></TableShell><Pagination disabled={status === 'loading'} onChange={(page) => updateQuery({ pageNum: page }, false)} pageNum={pageNum} pageSize={20} total={total} /></>}
      </SectionCard>

      <AuditDetailDrawer onClose={() => setSelected(null)} record={selected} />
    </div>
  )
}

function AuditDetailDrawer({ record, onClose }: { record: OperationLog | null; onClose: () => void }) {
  const actorName = record ? auditActorName(record) : ''
  return <EditorDrawer isOpen={record !== null} onClose={onClose} title="操作审计详情" widthClass="max-w-2xl">{record ? <div className="space-y-5"><section className="rounded-xl bg-slate-950 p-4 text-white"><div className="flex items-center gap-2 text-blue-300"><FileClock className="h-4 w-4" /><span className="text-xs font-semibold">操作摘要</span></div><p className="mt-3 text-lg font-bold">{actorName} {auditActionLabel(record.action)}</p><p className="mt-2 text-sm text-slate-300">{formatDateTime(record.createdAt)} · {auditResourceLabel(record.resourceType)}</p></section><dl className="grid gap-2 sm:grid-cols-2"><Detail label="操作者" value={actorName} /><Detail label="操作时间" value={formatDateTime(record.createdAt)} /><Detail label="操作内容" value={auditActionLabel(record.action)} /><Detail label="影响对象" value={auditResourceLabel(record.resourceType)} /></dl><details className="rounded-lg border border-slate-200"><summary className="cursor-pointer px-3 py-2 text-sm font-semibold text-slate-700">技术详情</summary><div className="space-y-3 border-t border-slate-200 p-3 text-xs">{record.actorUserId ? <Detail label="操作者ID" value={String(record.actorUserId)} /> : null}<Detail label="原始动作码" value={record.action} /><Detail label="资源类型" value={record.resourceType} /><Detail label="资源ID" value={record.resourceId || '未记录'} /><Detail label="IP地址" value={record.ip || '未记录'} /><div><p className="font-semibold text-slate-500">脱敏元数据</p><pre className="mt-1 max-h-80 overflow-auto rounded-lg bg-slate-950 p-3 text-[11px] leading-5 text-slate-200">{JSON.stringify(record.metadata ?? {}, null, 2)}</pre></div></div></details></div> : null}</EditorDrawer>
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="rounded-lg border border-slate-200 bg-slate-50 p-3"><dt className="text-[11px] font-semibold text-slate-500">{label}</dt><dd className="mt-1 break-words text-sm font-semibold text-slate-800">{value}</dd></div>
}

function FilterSelect({ label, value, options, onChange }: { label: string; value: string; options: Array<{ value: string; label: string }>; onChange: (value: string) => void }) {
  return <label className="grid min-w-[170px] gap-1 text-[11px] font-semibold text-slate-500">{label}<select className="min-h-10 rounded-lg border border-slate-200 bg-white px-2.5 text-xs font-medium text-slate-700 outline-none focus:border-amazon-500 focus:ring-2 focus:ring-amazon-500/20" onChange={(event) => onChange(event.target.value)} value={value}><option value="">全部{label}</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>
}

function auditActorName(record: OperationLog): string {
  if (!record.actorUserId) return '平台系统'
  return record.actorName?.trim() || record.actorEmail?.trim() || '已删除或名称不可用的用户'
}

const actionOptions = buildAuditActionOptions(ADMIN_AUDIT_ACTION_VALUES)
const resourceOptions = buildAuditResourceOptions(ADMIN_AUDIT_RESOURCE_VALUES)
