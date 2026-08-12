import { Library, Pencil, Plus, Search, Sparkles, Star, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import type { PromptTemplate } from '../../db/dexie'
import {
  deletePromptTemplate,
  listPromptTemplatesForProject,
  savePromptTemplate,
  updatePromptTemplate,
} from '../../db/promptTemplateRepository'
import { getPromptRecommendations } from '../../lib/promptRecommendations'
import type { Project, ProjectId } from '../../types/platform'
import { WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../../types/workbench'
import { PageHeader } from '../layout/PageHeader'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

interface TemplateLibraryPageProps {
  onNotice: (message: string) => void
  onSelectProject?: (projectId: ProjectId) => void
  onUseTemplate: (imageType: WorkbenchImageType, prompt: string) => void
  projectId?: ProjectId | null
  projectName?: string
  projects?: Project[]
}

type TemplateCategory = WorkbenchImageType | 'ALL'

interface TemplateFormState {
  imageType: WorkbenchImageType
  prompt: string
  title: string
}

const EMPTY_TEMPLATE_FORM: TemplateFormState = {
  imageType: 'MAIN',
  prompt: '',
  title: '',
}

export function TemplateLibraryPage({
  onNotice,
  onSelectProject,
  onUseTemplate,
  projectId = null,
  projectName,
  projects = [],
}: TemplateLibraryPageProps) {
  const [category, setCategory] = useState<TemplateCategory>('ALL')
  const [query, setQuery] = useState('')
  const [savedTemplates, setSavedTemplates] = useState<PromptTemplate[]>([])
  const [isLoading, setLoading] = useState(true)
  const [editingTemplate, setEditingTemplate] = useState<PromptTemplate | null>(null)
  const [isEditorOpen, setEditorOpen] = useState(false)
  const [isSaving, setSaving] = useState(false)
  const [form, setForm] = useState<TemplateFormState>(EMPTY_TEMPLATE_FORM)
  const loadRequestVersionRef = useRef(0)

  const loadTemplates = useCallback(async () => {
    const requestVersion = ++loadRequestVersionRef.current
    if (!projectId) {
      setSavedTemplates([])
      setLoading(false)
      return
    }

    setLoading(true)
    try {
      const templates = await listPromptTemplatesForProject(projectId)
      if (requestVersion === loadRequestVersionRef.current) {
        setSavedTemplates(templates)
      }
    } catch {
      if (requestVersion === loadRequestVersionRef.current) {
        onNotice('模板读取失败，请稍后重试。')
      }
    } finally {
      if (requestVersion === loadRequestVersionRef.current) {
        setLoading(false)
      }
    }
  }, [onNotice, projectId])

  useEffect(() => {
    void loadTemplates()
  }, [loadTemplates])

  const builtinTemplates = useMemo(
    () => WORKBENCH_IMAGE_TYPE_OPTIONS.flatMap((option) => getPromptRecommendations(option.value).map((template) => ({
      ...template,
      imageType: option.value,
      kind: 'builtin' as const,
    }))),
    [],
  )
  const normalizedQuery = query.trim().toLocaleLowerCase('zh-CN')
  const matches = (item: { title: string; prompt: string; imageType: WorkbenchImageType }) => (
    (category === 'ALL' || item.imageType === category)
    && (!normalizedQuery || `${item.title}\n${item.prompt}`.toLocaleLowerCase('zh-CN').includes(normalizedQuery))
  )
  const visibleSaved = savedTemplates.filter(matches)
  const visibleBuiltin = builtinTemplates.filter(matches)

  const openCreateEditor = () => {
    if (!projectId) {
      onNotice('请先选择产品，再新增提示词模板。')
      return
    }
    setEditingTemplate(null)
    setForm({
      ...EMPTY_TEMPLATE_FORM,
      imageType: category === 'ALL' ? 'MAIN' : category,
    })
    setEditorOpen(true)
  }

  const openEditEditor = (template: PromptTemplate) => {
    setEditingTemplate(template)
    setForm({ imageType: template.imageType, prompt: template.prompt, title: template.title })
    setEditorOpen(true)
  }

  const closeEditor = () => {
    if (isSaving) {
      return
    }
    setEditorOpen(false)
    setEditingTemplate(null)
  }

  const saveTemplate = async () => {
    const title = form.title.trim()
    const prompt = form.prompt.trim()
    if (!projectId) {
      onNotice('请先选择产品，再保存提示词模板。')
      return
    }
    if (!title) {
      onNotice('请输入模板名称。')
      return
    }
    if (!prompt) {
      onNotice('请输入提示词内容。')
      return
    }

    setSaving(true)
    try {
      if (editingTemplate) {
        await updatePromptTemplate(editingTemplate.id, projectId, editingTemplate.imageType, title, prompt)
        onNotice('提示词模板已更新。')
      } else {
        await savePromptTemplate(projectId, form.imageType, title, prompt)
        onNotice('提示词模板已添加到当前产品。')
      }
      await loadTemplates()
      setEditorOpen(false)
      setEditingTemplate(null)
    } catch (error) {
      onNotice(error instanceof Error ? error.message : '模板保存失败，请稍后重试。')
    } finally {
      setSaving(false)
    }
  }

  const removeTemplate = async (template: PromptTemplate) => {
    if (!projectId || !window.confirm(`确定删除模板“${template.title}”吗？`)) return
    try {
      await deletePromptTemplate(template.id, projectId, template.imageType)
      setSavedTemplates((items) => items.filter((item) => item.id !== template.id))
      onNotice('模板已删除。')
    } catch {
      onNotice('模板删除失败，请稍后重试。')
    }
  }

  const currentProjectLabel = projectName ?? (projectId ? '当前产品' : '未选择产品')

  return (
    <div className="workspace-page">
      <div className="workspace-page-inner">
        <PageHeader
          actions={(
            <div className="flex flex-wrap items-end justify-end gap-2">
              {projects.length > 0 && onSelectProject ? (
                <label className="grid gap-1 text-xs font-semibold text-ink-500" htmlFor="template-library-project">
                  当前产品
                  <select
                    className="field-input min-w-44"
                    id="template-library-project"
                    onChange={(event) => onSelectProject(event.target.value as ProjectId)}
                    value={projectId ?? ''}
                  >
                    {projects.map((project) => (
                      <option key={project.id} value={project.id}>{project.name}</option>
                    ))}
                  </select>
                </label>
              ) : null}
              <Button disabled={!projectId} icon={<Plus className="h-4 w-4" />} onClick={openCreateEditor} variant="primary">
                新增提示词模板
              </Button>
            </div>
          )}
          description={projectId
            ? `管理「${currentProjectLabel}」专属的提示词模板；平台推荐模板可直接套用，个人模板仅在当前产品中可见。`
            : '请先选择一个产品，再管理该产品专属的提示词模板。'}
          eyebrow="创作资源"
          title="模板库"
        />

        <section className="workspace-card p-4">
          <div className="grid gap-3 lg:grid-cols-[minmax(260px,1fr)_auto]">
            <label className="relative block">
              <span className="sr-only">搜索模板</span>
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-ink-400" />
              <input className="field-input pl-9" onChange={(event) => setQuery(event.target.value)} placeholder="搜索模板标题或提示词" value={query} />
            </label>
            <div className="flex max-w-full gap-2 overflow-x-auto" role="tablist" aria-label="模板分类">
              <CategoryButton active={category === 'ALL'} label="全部" onClick={() => setCategory('ALL')} />
              {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
                <CategoryButton active={category === option.value} key={option.value} label={option.label} onClick={() => setCategory(option.value)} />
              ))}
            </div>
          </div>
        </section>

        <TemplateSection
          icon={<Star className="h-5 w-5 text-amazon-600" />}
          title={projectId ? `当前产品模板 · ${currentProjectLabel}` : '当前产品模板'}
        >
          {visibleSaved.map((template) => (
            <TemplateCard
              description="仅当前产品可用"
              imageType={template.imageType}
              key={template.id}
              onDelete={() => void removeTemplate(template)}
              onEdit={() => openEditEditor(template)}
              onUse={() => onUseTemplate(template.imageType, template.prompt)}
              prompt={template.prompt}
              title={template.title}
            />
          ))}
          {!isLoading && visibleSaved.length === 0 ? (
            <TemplateEmpty text={projectId ? '当前产品还没有符合条件的模板，可点击右上角新增。' : '选择产品后，这里会显示该产品专属模板。'} />
          ) : null}
          {isLoading ? <TemplateEmpty text="正在加载当前产品模板..." /> : null}
        </TemplateSection>

        <TemplateSection
          icon={<Library className="h-5 w-5 text-amazon-600" />}
          title="平台推荐"
        >
          {visibleBuiltin.map((template) => (
            <TemplateCard
              description={template.description}
              imageType={template.imageType}
              key={template.id}
              onUse={() => onUseTemplate(template.imageType, template.prompt)}
              prompt={template.prompt}
              title={template.title}
            />
          ))}
          {visibleBuiltin.length === 0 ? <TemplateEmpty text="暂无符合当前条件的平台模板。" /> : null}
        </TemplateSection>
      </div>

      <Modal
        footer={(
          <div className="flex justify-end gap-2">
            <Button disabled={isSaving} onClick={closeEditor} variant="ghost">取消</Button>
            <Button disabled={isSaving} onClick={() => void saveTemplate()} variant="primary">
              {isSaving ? '保存中...' : editingTemplate ? '保存修改' : '添加模板'}
            </Button>
          </div>
        )}
        isOpen={isEditorOpen}
        maxWidthClass="max-w-2xl"
        onClose={closeEditor}
        title={editingTemplate ? '编辑提示词模板' : '新增提示词模板'}
      >
        <div className="space-y-4">
          <label className="grid gap-2 text-sm font-semibold text-ink-700" htmlFor="template-form-image-type">
            图片类型
            <select
              className="field-input"
              disabled={Boolean(editingTemplate) || isSaving}
              id="template-form-image-type"
              onChange={(event) => setForm((current) => ({ ...current, imageType: event.target.value as WorkbenchImageType }))}
              value={form.imageType}
            >
              {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>{option.label}</option>
              ))}
            </select>
            {editingTemplate ? <span className="text-xs font-normal text-ink-400">编辑时不改变图片类型。</span> : null}
          </label>
          <label className="grid gap-2 text-sm font-semibold text-ink-700" htmlFor="template-form-title">
            模板名称
            <input
              className="field-input"
              disabled={isSaving}
              id="template-form-title"
              maxLength={80}
              onChange={(event) => setForm((current) => ({ ...current, title: event.target.value }))}
              placeholder="例如：厨房自然光场景"
              value={form.title}
            />
          </label>
          <label className="grid gap-2 text-sm font-semibold text-ink-700" htmlFor="template-form-prompt">
            提示词
            <textarea
              className="field-input min-h-64 resize-y leading-6"
              disabled={isSaving}
              id="template-form-prompt"
              maxLength={4000}
              onChange={(event) => setForm((current) => ({ ...current, prompt: event.target.value }))}
              placeholder="输入可复用的提示词内容。"
              value={form.prompt}
            />
          </label>
          <p className="rounded-md bg-amazon-50 px-3 py-2 text-xs leading-5 text-amazon-800">保存后仅当前产品可以看到和使用此模板。</p>
        </div>
      </Modal>
    </div>
  )
}

