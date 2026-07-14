import { beforeEach, describe, expect, it } from 'vitest'
import { db } from '../db/dexie'
import {
  deletePromptTemplate,
  listPromptTemplates,
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
})
