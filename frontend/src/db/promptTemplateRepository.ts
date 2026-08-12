import { createId, db, LEGACY_PROMPT_TEMPLATE_PROJECT_ID, type PromptTemplate } from './dexie'
import { WORKBENCH_IMAGE_TYPE_OPTIONS, type WorkbenchImageType } from '../types/workbench'

export function listPromptTemplates(imageType: WorkbenchImageType): Promise<PromptTemplate[]>
export function listPromptTemplates(projectId: string | null | undefined, imageType: WorkbenchImageType): Promise<PromptTemplate[]>
export async function listPromptTemplates(
  projectOrImageType: string | null | undefined,
  requestedImageType?: WorkbenchImageType,
): Promise<PromptTemplate[]> {
  const projectId = requestedImageType ? normalizeProjectId(projectOrImageType) : LEGACY_PROMPT_TEMPLATE_PROJECT_ID
  const imageType = requestedImageType ?? normalizeImageType(projectOrImageType)
  const templates = await db.promptTemplates
    .where('[projectId+imageType]')
    .equals([projectId, imageType])
    .toArray()
  return sortTemplates(templates)
}

export async function listPromptTemplatesForProject(projectId: string | null | undefined): Promise<PromptTemplate[]> {
  if (!projectId?.trim()) {
    return []
  }

  const templates = await db.promptTemplates.where('projectId').equals(projectId).toArray()
  return sortTemplates(templates)
}

export function savePromptTemplate(imageType: WorkbenchImageType, title: string, prompt: string): Promise<PromptTemplate>
export function savePromptTemplate(
  projectId: string | null | undefined,
  imageType: WorkbenchImageType,
  title: string,
  prompt: string,
): Promise<PromptTemplate>
export async function savePromptTemplate(
  projectOrImageType: string | null | undefined,
  imageTypeOrTitle: WorkbenchImageType | string,
  titleOrPrompt: string,
  requestedPrompt?: string,
): Promise<PromptTemplate> {
  const isLegacyCall = requestedPrompt === undefined
  const projectId = isLegacyCall ? LEGACY_PROMPT_TEMPLATE_PROJECT_ID : normalizeProjectId(projectOrImageType)
  const imageType = (isLegacyCall ? projectOrImageType : imageTypeOrTitle) as WorkbenchImageType
  const title = isLegacyCall ? imageTypeOrTitle : titleOrPrompt
  const prompt = isLegacyCall ? titleOrPrompt : requestedPrompt
  const normalized = normalizeTemplateContent(title, prompt)
  const now = new Date().toISOString()
  const template: PromptTemplate = {
    id: createId('tpl'),
    projectId,
    imageType,
    title: normalized.title,
    prompt: normalized.prompt,
    createdAt: now,
    updatedAt: now,
  }

  await db.promptTemplates.put(template)
  return template
}

export function updatePromptTemplate(
  id: string,
  imageType: WorkbenchImageType,
  title: string,
  prompt: string,
): Promise<PromptTemplate>
export function updatePromptTemplate(
  id: string,
  projectId: string | null | undefined,
  imageType: WorkbenchImageType,
  title: string,
  prompt: string,
): Promise<PromptTemplate>
export async function updatePromptTemplate(
  id: string,
  projectOrImageType: string | null | undefined,
  imageTypeOrTitle: WorkbenchImageType | string,
  titleOrPrompt: string,
  requestedPrompt?: string,
): Promise<PromptTemplate> {
  const isLegacyCall = requestedPrompt === undefined
  const projectId = isLegacyCall ? LEGACY_PROMPT_TEMPLATE_PROJECT_ID : normalizeProjectId(projectOrImageType)
  const imageType = (isLegacyCall ? projectOrImageType : imageTypeOrTitle) as WorkbenchImageType
  const title = isLegacyCall ? imageTypeOrTitle : titleOrPrompt
  const prompt = isLegacyCall ? titleOrPrompt : requestedPrompt
  const existing = await db.promptTemplates.get(id)
  if (!existing || existing.projectId !== projectId || existing.imageType !== imageType) {
    throw new Error('当前图片类型下未找到该提示词模板。')
  }

  const normalized = normalizeTemplateContent(title, prompt)
  const template: PromptTemplate = {
    ...existing,
    title: normalized.title,
    prompt: normalized.prompt,
    updatedAt: new Date().toISOString(),
  }
  await db.promptTemplates.put(template)
  return template
}

export function deletePromptTemplate(id: string, imageType: WorkbenchImageType): Promise<void>
export function deletePromptTemplate(
  id: string,
  projectId: string | null | undefined,
  imageType: WorkbenchImageType,
): Promise<void>
export async function deletePromptTemplate(
  id: string,
  projectOrImageType: string | null | undefined,
  requestedImageType?: WorkbenchImageType,
): Promise<void> {
  const isLegacyCall = requestedImageType === undefined
  const projectId = isLegacyCall ? LEGACY_PROMPT_TEMPLATE_PROJECT_ID : normalizeProjectId(projectOrImageType)
  const imageType = (isLegacyCall ? projectOrImageType : requestedImageType) as WorkbenchImageType
  const existing = await db.promptTemplates.get(id)
  if (!existing || existing.projectId !== projectId || existing.imageType !== imageType) {
    return
  }
  await db.promptTemplates.delete(id)
}

function normalizeProjectId(projectId: string | null | undefined): string {
  return projectId?.trim() || LEGACY_PROMPT_TEMPLATE_PROJECT_ID
}

function normalizeImageType(imageType: string | null | undefined): WorkbenchImageType {
  if (WORKBENCH_IMAGE_TYPE_OPTIONS.some((option) => option.value === imageType)) {
    return imageType as WorkbenchImageType
  }
  return 'MAIN'
}

function normalizeTemplateContent(title: string, prompt: string | undefined): { title: string; prompt: string } {
  const normalizedTitle = title.trim()
  const normalizedPrompt = prompt?.trim() ?? ''
  if (!normalizedTitle) {
    throw new Error('请输入模板名称。')
  }
  if (!normalizedPrompt) {
    throw new Error('请输入提示词内容。')
  }
  return { title: normalizedTitle, prompt: normalizedPrompt }
}

function sortTemplates(templates: PromptTemplate[]): PromptTemplate[] {
  return templates.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))
}
