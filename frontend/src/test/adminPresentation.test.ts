import { describe, expect, it } from 'vitest'
import {
  ADMIN_IMAGE_TYPE_VALUES,
  ADMIN_AUDIT_ACTION_VALUES,
  ADMIN_AUDIT_RESOURCE_VALUES,
  apiCallStatusLabel,
  auditActionLabel,
  auditResourceLabel,
  costStatusLabel,
  errorCategory,
  formatChange,
  formatCompactNumber,
  formatCurrency,
  formatDuration,
  imageTypeLabel,
  lifecycleLabel,
  taskStatusLabel,
} from '../lib/adminPresentation'

describe('管理控制台中文展示字典', () => {
  it('统一翻译任务、调用、生命周期和费用状态', () => {
    expect(taskStatusLabel('SUCCEEDED')).toBe('已完成')
    expect(taskStatusLabel('RETRYING')).toBe('正在重试')
    expect(apiCallStatusLabel('FAILURE')).toBe('失败')
    expect(lifecycleLabel('RESURRECTED')).toBe('回流')
    expect(costStatusLabel('LEGACY_UNKNOWN')).toBe('历史费用状态未知')
    expect(imageTypeLabel('MAIN')).toBe('商品主图')
    expect(imageTypeLabel('A_PLUS')).toBe('A+ 图片')
    expect(imageTypeLabel('DIMENSION')).toBe('尺寸图')
    expect(ADMIN_IMAGE_TYPE_VALUES).toEqual(['MAIN', 'A_PLUS', 'SCENE', 'DETAIL', 'DIMENSION', 'SELLING_POINT', 'PROMOTION', 'COMPARISON'])
    expect(auditActionLabel('user.ai_access.replace')).toBe('调整用户可用模型')
    expect(auditActionLabel('storage.orphan_cleanup')).toBe('清理孤立存储对象')
    expect(auditResourceLabel('generation_task')).toBe('生图任务')
    expect(ADMIN_AUDIT_ACTION_VALUES).toContain('project_member.update')
    expect(ADMIN_AUDIT_RESOURCE_VALUES).toContain('system_settings')
  })

  it('未知枚举始终使用中文兜底且不泄漏原值', () => {
    expect(taskStatusLabel('NEW_PROVIDER_STATE')).toBe('未知任务状态')
    expect(apiCallStatusLabel('MAYBE')).toBe('未知调用状态')
    expect(lifecycleLabel('LIFECYCLE_V2')).toBe('状态待确认')
    expect(costStatusLabel('COST_V2')).toBe('费用状态待确认')
    expect(imageTypeLabel('RAW_TECH_TYPE')).toBe('其他图片类型')
    expect(auditActionLabel('internal.action_v2')).toBe('执行了其他管理操作')
  })

  it('用中文单位、变化说明和业务错误类别解释数据', () => {
    expect(formatCompactNumber(12345, '次')).toBe('1.23万次')
    expect(formatDuration(820)).toBe('820毫秒')
    expect(formatDuration(18_400)).toBe('18.4秒')
    expect(formatCurrency('3.68', 'USD')).toBe('3.68美元')
    expect(formatChange(12.34)).toBe('较上一周期增加 12.3%')
    expect(errorCategory('RATE_LIMITED').label).toBe('调用频率受限')
    expect(errorCategory('UNRECOGNIZED_CODE').label).toBe('其他调用异常')
  })
})
