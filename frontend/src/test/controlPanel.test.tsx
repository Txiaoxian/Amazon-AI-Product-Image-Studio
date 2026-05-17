import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ControlPanel } from '../components/studio/ControlPanel'
import type { Model } from '../types/platform'
import type { WorkbenchReferenceInput } from '../types/workbench'

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

  it('keeps the current visible legacy parameters aligned with the submitted request', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)

    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        onError={vi.fn()}
        onGenerate={onGenerate}
      />,
    )

    expect(screen.getByLabelText('模型')).toHaveValue('openai-gpt-image-2')
    expect(screen.getByLabelText('分辨率')).toHaveValue('1K')
    expect(screen.getByLabelText('图片比例')).toHaveValue('1:1')
    expect(screen.getByLabelText('生成张数')).toHaveValue('1')

    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.selectOptions(screen.getByLabelText('分辨率'), '2K')
    await user.selectOptions(screen.getByLabelText('图片比例'), '1.62:1')
    await user.selectOptions(screen.getByLabelText('生成张数'), '4')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        model: expect.objectContaining({ id: 'openai-gpt-image-2' }),
        quality: '2K',
        aspectRatio: '1.62:1',
        imageCount: 4,
      }),
    )
  })

  it('orders OpenAI image ratios by the width-side number', () => {
    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        onError={vi.fn()}
        onGenerate={vi.fn().mockResolvedValue(undefined)}
      />,
    )

    const ratioSelect = screen.getByLabelText('图片比例') as HTMLSelectElement

    expect(Array.from(ratioSelect.options).map((option) => option.value)).toEqual([
      '1:1',
      '1:4',
      '1:8',
      '1.62:1',
      '2:3',
      '3:2',
      '3:4',
      '4:1',
      '4:3',
      '4:5',
      '5:4',
      '8:1',
      '9:16',
      '16:9',
      '21:9',
    ])
  })

  it('uses OpenAI auto resolution but normalizes parameters when switching to Nano Banana 2', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)

    const view = render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="auto"
        isGenerating={false}
        onError={vi.fn()}
        onGenerate={onGenerate}
      />,
    )

    const canvas = within(view.container)
    const promptInput = canvas.getByRole('textbox')
    const [modelSelect, resolutionSelect] = canvas.getAllByRole('combobox')

    expect(resolutionSelect).toHaveValue('auto')
    expect(canvas.getByRole('option', { name: '自动' })).toBeInTheDocument()

    await user.selectOptions(modelSelect, 'gemini-nano-banana-2')

    expect(canvas.getByLabelText('质量')).toHaveValue('auto')
    expect(canvas.getByLabelText('尺寸')).toHaveValue('1024x1024')
    expect(canvas.getByLabelText('参考图 URL')).toBeInTheDocument()

    await user.type(promptInput, 'Clean Amazon product image')
    await user.type(canvas.getByLabelText('参考图 URL'), 'https://cdn.example.com/ref.png')
    await user.click(canvas.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        model: expect.objectContaining({ provider: 'gemini' }),
        quality: 'auto',
        aspectRatio: '1024x1024',
        references: [],
        referenceImageUrls: ['https://cdn.example.com/ref.png'],
      }),
    )
  })

  it('submits project assets as real reference files on the current legacy path', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)
    const legacyFile = new File(['asset-bytes'], 'reference.png', { type: 'image/png' })

    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        onError={vi.fn()}
        onGenerate={onGenerate}
        referenceToAdd={createAssetReference(legacyFile)}
      />,
    )

    expect(await screen.findByAltText('reference.png')).toBeInTheDocument()

    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        references: [legacyFile],
      }),
    )
  })

  it('keeps legacy history drafts editable and submittable before history migration', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)
    const referenceFile = new File(['history-bytes'], 'reference-history.png', { type: 'image/png' })

    render(
      <ControlPanel
        defaultModelId="openai-gpt-image-2"
        defaultResolution="1K"
        draft={{
          prompt: 'Edit from history',
          modelId: 'relay2-gpt-image-2',
          quality: '2K',
          aspectRatio: '16:9',
          imageCount: 1,
          references: [
            {
              kind: 'pending',
              file: referenceFile,
              previewUrl: 'blob:history-reference',
            },
          ],
        }}
        isGenerating={false}
        onError={vi.fn()}
        onGenerate={onGenerate}
      />,
    )

    expect(screen.getByLabelText('模型')).toHaveValue('relay2-gpt-image-2')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '生成图片' })).toBeEnabled()

    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: 'Edit from history',
        model: expect.objectContaining({ id: 'relay2-gpt-image-2' }),
        quality: '2K',
        aspectRatio: '16:9',
        references: [referenceFile],
      }),
    )
  })

  it('renders backend model capabilities and prepares backend task input when explicitly enabled', async () => {
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
        submissionMode="backend"
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
        submissionMode="backend"
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

  it('keeps project asset IDs as the stable backend task input state', async () => {
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
        referenceToAdd={createAssetReference(new File(['asset-bytes'], 'reference.png', { type: 'image/png' }))}
        submissionMode="backend"
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
        submissionMode="backend"
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
        submissionMode="backend"
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

function createAssetReference(legacyFile: File): WorkbenchReferenceInput {
  return {
    kind: 'asset',
    assetId: 'asset_1' as never,
    filename: 'reference.png',
    previewUrl: 'blob:reference-asset',
    legacyFile,
  }
}
