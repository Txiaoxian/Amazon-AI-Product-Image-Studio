import {
  PanelLeftClose,
  PanelLeftOpen,
  Plus,
  Settings,
  Sparkles,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import type { Project, ProjectId } from '../../types/platform'
import {
  WORKBENCH_IMAGE_TYPE_OPTIONS,
  type WorkbenchImageType,
} from '../../types/workbench'
import { TaskCenterButton } from '../tasks/TaskCenter'
import { ImageTypeTabs, type ImageTypeActivity } from './WorkbenchNavigation'

interface CanvasStudioLayoutProps {
  activeTaskCount: number
  activityByType: Partial<Record<WorkbenchImageType, ImageTypeActivity>>
  completedImageTypeCount: number
  controlPanel: ReactNode
  imageType: WorkbenchImageType
  isGenerating: boolean
  onImageTypeChange: (imageType: WorkbenchImageType) => void
  onManageProject: (project?: Project) => void
  onOpenTaskCenter: () => void
  onSaveDraft: () => void
  onSelectProject: (projectId: ProjectId) => void
  productPreviewUrl?: string
  projects: Project[]
  resultCanvas: ReactNode
  selectedProject: Project
}

export function CanvasStudioLayout({
  activeTaskCount,
  activityByType,
  completedImageTypeCount,
  controlPanel,
  imageType,
  isGenerating,
  onImageTypeChange,
  onManageProject,
  onOpenTaskCenter,
  onSaveDraft,
  onSelectProject,
  productPreviewUrl = '/studio-assets/demo-bottle-candidate-studio.jpg',
  projects,
  resultCanvas,
  selectedProject,
}: CanvasStudioLayoutProps) {
  const [isControlCollapsed, setControlCollapsed] = useState(false)
  const imageTypeLabel = WORKBENCH_IMAGE_TYPE_OPTIONS.find((option) => option.value === imageType)?.label ?? '图片'
  const completion = Math.min(completedImageTypeCount, WORKBENCH_IMAGE_TYPE_OPTIONS.length)
  const completionPercent = Math.round((completion / WORKBENCH_IMAGE_TYPE_OPTIONS.length) * 100)

  return (
    <div className="canvas-studio-view" data-testid="fixed-product-workspace">
      <header className="canvas-product-header">
        <div className="canvas-product-context">
          <img
            alt={`${selectedProject.name}产品图`}
            className="canvas-product-thumbnail"
            src={productPreviewUrl}
          />
          <div className="min-w-0">
            <label className="sr-only" htmlFor="canvas-product-select">当前产品</label>
            <div className="flex min-w-0 items-center gap-1.5">
              <select
                className="canvas-product-select"
                id="canvas-product-select"
                onChange={(event) => onSelectProject(event.target.value as ProjectId)}
                value={selectedProject.id}
              >
                {projects.map((project) => (
                  <option key={project.id} value={project.id}>{project.name}</option>
                ))}
              </select>
              <button
                aria-label="新增产品"
                className="canvas-product-add"
                onClick={() => onManageProject()}
                title="新增产品"
                type="button"
              >
                <Plus className="h-3.5 w-3.5" />
              </button>
            </div>
            <p className="canvas-product-meta">
              {selectedProject.asin ? `ASIN：${selectedProject.asin}` : '未填写 ASIN'}
              <span aria-hidden="true">·</span>
              {selectedProject.site || '未设置站点'}
            </p>
          </div>
        </div>

        <div className="canvas-header-actions">
          <div className="canvas-listing-progress" aria-label={`Listing 完成度 ${completion}/${WORKBENCH_IMAGE_TYPE_OPTIONS.length}`}>
            <span>Listing 完成度</span>
            <strong>{completion}/{WORKBENCH_IMAGE_TYPE_OPTIONS.length}</strong>
            <div aria-hidden="true" className="canvas-listing-progress-track">
              <span style={{ width: `${completionPercent}%` }} />
            </div>
          </div>
          <TaskCenterButton activeTaskCount={activeTaskCount} onClick={onOpenTaskCenter} />
          <button
            aria-label="产品设置"
            className="canvas-header-icon-button"
            onClick={() => onManageProject(selectedProject)}
            title="产品设置"
            type="button"
          >
            <Settings className="h-4 w-4" />
          </button>
        </div>
      </header>

      <ImageTypeTabs
        activityByType={activityByType}
        imageType={imageType}
        onChange={onImageTypeChange}
        variant="horizontal"
      />

      <div className={`canvas-workspace ${isControlCollapsed ? 'is-control-collapsed' : ''}`} data-testid="generation-workbench">
        <aside className="canvas-control-column" aria-label="创作参数">
          <div className="canvas-control-column-header">
            <div className="min-w-0">
              <p className="canvas-control-column-title">创作参数</p>
              <p className="canvas-control-column-summary">{imageTypeLabel} · 参数已自动保留</p>
            </div>
            <button
              aria-label={isControlCollapsed ? '展开创作参数' : '收起创作参数'}
              className="canvas-control-collapse"
              onClick={() => setControlCollapsed((value) => !value)}
              title={isControlCollapsed ? '展开创作参数' : '收起创作参数'}
              type="button"
            >
              {isControlCollapsed ? <PanelLeftOpen className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}
            </button>
          </div>
          {!isControlCollapsed ? <div className="canvas-control-slot">{controlPanel}</div> : null}
          <div className="canvas-control-actions">
            <button
              aria-label="生成图片"
              className="canvas-command-primary"
              disabled={isGenerating}
              form="canvas-generation-form"
              title={isControlCollapsed ? `生成${imageTypeLabel}` : undefined}
              type="submit"
            >
              <Sparkles className="h-4 w-4 shrink-0" />
              <span>{isGenerating ? '正在提交...' : isControlCollapsed ? '生成' : `生成图片 · ${imageTypeLabel}`}</span>
            </button>
            <button className="canvas-command-secondary" onClick={onSaveDraft} type="button">
              保存草稿
            </button>
          </div>
        </aside>
        <div className="canvas-result-slot">{resultCanvas}</div>
      </div>
    </div>
  )
}
