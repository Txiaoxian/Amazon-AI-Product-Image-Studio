import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { PromptEditor } from '../components/studio/PromptEditor'
import { db } from '../db/dexie'

function PromptEditorHarness() {
  const [prompt, setPrompt] = useState('')

  return <PromptEditor onChange={setPrompt} onError={vi.fn()} value={prompt} />
}

describe('PromptEditor', () => {
  beforeEach(async () => {
    await db.delete()
    await db.open()
  })

  it('fills the prompt textarea when clicking a saved template', async () => {
    const user = userEvent.setup()
    render(<PromptEditorHarness />)

    const textarea = screen.getByLabelText('提示词')
    await user.type(textarea, 'Premium insulated bottle on a clean white Amazon background')
    await user.click(screen.getByRole('button', { name: '保存模板' }))

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
})
