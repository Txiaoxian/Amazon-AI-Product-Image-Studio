import { describe, expect, it } from 'vitest'
import {
  MODEL_CAPABILITY_TEMPLATES,
  labelForQualityPreset,
  modelParameterPresetsForProvider,
  parameterLabelsForCapabilities,
} from '../lib/modelCapabilityPresets'

describe('model capability presets', () => {
  it('uses GPT Image 2 resolution tiers and the configured aspect-ratio order', () => {
    const openAI = modelParameterPresetsForProvider('OPENAI_COMPATIBLE', 'gpt-image-2')
    const gemini = modelParameterPresetsForProvider('GEMINI')

    expect(openAI.sizeLabel).toBe('图片比例')
    expect(openAI.sizePresets.map((preset) => preset.value)).toEqual([
      'auto',
      '1:1',
      '1.62:1',
      '2:3',
      '3:2',
      '3:4',
      '4:3',
      '4:5',
      '5:4',
      '9:16',
      '16:9',
      '21:9',
    ])
    expect(openAI.qualityLabel).toBe('生成质量')
    expect(openAI.qualityPresets.map(({ value, label }) => ({ value, label }))).toEqual([
      { value: 'auto', label: '自动' },
      { value: 'low', label: '低质量（1K）' },
      { value: 'medium', label: '中等质量（2K）' },
      { value: 'high', label: '高质量（4K）' },
    ])
    expect(labelForQualityPreset('high', { modelName: 'gpt-image-2-2026-04-21' })).toBe('高质量（4K）')
    expect(labelForQualityPreset('high', { modelName: 'gpt-image-1' })).toBe('高质量')

    expect(gemini.sizeLabel).toBe('画面比例')
    expect(gemini.sizePresets.map((preset) => preset.value)).toEqual(
      expect.arrayContaining(['1:1', '4:5', '16:9']),
    )
    expect(gemini.qualityLabel).toBe('输出分辨率')
    expect(gemini.qualityPresets.map((preset) => preset.value)).toEqual(['1k', '2k', '4k'])
  })

  it('uses the same official capability template for direct and compatible gpt-image-2 models', () => {
    const compatibleTemplate = MODEL_CAPABILITY_TEMPLATES.find(
      (candidate) => candidate.id === 'openai-compatible-gpt-image-2-euzhi',
    )
    const officialTemplate = MODEL_CAPABILITY_TEMPLATES.find(
      (candidate) => candidate.id === 'openai-gpt-image-2',
    )

    expect(compatibleTemplate).toMatchObject({
      providerType: 'OPENAI_COMPATIBLE',
      modelName: 'gpt-image-2',
      supportsEdit: true,
      supportsMultiReference: true,
      supportsN: false,
      maxOutputCount: 1,
      supportedSizes: ['auto', '1:1', '1.62:1', '2:3', '3:2', '3:4', '4:3', '4:5', '5:4', '9:16', '16:9', '21:9'],
      supportedQualities: ['auto', 'low', 'medium', 'high'],
      supportedOutputFormats: ['png'],
    })
    expect(officialTemplate?.supportedSizes).toEqual(compatibleTemplate?.supportedSizes)
    expect(officialTemplate?.supportedQualities).toEqual(compatibleTemplate?.supportedQualities)
  })

  it('stores Gemini ratios independently from 1K, 2K, and 4K resolution choices', () => {
    const template = MODEL_CAPABILITY_TEMPLATES.find(
      (candidate) => candidate.id === 'gemini-nano-banana-2',
    )

    expect(template).toMatchObject({
      providerType: 'GEMINI',
      supportedSizes: ['1:1', '2:3', '3:2', '3:4', '4:3', '4:5', '5:4', '9:16', '16:9', '21:9'],
      supportedQualities: ['1k', '2k', '4k'],
    })
  })

  it('uses the same parameter names in generation as the configured capability semantics', () => {
    expect(parameterLabelsForCapabilities(['16:9'], ['medium'], 'OPENAI_COMPATIBLE')).toEqual({
      sizeLabel: '图片比例',
      qualityLabel: '生成质量',
    })
    expect(parameterLabelsForCapabilities(['16:9'], ['2k'])).toEqual({
      sizeLabel: '画面比例',
      qualityLabel: '输出分辨率',
    })
    expect(parameterLabelsForCapabilities(['1.62:1'], ['high'], 'OPENAI')).toEqual({
      sizeLabel: '图片比例',
      qualityLabel: '生成质量',
    })
  })
})
