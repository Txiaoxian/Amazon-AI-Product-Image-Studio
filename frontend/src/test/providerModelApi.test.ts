import { describe, expect, it, vi } from 'vitest'
import { createApiClient } from '../api/client'
import { createModelApi } from '../api/models'
import { createProviderApi } from '../api/providers'

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_provider_model_api',
    }),
    { status },
  )
}

const provider = {
  id: 'provider_1',
  tenantId: 'tenant_1',
  type: 'OPENAI_COMPATIBLE',
  name: 'Secure Gateway',
  baseUrl: 'https://provider.example/v1',
  status: 'ENABLED',
  timeoutSeconds: 30,
  concurrencyLimit: 2,
  apiKeyHint: '****1234',
  apiKeyUpdatedAt: '2026-05-13T00:00:00Z',
  lastTestStatus: 'SUCCESS',
  lastTestedAt: '2026-05-13T00:01:00Z',
  createdAt: '2026-05-13T00:00:00Z',
  updatedAt: '2026-05-13T00:01:00Z',
}

const model = {
  id: 'model_1',
  tenantId: 'tenant_1',
  providerId: 'provider_1',
  providerName: 'Secure Gateway',
  modelName: 'image-model',
  displayName: 'Image Model',
  supportsGenerate: true,
  supportsEdit: true,
  supportsMultiReference: true,
  supportsN: true,
  maxOutputCount: 4,
  supportedSizes: ['1024x1024', '1536x1024'],
  supportedQualities: ['standard', 'hd'],
  supportedOutputFormats: ['png', 'jpeg'],
  pricing: {
    currency: 'USD',
    unitPrices: {
      image: 0.04,
    },
  },
  status: 'ENABLED',
  createdAt: '2026-05-13T00:00:00Z',
  updatedAt: '2026-05-13T00:01:00Z',
}

