import type { ProviderType } from '../types/platform'

export interface CapabilityPreset {
  value: string
  label: string
  description?: string
}

export interface ModelCapabilityTemplate {
  id: string
  label: string
  providerType: ProviderType
  modelName: string
  displayName: string
  supportsGenerate: boolean
  supportsEdit: boolean
  supportsMultiReference: boolean
  supportsN: boolean
  maxOutputCount: number
  supportedSizes: string[]
  supportedQualities: string[]
  supportedOutputFormats: string[]
}

const OPENAI_ASPECT_RATIO_PRESETS: CapabilityPreset[] = [
  { value: 'auto', label: '自动', description: '由模型根据提示词自动选择画面比例' },
  { value: '1:1', label: '1:1', description: '正方形主图' },
  { value: '1.62:1', label: '1.62:1', description: '接近黄金比例的横图' },
  { value: '2:3', label: '2:3', description: '竖向商品图' },
  { value: '3:2', label: '3:2', description: '横向场景图' },
  { value: '3:4', label: '3:4' },
  { value: '4:3', label: '4:3' },
  { value: '4:5', label: '4:5', description: '电商详情常用比例' },
  { value: '5:4', label: '5:4' },
  { value: '9:16', label: '9:16', description: '竖屏宣传图' },
  { value: '16:9', label: '16:9', description: '横屏宣传图' },
  { value: '21:9', label: '21:9', description: '超宽幅横图' },
]

const GEMINI_ASPECT_RATIO_PRESETS: CapabilityPreset[] = [
  { value: '1:1', label: '1:1', description: '正方形主图' },
  { value: '2:3', label: '2:3', description: '竖向商品图' },
  { value: '3:2', label: '3:2', description: '横向场景图' },
  { value: '3:4', label: '3:4' },
  { value: '4:3', label: '4:3' },
  { value: '4:5', label: '4:5', description: '电商详情常用比例' },
  { value: '5:4', label: '5:4' },
  { value: '9:16', label: '9:16', description: '竖屏宣传图' },
  { value: '16:9', label: '16:9', description: '横屏宣传图' },
  { value: '21:9', label: '21:9', description: '超宽幅横图' },
]

const OPENAI_QUALITY_PRESETS: CapabilityPreset[] = [
  { value: 'auto', label: '自动', description: '由模型或 Provider 默认值决定' },
  { value: 'low', label: '低质量', description: '快速草稿或低成本预览' },
  { value: 'medium', label: '中等质量', description: '通用生成质量' },
  { value: 'high', label: '高质量', description: '更适合最终出图' },
]

const GEMINI_RESOLUTION_PRESETS: CapabilityPreset[] = [
  { value: '1k', label: '1K', description: '常规商品图与快速预览' },
  { value: '2k', label: '2K', description: '详情页、A+ 图和场景图' },
  { value: '4k', label: '4K', description: '高精度最终交付素材' },
]

const LEGACY_QUALITY_PRESETS: CapabilityPreset[] = [
  { value: 'standard', label: '标准质量', description: '兼容旧版图片接口命名' },
  { value: 'hd', label: 'HD', description: '兼容旧 OpenAI 图片接口命名' },
]

export const MODEL_SIZE_PRESETS: CapabilityPreset[] = [...OPENAI_ASPECT_RATIO_PRESETS, ...GEMINI_ASPECT_RATIO_PRESETS]

export const MODEL_QUALITY_PRESETS: CapabilityPreset[] = [
  ...OPENAI_QUALITY_PRESETS,
  ...GEMINI_RESOLUTION_PRESETS,
  ...LEGACY_QUALITY_PRESETS,
]

export interface ModelParameterPresetConfig {
  sizeLabel: string
  sizeHelp: string
  sizePresets: CapabilityPreset[]
  qualityLabel: string
  qualityHelp: string
  qualityPresets: CapabilityPreset[]
}

