import {
  AlertTriangle,
  BarChart3,
  CheckCircle2,
  ClipboardList,
  Loader2,
  RefreshCw,
  Save,
  Settings2,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { adminApi as defaultAdminApi, type AdminApi } from '../../api/admin'
import { isApiClientError } from '../../api/client'
import type {
  ApiCallLog,
  OperationLog,
  SystemSettings,
  UsageRecord,
  UsageSummary,
  UsageSummaryDimension,
} from '../../types/admin'
import type { ApiPage } from '../../types/api'
import { AdminApiCallLogsView } from './AdminApiCallLogsView'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

type AdminObservabilityTab = 'usage' | 'operations' | 'apiCalls' | 'settings'

interface AdminObservabilitySettingsPanelProps {
  isOpen: boolean
  csrfToken?: string
  canReadUsage: boolean
  canReadAudit: boolean
  canManageSystemSettings: boolean
  onClose: () => void
  adminApi?: AdminApi
}

interface PageViewState<TRecord> {
  records: TRecord[]
  total: number
  pageNum: number
  pageSize: number
}

interface UploadPolicyDraft {
  maxFileSizeBytes: string
  maxWidth: string
  maxHeight: string
  maxPixels: string
}

const PAGE_SIZE = 10
const METADATA_PREVIEW_LIMIT = 1200

const usageDimensions: Array<{ value: UsageSummaryDimension; label: string }> = [
  { value: 'provider', label: 'Provider' },
  { value: 'model', label: '模型' },
  { value: 'project', label: '项目' },
  { value: 'user', label: '用户' },
]

export function AdminObservabilitySettingsPanel({
  isOpen,
  csrfToken,
  canReadUsage,
  canReadAudit,
  canManageSystemSettings,
  onClose,
  adminApi = defaultAdminApi,
}: AdminObservabilitySettingsPanelProps) {
  const apiCallDetailRequestSeqRef = useRef(0)
  const availableTabs = useMemo(
    () => getAvailableTabs({ canManageSystemSettings, canReadAudit, canReadUsage }),
    [canManageSystemSettings, canReadAudit, canReadUsage],
  )
  const [activeTab, setActiveTab] = useState<AdminObservabilityTab>(availableTabs[0] ?? 'usage')
  const [usageDimension, setUsageDimension] = useState<UsageSummaryDimension>('provider')
  const [summaryPageNum, setSummaryPageNum] = useState(1)
  const [usageRecordsPageNum, setUsageRecordsPageNum] = useState(1)
  const [operationLogsPageNum, setOperationLogsPageNum] = useState(1)
  const [apiCallLogsPageNum, setApiCallLogsPageNum] = useState(1)
  const [summaryPage, setSummaryPage] = useState<PageViewState<UsageSummary>>(emptyPage())
  const [usageRecordsPage, setUsageRecordsPage] = useState<PageViewState<UsageRecord>>(emptyPage())
  const [operationLogsPage, setOperationLogsPage] = useState<PageViewState<OperationLog>>(emptyPage())
  const [apiCallLogsPage, setApiCallLogsPage] = useState<PageViewState<ApiCallLog>>(emptyPage())
  const [isLoadingUsage, setLoadingUsage] = useState(false)
  const [isLoadingOperationLogs, setLoadingOperationLogs] = useState(false)
  const [isLoadingApiCallLogs, setLoadingApiCallLogs] = useState(false)
  const [isLoadingSettings, setLoadingSettings] = useState(false)
  const [usageError, setUsageError] = useState<string | null>(null)
  const [operationLogsError, setOperationLogsError] = useState<string | null>(null)
  const [apiCallLogsError, setApiCallLogsError] = useState<string | null>(null)
  const [settingsError, setSettingsError] = useState<string | null>(null)
  const [settingsNotice, setSettingsNotice] = useState<string | null>(null)
  const [systemSettings, setSystemSettings] = useState<SystemSettings | null>(null)
  const [settingsDraft, setSettingsDraft] = useState<UploadPolicyDraft>(() => emptyUploadPolicyDraft())
  const [isSavingSettings, setSavingSettings] = useState(false)
  const [selectedApiCallLogId, setSelectedApiCallLogId] = useState<ApiCallLog['id'] | null>(null)
  const [selectedApiCallLog, setSelectedApiCallLog] = useState<ApiCallLog | null>(null)
  const [apiCallDetailError, setApiCallDetailError] = useState<string | null>(null)
  const [loadingApiCallDetailId, setLoadingApiCallDetailId] = useState<ApiCallLog['id'] | null>(null)

  const resetApiCallDetail = useCallback(() => {
    apiCallDetailRequestSeqRef.current += 1
    setSelectedApiCallLogId(null)
    setSelectedApiCallLog(null)
    setApiCallDetailError(null)
    setLoadingApiCallDetailId(null)
  }, [])

  useEffect(() => {
    if (!isOpen) {
      setSettingsNotice(null)
      resetApiCallDetail()
      return
    }

    if (!availableTabs.includes(activeTab)) {
      setActiveTab(availableTabs[0] ?? 'usage')
    }
  }, [activeTab, availableTabs, isOpen, resetApiCallDetail])

  const loadUsage = useCallback(async () => {
    if (!canReadUsage) {
      return
    }

    setLoadingUsage(true)
    setUsageError(null)
    try {
      const [summary, records] = await Promise.all([
        adminApi.getUsageSummary({
          dimension: usageDimension,
          pageNum: summaryPageNum,
          pageSize: PAGE_SIZE,
          sortBy: 'createdAt',
          sortOrder: 'desc',
        }),
        adminApi.listUsageRecords({
          pageNum: usageRecordsPageNum,
          pageSize: PAGE_SIZE,
          sortBy: 'createdAt',
          sortOrder: 'desc',
        }),
      ])
      setSummaryPage(pageFromResponse(summary))
      setUsageRecordsPage(pageFromResponse(records))
    } catch (error) {
      setUsageError(formatAdminError(error))
    } finally {
      setLoadingUsage(false)
    }
  }, [adminApi, canReadUsage, summaryPageNum, usageDimension, usageRecordsPageNum])

  const loadOperationLogs = useCallback(async () => {
    if (!canReadAudit) {
      return
    }

    setLoadingOperationLogs(true)
    setOperationLogsError(null)
    try {
      const page = await adminApi.listOperationLogs({
        pageNum: operationLogsPageNum,
        pageSize: PAGE_SIZE,
        sortBy: 'createdAt',
        sortOrder: 'desc',
      })
      setOperationLogsPage(pageFromResponse(page))
    } catch (error) {
      setOperationLogsError(formatAdminError(error))
    } finally {
      setLoadingOperationLogs(false)
    }
  }, [adminApi, canReadAudit, operationLogsPageNum])

  const loadApiCallLogs = useCallback(async () => {
    if (!canReadAudit) {
      return
    }

    setLoadingApiCallLogs(true)
    setApiCallLogsError(null)
    try {
      const page = await adminApi.listApiCallLogs({
        pageNum: apiCallLogsPageNum,
        pageSize: PAGE_SIZE,
        sortBy: 'createdAt',
        sortOrder: 'desc',
      })
      setApiCallLogsPage(pageFromResponse(page))
    } catch (error) {
      setApiCallLogsError(formatAdminError(error))
    } finally {
      setLoadingApiCallLogs(false)
    }
  }, [adminApi, apiCallLogsPageNum, canReadAudit])

  const loadSettings = useCallback(async () => {
    if (!canManageSystemSettings) {
      return
    }

    setLoadingSettings(true)
    setSettingsError(null)
    setSettingsNotice(null)
    try {
      const settings = await adminApi.getSystemSettings()
      setSystemSettings(settings)
      setSettingsDraft(draftFromSettings(settings))
    } catch (error) {
      setSettingsError(formatAdminError(error))
    } finally {
      setLoadingSettings(false)
    }
  }, [adminApi, canManageSystemSettings])

  useEffect(() => {
    if (!isOpen || activeTab !== 'usage' || !canReadUsage) {
      return
    }
    void loadUsage()
  }, [activeTab, canReadUsage, isOpen, loadUsage])

  useEffect(() => {
    if (!isOpen || activeTab !== 'operations' || !canReadAudit) {
      return
    }
    void loadOperationLogs()
  }, [activeTab, canReadAudit, isOpen, loadOperationLogs])

  useEffect(() => {
    if (!isOpen || activeTab !== 'apiCalls' || !canReadAudit) {
      return
    }
    void loadApiCallLogs()
  }, [activeTab, canReadAudit, isOpen, loadApiCallLogs])

  useEffect(() => {
    if (!isOpen || activeTab !== 'settings' || !canManageSystemSettings) {
      return
    }
    void loadSettings()
  }, [activeTab, canManageSystemSettings, isOpen, loadSettings])

  const loadApiCallDetail = async (id: ApiCallLog['id']) => {
    if (!isOpen || !canReadAudit) {
      return
    }

    const requestSeq = apiCallDetailRequestSeqRef.current + 1
    apiCallDetailRequestSeqRef.current = requestSeq
    setSelectedApiCallLogId(id)
    setSelectedApiCallLog(null)
    setApiCallDetailError(null)
    setLoadingApiCallDetailId(id)
    try {
      const detail = await adminApi.getApiCallLog(id)
      if (apiCallDetailRequestSeqRef.current !== requestSeq || detail.id !== id) {
        return
      }
      setSelectedApiCallLog(detail)
    } catch (error) {
      if (apiCallDetailRequestSeqRef.current !== requestSeq) {
        return
      }
      setApiCallDetailError(formatAdminError(error))
    } finally {
      if (apiCallDetailRequestSeqRef.current === requestSeq) {
        setLoadingApiCallDetailId(null)
      }
    }
  }

  const saveSystemSettings = async () => {
    if (!csrfToken) {
      setSettingsError('登录状态缺少 CSRF 凭据，请重新登录。')
      setSettingsNotice(null)
      return
    }

    setSavingSettings(true)
    setSettingsError(null)
    setSettingsNotice(null)
    try {
      const saved = await adminApi.updateSystemSettings(
        {
          uploadPolicy: parseUploadPolicyDraft(settingsDraft),
        },
        csrfToken,
      )
      setSystemSettings(saved)
      setSettingsDraft(draftFromSettings(saved))
      setSettingsNotice('上传策略已更新。')
    } catch (error) {
      setSettingsError(formatAdminError(error))
    } finally {
      setSavingSettings(false)
    }
  }

  if (availableTabs.length === 0) {
    return null
  }

  return (
    <Modal isOpen={isOpen} maxWidthClass="max-w-7xl" onClose={onClose} title="管理端观测与设置">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="inline-flex rounded-md border border-ink-200 bg-ink-50 p-1">
            {availableTabs.map((tab) => (
              <button className={tabClassName(activeTab === tab)} key={tab} onClick={() => setActiveTab(tab)} type="button">
                {tabLabel(tab)}
              </button>
            ))}
          </div>

          <Button
            icon={activeTabIsLoading(activeTab, {
              isLoadingApiCallLogs,
              isLoadingOperationLogs,
              isLoadingSettings,
              isLoadingUsage,
            }) ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
            onClick={() => {
              if (activeTab === 'usage') {
                void loadUsage()
              } else if (activeTab === 'operations') {
                void loadOperationLogs()
              } else if (activeTab === 'apiCalls') {
                void loadApiCallLogs()
              } else {
                void loadSettings()
              }
            }}
            variant="secondary"
          >
            刷新
          </Button>
        </div>

        {activeTab === 'usage' ? (
          <UsageView
            dimension={usageDimension}
            error={usageError}
            isLoading={isLoadingUsage}
            onDimensionChange={(dimension) => {
              setUsageDimension(dimension)
              setSummaryPageNum(1)
            }}
            onRefresh={() => void loadUsage()}
            onSummaryPageChange={setSummaryPageNum}
            onUsageRecordsPageChange={setUsageRecordsPageNum}
            summaryPage={summaryPage}
            usageRecordsPage={usageRecordsPage}
          />
        ) : null}

        {activeTab === 'operations' ? (
          <OperationLogsView
            error={operationLogsError}
            isLoading={isLoadingOperationLogs}
            onPageChange={setOperationLogsPageNum}
            page={operationLogsPage}
          />
        ) : null}

        {activeTab === 'apiCalls' ? (
          <AdminApiCallLogsView
            detail={selectedApiCallLog}
            detailError={apiCallDetailError}
            isLoading={isLoadingApiCallLogs}
            loadingDetailId={loadingApiCallDetailId}
            error={apiCallLogsError}
            onLoadDetail={(id) => void loadApiCallDetail(id)}
            onPageChange={(pageNum) => {
              resetApiCallDetail()
              setApiCallLogsPageNum(pageNum)
            }}
            page={apiCallLogsPage}
            selectedDetailId={selectedApiCallLogId}
          />
        ) : null}

        {activeTab === 'settings' ? (
          <SystemSettingsView
            draft={settingsDraft}
            error={settingsError}
            isLoading={isLoadingSettings}
            isSaving={isSavingSettings}
            notice={settingsNotice}
            onDraftChange={setSettingsDraft}
            onSubmit={() => void saveSystemSettings()}
            settings={systemSettings}
          />
        ) : null}
      </div>
    </Modal>
  )
}

interface UsageViewProps {
  dimension: UsageSummaryDimension
  summaryPage: PageViewState<UsageSummary>
  usageRecordsPage: PageViewState<UsageRecord>
  isLoading: boolean
  error: string | null
  onDimensionChange: (dimension: UsageSummaryDimension) => void
  onSummaryPageChange: (pageNum: number) => void
  onUsageRecordsPageChange: (pageNum: number) => void
  onRefresh: () => void
}

function UsageView({
  dimension,
  summaryPage,
  usageRecordsPage,
  isLoading,
  error,
  onDimensionChange,
  onSummaryPageChange,
  onUsageRecordsPageChange,
  onRefresh,
}: UsageViewProps) {
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <BarChart3 className="h-4 w-4 text-ink-500" />
          <h3 className="text-sm font-semibold text-ink-900">用量汇总</h3>
        </div>
        <label className="flex items-center gap-2 text-sm text-ink-700">
          <span className="field-label">汇总维度</span>
          <select
            className="field-input min-w-32"
            onChange={(event) => onDimensionChange(event.target.value as UsageSummaryDimension)}
            value={dimension}
          >
            {usageDimensions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      </div>

      <StatusMessage message={error} tone="error" />
      {isLoading ? <LoadingState text="正在加载用量数据..." /> : null}

      {!isLoading && !error && summaryPage.records.length === 0 ? <EmptyState body="后端返回空分页，当前筛选下暂无汇总数据。" title="暂无用量汇总" /> : null}
      {!isLoading && summaryPage.records.length > 0 ? <UsageSummaryGrid records={summaryPage.records} /> : null}
      <PaginationControls isLoading={isLoading} label="用量汇总分页" onPageChange={onSummaryPageChange} page={summaryPage} />

      <div className="border-t border-ink-200 pt-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div>
            <h3 className="text-sm font-semibold text-ink-900">用量记录</h3>
            <p className="text-xs text-ink-400">分页读取后端 usage records。</p>
          </div>
          <button aria-label="刷新用量记录" className="icon-button" disabled={isLoading} onClick={onRefresh} title="刷新用量记录" type="button">
            <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
        </div>
        {!isLoading && !error && usageRecordsPage.records.length === 0 ? <EmptyState body="当前分页没有 usage record。" title="暂无用量记录" /> : null}
        {!isLoading && usageRecordsPage.records.length > 0 ? <UsageRecordTable records={usageRecordsPage.records} /> : null}
        <PaginationControls isLoading={isLoading} label="用量记录分页" onPageChange={onUsageRecordsPageChange} page={usageRecordsPage} />
      </div>
    </section>
  )
}

function UsageSummaryGrid({ records }: { records: UsageSummary[] }) {
  return (
    <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
      {records.map((record) => (
        <article className="rounded-lg border border-ink-200 bg-white p-3" key={`${record.dimension}-${record.dimensionId}`}>
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold text-ink-900">{record.dimensionId || '未归类'}</p>
              <p className="mt-1 text-xs text-ink-400">
                {record.dimension} · {record.recordCount} 条
              </p>
            </div>
            <span className="rounded-md bg-ink-100 px-2 py-1 text-xs font-semibold text-ink-600">{record.currency || 'N/A'}</span>
          </div>
          <dl className="mt-3 grid grid-cols-2 gap-2 text-xs text-ink-500">
            <Metric label="输入 tokens" value={formatNumber(record.inputTokens)} />
            <Metric label="输出 tokens" value={formatNumber(record.outputTokens)} />
            <Metric label="图片" value={formatNumber(record.imageCount)} />
            <Metric label="成本" value={record.estimatedCost} />
          </dl>
          <p className="mt-2 text-xs text-ink-400">最近：{formatDateTime(record.latestCreatedAt)}</p>
        </article>
      ))}
    </div>
  )
}

function UsageRecordTable({ records }: { records: UsageRecord[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-ink-200">
      <table className="min-w-full divide-y divide-ink-200 text-left text-sm">
        <thead className="bg-ink-50 text-xs font-semibold text-ink-500">
          <tr>
            <th className="px-3 py-2">记录</th>
            <th className="px-3 py-2">任务</th>
            <th className="px-3 py-2">Provider / 模型</th>
            <th className="px-3 py-2">用量</th>
            <th className="px-3 py-2">Raw usage</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-ink-100 bg-white">
          {records.map((record) => (
            <tr key={record.id}>
              <td className="max-w-[160px] px-3 py-2 align-top">
                <p className="truncate font-semibold text-ink-900">{record.id}</p>
                <p className="text-xs text-ink-400">{formatDateTime(record.createdAt)}</p>
              </td>
              <td className="max-w-[180px] px-3 py-2 align-top text-xs text-ink-600">
                <p className="truncate">{record.taskId}</p>
                <p className="truncate">{record.projectId}</p>
              </td>
              <td className="max-w-[180px] px-3 py-2 align-top text-xs text-ink-600">
                <p className="truncate">{record.providerId}</p>
                <p className="truncate">{record.modelId}</p>
              </td>
              <td className="px-3 py-2 align-top text-xs text-ink-600">
                <p>{record.inputTokens} / {record.outputTokens} tokens</p>
                <p>{record.imageCount} 图 · {record.estimatedCost} {record.currency}</p>
              </td>
              <td className="min-w-[220px] px-3 py-2 align-top">
                <MetadataPreview value={record.rawUsage} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

interface OperationLogsViewProps {
  page: PageViewState<OperationLog>
  isLoading: boolean
  error: string | null
  onPageChange: (pageNum: number) => void
}

function OperationLogsView({ page, isLoading, error, onPageChange }: OperationLogsViewProps) {
  return (
    <section className="space-y-3">
      <div className="flex items-center gap-2">
        <ClipboardList className="h-4 w-4 text-ink-500" />
        <h3 className="text-sm font-semibold text-ink-900">操作日志</h3>
      </div>
      <StatusMessage message={error} tone="error" />
      {isLoading ? <LoadingState text="正在加载操作日志..." /> : null}
      {!isLoading && !error && page.records.length === 0 ? <EmptyState body="后端返回空分页，当前没有 operation logs。" title="暂无操作日志" /> : null}
      {!isLoading && page.records.length > 0 ? (
        <div className="space-y-2">
          {page.records.map((record) => (
            <article className="rounded-lg border border-ink-200 bg-white p-3" key={record.id}>
              <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink-900">{record.action}</p>
                  <p className="mt-1 truncate text-xs text-ink-500">
                    {record.resourceType}:{record.resourceId} · {formatDateTime(record.createdAt)}
                  </p>
                  <p className="mt-1 truncate text-xs text-ink-400">actor: {record.actorUserId ?? 'system'}</p>
                </div>
                <span className="rounded-md bg-ink-100 px-2 py-1 text-xs font-semibold text-ink-600">{record.ip || 'no-ip'}</span>
              </div>
              <div className="mt-3">
                <MetadataPreview value={record.metadata} />
              </div>
            </article>
          ))}
        </div>
      ) : null}
      <PaginationControls isLoading={isLoading} label="操作日志分页" onPageChange={onPageChange} page={page} />
    </section>
  )
}

interface SystemSettingsViewProps {
  settings: SystemSettings | null
  draft: UploadPolicyDraft
  isLoading: boolean
  isSaving: boolean
  error: string | null
  notice: string | null
  onDraftChange: (draft: UploadPolicyDraft | ((current: UploadPolicyDraft) => UploadPolicyDraft)) => void
  onSubmit: () => void
}

function SystemSettingsView({
  settings,
  draft,
  isLoading,
  isSaving,
  error,
  notice,
  onDraftChange,
  onSubmit,
}: SystemSettingsViewProps) {
  return (
    <section className="space-y-4">
      <div className="flex items-center gap-2">
        <Settings2 className="h-4 w-4 text-ink-500" />
        <h3 className="text-sm font-semibold text-ink-900">系统设置</h3>
      </div>
      <StatusMessage message={error} tone="error" />
      <StatusMessage message={notice} tone="success" />
      {isLoading ? <LoadingState text="正在加载系统设置..." /> : null}
      {!isLoading && !error && !settings ? <EmptyState body="尚未读取到后端 system settings。" title="暂无系统设置" /> : null}
      <form
        className="grid gap-4 rounded-lg border border-ink-200 bg-ink-50 p-4"
        onSubmit={(event) => {
          event.preventDefault()
          onSubmit()
        }}
      >
        <div>
          <h4 className="text-sm font-semibold text-ink-900">Upload policy</h4>
          <p className="mt-1 text-xs text-ink-400">仅显示后端已生效并由上传校验消费的四个字段。</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <NumberField
            label="最大文件字节数"
            min={1}
            onChange={(value) => onDraftChange((current) => ({ ...current, maxFileSizeBytes: value }))}
            value={draft.maxFileSizeBytes}
          />
          <NumberField
            label="最大宽度"
            min={1}
            onChange={(value) => onDraftChange((current) => ({ ...current, maxWidth: value }))}
            value={draft.maxWidth}
          />
          <NumberField
            label="最大高度"
            min={1}
            onChange={(value) => onDraftChange((current) => ({ ...current, maxHeight: value }))}
            value={draft.maxHeight}
          />
          <NumberField
            label="最大像素数"
            min={1}
            onChange={(value) => onDraftChange((current) => ({ ...current, maxPixels: value }))}
            value={draft.maxPixels}
          />
        </div>
        <Button
          className="justify-self-start"
          disabled={isLoading || isSaving}
          icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
          type="submit"
          variant="primary"
        >
          保存上传策略
        </Button>
      </form>
    </section>
  )
}

function NumberField({ label, min, value, onChange }: { label: string; min: number; value: string; onChange: (value: string) => void }) {
  const id = `admin-observability-${label}`

  return (
    <label className="grid gap-1 text-sm text-ink-700" htmlFor={id}>
      <span className="field-label">{label}</span>
      <input className="field-input" id={id} min={min} onChange={(event) => onChange(event.target.value)} type="number" value={value} />
    </label>
  )
}

function PaginationControls<TRecord>({
  page,
  isLoading,
  label,
  onPageChange,
}: {
  page: PageViewState<TRecord>
  isLoading: boolean
  label: string
  onPageChange: (pageNum: number) => void
}) {
  const hasPrevious = page.pageNum > 1
  const hasNext = page.pageNum * page.pageSize < page.total

  return (
    <div aria-label={label} className="flex flex-wrap items-center justify-between gap-2 text-xs text-ink-500">
      <span>
        第 {page.pageNum} 页 · 共 {page.total} 条 · 每页 {page.pageSize} 条
      </span>
      <div className="flex gap-2">
        <button
          className="rounded-md border border-ink-200 bg-white px-3 py-1.5 font-semibold text-ink-700 transition hover:bg-ink-50 disabled:text-ink-300"
          disabled={!hasPrevious || isLoading}
          onClick={() => onPageChange(page.pageNum - 1)}
          type="button"
        >
          上一页
        </button>
        <button
          className="rounded-md border border-ink-200 bg-white px-3 py-1.5 font-semibold text-ink-700 transition hover:bg-ink-50 disabled:text-ink-300"
          disabled={!hasNext || isLoading}
          onClick={() => onPageChange(page.pageNum + 1)}
          type="button"
        >
          下一页
        </button>
      </div>
    </div>
  )
}

function MetadataPreview({ value }: { value: unknown }) {
  return (
    <pre className="max-h-28 overflow-auto whitespace-pre-wrap break-words rounded-md bg-ink-50 px-3 py-2 text-xs leading-5 text-ink-600">
      {formatRedactedJson(value, METADATA_PREVIEW_LIMIT)}
    </pre>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md border border-ink-200 bg-white px-3 py-2">
      <dt className="truncate text-ink-400">{label}</dt>
      <dd className="mt-1 break-words font-semibold text-ink-800">{value}</dd>
    </div>
  )
}

function LoadingState({ text }: { text: string }) {
  return (
    <div className="flex items-center justify-center gap-2 rounded-md bg-ink-50 px-4 py-8 text-sm text-ink-500">
      <Loader2 className="h-4 w-4 animate-spin" />
      <span>{text}</span>
    </div>
  )
}

function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
      <p className="text-sm font-medium text-ink-700">{title}</p>
      <p className="mt-1 text-xs text-ink-400">{body}</p>
    </div>
  )
}

function StatusMessage({ message, tone }: { message: string | null; tone: 'error' | 'success' }) {
  if (!message) {
    return null
  }

  const className =
    tone === 'error'
      ? 'border-red-200 bg-red-50 text-red-700'
      : 'border-emerald-200 bg-emerald-50 text-emerald-700'

  return (
    <div className={`flex items-start gap-2 rounded-md border px-3 py-2 text-sm leading-6 ${className}`} role={tone === 'error' ? 'alert' : 'status'}>
      {tone === 'error' ? <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" /> : <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />}
      <span>{message}</span>
    </div>
  )
}

function getAvailableTabs({
  canManageSystemSettings,
  canReadAudit,
  canReadUsage,
}: Pick<AdminObservabilitySettingsPanelProps, 'canManageSystemSettings' | 'canReadAudit' | 'canReadUsage'>): AdminObservabilityTab[] {
  const tabs: AdminObservabilityTab[] = []
  if (canReadUsage) {
    tabs.push('usage')
  }
  if (canReadAudit) {
    tabs.push('operations', 'apiCalls')
  }
  if (canManageSystemSettings) {
    tabs.push('settings')
  }
  return tabs
}

function emptyPage<TRecord>(): PageViewState<TRecord> {
  return {
    records: [],
    total: 0,
    pageNum: 1,
    pageSize: PAGE_SIZE,
  }
}

function pageFromResponse<TRecord>(page: ApiPage<TRecord>): PageViewState<TRecord> {
  return {
    records: page.records,
    total: page.total,
    pageNum: page.pageNum,
    pageSize: page.pageSize,
  }
}

function emptyUploadPolicyDraft(): UploadPolicyDraft {
  return {
    maxFileSizeBytes: '',
    maxWidth: '',
    maxHeight: '',
    maxPixels: '',
  }
}

function draftFromSettings(settings: SystemSettings): UploadPolicyDraft {
  return {
    maxFileSizeBytes: String(settings.uploadPolicy.maxFileSizeBytes),
    maxWidth: String(settings.uploadPolicy.maxWidth),
    maxHeight: String(settings.uploadPolicy.maxHeight),
    maxPixels: String(settings.uploadPolicy.maxPixels),
  }
}

function parseUploadPolicyDraft(draft: UploadPolicyDraft): SystemSettings['uploadPolicy'] {
  return {
    maxFileSizeBytes: parsePositiveInteger(draft.maxFileSizeBytes, '最大文件字节数'),
    maxWidth: parsePositiveInteger(draft.maxWidth, '最大宽度'),
    maxHeight: parsePositiveInteger(draft.maxHeight, '最大高度'),
    maxPixels: parsePositiveInteger(draft.maxPixels, '最大像素数'),
  }
}

function parsePositiveInteger(value: string, label: string): number {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${label}必须是正整数。`)
  }
  return parsed
}

function activeTabIsLoading(
  tab: AdminObservabilityTab,
  state: {
    isLoadingUsage: boolean
    isLoadingOperationLogs: boolean
    isLoadingApiCallLogs: boolean
    isLoadingSettings: boolean
  },
): boolean {
  if (tab === 'usage') {
    return state.isLoadingUsage
  }
  if (tab === 'operations') {
    return state.isLoadingOperationLogs
  }
  if (tab === 'apiCalls') {
    return state.isLoadingApiCallLogs
  }
  return state.isLoadingSettings
}

function tabLabel(tab: AdminObservabilityTab): string {
  if (tab === 'usage') {
    return '用量'
  }
  if (tab === 'operations') {
    return '操作日志'
  }
  if (tab === 'apiCalls') {
    return 'API 调用日志'
  }
  return '系统设置'
}

function tabClassName(active: boolean): string {
  return `rounded px-3 py-1.5 text-sm font-semibold transition ${
    active ? 'bg-white text-ink-900 shadow-sm' : 'text-ink-500 hover:text-ink-900'
  }`
}

function formatAdminError(error: unknown): string {
  if (isApiClientError(error)) {
    if (error.status === 401) {
      return '登录状态已过期，请重新登录。'
    }
    if (error.status === 403) {
      return '当前账号没有此管理权限。'
    }
    if (error.status === 404) {
      return '记录不存在或已不可见。'
    }
    if (error.status === 422) {
      return `表单内容未通过校验：${error.message}`
    }
    return error.message
  }

  if (error instanceof Error) {
    return error.message
  }

  return '请求失败，请稍后重试。'
}

function formatRedactedJson(value: unknown, maxLength: number): string {
  let rendered: string
  try {
    rendered = JSON.stringify(value ?? {}, null, 2)
  } catch {
    rendered = String(value)
  }

  if (rendered.length <= maxLength) {
    return rendered
  }
  return `${rendered.slice(0, maxLength)}\n... 内容已截断`
}

function formatDateTime(value: string): string {
  if (!value) {
    return '未返回'
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(parsed)
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value)
}
