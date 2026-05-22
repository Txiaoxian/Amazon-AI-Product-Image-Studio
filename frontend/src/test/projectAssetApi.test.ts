import { describe, expect, it, vi } from 'vitest'
import { createAssetApi } from '../api/assets'
import { createApiClient } from '../api/client'
import { createProjectApi } from '../api/projects'

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_project_asset_test',
    }),
    { status },
  )
}

describe('project and asset API wrappers', () => {
  it('lists and creates projects through the authenticated API client', async () => {
    const project = {
      id: 'project_1',
      tenantId: 'tenant_1',
      name: 'Summer Launch',
      brand: 'Acme',
      asin: 'B000TEST',
      site: 'US',
      notes: 'Hero image set',
      status: 'ACTIVE',
      createdBy: 'user_1',
      createdAt: '2026-05-12T00:00:00Z',
      updatedAt: '2026-05-12T00:00:00Z',
    }
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse({
          records: [project],
          total: 1,
          pageNum: 1,
          pageSize: 20,
        }),
      )
      .mockResolvedValueOnce(jsonResponse(project, 201))
    const projectApi = createProjectApi(createApiClient({ fetchImpl }))

    await expect(projectApi.list({ status: 'ACTIVE', pageNum: 1, pageSize: 20 })).resolves.toMatchObject({
      records: [project],
      total: 1,
    })
    await expect(
      projectApi.create(
        {
          name: 'Summer Launch',
          brand: 'Acme',
          asin: 'B000TEST',
          site: 'US',
          notes: 'Hero image set',
        },
        'csrf_memory_only',
      ),
    ).resolves.toEqual(project)

    expect(fetchImpl.mock.calls[0][0]).toBe('/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=20')
    expect(fetchImpl.mock.calls[0][1]).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
    expect(fetchImpl.mock.calls[1][0]).toBe('/api/v1/projects')
    expect(fetchImpl.mock.calls[1][1]).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toEqual({
      name: 'Summer Launch',
      brand: 'Acme',
      asin: 'B000TEST',
      site: 'US',
      notes: 'Hero image set',
    })
  })

  it('uploads reference images as multipart form data without setting a JSON content type', async () => {
    const asset = {
      id: 'asset_1',
      tenantId: 'tenant_1',
      projectId: 'project_1',
      kind: 'REFERENCE',
      category: 'reference',
      filename: 'reference.png',
      mimeType: 'image/png',
      fileSize: 68,
      width: 2,
      height: 2,
      thumbnailUrl: '',
      previewUrl: '/api/v1/assets/asset_1/download',
      isFavorite: true,
      createdBy: 'user_1',
      createdAt: '2026-05-12T00:00:00Z',
      updatedAt: '2026-05-12T00:00:00Z',
    }
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(asset, 201))
    const assetApi = createAssetApi(createApiClient({ fetchImpl }))
    const file = new File(['png-bytes'], 'reference.png', { type: 'image/png' })

    await expect(
      assetApi.uploadReference(
        'project_1',
        {
          file,
          category: 'reference',
          filename: 'reference.png',
          isFavorite: true,
        },
        'csrf_memory_only',
      ),
    ).resolves.toEqual(asset)

    const [, init] = fetchImpl.mock.calls[0]
    const headers = init?.headers as Headers
    const body = init?.body as FormData
    expect(fetchImpl.mock.calls[0][0]).toBe('/api/v1/projects/project_1/assets/uploads')
    expect(init).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'POST',
      }),
    )
    expect(headers.get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(headers.has('Content-Type')).toBe(false)
    expect(body.get('file')).toBe(file)
    expect(body.get('kind')).toBe('REFERENCE')
    expect(body.get('category')).toBe('reference')
    expect(body.get('filename')).toBe('reference.png')
    expect(body.get('isFavorite')).toBe('true')
  })

  it('calls project update with CSRF and project metadata payload', async () => {
    const updated = {
      id: 'project_1',
      tenantId: 'tenant_1',
      name: 'Summer Launch Updated',
      brand: 'Acme',
      asin: 'B000TEST',
      site: 'US',
      notes: 'Updated notes',
      status: 'ACTIVE',
      createdBy: 'user_1',
      createdAt: '2026-05-12T00:00:00Z',
      updatedAt: '2026-05-13T00:00:00Z',
    }
    const fetchImpl = vi.fn<typeof fetch>().mockResolvedValue(jsonResponse(updated))
    const projectApi = createProjectApi(createApiClient({ fetchImpl }))

    await expect(
      projectApi.update(
        'project_1',
        {
          name: 'Summer Launch Updated',
          brand: 'Acme',
          asin: 'B000TEST',
          site: 'US',
          notes: 'Updated notes',
        },
        'csrf_memory_only',
      ),
    ).resolves.toEqual(updated)

    expect(fetchImpl.mock.calls[0][0]).toBe('/api/v1/projects/project_1')
    expect(fetchImpl.mock.calls[0][1]).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'PATCH',
      }),
    )
    expect((fetchImpl.mock.calls[0][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[0][1]?.body as string)).toEqual({
      name: 'Summer Launch Updated',
      brand: 'Acme',
      asin: 'B000TEST',
      site: 'US',
      notes: 'Updated notes',
    })
  })

  it('calls project member list and write endpoints through the project API', async () => {
    const member = {
      id: 'member_1',
      tenantId: 'tenant_1',
      projectId: 'project_1',
      userId: 'user_2',
      role: 'VIEWER',
      createdAt: '2026-05-12T00:00:00Z',
      updatedAt: '2026-05-12T00:00:00Z',
    }
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse([member]))
      .mockResolvedValueOnce(jsonResponse(member, 201))
      .mockResolvedValueOnce(jsonResponse({ ...member, role: 'EDITOR' }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
    const projectApi = createProjectApi(createApiClient({ fetchImpl }))

    await expect(projectApi.listMembers('project_1')).resolves.toEqual([member])
    await expect(projectApi.addMember('project_1', { userId: 'user_2', role: 'VIEWER' }, 'csrf_memory_only')).resolves.toEqual(member)
    await expect(projectApi.updateMember('project_1', 'user_2', { role: 'EDITOR' }, 'csrf_memory_only')).resolves.toMatchObject({
      role: 'EDITOR',
    })
    await expect(projectApi.removeMember('project_1', 'user_2', 'csrf_memory_only')).resolves.toEqual({ ok: true })

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/projects/project_1/members',
      '/api/v1/projects/project_1/members',
      '/api/v1/projects/project_1/members/user_2',
      '/api/v1/projects/project_1/members/user_2',
    ])
    expect(fetchImpl.mock.calls[0][1]).toEqual(expect.objectContaining({ method: 'GET' }))
    expect(fetchImpl.mock.calls[1][1]).toEqual(expect.objectContaining({ method: 'POST' }))
    expect(fetchImpl.mock.calls[2][1]).toEqual(expect.objectContaining({ method: 'PATCH' }))
    expect(fetchImpl.mock.calls[3][1]).toEqual(expect.objectContaining({ method: 'DELETE' }))
    expect((fetchImpl.mock.calls[1][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[2][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[3][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[1][1]?.body as string)).toEqual({ userId: 'user_2', role: 'VIEWER' })
    expect(JSON.parse(fetchImpl.mock.calls[2][1]?.body as string)).toEqual({ role: 'EDITOR' })
  })

  it('calls asset list, update, favorite, delete, detail, and download endpoints', async () => {
    const asset = {
      id: 'asset_1',
      tenantId: 'tenant_1',
      projectId: 'project_1',
      kind: 'REFERENCE',
      category: 'reference',
      filename: 'reference.png',
      mimeType: 'image/png',
      fileSize: 68,
      width: 2,
      height: 2,
      thumbnailUrl: '',
      previewUrl: '/api/v1/assets/asset_1/download',
      isFavorite: false,
      createdBy: 'user_1',
      createdAt: '2026-05-12T00:00:00Z',
      updatedAt: '2026-05-12T00:00:00Z',
    }
    const fetchImpl = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(
        jsonResponse({
          records: [asset],
          total: 1,
          pageNum: 1,
          pageSize: 20,
        }),
      )
      .mockResolvedValueOnce(jsonResponse(asset))
      .mockResolvedValueOnce(jsonResponse({ ...asset, filename: 'renamed.png', category: 'hero', isFavorite: true }))
      .mockResolvedValueOnce(jsonResponse({ ...asset, isFavorite: true }))
      .mockResolvedValueOnce(jsonResponse({ ...asset, isFavorite: false }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }))
      .mockResolvedValueOnce(
        new Response('image-bytes', {
          headers: {
            'Content-Disposition': 'attachment; filename="reference.png"',
            'Content-Type': 'image/png',
          },
          status: 200,
        }),
      )
    const assetApi = createAssetApi(createApiClient({ fetchImpl }))

    await expect(assetApi.list('project_1', { kind: 'REFERENCE', category: 'hero', favorite: false })).resolves.toMatchObject({
      records: [asset],
    })
    await expect(assetApi.get('asset_1')).resolves.toEqual(asset)
    await expect(
      assetApi.update('asset_1', { filename: 'renamed.png', category: 'hero', isFavorite: true }, 'csrf_memory_only'),
    ).resolves.toMatchObject({ filename: 'renamed.png', category: 'hero', isFavorite: true })
    await expect(assetApi.favorite('asset_1', 'csrf_memory_only')).resolves.toMatchObject({ isFavorite: true })
    await expect(assetApi.unfavorite('asset_1', 'csrf_memory_only')).resolves.toMatchObject({ isFavorite: false })
    await expect(assetApi.delete('asset_1', 'csrf_memory_only')).resolves.toEqual({ ok: true })
    const downloaded = await assetApi.download('asset_1')
    expect(downloaded.filename).toBe('reference.png')
    expect(downloaded.blob.type).toBe('image/png')
    expect(await downloaded.blob.text()).toBe('image-bytes')

    expect(fetchImpl.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/projects/project_1/assets?kind=REFERENCE&category=hero&favorite=false',
      '/api/v1/assets/asset_1',
      '/api/v1/assets/asset_1',
      '/api/v1/assets/asset_1/favorite',
      '/api/v1/assets/asset_1/favorite',
      '/api/v1/assets/asset_1',
      '/api/v1/assets/asset_1/download',
    ])
    expect(fetchImpl.mock.calls[2][1]).toEqual(expect.objectContaining({ method: 'PATCH' }))
    expect(fetchImpl.mock.calls[3][1]).toEqual(expect.objectContaining({ method: 'POST' }))
    expect(fetchImpl.mock.calls[4][1]).toEqual(expect.objectContaining({ method: 'DELETE' }))
    expect(fetchImpl.mock.calls[5][1]).toEqual(expect.objectContaining({ method: 'DELETE' }))
    expect((fetchImpl.mock.calls[2][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[3][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[4][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect((fetchImpl.mock.calls[5][1]?.headers as Headers).get('X-CSRF-Token')).toBe('csrf_memory_only')
    expect(JSON.parse(fetchImpl.mock.calls[2][1]?.body as string)).toEqual({
      filename: 'renamed.png',
      category: 'hero',
      isFavorite: true,
    })
    expect(fetchImpl.mock.calls[6][1]).toEqual(
      expect.objectContaining({
        credentials: 'include',
        method: 'GET',
      }),
    )
  })
})
