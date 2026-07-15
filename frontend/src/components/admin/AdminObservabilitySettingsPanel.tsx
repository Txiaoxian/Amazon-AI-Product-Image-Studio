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
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { adminApi as defaultAdminApi, type AdminApi } from '../../api/admin'
import { isApiClientError } from '../../api/client'
import { modelApi as defaultModelApi, type ModelApi } from '../../api/models'
import { providerApi as defaultProviderApi, type ProviderApi } from '../../api/providers'
import type {
  AdminUsageQuery,
  ApiCallLog,
  IntegerRangeConstraint,
  OperationLog,
  SystemSettings,
  UpdateSystemSettingsRequest,
  UsageRecord,
  UsageSummary,
  UsageSummaryDimension,
} from '../../types/admin'
import type { ApiPage } from '../../types/api'
import type { Model, Provider } from '../../types/platform'
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
  modelApi?: ModelApi
  providerApi?: ProviderApi
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

interface TaskDefaultsDraft {
  defaultProviderId: string
  defaultModelId: string
}

interface TaskConcurrencyDraft {
  tenantLimit: string
  userLimit: string
  providerLimit: string
  modelLimit: string
}

interface StorageRetentionDraft {
  enabled: boolean
  deletedAssetRetentionDays: string
}

interface StorageQuotaDraft {
  enabled: boolean
  maxBytes: string
}

interface LogRetentionDraft {
  operationLogEnabled: boolean
  operationLogRetentionDays: string
  apiCallLogEnabled: boolean
  apiCallLogRetentionDays: string
  taskEventEnabled: boolean
  taskEventRetentionDays: string
}

interface SystemSettingsDraft {
  uploadPolicy: UploadPolicyDraft
  taskDefaults: TaskDefaultsDraft
  taskConcurrency: TaskConcurrencyDraft
  storageRetention: StorageRetentionDraft
  storageQuota: StorageQuotaDraft
  logRetention: LogRetentionDraft
}

type SystemSettingsGroup = keyof SystemSettingsDraft
type SettingsGroupFeedback = { message: string; field?: string }
type SettingsGroupFeedbackMap = Partial<Record<SystemSettingsGroup, SettingsGroupFeedback>>
type SettingsGroupNoticeMap = Partial<Record<SystemSettingsGroup, string>>
type UsageFilterField = 'createdAtFrom' | 'createdAtTo' | 'taskId' | 'userId' | 'projectId' | 'providerId' | 'modelId'
type UsageFiltersDraft = Record<UsageFilterField, string>

const PAGE_SIZE = 10
const TENANT_TOTALS_PAGE_SIZE = 50
const METADATA_PREVIEW_LIMIT = 1200

