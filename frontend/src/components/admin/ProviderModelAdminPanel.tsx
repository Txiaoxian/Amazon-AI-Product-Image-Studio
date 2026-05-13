import { AlertTriangle, CheckCircle2, Loader2, Pencil, Plus, Power, PowerOff, RefreshCw, Trash2 } from 'lucide-react'
import { cloneElement, useCallback, useEffect, useMemo, useState, type ReactElement } from 'react'
import { isApiClientError } from '../../api/client'
import { modelApi as defaultModelApi, type CreateModelRequest, type ModelApi, type UpdateModelRequest } from '../../api/models'
import { providerApi as defaultProviderApi, type ProviderApi } from '../../api/providers'
import type { Model, ModelStatus, Provider, ProviderStatus, ProviderType } from '../../types/platform'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

type AdminTab = 'providers' | 'models'

interface ProviderModelAdminPanelProps {
  isOpen: boolean
  csrfToken?: string
  canManageProviders: boolean
  canManageModels: boolean
  onClose: () => void
  providerApi?: ProviderApi
  modelApi?: ModelApi
}

interface ProviderDraft {
  editingId: string | null
  type: ProviderType
  name: string
  baseUrl: string
  apiKey: string
  timeoutSeconds: string
  concurrencyLimit: string
  status: ProviderStatus
}

interface ProviderFormRequest {
  name: string
  baseUrl: string
  status: ProviderStatus
  timeoutSeconds: number
  concurrencyLimit: number
  apiKey?: string
}

interface ModelDraft {
  editingId: string | null
  providerId: string
  modelName: string
  displayName: string
  supportsGenerate: boolean
  supportsEdit: boolean
  supportsMultiReference: boolean
  supportsN: boolean
  maxOutputCount: string
  supportedSizes: string
  supportedQualities: string
  supportedOutputFormats: string
  pricingCurrency: string
  pricingUnitPrices: string
  status: ModelStatus
}

const PAGE_SIZE = 50

const providerTypeOptions: Array<{ value: ProviderType; label: string }> = [
  { value: 'OPENAI_COMPATIBLE', label: 'OpenAI Compatible' },
  { value: 'OPENAI', label: 'OpenAI' },
  { value: 'GEMINI', label: 'Gemini' },
]

const statusOptions: Array<{ value: ProviderStatus; label: string }> = [
  { value: 'ENABLED', label: '启用' },
  { value: 'DISABLED', label: '禁用' },
]

