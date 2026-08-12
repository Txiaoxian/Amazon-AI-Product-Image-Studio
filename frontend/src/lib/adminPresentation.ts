export const ADMIN_CHART_TOKENS = {
  axis: '#64748b',
  grid: '#e2e8f0',
  palette: ['#2563eb', '#f59e0b', '#10b981', '#ef4444', '#8b5cf6', '#64748b'],
  text: '#475569',
} as const

const taskStatusLabels: Record<string, string> = {
  QUEUED: '排队中',
  RUNNING: '生成中',
  RETRYING: '正在重试',
  SUCCEEDED: '已完成',
  SUCCESS: '成功',
  FAILED: '失败',
  FAILURE: '失败',
  TIMED_OUT: '已超时',
  CANCELLED: '已取消',
}

const lifecycleLabels: Record<string, string> = {
  NEW: '新活跃',
  RETURNING: '持续活跃',
  RESURRECTED: '回流',
  DORMANT: '沉默',
  INACTIVE: '未活跃',
}

const entityStatusLabels: Record<string, string> = {
  ACTIVE: '正常',
  ENABLED: '已启用',
  DISABLED: '已停用',
  ARCHIVED: '已归档',
}

const costStatusLabels: Record<string, string> = {
  CALCULATED: '费用已计算',
  UNAVAILABLE: '费用暂不可用',
  LEGACY_UNKNOWN: '历史费用状态未知',
}

const taskTypeLabels: Record<string, string> = {
  IMAGE_GENERATION: '图片生成',
  IMAGE_EDIT: '图片编辑',
}

export const ADMIN_IMAGE_TYPE_VALUES = [
  'MAIN',
  'A_PLUS',
  'SCENE',
  'DETAIL',
  'DIMENSION',
  'SELLING_POINT',
  'PROMOTION',
  'COMPARISON',
] as const

const imageTypeLabels: Record<string, string> = {
  MAIN: '商品主图',
  A_PLUS: 'A+ 图片',
  SCENE: '场景图',
  DETAIL: '细节图',
  DIMENSION: '尺寸图',
  SELLING_POINT: '卖点图',
  PROMOTION: '宣传图',
  COMPARISON: '对比图',
  // 兼容早期数据，不再作为新任务筛选项。
  WHITE_BACKGROUND: '白底图',
  LIFESTYLE: '生活方式图',
  INFOGRAPHIC: '信息图',
  PACKAGING: '包装图',
  OTHER: '其他图片',
}

const dimensionLabels: Record<string, string> = {
  user: '用户',
  project: '项目',
  provider: '中转站',
  model: '模型',
  imageType: '图片类型',
}

const roleLabels: Record<string, string> = {
  admin: '运营管理员',
  seller: '业务用户',
  viewer: '只读成员',
  editor: '编辑成员',
  owner: '负责人',
}

export const ADMIN_AUDIT_ACTION_VALUES = [
  'auth.init_admin',
  'auth.login',
  'auth.logout',
  'auth.password_change',
  'auth.provision_tenant',
  'tenant.update',
  'provider.create',
  'provider.update',
  'provider.enable',
  'provider.disable',
  'provider.delete',
  'provider.test',
  'model.create',
  'model.update',
  'model.enable',
  'model.disable',
  'model.delete',
  'user.create',
  'user.update',
  'user.enable',
  'user.disable',
  'user.roles.replace',
  'user.ai_access.replace',
  'role.create',
  'role.update',
  'role.delete',
  'role.permissions.replace',
  'project.create',
  'project.update',
  'project.delete',
  'project_member.create',
  'project_member.update',
  'project_member.delete',
  'asset.upload',
  'asset.update',
  'asset.favorite',
  'asset.unfavorite',
  'asset.download',
  'asset.thumbnail',
  'asset.delete',
  'task.create',
  'task.cancel',
  'task.retry',
  'storage.orphan_cleanup',
  'system_settings.update',
] as const

