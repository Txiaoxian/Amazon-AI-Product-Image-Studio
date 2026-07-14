import { Loader2, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { type FormEvent, type ReactElement, useEffect, useMemo, useState } from 'react'
import type { Project, ProjectId, ProjectMember, ProjectMemberCandidate, ProjectMemberRole, ProjectStatus, UserId } from '../../types/platform'
import { Button } from '../ui/Button'
import { EditorDrawer } from '../ui/EditorDrawer'
import { Modal } from '../ui/Modal'

interface ProjectDraft {
  name: string
  brand: string
  asin: string
  site: string
  notes: string
  status: ProjectStatus
  sortOrder: string
}

interface ProjectManagementModalProps {
  actionUserId: UserId | string | null
  candidates: ProjectMemberCandidate[]
  candidateStatus: 'idle' | 'loading' | 'success' | 'error'
  canDeleteProject: boolean
  canManageProjectMembers: boolean
  error: string | null
  initialProject: Project | null
  isCreatingProject: boolean
  isDeletingProject: boolean
  isOpen: boolean
  isSavingMember: boolean
  isUpdatingProject: boolean
  memberError: string | null
  members: ProjectMember[]
  memberStatus: 'idle' | 'loading' | 'success' | 'error'
  projects: Project[]
  onAddMember: (projectId: ProjectId, request: { userId: UserId | string; role: ProjectMemberRole }) => void
  onClose: () => void
  onCreateProject: (request: { name: string; brand?: string; asin?: string; site?: string; notes?: string; status?: ProjectStatus; sortOrder?: number }) => void
  onDeleteProject: (project: Project) => void
  onRefreshCandidates: (projectId: ProjectId, q?: string) => void
  onRefreshMembers: (projectId: ProjectId) => void
  onRemoveMember: (projectId: ProjectId, userId: UserId | string) => void
  onSelectProject: (projectId: ProjectId) => void
  onUpdateMember: (projectId: ProjectId, userId: UserId | string, request: { role: ProjectMemberRole }) => void
  onUpdateProject: (projectId: ProjectId, request: { name?: string; brand?: string; asin?: string; site?: string; notes?: string; status?: ProjectStatus; sortOrder?: number }) => void
}

export function ProjectManagementModal({
  actionUserId,
  candidates,
  candidateStatus,
  canDeleteProject,
  canManageProjectMembers,
  error,
  initialProject,
  isCreatingProject,
  isDeletingProject,
  isOpen,
  isSavingMember,
  isUpdatingProject,
  memberError,
  members,
  memberStatus,
  projects,
  onAddMember,
  onClose,
  onCreateProject,
  onDeleteProject,
  onRefreshCandidates,
  onRefreshMembers,
  onRemoveMember,
  onSelectProject,
  onUpdateMember,
  onUpdateProject,
}: ProjectManagementModalProps) {
  const [activeProjectId, setActiveProjectId] = useState<ProjectId | null>(null)
  const [isProjectEditorOpen, setProjectEditorOpen] = useState(false)
  const [draft, setDraft] = useState<ProjectDraft>(() => emptyDraft())
  const [candidateQuery, setCandidateQuery] = useState('')
  const [memberDraft, setMemberDraft] = useState<{ userId: string; role: ProjectMemberRole }>({ userId: '', role: 'VIEWER' })

  const activeProject = useMemo(
    () => projects.find((project) => project.id === activeProjectId) ?? null,
    [activeProjectId, projects],
  )
  const activeProjectIdForRequests = activeProject?.id ?? null

  useEffect(() => {
    if (!isOpen) {
      return
    }
    setActiveProjectId(initialProject?.id ?? null)
    setProjectEditorOpen(true)
  }, [initialProject?.id, isOpen])

  useEffect(() => {
    if (!activeProject) {
      setDraft(emptyDraft())
      return
    }
    setDraft({
      name: activeProject.name,
      brand: activeProject.brand,
      asin: activeProject.asin,
      site: activeProject.site,
      notes: activeProject.notes,
      status: activeProject.status,
      sortOrder: String(activeProject.sortOrder),
    })
  }, [activeProject])

  useEffect(() => {
    if (!isOpen || !activeProjectIdForRequests || !canManageProjectMembers) {
      return
    }
    onRefreshMembers(activeProjectIdForRequests)
  }, [activeProjectIdForRequests, canManageProjectMembers, isOpen, onRefreshMembers])

  useEffect(() => {
    if (!isOpen || !activeProjectIdForRequests || !canManageProjectMembers) {
      return
    }
    onRefreshCandidates(activeProjectIdForRequests, candidateQuery)
  }, [activeProjectIdForRequests, canManageProjectMembers, candidateQuery, isOpen, onRefreshCandidates])

  const submitProject = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const payload = {
      name: draft.name.trim(),
      brand: draft.brand.trim() || undefined,
      asin: draft.asin.trim() || undefined,
      site: draft.site.trim() || undefined,
      notes: draft.notes.trim() || undefined,
      status: draft.status,
      sortOrder: parseSortOrder(draft.sortOrder),
    }
    if (activeProject) {
      onUpdateProject(activeProject.id, payload)
    } else {
      onCreateProject(payload)
    }
  }

  const submitMember = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!activeProject || !memberDraft.userId) {
      return
    }
    onAddMember(activeProject.id, memberDraft)
    setMemberDraft({ userId: '', role: 'VIEWER' })
  }

  return (
    <>
      <Modal isOpen={isOpen} maxWidthClass="max-w-6xl" onClose={onClose} title="产品管理">
        <div className="grid gap-4 lg:grid-cols-[260px_minmax(0,1fr)]">
        <aside className="rounded-lg border border-ink-200 bg-ink-50 p-3">
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-ink-900">产品列表</h3>
            <button
              className="text-xs font-semibold text-amazon-700"
              onClick={() => {
                setActiveProjectId(null)
                setDraft(emptyDraft())
                setProjectEditorOpen(true)
              }}
              type="button"
            >
              新建
            </button>
          </div>
          <div className="grid gap-2">
            {projects.map((project) => (
              <button
                className={`rounded-md border px-3 py-2 text-left transition ${
                  project.id === activeProjectId ? 'border-amazon-300 bg-white text-amazon-800' : 'border-ink-200 bg-white text-ink-700 hover:bg-ink-50'
                }`}
                key={project.id}
                onClick={() => {
                  setActiveProjectId(project.id)
                  setProjectEditorOpen(false)
                  onSelectProject(project.id)
                }}
                type="button"
              >
                <span className="block truncate text-sm font-semibold">{project.name}</span>
                <span className="block truncate text-xs text-ink-500">
                  {project.brand || '未填写品牌'} · {projectStatusLabel(project.status)} · 排序 {project.sortOrder}
                </span>
              </button>
            ))}
            {projects.length === 0 ? <p className="rounded-md bg-white px-3 py-8 text-center text-sm text-ink-400">暂无产品</p> : null}
          </div>
        </aside>

        <div className="grid gap-4">
          {activeProject ? (
            <section className="rounded-lg border border-ink-200 bg-white p-4">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <h3 className="truncate text-sm font-semibold text-ink-900">{activeProject.name}</h3>
                  <p className="mt-1 text-xs text-ink-500">
                    {activeProject.brand || '未填写品牌'} · {activeProject.asin || '未填写 ASIN'} · {activeProject.site || '未填写站点'}
                  </p>
                  <p className="mt-1 text-xs text-ink-500">
                    {projectStatusLabel(activeProject.status)} · 排序 {activeProject.sortOrder}
                  </p>
                </div>
                <Button icon={<Pencil className="h-4 w-4" />} onClick={() => setProjectEditorOpen(true)} variant="secondary">
                  编辑产品
                </Button>
              </div>
              {activeProject.notes ? <p className="mt-3 rounded-md bg-ink-50 px-3 py-2 text-sm leading-6 text-ink-600">{activeProject.notes}</p> : null}
            </section>
          ) : (
            <section className="rounded-lg border border-dashed border-ink-200 bg-ink-50 px-4 py-10 text-center">
              <h3 className="text-sm font-semibold text-ink-800">选择一个产品查看详情</h3>
              <p className="mt-1 text-xs text-ink-500">编辑表单会在独立侧栏中打开，不会挤压产品列表和成员信息。</p>
              <Button className="mt-4" icon={<Plus className="h-4 w-4" />} onClick={() => {
                setDraft(emptyDraft())
                setProjectEditorOpen(true)
              }} variant="primary">
                新建产品
              </Button>
            </section>
          )}

          {canManageProjectMembers && activeProject ? (
            <section className="rounded-lg border border-ink-200 bg-white p-4">
              <div className="mb-3 flex items-center justify-between gap-2">
                <div>
                  <h3 className="text-sm font-semibold text-ink-900">产品成员</h3>
                  <p className="mt-1 text-xs text-ink-500">所有者可编辑产品和成员；编辑者可维护素材；查看者只能查看产品内容。</p>
                </div>
                <button className="icon-button" onClick={() => onRefreshMembers(activeProject.id)} title="刷新成员" type="button">
                  <RefreshCw className={`h-4 w-4 ${memberStatus === 'loading' ? 'animate-spin' : ''}`} />
                </button>
              </div>

              <form className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_160px_auto]" onSubmit={submitMember}>
                <select
                  aria-label="选择产品成员"
                  className="field-input"
                  disabled={isSavingMember || candidateStatus === 'loading'}
                  onChange={(event) => setMemberDraft((current) => ({ ...current, userId: event.target.value }))}
                  value={memberDraft.userId}
                >
                  <option value="">选择平台用户</option>
                  {candidates.map((candidate) => (
                    <option key={candidate.userId} value={candidate.userId}>
                      {candidate.userName || candidate.userEmail} · {candidate.userEmail}
                    </option>
                  ))}
                </select>
                <select
                  aria-label="成员角色"
                  className="field-input"
                  disabled={isSavingMember}
                  onChange={(event) => setMemberDraft((current) => ({ ...current, role: event.target.value as ProjectMemberRole }))}
                  value={memberDraft.role}
                >
                  <option value="OWNER">所有者</option>
                  <option value="EDITOR">编辑者</option>
                  <option value="VIEWER">查看者</option>
                </select>
                <Button disabled={isSavingMember || !memberDraft.userId} type="submit" variant="primary">
                  添加
                </Button>
              </form>

              <div className="mt-2">
                <input
                  aria-label="搜索可添加用户"
                  className="field-input"
                  onChange={(event) => setCandidateQuery(event.target.value)}
                  placeholder="搜索用户名称或邮箱"
                  value={candidateQuery}
                />
              </div>

              {memberError ? (
                <div className="mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">
                  {memberError}
                </div>
              ) : null}

              <div className="mt-3 grid gap-2">
                {members.map((member) => (
                  <article className="rounded-md border border-ink-200 bg-ink-50 p-3" key={member.id}>
                    <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-ink-900">{member.userName || member.userEmail || member.userId}</p>
                        <p className="truncate text-xs text-ink-500">{member.userEmail || member.userId}</p>
                      </div>
                      <div className="flex items-center gap-2">
                        <select
                          aria-label={`产品成员角色 ${member.userName || member.userEmail || member.userId}`}
                          className="field-input min-w-32 bg-white"
                          disabled={isSavingMember && actionUserId === member.userId}
                          onChange={(event) => onUpdateMember(activeProject.id, member.userId, { role: event.target.value as ProjectMemberRole })}
                          value={member.role}
                        >
                          <option value="OWNER">所有者</option>
                          <option value="EDITOR">编辑者</option>
                          <option value="VIEWER">查看者</option>
                        </select>
                        <button
                          aria-label={`移除成员 ${member.userName || member.userEmail || member.userId}`}
                          className="icon-button h-10 w-10 hover:border-red-200 hover:bg-red-50 hover:text-red-700"
                          disabled={isSavingMember && actionUserId === member.userId}
                          onClick={() => onRemoveMember(activeProject.id, member.userId)}
                          title="移除成员"
                          type="button"
                        >
                          <Trash2 className="h-4 w-4" />
                        </button>
                      </div>
                    </div>
                  </article>
                ))}
                {memberStatus !== 'loading' && members.length === 0 ? <p className="rounded-md bg-ink-50 px-3 py-6 text-center text-sm text-ink-400">暂无产品成员。</p> : null}
              </div>
            </section>
          ) : null}
          </div>
        </div>
      </Modal>

      <EditorDrawer
        isOpen={isOpen && isProjectEditorOpen}
        onClose={() => setProjectEditorOpen(false)}
        title={activeProject ? '编辑产品' : '新建产品'}
      >
        <form className="space-y-4" onSubmit={submitProject}>
          <div>
            <p className="text-sm font-semibold text-ink-900">{activeProject ? activeProject.name : '创建新的产品工作区'}</p>
            <p className="mt-1 text-xs leading-5 text-ink-500">产品工作区用于集中管理参考图、生成图、任务记录和提示词。</p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="产品名称">
              <input autoFocus className="field-input" onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))} required value={draft.name} />
            </Field>
            <Field label="品牌">
              <input className="field-input" onChange={(event) => setDraft((current) => ({ ...current, brand: event.target.value }))} value={draft.brand} />
            </Field>
            <Field label="ASIN">
              <input className="field-input" onChange={(event) => setDraft((current) => ({ ...current, asin: event.target.value }))} value={draft.asin} />
            </Field>
            <Field label="站点">
              <input className="field-input" onChange={(event) => setDraft((current) => ({ ...current, site: event.target.value }))} placeholder="例如 US、JP、DE" value={draft.site} />
            </Field>
            <Field label="产品状态">
              <select className="field-input" onChange={(event) => setDraft((current) => ({ ...current, status: event.target.value as ProjectStatus }))} value={draft.status}>
                <option value="ACTIVE">启用</option>
                <option value="ARCHIVED">归档</option>
              </select>
            </Field>
            <Field label="排序号">
              <input className="field-input" min={0} onChange={(event) => setDraft((current) => ({ ...current, sortOrder: event.target.value }))} type="number" value={draft.sortOrder} />
            </Field>
          </div>

          <Field label="备注">
            <textarea className="field-input min-h-28 resize-y" onChange={(event) => setDraft((current) => ({ ...current, notes: event.target.value }))} value={draft.notes} />
          </Field>

          {error ? (
            <div className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700" role="alert">
              {error}
            </div>
          ) : null}

          {activeProject && canDeleteProject ? (
            <section aria-label="危险操作" className="rounded-md border border-red-200 bg-red-50/60 p-3">
              <h3 className="text-sm font-semibold text-red-800">危险操作</h3>
              <p className="mt-1 text-xs leading-5 text-red-700">
                删除后，该产品及其素材、生成记录将不再显示，且无法在页面中撤销。
              </p>
              <Button
                className="mt-3"
                disabled={isCreatingProject || isDeletingProject || isUpdatingProject}
                icon={isDeletingProject ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
                onClick={() => onDeleteProject(activeProject)}
                variant="danger"
              >
                {isDeletingProject ? '正在删除...' : '删除产品'}
              </Button>
            </section>
          ) : null}

          <div className="sticky bottom-0 -mx-5 mt-6 flex flex-wrap gap-2 border-t border-ink-200 bg-white px-5 py-4">
            <Button disabled={isCreatingProject || isDeletingProject || isUpdatingProject || draft.name.trim().length === 0} icon={isCreatingProject || isUpdatingProject ? <Loader2 className="h-4 w-4 animate-spin" /> : undefined} type="submit" variant="primary">
              {activeProject ? '保存产品' : '创建产品'}
            </Button>
            <Button disabled={isCreatingProject || isDeletingProject || isUpdatingProject} onClick={() => setProjectEditorOpen(false)}>
              取消
            </Button>
          </div>
        </form>
      </EditorDrawer>
    </>
  )
}

function emptyDraft(): ProjectDraft {
  return {
    name: '',
    brand: '',
    asin: '',
    site: '',
    notes: '',
    status: 'ACTIVE',
    sortOrder: '0',
  }
}

function parseSortOrder(value: string): number | undefined {
  const parsed = Number(value)
  return Number.isInteger(parsed) && parsed >= 0 ? parsed : undefined
}

function projectStatusLabel(status: ProjectStatus): string {
  return status === 'ACTIVE' ? '启用' : '归档'
}

interface FieldProps {
  label: string
  children: ReactElement
}

function Field({ label, children }: FieldProps) {
  return (
    <label className="grid gap-1 text-sm text-ink-700">
      <span className="field-label">{label}</span>
      {children}
    </label>
  )
}