export function ProviderModelAdminPanel({
  isOpen,
  csrfToken,
  canManageProviders,
  canManageModels,
  onClose,
  providerApi = defaultProviderApi,
  modelApi = defaultModelApi,
}: ProviderModelAdminPanelProps) {
  const [activeTab, setActiveTab] = useState<AdminTab>(canManageProviders ? 'providers' : 'models')
  const [providers, setProviders] = useState<Provider[]>([])
  const [models, setModels] = useState<Model[]>([])
  const [isLoadingProviders, setLoadingProviders] = useState(false)
  const [isLoadingModels, setLoadingModels] = useState(false)
  const [providerError, setProviderError] = useState<string | null>(null)
  const [modelError, setModelError] = useState<string | null>(null)
  const [providerFormError, setProviderFormError] = useState<string | null>(null)
  const [modelFormError, setModelFormError] = useState<string | null>(null)
  const [providerNotice, setProviderNotice] = useState<string | null>(null)
  const [modelNotice, setModelNotice] = useState<string | null>(null)
  const [providerActionId, setProviderActionId] = useState<string | null>(null)
  const [modelActionId, setModelActionId] = useState<string | null>(null)
  const [isSavingProvider, setSavingProvider] = useState(false)
  const [isSavingModel, setSavingModel] = useState(false)
  const [providerDraft, setProviderDraft] = useState<ProviderDraft>(() => emptyProviderDraft())
  const [modelDraft, setModelDraft] = useState<ModelDraft>(() => emptyModelDraft())

  useEffect(() => {
    if (!canManageProviders && !canManageModels) {
      return
    }
    if (!canManageProviders && activeTab === 'providers') {
      setActiveTab('models')
    }
    if (!canManageModels && activeTab === 'models') {
      setActiveTab('providers')
    }
  }, [activeTab, canManageModels, canManageProviders])

  const refreshProviders = useCallback(async () => {
    if (!canManageProviders && !canManageModels) {
      return
    }

    setLoadingProviders(true)
    setProviderError(null)
    try {
      const page = await providerApi.list({ pageNum: 1, pageSize: PAGE_SIZE })
      setProviders(page.records)
    } catch (error) {
      setProviderError(formatAdminError(error))
    } finally {
      setLoadingProviders(false)
    }
  }, [canManageModels, canManageProviders, providerApi])

  const refreshModels = useCallback(async () => {
    if (!canManageModels) {
      return
    }

    setLoadingModels(true)
    setModelError(null)
    try {
      const page = await modelApi.list({ pageNum: 1, pageSize: PAGE_SIZE })
      setModels(page.records)
    } catch (error) {
      setModelError(formatAdminError(error))
    } finally {
      setLoadingModels(false)
    }
  }, [canManageModels, modelApi])

  useEffect(() => {
    if (!isOpen) {
      return
    }

    setProviderNotice(null)
    setModelNotice(null)
    setProviderFormError(null)
    setModelFormError(null)
    void refreshProviders()
    void refreshModels()
  }, [isOpen, refreshModels, refreshProviders])

  useEffect(() => {
    if (modelDraft.providerId || providers.length === 0) {
      return
    }

    setModelDraft((current) => ({
      ...current,
      providerId: providers[0].id,
    }))
  }, [modelDraft.providerId, providers])

  const editingProvider = useMemo(
    () => providers.find((provider) => provider.id === providerDraft.editingId) ?? null,
    [providerDraft.editingId, providers],
  )

  const submitProvider = async () => {
    if (!csrfToken) {
      setProviderFormError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    setSavingProvider(true)
    setProviderFormError(null)
    setProviderNotice(null)

    try {
      const baseRequest = buildProviderRequest(providerDraft)
      const saved = providerDraft.editingId
        ? await providerApi.update(providerDraft.editingId, baseRequest, csrfToken)
        : await providerApi.create({ ...baseRequest, type: providerDraft.type, apiKey: requireProviderKey(providerDraft.apiKey) }, csrfToken)

      setProviders((current) => upsertById(current, saved))
      setProviderDraft(emptyProviderDraft())
      setProviderNotice(providerDraft.editingId ? 'Provider 已更新。' : 'Provider 已创建。')
    } catch (error) {
      setProviderFormError(formatAdminError(error))
    } finally {
      setSavingProvider(false)
    }
  }

  const submitModel = async () => {
    if (!csrfToken) {
      setModelFormError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }

    setSavingModel(true)
    setModelFormError(null)
    setModelNotice(null)

    try {
      const request = buildModelRequest(modelDraft)
      const saved = modelDraft.editingId
        ? await modelApi.update(modelDraft.editingId, request, csrfToken)
        : await modelApi.create(request as CreateModelRequest, csrfToken)

      setModels((current) => upsertById(current, saved))
      setModelDraft(emptyModelDraft(providers[0]?.id ?? ''))
      setModelNotice(modelDraft.editingId ? '模型已更新。' : '模型已创建。')
    } catch (error) {
      setModelFormError(formatAdminError(error))
    } finally {
      setSavingModel(false)
    }
  }

  const runProviderAction = async (provider: Provider, action: 'enable' | 'disable' | 'delete' | 'test') => {
    if (!csrfToken) {
      setProviderError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }
    if (action === 'delete' && !window.confirm(`确定删除 Provider ${provider.name} 吗？`)) {
      return
    }

    setProviderActionId(provider.id)
    setProviderError(null)
    setProviderNotice(null)
    try {
      if (action === 'enable') {
        const saved = await providerApi.enable(provider.id, csrfToken)
        setProviders((current) => upsertById(current, saved))
        setProviderNotice('Provider 已启用。')
      } else if (action === 'disable') {
        const saved = await providerApi.disable(provider.id, csrfToken)
        setProviders((current) => upsertById(current, saved))
        setProviderNotice('Provider 已禁用。')
      } else if (action === 'delete') {
        await providerApi.delete(provider.id, csrfToken)
        setProviders((current) => current.filter((item) => item.id !== provider.id))
        setProviderNotice('Provider 已删除。')
      } else {
        const result = await providerApi.test(provider.id, csrfToken)
        setProviderNotice(`${result.message} ${result.durationMs}ms`)
        await refreshProviders()
      }
    } catch (error) {
      setProviderError(formatAdminError(error))
    } finally {
      setProviderActionId(null)
    }
  }

  const runModelAction = async (model: Model, action: 'enable' | 'disable' | 'delete') => {
    if (!csrfToken) {
      setModelError('登录状态缺少 CSRF 凭据，请重新登录。')
      return
    }
    if (action === 'delete' && !window.confirm(`确定删除模型 ${model.displayName} 吗？`)) {
      return
    }

    setModelActionId(model.id)
    setModelError(null)
    setModelNotice(null)
    try {
      if (action === 'enable') {
        const saved = await modelApi.enable(model.id, csrfToken)
        setModels((current) => upsertById(current, saved))
        setModelNotice('模型已启用。')
      } else if (action === 'disable') {
        const saved = await modelApi.disable(model.id, csrfToken)
        setModels((current) => upsertById(current, saved))
        setModelNotice('模型已禁用。')
      } else {
        await modelApi.delete(model.id, csrfToken)
        setModels((current) => current.filter((item) => item.id !== model.id))
        setModelNotice('模型已删除。')
      }
    } catch (error) {
      setModelError(formatAdminError(error))
    } finally {
      setModelActionId(null)
    }
  }

  return (
    <Modal isOpen={isOpen} maxWidthClass="max-w-6xl" onClose={onClose} title="Provider 与模型管理">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="inline-flex rounded-md border border-ink-200 bg-ink-50 p-1">
            {canManageProviders ? (
              <button
                className={tabClassName(activeTab === 'providers')}
                onClick={() => setActiveTab('providers')}
                type="button"
              >
                Provider
              </button>
            ) : null}
            {canManageModels ? (
              <button className={tabClassName(activeTab === 'models')} onClick={() => setActiveTab('models')} type="button">
                模型
              </button>
            ) : null}
          </div>

          <Button
            icon={<RefreshCw className={`h-4 w-4 ${isLoadingProviders || isLoadingModels ? 'animate-spin' : ''}`} />}
            onClick={() => {
              void refreshProviders()
              void refreshModels()
            }}
            variant="secondary"
          >
            刷新
          </Button>
        </div>

        {activeTab === 'providers' ? (
          <ProviderManagementView
            actionId={providerActionId}
            draft={providerDraft}
            editingProvider={editingProvider}
            error={providerError}
            formError={providerFormError}
            isLoading={isLoadingProviders}
            isSaving={isSavingProvider}
            notice={providerNotice}
            onAction={(provider, action) => void runProviderAction(provider, action)}
            onDraftChange={setProviderDraft}
            onRefresh={() => void refreshProviders()}
            onReset={() => {
              setProviderDraft(emptyProviderDraft())
              setProviderFormError(null)
            }}
            onSubmit={() => void submitProvider()}
            providers={providers}
          />
        ) : null}

        {activeTab === 'models' ? (
          <ModelManagementView
            actionId={modelActionId}
            draft={modelDraft}
            error={modelError}
            formError={modelFormError}
            isLoading={isLoadingModels}
            isSaving={isSavingModel}
            models={models}
            notice={modelNotice}
            onAction={(model, action) => void runModelAction(model, action)}
            onDraftChange={setModelDraft}
            onReset={() => {
              setModelDraft(emptyModelDraft(providers[0]?.id ?? ''))
              setModelFormError(null)
            }}
            onSubmit={() => void submitModel()}
            providers={providers}
          />
        ) : null}
      </div>
    </Modal>
  )
}

interface ProviderManagementViewProps {
  providers: Provider[]
  draft: ProviderDraft
  editingProvider: Provider | null
  isLoading: boolean
  isSaving: boolean
  actionId: string | null
  error: string | null
  formError: string | null
  notice: string | null
  onAction: (provider: Provider, action: 'enable' | 'disable' | 'delete' | 'test') => void
  onDraftChange: (draft: ProviderDraft | ((current: ProviderDraft) => ProviderDraft)) => void
  onRefresh: () => void
  onReset: () => void
  onSubmit: () => void
}

function ProviderManagementView({
  providers,
  draft,
  editingProvider,
  isLoading,
  isSaving,
  actionId,
  error,
  formError,
  notice,
  onAction,
  onDraftChange,
  onRefresh,
  onReset,
  onSubmit,
}: ProviderManagementViewProps) {
  return (
    <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_380px]">
      <div className="min-w-0 space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div>
            <h3 className="text-sm font-semibold text-ink-900">Provider 列表</h3>
            <p className="text-xs text-ink-400">{providers.length} 个 Provider</p>
          </div>
          <button aria-label="刷新 Provider" className="icon-button" disabled={isLoading} onClick={onRefresh} title="刷新 Provider" type="button">
            <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
          </button>
        </div>

        <StatusMessage message={error} tone="error" />
        <StatusMessage message={notice} tone="success" />

        {isLoading ? <p className="rounded-md bg-ink-50 px-4 py-8 text-center text-sm text-ink-500">正在加载 Provider...</p> : null}
        {!isLoading && providers.length === 0 ? (
          <EmptyState title="暂无 Provider" body="创建 Provider 后可在这里启用、禁用、测试或维护密钥元数据。" />
        ) : null}

        <div className="space-y-2">
          {providers.map((provider) => (
            <ProviderListItem
              actionId={actionId}
              key={provider.id}
              onAction={onAction}
              onEdit={(selected) =>
                onDraftChange({
                  editingId: selected.id,
                  type: selected.type,
                  name: selected.name,
                  baseUrl: selected.baseUrl,
                  apiKey: '',
                  timeoutSeconds: String(selected.timeoutSeconds),
                  concurrencyLimit: String(selected.concurrencyLimit),
                  status: selected.status,
                })
              }
              provider={provider}
            />
          ))}
        </div>
      </div>

      <ProviderForm
        draft={draft}
        editingProvider={editingProvider}
        error={formError}
        isSaving={isSaving}
        onDraftChange={onDraftChange}
        onReset={onReset}
        onSubmit={onSubmit}
      />
    </section>
  )
}