const actionLabels: Record<string, string> = {
  'auth.init_admin': '初始化管理员账号',
  'auth.login': '登录平台',
  'auth.logout': '退出平台',
  'auth.password_change': '修改登录密码',
  'auth.provision_tenant': '配置新租户',
  'provider.create': '新建中转站',
  'provider.update': '修改中转站',
  'provider.enable': '启用中转站',
  'provider.disable': '停用中转站',
  'provider.delete': '删除中转站',
  'provider.test': '测试中转站连接',
  'model.create': '新建模型',
  'model.update': '修改模型',
  'model.enable': '启用模型',
  'model.disable': '停用模型',
  'model.delete': '删除模型',
  'user.create': '新建用户',
  'user.update': '修改用户',
  'user.enable': '启用用户',
  'user.disable': '停用用户',
  'user.roles.replace': '调整用户角色',
  'user.ai_access.replace': '调整用户可用模型',
  'role.create': '新建角色',
  'role.update': '修改角色',
  'role.delete': '删除角色',
  'role.permissions.replace': '调整角色权限',
  'system_settings.update': '修改系统设置',
  'project.create': '新建项目',
  'project.update': '修改项目',
  'project.delete': '删除项目',
  'project_member.create': '添加项目成员',
  'project_member.update': '修改项目成员',
  'project_member.delete': '移除项目成员',
  'tenant.update': '修改租户信息',
  'asset.upload': '上传图片资产',
  'asset.update': '修改图片资产',
  'asset.favorite': '收藏图片资产',
  'asset.unfavorite': '取消收藏图片资产',
  'asset.download': '下载图片资产',
  'asset.thumbnail': '查看图片缩略图',
  'asset.delete': '删除图片资产',
  'task.create': '创建生图任务',
  'task.cancel': '取消生图任务',
  'task.retry': '重试生图任务',
  'storage.orphan_cleanup': '清理孤立存储对象',
}

const resourceLabels: Record<string, string> = {
  auth: '登录会话',
  session: '登录会话',
  provider: '中转站',
  model: '模型',
  user: '用户',
  role: '角色',
  tenant: '租户',
  settings: '系统设置',
  system_settings: '系统设置',
  project: '项目',
  project_member: '项目成员',
  asset: '图片资产',
  task: '生图任务',
  generation_task: '生图任务',
  storage: '存储',
  system: '系统维护',
}

export const ADMIN_AUDIT_RESOURCE_VALUES = [
  'session',
  'tenant',
  'provider',
  'model',
  'user',
  'role',
  'project',
  'project_member',
  'asset',
  'generation_task',
  'storage',
  'system',
  'system_settings',
] as const

const currencyLabels: Record<string, string> = {
  USD: '美元',
  CNY: '人民币',
  RMB: '人民币',
  EUR: '欧元',
  GBP: '英镑',
  JPY: '日元',
  HKD: '港币',
}

const providerTypeLabels: Record<string, string> = {
  OPENAI_COMPATIBLE: 'OpenAI 兼容中转站',
  OPENAI: 'OpenAI 中转站',
  GEMINI: 'Gemini 中转站',
}

export type StatusTone = 'success' | 'warning' | 'danger' | 'info' | 'neutral'

export interface ErrorCategory {
  label: string
  guidance: string
  tone: StatusTone
}

export function taskStatusLabel(value: string): string {
  return taskStatusLabels[normalizeKey(value)] ?? '未知任务状态'
}

export function apiCallStatusLabel(value: string): string {
  const key = normalizeKey(value)
  if (key === 'SUCCESS') return '成功'
  if (key === 'FAILURE') return '失败'
  return '未知调用状态'
}

export function lifecycleLabel(value: string): string {
  return lifecycleLabels[normalizeKey(value)] ?? '状态待确认'
}

export function entityStatusLabel(value: string): string {
  return entityStatusLabels[normalizeKey(value)] ?? '未知状态'
}

export function costStatusLabel(value: string): string {
  return costStatusLabels[normalizeKey(value)] ?? '费用状态待确认'
}

export function taskTypeLabel(value: string): string {
  return taskTypeLabels[normalizeKey(value)] ?? '其他生图任务'
}

export function imageTypeLabel(value: string): string {
  return imageTypeLabels[normalizeKey(value)] ?? '其他图片类型'
}

