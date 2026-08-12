import { Lightbulb } from 'lucide-react'
import { useState } from 'react'
import { getPromptRecommendations, type PromptRecommendation } from '../../lib/promptRecommendations'
import type { WorkbenchImageType } from '../../types/workbench'

interface PromptRecommendationPickerProps {
  disabled?: boolean
  imageType: WorkbenchImageType
  imageTypeLabel: string
  onSelect: (recommendation: PromptRecommendation) => void
  variant?: 'default' | 'compact'
}

export function PromptRecommendationPicker({
  disabled,
  imageType,
  imageTypeLabel,
  onSelect,
  variant = 'default',
}: PromptRecommendationPickerProps) {
  const [selectedId, setSelectedId] = useState('')
  const recommendations = getPromptRecommendations(imageType)
  const selected = recommendations.find((recommendation) => recommendation.id === selectedId)
  const isCompact = variant === 'compact'

  if (isCompact) {
    return (
      <div className="space-y-2 rounded-lg border border-white/10 bg-white/[0.03] p-2.5" data-testid="compact-prompt-recommendations">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 text-xs font-semibold text-slate-300">
            <Lightbulb className="h-4 w-4 text-amazon-400" />
            <span>推荐模板</span>
          </div>
          <span className="text-[10px] text-slate-500">{imageTypeLabel}</span>
        </div>
        <div className="grid gap-2">
          {recommendations.map((recommendation) => (
            <article className="rounded-md border border-white/10 bg-white/[0.04] px-2.5 py-2" key={recommendation.id}>
              <h4 className="truncate text-xs font-semibold text-slate-200">{recommendation.title}</h4>
              <p className="mt-1 text-[10px] leading-4 text-slate-500">{recommendation.description}</p>
              <p className="mt-1 line-clamp-3 text-[11px] leading-5 text-slate-400">{recommendation.prompt}</p>
            </article>
          ))}
        </div>
      </div>
    )
  }

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
