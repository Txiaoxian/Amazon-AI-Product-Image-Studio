import { Bookmark, Maximize2 } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import {
  deletePromptTemplate,
  listPromptTemplates,
  savePromptTemplate,
  updatePromptTemplate,
} from '../../db/promptTemplateRepository'
import type { PromptTemplate } from '../../db/dexie'
import type { ProjectId } from '../../types/platform'
import { WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../../types/workbench'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'
import { PromptRecommendationPicker } from './PromptRecommendationPicker'
import { SavedPromptTemplateList } from './SavedPromptTemplateList'

interface PromptEditorProps {
  imageType: WorkbenchImageType
  projectId?: ProjectId | null
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  onError: (message: string) => void
  variant?: 'default' | 'compact'
}

export function PromptEditor({ imageType, projectId, value, onChange, disabled, onError, variant = 'default' }: PromptEditorProps) {
  const [templates, setTemplates] = useState<PromptTemplate[]>([])
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null)
  const [templateTitleHint, setTemplateTitleHint] = useState('')
  const [status, setStatus] = useState('')
  const [isExpandedEditorOpen, setExpandedEditorOpen] = useState(false)
  const expandedPromptRef = useRef<HTMLTextAreaElement>(null)
  const isMounted = useRef(true)
  const onErrorRef = useRef(onError)
  const templateRequestVersionRef = useRef(0)
  const imageTypeLabel = WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === imageType)?.label ?? '当前图片类型'
  const isCompact = variant === 'compact'

  const refreshTemplates = useCallback(async () => {
    const requestVersion = ++templateRequestVersionRef.current
    try {
      const nextTemplates = await listPromptTemplates(projectId, imageType)
      if (isMounted.current && requestVersion === templateRequestVersionRef.current) {
        setTemplates(nextTemplates)
      }
    } catch (error) {
      if (isMounted.current && requestVersion === templateRequestVersionRef.current) {
        onErrorRef.current(error instanceof Error ? error.message : '提示词模板读取失败。')
      }
    }
  }, [imageType, projectId])

  useEffect(() => {
    onErrorRef.current = onError
  }, [onError])

  useEffect(() => {
    setEditingTemplateId(null)
    setTemplateTitleHint('')
    setStatus('')
    void refreshTemplates()
  }, [refreshTemplates])

  useEffect(() => {
    isMounted.current = true
    return () => {
      isMounted.current = false
    }
  }, [])

  useEffect(() => {
    if (!isExpandedEditorOpen) {
      return
    }

    const textarea = expandedPromptRef.current
    textarea?.focus()
    textarea?.setSelectionRange(textarea.value.length, textarea.value.length)
  }, [isExpandedEditorOpen])

  const openExpandedEditor = () => {
    setExpandedEditorOpen(true)
  }

  const closeExpandedEditor = () => {
    setExpandedEditorOpen(false)
  }

  const saveCurrentPrompt = async () => {
    const prompt = value.trim()
    if (!prompt) {
      onError('请输入提示词后再保存模板。')
      return
    }

    const title = editingTemplateId ? createTemplateTitle(prompt) : templateTitleHint || createTemplateTitle(prompt)
    try {
      if (editingTemplateId) {
        await updatePromptTemplate(editingTemplateId, projectId, imageType, title, prompt)
        setStatus(`已更新${imageTypeLabel}模板。`)
      } else {
        const template = await savePromptTemplate(projectId, imageType, title, prompt)
        setEditingTemplateId(template.id)
        setTemplateTitleHint('')
        setStatus(`已保存到${imageTypeLabel}模板。`)
      }
      await refreshTemplates()
    } catch (error) {
      onError(error instanceof Error ? error.message : '提示词模板保存失败。')
    }
  }

  const selectSavedTemplate = (template: PromptTemplate) => {
    setEditingTemplateId(template.id)
    setTemplateTitleHint('')
    setStatus(`正在编辑“${template.title}”。`)
    onChange(template.prompt)
  }

  const deleteSavedTemplate = async (template: PromptTemplate) => {
    try {
      await deletePromptTemplate(template.id, projectId, imageType)
      if (editingTemplateId === template.id) {
        setEditingTemplateId(null)
      }
      setStatus(`已删除${imageTypeLabel}模板“${template.title}”。`)
      await refreshTemplates()
    } catch (error) {
      onError(error instanceof Error ? error.message : '提示词模板删除失败。')
    }
  }

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <label className="field-label" htmlFor="prompt">
          {isCompact ? '创作要求' : '提示词'}
        </label>
        {isCompact ? (
          <div className="flex min-w-0 items-center gap-2">
            <span className="truncate text-[11px] text-slate-400">用自然语言描述画面</span>
            <button
              aria-label={editingTemplateId ? `更新${imageTypeLabel}模板` : `保存到${imageTypeLabel}模板`}
              className="inline-flex min-h-8 shrink-0 items-center gap-1 rounded-md border border-amazon-500/35 bg-amazon-500/10 px-2 text-[11px] font-semibold text-amazon-300 transition hover:bg-amazon-500/20 focus:outline-none focus:ring-2 focus:ring-amazon-500/30 disabled:opacity-50"
              disabled={disabled}
              onClick={() => void saveCurrentPrompt()}
              title="保存到当前产品模板库"
              type="button"
            >
              <Bookmark className="h-3.5 w-3.5" />
              保存模板
            </button>
          </div>
        ) : (
          <Button disabled={disabled} icon={<Bookmark className="h-4 w-4" />} onClick={saveCurrentPrompt} variant="ghost">
            {editingTemplateId ? `更新${imageTypeLabel}模板` : `保存到${imageTypeLabel}模板`}
          </Button>
        )}
      </div>
      <div className="relative">
        <textarea
          aria-label={isCompact ? '提示词' : undefined}
          className={isCompact
            ? 'min-h-24 w-full resize-none rounded-lg border border-white/10 bg-white/[0.06] px-3 py-2.5 pr-11 text-sm leading-6 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-amazon-500/70 focus:ring-2 focus:ring-amazon-500/15'
            : 'field-input min-h-40 resize-y pr-12 leading-6'}
          disabled={disabled}
          id="prompt"
          maxLength={4000}
          onChange={(event) => {
            setStatus('')
            onChange(event.target.value)
          }}
          placeholder={isCompact ? `描述${imageTypeLabel}的场景、光线、构图和氛围。` : `描述${imageTypeLabel}的产品、背景、光线、构图、材质和使用要求。`}
          value={value}
        />
        <button
          aria-label="放大编辑提示词"
          className={isCompact
            ? 'absolute right-2 top-2 flex h-8 w-8 items-center justify-center rounded-md text-slate-400 transition hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-amazon-500/30'
            : 'icon-button absolute right-2 top-2 h-9 w-9 bg-white/95 shadow-sm'}
          disabled={disabled}
          onClick={openExpandedEditor}
          title="放大编辑提示词"
          type="button"
        >
          <Maximize2 className="h-4 w-4" />
        </button>
      </div>
      <PromptRecommendationPicker
        disabled={disabled}
        imageType={imageType}
        imageTypeLabel={imageTypeLabel}
        key={`${imageType}-${variant}`}
        onSelect={(recommendation) => {
          setEditingTemplateId(null)
          setTemplateTitleHint(`${recommendation.title}（自定义）`)
          setStatus(`已填入“${recommendation.title}”，可继续修改。`)
          onChange(recommendation.prompt)
        }}
        variant={isCompact ? 'compact' : 'default'}
      />
      {status ? (
        <p className="text-xs leading-5 text-ink-500" role="status">
          {status}
        </p>
      ) : null}
      <SavedPromptTemplateList
        disabled={disabled}
        imageTypeLabel={imageTypeLabel}
        onDelete={(template) => void deleteSavedTemplate(template)}
        onSelect={selectSavedTemplate}
        templates={templates}
        variant={isCompact ? 'compact' : 'default'}
      />
      <Modal
        isOpen={isExpandedEditorOpen}
        maxWidthClass="max-w-4xl"
        onClose={closeExpandedEditor}
        title={`编辑${imageTypeLabel}提示词`}
      >
        <div className="flex h-full min-h-0 flex-col gap-2">
          <label className="field-label" htmlFor="expanded-prompt">
            完整提示词
          </label>
          <textarea
            className="field-input min-h-80 flex-1 resize-none leading-7 sm:min-h-[56vh]"
            disabled={disabled}
            id="expanded-prompt"
            maxLength={4000}
            onChange={(event) => {
              setStatus('')
              onChange(event.target.value)
            }}
            placeholder={`详细描述${imageTypeLabel}的产品、背景、光线、构图、材质和使用要求。`}
            ref={expandedPromptRef}
            value={value}
          />
          <div className="flex items-center justify-between gap-3 text-xs text-ink-400">
            <span>修改会实时同步到生成参数</span>
            <span>{value.length} / 4000</span>
          </div>
        </div>
      </Modal>
    </section>
  )
}

function createTemplateTitle(prompt: string): string {
  return prompt.split('\n').find((line) => line.trim())?.trim().slice(0, 28) ?? prompt.slice(0, 28)
}
