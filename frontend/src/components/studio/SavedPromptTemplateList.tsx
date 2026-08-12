import { Trash2 } from 'lucide-react'
import type { PromptTemplate } from '../../db/dexie'

interface SavedPromptTemplateListProps {
  disabled?: boolean
  imageTypeLabel: string
  onDelete: (template: PromptTemplate) => void
  onSelect: (template: PromptTemplate) => void
  templates: PromptTemplate[]
  variant?: 'default' | 'compact'
}

export function SavedPromptTemplateList({
  disabled,
  imageTypeLabel,
  onDelete,
  onSelect,
  templates,
  variant = 'default',
}: SavedPromptTemplateListProps) {
  if (templates.length === 0) {
    return null
  }

  const isCompact = variant === 'compact'

  return (
    <div className={isCompact ? 'space-y-2 rounded-lg border border-white/10 bg-white/[0.03] p-2.5' : 'space-y-2'}>
      {isCompact ? (
        <div className="flex items-center justify-between gap-2">
          <label className="text-xs font-semibold text-slate-300" htmlFor="saved-prompt-template">
            {imageTypeLabel}模板库
          </label>
          <span className="text-[10px] text-slate-500">仅当前产品</span>
        </div>
      ) : (
        <label className="field-label" htmlFor="saved-prompt-template">
          我的{imageTypeLabel}模板
        </label>
      )}
      <select
        className={isCompact ? 'canvas-dark-field' : 'field-input'}
        disabled={disabled}
        id="saved-prompt-template"
        onChange={(event) => {
          const template = templates.find((item) => item.id === event.target.value)
          if (template) {
            onSelect(template)
          }
          event.target.value = ''
        }}
        value=""
      >
        <option value="">选择我的模板</option>
        {templates.map((template) => (
          <option key={template.id} value={template.id}>
            {template.title}
          </option>
        ))}
      </select>
      <div className={isCompact ? 'flex gap-2 overflow-x-auto pb-1' : 'grid gap-2'}>
        {templates.slice(0, isCompact ? templates.length : 3).map((template) => (
          <div
            className={isCompact
              ? 'flex min-w-[172px] items-stretch overflow-hidden rounded-md border border-white/10 bg-white/[0.04]'
              : 'flex items-stretch overflow-hidden rounded-md border border-ink-200 bg-white'}
            key={template.id}
          >
            <button
              aria-label={`填入模板 ${template.title}`}
              className={isCompact
                ? 'min-w-0 flex-1 px-2.5 py-2 text-left text-xs text-slate-200 transition hover:bg-white/[0.08]'
                : 'min-w-0 flex-1 px-3 py-2 text-left text-xs text-ink-700 transition hover:bg-ink-50'}
              disabled={disabled}
              onClick={() => onSelect(template)}
              title={template.prompt}
              type="button"
            >
              <span className="block truncate font-medium">{template.title}</span>
              <span className={isCompact ? 'mt-0.5 block truncate text-slate-500' : 'mt-0.5 block truncate text-ink-400'}>{template.prompt}</span>
            </button>
            <button
              aria-label={`删除模板 ${template.title}`}
              className={isCompact
                ? 'flex w-8 shrink-0 items-center justify-center border-l border-white/10 text-slate-500 transition hover:bg-red-500/10 hover:text-red-300'
                : 'flex w-9 shrink-0 items-center justify-center border-l border-ink-200 text-ink-500 transition hover:bg-red-50 hover:text-red-700'}
              disabled={disabled}
              onClick={() => onDelete(template)}
              title="删除模板"
              type="button"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
