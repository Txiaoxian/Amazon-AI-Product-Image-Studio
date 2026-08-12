import type { ISODateTimeString } from './platform'

export type AnalyticsGranularity = 'hour' | 'day' | 'week'
export type AnalyticsGroup = 'user' | 'project' | 'provider' | 'model' | 'imageType'
export type AnalyticsDataset = 'usage' | 'users' | 'tasks' | 'requests'

export interface AnalyticsQuery {
  from?: string
  to?: string
  granularity?: AnalyticsGranularity
  compare?: boolean
  userId?: string
  projectId?: string
  providerId?: string
  modelId?: string
  status?: string
  imageType?: string
  groupBy?: AnalyticsGroup
  search?: string
  pageNum?: number
  pageSize?: number
}

export interface AnalyticsMeta {
  from: ISODateTimeString
  to: ISODateTimeString
  timezone: 'Asia/Shanghai' | string
  granularity: AnalyticsGranularity
  comparedFrom?: ISODateTimeString
  comparedTo?: ISODateTimeString
  costType: 'ESTIMATED' | string
  generatedAt: ISODateTimeString
}

export interface AnalyticsMetricSet {
  taskCount: number
  outputCount: number
  terminalTaskCount: number
  succeededTaskCount: number
  taskSuccessRate: number
  activeUserCount: number
  loginActiveUserCount: number
  p95DurationMs: number
}

export interface AnalyticsMetricChanges {
  taskCount: number | null
  outputCount: number | null
  taskSuccessRate: number | null
  activeUserCount: number | null
  loginActiveUserCount: number | null
  p95DurationMs: number | null
}

export interface AnalyticsCostMetric {
  currency: string
  amount: string
  previousAmount?: string
  changePercent: number | null
  recordCount: number
  pricedRecordCount: number
  pricingCoverage: number
}

export interface AnalyticsTrendPoint {
  bucket: string
  taskCount: number
  outputCount: number
  succeededCount: number
  failedCount: number
  timedOutCount: number
  cancelledCount: number
  activeUserCount: number
  loginActiveUsers: number
}

export interface AnalyticsCostTrendPoint {
  bucket: string
  currency: string
  estimatedCost: string
}

export interface AnalyticsRankingItem {
  dimension: AnalyticsGroup | string
  dimensionId: string
  name: string
  secondaryName?: string
  taskCount: number
  outputCount: number
  successRate: number
  costs: AnalyticsCostMetric[]
}

export interface AnalyticsErrorGroup {
  errorCode: string
  count: number
}

export interface AnalyticsOverviewResponse {
  meta: AnalyticsMeta
  current: AnalyticsMetricSet
  previous: AnalyticsMetricSet
  changes: AnalyticsMetricChanges
  costs: AnalyticsCostMetric[]
  trend: AnalyticsTrendPoint[]
  costTrend: AnalyticsCostTrendPoint[]
  rankings: AnalyticsRankingItem[]
  errorGroups: AnalyticsErrorGroup[]
}

export interface AnalyticsUsageBreakdown {
  dimension: AnalyticsGroup | string
  dimensionId: string
  name: string
  recordCount: number
  inputTokens: number
  outputTokens: number
  billedImageCount: number
  outputCount: number
  costs: AnalyticsCostMetric[]
}

export interface AnalyticsUsageResponse {
  meta: AnalyticsMeta
  costs: AnalyticsCostMetric[]
  outputCount: number
  unitCosts: AnalyticsUnitCost[]
  costTrend: AnalyticsCostTrendPoint[]
  breakdowns: AnalyticsUsageBreakdown[]
}

export interface AnalyticsUnitCost {
  currency: string
  amount: string
  outputCount: number
  available: boolean
}

export interface AnalyticsUserRecord {
  userId: string
  displayName: string
  email: string
  status: string
  lastLoginAt: ISODateTimeString | ''
  activeDays: number
  taskCount: number
  outputCount: number
  successRate: number
  costs: AnalyticsCostMetric[]
  lastTaskAt: ISODateTimeString | ''
  lifecycle: string
}

export interface AnalyticsUserPage {
  meta: AnalyticsMeta
  records: AnalyticsUserRecord[]
  total: number
  pageNum: number
  pageSize: number
}

export interface AnalyticsTaskRecord {
  taskId: string
  userId: string
  userName: string
  projectId: string
  projectName: string
  providerId: string
  providerName: string
  modelId: string
  modelName: string
  type: string
  imageType: string
  status: string
  outputCount: number
  durationMs: number
  estimatedCost: string
  currency: string
  costStatus: string
  errorCode: string
  errorMessage: string
  createdAt: ISODateTimeString
  finishedAt: ISODateTimeString | ''
}

export interface AnalyticsTaskPage {
  meta: AnalyticsMeta
  summary: AnalyticsMetricSet
  records: AnalyticsTaskRecord[]
  total: number
  pageNum: number
  pageSize: number
}

export interface AnalyticsRequestSummary {
  callCount: number
  successCount: number
  failureCount: number
  successRate: number
  p50DurationMs: number
  p95DurationMs: number
}

export interface AnalyticsRequestTrendPoint {
  bucket: string
  callCount: number
  successCount: number
  failureCount: number
}

export interface AnalyticsProviderHealth {
  providerId: string
  providerName: string
  callCount: number
  successRate: number
  p95DurationMs: number
  lastFailureAt: ISODateTimeString | ''
  costs: AnalyticsCostMetric[]
}

export interface AnalyticsRequestsResponse {
  meta: AnalyticsMeta
  summary: AnalyticsRequestSummary
  trend: AnalyticsRequestTrendPoint[]
  providers: AnalyticsProviderHealth[]
  errorGroups: AnalyticsErrorGroup[]
}

export interface AnalyticsUserDetailResponse {
  meta: AnalyticsMeta
  user: AnalyticsUserRecord
  trend: AnalyticsTrendPoint[]
  costTrend: AnalyticsCostTrendPoint[]
  projects: AnalyticsUsageBreakdown[]
  models: AnalyticsUsageBreakdown[]
  failedTasks: AnalyticsTaskRecord[]
  auditVisible: boolean
}

export interface AnalyticsExport {
  blob: Blob
  filename: string
}
