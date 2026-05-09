import { useCallback, useMemo, useState } from 'react'
import {
  DEFAULT_OPENAI_API_URL,
  DEFAULT_RELAY2_API_URL,
  LEGACY_TUTUJIN_APP_API_URL,
  LEGACY_OPENAI_API_URL,
  RELAY2_PROXY_API_URL,
  DEFAULT_STORAGE_LIMIT_BYTES,
  RESOLUTION_OPTIONS,
  getDefaultResolutionForProvider,
  getResolutionOptionsForProvider,
  isResolutionOption,
} from '../lib/constants'
import { DEFAULT_MODEL_ID, getModelById } from '../providers/registry'
import type { AppSettings, ProviderSettings } from '../providers/types'

const SETTINGS_KEY = 'amazon-ai-product-image-studio.settings'
const LEGACY_RELAY106_API_URL = '/relay106'
const FLYMUX_API_HOST = 'api.flymux.com'
const SECONDARY_RELAY_DIRECT_API_HOSTS = ['106.75.146.14', 'api.flymux.com']

type PersistedSettings = Partial<Omit<AppSettings, 'providers'>> & {
  providers?: Partial<AppSettings['providers']> & {
    relay106?: ProviderSettings
  }
}

export const defaultSettings: AppSettings = {
  defaultModelId: DEFAULT_MODEL_ID,
  defaultResolution: RESOLUTION_OPTIONS[0],
  storageLimitBytes: DEFAULT_STORAGE_LIMIT_BYTES,
  providers: {
    openai: {
      apiUrl: DEFAULT_OPENAI_API_URL,
      apiKey: '',
    },
    gemini: {
      apiUrl: DEFAULT_OPENAI_API_URL,
      apiKey: '',
    },
    relay2: {
      apiUrl: DEFAULT_RELAY2_API_URL,
      apiKey: '',
    },
  },
}

function normalizeConfiguredApiUrl(apiUrl: string, fallbackApiUrl = DEFAULT_OPENAI_API_URL): string {
  const trimmedApiUrl = apiUrl.trim().replace(/\/+$/, '')

  if (!trimmedApiUrl || trimmedApiUrl === LEGACY_OPENAI_API_URL) {
    return fallbackApiUrl
  }

  try {
    const url = new URL(trimmedApiUrl)

    if (url.hostname === 'api.tutujin.app') {
      url.hostname = 'api.tutujin.com'
      return url.toString().replace(/\/+$/, '')
    }
  } catch {
    if (trimmedApiUrl === LEGACY_TUTUJIN_APP_API_URL) {
      return fallbackApiUrl
    }
  }

  return trimmedApiUrl
}

function normalizeSecondaryRelayApiUrl(apiUrl: string): string {
  const normalizedApiUrl = normalizeConfiguredApiUrl(apiUrl, DEFAULT_RELAY2_API_URL)

  if (normalizedApiUrl === LEGACY_RELAY106_API_URL || normalizedApiUrl === RELAY2_PROXY_API_URL) {
    return DEFAULT_RELAY2_API_URL
  }

  try {
    const url = new URL(normalizedApiUrl)

    if (url.hostname === FLYMUX_API_HOST && ['', '/', '/v1'].includes(url.pathname)) {
      return DEFAULT_RELAY2_API_URL
    }

    if (SECONDARY_RELAY_DIRECT_API_HOSTS.includes(url.hostname) && url.hostname !== FLYMUX_API_HOST) {
      return DEFAULT_RELAY2_API_URL
    }
  } catch {
    return normalizedApiUrl
  }

  return normalizedApiUrl
}

function normalizeSharedProviderSettings(settings: AppSettings): AppSettings {
  const sharedApiUrl = normalizeConfiguredApiUrl(settings.providers.openai.apiUrl)
  const sharedProviderSettings = {
    ...settings.providers.openai,
    apiUrl: sharedApiUrl,
  }
  const relay2ProviderSettings = {
    ...settings.providers.relay2,
    apiUrl: normalizeSecondaryRelayApiUrl(settings.providers.relay2.apiUrl),
  }

  return {
    ...settings,
    providers: {
      openai: sharedProviderSettings,
      gemini: sharedProviderSettings,
      relay2: relay2ProviderSettings,
    },
  }
}

function normalizeSettings(settings: AppSettings): AppSettings {
  const sharedSettings = normalizeSharedProviderSettings(settings)
  const model = getModelById(sharedSettings.defaultModelId)
  const resolutionOptions = getResolutionOptionsForProvider(model.provider)

  if (isResolutionOption(sharedSettings.defaultResolution, resolutionOptions)) {
    return sharedSettings
  }

  return {
    ...sharedSettings,
    defaultResolution: getDefaultResolutionForProvider(model.provider),
  }
}

function loadSettings(): AppSettings {
  const raw = localStorage.getItem(SETTINGS_KEY)

  if (!raw) {
    return defaultSettings
  }

  try {
    const parsed = JSON.parse(raw) as PersistedSettings
    const persistedRelay2Settings = parsed.providers?.relay2 ?? parsed.providers?.relay106

    return normalizeSettings({
      ...defaultSettings,
      ...parsed,
      providers: {
        openai: {
          ...defaultSettings.providers.openai,
          ...parsed.providers?.openai,
        },
        gemini: {
          ...defaultSettings.providers.gemini,
          ...parsed.providers?.gemini,
        },
        relay2: {
          ...defaultSettings.providers.relay2,
          ...persistedRelay2Settings,
        },
      },
    })
  } catch {
    return defaultSettings
  }
}

export function useSettings() {
  const [settings, setSettingsState] = useState<AppSettings>(() => loadSettings())

  const setSettings = useCallback((nextSettings: AppSettings) => {
    const normalizedSettings = normalizeSettings(nextSettings)
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(normalizedSettings))
    setSettingsState(normalizedSettings)
  }, [])

  const updateSettings = useCallback(
    (updater: (current: AppSettings) => AppSettings) => {
      setSettingsState((current) => {
        const nextSettings = normalizeSettings(updater(current))
        localStorage.setItem(SETTINGS_KEY, JSON.stringify(nextSettings))
        return nextSettings
      })
    },
    [],
  )

  const defaultModel = useMemo(() => getModelById(settings.defaultModelId), [settings.defaultModelId])

  return {
    settings,
    setSettings,
    updateSettings,
    defaultModel,
  }
}
