import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ControlPanel } from '../components/studio/ControlPanel'
import type { Model } from '../types/platform'

const imageModel: Model = {
  id: 'model_image' as Model['id'],
  tenantId: 'tenant_1' as Model['tenantId'],
  providerId: 'provider_1' as Model['providerId'],
  providerName: 'Studio Provider',
  modelName: 'image-model',
  displayName: 'Image Model',
  supportsGenerate: true,
  supportsEdit: true,
  supportsMultiReference: true,
  supportsN: true,
  maxOutputCount: 4,
  supportedSizes: ['1024x1024', '1536x1024'],
  supportedQualities: ['standard', 'hd'],
  supportedOutputFormats: ['png', 'jpeg'],
  pricing: {
    currency: 'USD',
    unitPrices: {
      image: 0.04,
    },
  },
  status: 'ENABLED',
  createdAt: '2026-05-13T00:00:00Z',
  updatedAt: '2026-05-13T00:01:00Z',
}

const basicModel: Model = {
  ...imageModel,
  id: 'model_basic' as Model['id'],
  displayName: 'Basic Model',
  supportsEdit: false,
  supportsMultiReference: false,
  supportsN: false,
  maxOutputCount: 1,
  supportedSizes: ['1024x1024'],
  supportedQualities: [],
  supportedOutputFormats: [],
}

describe('ControlPanel', () => {
  afterEach(() => {
    cleanup()
  })

  it('renders backend model capabilities and prepares backend task input', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)

    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        modelStatus="success"
        models={[imageModel]}
        onError={vi.fn()}
        onGenerate={onGenerate}
        onRefreshModels={vi.fn()}
      />,
    )

    expect(screen.getByLabelText('模型')).toHaveValue('model_image')
    expect(screen.getByRole('option', { name: 'Image Model · Studio Provider' })).toBeInTheDocument()
    expect(screen.getByLabelText('尺寸')).toHaveValue('1024x1024')
    expect(screen.getByLabelText('质量')).toHaveValue('standard')
    expect(screen.getByLabelText('输出格式')).toHaveValue('png')
    expect(screen.getByLabelText('生成张数')).toHaveValue('1')
    expect(screen.getByText(/支持多张参考图/)).toBeInTheDocument()

    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.selectOptions(screen.getByLabelText('尺寸'), '1536x1024')
    await user.selectOptions(screen.getByLabelText('质量'), 'hd')
    await user.selectOptions(screen.getByLabelText('输出格式'), 'jpeg')
    await user.selectOptions(screen.getByLabelText('生成张数'), '4')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        imageCount: 4,
      }),
      {
        providerId: 'provider_1',
        modelId: 'model_image',
        referenceAssetIds: [],
        parameters: {
          size: '1536x1024',
          quality: 'hd',
          outputFormat: 'jpeg',
          outputCount: 4,
        },
      },
    )
  })

  it('disables unsupported parameter combinations from backend capabilities', async () => {
    const user = userEvent.setup()

    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        modelStatus="success"
        models={[imageModel, basicModel]}
        onError={vi.fn()}
        onGenerate={vi.fn().mockResolvedValue(undefined)}
        onRefreshModels={vi.fn()}
      />,
    )

    await user.selectOptions(screen.getByLabelText('模型'), 'model_basic')

    expect(screen.getByLabelText('尺寸')).toHaveValue('1024x1024')
    expect(screen.queryByLabelText('质量')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('输出格式')).not.toBeInTheDocument()
    expect(screen.getByLabelText('生成张数')).toBeDisabled()
    expect(screen.queryByText('支持多张参考图')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('参考图')).not.toBeInTheDocument()
  })

  it('keeps project asset IDs as the stable reference submission state', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)

    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        modelStatus="success"
        models={[imageModel]}
        onError={vi.fn()}
        onGenerate={onGenerate}
        onRefreshModels={vi.fn()}
        referenceToAdd={{
          kind: 'asset',
          assetId: 'asset_1' as never,
          filename: 'reference.png',
          previewUrl: '/api/v1/assets/asset_1/download',
        }}
      />,
    )

    expect(await screen.findByAltText('reference.png')).toBeInTheDocument()

    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.any(Object),
      expect.objectContaining({
        referenceAssetIds: ['asset_1'],
      }),
    )
  })

  it('requires refresh and reselection when the selected backend model becomes unavailable', async () => {
    const user = userEvent.setup()
    const onRefreshModels = vi.fn()
    const view = render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        modelStatus="success"
        models={[imageModel]}
        onError={vi.fn()}
        onGenerate={vi.fn().mockResolvedValue(undefined)}
        onRefreshModels={onRefreshModels}
      />,
    )

    view.rerender(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        modelStatus="success"
        models={[basicModel]}
        onError={vi.fn()}
        onGenerate={vi.fn().mockResolvedValue(undefined)}
        onRefreshModels={onRefreshModels}
      />,
    )

    expect(screen.getByRole('alert')).toHaveTextContent('所选模型当前不可用')
    expect(screen.getByRole('button', { name: '生成图片' })).toBeDisabled()

    await user.click(screen.getByRole('button', { name: '刷新模型' }))
    expect(onRefreshModels).toHaveBeenCalledTimes(1)

    await user.selectOptions(screen.getByLabelText('模型'), 'model_basic')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
