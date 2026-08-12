import { beforeEach, describe, expect, it } from 'vitest'
import { db } from '../db/dexie'
import {
  deletePromptTemplate,
  listPromptTemplates,
  listPromptTemplatesForProject,
  savePromptTemplate,
  updatePromptTemplate,
} from '../db/promptTemplateRepository'

describe('prompt template repository', () => {
  beforeEach(async () => {
    await db.delete()
    await db.open()
  })

  it('lists only templates saved for the requested image type', async () => {
    await savePromptTemplate('MAIN', '主图模板', '主图内容')
    await savePromptTemplate('A_PLUS', 'A+ 模板', 'A+ 内容')

    await expect(listPromptTemplates('MAIN')).resolves.toMatchObject([{ imageType: 'MAIN', title: '主图模板' }])
    await expect(listPromptTemplates('A_PLUS')).resolves.toMatchObject([{ imageType: 'A_PLUS', title: 'A+ 模板' }])
  })

  it('does not update or delete a template through a different image type', async () => {
    const aPlusTemplate = await savePromptTemplate('A_PLUS', 'A+ 模板', '原始内容')

    await expect(updatePromptTemplate(aPlusTemplate.id, 'MAIN', '错误标题', '错误内容')).rejects.toThrow(
      '当前图片类型下未找到该提示词模板。',
    )
    await deletePromptTemplate(aPlusTemplate.id, 'MAIN')

    await expect(db.promptTemplates.get(aPlusTemplate.id)).resolves.toMatchObject({
      imageType: 'A_PLUS',
      title: 'A+ 模板',
      prompt: '原始内容',
    })
  })

  it('keeps templates isolated by product while allowing CRUD in the owning product', async () => {
    const firstProductTemplate = await savePromptTemplate('project_1', 'MAIN', '产品一模板', '产品一提示词')
    await savePromptTemplate('project_2', 'MAIN', '产品二模板', '产品二提示词')

    await expect(listPromptTemplatesForProject('project_1')).resolves.toMatchObject([
      { projectId: 'project_1', title: '产品一模板', prompt: '产品一提示词' },
    ])
    await expect(listPromptTemplatesForProject('project_2')).resolves.toMatchObject([
      { projectId: 'project_2', title: '产品二模板', prompt: '产品二提示词' },
    ])

    await expect(updatePromptTemplate(firstProductTemplate.id, 'project_2', 'MAIN', '越权修改', '越权内容')).rejects.toThrow(
      '当前图片类型下未找到该提示词模板。',
    )
    await deletePromptTemplate(firstProductTemplate.id, 'project_2', 'MAIN')
    await expect(db.promptTemplates.get(firstProductTemplate.id)).resolves.toMatchObject({
      projectId: 'project_1',
      title: '产品一模板',
    })

    await updatePromptTemplate(firstProductTemplate.id, 'project_1', 'MAIN', '已修改模板', '已修改提示词')
    await deletePromptTemplate(firstProductTemplate.id, 'project_1', 'MAIN')
    await expect(listPromptTemplatesForProject('project_1')).resolves.toEqual([])
  })
})