interface ProviderListItemProps {
  provider: Provider
  actionId: string | null
  onAction: (provider: Provider, action: 'enable' | 'disable' | 'delete' | 'test') => void
  onEdit: (provider: Provider) => void
}

function ProviderListItem({ provider, actionId, onAction, onEdit }: ProviderListItemProps) {
  const isPending = actionId === provider.id

  return (
    <article className="rounded-lg border border-ink-200 bg-white p-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="truncate text-sm font-semibold text-ink-900">{provider.name}</h4>
            <StatusBadge status={provider.status} />
            <span className="rounded-md bg-ink-100 px-2 py-1 text-xs font-semibold text-ink-600">{provider.type}</span>
          </div>
          <p className="mt-1 truncate text-xs text-ink-500">{provider.baseUrl}</p>
          <dl className="mt-2 grid grid-cols-1 gap-1 text-xs text-ink-500 sm:grid-cols-2">
            <div>
              <dt className="inline text-ink-400">密钥</dt>
              <dd className="ml-1 inline font-medium text-ink-700">{provider.apiKeyHint || '未返回'}</dd>
            </div>
            <div>
              <dt className="inline text-ink-400">测试</dt>
              <dd className="ml-1 inline font-medium text-ink-700">{formatProviderTest(provider)}</dd>
            </div>
          </dl>
        </div>

        <div className="flex flex-wrap gap-1">
          <button aria-label={`编辑 Provider ${provider.name}`} className="icon-button h-8 w-8" onClick={() => onEdit(provider)} title="编辑" type="button">
            <Pencil className="h-4 w-4" />
          </button>
          <button
            aria-label={`测试 Provider ${provider.name}`}
            className="icon-button h-8 w-8"
            disabled={isPending}
            onClick={() => onAction(provider, 'test')}
            title="测试"
            type="button"
          >
            {isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
          </button>
          {provider.status === 'ENABLED' ? (
            <button
              aria-label={`禁用 Provider ${provider.name}`}
              className="icon-button h-8 w-8"
              disabled={isPending}
              onClick={() => onAction(provider, 'disable')}
              title="禁用"
              type="button"
            >
              <PowerOff className="h-4 w-4" />
            </button>
          ) : (
            <button
              aria-label={`启用 Provider ${provider.name}`}
              className="icon-button h-8 w-8"
              disabled={isPending}
              onClick={() => onAction(provider, 'enable')}
              title="启用"
              type="button"
            >
              <Power className="h-4 w-4" />
            </button>
          )}
          <button
            aria-label={`删除 Provider ${provider.name}`}
            className="icon-button h-8 w-8 hover:border-red-200 hover:bg-red-50 hover:text-red-700"
            disabled={isPending}
            onClick={() => onAction(provider, 'delete')}
            title="删除"
            type="button"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>
    </article>
  )
}

interface ProviderFormProps {
  draft: ProviderDraft
  editingProvider: Provider | null
  error: string | null
  isSaving: boolean
  onDraftChange: (draft: ProviderDraft | ((current: ProviderDraft) => ProviderDraft)) => void
  onReset: () => void
  onSubmit: () => void
}

function ProviderForm({ draft, editingProvider, error, isSaving, onDraftChange, onReset, onSubmit }: ProviderFormProps) {
  return (
    <form
      className="space-y-3 rounded-lg border border-ink-200 bg-ink-50 p-4"
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink-900">{draft.editingId ? '编辑 Provider' : '新建 Provider'}</h3>
        {draft.editingId ? (
          <button className="text-xs font-semibold text-ink-500 hover:text-ink-900" onClick={onReset} type="button">
            新建 Provider
          </button>
        ) : null}
      </div>

      <div className="grid gap-3">
        <Field label="Provider 名称">
          <input
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, name: event.target.value }))}
            required
            value={draft.name}
          />
        </Field>
        <Field label="Provider 类型">
          <select
            className="field-input"
            disabled={Boolean(draft.editingId)}
            onChange={(event) => onDraftChange((current) => ({ ...current, type: event.target.value as ProviderType }))}
            value={draft.type}
          >
            {providerTypeOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Base URL">
          <input
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, baseUrl: event.target.value }))}
            placeholder="https://provider.example/v1"
            value={draft.baseUrl}
          />
        </Field>
        <Field label="API Key">
          <input
            autoComplete="new-password"
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, apiKey: event.target.value }))}
            placeholder={draft.editingId ? '留空则保留现有密钥' : '仅本次提交给后端'}
            required={!draft.editingId}
            type="password"
            value={draft.apiKey}
          />
        </Field>
        {editingProvider ? (
          <div className="rounded-md border border-ink-200 bg-white px-3 py-2 text-xs leading-6 text-ink-600">
            <p>
              当前密钥：<span className="font-semibold text-ink-800">{editingProvider.apiKeyHint || '未返回'}</span>
            </p>
            <p>更新时间：{editingProvider.apiKeyUpdatedAt ? formatDateTime(editingProvider.apiKeyUpdatedAt) : '未返回'}</p>
          </div>
        ) : null}
        <div className="grid grid-cols-2 gap-2">
          <Field label="超时秒数">
            <input
              className="field-input"
              min={1}
              onChange={(event) => onDraftChange((current) => ({ ...current, timeoutSeconds: event.target.value }))}
              type="number"
              value={draft.timeoutSeconds}
            />
          </Field>
          <Field label="并发限制">
            <input
              className="field-input"
              min={0}
              onChange={(event) => onDraftChange((current) => ({ ...current, concurrencyLimit: event.target.value }))}
              type="number"
              value={draft.concurrencyLimit}
            />
          </Field>
        </div>
        <Field label="Provider 状态">
          <select
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, status: event.target.value as ProviderStatus }))}
            value={draft.status}
          >
            {statusOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </Field>
      </div>

      <StatusMessage message={error} tone="error" />
      <Button className="w-full" disabled={isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />} type="submit" variant="primary">
        保存 Provider
      </Button>
    </form>
  )
}

