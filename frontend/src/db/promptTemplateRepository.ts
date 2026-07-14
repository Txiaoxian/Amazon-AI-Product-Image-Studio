import { createId, db, type PromptTemplate } from './dexie'
import type { WorkbenchImageType } from '../types/workbench'

export async function listPromptTemplates(imageType: WorkbenchImageType): Promise<PromptTemplate[]> {
  const templates = await db.promptTemplates.where('imageType').equals(imageType).toArray()
  return templates.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
}

export async function savePromptTemplate(
  imageType: WorkbenchImageType,
  title: string,
  prompt: string,
): Promise<PromptTemplate> {
  const now = new Date().toISOString()
  const template: PromptTemplate = {
    id: createId('tpl'),
    imageType,
    title,
    prompt,
    createdAt: now,
    updatedAt: now,
  }

  await db.promptTemplates.put(template)
  return template
}

export async function updatePromptTemplate(
  id: string,
  imageType: WorkbenchImageType,
  title: string,
  prompt: string,
): Promise<PromptTemplate> {
  const existing = await db.promptTemplates.get(id)
  if (!existing || existing.imageType !== imageType) {
    throw new Error('当前图片类型下未找到该提示词模板。')
  }

  const template: PromptTemplate = {
    ...existing,
    title,
    prompt,
    updatedAt: new Date().toISOString(),
  }
  await db.promptTemplates.put(template)
  return template
}

export async function deletePromptTemplate(id: string, imageType: WorkbenchImageType): Promise<void> {
  const existing = await db.promptTemplates.get(id)
  if (!existing || existing.imageType !== imageType) {
    return
  }
  await db.promptTemplates.delete(id)
}