const usageDimensions: Array<{ value: UsageSummaryDimension; label: string }> = [
  { value: 'tenant', label: '租户' },
  { value: 'provider', label: 'Provider' },
  { value: 'model', label: '模型' },
  { value: 'project', label: '产品' },
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
  modelApi = defaultModelApi,
  providerApi = defaultProviderApi,
}: AdminObservabilitySettingsPanelProps) {
  const apiCallDetailRequestSeqRef = useRef(0)
  const usageRequestSeqRef = useRef(0)
  const availableTabs = useMemo(
    () => getAvailableTabs({ canManageSystemSettings, canReadAudit, canReadUsage }),
    [canManageSystemSettings, canReadAudit, canReadUsage],
  )
  const [activeTab, setActiveTab] = useState<AdminObservabilityTab>(availableTabs[0] ?? 'usage')
  const [usageDimension, setUsageDimension] = useState<UsageSummaryDimension>('provider')
  const [usageFiltersDraft, setUsageFiltersDraft] = useState<UsageFiltersDraft>(() => emptyUsageFiltersDraft())
  const [usageFilters, setUsageFilters] = useState<UsageFiltersDraft>(() => emptyUsageFiltersDraft())
  const [summaryPageNum, setSummaryPageNum] = useState(1)
  const [usageRecordsPageNum, setUsageRecordsPageNum] = useState(1)
  const [operationLogsPageNum, setOperationLogsPageNum] = useState(1)
  const [apiCallLogsPageNum, setApiCallLogsPageNum] = useState(1)
  const [summaryPage, setSummaryPage] = useState<PageViewState<UsageSummary>>(emptyPage())
  const [tenantTotalsPage, setTenantTotalsPage] = useState<PageViewState<UsageSummary>>(emptyPage())
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
  const [settingsGroupErrors, setSettingsGroupErrors] = useState<SettingsGroupFeedbackMap>({})
  const [settingsGroupNotices, setSettingsGroupNotices] = useState<SettingsGroupNoticeMap>({})
  const [systemSettings, setSystemSettings] = useState<SystemSettings | null>(null)
  const [settingsDraft, setSettingsDraft] = useState<SystemSettingsDraft>(() => emptySystemSettingsDraft())
  const [savingSettingsGroups, setSavingSettingsGroups] = useState<Set<SystemSettingsGroup>>(() => new Set())
  const [settingsProviders, setSettingsProviders] = useState<Provider[]>([])
  const [settingsModels, setSettingsModels] = useState<Model[]>([])
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
      usageRequestSeqRef.current += 1
      setSettingsGroupErrors({})
      setSettingsGroupNotices({})
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

    const requestSeq = usageRequestSeqRef.current + 1
    usageRequestSeqRef.current = requestSeq
    setLoadingUsage(true)
    setUsageError(null)
    try {
      const filters = usageFiltersToQuery(usageFilters)
      const [tenantTotals, summary, records] = await Promise.all([
        adminApi
          .getUsageSummary({
            ...filters,
            dimension: 'tenant',
            pageNum: 1,
            pageSize: TENANT_TOTALS_PAGE_SIZE,
            sortBy: 'createdAt',
            sortOrder: 'desc',
          })
          .catch((error) => {
            if (isApiClientError(error) && error.status === 404) {
              return emptyApiPage<UsageSummary>(TENANT_TOTALS_PAGE_SIZE)
            }
            throw error
          }),
        adminApi.getUsageSummary({
          ...filters,
          dimension: usageDimension,
          pageNum: summaryPageNum,
          pageSize: PAGE_SIZE,
          sortBy: 'createdAt',
          sortOrder: 'desc',
        }),
        adminApi.listUsageRecords({
          ...filters,
          pageNum: usageRecordsPageNum,
          pageSize: PAGE_SIZE,
          sortBy: 'createdAt',
          sortOrder: 'desc',
        }),
      ])
      if (usageRequestSeqRef.current !== requestSeq) {
        return
      }
      setTenantTotalsPage(pageFromResponse(tenantTotals))
      setSummaryPage(pageFromResponse(summary))
      setUsageRecordsPage(pageFromResponse(records))
    } catch (error) {
      if (usageRequestSeqRef.current !== requestSeq) {
        return
      }
      setUsageError(formatAdminError(error))
    } finally {
      if (usageRequestSeqRef.current === requestSeq) {
        setLoadingUsage(false)
      }
    }
  }, [adminApi, canReadUsage, summaryPageNum, usageDimension, usageFilters, usageRecordsPageNum])

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
    setSettingsGroupErrors({})
    setSettingsGroupNotices({})
    try {
      const [settingsResult, providerResult, modelResult] = await Promise.allSettled([
        adminApi.getSystemSettings(),
        providerApi.list({ status: 'ENABLED', pageNum: 1, pageSize: 100 }),
        modelApi.list({ status: 'ENABLED', pageNum: 1, pageSize: 100 }),
      ])
      if (settingsResult.status === 'rejected') {
        throw settingsResult.reason
      }
      const settings = settingsResult.value
      setSystemSettings(settings)
      setSettingsDraft(draftFromSettings(settings))
      setSettingsProviders(providerResult.status === 'fulfilled' ? providerResult.value.records : [])
      setSettingsModels(modelResult.status === 'fulfilled' ? modelResult.value.records : [])
      if (providerResult.status === 'rejected' || modelResult.status === 'rejected') {
        setSettingsGroupErrors({
          taskDefaults: { message: 'Provider 或模型选项加载失败，其他设置仍可查看和保存。请刷新后重试。' },
        })
      }
    } catch (error) {
      setSettingsError(formatAdminError(error))
    } finally {
      setLoadingSettings(false)
    }
  }, [adminApi, canManageSystemSettings, modelApi, providerApi])

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

  const saveSystemSettingsGroup = async (group: SystemSettingsGroup) => {
    if (!csrfToken) {
      setSettingsGroupErrors((current) => ({
        ...current,
        [group]: { message: '登录状态缺少 CSRF 凭据，请重新登录。' },
      }))
      return
    }

    setSavingSettingsGroups((current) => new Set(current).add(group))
    setSettingsGroupErrors((current) => ({ ...current, [group]: undefined }))
    setSettingsGroupNotices((current) => ({ ...current, [group]: undefined }))
    try {
      const saved = await adminApi.updateSystemSettings(parseSettingsGroupPatch(group, settingsDraft), csrfToken)
      const savedDraft = draftFromSettings(saved)
      setSystemSettings((current) => mergeSavedSettingsGroup(current, saved, group))
      setSettingsDraft((current) => mergeSavedDraftGroup(current, savedDraft, group))
      setSettingsGroupNotices((current) => ({ ...current, [group]: `${settingsGroupLabel(group)}已更新。` }))
    } catch (error) {
      setSettingsGroupErrors((current) => ({ ...current, [group]: formatSettingsGroupError(error) }))
    } finally {
      setSavingSettingsGroups((current) => {
        const next = new Set(current)
        next.delete(group)
        return next
      })
    }
  }

  const updateSettingsDraft = (
    group: SystemSettingsGroup,
    update: SystemSettingsDraft | ((current: SystemSettingsDraft) => SystemSettingsDraft),
  ) => {
    setSettingsDraft(update)
    setSettingsGroupErrors((current) => ({ ...current, [group]: undefined }))
    setSettingsGroupNotices((current) => ({ ...current, [group]: undefined }))
  }

  const applyUsageFilters = () => {
    setUsageFilters(normalizeUsageFilters(usageFiltersDraft))
    setSummaryPageNum(1)
    setUsageRecordsPageNum(1)
  }

  const clearUsageFilters = () => {
    const emptyFilters = emptyUsageFiltersDraft()
    setUsageFiltersDraft(emptyFilters)
    setUsageFilters(emptyFilters)
    setSummaryPageNum(1)
    setUsageRecordsPageNum(1)
  }

  const drillDownUsageSummary = (summary: UsageSummary) => {
    const filterField = usageSummaryFilterField(summary.dimension)
    if (!filterField) {
      setUsageRecordsPageNum(1)
      return
    }

    const nextFilters = normalizeUsageFilters({
      ...usageFilters,
      [filterField]: summary.dimensionId,
    })
    setUsageFiltersDraft(nextFilters)
    setUsageFilters(nextFilters)
    setSummaryPageNum(1)
    setUsageRecordsPageNum(1)
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
            filtersDraft={usageFiltersDraft}
            isLoading={isLoadingUsage}
            onApplyFilters={applyUsageFilters}
            onClearFilters={clearUsageFilters}
            onDimensionChange={(dimension) => {
              setUsageDimension(dimension)
              setSummaryPageNum(1)
            }}
            onFiltersDraftChange={setUsageFiltersDraft}
            onRefresh={() => void loadUsage()}
            onSummaryDrilldown={drillDownUsageSummary}
            onSummaryPageChange={setSummaryPageNum}
            onUsageRecordsPageChange={setUsageRecordsPageNum}
            summaryPage={summaryPage}
            tenantTotalsPage={tenantTotalsPage}
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
            onCloseDetail={resetApiCallDetail}
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
            groupErrors={settingsGroupErrors}
            groupNotices={settingsGroupNotices}
            isLoading={isLoadingSettings}
            models={settingsModels}
            onDraftChange={updateSettingsDraft}
            onSubmit={(group) => void saveSystemSettingsGroup(group)}
            providers={settingsProviders}
            savingGroups={savingSettingsGroups}
            settings={systemSettings}
          />
        ) : null}
      </div>
    </Modal>
  )
}

interface UsageViewProps {
  dimension: UsageSummaryDimension
  tenantTotalsPage: PageViewState<UsageSummary>
  summaryPage: PageViewState<UsageSummary>
  usageRecordsPage: PageViewState<UsageRecord>
  filtersDraft: UsageFiltersDraft
  isLoading: boolean
  error: string | null
  onApplyFilters: () => void
  onClearFilters: () => void
  onDimensionChange: (dimension: UsageSummaryDimension) => void
  onFiltersDraftChange: (draft: UsageFiltersDraft | ((current: UsageFiltersDraft) => UsageFiltersDraft)) => void
  onSummaryDrilldown: (summary: UsageSummary) => void
  onSummaryPageChange: (pageNum: number) => void
  onUsageRecordsPageChange: (pageNum: number) => void
  onRefresh: () => void
}

