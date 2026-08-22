/**
 * File purpose: pure assembly helpers for the RDI Overview alarm-trend card.
 * Builds the ECharts option from normalized monthly trend points and the year
 * selector options from the current year. No network, state or DOM concerns.
 */
import type { AlarmTrendPoint } from './rdiOverviewState'

export function buildAlarmTrendYearOptions(currentYear: number): Array<{ label: string; value: number }> {
  return Array.from({ length: currentYear - 1999 }, (_, index) => {
    const year = currentYear - index
    return { label: String(year), value: year }
  })
}

export function buildAlarmTrendChartOptions(points: AlarmTrendPoint[], seriesName: string) {
  return {
    tooltip: {
      trigger: 'axis'
    },
    grid: {
      left: '3%',
      right: '3%',
      top: 24,
      bottom: 24,
      containLabel: true
    },
    xAxis: {
      type: 'category' as const,
      boundaryGap: false,
      data: points.map((point) => String(point.month).padStart(2, '0'))
    },
    yAxis: {
      type: 'value' as const,
      minInterval: 1
    },
    series: [
      {
        name: seriesName,
        type: 'line' as const,
        smooth: true,
        areaStyle: {},
        data: points.map((point) => point.count)
      }
    ]
  }
}
