import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { useState } from 'react'
import { EditorDrawer } from '../components/ui/EditorDrawer'
import { Modal } from '../components/ui/Modal'

function ModalHarness() {
  const [isOpen, setOpen] = useState(false)

  return (
    <>
      <button onClick={() => setOpen(true)} type="button">
        打开测试弹窗
      </button>
      <Modal isOpen={isOpen} onClose={() => setOpen(false)} title="测试弹窗">
        <button type="button">次要操作</button>
        <input aria-label="名称" />
      </Modal>
    </>
  )
}

function NestedModalHarness() {
  const [isOuterOpen, setOuterOpen] = useState(false)
  const [isInnerOpen, setInnerOpen] = useState(false)

  return (
    <>
      <button onClick={() => setOuterOpen(true)} type="button">
        打开管理弹窗
      </button>
      <Modal isOpen={isOuterOpen} onClose={() => setOuterOpen(false)} title="管理弹窗">
        <button onClick={() => setInnerOpen(true)} type="button">
          查看调用详情
        </button>
        <Modal isOpen={isInnerOpen} onClose={() => setInnerOpen(false)} title="调用详情">
          <button type="button">详情操作</button>
        </Modal>
      </Modal>
    </>
  )
}

function EditorDrawerHarness() {
  const [isOpen, setOpen] = useState(false)

  return (
    <>
      <button onClick={() => setOpen(true)} type="button">
        编辑模型
      </button>
      <EditorDrawer isOpen={isOpen} onClose={() => setOpen(false)} title="模型配置">
        <input aria-label="模型名称" />
        <button type="button">保存模型</button>
      </EditorDrawer>
    </>
  )
}

describe('Modal accessibility', () => {
  afterEach(() => {
    cleanup()
  })

  it('keeps keyboard focus inside the dialog and restores it after Escape closes', async () => {
    const user = userEvent.setup()
    render(<ModalHarness />)

    const trigger = screen.getByRole('button', { name: '打开测试弹窗' })
    await user.click(trigger)

    const closeButton = screen.getByRole('button', { name: '关闭弹窗' })
    expect(closeButton).toHaveFocus()

    await user.tab({ shift: true })
    expect(screen.getByRole('textbox', { name: '名称' })).toHaveFocus()

    await user.tab()
    expect(closeButton).toHaveFocus()

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('dialog', { name: '测试弹窗' })).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('closes only the top dialog when nested dialogs receive Escape', async () => {
    const user = userEvent.setup()
    render(<NestedModalHarness />)

    await user.click(screen.getByRole('button', { name: '打开管理弹窗' }))
    const detailTrigger = screen.getByRole('button', { name: '查看调用详情' })
    await user.click(detailTrigger)
    expect(screen.getByRole('dialog', { name: '调用详情' })).toBeInTheDocument()

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('dialog', { name: '调用详情' })).not.toBeInTheDocument()
    expect(screen.getByRole('dialog', { name: '管理弹窗' })).toBeInTheDocument()
    expect(detailTrigger).toHaveFocus()
  })

  it('keeps a long editor in an independent drawer and restores the list trigger focus', async () => {
    const user = userEvent.setup()
    render(<EditorDrawerHarness />)

    const trigger = screen.getByRole('button', { name: '编辑模型' })
    await user.click(trigger)

    const drawer = screen.getByRole('dialog', { name: '模型配置' })
    expect(drawer).toHaveClass('overflow-hidden')
    expect(screen.getByTestId('editor-drawer-body')).toHaveClass('overflow-y-auto')
    expect(screen.getByRole('button', { name: '关闭编辑面板' })).toHaveFocus()

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('dialog', { name: '模型配置' })).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })
})