function UsageView({
  dimension,
  tenantTotalsPage,
  summaryPage,
  usageRecordsPage,
  filtersDraft,
  isLoading,
  error,
  onApplyFilters,
  onClearFilters,
  onDimensionChange,
  onFiltersDraftChange,
  onSummaryDrilldown,
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

      <UsageFilterForm
        disabled={isLoading}
        draft={filtersDraft}
        onApply={onApplyFilters}
        onClear={onClearFilters}
        onDraftChange={onFiltersDraftChange}
      />

      <StatusMessage message={error} tone="error" />
      {isLoading ? <LoadingState text="正在加载用量数据..." /> : null}

      {!isLoading && !error ? <TenantTotalsGrid records={tenantTotalsPage.records} /> : null}
      {!isLoading && !error && summaryPage.records.length === 0 ? <EmptyState body="后端返回空分页，当前筛选下暂无汇总数据。" title="暂无用量汇总" /> : null}
      {!isLoading && summaryPage.records.length > 0 ? <UsageSummaryGrid onDrilldown={onSummaryDrilldown} records={summaryPage.records} /> : null}
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

function UsageFilterForm({
  disabled,
  draft,
  onApply,
  onClear,
  onDraftChange,
}: {
  disabled: boolean
  draft: UsageFiltersDraft
  onApply: () => void
  onClear: () => void
  onDraftChange: (draft: UsageFiltersDraft | ((current: UsageFiltersDraft) => UsageFiltersDraft)) => void
}) {
  return (
    <form
      className="grid gap-3 rounded-lg border border-ink-200 bg-ink-50 p-3"
      onSubmit={(event) => {
        event.preventDefault()
        onApply()
      }}
    >
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <UsageFilterFieldInput
          label="开始时间"
          name="createdAtFrom"
          onChange={onDraftChange}
          type="date"
          value={draft.createdAtFrom}
        />
        <UsageFilterFieldInput label="结束时间" name="createdAtTo" onChange={onDraftChange} type="date" value={draft.createdAtTo} />
        <UsageFilterFieldInput label="Task ID" name="taskId" onChange={onDraftChange} value={draft.taskId} />
        <UsageFilterFieldInput label="User ID" name="userId" onChange={onDraftChange} value={draft.userId} />
        <UsageFilterFieldInput label="Project ID" name="projectId" onChange={onDraftChange} value={draft.projectId} />
        <UsageFilterFieldInput label="Provider ID" name="providerId" onChange={onDraftChange} value={draft.providerId} />
        <UsageFilterFieldInput label="Model ID" name="modelId" onChange={onDraftChange} value={draft.modelId} />
      </div>
      <div className="flex flex-wrap gap-2">
        <Button disabled={disabled} type="submit" variant="primary">
          应用筛选
        </Button>
        <Button disabled={disabled} onClick={onClear} type="button" variant="secondary">
          清空筛选
        </Button>
      </div>
    </form>
  )
}

function UsageFilterFieldInput({
  label,
  name,
  type = 'text',
  value,
  onChange,
}: {
  label: string
  name: UsageFilterField
  type?: 'date' | 'text'
  value: string
  onChange: (draft: UsageFiltersDraft | ((current: UsageFiltersDraft) => UsageFiltersDraft)) => void
}) {
  const id = `admin-usage-filter-${name}`
  return (
    <label className="grid min-w-0 gap-1 text-sm text-ink-700" htmlFor={id}>
      <span className="field-label">{label}</span>
      <input
        className="field-input min-w-0"
        id={id}
        onChange={(event) => onChange((current) => ({ ...current, [name]: event.target.value }))}
        type={type}
        value={value}
      />
    </label>
  )
}

function TenantTotalsGrid({ records }: { records: UsageSummary[] }) {
  return (
    <section className="space-y-2">
      <h4 className="text-sm font-semibold text-ink-900">租户总览</h4>
      {records.length === 0 ? (
        <EmptyState body="后端 tenant 汇总当前没有返回 totals。" title="暂无租户总览" />
      ) : (
        <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
          {records.map((record) => (
            <article className="rounded-lg border border-ink-200 bg-white p-3" key={`tenant-total-${record.currency}-${record.dimensionId}`}>
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink-900">{record.dimensionId || '当前租户'}</p>
                  <p className="mt-1 text-xs text-ink-400">{record.recordCount} 条 usage records</p>
                </div>
                <span className="rounded-md bg-ink-100 px-2 py-1 text-xs font-semibold text-ink-600">{record.currency || 'N/A'}</span>
              </div>
              <dl className="mt-3 grid grid-cols-2 gap-2 text-xs text-ink-500">
                <Metric label="输入 tokens" value={formatNumber(record.inputTokens)} />
                <Metric label="输出 tokens" value={formatNumber(record.outputTokens)} />
                <Metric label="图片" value={formatNumber(record.imageCount)} />
                <Metric label="成本" value={`${record.estimatedCost} ${record.currency || 'N/A'}`} />
              </dl>
            </article>
          ))}
        </div>
      )}
    </section>
  )
}

function UsageSummaryGrid({ records, onDrilldown }: { records: UsageSummary[]; onDrilldown: (summary: UsageSummary) => void }) {
  return (
    <div className="grid gap-2 md:grid-cols-2 xl:grid-cols-3">
      {records.map((record) => (
        <button
          className="rounded-lg border border-ink-200 bg-white p-3 text-left transition hover:border-ink-300 hover:bg-ink-50"
          key={`${record.dimension}-${record.dimensionId}-${record.currency}`}
          onClick={() => onDrilldown(record)}
          type="button"
        >
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
            <Metric label="成本" value={`${record.estimatedCost} ${record.currency || 'N/A'}`} />
          </dl>
          <p className="mt-2 text-xs text-ink-400">最近：{formatDateTime(record.latestCreatedAt)}</p>
        </button>
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
  draft: SystemSettingsDraft
  isLoading: boolean
  error: string | null
  groupErrors: SettingsGroupFeedbackMap
  groupNotices: SettingsGroupNoticeMap
  models: Model[]
  onDraftChange: (
    group: SystemSettingsGroup,
    draft: SystemSettingsDraft | ((current: SystemSettingsDraft) => SystemSettingsDraft),
  ) => void
  onSubmit: (group: SystemSettingsGroup) => void
  providers: Provider[]
  savingGroups: Set<SystemSettingsGroup>
}

function SystemSettingsView({
  settings,
  draft,
  isLoading,
  error,
  groupErrors,
  groupNotices,
  models,
  onDraftChange,
  onSubmit,
  providers,
  savingGroups,
}: SystemSettingsViewProps) {
  const savedDraft = settings ? draftFromSettings(settings) : null
  const availableModels = models.filter((model) => model.providerId === draft.taskDefaults.defaultProviderId)
  const groupProps = (group: SystemSettingsGroup) => ({
    disabled: isLoading || savingGroups.has(group),
    error: groupErrors[group]?.field ? undefined : groupErrors[group]?.message,
    isDirty: savedDraft ? !settingsDraftGroupEquals(draft, savedDraft, group) : false,
    isSaving: savingGroups.has(group),
    notice: groupNotices[group],
  })

  return (
    <section className="space-y-4">
      <div className="flex items-center gap-2">
        <Settings2 className="h-4 w-4 text-ink-500" />
        <h3 className="text-sm font-semibold text-ink-900">系统设置</h3>
      </div>
      <StatusMessage message={error} tone="error" />
      {isLoading ? <LoadingState text="正在加载系统设置..." /> : null}
      {!isLoading && !error && !settings ? <EmptyState body="尚未读取到后端系统设置。" title="暂无系统设置" /> : null}
      {settings ? (
        <div className="grid items-start gap-4 lg:grid-cols-2">
          <SettingsGroupForm
        group="uploadPolicy"
        onSubmit={onSubmit}
        title="上传策略"
        submitLabel="保存上传策略"
        {...groupProps('uploadPolicy')}
      >
        <div>
          <p className="mt-1 text-xs text-ink-400">仅显示后端已生效并由上传校验消费的四个字段。</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <NumberField
            error={fieldError(groupErrors.uploadPolicy, 'uploadPolicy.maxFileSizeBytes')}
            hint={formatByteRangeHint(settings?.constraints.uploadPolicy.maxFileSizeBytes, draft.uploadPolicy.maxFileSizeBytes)}
            label="最大文件字节数"
            max={settings?.constraints.uploadPolicy.maxFileSizeBytes.max}
            min={1}
            onChange={(value) => onDraftChange('uploadPolicy', (current) => ({ ...current, uploadPolicy: { ...current.uploadPolicy, maxFileSizeBytes: value } }))}
            value={draft.uploadPolicy.maxFileSizeBytes}
          />
          <NumberField
            error={fieldError(groupErrors.uploadPolicy, 'uploadPolicy.maxWidth')}
            hint={formatIntegerRangeHint(settings?.constraints.uploadPolicy.maxWidth, '像素')}
            label="最大宽度"
            max={settings?.constraints.uploadPolicy.maxWidth.max}
            min={1}
            onChange={(value) => onDraftChange('uploadPolicy', (current) => ({ ...current, uploadPolicy: { ...current.uploadPolicy, maxWidth: value } }))}
            value={draft.uploadPolicy.maxWidth}
          />
          <NumberField
            error={fieldError(groupErrors.uploadPolicy, 'uploadPolicy.maxHeight')}
            hint={formatIntegerRangeHint(settings?.constraints.uploadPolicy.maxHeight, '像素')}
            label="最大高度"
            max={settings?.constraints.uploadPolicy.maxHeight.max}
            min={1}
            onChange={(value) => onDraftChange('uploadPolicy', (current) => ({ ...current, uploadPolicy: { ...current.uploadPolicy, maxHeight: value } }))}
            value={draft.uploadPolicy.maxHeight}
          />
          <NumberField
            error={fieldError(groupErrors.uploadPolicy, 'uploadPolicy.maxPixels')}
            hint={formatIntegerRangeHint(settings?.constraints.uploadPolicy.maxPixels, '像素')}
            label="最大像素数"
            max={settings?.constraints.uploadPolicy.maxPixels.max}
            min={1}
            onChange={(value) => onDraftChange('uploadPolicy', (current) => ({ ...current, uploadPolicy: { ...current.uploadPolicy, maxPixels: value } }))}
            value={draft.uploadPolicy.maxPixels}
          />
        </div>
      </SettingsGroupForm>

      <SettingsGroupForm
        group="taskDefaults"
        onSubmit={onSubmit}
        title="任务默认模型"
        submitLabel="保存任务默认模型"
        {...groupProps('taskDefaults')}
      >
        <div>
          <p className="mt-1 text-xs text-ink-400">Provider 与模型必须成对选择；两项都不选择时会清除默认值。</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <SelectField
            error={fieldError(groupErrors.taskDefaults, 'taskDefaults.defaultProviderId')}
            hint="只能选择当前租户已启用的 Provider。"
            label="默认 Provider"
            onChange={(value) =>
              onDraftChange('taskDefaults', (current) => ({
                ...current,
                taskDefaults: {
                  defaultProviderId: value,
                  defaultModelId: models.some((model) => model.id === current.taskDefaults.defaultModelId && model.providerId === value)
                    ? current.taskDefaults.defaultModelId
                    : '',
                },
              }))
            }
            options={providers.map((provider) => ({ label: `${provider.name} · ${provider.type}`, value: provider.id }))}
            placeholder="不设置默认 Provider"
            value={draft.taskDefaults.defaultProviderId}
          />
          <SelectField
            disabled={!draft.taskDefaults.defaultProviderId}
            error={fieldError(groupErrors.taskDefaults, 'taskDefaults.defaultModelId')}
            hint="模型选项会根据已选择的 Provider 自动筛选。"
            label="默认模型"
            onChange={(value) =>
              onDraftChange('taskDefaults', (current) => ({ ...current, taskDefaults: { ...current.taskDefaults, defaultModelId: value } }))
            }
            options={availableModels.map((model) => ({ label: `${model.displayName} · ${model.modelName}`, value: model.id }))}
            placeholder={draft.taskDefaults.defaultProviderId ? '不设置默认模型' : '请先选择 Provider'}
            value={draft.taskDefaults.defaultModelId}
          />
        </div>
      </SettingsGroupForm>

      <SettingsGroupForm
        group="taskConcurrency"
        onSubmit={onSubmit}
        title="任务并发限制"
        submitLabel="保存并发限制"
        {...groupProps('taskConcurrency')}
      >
        <div>
          <p className="mt-1 text-xs text-ink-400">
            保存后会对新开始的任务立即生效，无需重启；降低限制不会中断正在生成的任务。当前全局安全容量为
            {' '}{settings?.constraints.taskConcurrency.globalCapacity ?? '—'} 个任务，实际吞吐量还会受到 Worker 数量和中转站能力限制。
          </p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <NumberField
            error={fieldError(groupErrors.taskConcurrency, 'taskConcurrency.tenantLimit')}
            hint={formatIntegerRangeHint(settings?.constraints.taskConcurrency.tenantLimit, '个并发任务')}
            label="租户并发上限"
            max={settings?.constraints.taskConcurrency.tenantLimit.max}
            min={1}
            onChange={(value) =>
              onDraftChange('taskConcurrency', (current) => ({ ...current, taskConcurrency: { ...current.taskConcurrency, tenantLimit: value } }))
            }
            value={draft.taskConcurrency.tenantLimit}
          />
          <NumberField
            error={fieldError(groupErrors.taskConcurrency, 'taskConcurrency.userLimit')}
            hint={formatIntegerRangeHint(settings?.constraints.taskConcurrency.userLimit, '个并发任务')}
            label="用户并发上限"
            max={settings?.constraints.taskConcurrency.userLimit.max}
            min={1}
            onChange={(value) =>
              onDraftChange('taskConcurrency', (current) => ({ ...current, taskConcurrency: { ...current.taskConcurrency, userLimit: value } }))
            }
            value={draft.taskConcurrency.userLimit}
          />
          <NumberField
            error={fieldError(groupErrors.taskConcurrency, 'taskConcurrency.providerLimit')}
            hint={formatIntegerRangeHint(settings?.constraints.taskConcurrency.providerLimit, '个并发任务')}
            label="Provider 并发上限"
            max={settings?.constraints.taskConcurrency.providerLimit.max}
            min={1}
            onChange={(value) =>
              onDraftChange('taskConcurrency', (current) => ({ ...current, taskConcurrency: { ...current.taskConcurrency, providerLimit: value } }))
            }
            value={draft.taskConcurrency.providerLimit}
          />
          <NumberField
            error={fieldError(groupErrors.taskConcurrency, 'taskConcurrency.modelLimit')}
            hint={formatIntegerRangeHint(settings?.constraints.taskConcurrency.modelLimit, '个并发任务')}
            label="模型并发上限"
            max={settings?.constraints.taskConcurrency.modelLimit.max}
            min={1}
            onChange={(value) =>
              onDraftChange('taskConcurrency', (current) => ({ ...current, taskConcurrency: { ...current.taskConcurrency, modelLimit: value } }))
            }
            value={draft.taskConcurrency.modelLimit}
          />
        </div>
      </SettingsGroupForm>

      <SettingsGroupForm
        group="storageRetention"
        onSubmit={onSubmit}
        title="删除资产保留期"
        submitLabel="保存删除资产保留期"
        {...groupProps('storageRetention')}
      >
        <div>
          <p className="mt-1 text-xs text-ink-400">关闭自动清理后，软删除资产不会按时间自动物理清理。</p>
        </div>
        <ToggleField
          checked={draft.storageRetention.enabled}
          label="启用软删除资产自动清理"
          onChange={(enabled) =>
            onDraftChange('storageRetention', (current) => ({
              ...current,
              storageRetention: {
                ...current.storageRetention,
                enabled,
                deletedAssetRetentionDays:
                  enabled && !current.storageRetention.deletedAssetRetentionDays
                    ? String(settings?.constraints.storageRetention.min ?? 1)
                    : current.storageRetention.deletedAssetRetentionDays,
              },
            }))
          }
        />
        <NumberField
          disabled={!draft.storageRetention.enabled}
          error={fieldError(groupErrors.storageRetention, 'storageRetention.deletedAssetRetentionDays')}
          hint={formatIntegerRangeHint(settings?.constraints.storageRetention, '天')}
          label="删除资产保留天数"
          max={settings?.constraints.storageRetention.max}
          min={settings?.constraints.storageRetention.min ?? 1}
          onChange={(value) =>
            onDraftChange('storageRetention', (current) => ({ ...current, storageRetention: { ...current.storageRetention, deletedAssetRetentionDays: value } }))
          }
          value={draft.storageRetention.deletedAssetRetentionDays}
        />
      </SettingsGroupForm>

      <SettingsGroupForm
        group="storageQuota"
        onSubmit={onSubmit}
        title="存储配额"
        submitLabel="保存存储配额"
        {...groupProps('storageQuota')}
      >
        <div>
          <p className="mt-1 text-xs text-ink-400">关闭存储配额后，租户资产存储不设容量上限。</p>
        </div>
        <ToggleField
          checked={draft.storageQuota.enabled}
          label="启用租户存储配额"
          onChange={(enabled) =>
            onDraftChange('storageQuota', (current) => ({
              ...current,
              storageQuota: {
                ...current.storageQuota,
                enabled,
                maxBytes:
                  enabled && !current.storageQuota.maxBytes
                    ? String(Math.max(settings?.storageQuota.usedBytes ?? 0, 1024 * 1024 * 1024))
                    : current.storageQuota.maxBytes,
              },
            }))
          }
        />
        <div className="grid gap-3 md:grid-cols-2">
          <NumberField
            disabled={!draft.storageQuota.enabled}
            error={fieldError(groupErrors.storageQuota, 'storageQuota.maxBytes')}
            hint={formatByteRangeHint(settings?.constraints.storageQuota, draft.storageQuota.maxBytes)}
            label="最大存储字节数"
            max={settings?.constraints.storageQuota.max}
            min={settings?.constraints.storageQuota.min ?? 1}
            onChange={(value) => onDraftChange('storageQuota', (current) => ({ ...current, storageQuota: { ...current.storageQuota, maxBytes: value } }))}
            value={draft.storageQuota.maxBytes}
          />
          <ReadOnlyField
            hint="该值由现有资产自动计算，不可手动修改。"
            label="已用存储容量"
            value={settings ? `${formatBytes(settings.storageQuota.usedBytes)}（${formatNumber(settings.storageQuota.usedBytes)} 字节）` : '0 B'}
          />
        </div>
      </SettingsGroupForm>

      <SettingsGroupForm
        wide
        group="logRetention"
        onSubmit={onSubmit}
        title="日志保留期"
        submitLabel="保存日志保留期"
        {...groupProps('logRetention')}
      >
        <div>
          <p className="mt-1 text-xs text-ink-400">每类日志可独立启用自动清理；关闭后不会按时间自动删除对应日志。</p>
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <div className="grid gap-2 rounded-md border border-ink-200 bg-white p-3">
            <ToggleField
              checked={draft.logRetention.operationLogEnabled}
              label="自动清理操作日志"
              onChange={(enabled) =>
                onDraftChange('logRetention', (current) => ({
                  ...current,
                  logRetention: {
                    ...current.logRetention,
                    operationLogEnabled: enabled,
                    operationLogRetentionDays:
                      enabled && !current.logRetention.operationLogRetentionDays
                        ? String(settings?.constraints.logRetention.min ?? 1)
                        : current.logRetention.operationLogRetentionDays,
                  },
                }))
              }
            />
            <NumberField
              disabled={!draft.logRetention.operationLogEnabled}
              error={fieldError(groupErrors.logRetention, 'logRetention.operationLogRetentionDays')}
              hint={formatIntegerRangeHint(settings?.constraints.logRetention, '天')}
              label="操作日志保留天数"
              max={settings?.constraints.logRetention.max}
              min={settings?.constraints.logRetention.min ?? 1}
              onChange={(value) =>
                onDraftChange('logRetention', (current) => ({ ...current, logRetention: { ...current.logRetention, operationLogRetentionDays: value } }))
              }
              value={draft.logRetention.operationLogRetentionDays}
            />
          </div>
          <div className="grid gap-2 rounded-md border border-ink-200 bg-white p-3">
            <ToggleField
              checked={draft.logRetention.apiCallLogEnabled}
              label="自动清理 API 调用日志"
              onChange={(enabled) =>
                onDraftChange('logRetention', (current) => ({
                  ...current,
                  logRetention: {
                    ...current.logRetention,
                    apiCallLogEnabled: enabled,
                    apiCallLogRetentionDays:
                      enabled && !current.logRetention.apiCallLogRetentionDays
                        ? String(settings?.constraints.logRetention.min ?? 1)
                        : current.logRetention.apiCallLogRetentionDays,
                  },
                }))
              }
            />
            <NumberField
              disabled={!draft.logRetention.apiCallLogEnabled}
              error={fieldError(groupErrors.logRetention, 'logRetention.apiCallLogRetentionDays')}
              hint={formatIntegerRangeHint(settings?.constraints.logRetention, '天')}
              label="API 调用日志保留天数"
              max={settings?.constraints.logRetention.max}
              min={settings?.constraints.logRetention.min ?? 1}
              onChange={(value) =>
                onDraftChange('logRetention', (current) => ({ ...current, logRetention: { ...current.logRetention, apiCallLogRetentionDays: value } }))
              }
              value={draft.logRetention.apiCallLogRetentionDays}
            />
          </div>
          <div className="grid gap-2 rounded-md border border-ink-200 bg-white p-3">
            <ToggleField
              checked={draft.logRetention.taskEventEnabled}
              label="自动清理任务事件"
              onChange={(enabled) =>
                onDraftChange('logRetention', (current) => ({
                  ...current,
                  logRetention: {
                    ...current.logRetention,
                    taskEventEnabled: enabled,
                    taskEventRetentionDays:
                      enabled && !current.logRetention.taskEventRetentionDays
                        ? String(settings?.constraints.logRetention.min ?? 1)
                        : current.logRetention.taskEventRetentionDays,
                  },
                }))
              }
            />
            <NumberField
              disabled={!draft.logRetention.taskEventEnabled}
              error={fieldError(groupErrors.logRetention, 'logRetention.taskEventRetentionDays')}
              hint={formatIntegerRangeHint(settings?.constraints.logRetention, '天')}
              label="任务事件保留天数"
              max={settings?.constraints.logRetention.max}
              min={settings?.constraints.logRetention.min ?? 1}
              onChange={(value) =>
                onDraftChange('logRetention', (current) => ({ ...current, logRetention: { ...current.logRetention, taskEventRetentionDays: value } }))
              }
              value={draft.logRetention.taskEventRetentionDays}
            />
          </div>
        </div>
          </SettingsGroupForm>
        </div>
      ) : null}
    </section>
  )
}

function SettingsGroupForm({
  children,
  disabled,
  error,
  group,
  isDirty,
  isSaving,
  notice,
  onSubmit,
  submitLabel,
  title,
  wide = false,
}: {
  children: ReactNode
  disabled: boolean
  error?: string
  group: SystemSettingsGroup
  isDirty: boolean
  isSaving: boolean
  notice?: string
  onSubmit: (group: SystemSettingsGroup) => void
  submitLabel: string
  title: string
  wide?: boolean
}) {
  return (
    <form
      className={`grid gap-4 rounded-lg border border-ink-200 bg-ink-50 p-4 ${wide ? 'lg:col-span-2' : ''}`}
      noValidate
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit(group)
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h4 className="text-sm font-semibold text-ink-900">{title}</h4>
        {isDirty ? <span className="rounded-full bg-amber-50 px-2 py-1 text-xs font-medium text-amber-700">有未保存修改</span> : null}
      </div>
      <StatusMessage message={error ?? null} tone="error" />
      <StatusMessage message={notice ?? null} tone="success" />
      {children}
      <Button
        className="justify-self-start"
        disabled={disabled || !isDirty}
        icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
        type="submit"
        variant="primary"
      >
        {isSaving ? '正在保存...' : submitLabel}
      </Button>
    </form>
  )
}

function NumberField({
  disabled = false,
  error,
  hint,
  label,
  max,
  min,
  value,
  onChange,
}: {
  disabled?: boolean
  error?: string
  hint?: string
  label: string
  max?: number
  min: number
  value: string
  onChange: (value: string) => void
}) {
  const id = `admin-observability-${label}`

  return (
    <div className="grid gap-1 text-sm text-ink-700">
      <label className="field-label" htmlFor={id}>
        {label}
      </label>
      <input
        aria-describedby={hint || error ? `${id}-help` : undefined}
        aria-invalid={Boolean(error)}
        className={`field-input ${error ? 'border-red-300 focus:border-red-400 focus:ring-red-100' : ''}`}
        disabled={disabled}
        id={id}
        max={max}
        min={min}
        onChange={(event) => onChange(event.target.value)}
        step={1}
        type="number"
        value={value}
      />
      {error || hint ? (
        <span className={error ? 'text-xs text-red-600' : 'text-xs text-ink-400'} id={`${id}-help`}>
          {error ?? hint}
        </span>
      ) : null}
    </div>
  )
}

function SelectField({
  disabled = false,
  error,
  hint,
  label,
  options,
  placeholder,
  value,
  onChange,
}: {
  disabled?: boolean
  error?: string
  hint?: string
  label: string
  options: Array<{ label: string; value: string }>
  placeholder: string
  value: string
  onChange: (value: string) => void
}) {
  const id = `admin-observability-${label}`

  return (
    <div className="grid gap-1 text-sm text-ink-700">
      <label className="field-label" htmlFor={id}>
        {label}
      </label>
      <select
        aria-describedby={hint || error ? `${id}-help` : undefined}
        aria-invalid={Boolean(error)}
        className={`field-input ${error ? 'border-red-300 focus:border-red-400 focus:ring-red-100' : ''}`}
        disabled={disabled}
        id={id}
        onChange={(event) => onChange(event.target.value)}
        value={value}
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {error || hint ? (
        <span className={error ? 'text-xs text-red-600' : 'text-xs text-ink-400'} id={`${id}-help`}>
          {error ?? hint}
        </span>
      ) : null}
    </div>
  )
}

function ToggleField({ checked, label, onChange }: { checked: boolean; label: string; onChange: (checked: boolean) => void }) {
  return (
    <label className="flex items-center gap-2 text-sm font-medium text-ink-700">
      <input checked={checked} className="h-4 w-4 rounded border-ink-300 text-brand-600" onChange={(event) => onChange(event.target.checked)} type="checkbox" />
      <span>{label}</span>
    </label>
  )
}

function ReadOnlyField({ hint, label, value }: { hint?: string; label: string; value: string }) {
  const id = `admin-observability-${label}`

  return (
    <div className="grid gap-1 text-sm text-ink-700">
      <label className="field-label" htmlFor={id}>
        {label}
      </label>
      <input aria-describedby={hint ? `${id}-help` : undefined} className="field-input bg-ink-100 text-ink-500" id={id} readOnly type="text" value={value} />
      {hint ? (
        <span className="text-xs text-ink-400" id={`${id}-help`}>
          {hint}
        </span>
      ) : null}
    </div>
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

function emptyApiPage<TRecord>(pageSize: number): ApiPage<TRecord> {
  return {
    records: [],
    total: 0,
    pageNum: 1,
    pageSize,
  }
}

function emptyUsageFiltersDraft(): UsageFiltersDraft {
  return {
    createdAtFrom: '',
    createdAtTo: '',
    taskId: '',
    userId: '',
    projectId: '',
    providerId: '',
    modelId: '',
  }
}

function normalizeUsageFilters(filters: UsageFiltersDraft): UsageFiltersDraft {
  return {
    createdAtFrom: filters.createdAtFrom.trim(),
    createdAtTo: filters.createdAtTo.trim(),
    taskId: filters.taskId.trim(),
    userId: filters.userId.trim(),
    projectId: filters.projectId.trim(),
    providerId: filters.providerId.trim(),
    modelId: filters.modelId.trim(),
  }
}

function usageFiltersToQuery(filters: UsageFiltersDraft): AdminUsageQuery {
  const normalized = normalizeUsageFilters(filters)
  return {
    createdAtFrom: normalized.createdAtFrom || undefined,
    createdAtTo: normalized.createdAtTo || undefined,
    taskId: normalized.taskId || undefined,
    userId: normalized.userId || undefined,
    projectId: normalized.projectId || undefined,
    providerId: normalized.providerId || undefined,
    modelId: normalized.modelId || undefined,
  }
}

function usageSummaryFilterField(dimension: UsageSummaryDimension): UsageFilterField | null {
  if (dimension === 'user') {
    return 'userId'
  }
  if (dimension === 'project') {
    return 'projectId'
  }
  if (dimension === 'provider') {
    return 'providerId'
  }
  if (dimension === 'model') {
    return 'modelId'
  }
  return null
}

function emptySystemSettingsDraft(): SystemSettingsDraft {
  return {
    uploadPolicy: {
      maxFileSizeBytes: '',
      maxWidth: '',
      maxHeight: '',
      maxPixels: '',
    },
    taskDefaults: {
      defaultProviderId: '',
      defaultModelId: '',
    },
    taskConcurrency: {
      tenantLimit: '',
      userLimit: '',
      providerLimit: '',
      modelLimit: '',
    },
    storageRetention: {
      enabled: false,
      deletedAssetRetentionDays: '',
    },
    storageQuota: {
      enabled: false,
      maxBytes: '',
    },
    logRetention: {
      operationLogEnabled: false,
      operationLogRetentionDays: '',
      apiCallLogEnabled: false,
      apiCallLogRetentionDays: '',
      taskEventEnabled: false,
      taskEventRetentionDays: '',
    },
  }
}

function draftFromSettings(settings: SystemSettings): SystemSettingsDraft {
  return {
    uploadPolicy: {
      maxFileSizeBytes: String(settings.uploadPolicy.maxFileSizeBytes),
      maxWidth: String(settings.uploadPolicy.maxWidth),
      maxHeight: String(settings.uploadPolicy.maxHeight),
      maxPixels: String(settings.uploadPolicy.maxPixels),
    },
    taskDefaults: {
      defaultProviderId: settings.taskDefaults.defaultProviderId ?? '',
      defaultModelId: settings.taskDefaults.defaultModelId ?? '',
    },
    taskConcurrency: {
      tenantLimit: String(settings.taskConcurrency.tenantLimit),
      userLimit: String(settings.taskConcurrency.userLimit),
      providerLimit: String(settings.taskConcurrency.providerLimit),
      modelLimit: String(settings.taskConcurrency.modelLimit),
    },
    storageRetention: {
      enabled: settings.storageRetention.deletedAssetRetentionDays !== null,
      deletedAssetRetentionDays: settings.storageRetention.deletedAssetRetentionDays === null ? '' : String(settings.storageRetention.deletedAssetRetentionDays),
    },
    storageQuota: {
      enabled: settings.storageQuota.maxBytes !== null,
      maxBytes: settings.storageQuota.maxBytes === null ? '' : String(settings.storageQuota.maxBytes),
    },
    logRetention: {
      operationLogEnabled: settings.logRetention.operationLogRetentionDays !== null,
      operationLogRetentionDays: settings.logRetention.operationLogRetentionDays === null ? '' : String(settings.logRetention.operationLogRetentionDays),
      apiCallLogEnabled: settings.logRetention.apiCallLogRetentionDays !== null,
      apiCallLogRetentionDays: settings.logRetention.apiCallLogRetentionDays === null ? '' : String(settings.logRetention.apiCallLogRetentionDays),
      taskEventEnabled: settings.logRetention.taskEventRetentionDays !== null,
      taskEventRetentionDays: settings.logRetention.taskEventRetentionDays === null ? '' : String(settings.logRetention.taskEventRetentionDays),
    },
  }
}

function parseSettingsGroupPatch(group: SystemSettingsGroup, draft: SystemSettingsDraft): UpdateSystemSettingsRequest {
  if (group === 'uploadPolicy') {
    return {
      uploadPolicy: parseUploadPolicyDraft(draft.uploadPolicy),
    }
  }
  if (group === 'taskDefaults') {
    return {
      taskDefaults: parseTaskDefaultsDraft(draft.taskDefaults),
    }
  }
  if (group === 'taskConcurrency') {
    return {
      taskConcurrency: parseTaskConcurrencyDraft(draft.taskConcurrency),
    }
  }
  if (group === 'storageRetention') {
    return {
      storageRetention: {
        deletedAssetRetentionDays: draft.storageRetention.enabled
          ? parsePositiveInteger(draft.storageRetention.deletedAssetRetentionDays, '删除资产保留天数')
          : null,
      },
    }
  }
  if (group === 'logRetention') {
    return {
      logRetention: {
        operationLogRetentionDays: draft.logRetention.operationLogEnabled
          ? parsePositiveInteger(draft.logRetention.operationLogRetentionDays, '操作日志保留天数')
          : null,
        apiCallLogRetentionDays: draft.logRetention.apiCallLogEnabled
          ? parsePositiveInteger(draft.logRetention.apiCallLogRetentionDays, 'API 调用日志保留天数')
          : null,
        taskEventRetentionDays: draft.logRetention.taskEventEnabled
          ? parsePositiveInteger(draft.logRetention.taskEventRetentionDays, '任务事件保留天数')
          : null,
      },
    }
  }
  return {
    storageQuota: {
      maxBytes: draft.storageQuota.enabled ? parsePositiveInteger(draft.storageQuota.maxBytes, '最大存储字节数') : null,
    },
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

function parseTaskDefaultsDraft(draft: TaskDefaultsDraft): SystemSettings['taskDefaults'] {
  const defaultProviderId = draft.defaultProviderId.trim()
  const defaultModelId = draft.defaultModelId.trim()
  if (!defaultProviderId && !defaultModelId) {
    return {
      defaultProviderId: null,
      defaultModelId: null,
    }
  }
  if (!defaultProviderId || !defaultModelId) {
    throw new Error('默认 Provider 与默认模型必须成对选择或同时清空。')
  }
  return {
    defaultProviderId: defaultProviderId as SystemSettings['taskDefaults']['defaultProviderId'],
    defaultModelId: defaultModelId as SystemSettings['taskDefaults']['defaultModelId'],
  }
}

function parseTaskConcurrencyDraft(draft: TaskConcurrencyDraft): SystemSettings['taskConcurrency'] {
  return {
    tenantLimit: parsePositiveInteger(draft.tenantLimit, '租户并发上限'),
    userLimit: parsePositiveInteger(draft.userLimit, '用户并发上限'),
    providerLimit: parsePositiveInteger(draft.providerLimit, 'Provider 并发上限'),
    modelLimit: parsePositiveInteger(draft.modelLimit, '模型并发上限'),
  }
}

function parsePositiveInteger(value: string, label: string): number {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${label}必须是正整数。`)
  }
  return parsed
}

function mergeSavedSettingsGroup(current: SystemSettings | null, saved: SystemSettings, group: SystemSettingsGroup): SystemSettings {
  if (!current) {
    return saved
  }
  return {
    ...current,
    [group]: saved[group],
    constraints: saved.constraints,
  } as SystemSettings
}

function mergeSavedDraftGroup(current: SystemSettingsDraft, saved: SystemSettingsDraft, group: SystemSettingsGroup): SystemSettingsDraft {
  return {
    ...current,
    [group]: saved[group],
  } as SystemSettingsDraft
}

function settingsDraftGroupEquals(current: SystemSettingsDraft, saved: SystemSettingsDraft, group: SystemSettingsGroup): boolean {
  return JSON.stringify(current[group]) === JSON.stringify(saved[group])
}

function formatSettingsGroupError(error: unknown): SettingsGroupFeedback {
  if (isApiClientError(error)) {
    if (error.status === 422) {
      const message = error.message === 'Invalid request.' ? '设置内容不符合要求，请检查填写范围。' : error.message
      return {
        field: validationFieldFromDetails(error.details),
        message: error.requestId ? `${message}（请求标识：${error.requestId}）` : message,
      }
    }
    return { message: formatAdminError(error) }
  }
  if (error instanceof Error) {
    return { message: error.message }
  }
  return { message: '保存失败，请稍后重试。' }
}

function validationFieldFromDetails(details: unknown): string | undefined {
  if (!details || typeof details !== 'object') {
    return undefined
  }
  const field = Reflect.get(details, 'field')
  return typeof field === 'string' && field ? field : undefined
}

function fieldError(feedback: SettingsGroupFeedback | undefined, field: string): string | undefined {
  return feedback?.field === field ? feedback.message : undefined
}

function formatIntegerRangeHint(range: IntegerRangeConstraint | undefined, unit: string): string | undefined {
  if (!range) {
    return undefined
  }
  return `允许范围：${formatNumber(range.min)}–${formatNumber(range.max)} ${unit}`
}

function formatByteRangeHint(range: IntegerRangeConstraint | undefined, currentValue: string): string | undefined {
  if (!range) {
    return undefined
  }
  const value = Number(currentValue)
  const currentHint = Number.isFinite(value) && value > 0 ? `；当前约 ${formatBytes(value)}` : ''
  return `允许范围：${formatNumber(range.min)}–${formatNumber(range.max)} 字节（最大 ${formatBytes(range.max)}）${currentHint}`
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${formatNumber(bytes)} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 'B'
  for (const candidate of units) {
    value /= 1024
    unit = candidate
    if (value < 1024 || candidate === 'TB') {
      break
    }
  }
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value)} ${unit}`
}

function settingsGroupLabel(group: SystemSettingsGroup): string {
  if (group === 'uploadPolicy') {
    return '上传策略'
  }
  if (group === 'taskDefaults') {
    return '任务默认模型'
  }
  if (group === 'taskConcurrency') {
    return '并发限制'
  }
  if (group === 'storageRetention') {
    return '删除资产保留期'
  }
  if (group === 'logRetention') {
    return '日志保留期'
  }
  return '存储配额'
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