function CategoryButton({ active, label, onClick }: { active: boolean; label: string; onClick: () => void }) {
  return (
    <button
      aria-selected={active}
      className={`min-h-10 shrink-0 rounded-lg border px-3 text-sm font-semibold transition ${active ? 'border-amazon-500 bg-amazon-500/10 text-amazon-600' : 'border-ink-200 bg-white text-ink-600 hover:bg-ink-50'}`}
      onClick={onClick}
      role="tab"
      type="button"
    >
      {label}
    </button>
  )
}

function TemplateSection({ children, icon, title }: { children: ReactNode; icon: ReactNode; title: string }) {
  return (
    <section className="mt-6">
      <div className="mb-3 flex items-center gap-2">
        {icon}
        <h2 className="text-base font-semibold text-ink-900">{title}</h2>
      </div>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{children}</div>
    </section>
  )
}

function TemplateCard({ description, imageType, onDelete, onEdit, onUse, prompt, title }: {
  description: string
  imageType: WorkbenchImageType
  onDelete?: () => void
  onEdit?: () => void
  onUse: () => void
  prompt: string
  title: string
}) {
  const imageTypeLabel = WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === imageType)?.label ?? imageType
  return (
    <article className="workspace-card flex min-h-60 flex-col p-5">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <span className="inline-flex rounded-full bg-amazon-500/10 px-2 py-1 text-[11px] font-semibold text-amazon-600">{imageTypeLabel}</span>
          <h3 className="mt-3 truncate text-base font-semibold text-ink-900">{title}</h3>
          <p className="mt-1 text-xs leading-5 text-ink-500">{description}</p>
        </div>
        {onEdit || onDelete ? (
          <div className="flex shrink-0 items-center gap-1">
            {onEdit ? (
              <button aria-label={`编辑模板 ${title}`} className="icon-button h-9 w-9 hover:border-amazon-200 hover:bg-amazon-50 hover:text-amazon-700" onClick={onEdit} title="编辑模板" type="button">
                <Pencil className="h-4 w-4" />
              </button>
            ) : null}
            {onDelete ? (
              <button aria-label={`删除模板 ${title}`} className="icon-button h-9 w-9 hover:border-red-200 hover:bg-red-50 hover:text-red-700" onClick={onDelete} title="删除模板" type="button">
                <Trash2 className="h-4 w-4" />
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
      <p className="mt-4 line-clamp-4 text-sm leading-6 text-ink-600">{prompt}</p>
      <Button className="mt-auto w-full" icon={<Sparkles className="h-4 w-4" />} onClick={onUse} variant="primary">使用此模板</Button>
    </article>
  )
}

function TemplateEmpty({ text }: { text: string }) {
  return <div className="workspace-card col-span-full px-5 py-10 text-center text-sm text-ink-500">{text}</div>
}
