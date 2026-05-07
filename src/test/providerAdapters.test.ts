import { beforeEach, describe, expect, it, vi } from 'vitest'
import { geminiImageAdapter } from '../providers/geminiImageAdapter'
import { mapOpenAIImageSize, openaiImageAdapter } from '../providers/openaiImageAdapter'
import { IMAGE_MODELS } from '../providers/registry'
import type { GenerationRequest } from '../providers/types'
import { DEFAULT_RELAY2_API_URL } from '../lib/constants'
import { resizeImageBlob } from '../lib/imageResize'

vi.mock('../lib/imageMeta', () => ({
  getImageDimensions: vi.fn().mockResolvedValue({ width: 1024, height: 1024 }),
}))

vi.mock('../lib/imageResize', () => ({
  resizeImageBlob: vi.fn().mockResolvedValue(new Blob(['resized-image'], { type: 'image/png' })),
}))

const imageBase64 = btoa('image-bytes')

function buildRequest(overrides: Partial<GenerationRequest> = {}): GenerationRequest {
  return {
    prompt: 'studio product image',
    model: IMAGE_MODELS[0],
    quality: '1K',
    aspectRatio: '1:1',
    imageCount: 1,
    references: [],
    referenceImageUrls: [],
    ...overrides,
  }
}

describe('provider adapters', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('maps OpenAI aspect ratio and resolution to pixel size', () => {
    const expectedSizes = {
      '1K': {
        '1:1': '1024x1024',
        '4:5': '816x1024',
        '5:4': '1024x816',
        '3:4': '768x1024',
        '4:3': '1024x768',
        '2:3': '672x1024',
        '3:2': '1024x672',
        '9:16': '608x1088',
        '16:9': '1088x608',
      },
      '2K': {
        '1:1': '2048x2048',
        '4:5': '1632x2048',
        '5:4': '2048x1632',
        '3:4': '1536x2048',
        '4:3': '2048x1536',
        '2:3': '1360x2048',
        '3:2': '2048x1360',
        '9:16': '1152x2048',
        '16:9': '2048x1152',
      },
      '4K': {
        '1:1': '2880x2880',
        '4:5': '2560x3216',
        '5:4': '3216x2560',
        '3:4': '2480x3312',
        '4:3': '3312x2480',
        '2:3': '2336x3520',
        '3:2': '3520x2336',
        '9:16': '2160x3840',
        '16:9': '3840x2160',
      },
    } as const

    for (const [quality, ratioMap] of Object.entries(expectedSizes)) {
      for (const [aspectRatio, size] of Object.entries(ratioMap)) {
        expect(mapOpenAIImageSize(aspectRatio as keyof typeof ratioMap, quality as keyof typeof expectedSizes)).toBe(size)
      }
    }
  })

  it('passes OpenAI auto size through without pixel mapping', () => {
    expect(mapOpenAIImageSize('1:1', 'auto')).toBe('auto')
    expect(mapOpenAIImageSize('16:9', 'auto')).toBe('auto')
  })

  it('maps the Amazon 1.62:1 OpenAI ratio to valid gpt-image-2 request sizes', () => {
    expect(mapOpenAIImageSize('1.62:1', '1K')).toBe('1040x640')
    expect(mapOpenAIImageSize('1.62:1', '2K')).toBe('2048x1264')
    expect(mapOpenAIImageSize('1.62:1', '4K')).toBe('3104x1920')
  })

  it('builds OpenAI text generation requests', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ b64_json: imageBase64 }, { b64_json: imageBase64 }, { b64_json: imageBase64 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const results = await openaiImageAdapter.generateImages(buildRequest({ imageCount: 3 }), {
      apiUrl: 'https://api.openai.test/v1/',
      apiKey: 'openai-key',
    })

    expect(results).toHaveLength(3)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.openai.test/v1/images/generations',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer openai-key' }),
      }),
    )
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body).toMatchObject({
      model: 'gpt-image-2',
      prompt: 'studio product image',
      size: '1024x1024',
      n: 3,
    })
    expect(body).not.toHaveProperty('response_format')
  })

  it('accepts partial OpenAI multi-image responses', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ b64_json: imageBase64 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const results = await openaiImageAdapter.generateImages(buildRequest({ imageCount: 3 }), {
      apiUrl: 'https://api.openai.test/v1/',
      apiKey: 'openai-key',
    })

    expect(results).toHaveLength(1)
    expect(JSON.parse(fetchMock.mock.calls[0][1].body as string)).toMatchObject({
      n: 3,
    })
  })

  it('accepts OpenAI-compatible responses that return image URLs', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: [{ revised_prompt: 'updated prompt', url: 'https://cdn.example.com/generated.png' }] }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }),
      )
      .mockResolvedValueOnce(
        new Response(new Blob(['downloaded-image'], { type: 'image/png' }), {
          status: 200,
          headers: { 'Content-Type': 'image/png' },
        }),
      )
    vi.stubGlobal('fetch', fetchMock)

    const results = await openaiImageAdapter.generateImages(buildRequest(), {
      apiUrl: 'https://api.openai.test/v1/',
      apiKey: 'openai-key',
    })

    expect(results).toHaveLength(1)
    expect(fetchMock).toHaveBeenNthCalledWith(2, 'https://cdn.example.com/generated.png')
    expect(results[0]).toMatchObject({
      width: 1024,
      height: 1024,
      mimeType: 'image/png',
    })
  })

  it('post-processes OpenAI Amazon ratio images to the selected resolution output size', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ b64_json: imageBase64 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const results = await openaiImageAdapter.generateImages(buildRequest({ aspectRatio: '1.62:1' }), {
      apiUrl: 'https://api.openai.test/v1/',
      apiKey: 'openai-key',
    })

    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body).toMatchObject({
      size: '1040x640',
    })
    expect(resizeImageBlob).toHaveBeenCalledWith(expect.any(Blob), { width: 970, height: 600 })
    expect(results[0]).toMatchObject({
      width: 970,
      height: 600,
      mimeType: 'image/png',
    })
  })

  it.each([
    ['2K', '2048x1264', { width: 1940, height: 1200 }],
    ['4K', '3104x1920', { width: 3104, height: 1920 }],
  ] as const)('post-processes OpenAI Amazon ratio images for %s output', async (quality, requestSize, outputSize) => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ b64_json: imageBase64 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await openaiImageAdapter.generateImages(buildRequest({ aspectRatio: '1.62:1', quality }), {
      apiUrl: 'https://api.openai.test/v1/',
      apiKey: 'openai-key',
    })

    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body).toMatchObject({
      size: requestSize,
    })
    expect(resizeImageBlob).toHaveBeenCalledWith(expect.any(Blob), outputSize)
  })

  it('builds OpenAI edit requests without unsupported response_format', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ b64_json: imageBase64 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const reference = new File(['reference-bytes'], 'reference.png', { type: 'image/png' })

    await openaiImageAdapter.generateImages(buildRequest({ references: [reference] }), {
      apiUrl: 'https://api.openai.test/v1/',
      apiKey: 'openai-key',
    })

    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.openai.test/v1/images/edits',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer openai-key' }),
      }),
    )

    const body = fetchMock.mock.calls[0][1].body as FormData
    expect(body.get('model')).toBe('gpt-image-2')
    expect(body.get('prompt')).toBe('studio product image')
    expect(body.get('size')).toBe('1024x1024')
    expect(body.get('n')).toBe('1')
    expect(body.has('response_format')).toBe(false)
  })

  it('proxies default secondary relay edit requests while preserving the editable API URL setting', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: [{ b64_json: imageBase64 }] }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    const reference = new File(['reference-bytes'], 'reference.png', { type: 'image/png' })

    await openaiImageAdapter.generateImages(
      buildRequest({
        model: IMAGE_MODELS[2],
        references: [reference],
      }),
      {
        apiUrl: DEFAULT_RELAY2_API_URL,
        apiKey: 'relay-key',
      },
    )

    expect(fetchMock).toHaveBeenCalledWith(
      '/relay2/v1/images/edits',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({ Authorization: 'Bearer relay-key' }),
      }),
    )

    const body = fetchMock.mock.calls[0][1].body as FormData
    expect(body.get('model')).toBe('gpt-image-2')
    expect(body.getAll('image[]')).toHaveLength(1)
  })

  it('builds Nano Banana 2 native Gemini requests with official 4K imageConfig fields', async () => {
    const fetchMock = vi.fn().mockImplementation(() =>
      Promise.resolve(
        new Response(
          JSON.stringify({
            candidates: [
              {
                content: {
                  parts: [
                    {
                      inlineData: {
                        data: imageBase64,
                        mimeType: 'image/png',
                      },
                    },
                  ],
                },
              },
            ],
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          },
        ),
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    const results = await geminiImageAdapter.generateImages(
      buildRequest({
        model: IMAGE_MODELS[1],
        aspectRatio: '3840x2160',
        quality: 'high',
        imageCount: 2,
        referenceImageUrls: ['https://cdn.example.com/reference.png'],
      }),
      {
        apiUrl: 'https://api.tutujin.com/',
        apiKey: 'shared-key',
      },
    )

    expect(results).toHaveLength(2)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock).toHaveBeenCalledWith(
      'https://api.tutujin.com/v1beta/models/nano-banana-2:generateContent',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          Authorization: 'Bearer shared-key',
          'Content-Type': 'application/json',
        }),
      }),
    )
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string)
    expect(body).toMatchObject({
      contents: [
        {
          parts: [
            { text: 'studio product image' },
            {
              fileData: {
                fileUri: 'https://cdn.example.com/reference.png',
                mimeType: 'image/png',
              },
            },
          ],
        },
      ],
      generationConfig: {
        responseModalities: ['TEXT', 'IMAGE'],
        imageConfig: {
          aspectRatio: '16:9',
          imageSize: '4K',
        },
      },
    })
    expect(body).not.toHaveProperty('extra_body')
    expect(body).not.toHaveProperty('image_urls')
    expect(fetchMock.mock.calls[0][1]).not.toHaveProperty('signal')
  })

  it('normalizes the unreachable Tutujin app API host to the working com host', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          candidates: [
            {
              content: {
                parts: [
                  {
                    inlineData: {
                      data: imageBase64,
                      mimeType: 'image/png',
                    },
                  },
                ],
              },
            },
          ],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    await geminiImageAdapter.generateImages(
      buildRequest({
        model: IMAGE_MODELS[1],
        aspectRatio: '1024x1024',
        quality: 'medium',
      }),
      {
        apiUrl: 'https://api.tutujin.app/v1/images/generations',
        apiKey: 'shared-key',
      },
    )

    expect(fetchMock).toHaveBeenCalledWith('https://api.tutujin.com/v1beta/models/nano-banana-2:generateContent', expect.any(Object))
  })

  it('rejects local file references for Nano Banana 2 because the proxy expects HTTPS URLs', async () => {
    const reference = new File(['reference-bytes'], 'reference.png', { type: 'image/png' })

    await expect(
      geminiImageAdapter.generateImages(
        buildRequest({
          model: IMAGE_MODELS[1],
          aspectRatio: '1024x1024',
          quality: 'medium',
          references: [reference],
        }),
        {
          apiUrl: 'https://api.tutujin.app/v1',
          apiKey: 'shared-key',
        },
      ),
    ).rejects.toThrow('公开 HTTPS 图片 URL')
  })
})
