import { Settings, X } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { APP_NAME } from '../../lib/constants'

interface AppShellProps {
  children: ReactNode
  accountSlot?: ReactNode
  notice?: string
  onOpenSettings?: () => void
}

export function AppShell({ accountSlot, children, notice, onOpenSettings }: AppShellProps) {
  const [dismissedNotice, setDismissedNotice] = useState<string>()
  const visibleNotice = notice && notice !== dismissedNotice ? notice : undefined

  return (
    <div className="min-h-screen bg-[#f4f7fb] text-ink-900">
      <a
        className="fixed left-3 top-3 z-[60] -translate-y-20 rounded-md bg-ink-900 px-3 py-2 text-sm font-semibold text-white transition focus:translate-y-0"
        href="#main-content"
      >
        跳到主要内容
      </a>
      <header className="sticky top-0 z-30 border-b border-ink-200 bg-white/95 backdrop-blur">
        <div className="mx-auto flex min-h-16 w-full max-w-[1920px] items-center justify-between gap-2 px-3 py-2 sm:gap-3 sm:px-4">
          <div className="min-w-0 flex-1">
            <h1 className="truncate text-base font-semibold leading-tight text-ink-900 sm:text-lg">{APP_NAME}</h1>
            <p className="mt-0.5 hidden truncate text-xs text-ink-500 lg:block">为亚马逊卖家生成、编辑和管理 AI 产品图片</p>
          </div>
          <div className="flex min-w-0 shrink-0 items-center gap-1.5 sm:gap-2">
            {accountSlot}
            {onOpenSettings ? (
              <button aria-label="打开设置" className="icon-button shrink-0" onClick={onOpenSettings} title="设置" type="button">
                <Settings className="h-4 w-4" />
              </button>
            ) : null}
          </div>
        </div>
      </header>
      {visibleNotice ? (
        <div
          aria-live="polite"
          className="toast-notice fixed right-3 top-20 z-40 flex w-[min(400px,calc(100vw-24px))] items-start gap-3 rounded-lg border border-amazon-500/30 bg-white px-4 py-3 text-sm leading-6 text-ink-800 shadow-xl sm:right-4"
          data-overlay-notice="true"
          onAnimationEnd={() => setDismissedNotice(visibleNotice)}
          role="status"
        >
          <span className="min-w-0 flex-1">{visibleNotice}</span>
          <button
            aria-label="关闭提示"
            className="icon-button h-8 w-8 shrink-0"
            onClick={() => setDismissedNotice(visibleNotice)}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ) : null}
      <main className="mx-auto w-full max-w-[1920px] px-2 py-2 sm:px-3 sm:py-3 lg:px-4" id="main-content">
        {children}
      </main>
    </div>
  )
}
