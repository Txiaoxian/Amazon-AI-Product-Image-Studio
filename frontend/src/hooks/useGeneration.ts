import { useCallback, useState } from 'react'
import { createHistoryItem, type HistoryWithImage } from '../db/historyRepository'
import { getTotalImageBytes, saveImage } from '../db/imageRepository'
import { FriendlyError, StorageLimitError, toFriendlyError } from '../lib/errors'
import { getImageDimensions } from '../lib/imageMeta'
import { isWithinStorageLimit } from '../lib/storageLimit'
import { getProviderAdapter } from '../providers/registry'
import type { AppSettings, GeneratedImageResult, GenerationRequest } from '../providers/types'

type GenerationStatus = 'idle' | 'loading' | 'success' | 'error'

export interface CurrentGeneration {
  history: HistoryWithImage
  result: GeneratedImageResult
}

export function useGeneration(settings: AppSettings) {
  const [status, setStatus] = useState<GenerationStatus>('idle')
  const [error, setError] = useState('')
  const [currentItems, setCurrentItems] = useState<CurrentGeneration[]>([])
  const [selectedIndex, setSelectedIndex] = useState(0)
  const current = currentItems[selectedIndex] ?? null

  const generate = useCallback(
    async (request: GenerationRequest): Promise<CurrentGeneration[] | null> => {
      setStatus('loading')
      setError('')

      try {
        const prompt = request.prompt.trim()
        if (!prompt) {
          throw new FriendlyError('请输入图片生成提示词。', 'MISSING_PROMPT')
        }

        const adapter = getProviderAdapter(request.model.provider)
        const providerSettings = settings.providers[request.model.provider]
        const normalizedRequest = { ...request, prompt }
        const results = await adapter.generateImages(normalizedRequest, providerSettings)
        if (results.length === 0) {
          throw new FriendlyError('图片服务没有返回可用图片，请调整提示词或稍后重试。', 'EMPTY_IMAGE_RESPONSE')
        }
        const referenceBytes = request.references.reduce((total, file) => total + file.size, 0)
        const generatedBytes = results.reduce((total, result) => total + result.fileSize, 0)
        const currentBytes = await getTotalImageBytes()

        if (!isWithinStorageLimit(currentBytes, generatedBytes + referenceBytes * results.length, settings.storageLimitBytes)) {
          throw new StorageLimitError()
        }

        const referenceInputs = await Promise.all(
          request.references.map(async (file) => {
            const dimensions = await getImageDimensions(file).catch(() => undefined)

            return {
              file,
              dimensions,
            }
          }),
        )

        const nextCurrentItems = await Promise.all(
          results.map(async (result) => {
            const generatedImage = await saveImage({
              blob: result.blob,
              mimeType: result.mimeType,
              size: result.fileSize,
              width: result.width,
              height: result.height,
              purpose: 'generated',
            })

            const referenceImages = await Promise.all(
              referenceInputs.map(({ file, dimensions }) =>
                saveImage({
                  blob: file,
                  mimeType: file.type,
                  size: file.size,
                  width: dimensions?.width,
                  height: dimensions?.height,
                  purpose: 'reference',
                }),
              ),
            )

            const item = await createHistoryItem({
              request: normalizedRequest,
              result,
              generatedImageId: generatedImage.id,
              referenceImageIds: referenceImages.map((image) => image.id),
            })

            return {
              history: {
                item,
                image: generatedImage,
              },
              result,
            }
          }),
        )

        setCurrentItems(nextCurrentItems)
        setSelectedIndex(0)
        setStatus('success')
        return nextCurrentItems
      } catch (err) {
        const friendlyError = toFriendlyError(err)
        setError(friendlyError.message)
        setStatus('error')
        return null
      }
    },
    [settings],
  )

  const setFromHistory = useCallback((history: HistoryWithImage) => {
    if (!history.image) {
      setError('历史记录中的原图不存在，可能已被浏览器清理。')
      setStatus('error')
      return
    }

    const result: GeneratedImageResult = {
      blob: history.image.blob,
      mimeType: history.image.mimeType,
      width: history.item.width,
      height: history.item.height,
      fileSize: history.item.fileSize,
      durationMs: history.item.durationMs,
    }

    setCurrentItems([{ history, result }])
    setSelectedIndex(0)
    setError('')
    setStatus('success')
  }, [])

  const selectCurrent = useCallback((index: number) => {
    setSelectedIndex(index)
  }, [])

  return {
    status,
    error,
    current,
    currentItems,
    selectedIndex,
    generate,
    setFromHistory,
    selectCurrent,
  }
}
