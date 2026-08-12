import { Archive, Database, Gauge, HardDrive, ImageUp, RefreshCw, Save, Settings2 } from 'lucide-react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { adminApi } from '../../../api/admin'
import { modelApi } from '../../../api/models'
import { providerApi } from '../../../api/providers'
import { formatCompactNumber } from '../../../lib/adminPresentation'
import type { SystemSettings, UpdateSystemSettingsRequest } from '../../../types/admin'
import type { Model, Provider } from '../../../types/platform'
import { Button } from '../../ui/Button'
import { useAdminConsole } from './AdminConsoleContext'
import { ErrorState, LoadingState } from './AdminUi'

type SettingsGroup = 'upload' | 'defaults' | 'concurrency' | 'storage' | 'logs'

interface SettingsDraft {
  maxFileSizeMB: string
  maxWidth: string
  maxHeight: string
  maxMegapixels: string
  defaultProviderId: string
  defaultModelId: string
  tenantLimit: string
  userLimit: string
  providerLimit: string
  modelLimit: string
  storageLimitGB: string
  deletedAssetRetentionDays: string
  operationLogRetentionDays: string
  apiCallLogRetentionDays: string
  taskEventRetentionDays: string
}

export function AdminSettingsPage() {
  const { session } = useAdminConsole()
  const [settings, setSettings] = useState<SystemSettings | null>(null)
  const [draft, setDraft] = useState<SettingsDraft>(emptyDraft())
  const [providers, setProviders] = useState<Provider[]>([])
  const [models, setModels] = useState<Model[]>([])
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [savingGroup, setSavingGroup] = useState<SettingsGroup | null>(null)
  const [notices, setNotices] = useState<Partial<Record<SettingsGroup, string>>>({})
  const [errors, setErrors] = useState<Partial<Record<SettingsGroup, string>>>({})

  const load = useCallback(async () => {
    setStatus('loading')
    try {
      const [nextSettings, providerPage, modelPage] = await Promise.all([
        adminApi.getSystemSettings(),
        providerApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null),
        modelApi.list({ pageNum: 1, pageSize: 100 }).catch(() => null),
      ])
      setSettings(nextSettings); setDraft(draftFromSettings(nextSettings)); setProviders(providerPage?.records ?? []); setModels(modelPage?.records ?? []); setStatus('success')
    } catch { setStatus('error') }
  }, [])

  useEffect(() => { void load() }, [load])

  const update = (key: keyof SettingsDraft, value: string) => setDraft((current) => ({ ...current, [key]: value }))

  const save = async (group: SettingsGroup) => {
    if (!settings) return
    setErrors((current) => ({ ...current, [group]: undefined })); setNotices((current) => ({ ...current, [group]: undefined }))
    let request: UpdateSystemSettingsRequest
    try { request = requestForGroup(group, draft) } catch { setErrors((current) => ({ ...current, [group]: '输入值无效，请按字段说明填写可接受的数字。' })); return }
    if (group === 'storage' && isDangerousStorageChange(settings, request)) {
      if (!window.confirm('新的存储或保留设置可能加快已删除图片的物理清理，且无法在页面中恢复。确定继续吗？')) return
    }
    if (group === 'logs' && isDangerousLogChange(settings, request)) {
      if (!window.confirm('缩短日志保留时间后，较早的审计和任务事件可能被永久清理。确定继续吗？')) return
    }
    setSavingGroup(group)
    try {
      const updated = await adminApi.updateSystemSettings(request, session.csrfToken ?? '')
      setSettings(updated); setDraft(draftFromSettings(updated)); setNotices((current) => ({ ...current, [group]: '设置已保存并按说明范围生效。' }))
    } catch { setErrors((current) => ({ ...current, [group]: '设置保存失败，现有配置未改变。请检查输入后重试。' })) } finally { setSavingGroup(null) }
  }

  if (status === 'loading' && !settings) return <LoadingState moduleName="系统设置" />
  if (status === 'error' && !settings) return <ErrorState moduleName="系统设置" onRetry={() => void load()} />
  if (!settings) return null

  const visibleModels = models.filter((model) => !draft.defaultProviderId || model.providerId === draft.defaultProviderId)
  const hasCurrentProviderName = providers.some((provider) => provider.id === draft.defaultProviderId)
  const hasCurrentModelName = visibleModels.some((model) => model.id === draft.defaultModelId)

  return (
    <div className="space-y-4 sm:space-y-5">
      <div className="flex flex-wrap items-start justify-between gap-3 rounded-xl border border-blue-200 bg-blue-50 px-4 py-3"><div className="flex items-start gap-3"><Settings2 className="mt-0.5 h-5 w-5 shrink-0 text-blue-700" /><div><p className="text-sm font-semibold text-blue-950">所有设置均属于当前租户</p><p className="mt-1 text-xs leading-5 text-blue-800">界面使用 MB、GB、天等可读单位；保存时由前端转换为稳定接口字段。只有可能加快数据清理的设置需要二次确认。</p></div></div><Button disabled={status === 'loading'} icon={<RefreshCw className={`h-4 w-4 ${status === 'loading' ? 'animate-spin' : ''}`} />} onClick={() => void load()} variant="secondary">重新读取</Button></div>

      <SettingsSection description="控制用户上传参考图时的单文件体积、尺寸和像素总量。新上传立即生效，已有图片不受影响。" group="upload" icon={<ImageUp className="h-5 w-5" />} onSave={() => void save('upload')} saving={savingGroup === 'upload'} title="上传限制" notice={notices.upload} error={errors.upload} limit={`系统范围：单文件 ${formatBytes(settings.constraints.uploadPolicy.maxFileSizeBytes.min)}—${formatBytes(settings.constraints.uploadPolicy.maxFileSizeBytes.max)}；宽高和像素值须在对应系统上限内。`}>
        <SettingGrid><SettingField label="单文件最大体积" unit="MB" help="超过此体积的文件会在上传前被拒绝。"><NumberInput min={1} onChange={(value) => update('maxFileSizeMB', value)} value={draft.maxFileSizeMB} /></SettingField><SettingField label="最大宽度" unit="像素" help="图片宽度超过此值时拒绝上传。"><NumberInput min={1} onChange={(value) => update('maxWidth', value)} value={draft.maxWidth} /></SettingField><SettingField label="最大高度" unit="像素" help="图片高度超过此值时拒绝上传。"><NumberInput min={1} onChange={(value) => update('maxHeight', value)} value={draft.maxHeight} /></SettingField><SettingField label="最大像素总量" unit="百万像素" help="同时限制宽×高，避免超大图片消耗过多内存。"><NumberInput min={0.1} step="0.1" onChange={(value) => update('maxMegapixels', value)} value={draft.maxMegapixels} /></SettingField></SettingGrid>
      </SettingsSection>

      <SettingsSection description="为未主动选择中转站或模型的任务提供默认值。仅影响后续新任务，不修改历史记录。" group="defaults" icon={<Gauge className="h-5 w-5" />} onSave={() => void save('defaults')} saving={savingGroup === 'defaults'} title="默认中转站与模型" notice={notices.defaults} error={errors.defaults} limit="默认模型必须属于所选中转站；停用或删除配置前请先更新默认值。">
        <SettingGrid><SettingField label="默认中转站" help="可留空，表示由用户在工作台中选择。"><select className="field-input" onChange={(event) => { update('defaultProviderId', event.target.value); update('defaultModelId', '') }} value={draft.defaultProviderId}><option value="">不设置默认中转站</option>{draft.defaultProviderId && !hasCurrentProviderName ? <option value={draft.defaultProviderId}>当前中转站（名称不可用）</option> : null}{providers.map((provider) => <option key={provider.id} value={provider.id}>{provider.name}</option>)}</select></SettingField><SettingField label="默认模型" help="只显示所选中转站下的模型显示名称。"><select className="field-input" disabled={!draft.defaultProviderId} onChange={(event) => update('defaultModelId', event.target.value)} value={draft.defaultModelId}><option value="">不设置默认模型</option>{draft.defaultModelId && !hasCurrentModelName ? <option value={draft.defaultModelId}>当前模型（名称不可用）</option> : null}{visibleModels.map((model) => <option key={model.id} value={model.id}>{model.displayName || model.modelName}</option>)}</select></SettingField></SettingGrid>
      </SettingsSection>

      <SettingsSection description="限制同时处理的生图任务数量，防止单个用户、模型或中转站占满全部容量。新任务调度立即使用新值。" group="concurrency" icon={<Gauge className="h-5 w-5" />} onSave={() => void save('concurrency')} saving={savingGroup === 'concurrency'} title="任务并发" notice={notices.concurrency} error={errors.concurrency} limit={`系统总容量为 ${settings.constraints.taskConcurrency.globalCapacity}；各层限制不能超过系统允许范围。`}>
        <SettingGrid><SettingField label="当前租户并发上限" unit="个任务" help="当前租户所有用户合计可同时运行的任务数。"><NumberInput min={1} onChange={(value) => update('tenantLimit', value)} value={draft.tenantLimit} /></SettingField><SettingField label="单个用户并发上限" unit="个任务" help="防止单个用户占用全部租户容量。"><NumberInput min={1} onChange={(value) => update('userLimit', value)} value={draft.userLimit} /></SettingField><SettingField label="单个中转站并发上限" unit="个任务" help="与中转站自身配置共同限制实际并发。"><NumberInput min={1} onChange={(value) => update('providerLimit', value)} value={draft.providerLimit} /></SettingField><SettingField label="单个模型并发上限" unit="个任务" help="用于保护高成本或限流严格的模型。"><NumberInput min={1} onChange={(value) => update('modelLimit', value)} value={draft.modelLimit} /></SettingField></SettingGrid>
      </SettingsSection>

      <SettingsSection description="设置当前租户可使用的图片存储容量，以及软删除图片的物理保留天数。降低数值可能影响后续上传或加快清理。" group="storage" icon={<HardDrive className="h-5 w-5" />} onSave={() => void save('storage')} saving={savingGroup === 'storage'} title="存储与图片保留" notice={notices.storage} error={errors.storage} limit={`当前已使用 ${formatBytes(settings.storageQuota.usedBytes)}；留空表示不设置租户容量上限或不自动清理。`}>
        <SettingGrid><SettingField label="租户存储上限" unit="GB" help="不得低于当前已使用容量；留空表示不限制。"><NumberInput min={0.01} step="0.01" onChange={(value) => update('storageLimitGB', value)} placeholder="不限制" value={draft.storageLimitGB} /></SettingField><SettingField label="已删除图片保留时间" unit="天" help="到期后才允许物理清理；留空表示长期保留。"><NumberInput min={1} onChange={(value) => update('deletedAssetRetentionDays', value)} placeholder="长期保留" value={draft.deletedAssetRetentionDays} /></SettingField></SettingGrid>
      </SettingsSection>

      <SettingsSection description="分别控制操作审计、模型调用日志和任务事件的保留时间。缩短时间会减少排障和追溯窗口。" group="logs" icon={<Archive className="h-5 w-5" />} onSave={() => void save('logs')} saving={savingGroup === 'logs'} title="日志保留" notice={notices.logs} error={errors.logs} limit="留空表示不按租户设置自动清理；实际清理仍由后台保留任务执行。">
        <SettingGrid><SettingField label="操作审计保留" unit="天" help="影响“操作审计”可查询的历史范围。"><NumberInput min={1} onChange={(value) => update('operationLogRetentionDays', value)} placeholder="长期保留" value={draft.operationLogRetentionDays} /></SettingField><SettingField label="模型调用日志保留" unit="天" help="影响调用详情和异常排查的历史范围。"><NumberInput min={1} onChange={(value) => update('apiCallLogRetentionDays', value)} placeholder="长期保留" value={draft.apiCallLogRetentionDays} /></SettingField><SettingField label="任务事件保留" unit="天" help="影响实时事件回放和历史任务诊断。"><NumberInput min={1} onChange={(value) => update('taskEventRetentionDays', value)} placeholder="长期保留" value={draft.taskEventRetentionDays} /></SettingField></SettingGrid>
      </SettingsSection>
    </div>
  )
}

