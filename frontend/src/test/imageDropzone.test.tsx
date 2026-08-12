import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ImageDropzone } from '../components/studio/ImageDropzone'
import type { AssetId } from '../types/platform'
import type { WorkbenchReferenceInput } from '../types/workbench'

const availableReferences: WorkbenchReferenceInput[] = [
  { assetId: 'asset_1' as AssetId, filename: 'first.png', kind: 'asset', previewUrl: '/first.png' },
  { assetId: 'asset_2' as AssetId, filename: 'second.png', kind: 'asset', previewUrl: '/second.png' },
]

function ImageDropzoneHarness() {
  const [references, setReferences] = useState<WorkbenchReferenceInput[]>([])

  return (
    <ImageDropzone
      availableReferences={availableReferences}
      onChange={setReferences}
      onError={vi.fn()}
      references={references}
      variant="canvas"
    />
  )
}

describe('ImageDropzone canvas variant', () => {
  afterEach(cleanup)

  it('keeps reference cards in the product asset order after selecting one', async () => {
    const user = userEvent.setup()
    render(<ImageDropzoneHarness />)

    await user.click(screen.getByRole('button', { name: '选择产品参考图 second.png' }))

    const firstCard = screen.getByRole('button', { name: '选择产品参考图 first.png' })
    const secondCard = screen.getByRole('button', { name: '取消选择产品参考图 second.png' })
    expect(within(firstCard).getByRole('img')).toHaveAttribute('src', '/first.png')
    expect(within(secondCard).getByRole('img')).toHaveAttribute('src', '/second.png')
  })
})
