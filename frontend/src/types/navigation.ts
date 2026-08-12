export type WorkspacePrimaryRoute = 'products' | 'studio' | 'assets' | 'templates' | 'analytics' | 'settings'

export type SurfaceMode = 'light' | 'workbench'

export interface WorkspaceRoute {
  pathname: string
  primary: WorkspacePrimaryRoute
}

export interface ShellNavItem {
  label: string
  pathname: string
  route: WorkspacePrimaryRoute
}

export function primaryRouteFromPathname(pathname: string): WorkspacePrimaryRoute {
  if (pathname === '/products') return 'products'
  if (pathname === '/assets') return 'assets'
  if (pathname === '/templates') return 'templates'
  if (pathname.startsWith('/admin/providers') || pathname.startsWith('/admin/settings')) return 'settings'
  if (pathname.startsWith('/admin')) return 'analytics'
  return 'studio'
}

export function normalizeWorkspacePathname(pathname: string): string {
  return pathname === '/' ? '/studio' : pathname
}
