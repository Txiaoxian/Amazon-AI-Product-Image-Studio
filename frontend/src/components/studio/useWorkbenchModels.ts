import { useCallback, useEffect, useState } from 'react'
import { modelApi as defaultModelApi, type ModelApi } from '../../api/models'
import { isApiClientError } from '../../api/client'
import type { Model } from '../../types/platform'

export type WorkbenchModelStatus = 'idle' | 'loading' | 'success' | 'error'

interface UseWorkbenchModelsOptions {
  modelApi?: ModelApi
}

export function useWorkbenchModels({ modelApi = defaultModelApi }: UseWorkbenchModelsOptions = {}) {
  const [models, setModels] = useState<Model[]>([])
  const [status, setStatus] = useState<WorkbenchModelStatus>('idle')
  const [error, setError] = useState<string | null>(null)

  const refreshModels = useCallback(async () => {
    setStatus('loading')
    setError(null)

    try {
      const records = await modelApi.listEnabledCapabilities('generate')
      setModels(records.filter(isSelectableWorkbenchModel))
      setStatus('success')
    } catch (requestError) {
      setStatus('error')
      setError(getWorkbenchModelErrorMessage(requestError))
    }
  }, [modelApi])

  useEffect(() => {
    void refreshModels()
  }, [refreshModels])

  return {
    error,
    models,
    refreshModels,
    status,
  }
}

function isSelectableWorkbenchModel(model: Model): boolean {
  return model.status === 'ENABLED' && model.supportsGenerate && model.providerName.trim().length > 0
}

function getWorkbenchModelErrorMessage(error: unknown): string {
  if (!isApiClientError(error)) {
    return '无法加载可用模型，请稍后重试。'
  }

  if (error.status === 401) {
    return '登录状态已失效，请重新登录。'
  }

  if (error.status === 403) {
    return '没有权限读取可用模型。'
  }

  return '无法加载可用模型，请稍后重试。'
}