interface ModelManagementViewProps {
  models: Model[]
  providers: Provider[]
  draft: ModelDraft
  isLoading: boolean
  isSaving: boolean
  actionId: string | null
  error: string | null
  formError: string | null
  notice: string | null
  onAction: (model: Model, action: 'enable' | 'disable' | 'delete') => void
  onDraftChange: (draft: ModelDraft | ((current: ModelDraft) => ModelDraft)) => void
  onReset: () => void
  onSubmit: () => void
}

function ModelManagementView({
  models,
  providers,
  draft,
  isLoading,
  isSaving,
  actionId,
  error,
  formError,
  notice,
  onAction,
  onDraftChange,
  onReset,
  onSubmit,
}: ModelManagementViewProps) {
  return (
    <section className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_420px]">
      <div className="min-w-0 space-y-3">
        <div>
          <h3 className="text-sm font-semibold text-ink-900">模型列表</h3>
          <p className="text-xs text-ink-400">{models.length} 个模型</p>
        </div>

        <StatusMessage message={error} tone="error" />
        <StatusMessage message={notice} tone="success" />

        {isLoading ? <p className="rounded-md bg-ink-50 px-4 py-8 text-center text-sm text-ink-500">正在加载模型...</p> : null}
        {!isLoading && models.length === 0 ? <EmptyState title="暂无模型" body="创建模型后可维护能力、价格和启用状态。" /> : null}

        <div className="space-y-2">
          {models.map((model) => (
            <ModelListItem
              actionId={actionId}
              key={model.id}
              model={model}
              onAction={onAction}
              onEdit={(selected) =>
                onDraftChange({
                  editingId: selected.id,
                  providerId: selected.providerId,
                  modelName: selected.modelName,
                  displayName: selected.displayName,
                  supportsGenerate: selected.supportsGenerate,
                  supportsEdit: selected.supportsEdit,
                  supportsMultiReference: selected.supportsMultiReference,
                  supportsN: selected.supportsN,
                  maxOutputCount: String(selected.maxOutputCount),
                  supportedSizes: selected.supportedSizes.join(', '),
                  supportedQualities: selected.supportedQualities.join(', '),
                  supportedOutputFormats: selected.supportedOutputFormats.join(', '),
                  pricingCurrency: selected.pricing.currency,
                  pricingUnitPrices: formatUnitPrices(selected.pricing.unitPrices),
                  status: selected.status,
                })
              }
            />
          ))}
        </div>
      </div>

      <ModelForm
        draft={draft}
        error={formError}
        isSaving={isSaving}
        onDraftChange={onDraftChange}
        onReset={onReset}
        onSubmit={onSubmit}
        providers={providers}
      />
    </section>
  )
}