function SettingsSection({ title, description, limit, icon, group, saving, notice, error, onSave, children }: { title: string; description: string; limit: string; icon: ReactNode; group: SettingsGroup; saving: boolean; notice?: string; error?: string; onSave: () => void; children: ReactNode }) {
  return <section className="overflow-hidden rounded-xl border border-slate-200 bg-white shadow-panel"><header className="flex flex-wrap items-start justify-between gap-3 border-b border-slate-100 px-4 py-4 sm:px-5"><div className="flex items-start gap-3"><span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-slate-950 text-white">{icon}</span><div><h2 className="text-sm font-bold text-slate-950">{title}</h2><p className="mt-1 max-w-4xl text-xs leading-5 text-slate-500">{description}</p></div></div><Button data-settings-group={group} disabled={saving} icon={<Save className="h-4 w-4" />} onClick={onSave} variant="primary">{saving ? '保存中...' : '保存本组'}</Button></header><div className="p-4 sm:p-5">{children}<p className="mt-4 flex items-start gap-2 rounded-lg bg-slate-50 px-3 py-2 text-xs leading-5 text-slate-600"><Database className="mt-0.5 h-4 w-4 shrink-0" />{limit}</p>{notice ? <p className="mt-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-800" role="status">{notice}</p> : null}{error ? <p className="mt-3 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-800" role="alert">{error}</p> : null}</div></section>
}

