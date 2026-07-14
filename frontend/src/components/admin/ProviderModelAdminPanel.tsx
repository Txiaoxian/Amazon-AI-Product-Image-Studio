import { AlertTriangle, CheckCircle2, Loader2, Pencil, Plus, Power, PowerOff, RefreshCw, Trash2 } from 'lucide-react'
import { cloneElement, useCallback, useEffect, useMemo, useState, type ReactElement } from 'react'
import { isApiClientError } from '../../api/client'
import { modelApi as defaultModelApi, type CreateModelRequest, type ModelApi, type UpdateModelRequest } from '../../api/models'
import { providerApi as defaultProviderApi, type ProviderApi } from '../../api/providers'
import {
  MODEL_CAPABILITY_TEMPLATES,
  labelForQualityPreset,
  labelForSizePreset,
  modelParameterPresetsForProvider,
  type CapabilityPreset,
  type ModelCapabilityTemplate,
} from '../../lib/modelCapabilityPresets'
import type { Model, ModelStatus, Provider, ProviderStatus, ProviderType } from '../../types/platform'
import { Button } from '../ui/Button'
import { EditorDrawer } from '../ui/EditorDrawer'
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
const MAX_PROVIDER_TIMEOUT_SECONDS = 600

const providerTypeOptions: Array<{ value: ProviderType; label: string }> = [
  { value: 'OPENAI_COMPATIBLE', label: 'OpenAI-Compatible 中转接口' },
  { value: 'OPENAI', label: 'OpenAI 官方接口' },
  { value: 'GEMINI', label: 'Gemini 官方接口' },
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
  const [isProviderEditorOpen, setProviderEditorOpen] = useState(false)
  const [isModelEditorOpen, setModelEditorOpen] = useState(false)
  const [providerDraft, setProviderDraft] = useState<ProviderDraft>(() => emptyProviderDraft())
  const [modelDraft, setModelDraft] = useState<ModelDraft>(() => emptyModelDraft())

  const resetProviderFormState = useCallback(() => {
    setProviderDraft(emptyProviderDraft())
    setProviderFormError(null)
    setProviderNotice(null)
  }, [])

  const handleClose = () => {
    resetProviderFormState()
    setModelDraft(emptyModelDraft())
    setModelFormError(null)
    setProviderEditorOpen(false)
    setModelEditorOpen(false)
    onClose()
  }

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
      resetProviderFormState()
      setModelDraft(emptyModelDraft())
      setProviderEditorOpen(false)
      setModelEditorOpen(false)
      return
    }

    setProviderNotice(null)
    setModelNotice(null)
    setProviderFormError(null)
    setModelFormError(null)
    void refreshProviders()
    void refreshModels()
  }, [isOpen, refreshModels, refreshProviders, resetProviderFormState])

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
      setProviderNotice(providerDraft.editingId ? '中转站已更新。' : '中转站已创建。')
      setProviderEditorOpen(false)
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
      setModelEditorOpen(false)
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
    if (action === 'delete' && !window.confirm(`确定删除中转站 ${provider.name} 吗？`)) {
      return
    }

    setProviderActionId(provider.id)
    setProviderError(null)
    setProviderNotice(null)
    try {
      if (action === 'enable') {
        const saved = await providerApi.enable(provider.id, csrfToken)
        setProviders((current) => upsertById(current, saved))
        setProviderNotice('中转站已启用。')
      } else if (action === 'disable') {
        const saved = await providerApi.disable(provider.id, csrfToken)
        setProviders((current) => upsertById(current, saved))
        setProviderNotice('中转站已禁用。')
      } else if (action === 'delete') {
        await providerApi.delete(provider.id, csrfToken)
        setProviders((current) => current.filter((item) => item.id !== provider.id))
        setProviderNotice('中转站已删除。')
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
    <>
      <Modal isOpen={isOpen} maxWidthClass="max-w-6xl" onClose={handleClose} title="AI 中转站与模型管理">
      <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="inline-flex rounded-md border border-ink-200 bg-ink-50 p-1">
            {canManageProviders ? (
              <button
                className={tabClassName(activeTab === 'providers')}
                onClick={() => setActiveTab('providers')}
                type="button"
              >
                中转站
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
            error={providerError}
            isLoading={isLoadingProviders}
            notice={providerNotice}
            onAction={(provider, action) => void runProviderAction(provider, action)}
            onCreate={() => {
              setProviderDraft(emptyProviderDraft())
              setProviderFormError(null)
              setProviderEditorOpen(true)
            }}
            onEdit={(selected) => {
              setProviderDraft(providerDraftFromProvider(selected))
              setProviderFormError(null)
              setProviderEditorOpen(true)
            }}
            onRefresh={() => void refreshProviders()}
            providers={providers}
          />
        ) : null}

        {activeTab === 'models' ? (
          <ModelManagementView
            actionId={modelActionId}
            error={modelError}
            isLoading={isLoadingModels}
            models={models}
            notice={modelNotice}
            onAction={(model, action) => void runModelAction(model, action)}
            onCreate={() => {
              setModelDraft(emptyModelDraft(providers[0]?.id ?? ''))
              setModelFormError(null)
              setModelEditorOpen(true)
            }}
            onEdit={(selected) => {
              setModelDraft(modelDraftFromModel(selected))
              setModelFormError(null)
              setModelEditorOpen(true)
            }}
          />
        ) : null}
      </div>
      </Modal>

      <EditorDrawer
        isOpen={isOpen && isProviderEditorOpen}
        onClose={() => {
          setProviderEditorOpen(false)
          setProviderFormError(null)
        }}
        title={providerDraft.editingId ? '编辑中转站' : '新建中转站'}
      >
        <ProviderForm
          draft={providerDraft}
          editingProvider={editingProvider}
          error={providerFormError}
          isSaving={isSavingProvider}
          onDraftChange={setProviderDraft}
          onSubmit={() => void submitProvider()}
        />
      </EditorDrawer>

      <EditorDrawer
        isOpen={isOpen && isModelEditorOpen}
        onClose={() => {
          setModelEditorOpen(false)
          setModelFormError(null)
        }}
        title={modelDraft.editingId ? '编辑模型' : '新建模型'}
        widthClass="max-w-2xl"
      >
        <ModelForm
          draft={modelDraft}
          error={modelFormError}
          isSaving={isSavingModel}
          onDraftChange={setModelDraft}
          onSubmit={() => void submitModel()}
          providers={providers}
        />
      </EditorDrawer>
    </>
  )
}

