import { LogOut, UserRound } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import type { AuthSession } from '../../types/auth'
import { Button } from '../ui/Button'

interface AuthStatusProps {
  isSubmitting: boolean
  session: AuthSession
  onLogout: () => Promise<void>
  variant?: 'default' | 'compact'
}

export function AuthStatus({ isSubmitting, session, onLogout, variant = 'default' }: AuthStatusProps) {
  const [isMenuOpen, setMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (!isMenuOpen) return
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      setMenuOpen(false)
      triggerRef.current?.focus()
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (menuRef.current?.contains(event.target as Node) || triggerRef.current?.contains(event.target as Node)) return
      setMenuOpen(false)
    }
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('mousedown', handlePointerDown)
    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('mousedown', handlePointerDown)
    }
  }, [isMenuOpen])

  if (variant === 'compact') {
    return (
      <div className="relative">
        <button
          aria-expanded={isMenuOpen}
          aria-haspopup="menu"
          aria-label="打开账户菜单"
          className="flex h-11 w-11 cursor-pointer list-none items-center justify-center rounded-lg border border-white/10 bg-white/5 text-slate-300 transition hover:bg-white/10 hover:text-white focus:outline-none focus:ring-2 focus:ring-amazon-500/40 [&::-webkit-details-marker]:hidden"
          onClick={() => setMenuOpen((open) => !open)}
          ref={triggerRef}
          type="button"
        >
          <UserRound className="h-5 w-5" />
        </button>
        {isMenuOpen ? <div className="fixed bottom-20 left-3 z-50 w-[min(18rem,calc(100vw-1.5rem))] rounded-xl border border-slate-200 bg-white p-3 text-left shadow-2xl lg:absolute lg:bottom-0 lg:left-[calc(100%+0.75rem)]" ref={menuRef} role="menu">
          <div className="rounded-lg bg-slate-50 px-3 py-2">
            <p className="truncate text-sm font-semibold text-slate-900">{session.user.displayName}</p>
            <p className="mt-0.5 truncate text-xs text-slate-500">{session.tenant.name}</p>
          </div>
          <Button
            className="mt-2 w-full"
            disabled={isSubmitting}
            icon={<LogOut className="h-4 w-4" />}
            onClick={() => void onLogout()}
            role="menuitem"
            variant="secondary"
          >
            退出登录
          </Button>
        </div> : null}
      </div>
    )
  }

  return (
    <div className="flex min-w-0 items-center gap-1.5 sm:gap-2">
      <div className="hidden min-w-[96px] max-w-[180px] items-center gap-2 rounded-md border border-ink-200 bg-ink-50 px-3 py-2 md:flex xl:max-w-[220px]">
        <UserRound className="h-4 w-4 shrink-0 text-ink-500" />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold leading-5 text-ink-900">{session.user.displayName}</p>
          <p className="truncate text-xs leading-5 text-ink-500">{session.tenant.name}</p>
        </div>
      </div>
      <Button
        aria-label="退出"
        className="h-11 w-11 px-0 sm:w-auto sm:px-3"
        disabled={isSubmitting}
        icon={<LogOut className="h-4 w-4" />}
        onClick={() => void onLogout()}
        variant="secondary"
      >
        <span className="hidden sm:inline">退出</span>
      </Button>
    </div>
  )
}
