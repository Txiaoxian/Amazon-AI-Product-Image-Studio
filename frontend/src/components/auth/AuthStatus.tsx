import { LogOut, UserRound } from 'lucide-react'
import type { AuthSession } from '../../types/auth'
import { Button } from '../ui/Button'

interface AuthStatusProps {
  isSubmitting: boolean
  session: AuthSession
  onLogout: () => Promise<void>
}

export function AuthStatus({ isSubmitting, session, onLogout }: AuthStatusProps) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <div className="flex min-w-[96px] max-w-[150px] items-center gap-2 rounded-md border border-ink-200 bg-ink-50 px-3 py-2 sm:max-w-[220px]">
        <UserRound className="h-4 w-4 shrink-0 text-ink-500" />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold leading-5 text-ink-900">{session.user.displayName}</p>
          <p className="truncate text-xs leading-5 text-ink-500">{session.tenant.name}</p>
        </div>
      </div>
      <Button disabled={isSubmitting} icon={<LogOut className="h-4 w-4" />} onClick={() => void onLogout()} variant="secondary">
        退出
      </Button>
    </div>
  )
}
