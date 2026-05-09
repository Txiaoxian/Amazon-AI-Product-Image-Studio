import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SettingsModal } from '../components/modals/SettingsModal'
import { DEFAULT_OPENAI_API_URL, DEFAULT_RELAY2_API_URL } from '../lib/constants'
import { defaultSettings } from '../hooks/useSettings'
import type { AppSettings } from '../providers/types'

function buildSettings(overrides: Partial<AppSettings> = {}): AppSettings {
  return {
    ...defaultSettings,
    providers: {
      openai: {
        apiUrl: 'https://openai-proxy.test/v1',
        apiKey: 'openai-key',
      },
      gemini: {
        apiUrl: 'https://gemini-proxy.test/v1',
        apiKey: 'gemini-key',
      },
      relay2: {
        apiUrl: DEFAULT_RELAY2_API_URL,
        apiKey: '',
      },
    },
    ...overrides,
  }
}

describe('SettingsModal', () => {
  afterEach(() => {
    cleanup()
  })

  it('uses provider-specific default API URLs', () => {
    expect(defaultSettings.providers.openai.apiUrl).toBe(DEFAULT_OPENAI_API_URL)
    expect(defaultSettings.providers.gemini.apiUrl).toBe(DEFAULT_OPENAI_API_URL)
    expect(defaultSettings.providers.relay2.apiUrl).toBe(DEFAULT_RELAY2_API_URL)
  })

  it('edits the shared Tutujin API configuration and saves it to OpenAI and Gemini providers', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()

    render(<SettingsModal isOpen={true} onClose={vi.fn()} onSave={onSave} settings={buildSettings()} />)

    const apiUrlInputs = screen.getAllByLabelText('API URL')
    const apiKeyInputs = screen.getAllByLabelText('API Key')

    expect(apiUrlInputs).toHaveLength(2)
    expect(apiKeyInputs).toHaveLength(2)

    await user.clear(apiUrlInputs[0])
    await user.type(apiUrlInputs[0], 'https://proxy.example.com/v1')
    await user.clear(apiKeyInputs[0])
    await user.type(apiKeyInputs[0], 'shared-key')
    await user.click(screen.getByRole('button', { name: '保存设置' }))

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        providers: {
          openai: {
            apiUrl: 'https://proxy.example.com/v1',
            apiKey: 'shared-key',
          },
          gemini: {
            apiUrl: 'https://proxy.example.com/v1',
            apiKey: 'shared-key',
          },
          relay2: {
            apiUrl: DEFAULT_RELAY2_API_URL,
            apiKey: '',
          },
        },
      }),
    )
  })

  it('edits the secondary relay API configuration independently', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn()

    render(<SettingsModal isOpen={true} onClose={vi.fn()} onSave={onSave} settings={buildSettings()} />)

    const apiUrlInputs = screen.getAllByLabelText('API URL')
    const apiKeyInputs = screen.getAllByLabelText('API Key')

    await user.clear(apiUrlInputs[1])
    await user.type(apiUrlInputs[1], 'https://relay2.example.com/v1')
    await user.clear(apiKeyInputs[1])
    await user.type(apiKeyInputs[1], 'relay-key')
    await user.click(screen.getByRole('button', { name: '保存设置' }))

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        providers: {
          openai: {
            apiUrl: 'https://openai-proxy.test/v1',
            apiKey: 'openai-key',
          },
          gemini: {
            apiUrl: 'https://openai-proxy.test/v1',
            apiKey: 'openai-key',
          },
          relay2: {
            apiUrl: 'https://relay2.example.com/v1',
            apiKey: 'relay-key',
          },
        },
      }),
    )
  })
})
