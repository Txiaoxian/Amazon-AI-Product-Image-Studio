import { describe, expect, it, vi } from 'vitest'
import { ApiClientError, apiRequest, buildApiUrl, parseApiErrorResponse } from '../api/client'
import type { ApiPage } from '../types/api'
import type { Project } from '../types/platform'

describe('api client', () => {
  it('includes credentials and returns data from the success envelope', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: { id: 'project_1', name: 'Studio project' },
          requestId: 'req_1',
        }),
        { status: 200 },
      ),
    )

    const data = await apiRequest<Pick<Project, 'id' | 'name'>>('/projects/project_1', {
      fetchImpl,
    })

    expect(data).toEqual({ id: 'project_1', name: 'Studio project' })
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/projects/project_1',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('returns paginated data from the response envelope', async () => {
    const page: ApiPage<{ id: string }> = {
      records: [{ id: 'project_1' }],
      total: 1,
      pageNum: 1,
      pageSize: 20,
    }
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          data: page,
          requestId: 'req_page',
        }),
        { status: 200 },
      ),
    )

    await expect(
      apiRequest<ApiPage<{ id: string }>>('/projects', {
        fetchImpl,
        query: { pageNum: 1, pageSize: 20 },
      }),
    ).resolves.toEqual(page)
    expect(fetchImpl).toHaveBeenCalledWith(
      '/api/v1/projects?pageNum=1&pageSize=20',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('parses structured API errors without exposing raw response text', async () => {
    const error = await parseApiErrorResponse(
      new Response(
        JSON.stringify({
          error: {
            code: 'VALIDATION_ERROR',
            message: 'Invalid request.',
            details: { field: 'prompt' },
          },
          requestId: 'req_bad',
        }),
        { status: 422 },
      ),
    )

    expect(error).toBeInstanceOf(ApiClientError)
    expect(error.status).toBe(422)
    expect(error.code).toBe('VALIDATION_ERROR')
    expect(error.message).toBe('Invalid request.')
    expect(error.requestId).toBe('req_bad')
    expect(error.details).toEqual({ field: 'prompt' })
  })

  it('throws normalized errors for failed requests', async () => {
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        JSON.stringify({
          error: {
            code: 'FORBIDDEN',
            message: 'Not allowed.',
          },
          requestId: 'req_forbidden',
        }),
        { status: 403 },
      ),
    )

    await expect(apiRequest('/projects', { fetchImpl })).rejects.toMatchObject({
      code: 'FORBIDDEN',
      message: 'Not allowed.',
      requestId: 'req_forbidden',
      status: 403,
    })
  })

  it('builds query strings without empty values', () => {
    expect(
      buildApiUrl('/api/v1', 'projects', {
        pageNum: 1,
        pageSize: 20,
        sortBy: 'createdAt',
        sortOrder: 'desc',
        ignored: undefined,
      }),
    ).toBe('/api/v1/projects?pageNum=1&pageSize=20&sortBy=createdAt&sortOrder=desc')
  })
})
