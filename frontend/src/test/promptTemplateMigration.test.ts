import Dexie from 'dexie'
import { afterEach, describe, expect, it } from 'vitest'
import { StudioDatabase } from '../db/dexie'

const databaseNames: string[] = []

describe('prompt template database migration', () => {
  afterEach(async () => {
    await Promise.all(databaseNames.splice(0).map((name) => Dexie.delete(name)))
  })

  it('keeps legacy templates and assigns them to the main image type', async () => {
    const databaseName = `prompt-template-migration-${crypto.randomUUID()}`
    databaseNames.push(databaseName)
    const legacyDatabase = new Dexie(databaseName)
    legacyDatabase.version(1).stores({
      images: '&id, purpose, createdAt',
      historyItems: '&id, createdAt, model, provider',
      promptTemplates: '&id, updatedAt',
    })
    legacyDatabase.version(2).stores({
      images: null,
      historyItems: null,
      promptTemplates: '&id, updatedAt',
    })
    await legacyDatabase.open()
    await legacyDatabase.table('promptTemplates').put({
      id: 'tpl_legacy',
      title: '旧提示词',
      prompt: '保留这个旧模板',
      createdAt: '2026-07-01T00:00:00.000Z',
      updatedAt: '2026-07-01T00:00:00.000Z',
    })
    legacyDatabase.close()

    const migratedDatabase = new StudioDatabase(databaseName)
    await migratedDatabase.open()

    await expect(migratedDatabase.promptTemplates.get('tpl_legacy')).resolves.toMatchObject({
      id: 'tpl_legacy',
      imageType: 'MAIN',
      prompt: '保留这个旧模板',
    })
    migratedDatabase.close()
  })
})
