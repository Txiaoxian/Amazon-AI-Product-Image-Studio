import { Loader2, Save } from 'lucide-react'
import type { Model } from '../../types/platform'
import { Button } from '../ui/Button'

interface UserModelAccessEditorProps {
  userEmail: string
  models: Model[]
  modelIds: string[]
  isLoading: boolean
  isSaving: boolean
  error: string | null
  onChange: (modelIds: string[]) => void
  onSave: () => void
  onCancel: () => void
}

export function UserModelAccessEditor({
  error,
  isLoading,
  isSaving,
  modelIds,
  models,
  onCancel,
  onChange,
  onSave,
  userEmail,
}: UserModelAccessEditorProps) {
  const groups = groupModelsByProvider(models)

  return (
    <div className="space-y-3 rounded-md border border-amazon-200 bg-amazon-50/40 p-3">
      <div>
        <h4 className="text-sm font-semibold text-ink-900">可用中转站与模型</h4>
        <p className="mt-1 text-xs leading-5 text-ink-500">
          按中转站分组选择模型。用户只会在生图参数中看到已分配且已启用的模型，不会看到中转站或模型管理页面。
        </p>
      </div>

      {isLoading ? (
        <p className="rounded-md bg-white px-3 py-4 text-center text-xs text-ink-500">正在加载可分配模型...</p>
      ) : null}

      {!isLoading && groups.length === 0 ? (
        <p className="rounded-md border border-ink-200 bg-white px-3 py-3 text-xs text-ink-500">暂无可分配模型，请先在“中转站与模型管理”中创建模型。</p>
      ) : null}

      {!isLoading ? (
        <div aria-label={`可用模型 ${userEmail}`} className="max-h-72 space-y-2 overflow-y-auto pr-1">
          {groups.map((group) => (
            <section className="rounded-md border border-ink-200 bg-white p-3" key={group.providerId}>
              <div className="mb-2 flex items-center justify-between gap-2">
                <h5 className="text-xs font-semibold text-ink-800">{group.providerName}</h5>
                <span className="text-[11px] text-ink-400">{group.models.length} 个模型</span>
              </div>
              <div className="grid gap-2 md:grid-cols-2">
                {group.models.map((model) => {
                  const checked = modelIds.includes(model.id)
                  return (
                    <label className="flex cursor-pointer items-start gap-2 rounded-md border border-ink-100 px-2.5 py-2 hover:bg-ink-50" key={model.id}>
                      <input
                        aria-label={`${model.displayName} ${group.providerName}`}
                        checked={checked}
                        className="mt-0.5 h-4 w-4 rounded border-ink-300 text-amazon-600 focus:ring-amazon-500"
                        onChange={() => onChange(toggleModelId(modelIds, model.id))}
                        type="checkbox"
                      />
                      <span className="min-w-0">
                        <span className="block truncate text-xs font-semibold text-ink-800">{model.displayName}</span>
                        <span className="block truncate text-[11px] text-ink-400">
                          {model.modelName} · {model.status === 'ENABLED' ? '已启用' : '已停用'}
                        </span>
                      </span>
                    </label>
                  )
                })}
              </div>
            </section>
          ))}
        </div>
      ) : null}

      {error ? <p className="text-sm leading-6 text-red-700" role="alert">{error}</p> : null}
      <div className="flex flex-wrap gap-2">
        <Button disabled={isLoading || isSaving} icon={isSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />} onClick={onSave} variant="primary">
          保存可用模型
        </Button>
        <Button disabled={isSaving} onClick={onCancel}>
          取消
        </Button>
      </div>
    </div>
  )
}

function groupModelsByProvider(models: Model[]) {
  const groups = new Map<string, { providerId: string; providerName: string; models: Model[] }>()
  for (const model of models) {
    const providerId = String(model.providerId)
    const current = groups.get(providerId) ?? {
      providerId,
      providerName: model.providerName || '未命名中转站',
      models: [],
    }
    current.models.push(model)
    groups.set(providerId, current)
  }
  return [...groups.values()]
    .map((group) => ({
      ...group,
      models: [...group.models].sort((left, right) => left.displayName.localeCompare(right.displayName, 'zh-CN')),
    }))
    .sort((left, right) => left.providerName.localeCompare(right.providerName, 'zh-CN'))
}

function toggleModelId(values: string[], value: string): string[] {
  const normalized = String(value)
  return values.includes(normalized)
    ? values.filter((candidate) => candidate !== normalized)
    : [...values, normalized]
}
