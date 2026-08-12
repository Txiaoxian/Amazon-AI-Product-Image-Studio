import { useState } from 'react'
import { adminApi } from '../../../api/admin'
import { downloadBlob } from '../../../lib/download'
import type { AnalyticsDataset, AnalyticsQuery } from '../../../types/analytics'

const datasetLabels: Record<AnalyticsDataset, string> = {
  usage: '用量与费用',
  users: '用户与活跃',
  tasks: '生图任务',
  requests: '模型调用',
}

export function useAnalyticsExport(dataset: AnalyticsDataset, query: AnalyticsQuery) {
  const [exporting, setExporting] = useState(false)
  const [feedback, setFeedback] = useState<{ message: string; tone: 'success' | 'error' } | null>(null)

  const exportData = async () => {
    if (exporting) return
    setExporting(true)
    setFeedback(null)
    try {
      const result = await adminApi.exportAnalytics(dataset, query)
      downloadBlob(result.blob, result.filename)
      setFeedback({ message: `${datasetLabels[dataset]}数据已开始下载。`, tone: 'success' })
    } catch {
      setFeedback({ message: `${datasetLabels[dataset]}数据导出失败，当前页面数据不受影响，请稍后重试。`, tone: 'error' })
    } finally {
      setExporting(false)
    }
  }

  return { exportData, exporting, feedback }
}
