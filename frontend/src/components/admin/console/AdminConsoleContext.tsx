import { createContext, useContext } from 'react'
import type { AuthSession } from '../../../types/auth'
import type { AnalyticsQuery } from '../../../types/analytics'

export interface AdminRouteSnapshot {
  pathname: string
  searchParams: URLSearchParams
}

export interface AdminConsoleContextValue {
  session: AuthSession
  route: AdminRouteSnapshot
  analyticsQuery: AnalyticsQuery
  dateFrom: string
  dateTo: string
  compare: boolean
  hasPermission: (permission: string) => boolean
  navigate: (pathname: string, queryPatch?: Record<string, string | number | boolean | null | undefined>) => void
  updateQuery: (patch: Record<string, string | number | boolean | null | undefined>, replace?: boolean) => void
  setDateRange: (from: string, to: string) => void
  setCompare: (compare: boolean) => void
  onLogout: () => Promise<void>
}

export const AdminConsoleContext = createContext<AdminConsoleContextValue | null>(null)

export function useAdminConsole(): AdminConsoleContextValue {
  const context = useContext(AdminConsoleContext)
  if (!context) {
    throw new Error('Admin console context is unavailable.')
  }
  return context
}

export function useAdminAnalyticsQuery(): AnalyticsQuery {
  const { analyticsQuery, route } = useAdminConsole()
  return {
    ...analyticsQuery,
    userId: route.searchParams.get('userId') || undefined,
    projectId: route.searchParams.get('projectId') || undefined,
    providerId: route.searchParams.get('providerId') || undefined,
    modelId: route.searchParams.get('modelId') || undefined,
    status: route.searchParams.get('status') || undefined,
    imageType: route.searchParams.get('imageType') || undefined,
  }
}
