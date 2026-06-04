import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App'

const authenticatedSession = {
  user: {
    id: 'user_1',
    email: 'admin@example.com',
    displayName: 'Admin User',
    status: 'ACTIVE',
    createdAt: '2026-01-01T00:00:00.000Z',
    updatedAt: '2026-01-01T00:00:00.000Z',
  },
  tenant: {
    id: 'tenant_1',
    name: 'Studio Tenant',
    status: 'ACTIVE',
  },
  roles: [
    {
      id: 'role_admin',
      code: 'admin',
      name: '管理员',
    },
  ],
  permissions: ['project:read', 'project:create', 'asset:read', 'asset:download'],
  csrfToken: 'csrf_from_me',
}

function successResponse(data: unknown, status = 200): Response {
  return new Response(
    JSON.stringify({
      data,
      requestId: 'req_legacy_retirement',
    }),
    { status },
  )
}

function page(records: unknown[], pageSize = 50) {
  return {
    records,
    total: records.length,
    pageNum: 1,
    pageSize,
  }
}

function createWorkbenchFetch() {
  return vi.fn<typeof fetch>(async (input) => {
    const url = String(input)

    if (url === '/api/v1/me') {
      return successResponse(authenticatedSession)
    }
    if (url === '/api/v1/projects?status=ACTIVE&pageNum=1&pageSize=50') {
      return successResponse(page([]))
    }

    return new Response('', { status: 404 })
  })
}

describe('legacy frontend retirement', () => {
  afterEach(() => {
    cleanup()
    localStorage.clear()
    vi.unstubAllGlobals()
  })

  it('keeps the production import graph out of browser Provider and IndexedDB modules', () => {
    const productionModules = collectProductionModules(resolve(process.cwd(), 'src/App.tsx'))

    expect([...productionModules].some((file) => file.includes('/src/providers/'))).toBe(false)
    expect([...productionModules].some((file) => file.endsWith('/src/db/historyRepository.ts'))).toBe(false)
    expect([...productionModules].some((file) => file.endsWith('/src/db/imageRepository.ts'))).toBe(false)
    expect([...productionModules].some((file) => file.endsWith('/src/hooks/useStorageUsage.ts'))).toBe(false)
    expect([...productionModules].some((file) => file.endsWith('/src/components/history/LegacyHistoryPanel.tsx'))).toBe(false)
    expect([...productionModules].some((file) => file.endsWith('/src/components/history/LegacyHistoryItem.tsx'))).toBe(false)

    const dbModules = [...productionModules].filter((file) => file.includes('/src/db/')).map((file) => file.slice(file.indexOf('/src/db/')))
    expect(dbModules.sort()).toEqual(['/src/db/dexie.ts', '/src/db/promptTemplateRepository.ts'])
  })

  it('keeps deleted legacy display and storage helpers from returning as source files', () => {
    for (const relativePath of [
      'src/components/history/LegacyHistoryPanel.tsx',
      'src/components/history/LegacyHistoryItem.tsx',
      'src/db/historyRepository.ts',
      'src/db/imageRepository.ts',
      'src/hooks/useObjectUrl.ts',
      'src/hooks/useStorageUsage.ts',
    ]) {
      expect(existsSync(resolve(process.cwd(), relativePath))).toBe(false)
    }
  })

  it('keeps Dexie limited to prompt templates while deleting retired image and history stores on upgrade', () => {
    const dexieSource = readFileSync(resolve(process.cwd(), 'src/db/dexie.ts'), 'utf8')

    expect(dexieSource).not.toMatch(/StoredImage|HistoryItem|ImagePurpose|blob:\s*Blob|images!|historyItems!/)
    expect(dexieSource).toContain('promptTemplates!: Table<PromptTemplate, string>')
    expect(dexieSource).toContain('images: null')
    expect(dexieSource).toContain('historyItems: null')
  })

  it('keeps sensitive storage, Provider authorization, and polling out of the production graph', () => {
    const source = readProductionSources(resolve(process.cwd(), 'src/App.tsx'))

    expect(source).not.toMatch(/\blocalStorage\b|\bsessionStorage\b/)
    expect(source).not.toMatch(/\bindexedDB\b/)
    expect(source).not.toMatch(/(?:localStorage|sessionStorage)\s*\.[\s\S]{0,120}(?:apiKey|apiUrl|provider)/i)
    expect(source).not.toMatch(/Authorization\s*:\s*['"`]Bearer|headers\s*:\s*{[^}]*Authorization/i)
    expect(source).not.toMatch(/\bsetInterval\b|\bsetTimeout\b/)
    expect(source).not.toMatch(/api\.openai\.com|generativelanguage\.googleapis\.com|api\.tutujin\.(?:app|com)|api\.flymux\.com/)
    expect(source).not.toMatch(/\bminio\b|\bs3[.-]|X-Amz-Signature|objectKey|bucketOriginals/i)
    expect(source).not.toMatch(/tenantConcurrency|storageQuotaBytes|logRetentionDays|allowedMimeTypes/)
  })

  it('does not expose the retired ordinary settings or legacy history entry points', async () => {
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '打开设置' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '查看旧本地历史' })).not.toBeInTheDocument()
    expect(screen.queryByText('API Key')).not.toBeInTheDocument()
    expect(screen.queryByText('API URL')).not.toBeInTheDocument()
  })

  it('ignores residual browser Provider settings during startup', async () => {
    localStorage.setItem(
      'amazon-ai-product-image-studio.settings',
      JSON.stringify({
        defaultModelId: 'openai-gpt-image-2',
        providers: {
          openai: {
            apiUrl: 'https://legacy-provider.example/v1',
            apiKey: 'legacy-openai-key',
          },
          gemini: {
            apiUrl: 'https://legacy-provider.example/v1',
            apiKey: 'legacy-gemini-key',
          },
        },
      }),
    )
    vi.stubGlobal('fetch', createWorkbenchFetch())

    render(<App />)

    expect(await screen.findByText('Admin User')).toBeInTheDocument()
    expect(screen.queryByText('legacy-openai-key')).not.toBeInTheDocument()
    expect(screen.queryByText('legacy-gemini-key')).not.toBeInTheDocument()
  })
})

function collectProductionModules(entryFile: string): Set<string> {
  const visited = new Set<string>()
  const pending = [entryFile]

  while (pending.length > 0) {
    const current = pending.pop()

    if (!current || visited.has(current)) {
      continue
    }

    visited.add(current)
    const source = readFileSync(current, 'utf8')
    const importMatches = source.matchAll(/(?:import|export)\s+(?:type\s+)?(?:[^'"]+\s+from\s+)?['"]([^'"]+)['"]/g)

    for (const match of importMatches) {
      const request = match[1]

      if (!request.startsWith('.')) {
        continue
      }

      const resolvedImport = resolveModule(current, request)

      if (resolvedImport) {
        pending.push(resolvedImport)
      }
    }
  }

  return visited
}

function readProductionSources(entryFile: string): string {
  return [...collectProductionModules(entryFile)].map((file) => readFileSync(file, 'utf8')).join('\n')
}

function resolveModule(fromFile: string, request: string): string | null {
  const basePath = resolve(fromFile, '..', request)
  const candidates = [`${basePath}.ts`, `${basePath}.tsx`, resolve(basePath, 'index.ts'), resolve(basePath, 'index.tsx')]

  return candidates.find((candidate) => {
    try {
      readFileSync(candidate)
      return true
    } catch {
      return false
    }
  }) ?? null
}
