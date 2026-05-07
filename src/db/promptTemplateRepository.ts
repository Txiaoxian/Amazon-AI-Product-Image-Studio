import { createId, db, type PromptTemplate } from './dexie'

export async function listPromptTemplates(): Promise<PromptTemplate[]> {
  return db.promptTemplates.orderBy('updatedAt').reverse().toArray()
}

export async function savePromptTemplate(title: string, prompt: string): Promise<PromptTemplate> {
  const now = new Date().toISOString()
  const template: PromptTemplate = {
    id: createId('tpl'),
    title,
    prompt,
    createdAt: now,
    updatedAt: now,
  }

  await db.promptTemplates.put(template)
  return template
}

export async function deletePromptTemplate(id: string): Promise<void> {
  await db.promptTemplates.delete(id)
}
