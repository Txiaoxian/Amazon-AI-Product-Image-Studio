import {
  BarChart3,
  Boxes,
  FolderKanban,
  ImageIcon,
  Library,
  Settings,
  Sparkles,
  type LucideIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { APP_NAME } from '../../lib/constants'
import type { SurfaceMode, WorkspacePrimaryRoute } from '../../types/navigation'

interface WorkspaceShellProps {
  accountSlot: ReactNode
  activeRoute: WorkspacePrimaryRoute
  canViewAnalytics: boolean
  canViewSettings: boolean
  children: ReactNode
  onNavigate: (pathname: string) => void
  surfaceMode?: SurfaceMode
  analyticsPathname?: string
  settingsPathname?: string
}

interface NavigationItem {
  icon: LucideIcon
  label: string
  pathname: string
  route: WorkspacePrimaryRoute
  visible: boolean
}

export function WorkspaceShell({
  accountSlot,
  activeRoute,
  canViewAnalytics,
  canViewSettings,
  children,
  onNavigate,
  surfaceMode = 'light',
  analyticsPathname = '/admin/overview',
  settingsPathname = '/admin/settings',
}: WorkspaceShellProps) {
  const navigation: NavigationItem[] = [
    { icon: FolderKanban, label: '产品中心', pathname: '/products', route: 'products', visible: true },
    { icon: Sparkles, label: '创作室', pathname: '/studio', route: 'studio', visible: true },
    { icon: ImageIcon, label: '素材库', pathname: '/assets', route: 'assets', visible: true },
    { icon: Library, label: '模板库', pathname: '/templates', route: 'templates', visible: true },
    { icon: BarChart3, label: '数据看板', pathname: analyticsPathname, route: 'analytics', visible: canViewAnalytics },
    { icon: Settings, label: '设置', pathname: settingsPathname, route: 'settings', visible: canViewSettings },
  ]

  return (
    <div className={`workspace-shell workspace-shell--${surfaceMode}`}>
      <a className="workspace-skip-link" href="#workspace-main">跳到主要内容</a>
      <aside aria-label="主导航" className="workspace-global-nav">
        <button
          aria-label="进入创作室"
          className="workspace-brand-mark"
          onClick={() => onNavigate('/studio')}
          title={APP_NAME}
          type="button"
        >
          <Boxes aria-hidden="true" className="h-6 w-6" />
        </button>

        <nav aria-label="工作区导航" className="workspace-global-nav-items">
          {navigation.filter((item) => item.visible).map((item) => (
            <WorkspaceNavButton
              active={item.route === activeRoute}
              icon={item.icon}
              key={item.route}
              label={item.label}
              onClick={() => onNavigate(item.pathname)}
            />
          ))}
        </nav>

        <div className="workspace-global-nav-account">{accountSlot}</div>
      </aside>

      <main className="workspace-shell-main" id="workspace-main">{children}</main>
    </div>
  )
}

function WorkspaceNavButton({ active, icon: Icon, label, onClick }: {
  active: boolean
  icon: LucideIcon
  label: string
  onClick: () => void
}) {
  return (
    <button
      aria-current={active ? 'page' : undefined}
      className={`workspace-global-nav-button ${active ? 'is-active' : ''}`}
      onClick={onClick}
      type="button"
    >
      <Icon aria-hidden="true" className="h-5 w-5" />
      <span>{label}</span>
    </button>
  )
}