interface ProviderManagementViewProps {
  providers: Provider[]
  isLoading: boolean
  actionId: string | null
  error: string | null
  notice: string | null
  onAction: (provider: Provider, action: 'enable' | 'disable' | 'delete' | 'test') => void
  onCreate: () => void
  onEdit: (provider: Provider) => void
  onRefresh: () => void
}

function ProviderManagementView({
  providers,
  isLoading,
  actionId,
  error,
  notice,
  onAction,
  onCreate,
  onEdit,
  onRefresh,
}: ProviderManagementViewProps) {
  return (
    <section className="min-w-0 space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div>
            <h3 className="text-sm font-semibold text-ink-900">中转站列表</h3>
            <p className="text-xs text-ink-400">{providers.length} 个中转站</p>
          </div>
          <div className="flex items-center gap-2">
            <Button icon={<Plus className="h-4 w-4" />} onClick={onCreate} variant="primary">新建中转站</Button>
            <button aria-label="刷新中转站" className="icon-button" disabled={isLoading} onClick={onRefresh} title="刷新中转站" type="button">
              <RefreshCw className={`h-4 w-4 ${isLoading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>

        <StatusMessage message={error} tone="error" />
        <StatusMessage message={notice} tone="success" />

        {isLoading ? <p className="rounded-md bg-ink-50 px-4 py-8 text-center text-sm text-ink-500">正在加载中转站...</p> : null}
        {!isLoading && providers.length === 0 ? (
          <EmptyState title="暂无中转站" body="创建中转站后可在这里启用、禁用、测试或维护密钥元数据。" />
        ) : null}

        <div className="space-y-2">
          {providers.map((provider) => (
            <ProviderListItem
              actionId={actionId}
              key={provider.id}
              onAction={onAction}
              onEdit={onEdit}
              provider={provider}
            />
          ))}
        </div>
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
          <button aria-label={`编辑中转站 ${provider.name}`} className="icon-button h-8 w-8" onClick={() => onEdit(provider)} title="编辑" type="button">
            <Pencil className="h-4 w-4" />
          </button>
          <button
            aria-label={`测试中转站 ${provider.name}`}
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
              aria-label={`禁用中转站 ${provider.name}`}
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
              aria-label={`启用中转站 ${provider.name}`}
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
            aria-label={`删除中转站 ${provider.name}`}
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
  onSubmit: () => void
}

function ProviderForm({ draft, editingProvider, error, isSaving, onDraftChange, onSubmit }: ProviderFormProps) {
  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <div className="grid gap-3">
        <Field label="中转站名称">
          <input
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, name: event.target.value }))}
            required
            value={draft.name}
          />
        </Field>
        <Field label="中转站类型">
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
        <Field label="接口地址（Base URL）">
          <input
            className="field-input"
            onChange={(event) => onDraftChange((current) => ({ ...current, baseUrl: event.target.value }))}
            placeholder="https://provider.example/v1"
            value={draft.baseUrl}
          />
        </Field>
        <Field label="密钥（API Key）">
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
              max={MAX_PROVIDER_TIMEOUT_SECONDS}
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
        <p className="-mt-2 text-xs leading-5 text-ink-500">AI 调用超时最长 600 秒（10 分钟）。网络慢或大图任务可适当调高，但过长会占用并发槽位。</p>
        <Field label="中转站状态">
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
      <div className="sticky bottom-0 -mx-1 border-t border-ink-200 bg-white pb-1 pt-3">
        <Button className="w-full" disabled={isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />} type="submit" variant="primary">
          保存中转站
        </Button>
      </div>
    </form>
  )
}

interface ModelManagementViewProps {
  models: Model[]
  isLoading: boolean
  actionId: string | null
  error: string | null
  notice: string | null
  onAction: (model: Model, action: 'enable' | 'disable' | 'delete') => void
  onCreate: () => void
  onEdit: (model: Model) => void
}

function ModelManagementView({
  models,
  isLoading,
  actionId,
  error,
  notice,
  onAction,
  onCreate,
  onEdit,
}: ModelManagementViewProps) {
  return (
    <section className="min-w-0 space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div>
            <h3 className="text-sm font-semibold text-ink-900">模型列表</h3>
            <p className="text-xs text-ink-400">{models.length} 个模型</p>
          </div>
          <Button icon={<Plus className="h-4 w-4" />} onClick={onCreate} variant="primary">新建模型</Button>
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
              onEdit={onEdit}
            />
          ))}
        </div>
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
            {formatSizeList(model.supportedSizes) || '未配置尺寸'} · {formatQualityList(model.supportedQualities) || '未配置质量'} ·{' '}
            {model.supportedOutputFormats.join(', ') || '未配置格式'}
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
  onSubmit: () => void
}

function ModelForm({ providers, draft, error, isSaving, onDraftChange, onSubmit }: ModelFormProps) {
  const selectedProvider = providers.find((provider) => provider.id === draft.providerId)
  const parameterPresets = modelParameterPresetsForProvider(selectedProvider?.type ?? 'OPENAI_COMPATIBLE')
  const templateOptions = selectedProvider
    ? MODEL_CAPABILITY_TEMPLATES.filter((template) => template.providerType === selectedProvider.type)
    : MODEL_CAPABILITY_TEMPLATES
  const applyTemplate = (template: ModelCapabilityTemplate) => {
    onDraftChange((current) => ({
      ...current,
      modelName: template.modelName,
      displayName: template.displayName,
      supportsGenerate: template.supportsGenerate,
      supportsEdit: template.supportsEdit,
      supportsMultiReference: template.supportsMultiReference,
      supportsN: template.supportsN,
      maxOutputCount: String(template.maxOutputCount),
      supportedSizes: formatListField(template.supportedSizes),
      supportedQualities: formatListField(template.supportedQualities),
      supportedOutputFormats: formatListField(template.supportedOutputFormats),
    }))
  }

  return (
    <form
      className="space-y-4"
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <div className="grid gap-3">
        {providers.length > 0 ? (
          <Field label="所属中转站">
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
          <Field label="中转站 ID">
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

        {templateOptions.length > 0 ? (
          <div className="rounded-lg border border-ink-200 bg-white p-3">
            <p className="text-xs font-semibold text-ink-600">模型常用配置</p>
            <div className="mt-2 flex flex-wrap gap-2">
              {templateOptions.map((template) => (
                <button
                  className="rounded-md border border-ink-200 px-3 py-2 text-xs font-semibold text-ink-700 transition hover:border-amazon-300 hover:bg-amazon-50"
                  key={template.id}
                  onClick={() => applyTemplate(template)}
                  type="button"
                >
                  {template.label}
                </button>
              ))}
            </div>
            <p className="mt-2 text-xs leading-5 text-ink-500">预设只填充模型能力参数，仍可手动增减比例、质量和格式。</p>
          </div>
        ) : null}

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

        <PresetCheckboxGroup
          help={parameterPresets.sizeHelp}
          label={parameterPresets.sizeLabel}
          presets={parameterPresets.sizePresets}
          selectedValues={parseListField(draft.supportedSizes)}
          onChange={(values) => onDraftChange((current) => ({ ...current, supportedSizes: formatListField(values) }))}
        />
        <PresetCheckboxGroup
          help={parameterPresets.qualityHelp}
          label={parameterPresets.qualityLabel}
          presets={parameterPresets.qualityPresets}
          selectedValues={parseListField(draft.supportedQualities)}
          onChange={(values) => onDraftChange((current) => ({ ...current, supportedQualities: formatListField(values) }))}
        />
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
      <div className="sticky bottom-0 -mx-1 border-t border-ink-200 bg-white pb-1 pt-3">
        <Button className="w-full" disabled={isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />} type="submit" variant="primary">
          保存模型
        </Button>
      </div>
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

interface PresetCheckboxGroupProps {
  label: string
  help: string
  presets: CapabilityPreset[]
  selectedValues: string[]
  onChange: (values: string[]) => void
}

function PresetCheckboxGroup({ help, label, presets, selectedValues, onChange }: PresetCheckboxGroupProps) {
  const selected = new Set(selectedValues)
  const presetValues = new Set(presets.map((preset) => preset.value))
  const customValues = selectedValues.filter((value) => !presetValues.has(value))

  return (
    <fieldset className="rounded-lg border border-ink-200 bg-white p-3">
      <legend className="px-1 text-xs font-semibold text-ink-600">{label}</legend>
      <p className="mb-2 text-xs leading-5 text-ink-500">{help}</p>
      <div className="mt-2 grid gap-2 sm:grid-cols-2">
        {presets.map((preset) => (
          <label
            className={`flex min-h-14 items-start gap-2 rounded-md border px-3 py-2 text-sm transition ${
              selected.has(preset.value) ? 'border-amazon-300 bg-amazon-50 text-ink-900' : 'border-ink-200 bg-white text-ink-700'
            }`}
            key={preset.value}
          >
            <input
              checked={selected.has(preset.value)}
              className="mt-0.5 h-4 w-4 rounded border-ink-300 text-amazon-600 focus:ring-amazon-500"
              onChange={(event) => onChange(toggleValue(selectedValues, preset.value, event.target.checked))}
              type="checkbox"
            />
            <span>
              <span className="block font-semibold">{preset.label}</span>
              {preset.description ? <span className="mt-0.5 block text-xs leading-5 text-ink-500">{preset.description}</span> : null}
            </span>
          </label>
        ))}
      </div>
      {customValues.length > 0 ? (
        <div className="mt-3 border-t border-ink-100 pt-3">
          <p className="text-xs font-semibold text-ink-600">现有自定义值</p>
          <p className="mt-1 text-xs leading-5 text-ink-500">这些值不属于当前协议预设；保留勾选不会丢失，取消后将从模型配置移除。</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {customValues.map((value) => (
              <label className="inline-flex items-center gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-900" key={value}>
                <input
                  checked
                  className="h-4 w-4 rounded border-ink-300 text-amazon-600 focus:ring-amazon-500"
                  onChange={(event) => onChange(toggleValue(selectedValues, value, event.target.checked))}
                  type="checkbox"
                />
                <span className="font-semibold">{value}</span>
              </label>
            ))}
          </div>
        </div>
      ) : null}
    </fieldset>
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

function providerDraftFromProvider(provider: Provider): ProviderDraft {
  return {
    editingId: provider.id,
    type: provider.type,
    name: provider.name,
    baseUrl: provider.baseUrl,
    apiKey: '',
    timeoutSeconds: String(provider.timeoutSeconds),
    concurrencyLimit: String(provider.concurrencyLimit),
    status: provider.status,
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
    supportedSizes: 'auto',
    supportedQualities: 'auto',
    supportedOutputFormats: 'png',
    pricingCurrency: 'USD',
    pricingUnitPrices: '',
    status: 'ENABLED',
  }
}

function modelDraftFromModel(model: Model): ModelDraft {
  return {
    editingId: model.id,
    providerId: model.providerId,
    modelName: model.modelName,
    displayName: model.displayName,
    supportsGenerate: model.supportsGenerate,
    supportsEdit: model.supportsEdit,
    supportsMultiReference: model.supportsMultiReference,
    supportsN: model.supportsN,
    maxOutputCount: String(model.maxOutputCount),
    supportedSizes: model.supportedSizes.join(', '),
    supportedQualities: model.supportedQualities.join(', '),
    supportedOutputFormats: model.supportedOutputFormats.join(', '),
    pricingCurrency: model.pricing.currency,
    pricingUnitPrices: formatUnitPrices(model.pricing.unitPrices),
    status: model.status,
  }
}

function buildProviderRequest(draft: ProviderDraft): ProviderFormRequest {
  const request: ProviderFormRequest = {
    name: draft.name.trim(),
    baseUrl: draft.baseUrl.trim(),
    status: draft.status,
    timeoutSeconds: parseIntegerField(draft.timeoutSeconds, '超时秒数', 1, MAX_PROVIDER_TIMEOUT_SECONDS),
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
    throw new Error('请输入中转站 API Key。')
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

function parseIntegerField(value: string, label: string, min: number, max?: number): number {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed < min) {
    throw new Error(`${label}必须是不小于 ${min} 的整数。`)
  }
  if (max !== undefined && parsed > max) {
    throw new Error(`${label}不能超过 ${max}。`)
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

function formatListField(values: string[]): string {
  return values.join(', ')
}

function toggleValue(values: string[], value: string, checked: boolean): string[] {
  const normalizedValues = values.filter(Boolean)
  if (checked) {
    return normalizedValues.includes(value) ? normalizedValues : [...normalizedValues, value]
  }
  return normalizedValues.filter((candidate) => candidate !== value)
}

function formatSizeList(values: string[]): string {
  return values.map(labelForSizePreset).join(', ')
}

function formatQualityList(values: string[]): string {
  return values.map(labelForQualityPreset).join(', ')
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
      const message = error.message.trim()
      return /[\u3400-\u9fff]/.test(message)
        ? message
        : '表单内容未通过校验，请检查填写内容。'
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
