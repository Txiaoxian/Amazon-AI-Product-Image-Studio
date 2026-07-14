import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PromptEditor } from '../components/studio/PromptEditor'
import { db } from '../db/dexie'
import type { WorkbenchImageType } from '../types/workbench'

function PromptEditorHarness({ imageType = 'MAIN' }: { imageType?: WorkbenchImageType }) {
  const [prompt, setPrompt] = useState('')

  return <PromptEditor imageType={imageType} onChange={setPrompt} onError={vi.fn()} value={prompt} />
}

describe('PromptEditor', () => {
  afterEach(cleanup)

  beforeEach(async () => {
    await db.delete()
    await db.open()
  })

  it('fills the prompt textarea when clicking a saved template', async () => {
    const user = userEvent.setup()
    render(<PromptEditorHarness />)

    const textarea = screen.getByLabelText('提示词')
    await user.type(textarea, 'Premium insulated bottle on a clean white Amazon background')
    await user.click(screen.getByRole('button', { name: '保存到主图模板' }))

    const savedTemplate = await screen.findByRole('button', {
      name: /填入模板 Premium insulated bottle/,
    })

    await user.clear(textarea)
    expect(textarea).toHaveValue('')

    await user.click(savedTemplate)

    await waitFor(() => {
      expect(textarea).toHaveValue('Premium insulated bottle on a clean white Amazon background')
    })
  })

  it('shows recommendations for the current image type and keeps the inserted prompt editable', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<PromptEditorHarness imageType="MAIN" />)

    await user.selectOptions(screen.getByLabelText('推荐提示词'), 'main-white-background')

    const textarea = screen.getByLabelText('提示词')
    expect((textarea as HTMLTextAreaElement).value).toContain('纯白背景')
    await user.type(textarea, '\n补充要求：保留产品正面的蓝色按钮。')
    expect((textarea as HTMLTextAreaElement).value).toContain('保留产品正面的蓝色按钮')

    rerender(<PromptEditorHarness imageType="DIMENSION" />)
    expect(screen.getByRole('option', { name: '三视图尺寸标注' })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: '合规纯白主图' })).not.toBeInTheDocument()
  })

  it('keeps saved templates independent for each image type', async () => {
    const user = userEvent.setup()
    const { rerender } = render(<PromptEditorHarness imageType="MAIN" />)

    const textarea = screen.getByLabelText('提示词')
    await user.type(textarea, 'Main image template only')
    await user.click(screen.getByRole('button', { name: '保存到主图模板' }))
    expect(await screen.findByRole('button', { name: /填入模板 Main image template only/ })).toBeInTheDocument()

    rerender(<PromptEditorHarness imageType="A_PLUS" />)
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /填入模板 Main image template only/ })).not.toBeInTheDocument()
    })

    const aPlusTextarea = screen.getByLabelText('提示词')
    await user.clear(aPlusTextarea)
    await user.type(aPlusTextarea, 'A plus template only')
    await user.click(screen.getByRole('button', { name: '保存到A+ 图片模板' }))
    expect(await screen.findByRole('button', { name: /填入模板 A plus template only/ })).toBeInTheDocument()

    rerender(<PromptEditorHarness imageType="MAIN" />)
    expect(await screen.findByRole('button', { name: /填入模板 Main image template only/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /填入模板 A plus template only/ })).not.toBeInTheDocument()
  })

  it('updates a saved template after users edit its content', async () => {
    const user = userEvent.setup()
    render(<PromptEditorHarness imageType="SCENE" />)

    const textarea = screen.getByLabelText('提示词')
    await user.type(textarea, 'Living room scene')
    await user.click(screen.getByRole('button', { name: '保存到场景图模板' }))
    await user.click(await screen.findByRole('button', { name: /填入模板 Living room scene/ }))

    await user.clear(textarea)
    await user.type(textarea, 'Bright kitchen scene')
    await user.click(screen.getByRole('button', { name: '更新场景图模板' }))

    expect(await screen.findByRole('button', { name: /填入模板 Bright kitchen scene/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /填入模板 Living room scene/ })).not.toBeInTheDocument()
  })

  it('opens a centered large editor from inside the textarea and synchronizes changes immediately', async () => {
    const user = userEvent.setup()
    render(<PromptEditorHarness imageType="DETAIL" />)

    const prompt = screen.getByLabelText('提示词')
    await user.type(prompt, '保留产品原始材质。')
    const expandButton = screen.getByRole('button', { name: '放大编辑提示词' })
    expect(prompt.parentElement).toContainElement(expandButton)
    await user.click(expandButton)

    const dialog = screen.getByRole('dialog', { name: '编辑细节图提示词' })
    const expandedPrompt = within(dialog).getByRole('textbox', { name: '完整提示词' })
    expect(expandedPrompt).toHaveFocus()
    expect(expandedPrompt).toHaveValue('保留产品原始材质。')

    await user.clear(expandedPrompt)
    await user.type(expandedPrompt, '第一段：展示产品纹理。\n第二段：保留品牌标识并使用柔和侧光。')
    expect(prompt).toHaveValue('第一段：展示产品纹理。\n第二段：保留品牌标识并使用柔和侧光。')
    expect(within(dialog).queryByRole('button', { name: '取消' })).not.toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: '应用修改' })).not.toBeInTheDocument()

    await user.click(within(dialog).getByRole('button', { name: '关闭弹窗' }))

    expect(screen.queryByRole('dialog', { name: '编辑细节图提示词' })).not.toBeInTheDocument()
    expect(prompt).toHaveValue('第一段：展示产品纹理。\n第二段：保留品牌标识并使用柔和侧光。')
    expect(expandButton).toHaveFocus()
  })

  it('keeps real-time prompt changes when the expanded editor closes with Escape', async () => {
    const user = userEvent.setup()
    render(<PromptEditorHarness />)

    const prompt = screen.getByLabelText('提示词')
    await user.type(prompt, '原始提示词')
    await user.click(screen.getByRole('button', { name: '放大编辑提示词' }))

    const dialog = screen.getByRole('dialog', { name: '编辑主图提示词' })
    const expandedPrompt = within(dialog).getByRole('textbox', { name: '完整提示词' })
    await user.clear(expandedPrompt)
    await user.type(expandedPrompt, '实时同步的修改')
    expect(prompt).toHaveValue('实时同步的修改')
    await user.keyboard('{Escape}')

    expect(screen.queryByRole('dialog', { name: '编辑主图提示词' })).not.toBeInTheDocument()
    expect(prompt).toHaveValue('实时同步的修改')
  })
})
