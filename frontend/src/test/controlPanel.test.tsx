import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ControlPanel } from '../components/studio/ControlPanel'

describe('ControlPanel', () => {
  afterEach(() => {
    cleanup()
  })

  it('defaults image count to 1 and submits the selected image count', async () => {
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

    const promptInput = screen.getByLabelText('提示词')
    const imageCountSelect = screen.getByLabelText('生成张数')

    expect(imageCountSelect).toHaveValue('1')

    await user.type(promptInput, 'Clean Amazon product image')
    await user.selectOptions(imageCountSelect, '4')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(expect.objectContaining({ imageCount: 4 }))
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

  it('offers the Amazon 1.62:1 ratio for OpenAI Image 2', async () => {
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

    expect(screen.queryByRole('option', { name: '970x600' })).not.toBeInTheDocument()

    await user.type(screen.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.selectOptions(screen.getByLabelText('图片比例'), '1.62:1')
    await user.click(screen.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        model: expect.objectContaining({ id: 'openai-gpt-image-2' }),
        aspectRatio: '1.62:1',
      }),
    )
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

  it('shows local reference image upload for secondary relay requests', async () => {
    const user = userEvent.setup()
    const onGenerate = vi.fn().mockResolvedValue(undefined)

    const view = render(
      <ControlPanel
        defaultModelId="relay2-gpt-image-2"
        defaultResolution="1K"
        isGenerating={false}
        onError={vi.fn()}
        onGenerate={onGenerate}
      />,
    )

    const canvas = within(view.container)

    const referenceInput = canvas.getByLabelText('参考图')
    expect(referenceInput).toHaveAttribute('type', 'file')
    expect(canvas.getByText('上传或拖入图片')).toBeInTheDocument()
    expect(canvas.queryByLabelText('参考图 URL')).not.toBeInTheDocument()

    await user.type(canvas.getByLabelText('提示词'), 'Clean Amazon product image')
    await user.click(canvas.getByRole('button', { name: '生成图片' }))

    expect(onGenerate).toHaveBeenCalledWith(
      expect.objectContaining({
        model: expect.objectContaining({ provider: 'relay2', model: 'gpt-image-2' }),
        references: [],
        referenceImageUrls: [],
      }),
    )
  })
})
