import { AlertTriangle, FileClock, Loader2 } from 'lucide-react'
import type { ApiCallLog } from '../../types/admin'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

interface ApiCallLogPageView {
  records: ApiCallLog[]
  total: number
  pageNum: number
  pageSize: number
}

interface AdminApiCallLogsViewProps {
  page: ApiCallLogPageView
  detail: ApiCallLog | null
  selectedDetailId: ApiCallLog['id'] | null
  loadingDetailId: ApiCallLog['id'] | null
  isLoading: boolean
  error: string | null
  detailError: string | null
  onCloseDetail: () => void
  onPageChange: (pageNum: number) => void
  onLoadDetail: (id: ApiCallLog['id']) => void
}

const METADATA_DETAIL_LIMIT = 5000

export function AdminApiCallLogsView({
  page,
  detail,
  selectedDetailId,
  loadingDetailId,
  isLoading,
  error,
  detailError,
  onCloseDetail,
  onPageChange,
  onLoadDetail,
}: AdminApiCallLogsViewProps) {
  const currentDetail = detail?.id === selectedDetailId ? detail : null

  return (
    <section className="space-y-3">
      <div className="flex items-center gap-2">
        <FileClock className="h-4 w-4 text-ink-500" />
        <h3 className="text-sm font-semibold text-ink-900">API 调用日志</h3>
      </div>
      <StatusMessage message={error} />
      {isLoading ? <LoadingState text="正在加载 API 调用日志..." /> : null}
      {!isLoading && !error && page.records.length === 0 ? <EmptyState body="后端返回空分页，当前没有 api call logs。" title="暂无 API 调用日志" /> : null}
      {!isLoading && page.records.length > 0 ? (
        <div className="overflow-x-auto rounded-lg border border-ink-200">
          <table className="min-w-full divide-y divide-ink-200 text-left text-sm">
            <thead className="bg-ink-50 text-xs font-semibold text-ink-500">
              <tr>
                <th className="px-3 py-2">调用</th>
                <th className="px-3 py-2">任务</th>
                <th className="px-3 py-2">状态</th>
                <th className="px-3 py-2">错误</th>
                <th className="px-3 py-2">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-ink-100 bg-white">
              {page.records.map((record) => {
                const isLoadingDetail = loadingDetailId === record.id
                const errorGuidance = getApiCallErrorGuidance(record)
                return (
                  <tr key={record.id}>
                    <td className="max-w-[200px] px-3 py-2 align-top">
                      <p className="truncate font-semibold text-ink-900">{record.id}</p>
                      <p className="text-xs text-ink-400">{formatDateTime(record.createdAt)}</p>
                    </td>
                    <td className="max-w-[220px] px-3 py-2 align-top text-xs text-ink-600">
                      <p className="truncate">{record.taskId}</p>
                      <p className="truncate">{record.providerId} / {record.modelId}</p>
                    </td>
                    <td className="px-3 py-2 align-top">
                      <span className={statusPillClassName(record.status)}>{record.status}</span>
                      <p className="mt-1 text-xs text-ink-400">{record.httpStatus ?? 'no-http'} · {record.durationMs}ms</p>
                    </td>
                    <td className="max-w-[260px] px-3 py-2 align-top text-xs text-ink-600">
                      <p className="truncate">{record.errorCode || '无'}</p>
                      <p className="truncate">{errorGuidance?.summary || '无错误信息'}</p>
                    </td>
                    <td className="px-3 py-2 align-top">
                      <Button
                        icon={isLoadingDetail ? <Loader2 className="h-4 w-4 animate-spin" /> : undefined}
                        onClick={() => onLoadDetail(record.id)}
                        variant="secondary"
                      >
                        {isLoadingDetail ? '加载中' : '查看详情'}
                      </Button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      ) : null}
      <PaginationControls isLoading={isLoading} label="API 调用日志分页" onPageChange={onPageChange} page={page} />
      <Modal
        isOpen={selectedDetailId !== null}
        maxWidthClass="max-w-6xl"
        onClose={onCloseDetail}
        title={`API 调用详情：${selectedDetailId ?? ''}`}
      >
        <StatusMessage message={detailError} />
        {loadingDetailId ? <LoadingState text={`正在加载 API 调用详情：${loadingDetailId}...`} /> : null}
        {currentDetail ? <ApiCallDetail detail={currentDetail} /> : null}
      </Modal>
    </section>
  )
}

function ApiCallDetail({ detail }: { detail: ApiCallLog }) {
  const errorGuidance = getApiCallErrorGuidance(detail)

  return (
    <article>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="min-w-0">
          <p className="text-xs text-ink-500">请求标识：{detail.requestId || '未返回'} · {formatDateTime(detail.createdAt)}</p>
        </div>
        <span className={statusPillClassName(detail.status)}>{statusLabel(detail.status)}</span>
      </div>
      <dl className="mt-3 grid grid-cols-2 gap-2 text-xs text-ink-600 md:grid-cols-4">
        <Metric label="任务" value={detail.taskId} />
        <Metric label="HTTP" value={String(detail.httpStatus ?? 'N/A')} />
        <Metric label="耗时" value={`${detail.durationMs}ms`} />
        <Metric label="错误码" value={detail.errorCode || '无'} />
      </dl>
      {errorGuidance ? (
        <div className="mt-3 rounded-lg border border-amber-200 bg-amber-50 px-3 py-3 text-sm text-amber-950">
          <p className="font-semibold">{errorGuidance.summary}</p>
          <p className="mt-1 leading-6 text-amber-900">{errorGuidance.action}</p>
        </div>
      ) : null}
      {detail.errorMessage ? (
        <div className="mt-3 rounded-lg border border-ink-200 bg-ink-50 px-3 py-2">
          <p className="text-xs font-semibold text-ink-500">原始错误（已脱敏）</p>
          <p className="mt-1 break-words text-xs leading-6 text-ink-700">{detail.errorMessage}</p>
        </div>
      ) : null}
      <div className="mt-3 grid gap-3 lg:grid-cols-2">
        <BoundedMetadataBlock title="脱敏请求" value={detail.redactedRequest} />
        <BoundedMetadataBlock title="脱敏响应" value={detail.redactedResponse} />
      </div>
    </article>
  )
}

interface ApiCallErrorGuidance {
  summary: string
  action: string
}

function getApiCallErrorGuidance(record: Pick<ApiCallLog, 'errorCode' | 'errorMessage' | 'status'>): ApiCallErrorGuidance | null {
  if (record.status === 'SUCCESS' && !record.errorCode) {
    return null
  }

  switch (record.errorCode) {
    case 'PROVIDER_TRANSPORT_ERROR': {
      const errorMessage = record.errorMessage.toLowerCase()
      if (errorMessage.includes('unexpected eof')) {
        return {
          summary: '中转站在返回响应前提前断开了连接。',
          action: '建议稍后重试或切换中转站；若持续出现，请中转站检查上游连接和网关超时。',
        }
      }
      if (errorMessage.includes('deadline exceeded') || errorMessage.includes('timeout')) {
        return {
          summary: '中转站在超时时间内未返回响应。',
          action: '建议提高中转站超时时间，并降低并发后重试。',
        }
      }
      return {
        summary: '平台与中转站之间的连接异常。',
        action: '建议检查中转站网络状态后重试；若持续出现，请切换到可用线路。',
      }
    }
    case 'PROVIDER_RESPONSE_TOO_LARGE':
      return {
        summary: '中转站已返回成功响应，但响应数据超过平台配置的安全接收上限。',
        action: '平台会流式接收大图片。若仍出现此错误，请管理员核对返回内容，并按服务器磁盘与并发容量调整 Provider 响应和单图上限。',
      }
    case 'PROVIDER_INSUFFICIENT_QUOTA':
      return {
        summary: '中转站账户余额或可用额度不足。',
        action: '请补充额度，或切换到可用的中转站和模型后重试。',
      }
    case 'PROVIDER_RESPONSE_INVALID':
      return {
        summary: '中转站返回的数据格式无法解析。',
        action: '请检查中转站是否兼容当前图片接口，并在脱敏响应中确认返回字段。',
      }
    case 'PROVIDER_IMAGE_FETCH_FAILED':
      return {
        summary: '平台无法安全下载中转站返回的图片。',
        action: '请检查图片地址是否可访问且符合安全策略，或让中转站直接返回图片数据。',
      }
    case 'PROVIDER_REQUEST_INVALID':
      return {
        summary: '发送给中转站的请求参数无效。',
        action: '请核对模型能力、图片比例、质量和参考图参数后重试。',
      }
    case 'PROVIDER_URL_INVALID':
    case 'PROVIDER_BASE_URL_INVALID':
      return {
        summary: '中转站地址未通过安全校验。',
        action: '请管理员检查中转站 Base URL，确保使用允许访问的 HTTPS 公网地址。',
      }
    case 'PROVIDER_CREDENTIAL_UNAVAILABLE':
      return {
        summary: '中转站凭据当前不可用。',
        action: '请管理员重新保存并测试中转站 API Key。',
      }
    case 'PROVIDER_HTTP_ERROR':
      return {
        summary: '中转站返回了失败的 HTTP 响应。',
        action: '请结合 HTTP 状态和脱敏响应，检查额度、模型名称及中转站服务状态。',
      }
    default:
      return {
        summary: '本次中转站调用失败。',
        action: '请打开详情查看错误码和脱敏诊断信息，再根据中转站状态决定重试或切换线路。',
      }
  }
}

function PaginationControls({
  page,
  isLoading,
  label,
  onPageChange,
}: {
  page: ApiCallLogPageView
  isLoading: boolean
  label: string
  onPageChange: (pageNum: number) => void
}) {
  const hasPrevious = page.pageNum > 1
  const hasNext = page.pageNum * page.pageSize < page.total

  return (
    <div aria-label={label} className="flex flex-wrap items-center justify-between gap-2 text-xs text-ink-500">
      <span>
        第 {page.pageNum} 页 · 共 {page.total} 条 · 每页 {page.pageSize} 条
      </span>
      <div className="flex gap-2">
        <button
          className="rounded-md border border-ink-200 bg-white px-3 py-1.5 font-semibold text-ink-700 transition hover:bg-ink-50 disabled:text-ink-300"
          disabled={!hasPrevious || isLoading}
          onClick={() => onPageChange(page.pageNum - 1)}
          type="button"
        >
          上一页
        </button>
        <button
          className="rounded-md border border-ink-200 bg-white px-3 py-1.5 font-semibold text-ink-700 transition hover:bg-ink-50 disabled:text-ink-300"
          disabled={!hasNext || isLoading}
          onClick={() => onPageChange(page.pageNum + 1)}
          type="button"
        >
          下一页
        </button>
      </div>
    </div>
  )
}

function BoundedMetadataBlock({ title, value }: { title: string; value: unknown }) {
  return (
    <div className="min-w-0 rounded-lg border border-ink-200 bg-white p-3">
      <h4 className="text-xs font-semibold uppercase tracking-normal text-ink-500">{title}</h4>
      <pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap break-words text-xs leading-5 text-ink-700">
        {formatRedactedJson(value, METADATA_DETAIL_LIMIT)}
      </pre>
    </div>
  )
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md border border-ink-200 bg-white px-3 py-2">
      <dt className="truncate text-ink-400">{label}</dt>
      <dd className="mt-1 break-words font-semibold text-ink-800">{value}</dd>
    </div>
  )
}

function LoadingState({ text }: { text: string }) {
  return (
    <div className="flex items-center justify-center gap-2 rounded-md bg-ink-50 px-4 py-8 text-sm text-ink-500">
      <Loader2 className="h-4 w-4 animate-spin" />
      <span>{text}</span>
    </div>
  )
}

function EmptyState({ title, body }: { title: string; body: string }) {
  return (
    <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-8 text-center">
      <p className="text-sm font-medium text-ink-700">{title}</p>
      <p className="mt-1 text-xs text-ink-400">{body}</p>
    </div>
  )
}

function StatusMessage({ message }: { message: string | null }) {
  if (!message) {
    return null
  }

  return (
    <div className="flex items-start gap-2 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm leading-6 text-red-700" role="alert">
      <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  )
}

function statusPillClassName(status: ApiCallLog['status']): string {
  return `rounded-md px-2 py-1 text-xs font-semibold ${
    status === 'SUCCESS' ? 'bg-emerald-50 text-emerald-700' : 'bg-red-50 text-red-700'
  }`
}

function statusLabel(status: ApiCallLog['status']): string {
  return status === 'SUCCESS' ? '成功' : '失败'
}

function formatRedactedJson(value: unknown, maxLength: number): string {
  let rendered: string
  try {
    rendered = JSON.stringify(value ?? {}, null, 2)
  } catch {
    rendered = String(value)
  }

  if (rendered.length <= maxLength) {
    return rendered
  }
  return `${rendered.slice(0, maxLength)}\n... 内容已截断`
}

function formatDateTime(value: string): string {
  if (!value) {
    return '未返回'
  }

  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) {
    return value
  }

  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(parsed)
}