interface ModelListItemProps {
  model: Model
  actionId: string | null
  onAction: (model: Model, action: 'enable' | 'disable' | 'delete') => void
  onEdit: (model: Model) => void
}

function ModelListItem({ model, actionId, onAction, onEdit }: ModelListItemProps) {
  const isPending = actionId === model.id

  return (
    <article className="rounded-lg border border-ink-200 bg-white p-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h4 className="truncate text-sm font-semibold text-ink-900">{model.displayName}</h4>
            <StatusBadge status={model.status} />
          </div>
          <p className="mt-1 truncate text-xs text-ink-500">
            {model.providerName} · {model.modelName}
          </p>
          <p className="mt-2 text-xs text-ink-500">
            {model.supportsGenerate ? '生成' : null}
            {model.supportsGenerate && model.supportsEdit ? ' / ' : null}
            {model.supportsEdit ? '编辑' : null}
            {model.supportsMultiReference ? ' · 多参考图' : null}
            {model.supportsN ? ` · 最多 ${model.maxOutputCount} 张` : ' · 单张输出'}
          </p>
          <p className="mt-1 truncate text-xs text-ink-400">
            {model.supportedSizes.join(', ') || '未配置尺寸'} · {model.supportedOutputFormats.join(', ') || '未配置格式'}
          </p>
        </div>

        <div className="flex flex-wrap gap-1">
          <button aria-label={`编辑模型 ${model.displayName}`} className="icon-button h-8 w-8" onClick={() => onEdit(model)} title="编辑" type="button">
            <Pencil className="h-4 w-4" />
          </button>
          {model.status === 'ENABLED' ? (
            <button
              aria-label={`禁用模型 ${model.displayName}`}
              className="icon-button h-8 w-8"
              disabled={isPending}
              onClick={() => onAction(model, 'disable')}
              title="禁用"
              type="button"
            >
              <PowerOff className="h-4 w-4" />
            </button>
          ) : (
            <button
              aria-label={`启用模型 ${model.displayName}`}
              className="icon-button h-8 w-8"
              disabled={isPending}
              onClick={() => onAction(model, 'enable')}
              title="启用"
              type="button"
            >
              <Power className="h-4 w-4" />
            </button>
          )}
          <button
            aria-label={`删除模型 ${model.displayName}`}
            className="icon-button h-8 w-8 hover:border-red-200 hover:bg-red-50 hover:text-red-700"
            disabled={isPending}
            onClick={() => onAction(model, 'delete')}
            title="删除"
            type="button"
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>
    </article>
  )
}

