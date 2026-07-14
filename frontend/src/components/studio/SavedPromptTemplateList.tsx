import { Trash2 } from 'lucide-react'
import type { PromptTemplate } from '../../db/dexie'

interface SavedPromptTemplateListProps {
  disabled?: boolean
  imageTypeLabel: string
  onDelete: (template: PromptTemplate) => void
  onSelect: (template: PromptTemplate) => void
  templates: PromptTemplate[]
}

export function SavedPromptTemplateList({
  disabled,
  imageTypeLabel,
  onDelete,
  onSelect,
  templates,
}: SavedPromptTemplateListProps) {
  if (templates.length === 0) {
    return null
  }

  return (
    <div className="space-y-2">
      <label className="field-label" htmlFor="saved-prompt-template">
        我的{imageTypeLabel}模板
      </label>
      <select
        className="field-input"
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
      <div className="grid gap-2">
        {templates.slice(0, 3).map((template) => (
          <div className="flex items-stretch overflow-hidden rounded-md border border-ink-200 bg-white" key={template.id}>
            <button
              aria-label={`填入模板 ${template.title}`}
              className="min-w-0 flex-1 px-3 py-2 text-left text-xs text-ink-700 transition hover:bg-ink-50"
              disabled={disabled}
              onClick={() => onSelect(template)}
              title={template.prompt}
              type="button"
            >
              <span className="block truncate font-medium">{template.title}</span>
              <span className="mt-0.5 block truncate text-ink-400">{template.prompt}</span>
            </button>
            <button
              aria-label={`删除模板 ${template.title}`}
              className="flex w-9 shrink-0 items-center justify-center border-l border-ink-200 text-ink-500 transition hover:bg-red-50 hover:text-red-700"
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