export function modelParameterPresetsForProvider(providerType: ProviderType): ModelParameterPresetConfig {
  if (providerType === 'GEMINI') {
    return {
      sizeLabel: '画面比例',
      sizeHelp: '比例决定画面形状；只需选择一次，不需要为 1K、2K、4K 重复勾选。',
      sizePresets: GEMINI_ASPECT_RATIO_PRESETS,
      qualityLabel: '输出分辨率',
      qualityHelp: '1K、2K、4K 是输出分辨率档位，不等同于低、中、高生成质量。',
      qualityPresets: GEMINI_RESOLUTION_PRESETS,
    }
  }

  return {
    sizeLabel: '图片比例',
    sizeHelp: '比例决定画面形状；生成时由后端转换为符合 OpenAI 要求的像素尺寸。',
    sizePresets: OPENAI_ASPECT_RATIO_PRESETS,
    qualityLabel: '生成质量',
    qualityHelp: '按 OpenAI 官方语义配置：自动、低、中、高；质量与图片比例互相独立。',
    qualityPresets: OPENAI_QUALITY_PRESETS,
  }
}

export function parameterLabelsForCapabilities(
  supportedSizes: string[],
  supportedQualities: string[],
  providerType?: ProviderType,
): Pick<ModelParameterPresetConfig, 'sizeLabel' | 'qualityLabel'> {
  const hasAspectRatio = supportedSizes.some((value) => /^\d+(?:\.\d+)?:\d+$/.test(value.trim()))
  const usesGeminiImageConfig = hasAspectRatio
    && supportedQualities.some((value) => ['1k', '2k', '4k'].includes(value.trim().toLowerCase()))
  const resolvedProviderType = providerType ?? (usesGeminiImageConfig ? 'GEMINI' : 'OPENAI_COMPATIBLE')
  const config = modelParameterPresetsForProvider(resolvedProviderType)
  return {
    sizeLabel: config.sizeLabel,
    qualityLabel: config.qualityLabel,
  }
}

export const MODEL_CAPABILITY_TEMPLATES: ModelCapabilityTemplate[] = [
  {
    id: 'openai-compatible-gpt-image-2-euzhi',
    label: 'OpenAI 兼容 · gpt-image-2（官方参数）',
    providerType: 'OPENAI_COMPATIBLE',
    modelName: 'gpt-image-2',
    displayName: 'GPT Image 2',
    supportsGenerate: true,
    supportsEdit: true,
    supportsMultiReference: true,
    supportsN: false,
    maxOutputCount: 1,
    supportedSizes: OPENAI_ASPECT_RATIO_PRESETS.map((preset) => preset.value),
    supportedQualities: OPENAI_QUALITY_PRESETS.map((preset) => preset.value),
    supportedOutputFormats: ['png'],
  },
  {
    id: 'openai-gpt-image-2',
    label: 'OpenAI · gpt-image-2 常用配置',
    providerType: 'OPENAI',
    modelName: 'gpt-image-2',
    displayName: 'OpenAI GPT Image 2',
    supportsGenerate: true,
    supportsEdit: true,
    supportsMultiReference: true,
    supportsN: true,
    maxOutputCount: 4,
    supportedSizes: OPENAI_ASPECT_RATIO_PRESETS.map((preset) => preset.value),
    supportedQualities: OPENAI_QUALITY_PRESETS.map((preset) => preset.value),
    supportedOutputFormats: ['png', 'jpeg', 'webp'],
  },
  {
    id: 'gemini-nano-banana-2',
    label: 'Gemini · Nano Banana 2 常用配置',
    providerType: 'GEMINI',
    modelName: 'nano-banana-2',
    displayName: 'Gemini Nano Banana 2',
    supportsGenerate: true,
    supportsEdit: true,
    supportsMultiReference: true,
    supportsN: false,
    maxOutputCount: 1,
    supportedSizes: ['1:1', '2:3', '3:2', '3:4', '4:3', '4:5', '5:4', '9:16', '16:9', '21:9'],
    supportedQualities: ['1k', '2k', '4k'],
    supportedOutputFormats: ['png', 'jpeg', 'webp'],
  },
]

export function labelForSizePreset(value: string): string {
  return MODEL_SIZE_PRESETS.find((preset) => preset.value === value)?.label ?? value
}

export function labelForQualityPreset(value: string): string {
  return MODEL_QUALITY_PRESETS.find((preset) => preset.value === value)?.label ?? value
}