export function dimensionLabel(value: string): string {
  return dimensionLabels[value] ?? '其他维度'
}

export function roleLabel(value: string): string {
  return roleLabels[value.toLowerCase().trim()] ?? '自定义角色'
}

export function auditActionLabel(value: string): string {
  return actionLabels[value.trim()] ?? '执行了其他管理操作'
}

export function auditResourceLabel(value: string): string {
  return resourceLabels[value.toLowerCase().trim()] ?? '平台资源'
}

export function providerTypeLabel(value: string): string {
  return providerTypeLabels[normalizeKey(value)] ?? '其他类型中转站'
}

export function buildTaskStatusOptions(values: readonly string[]) {
  return values.map((value) => ({ value, label: taskStatusLabel(value) }))
}

export function buildApiCallStatusOptions(values: readonly string[]) {
  return values.map((value) => ({ value, label: apiCallStatusLabel(value) }))
}

export function buildEntityStatusOptions(values: readonly string[]) {
  return values.map((value) => ({ value, label: entityStatusLabel(value) }))
}

export function buildAuditActionOptions(values: readonly string[]) {
  return values.map((value) => ({ value, label: auditActionLabel(value) }))
}

export function buildAuditResourceOptions(values: readonly string[]) {
  return values.map((value) => ({ value, label: auditResourceLabel(value) }))
}

export function statusTone(value: string): StatusTone {
  const key = normalizeKey(value)
  if (['SUCCESS', 'SUCCEEDED', 'ACTIVE', 'ENABLED', 'CALCULATED'].includes(key)) return 'success'
  if (['QUEUED', 'RUNNING', 'RETRYING'].includes(key)) return 'info'
  if (['TIMED_OUT', 'DORMANT', 'LEGACY_UNKNOWN'].includes(key)) return 'warning'
  if (['FAILURE', 'FAILED', 'CANCELLED', 'DISABLED', 'UNAVAILABLE'].includes(key)) return 'danger'
  return 'neutral'
}

export function errorCategory(errorCode: string, errorMessage = ''): ErrorCategory {
  const code = normalizeKey(errorCode)
  const message = errorMessage.toLowerCase()
  if (code.includes('TIMEOUT') || code.includes('DEADLINE') || message.includes('timeout') || message.includes('deadline')) {
    return { label: '中转站响应超时', guidance: '检查中转站可用性和超时设置，必要时降低并发后重试。', tone: 'warning' }
  }
  if (code.includes('RATE') || code.includes('LIMIT') || code.includes('QUOTA') || message.includes('429')) {
    return { label: '调用频率受限', guidance: '降低并发或稍后重试，并检查中转站额度与限流策略。', tone: 'warning' }
  }
  if (code.includes('AUTH') || code.includes('CREDENTIAL') || code.includes('UNAUTHORIZED') || code.includes('FORBIDDEN')) {
    return { label: '鉴权失败', guidance: '重新保存并测试中转站密钥，确认账号仍有模型访问权限。', tone: 'danger' }
  }
  if (code.includes('PARAM') || code.includes('INVALID') || code.includes('UNSUPPORTED') || code.includes('REQUEST')) {
    return { label: '模型参数不支持', guidance: '核对模型能力、图片规格、质量和参考图数量后重试。', tone: 'warning' }
  }
  if (code.includes('TRANSPORT') || code.includes('NETWORK') || code.includes('CONNECTION')) {
    return { label: '中转站连接异常', guidance: '检查中转站网络与网关状态，持续失败时切换可用线路。', tone: 'danger' }
  }
  if (code.includes('HTTP') || code.includes('PROVIDER') || code.includes('SERVICE') || code.includes('INTERNAL')) {
    return { label: '中转站服务异常', guidance: '查看脱敏技术详情，并结合中转站状态决定重试或切换线路。', tone: 'danger' }
  }
  return { label: '其他调用异常', guidance: '查看技术详情中的错误码和脱敏信息，确认影响范围后处理。', tone: 'neutral' }
}