function SettingGrid({ children }: { children: ReactNode }) { return <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">{children}</div> }

function SettingField({ label, unit, help, children }: { label: string; unit?: string; help: string; children: ReactNode }) { return <label className="grid content-start gap-1.5"><span className="text-xs font-semibold text-slate-700">{label}{unit ? <span className="ml-1 font-normal text-slate-400">（{unit}）</span> : null}</span>{children}<span className="text-[11px] leading-5 text-slate-500">{help}</span></label> }

function NumberInput({ value, onChange, min, step = '1', placeholder }: { value: string; onChange: (value: string) => void; min: number; step?: string; placeholder?: string }) { return <input className="field-input" inputMode="decimal" min={min} onChange={(event) => onChange(event.target.value)} placeholder={placeholder} step={step} type="number" value={value} /> }

function emptyDraft(): SettingsDraft { return { maxFileSizeMB: '', maxWidth: '', maxHeight: '', maxMegapixels: '', defaultProviderId: '', defaultModelId: '', tenantLimit: '', userLimit: '', providerLimit: '', modelLimit: '', storageLimitGB: '', deletedAssetRetentionDays: '', operationLogRetentionDays: '', apiCallLogRetentionDays: '', taskEventRetentionDays: '' } }

function draftFromSettings(settings: SystemSettings): SettingsDraft {
  return { maxFileSizeMB: trimNumber(settings.uploadPolicy.maxFileSizeBytes / 1024 / 1024), maxWidth: String(settings.uploadPolicy.maxWidth), maxHeight: String(settings.uploadPolicy.maxHeight), maxMegapixels: trimNumber(settings.uploadPolicy.maxPixels / 1_000_000), defaultProviderId: settings.taskDefaults.defaultProviderId ?? '', defaultModelId: settings.taskDefaults.defaultModelId ?? '', tenantLimit: String(settings.taskConcurrency.tenantLimit), userLimit: String(settings.taskConcurrency.userLimit), providerLimit: String(settings.taskConcurrency.providerLimit), modelLimit: String(settings.taskConcurrency.modelLimit), storageLimitGB: settings.storageQuota.maxBytes === null ? '' : trimNumber(settings.storageQuota.maxBytes / 1024 / 1024 / 1024), deletedAssetRetentionDays: nullableNumber(settings.storageRetention.deletedAssetRetentionDays), operationLogRetentionDays: nullableNumber(settings.logRetention.operationLogRetentionDays), apiCallLogRetentionDays: nullableNumber(settings.logRetention.apiCallLogRetentionDays), taskEventRetentionDays: nullableNumber(settings.logRetention.taskEventRetentionDays) }
}

function requestForGroup(group: SettingsGroup, draft: SettingsDraft): UpdateSystemSettingsRequest {
  const positive = (value: string) => { const parsed = Number(value); if (!Number.isFinite(parsed) || parsed <= 0) throw new Error('invalid'); return parsed }
  const optional = (value: string) => value.trim() ? Math.round(positive(value)) : null
  switch (group) {
    case 'upload': return { uploadPolicy: { maxFileSizeBytes: Math.round(positive(draft.maxFileSizeMB) * 1024 * 1024), maxWidth: Math.round(positive(draft.maxWidth)), maxHeight: Math.round(positive(draft.maxHeight)), maxPixels: Math.round(positive(draft.maxMegapixels) * 1_000_000) } }
    case 'defaults': return { taskDefaults: { defaultProviderId: (draft.defaultProviderId || null) as SystemSettings['taskDefaults']['defaultProviderId'], defaultModelId: (draft.defaultModelId || null) as SystemSettings['taskDefaults']['defaultModelId'] } }
    case 'concurrency': return { taskConcurrency: { tenantLimit: Math.round(positive(draft.tenantLimit)), userLimit: Math.round(positive(draft.userLimit)), providerLimit: Math.round(positive(draft.providerLimit)), modelLimit: Math.round(positive(draft.modelLimit)) } }
    case 'storage': return { storageQuota: { maxBytes: draft.storageLimitGB.trim() ? Math.round(positive(draft.storageLimitGB) * 1024 * 1024 * 1024) : null }, storageRetention: { deletedAssetRetentionDays: optional(draft.deletedAssetRetentionDays) } }
    case 'logs': return { logRetention: { operationLogRetentionDays: optional(draft.operationLogRetentionDays), apiCallLogRetentionDays: optional(draft.apiCallLogRetentionDays), taskEventRetentionDays: optional(draft.taskEventRetentionDays) } }
  }
}

function isDangerousStorageChange(settings: SystemSettings, request: UpdateSystemSettingsRequest) { const nextRetention = request.storageRetention?.deletedAssetRetentionDays; const previousRetention = settings.storageRetention.deletedAssetRetentionDays; return (nextRetention !== null && nextRetention !== undefined && (previousRetention === null || nextRetention < previousRetention)) || (request.storageQuota?.maxBytes !== null && request.storageQuota?.maxBytes !== undefined && (settings.storageQuota.maxBytes === null || request.storageQuota.maxBytes < settings.storageQuota.maxBytes)) }
function isDangerousLogChange(settings: SystemSettings, request: UpdateSystemSettingsRequest) { const next = request.logRetention; if (!next) return false; return isShorter(settings.logRetention.operationLogRetentionDays, next.operationLogRetentionDays) || isShorter(settings.logRetention.apiCallLogRetentionDays, next.apiCallLogRetentionDays) || isShorter(settings.logRetention.taskEventRetentionDays, next.taskEventRetentionDays) }
function isShorter(previous: number | null, next: number | null) { return next !== null && (previous === null || next < previous) }
function nullableNumber(value: number | null) { return value === null ? '' : String(value) }
function trimNumber(value: number) { return Number(value.toFixed(2)).toString() }
function formatBytes(bytes: number) { if (bytes >= 1024 ** 3) return `${trimNumber(bytes / 1024 ** 3)} GB`; if (bytes >= 1024 ** 2) return `${trimNumber(bytes / 1024 ** 2)} MB`; return formatCompactNumber(bytes, '字节') }
