import { Bookmark, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { deletePromptTemplate, listPromptTemplates, savePromptTemplate } from '../../db/promptTemplateRepository'
import type { PromptTemplate } from '../../db/dexie'
import { Button } from '../ui/Button'

interface PromptEditorProps {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  onError: (message: string) => void
}

export function PromptEditor({ value, onChange, disabled, onError }: PromptEditorProps) {
  const [templates, setTemplates] = useState<PromptTemplate[]>([])

  const refreshTemplates = useCallback(async () => {
    try {
      setTemplates(await listPromptTemplates())
    } catch (error) {
      onError(error instanceof Error ? error.message : '提示词模板读取失败。')
    }
  }, [onError])

  useEffect(() => {
    void refreshTemplates()
  }, [refreshTemplates])

  const saveCurrentPrompt = async () => {
    const prompt = value.trim()
    if (!prompt) {
      onError('请输入提示词后再保存模板。')
      return
    }

    await savePromptTemplate(prompt.slice(0, 28), prompt)
    await refreshTemplates()
  }

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <label className="field-label" htmlFor="prompt">
          提示词
        </label>
        <Button disabled={disabled} icon={<Bookmark className="h-4 w-4" />} onClick={saveCurrentPrompt} variant="ghost">
          保存模板
        </Button>
      </div>
      <textarea
        className="field-input min-h-40 resize-y leading-6"
        disabled={disabled}
        id="prompt"
        maxLength={4000}
        onChange={(event) => onChange(event.target.value)}
        placeholder="描述产品、背景、光线、构图、材质和亚马逊使用场景。"
        value={value}
      />
      {templates.length > 0 ? (
        <div className="space-y-2">
          <select
            className="field-input"
            disabled={disabled}
            onChange={(event) => {
              const template = templates.find((item) => item.id === event.target.value)
              if (template) {
                onChange(template.prompt)
              }
              event.target.value = ''
            }}
            value=""
          >
            <option value="">选择提示词模板</option>
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
                  onClick={() => onChange(template.prompt)}
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
                  onClick={async () => {
                    await deletePromptTemplate(template.id)
                    await refreshTemplates()
                  }}
                  title="删除模板"
                  type="button"
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </section>
  )
}
