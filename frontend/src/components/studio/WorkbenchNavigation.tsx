import { AlertCircle, ArrowRight, Check, CheckCircle2, FolderPlus, ImageIcon, Loader2, Settings2, Sparkles } from 'lucide-react'
import type { ReactNode } from 'react'
import { WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../../types/workbench'
import { Button } from '../ui/Button'

interface ImageTypeTabsProps {
  activityByType?: Partial<Record<WorkbenchImageType, ImageTypeActivity>>
  imageType: WorkbenchImageType
  isDisabled?: boolean
  onChange: (imageType: WorkbenchImageType) => void
}

export interface ImageTypeActivity {
  activeCount: number
  completedCount: number
}

export function ImageTypeTabs({ activityByType = {}, imageType, onChange }: ImageTypeTabsProps) {
  return (
    <nav
      aria-label="商品图片类型选项"
      className="panel min-w-0 p-1.5 lg:max-h-full lg:self-start lg:overflow-y-auto"
      data-desktop-position="left"
      data-fill-axis="vertical"
      data-layout="content-sized"
    >
      <p className="px-2 pb-1.5 pt-1 text-xs font-semibold text-ink-400 lg:text-center">图片类型</p>
      <div
        aria-label="选择图片类型"
        className="flex min-w-0 gap-1 overflow-x-auto lg:flex-col lg:gap-1.5 lg:overflow-visible"
        role="tablist"
      >
        {WORKBENCH_IMAGE_TYPE_OPTIONS.map((option) => {
          const selected = option.value === imageType
          const activity = activityByType[option.value]
          const activeCount = activity?.activeCount ?? 0
          const completedCount = activity?.completedCount ?? 0
          const hasActivity = activeCount > 0 || completedCount > 0
          const statusDescriptionId = `image-type-status-${option.value.toLowerCase()}`
          const statusDescription = [
            activeCount > 0 ? `正在生成 ${activeCount} 个任务` : '',
            completedCount > 0 ? `已完成 ${completedCount} 个任务` : '',
          ]
            .filter(Boolean)
            .join('，')
          return (
            <button
              aria-describedby={hasActivity ? statusDescriptionId : undefined}
              aria-label={option.label}
              aria-selected={selected}
              className={`flex min-h-11 shrink-0 flex-col items-center justify-center rounded-md border px-3 py-2 text-sm font-semibold transition focus:outline-none focus:ring-2 focus:ring-ink-300 lg:w-full lg:px-2 ${
                selected
                  ? 'border-ink-300 bg-ink-100 text-ink-900 shadow-sm'
                  : 'border-transparent text-ink-600 hover:border-ink-200 hover:bg-ink-50 hover:text-ink-900'
              }`}
              key={option.value}
              onClick={() => onChange(option.value)}
              role="tab"
              type="button"
            >
              <span>{option.label}</span>
              {hasActivity ? (
                <span aria-hidden="true" className="mt-1 flex flex-wrap items-center justify-center gap-x-2 gap-y-0.5 text-[10px] font-medium leading-4">
                  {activeCount > 0 ? (
                    <span className="inline-flex items-center gap-1 text-amber-700" title={`${activeCount} 个任务正在生成`}>
                      <Loader2 className="h-3 w-3 animate-spin" />
                      生成中 {activeCount}
                    </span>
                  ) : null}
                  {completedCount > 0 ? (
                    <span className="inline-flex items-center gap-1 text-emerald-700" title={`${completedCount} 个任务已完成`}>
                      <CheckCircle2 className="h-3 w-3" />
                      已完成 {completedCount}
                    </span>
                  ) : null}
                </span>
              ) : null}
              {hasActivity ? (
                <span className="sr-only" id={statusDescriptionId}>
                  {statusDescription}
                </span>
              ) : null}
            </button>
          )
        })}
      </div>
    </nav>
  )
}

export type WorkspaceSection = 'assets' | 'history'

interface WorkspaceSectionTabsProps {
  activeSection: WorkspaceSection
  assets: ReactNode
  history: ReactNode
  onChange: (section: WorkspaceSection) => void
}

export function WorkspaceSectionTabs({ activeSection, assets, history, onChange }: WorkspaceSectionTabsProps) {
  const isAssets = activeSection === 'assets'

  return (
    <section aria-label="产品素材与生成记录" className="flex min-h-0 flex-col gap-2">
      <div aria-label="产品内容" className="panel grid grid-cols-2 gap-1 p-1" role="tablist">
        <WorkspaceTab isActive={isAssets} label="素材" onClick={() => onChange('assets')} />
        <WorkspaceTab isActive={!isAssets} label="历史" onClick={() => onChange('history')} />
      </div>
      <div
        aria-label={isAssets ? '素材' : '历史'}
        className="min-h-0 flex-1"
        role="tabpanel"
      >
        {isAssets ? assets : history}
      </div>
    </section>
  )
}

function WorkspaceTab({ isActive, label, onClick }: { isActive: boolean; label: string; onClick: () => void }) {
  return (
    <button
      aria-selected={isActive}
      className={`min-h-11 rounded-md px-3 text-sm font-semibold transition focus:outline-none focus:ring-2 focus:ring-amazon-500/30 ${
        isActive ? 'bg-ink-900 text-white' : 'text-ink-600 hover:bg-ink-50 hover:text-ink-900'
      }`}
      onClick={onClick}
      role="tab"
      type="button"
    >
      {label}
    </button>
  )
}

interface WorkspaceOnboardingProps {
  canCreateProject: boolean
  hasLoadError: boolean
  onCreateProject: () => void
  onRetry: () => void
}

export function WorkspaceOnboarding({ canCreateProject, hasLoadError, onCreateProject, onRetry }: WorkspaceOnboardingProps) {
  if (hasLoadError) {
    return (
      <section className="panel mx-auto w-full max-w-3xl px-5 py-12 text-center" role="alert">
        <AlertCircle className="mx-auto h-10 w-10 text-red-600" />
        <h2 className="mt-4 text-xl font-semibold text-ink-900">产品列表加载失败</h2>
        <p className="mx-auto mt-2 max-w-xl text-sm leading-6 text-ink-600">暂时无法读取产品，请检查服务状态后重新加载。</p>
        <Button className="mt-6" onClick={onRetry} variant="secondary">
          重新加载产品
        </Button>
      </section>
    )
  }

  return (
    <section className="panel overflow-hidden">
      <div className="grid gap-8 px-5 py-10 sm:px-8 lg:grid-cols-[minmax(0,1fr)_minmax(320px,0.8fr)] lg:items-center lg:px-12 lg:py-14">
        <div>
          <span className="inline-flex items-center gap-2 rounded-full bg-amazon-500/15 px-3 py-1 text-xs font-semibold text-ink-800">
            <Sparkles className="h-3.5 w-3.5" />
            开始第一组商品图
          </span>
          <h2 className="mt-4 text-2xl font-semibold tracking-tight text-ink-900 sm:text-3xl">从创建第一个产品开始</h2>
          <p className="mt-3 max-w-2xl text-sm leading-7 text-ink-600 sm:text-base">
            一个产品工作区集中管理参考图、不同类型的生成结果、任务记录和提示词。
          </p>

          {canCreateProject ? (
            <Button className="mt-6" icon={<FolderPlus className="h-4 w-4" />} onClick={onCreateProject} variant="primary">
              创建第一个产品
              <ArrowRight className="h-4 w-4" />
            </Button>
          ) : (
            <p className="mt-6 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
              当前账号没有创建产品权限，请联系管理员添加产品或调整权限。
            </p>
          )}
        </div>

        <ol className="grid gap-3" aria-label="生图准备步骤">
          <OnboardingStep icon={FolderPlus} label="创建产品" number="01" />
          <OnboardingStep icon={Settings2} label="选择可用模型" number="02" />
          <OnboardingStep icon={ImageIcon} label="上传参考图并填写提示词" number="03" />
          <OnboardingStep icon={Check} label="生成并管理商品图片" number="04" />
        </ol>
      </div>
    </section>
  )
}

function OnboardingStep({ icon: Icon, label, number }: { icon: typeof FolderPlus; label: string; number: string }) {
  return (
    <li className="flex items-center gap-3 rounded-lg border border-ink-200 bg-ink-50 px-4 py-3">
      <span className="text-xs font-semibold text-ink-500">{number}</span>
      <span className="flex h-9 w-9 items-center justify-center rounded-md bg-white text-ink-700 shadow-sm">
        <Icon className="h-4 w-4" />
      </span>
      <span className="text-sm font-semibold text-ink-800">{label}</span>
    </li>
  )
}

export function ModelSetupBanner({ canManageModels, onOpen }: { canManageModels: boolean; onOpen: () => void }) {
  return (
    <section className="flex flex-col gap-3 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
      <div className="min-w-0">
        <h2 className="text-sm font-semibold text-amber-950">还差一步：配置可用模型</h2>
        <p className="mt-1 text-sm leading-6 text-amber-900">
          当前没有可用的生图模型，配置完成后即可填写提示词并生成图片。
        </p>
      </div>
      {canManageModels ? (
        <Button className="shrink-0" onClick={onOpen} variant="secondary">
          配置中转站与模型
        </Button>
      ) : null}
    </section>
  )
}
