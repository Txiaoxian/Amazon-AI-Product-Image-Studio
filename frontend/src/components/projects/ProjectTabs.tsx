import { Plus, Settings } from 'lucide-react'
import type { Project, ProjectId } from '../../types/platform'

interface ProjectTabsProps {
  projects: Project[]
  selectedProjectId: ProjectId | null
  onManageProject: (project?: Project) => void
  onSelectProject: (projectId: ProjectId) => void
}

export function ProjectTabs({
  projects,
  selectedProjectId,
  onManageProject,
  onSelectProject,
}: ProjectTabsProps) {
  const selectedProduct = projects.find((product) => product.id === selectedProjectId)

  return (
    <section aria-label="产品切换" className="panel sticky top-16 z-20 min-w-0 p-1.5">
      <label className="sr-only" htmlFor="product-select">
        当前产品
      </label>
      <select
        className="sr-only"
        id="product-select"
        onChange={(event) => {
          if (event.target.value) {
            onSelectProject(event.target.value as ProjectId)
          }
        }}
        value={selectedProjectId ?? ''}
        tabIndex={-1}
      >
        {selectedProjectId ? null : <option value="">请选择产品</option>}
        {projects.map((product) => (
          <option key={product.id} value={product.id}>
            {product.name}{product.brand ? ` · ${product.brand}` : ''}
          </option>
        ))}
      </select>

      <div className="flex min-w-0 items-center gap-1.5">
        <div
          aria-label="产品列表"
          className="flex min-w-0 flex-1 flex-wrap gap-2 overflow-visible"
          data-fill-axis="horizontal"
          data-layout="wrapped"
          data-scroll-behavior="fixed"
          role="tablist"
        >
          {projects.map((product) => {
            const selected = product.id === selectedProjectId
            const meta = [product.brand, product.asin].filter(Boolean).join(' · ')
            return (
              <button
                aria-selected={selected}
                className={`group min-h-11 min-w-40 max-w-72 flex-none rounded-md border px-3 py-1.5 text-left transition focus:outline-none focus:ring-2 focus:ring-ink-300 ${
                  selected
                    ? 'border-ink-600 bg-white text-ink-900 shadow-sm ring-1 ring-ink-600'
                    : 'border-ink-200 bg-ink-50 text-ink-600 hover:border-ink-400 hover:bg-white hover:text-ink-900'
                }`}
                key={product.id}
                onClick={() => onSelectProject(product.id)}
                role="tab"
                type="button"
              >
                <span className="flex min-w-0 items-center">
                  <span className="min-w-0 flex-1 truncate text-sm font-semibold">{product.name}</span>
                  {selected ? (
                    <span className="ml-2 shrink-0 rounded-sm bg-ink-800 px-1.5 py-0.5 text-[10px] font-semibold leading-none text-white">
                      当前
                    </span>
                  ) : null}
                </span>
                {meta ? (
                  <span className={`block truncate text-[11px] ${selected ? 'text-ink-600' : 'text-ink-400'}`}>{meta}</span>
                ) : null}
              </button>
            )
          })}
          <button
            aria-label="新增产品"
            className="inline-flex min-h-11 min-w-11 shrink-0 items-center justify-center rounded-md border border-dashed border-ink-300 text-ink-500 transition hover:border-amazon-500 hover:bg-amazon-500/10 hover:text-ink-900 focus:outline-none focus:ring-2 focus:ring-amazon-500/30"
            onClick={() => onManageProject()}
            type="button"
          >
            <Plus className="h-5 w-5" />
          </button>
        </div>

        <button
          aria-label="产品设置"
          className="icon-button h-11 w-11 shrink-0"
          disabled={!selectedProduct}
          onClick={() => onManageProject(selectedProduct)}
          title="产品设置"
          type="button"
        >
          <Settings className="h-4 w-4" />
        </button>
      </div>
    </section>
  )
}
