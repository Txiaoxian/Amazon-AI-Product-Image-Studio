import { describe, expect, it } from 'vitest'
import { PROMPT_RECOMMENDATIONS } from '../lib/promptRecommendations'
import { WORKBENCH_IMAGE_TYPE_OPTIONS } from '../types/workbench'

describe('prompt recommendations', () => {
  it('provides three structured recommendations for every workbench image type', () => {
    const ids = new Set<string>()

    for (const { value } of WORKBENCH_IMAGE_TYPE_OPTIONS) {
      const recommendations = PROMPT_RECOMMENDATIONS[value]
      expect(recommendations).toHaveLength(3)

      for (const recommendation of recommendations) {
        expect(ids.has(recommendation.id)).toBe(false)
        ids.add(recommendation.id)
        expect(recommendation.title.trim()).not.toBe('')
        expect(recommendation.prompt).toContain('用途：')
        expect(recommendation.prompt).toContain('主体依据：')
        expect(recommendation.prompt).toContain('输出要求：')
        expect(recommendation.prompt.length).toBeLessThanOrEqual(4000)
      }
    }
  })

  it('keeps factual ecommerce claims constrained in information-heavy image types', () => {
    for (const imageType of ['A_PLUS', 'DIMENSION', 'SELLING_POINT', 'PROMOTION', 'COMPARISON'] as const) {
      for (const recommendation of PROMPT_RECOMMENDATIONS[imageType]) {
        expect(recommendation.prompt).toMatch(/不得|不生成|不添加|不显示|不使用|不虚构|不估算|不推断|不得猜测/)
      }
    }
  })
})
