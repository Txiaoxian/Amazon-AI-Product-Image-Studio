import type { ImageModelConfig } from './types'

export const IMAGE_MODELS: ImageModelConfig[] = [
  {
    id: 'openai-gpt-image-2',
    label: 'OpenAI Image 2',
    provider: 'openai',
    model: 'gpt-image-2',
    description: '适合高质量产品主图、场景图和编辑。',
  },
  {
    id: 'gemini-nano-banana-2',
    label: 'Nano Banana 2',
    provider: 'gemini',
    model: 'nano-banana-2',
    description: '通过土土金中转站调用，支持 low/medium/high/auto 质量和像素尺寸输出。',
  },
  {
    id: 'relay2-gpt-image-2',
    label: '二号中转站 · GPT Image 2',
    provider: 'relay2',
    model: 'gpt-image-2',
    description: '通过二号中转站调用 gpt-image-2。',
  },
]

export const DEFAULT_MODEL_ID = IMAGE_MODELS[0].id

export function getModelById(modelId: string): ImageModelConfig {
  if (modelId === 'relay106-gpt-image-2') {
    return IMAGE_MODELS[2]
  }

  return IMAGE_MODELS.find((model) => model.id === modelId) ?? IMAGE_MODELS[0]
}