interface ModelFormProps {
  providers: Provider[]
  draft: ModelDraft
  error: string | null
  isSaving: boolean
  onDraftChange: (draft: ModelDraft | ((current: ModelDraft) => ModelDraft)) => void
  onReset: () => void
  onSubmit: () => void
}

function ModelForm({ providers, draft, error, isSaving, onDraftChange, onReset, onSubmit }: ModelFormProps) {
  return (
    <form
      className="space-y-3 rounded-lg border border-ink-200 bg-ink-50 p-4"
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-sm font-semibold text-ink-900">{draft.editingId ? '编辑模型' : '新建模型'}</h3>
        {draft.editingId ? (
          <button className="text-xs font-semibold text-ink-500 hover:text-ink-900" onClick={onReset} type="button">
            新建模型
          </button>
        ) : null}
      </div>

      <div className="grid gap-3">
        {providers.length > 0 ? (
          <Field label="模型 Provider">
            <select
              className="field-input"
              onChange={(event) => onDraftChange((current) => ({ ...current, providerId: event.target.value }))}
              required
              value={draft.providerId}
            >
              {providers.map((provider) => (
                <option key={provider.id} value={provider.id}>
                  {provider.name}
                </option>
              ))}
            </select>
          </Field>
        ) : (
          <Field label="Provider ID">
            <input
              className="field-input"
              onChange={(event) => onDraftChange((current) => ({ ...current, providerId: event.target.value }))}
              required
              value={draft.providerId}
            />
          </Field>
        )}
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <Field label="模型 ID">
            <input
              className="field-input"
              onChange={(event) => onDraftChange((current) => ({ ...current, modelName: event.target.value }))}
              required
              value={draft.modelName}
            />
          </Field>
          <Field label="显示名称">
            <input
              className="field-input"
              onChange={(event) => onDraftChange((current) => ({ ...current, displayName: event.target.value }))}
              required
              value={draft.displayName}
            />
          </Field>
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <CheckboxField checked={draft.supportsGenerate} label="支持生成" onChange={(value) => onDraftChange((current) => ({ ...current, supportsGenerate: value }))} />
          <CheckboxField checked={draft.supportsEdit} label="支持编辑" onChange={(value) => onDraftChange((current) => ({ ...current, supportsEdit: value }))} />
          <CheckboxField
            checked={draft.supportsMultiReference}
            label="支持多参考图"
            onChange={(value) => onDraftChange((current) => ({ ...current, supportsMultiReference: value }))}
          />
          <CheckboxField checked={draft.supportsN} label="支持多张输出" onChange={(value) => onDraftChange((current) => ({ ...current, supportsN: value }))} />
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <Field label="最大输出数">
            <input
              className="field-input"
              min={1}
              onChange={(event) => onDraftChange((current) => ({ ...current, maxOutputCount: event.target.value }))}
              type="number"
              value={draft.maxOutputCount}
            />
          </Field>
          <Field label="模型状态">
            <select
              className="field-input"
              onChange={(event) => onDraftChange((current) => ({ ...current, status: event.target.value as ModelStatus }))}
              value={draft.status}
            >
              {statusOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </Field>
        </div>

        <Field label="支持尺寸">
          <textarea
            className="field-input min-h-20 resize-y"
            onChange={(event) => onDraftChange((current) => ({ ...current, supportedSizes: event.target.value }))}
            placeholder="1024x1024, 1536x1024"
            value={draft.supportedSizes}
          />
        </Field>
        <Field label="支持质量">
          <input
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, supportedQualities: event.target.value }))}
            placeholder="standard, hd"
            value={draft.supportedQualities}
          />
        </Field>
        <Field label="输出格式">
          <input
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, supportedOutputFormats: event.target.value }))}
            placeholder="png, jpeg, webp"
            value={draft.supportedOutputFormats}
          />
        </Field>
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-[120px_minmax(0,1fr)]">
          <Field label="计费币种">
            <input
              className="field-input"
              onChange={(event) => onDraftChange((current) => ({ ...current, pricingCurrency: event.target.value }))}
              value={draft.pricingCurrency}
            />
          </Field>
          <Field label="单位价格">
            <textarea
              className="field-input min-h-20 resize-y"
              onChange={(event) => onDraftChange((current) => ({ ...current, pricingUnitPrices: event.target.value }))}
              placeholder="image=0.04"
              value={draft.pricingUnitPrices}
            />
          </Field>
        </div>
      </div>

      <StatusMessage message={error} tone="error" />
      <Button className="w-full" disabled={isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />} type="submit" variant="primary">
        保存模型
      </Button>
    </form>
  )
}

