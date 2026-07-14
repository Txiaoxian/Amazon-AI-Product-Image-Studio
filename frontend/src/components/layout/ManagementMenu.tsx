import { ChevronDown, SlidersHorizontal, type LucideIcon } from 'lucide-react'
import { useRef } from 'react'

export interface ManagementMenuItem {
  description: string
  icon: LucideIcon
  label: string
  onSelect: () => void
}

interface ManagementMenuProps {
  items: ManagementMenuItem[]
}

export function ManagementMenu({ items }: ManagementMenuProps) {
  const detailsRef = useRef<HTMLDetailsElement>(null)
  const summaryRef = useRef<HTMLElement>(null)

  if (items.length === 0) {
    return null
  }

  const closeMenu = () => {
    detailsRef.current?.removeAttribute('open')
  }

  return (
    <details
      className="group relative"
      onKeyDown={(event) => {
        if (event.key !== 'Escape') {
          return
        }
        event.preventDefault()
        closeMenu()
        summaryRef.current?.focus()
      }}
      ref={detailsRef}
    >
      <summary
        aria-label="打开管理菜单"
        className="flex min-h-11 cursor-pointer list-none items-center justify-center gap-2 rounded-md border border-ink-200 bg-white px-3 text-sm font-semibold text-ink-700 transition hover:bg-ink-50 hover:text-ink-900 focus:outline-none focus:ring-2 focus:ring-amazon-500/30 [&::-webkit-details-marker]:hidden"
        ref={summaryRef}
        role="button"
      >
        <SlidersHorizontal className="h-4 w-4" />
        <span className="hidden sm:inline">管理</span>
        <ChevronDown className="hidden h-4 w-4 text-ink-400 transition group-open:rotate-180 sm:block" />
      </summary>

      <div className="fixed left-3 right-3 top-[4.5rem] z-50 rounded-lg border border-ink-200 bg-white p-2 shadow-2xl sm:absolute sm:left-auto sm:right-0 sm:top-[calc(100%+0.5rem)] sm:w-[min(20rem,calc(100vw-1.5rem))]">
        <p className="px-3 pb-2 pt-1 text-xs font-semibold text-ink-500">平台管理</p>
        <div className="grid gap-1" role="menu">
          {items.map((item) => {
            const Icon = item.icon
            return (
              <button
                aria-label={item.label}
                className="flex min-h-11 w-full items-start gap-3 rounded-md px-3 py-2 text-left transition hover:bg-ink-50 focus:outline-none focus:ring-2 focus:ring-amazon-500/30"
                key={item.label}
                onClick={() => {
                  closeMenu()
                  item.onSelect()
                }}
                role="menuitem"
                type="button"
              >
                <Icon className="mt-0.5 h-4 w-4 shrink-0 text-ink-500" />
                <span className="min-w-0">
                  <span className="block text-sm font-semibold text-ink-900">{item.label}</span>
                  <span className="mt-0.5 block text-xs leading-5 text-ink-500">{item.description}</span>
                </span>
              </button>
            )
          })}
        </div>
      </div>
    </details>
  )
}
