import { Bell, CheckCircle2, Clock3, Eye, Loader2, RotateCcw, X, XCircle } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import type { GenerationNotification, ManagedGenerationTask } from '../../hooks/useGeneration'
import type { Project, ProjectId, TaskId, TaskStatus } from '../../types/platform'
import { normalizeWorkbenchImageType, WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../../types/workbench'
import { Modal } from '../ui/Modal'

interface TaskCenterButtonProps {
  activeTaskCount: number
  onClick: () => void
}

export function TaskCenterButton({ activeTaskCount, onClick }: TaskCenterButtonProps) {
  return (
    <button
      aria-label={`任务中心，${activeTaskCount} 个进行中任务`}
      className="relative inline-flex min-h-11 items-center gap-2 rounded-md border border-ink-200 bg-white px-3 py-2 text-sm font-semibold text-ink-700 transition hover:bg-ink-50"
      onClick={onClick}
      type="button"
    >
      <Bell className="h-4 w-4" />
      <span className="hidden sm:inline">任务中心</span>
      {activeTaskCount > 0 ? (
        <span className="inline-flex min-w-5 items-center justify-center rounded-full bg-amazon-500 px-1.5 py-0.5 text-xs font-bold text-ink-950">
          {activeTaskCount}
        </span>
      ) : null}
    </button>
  )
}

interface TaskCenterProps {
  history?: ReactNode
  isOpen: boolean
  tasks: ManagedGenerationTask[]
  projects: Project[]
  pendingTaskId?: TaskId | null
  onCancel: (taskId: TaskId) => void
  onClose: () => void
  onRetry: (taskId: TaskId) => void
  onView: (taskId: TaskId) => void
}

export function TaskCenter({
  history,
  isOpen,
  tasks,
  projects,
  pendingTaskId,
  onCancel,
  onClose,
  onRetry,
  onView,
}: TaskCenterProps) {
  const [section, setSection] = useState<'tasks' | 'history'>('tasks')

  return (
    <Modal isOpen={isOpen} maxWidthClass="max-w-2xl" onClose={onClose} title="任务中心">
      {history ? (
        <div aria-label="任务中心内容" className="mb-4 grid grid-cols-2 rounded-lg bg-ink-100 p-1" role="tablist">
          <button
            aria-selected={section === 'tasks'}
            className={`min-h-10 rounded-md px-3 text-sm font-semibold transition ${section === 'tasks' ? 'bg-white text-ink-900 shadow-sm' : 'text-ink-500 hover:text-ink-900'}`}
            onClick={() => setSection('tasks')}
            role="tab"
            type="button"
          >
            进行中任务
          </button>
          <button
            aria-selected={section === 'history'}
            className={`min-h-10 rounded-md px-3 text-sm font-semibold transition ${section === 'history' ? 'bg-white text-ink-900 shadow-sm' : 'text-ink-500 hover:text-ink-900'}`}
            onClick={() => setSection('history')}
            role="tab"
            type="button"
          >
            生成历史
          </button>
        </div>
      ) : null}

      {section === 'history' && history ? (
        <aside aria-label="图片生成历史" className="max-h-[65dvh] overflow-y-auto rounded-lg border border-ink-200">
          {history}
        </aside>
      ) : tasks.length === 0 ? (
        <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-5 py-12 text-center">
          <Clock3 className="mx-auto h-9 w-9 text-ink-300" />
          <p className="mt-3 text-sm font-semibold text-ink-700">还没有生成任务</p>
          <p className="mt-1 text-sm text-ink-500">提交任务后可以继续处理其他产品或图片类型。</p>
        </div>
      ) : (
        <div className="grid gap-3">
          {[...tasks].reverse().map((item) => {
            const status = item.state.status ?? item.task.status
            const productName = projects.find((project) => project.id === item.task.projectId)?.name ?? '未知产品'
            const imageType = getImageTypeLabel(item.task.imageType)
            return (
              <article className="rounded-lg border border-ink-200 bg-white p-4" key={item.task.id}>
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="truncate text-sm font-semibold text-ink-900">{productName}</h3>
                      <span className="rounded-full bg-ink-100 px-2 py-1 text-[11px] font-semibold text-ink-600">{imageType}</span>
                      <TaskStatusBadge status={status} />
                    </div>
                    <p className="mt-2 line-clamp-2 text-sm leading-6 text-ink-500">{item.task.prompt}</p>
                  </div>
                  {item.items[0]?.result.thumbnailUrl || item.items[0]?.result.previewUrl ? (
                    <img
                      alt={`${imageType}任务结果`}
                      className="h-16 w-16 shrink-0 rounded-md border border-ink-200 object-cover"
                      src={item.items[0].result.thumbnailUrl || item.items[0].result.previewUrl}
                    />
                  ) : null}
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  <button className="task-action" onClick={() => onView(item.task.id)} type="button">
                    <Eye className="h-4 w-4" />
                    查看工作区
                  </button>
                  {canCancel(status) ? (
                    <button
                      className="task-action"
                      disabled={pendingTaskId === item.task.id}
                      onClick={() => onCancel(item.task.id)}
                      type="button"
                    >
                      <XCircle className="h-4 w-4" />
                      取消
                    </button>
                  ) : null}
                  {canRetry(status) ? (
                    <button
                      className="task-action"
                      disabled={pendingTaskId === item.task.id}
                      onClick={() => onRetry(item.task.id)}
                      type="button"
                    >
                      <RotateCcw className="h-4 w-4" />
                      重试
                    </button>
                  ) : null}
                </div>
              </article>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

interface GenerationHistoryRailProps {
  history: ReactNode
  imageType: WorkbenchImageType
  projectId: ProjectId
  tasks: ManagedGenerationTask[]
  projects: Project[]
  pendingTaskId?: TaskId | null
  onCancel: (taskId: TaskId) => void
  onView: (taskId: TaskId) => void
}

export function GenerationHistoryRail({
  history,
  imageType,
  projectId,
  tasks,
  projects,
  pendingTaskId,
  onCancel,
  onView,
}: GenerationHistoryRailProps) {
  const activeTasks = [...tasks]
    .filter(
      (item) =>
        item.task.projectId === projectId &&
        normalizeWorkbenchImageType(item.task.imageType) === imageType &&
        canCancel(item.state.status ?? item.task.status),
    )
    .reverse()
  const imageTypeLabel = getImageTypeLabel(imageType)

  return (
    <aside
      aria-label="图片生成历史"
      className="panel flex min-h-0 flex-col overflow-hidden lg:h-full lg:self-stretch"
      data-desktop-position="right"
    >
      <div className="flex items-center justify-between border-b border-ink-200 px-4 py-3">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h2 className="text-sm font-semibold text-ink-900">生成历史</h2>
            <span className="truncate rounded-full bg-ink-100 px-2 py-1 text-[11px] font-semibold text-ink-600">{imageTypeLabel}</span>
          </div>
          <p className="mt-0.5 text-xs text-ink-400">当前产品 · 进行中与已完成</p>
        </div>
        {activeTasks.length > 0 ? (
          <span className="rounded-full bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{activeTasks.length} 个进行中</span>
        ) : null}
      </div>

      <section aria-labelledby="active-task-heading" className="border-b border-ink-200 px-3 py-3">
        <h3 className="px-1 text-xs font-semibold text-ink-500" id="active-task-heading">正在生成</h3>
        {activeTasks.length === 0 ? (
          <p className="mt-2 rounded-md bg-ink-50 px-3 py-3 text-xs leading-5 text-ink-400">暂无进行中的任务</p>
        ) : (
          <div className="mt-2 grid gap-2">
            {activeTasks.map((item) => {
              const status = item.state.status ?? item.task.status
              const productName = projects.find((project) => project.id === item.task.projectId)?.name ?? '未知产品'
              return (
                <article className="rounded-md border border-blue-100 bg-blue-50/50 p-3" key={item.task.id}>
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-xs font-semibold text-ink-800">{productName} · {getImageTypeLabel(item.task.imageType)}</span>
                    <TaskStatusBadge status={status} />
                  </div>
                  <p className="mt-2 line-clamp-2 text-xs leading-5 text-ink-500">{item.task.prompt}</p>
                  <div className="mt-2 flex gap-2">
                    <button className="task-action min-h-9 px-2 text-xs" onClick={() => onView(item.task.id)} type="button">
                      <Eye className="h-3.5 w-3.5" />
                      查看
                    </button>
                    <button
                      className="task-action min-h-9 px-2 text-xs"
                      disabled={pendingTaskId === item.task.id}
                      onClick={() => onCancel(item.task.id)}
                      type="button"
                    >
                      <XCircle className="h-3.5 w-3.5" />
                      取消
                    </button>
                  </div>
                </article>
              )
            })}
          </div>
        )}
      </section>

      <div className="min-h-0 flex-1 overflow-y-auto">{history}</div>
    </aside>
  )
}

interface TaskNotificationsProps {
  notifications: GenerationNotification[]
  projects: Project[]
  onDismiss: (notificationId: string) => void
  onView: (taskId: TaskId, notificationId: string) => void
}

export function TaskNotifications({ notifications, projects, onDismiss, onView }: TaskNotificationsProps) {
  if (notifications.length === 0) {
    return null
  }

  return (
    <div aria-label="任务提醒" className="fixed bottom-4 right-3 z-40 grid w-[min(380px,calc(100vw-24px))] gap-2 sm:right-4">
      {notifications.slice(-3).map((notification) => {
        const imageTypeLabel = getImageTypeLabel(notification.imageType)
        const productName = projects.find((project) => project.id === notification.projectId)?.name ?? '产品'
        const isSuccess = notification.status === 'SUCCEEDED'
        const failureMessage = notificationFailureMessage(notification.errorCode)
        return (
          <section
            aria-live="polite"
            className={`rounded-lg border bg-white p-4 shadow-2xl ${isSuccess ? 'border-emerald-200' : 'border-amber-200'}`}
            key={notification.id}
            role="status"
          >
            <div className="flex items-start gap-3">
              {isSuccess ? (
                <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" />
              ) : (
                <XCircle className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" />
              )}
              <div className="min-w-0 flex-1">
                <h2 className="text-sm font-semibold text-ink-900">
                  {isSuccess ? `${imageTypeLabel}生成完成` : `${imageTypeLabel}任务未完成`}
                </h2>
                <p className="mt-1 text-xs leading-5 text-ink-500">{failureMessage ? `${productName} · ${failureMessage}` : productName}</p>
                <button
                  aria-label={`查看${imageTypeLabel}结果`}
                  className="mt-3 text-sm font-semibold text-amazon-700 hover:text-amazon-800"
                  onClick={() => onView(notification.taskId, notification.id)}
                  type="button"
                >
                  查看结果
                </button>
              </div>
              <button
                aria-label={`关闭${imageTypeLabel}任务提醒`}
                className="icon-button h-9 w-9 shrink-0"
                onClick={() => onDismiss(notification.id)}
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          </section>
        )
      })}
    </div>
  )
}

function notificationFailureMessage(errorCode?: string): string {
  if (errorCode === 'PROVIDER_INSUFFICIENT_QUOTA') {
    return '中转站余额不足，请充值后再生成。'
  }
  return ''
}

function TaskStatusBadge({ status }: { status: TaskStatus }) {
  const active = status === 'QUEUED' || status === 'RUNNING' || status === 'RETRYING'
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-1 text-[11px] font-semibold ${
        status === 'SUCCEEDED'
          ? 'bg-emerald-50 text-emerald-700'
          : active
            ? 'bg-blue-50 text-blue-700'
            : 'bg-amber-50 text-amber-800'
      }`}
    >
      {active ? <Loader2 className="h-3 w-3 animate-spin" /> : status === 'SUCCEEDED' ? <CheckCircle2 className="h-3 w-3" /> : null}
      {taskStatusLabel(status)}
    </span>
  )
}

function getImageTypeLabel(value: string): string {
  const normalized = normalizeWorkbenchImageType(value)
  return WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === normalized)?.label ?? '主图'
}

function taskStatusLabel(status: TaskStatus): string {
  return {
    QUEUED: '排队中',
    RUNNING: '生成中',
    SUCCEEDED: '已完成',
    FAILED: '失败',
    CANCELLED: '已取消',
    RETRYING: '重试中',
    TIMED_OUT: '已超时',
  }[status]
}

function canCancel(status: TaskStatus): boolean {
  return status === 'QUEUED' || status === 'RUNNING' || status === 'RETRYING'
}

function canRetry(status: TaskStatus): boolean {
  return status === 'FAILED' || status === 'CANCELLED' || status === 'TIMED_OUT'
}
