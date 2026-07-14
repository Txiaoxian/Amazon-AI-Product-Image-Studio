import { Lightbulb } from 'lucide-react'
import { useState } from 'react'
import { getPromptRecommendations, type PromptRecommendation } from '../../lib/promptRecommendations'
import type { WorkbenchImageType } from '../../types/workbench'

interface PromptRecommendationPickerProps {
  disabled?: boolean
  imageType: WorkbenchImageType
  imageTypeLabel: string
  onSelect: (recommendation: PromptRecommendation) => void
}

export function PromptRecommendationPicker({
  disabled,
  imageType,
  imageTypeLabel,
  onSelect,
}: PromptRecommendationPickerProps) {
  const [selectedId, setSelectedId] = useState('')
  const recommendations = getPromptRecommendations(imageType)
  const selected = recommendations.find((recommendation) => recommendation.id === selectedId)

  return (
    <div className="space-y-2 rounded-md border border-ink-200 bg-ink-50 p-3">
      <label className="flex items-center gap-2 text-xs font-semibold text-ink-700" htmlFor="prompt-recommendation">
        <Lightbulb className="h-4 w-4 text-amazon-600" />
        推荐提示词
      </label>
      <select
        className="field-input bg-white"
        disabled={disabled}
        id="prompt-recommendation"
        onChange={(event) => {
          const recommendation = recommendations.find((item) => item.id === event.target.value)
          setSelectedId(event.target.value)
          if (recommendation) {
            onSelect(recommendation)
          }
        }}
        value={selectedId}
      >
        <option value="">选择推荐提示词</option>
        {recommendations.map((recommendation) => (
          <option key={recommendation.id} value={recommendation.id}>
            {recommendation.title}
          </option>
        ))}
      </select>
      <p className="text-xs leading-5 text-ink-500">
        {selected?.description ?? `已内置 ${recommendations.length} 个${imageTypeLabel}方案，选择后可继续修改并保存为个人模板。`}
      </p>
    </div>
  )
}