interface FieldProps {
  label: string
  children: ReactElement<{ id?: string }>
}

function Field({ label, children }: FieldProps) {
  const id = useMemo(() => fieldId(label), [label])

  return (
    <label className="grid gap-1 text-sm text-ink-700" htmlFor={id}>
      <span className="field-label">{label}</span>
      {cloneField(children, id)}
    </label>
  )
}

interface CheckboxFieldProps {
  checked: boolean
  label: string
  onChange: (checked: boolean) => void
}

function CheckboxField({ checked, label, onChange }: CheckboxFieldProps) {
  return (
    <label className="flex items-center gap-2 rounded-md border border-ink-200 bg-white px-3 py-2 text-sm text-ink-700">
      <input
        checked={checked}
        className="h-4 w-4 rounded border-ink-300 text-amazon-600 focus:ring-amazon-500"
        onChange={(event) => onChange(event.target.checked)}
        type="checkbox"
      />
      <span>{label}</span>
    </label>
  )
}

interface StatusMessageProps {
  message: string | null
  tone: 'error' | 'success'
}

function StatusMessage({ message, tone }: StatusMessageProps) {
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

interface EmptyStateProps {
  title: string
  body: string
}

function EmptyState({ title, body }: EmptyStateProps) {
  return (
    <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
      <p className="text-sm font-medium text-ink-700">{title}</p>
      <p className="mt-1 text-xs text-ink-400">{body}</p>
    </div>
  )
}

function StatusBadge({ status }: { status: ProviderStatus | ModelStatus }) {
  const enabled = status === 'ENABLED'

  return (
    <span className={`rounded-md px-2 py-1 text-xs font-semibold ${enabled ? 'bg-emerald-50 text-emerald-700' : 'bg-ink-100 text-ink-500'}`}>
      {enabled ? '启用' : '禁用'}
    </span>
  )
}

function emptyProviderDraft(): ProviderDraft {
  return {
    editingId: null,
    type: 'OPENAI_COMPATIBLE',
    name: '',
    baseUrl: '',
    apiKey: '',
    timeoutSeconds: '120',
    concurrencyLimit: '0',
    status: 'ENABLED',
  }
}

function emptyModelDraft(providerId = ''): ModelDraft {
  return {
    editingId: null,
    providerId,
    modelName: '',
    displayName: '',
    supportsGenerate: true,
    supportsEdit: false,
    supportsMultiReference: false,
    supportsN: false,
    maxOutputCount: '1',
    supportedSizes: '1024x1024',
    supportedQualities: 'standard',
    supportedOutputFormats: 'png',
    pricingCurrency: 'USD',
    pricingUnitPrices: '',
    status: 'ENABLED',
  }
}

function buildProviderRequest(draft: ProviderDraft): ProviderFormRequest {
  const request: ProviderFormRequest = {
    name: draft.name.trim(),
    baseUrl: draft.baseUrl.trim(),
    status: draft.status,
    timeoutSeconds: parseIntegerField(draft.timeoutSeconds, '超时秒数', 1),
    concurrencyLimit: parseIntegerField(draft.concurrencyLimit, '并发限制', 0),
  }
  const apiKey = draft.apiKey.trim()
  if (apiKey) {
    request.apiKey = apiKey
  }
  return request
}

function requireProviderKey(value: string): string {
  const apiKey = value.trim()
  if (!apiKey) {
    throw new Error('请输入 Provider API Key。')
  }
  return apiKey
}

function buildModelRequest(draft: ModelDraft): UpdateModelRequest {
  if (!draft.supportsGenerate && !draft.supportsEdit) {
    throw new Error('至少选择一个模型能力。')
  }

  return {
    providerId: draft.providerId.trim(),
    modelName: draft.modelName.trim(),
    displayName: draft.displayName.trim(),
    supportsGenerate: draft.supportsGenerate,
    supportsEdit: draft.supportsEdit,
    supportsMultiReference: draft.supportsMultiReference,
    supportsN: draft.supportsN,
    maxOutputCount: parseIntegerField(draft.maxOutputCount, '最大输出数', 1),
    supportedSizes: parseListField(draft.supportedSizes),
    supportedQualities: parseListField(draft.supportedQualities),
    supportedOutputFormats: parseListField(draft.supportedOutputFormats),
    pricing: {
      currency: draft.pricingCurrency.trim() || 'USD',
      unitPrices: parseUnitPrices(draft.pricingUnitPrices),
    },
    status: draft.status,
  }
}

function parseIntegerField(value: string, label: string, min: number): number {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < min) {
    throw new Error(`${label}必须是不小于 ${min} 的整数。`)
  }
  return parsed
}

