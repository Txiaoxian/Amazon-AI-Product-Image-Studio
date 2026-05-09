import { Settings } from 'lucide-react'
import type { ReactNode } from 'react'
import { APP_NAME } from '../../lib/constants'

interface AppShellProps {
  children: ReactNode
  notice?: string
  onOpenSettings: () => void
}

export function AppShell({ children, notice, onOpenSettings }: AppShellProps) {
  return (
    <div className="min-h-screen bg-[#f4f7fb] text-ink-900">
      <header className="sticky top-0 z-30 border-b border-ink-200 bg-white/95 backdrop-blur">
        <div className="mx-auto flex max-w-[1600px] items-center justify-between gap-3 px-3 py-3 sm:px-4 lg:px-6">
          <div className="min-w-0">
            <h1 className="text-base font-semibold leading-tight text-ink-900 sm:text-lg">{APP_NAME}</h1>
            <p className="mt-0.5 text-xs text-ink-500">为亚马逊卖家生成、编辑和管理 AI 产品图片</p>
          </div>
          <button aria-label="打开设置" className="icon-button shrink-0" onClick={onOpenSettings} title="设置" type="button">
            <Settings className="h-4 w-4" />
          </button>
        </div>
        {notice ? (
          <div className="border-t border-amazon-500/20 bg-amazon-500/10 px-3 py-2 text-sm leading-6 text-ink-800 sm:px-4">
            {notice}
          </div>
        ) : null}
      </header>
      <main className="mx-auto max-w-[1600px] px-3 py-3 sm:px-4 sm:py-4 lg:px-6">{children}</main>
    </div>
  )
}