describe('Provider and model API wrappers', () => {
  it('uses the authenticated API client and CSRF header for Provider management', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse({
          records: [provider],
          total: 1,
          pageNum: 1,
          pageSize: 20,
        }),
      )
      .mockResolvedValueOnce(jsonResponse(provider, 201))
      .mockResolvedValueOnce(jsonResponse({ ...provider, name: 'Secure Gateway Rotated' }))
      .mockResolvedValueOnce(jsonResponse({ ...provider, status: 'DISABLED' }))
      .mockResolvedValueOnce(jsonResponse({ ...provider, status: 'ENABLED' }))
      .mockResolvedValueOnce(
        jsonResponse({
          status: 'SUCCESS',
          durationMs: 42,
          checkedAt: '2026-05-13T00:02:00Z',
          httpStatus: 200,
          requestId: 'provider-request-1',
          message: 'Provider test succeeded.',
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
    const providerApi = createProviderApi(createApiClient({ fetchImpl }))

    await expect(providerApi.list({ type: 'OPENAI_COMPATIBLE', status: 'ENABLED', pageNum: 1, pageSize: 20 })).resolves.toMatchObject({
      records: [provider],
      total: 1,
    })
    await providerApi.create(
      {
        type: 'OPENAI_COMPATIBLE',
        name: 'Secure Gateway',
        baseUrl: 'https://provider.example/v1',
        apiKey: 'one-time-secret-1234',
        timeoutSeconds: 30,
        concurrencyLimit: 2,
        status: 'ENABLED',
      },
      'csrf_memory_only',
    )
    await providerApi.update(
      'provider_1',
      {
        name: 'Secure Gateway Rotated',
      },
      'csrf_memory_only',
    )
    await providerApi.disable('provider_1', 'csrf_memory_only')
    await providerApi.enable('provider_1', 'csrf_memory_only')
    await providerApi.test('provider_1', 'csrf_memory_only')
    await providerApi.delete('provider_1', 'csrf_memory_only')

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/providers?type=OPENAI_COMPATIBLE&status=ENABLED&pageNum=1&pageSize=20',
      '/api/v1/providers',
      '/api/v1/providers/provider_1',
      '/api/v1/providers/provider_1/disable',
      '/api/v1/providers/provider_1/enable',
      '/api/v1/providers/provider_1/test',
      '/api/v1/providers/provider_1',
    ])
    expect(fetchImpl.mock.calls[0][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect(fetchImpl.mock.calls[1][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'POST' }))
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toEqual({
      type: 'OPENAI_COMPATIBLE',
      name: 'Secure Gateway',
      baseUrl: 'https://provider.example/v1',
      apiKey: 'one-time-secret-1234',
      timeoutSeconds: 30,
      concurrencyLimit: 2,
      status: 'ENABLED',
    })
    expect(JSON.parse(fetchImpl.mock.calls[2][1]?.body as string)).toEqual({
      name: 'Secure Gateway Rotated',
    })
    for (const index of [2, 3, 4, 5, 6]) {
      expect((fetchImpl.mock.calls[index][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    }
  })

  it('uses the authenticated API client and CSRF header for model capability management', async () => {
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse({
          records: [model],
          total: 1,
          pageNum: 1,
          pageSize: 20,
        }),
      )
      .mockResolvedValueOnce(jsonResponse(model, 201))
      .mockResolvedValueOnce(jsonResponse({ ...model, displayName: 'Image Model Updated' }))
      .mockResolvedValueOnce(jsonResponse({ ...model, status: 'DISABLED' }))
      .mockResolvedValueOnce(jsonResponse({ ...model, status: 'ENABLED' }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
    const modelApi = createModelApi(createApiClient({ fetchImpl }))

    await expect(
      modelApi.list({
        providerId: 'provider_1',
        status: 'ENABLED',
        capability: 'generate',
        pageNum: 1,
        pageSize: 20,
      }),
    ).resolves.toMatchObject({
      records: [model],
      total: 1,
    })
    await modelApi.create(
      {
        providerId: 'provider_1',
        modelName: 'image-model',
        displayName: 'Image Model',
        supportsGenerate: true,
        supportsEdit: true,
        supportsMultiReference: true,
        supportsN: true,
        maxOutputCount: 4,
        supportedSizes: ['1024x1024', '1536x1024'],
        supportedQualities: ['standard', 'hd'],
        supportedOutputFormats: ['png', 'jpeg'],
        pricing: {
          currency: 'USD',
          unitPrices: {
            image: 0.04,
          },
        },
        status: 'ENABLED',
      },
      'csrf_memory_only',
    )
    await modelApi.update('model_1', { displayName: 'Image Model Updated' }, 'csrf_memory_only')
    await modelApi.disable('model_1', 'csrf_memory_only')
    await modelApi.enable('model_1', 'csrf_memory_only')
    await modelApi.delete('model_1', 'csrf_memory_only')

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/models?providerId=provider_1&status=ENABLED&capability=generate&pageNum=1&pageSize=20',
      '/api/v1/models',
      '/api/v1/models/model_1',
      '/api/v1/models/model_1/disable',
      '/api/v1/models/model_1/enable',
      '/api/v1/models/model_1',
    ])
    expect(fetchImpl.mock.calls[0][1]).toEqual(expect.objectContaining({ credentials: 'include', method: 'GET' }))
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toMatchObject({
      providerId: 'provider_1',
      modelName: 'image-model',
      displayName: 'Image Model',
      supportsGenerate: true,
      supportsEdit: true,
      supportsMultiReference: true,
      supportsN: true,
      maxOutputCount: 4,
      supportedSizes: ['1024x1024', '1536x1024'],
      supportedQualities: ['standard', 'hd'],
      supportedOutputFormats: ['png', 'jpeg'],
      pricing: {
        currency: 'USD',
        unitPrices: {
          image: 0.04,
        },
      },
      status: 'ENABLED',
    })
    for (const index of [2, 3, 4, 5]) {
      expect((fetchImpl.mock.calls[index][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    }
  })
})
