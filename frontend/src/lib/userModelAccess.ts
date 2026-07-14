import type { ModelApi } from '../api/models'
import type { Model } from '../types/platform'

export async function loadAllModelsForAccess(api: ModelApi): Promise<Model[]> {
  const pageSize = 100
  const models: Model[] = []
  for (let pageNum = 1; pageNum <= 100; pageNum += 1) {
    const page = await api.list({ pageNum, pageSize })
    models.push(...page.records)
    if (models.length >= page.total) {
      break
    }
  }
  return models
}