function parseListField(value: string): string[] {
  const seen = new Set<string>()
  return value
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item) => {
      if (seen.has(item)) {
        return false
      }
      seen.add(item)
      return true
    })
}

function parseUnitPrices(value: string): Record<string, number> {
  const unitPrices: Record<string, number> = {}
  const entries = value
    .split(/[\n,]+/)
    .map((item) => item.trim())
    .filter(Boolean)

  for (const entry of entries) {
    const [rawKey, rawValue] = entry.split(/[:=]/, 2)
    const key = rawKey?.trim()
    const parsed = Number(rawValue?.trim())
    if (!key || !Number.isFinite(parsed) || parsed < 0) {
      throw new Error('单位价格格式应为 image=0.04。')
    }
    unitPrices[key] = parsed
  }

  return unitPrices
}

function formatUnitPrices(unitPrices: Record<string, number>): string {
  return Object.entries(unitPrices)
    .map(([key, value]) => `${key}=${value}`)
    .join('\n')
}

function upsertById<TItem extends { id: string }>(items: TItem[], nextItem: TItem): TItem[] {
  const index = items.findIndex((item) => item.id === nextItem.id)
  if (index === -1) {
    return [nextItem, ...items]
  }

  const nextItems = [...items]
  nextItems[index] = nextItem
  return nextItems
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

function formatProviderTest(provider: Provider): string {
  if (!provider.lastTestStatus) {
    return '未测试'
  }

  const testedAt = provider.lastTestedAt ? ` · ${formatDateTime(provider.lastTestedAt)}` : ''
  return `${provider.lastTestStatus}${testedAt}`
}

function formatDateTime(value: string): string {
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function tabClassName(active: boolean): string {
  return `rounded px-3 py-1.5 text-sm font-semibold transition ${
    active ? 'bg-white text-ink-900 shadow-sm' : 'text-ink-500 hover:text-ink-900'
  }`
}

function fieldId(label: string): string {
  return `admin-${label.replace(/\s+/g, '-').toLowerCase()}`
}

function cloneField(element: ReactElement<{ id?: string }>, id: string): ReactElement {
  return cloneElement(element, { id })
}
