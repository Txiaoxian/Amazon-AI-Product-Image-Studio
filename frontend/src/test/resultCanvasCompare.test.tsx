import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { WorkbenchGeneration } from '../hooks/useGeneration'
import { ResultCanvas } from '../components/studio/ResultCanvas'

const handlers = {
  onDownload: vi.fn(),
  onOpenDetail: vi.fn(),
  onRename: vi.fn(),
  onSelect: vi.fn(),
}

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function generation(index: number, type: 'IMAGE_GENERATION' | 'IMAGE_EDIT' = 'IMAGE_GENERATION'): WorkbenchGeneration {
  return {
    kind: 'backend',
    outputIndex: index,
    task: {
      id: 'task_compare',
      projectId: 'project_1',
      type,
    },
    result: {
      assetId: `asset_${index + 1}`,
      height: 1024,
      mimeType: 'image/png',
      previewUrl: `/api/v1/assets/asset_${index + 1}/download`,
      width: 1024,
    },
  } as unknown as WorkbenchGeneration
}

describe('画布双模式对比', () => {
  it('空结果使用同一商品的示例候选，并通过不占布局的文本说明缩略图用途', async () => {
    const user = userEvent.setup()

    render(
      <ResultCanvas
        {...handlers}
        current={null}
        currentItems={[]}
        imageTypeLabel="主图"
        selectedIndex={0}
        status="idle"
        variant="canvas"
      />,
    )

    expect(screen.getByText('示例候选预览')).toBeInTheDocument()
    const candidates = screen.getByRole('group', { name: '其他候选结果' })
    const help = screen.getByText('这些缩略图是同一任务的其他候选结果，点击即可切换到主画布查看。')
    expect(help).toHaveClass('sr-only')

    const coolCandidate = within(candidates).getByRole('button', {
      name: '示例候选 2：冷色展示台，点击切换到主画布查看',
    })
    expect(coolCandidate).toHaveAttribute('title', '示例候选 2：冷色展示台，点击切换到主画布查看')
    expect(within(screen.getByTestId('result-canvas-image')).getByRole('img', { name: '生成结果' }))
      .toHaveAttribute('src', '/studio-assets/demo-bottle-candidate-studio.jpg')

    await user.click(coolCandidate)
    expect(within(screen.getByTestId('result-canvas-image')).getByRole('img', { name: '生成结果' }))
      .toHaveAttribute('src', '/studio-assets/demo-bottle-candidate-cool.jpg')
  })

  it('默认关闭，并支持候选 A/B、交换、重置、键盘滑杆和退出', async () => {
    const user = userEvent.setup()
    const items = [generation(0), generation(1), generation(2)]

    render(
      <ResultCanvas
        {...handlers}
        current={items[0]}
        currentItems={items}
        imageTypeLabel="主图"
        selectedIndex={0}
        status="success"
        variant="canvas"
      />,
    )

    const compareButton = screen.getByRole('button', { name: '对比' })
    expect(compareButton).toHaveAttribute('aria-pressed', 'false')
    expect(screen.queryByRole('toolbar', { name: '图片对比工具' })).not.toBeInTheDocument()

    await user.click(compareButton)
    const toolbar = screen.getByRole('toolbar', { name: '图片对比工具' })
    expect(compareButton).toHaveAttribute('aria-pressed', 'true')
    expect(within(toolbar).getByRole('button', { name: '候选 A/B' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(toolbar).getByRole('button', { name: '原图/结果' })).toBeDisabled()
    expect(screen.getByLabelText('调整对比位置')).toHaveValue('50')

    await user.selectOptions(within(toolbar).getByLabelText('候选 B'), '2')
    expect(document.querySelector('.canvas-compare-label.is-left')).toHaveTextContent('候选 1')
    expect(document.querySelector('.canvas-compare-label.is-right')).toHaveTextContent('候选 3')

    await user.click(within(toolbar).getByRole('button', { name: '交换对比两侧' }))
    expect(document.querySelector('.canvas-compare-label.is-left')).toHaveTextContent('候选 3')
    expect(document.querySelector('.canvas-compare-label.is-right')).toHaveTextContent('候选 1')

    const slider = screen.getByLabelText('调整对比位置')
    slider.focus()
    await user.keyboard('{End}')
    expect(slider).toHaveValue('100')
    await user.click(within(toolbar).getByRole('button', { name: '重置对比位置' }))
    expect(slider).toHaveValue('50')
    slider.focus()
    await user.keyboard('{Home}')
    expect(slider).toHaveValue('0')

    vi.spyOn(slider, 'getBoundingClientRect').mockReturnValue({
      bottom: 700,
      height: 600,
      left: 100,
      right: 1100,
      top: 100,
      width: 1000,
      x: 100,
      y: 100,
      toJSON: () => ({}),
    })
    fireEvent.pointerDown(slider, { clientX: 300, pointerId: 1 })
    fireEvent.pointerMove(slider, { clientX: 850, pointerId: 1 })
    fireEvent.pointerUp(slider, { clientX: 850, pointerId: 1 })
    expect(slider).toHaveValue('75')
    expect(slider).toHaveAttribute('aria-valuetext', '75%')

    await user.click(within(toolbar).getByRole('button', { name: '退出对比' }))
    expect(screen.queryByRole('toolbar', { name: '图片对比工具' })).not.toBeInTheDocument()
    expect(compareButton).toHaveAttribute('aria-pressed', 'false')
  })

  it('编辑任务优先进入原图/结果，并允许切换到候选 A/B', async () => {
    const user = userEvent.setup()
    const items = [generation(0, 'IMAGE_EDIT'), generation(1, 'IMAGE_EDIT')]

    render(
      <ResultCanvas
        {...handlers}
        comparisonSource={{ id: 'source_1', label: '原图', url: '/api/v1/assets/source_1/download' }}
        current={items[0]}
        currentItems={items}
        imageTypeLabel="场景图"
        selectedIndex={0}
        status="success"
        variant="canvas"
      />,
    )

    await user.click(screen.getByRole('button', { name: '对比' }))
    const toolbar = screen.getByRole('toolbar', { name: '图片对比工具' })
    expect(within(toolbar).getByRole('button', { name: '原图/结果' })).toHaveAttribute('aria-pressed', 'true')
    expect(document.querySelector('.canvas-compare-label.is-left')).toHaveTextContent('原图')
    expect(document.querySelector('.canvas-compare-label.is-right')).toHaveTextContent('候选 1')

    await user.click(within(toolbar).getByRole('button', { name: '候选 A/B' }))
    expect(within(toolbar).getByLabelText('候选 A')).toHaveValue('0')
    expect(within(toolbar).getByLabelText('候选 B')).toHaveValue('1')
  })

  it('候选不足且没有原图时禁用对比并显示中文原因', () => {
    const item = generation(0)
    render(
      <ResultCanvas
        {...handlers}
        current={item}
        currentItems={[item]}
        imageTypeLabel="主图"
        selectedIndex={0}
        status="success"
        variant="canvas"
      />,
    )

    expect(screen.getByRole('button', { name: '对比' })).toBeDisabled()
    expect(screen.getByText('至少需要两张候选图，或为编辑任务提供原图后才能对比。')).toBeInTheDocument()
  })
})