export function formatCompactNumber(value: number, unit = ''): string {
  if (!Number.isFinite(value)) return `暂无${unit}`
  const absolute = Math.abs(value)
  if (absolute >= 10000) {
    const compact = (value / 10000).toFixed(absolute >= 100000 ? 1 : 2).replace(/\.0+$|(?<=\.[0-9])0+$/, '')
    return `${compact}万${unit}`
  }
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(value)}${unit}`
}

export function formatExactNumber(value: number, unit = ''): string {
  if (!Number.isFinite(value)) return `暂无${unit}`
  return `${new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 8 }).format(value)}${unit}`
}

export function formatPercentage(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return '暂无'
  return `${(value * 100).toFixed(digits)}%`
}

export function formatChange(value: number | null | undefined, lowerIsBetter = false): string {
  if (value === null || value === undefined || !Number.isFinite(value)) return '上一周期暂无可比数据'
  if (Math.abs(value) < 0.05) return '与上一周期基本持平'
  const direction = value > 0 ? '增加' : '减少'
  const impact = lowerIsBetter ? (value > 0 ? '，需关注' : '，表现改善') : ''
  return `较上一周期${direction} ${Math.abs(value).toFixed(1)}%${impact}`
}

export function formatDuration(durationMs: number): string {
  if (!Number.isFinite(durationMs) || durationMs < 0) return '暂无'
  if (durationMs < 1000) return `${Math.round(durationMs)}毫秒`
  const seconds = durationMs / 1000
  if (seconds < 60) return `${trimSingleDecimal(seconds)}秒`
  const minutes = seconds / 60
  if (minutes < 60) return `${trimSingleDecimal(minutes)}分钟`
  return `${trimSingleDecimal(minutes / 60)}小时`
}

function trimSingleDecimal(value: number): string {
  return value.toFixed(1).replace(/\.0$/, '')
}

export function formatCurrency(amount: string, currency: string): string {
  const label = currencyLabels[normalizeKey(currency)] ?? '未知币种'
  const numeric = Number(amount)
  if (!Number.isFinite(numeric)) return `暂无${label}费用`
  const absolute = Math.abs(numeric)
  const digits = absolute > 0 && absolute < 0.01 ? 6 : 2
  return `${numeric.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: digits })}${label}`
}

export function formatPreciseCurrency(amount: string, currency: string): string {
  const label = currencyLabels[normalizeKey(currency)] ?? '未知币种'
  const normalized = normalizeDecimalString(amount)
  return normalized ? `${normalized}${label}` : `暂无${label}费用`
}

export function currencyLabel(currency: string): string {
  return currencyLabels[normalizeKey(currency)] ?? '未知币种'
}

export function formatDateLabel(value: string | Date): string {
  const date = toDate(value)
  if (!date) return '日期未知'
  return new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', timeZone: 'Asia/Shanghai' }).format(date)
}

export function formatDateRange(from: string | Date, toInclusive: string | Date): string {
  const start = toDate(from)
  const end = toDate(toInclusive)
  if (!start || !end) return '时间范围待确认'
  const formatter = new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', timeZone: 'Asia/Shanghai' })
  return `${formatter.format(start)}—${formatter.format(end)}`
}

export function formatDateTime(value: string | Date, empty = '暂无记录'): string {
  const date = toDate(value)
  if (!date) return empty
  return `${new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
    timeZone: 'Asia/Shanghai',
  }).format(date)}（北京时间）`
}

export function formatTime(value: string | Date): string {
  const date = toDate(value)
  if (!date) return '时间未知'
  return new Intl.DateTimeFormat('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false, timeZone: 'Asia/Shanghai' }).format(date)
}

export function formatPricingCoverage(value: number): string {
  return `定价覆盖率 ${formatPercentage(value)}`
}

function normalizeKey(value: string): string {
  return value.trim().toUpperCase()
}

function normalizeDecimalString(value: string): string | null {
  const match = value.trim().match(/^(\d+)(?:\.(\d+))?$/)
  if (!match) return null
  const integer = match[1].replace(/^0+(?=\d)/, '')
  const fraction = (match[2] ?? '').replace(/0+$/, '')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return fraction ? `${grouped}.${fraction}` : `${grouped}.00`
}

function toDate(value: string | Date): Date | null {
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  if (!value.trim()) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}
