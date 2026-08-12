import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { TemplateLibraryPage } from '../components/templates/TemplateLibraryPage'
import { db } from '../db/dexie'
import type { ProjectId } from '../types/platform'

describe('TemplateLibraryPage', () => {
  afterEach(() => {
    cleanup()
    vi.unstubAllGlobals()
  })

  beforeEach(async () => {
    await db.delete()
    await db.open()
  })

  it('supports adding, searching, editing and deleting a current-product template', async () => {
    const user = userEvent.setup()
    vi.stubGlobal('confirm', vi.fn(() => true))
    render(<TemplateLibraryPage onNotice={vi.fn()} onUseTemplate={vi.fn()} projectId={'project_1' as ProjectId} />)

    await user.click(screen.getByRole('button', { name: '新增提示词模板' }))
    const createDialog = screen.getByRole('dialog', { name: '新增提示词模板' })
    await user.type(within(createDialog).getByLabelText('模板名称'), '厨房自然光模板')
    await user.type(within(createDialog).getByLabelText('提示词'), '使用厨房自然光，保留产品真实材质。')
    await user.click(within(createDialog).getByRole('button', { name: '添加模板' }))

    expect(await screen.findByRole('heading', { name: '厨房自然光模板' })).toBeInTheDocument()

    await user.type(screen.getByRole('textbox', { name: '搜索模板' }), '厨房自然光')
    expect(screen.getByRole('heading', { name: '厨房自然光模板' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '编辑模板 厨房自然光模板' }))
    const editDialog = screen.getByRole('dialog', { name: '编辑提示词模板' })
    const title = within(editDialog).getByLabelText('模板名称')
    const prompt = within(editDialog).getByLabelText('提示词')
    await user.clear(title)
    await user.type(title, '暖色厨房模板')
    await user.clear(prompt)
    await user.type(prompt, '使用暖色厨房自然光，保留产品真实材质。')
    await user.click(within(editDialog).getByRole('button', { name: '保存修改' }))

    expect(await screen.findByRole('heading', { name: '暖色厨房模板' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '删除模板 暖色厨房模板' }))
    expect(await screen.findByText('当前产品还没有符合条件的模板，可点击右上角新增。')).toBeInTheDocument()
  })
})
