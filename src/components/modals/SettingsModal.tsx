import { AlertTriangle, Save } from 'lucide-react'
import { useEffect, useState } from 'react'
import {
  RESOLUTION_LABELS,
  STORAGE_LIMIT_OPTIONS,
  getDefaultResolutionForProvider,
  getResolutionOptionsForProvider,
  isResolutionOption,
} from '../../lib/constants'
import { IMAGE_MODELS, getModelById } from '../../providers/registry'
import type { AppSettings, ImageResolution } from '../../providers/types'
import { Button } from '../ui/Button'
import { Modal } from '../ui/Modal'

interface SettingsModalProps {
  isOpen: boolean
  settings: AppSettings
  onClose: () => void
  onSave: (settings: AppSettings) => void
}

function normalizeDefaultResolution(modelId: string, resolution: ImageResolution): ImageResolution {
  const model = getModelById(modelId)
  const resolutionOptions = getResolutionOptionsForProvider(model.provider)
  return isResolutionOption(resolution, resolutionOptions) ? resolution : getDefaultResolutionForProvider(model.provider)
}

function normalizeSettingsDraft(settings: AppSettings): AppSettings {
  const sharedProviderSettings = settings.providers.openai

  return {
    ...settings,
    defaultResolution: normalizeDefaultResolution(settings.defaultModelId, settings.defaultResolution),
    providers: {
      openai: sharedProviderSettings,
      gemini: sharedProviderSettings,
      relay2: settings.providers.relay2,
    },
  }
}

export function SettingsModal({ isOpen, settings, onClose, onSave }: SettingsModalProps) {
  const [draft, setDraft] = useState(settings)
  const defaultModel = getModelById(draft.defaultModelId)
  const defaultResolutionOptions = getResolutionOptionsForProvider(defaultModel.provider)
  const defaultResolutionLabel = defaultModel.provider === 'gemini' ? '默认质量' : '默认分辨率'

  useEffect(() => {
    if (isOpen) {
      setDraft(normalizeSettingsDraft(settings))
    }
  }, [isOpen, settings])

  const updateSharedProvider = (field: 'apiUrl' | 'apiKey', value: string) => {
    setDraft((current) => ({
      ...current,
      providers: {
        ...current.providers,
        openai: {
          ...current.providers.openai,
          [field]: value,
        },
        gemini: {
          ...current.providers.openai,
          [field]: value,
        },
      },
    }))
  }

  const updateRelay2Provider = (field: 'apiUrl' | 'apiKey', value: string) => {
    setDraft((current) => ({
      ...current,
      providers: {
        ...current.providers,
        relay2: {
          ...current.providers.relay2,
          [field]: value,
        },
      },
    }))
  }

  return (
    <Modal
      footer={
        <div className="flex justify-end gap-2">
          <Button onClick={onClose}>取消</Button>
          <Button
            icon={<Save className="h-4 w-4" />}
            onClick={() => {
              onSave(draft)
              onClose()
            }}
            variant="primary"
          >
            保存设置
          </Button>
        </div>
      }
      isOpen={isOpen}
      onClose={onClose}
      title="设置"
    >
      <div className="space-y-6">
        <div className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-900">
          <div className="flex gap-3">
            <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0" />
            <p>
              本项目为纯前端应用，API Key 仅保存在当前浏览器本地。请勿在公共电脑或不可信设备上使用。
            </p>
          </div>
        </div>

        <section className="grid gap-4 md:grid-cols-2">
          <label className="space-y-2">
            <span className="field-label">默认模型</span>
            <select
              className="field-input"
              id="default-model"
              onChange={(event) => {
                const defaultModelId = event.target.value
                setDraft((current) => ({
                  ...current,
                  defaultModelId,
                  defaultResolution: normalizeDefaultResolution(defaultModelId, current.defaultResolution),
                }))
              }}
              value={draft.defaultModelId}
            >
              {IMAGE_MODELS.map((model) => (
                <option key={model.id} value={model.id}>
                  {model.label}
                </option>
              ))}
            </select>
          </label>

          <label className="space-y-2">
            <span className="field-label">{defaultResolutionLabel}</span>
            <select
              className="field-input"
              id="default-resolution"
              onChange={(event) => setDraft((current) => ({ ...current, defaultResolution: event.target.value as ImageResolution }))}
              value={draft.defaultResolution}
            >
              {defaultResolutionOptions.map((option) => (
                <option key={option} value={option}>
                  {RESOLUTION_LABELS[option]}
                </option>
              ))}
            </select>
          </label>

          <label className="space-y-2 md:col-span-2">
            <span className="field-label">存储上限</span>
            <select
              className="field-input"
              onChange={(event) => setDraft((current) => ({ ...current, storageLimitBytes: Number(event.target.value) }))}
              value={draft.storageLimitBytes}
            >
              {STORAGE_LIMIT_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
        </section>

        <section className="space-y-4">
          <h3 className="text-sm font-semibold text-ink-900">土土金中转站 API</h3>
          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <span className="field-label">API URL</span>
              <input
                className="field-input"
                onChange={(event) => updateSharedProvider('apiUrl', event.target.value)}
                value={draft.providers.openai.apiUrl}
              />
            </label>
            <label className="space-y-2">
              <span className="field-label">API Key</span>
              <input
                className="field-input"
                onChange={(event) => updateSharedProvider('apiKey', event.target.value)}
                type="password"
                value={draft.providers.openai.apiKey}
              />
            </label>
          </div>
        </section>

        <section className="space-y-4">
          <h3 className="text-sm font-semibold text-ink-900">二号中转站 API</h3>
          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <span className="field-label">API URL</span>
              <input
                className="field-input"
                onChange={(event) => updateRelay2Provider('apiUrl', event.target.value)}
                value={draft.providers.relay2.apiUrl}
              />
            </label>
            <label className="space-y-2">
              <span className="field-label">API Key</span>
              <input
                autoComplete="off"
                className="field-input"
                onChange={(event) => updateRelay2Provider('apiKey', event.target.value)}
                type="password"
                value={draft.providers.relay2.apiKey}
              />
            </label>
          </div>
        </section>
      </div>
    </Modal>
  )
}
