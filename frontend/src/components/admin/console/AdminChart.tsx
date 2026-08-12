import { BarChart, LineChart } from 'echarts/charts'
import { AriaComponent, GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import * as echarts from 'echarts/core'
import type { EChartsCoreOption } from 'echarts/core'
import { SVGRenderer } from 'echarts/renderers'
import { useEffect, useId, useRef } from 'react'
import { ADMIN_CHART_TOKENS } from '../../../lib/adminPresentation'

echarts.use([AriaComponent, BarChart, GridComponent, LegendComponent, LineChart, SVGRenderer, TooltipComponent])

interface AdminChartProps {
  title: string
  description: string
  option: EChartsCoreOption
  height?: number
}

export function AdminChart({ title, description, option, height = 280 }: AdminChartProps) {
  const chartRef = useRef<HTMLDivElement>(null)
  const descriptionId = useId()

  useEffect(() => {
    if (!chartRef.current) return
    const scrollContainer = chartRef.current.parentElement
    const chartWidth = Math.max(scrollContainer?.clientWidth ?? chartRef.current.clientWidth, 560)
    const chart = echarts.init(chartRef.current, undefined, {
      height,
      renderer: 'svg',
      width: chartWidth,
    })
    chart.setOption({
      animationDuration: 280,
      aria: { decal: { show: true }, enabled: true, description },
      backgroundColor: 'transparent',
      color: [...ADMIN_CHART_TOKENS.palette],
      textStyle: { color: ADMIN_CHART_TOKENS.text, fontFamily: 'Inter, PingFang SC, Microsoft YaHei, system-ui, sans-serif' },
      ...option,
    })

    const resize = () => {
      if (!chartRef.current) return
      chart.resize({ height, width: Math.max(chartRef.current.parentElement?.clientWidth ?? 0, 560) })
    }
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(resize)
    if (scrollContainer) observer?.observe(scrollContainer)
    window.addEventListener('resize', resize)
    return () => {
      observer?.disconnect()
      window.removeEventListener('resize', resize)
      chart.dispose()
    }
  }, [description, height, option])

  return (
    <figure aria-describedby={descriptionId} aria-label={title} className="w-full min-w-0 max-w-full overflow-hidden">
      <div className="w-full max-w-full overflow-x-auto">
        <div className="min-w-[560px]" ref={chartRef} style={{ height }} />
      </div>
      <figcaption className="sr-only" id={descriptionId}>
        {description}
      </figcaption>
    </figure>
  )
}
