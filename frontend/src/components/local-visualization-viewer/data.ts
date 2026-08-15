import type {
  BuiltChart,
  ChartWidgetConfig,
  LocalFieldValue,
  LocalViewerFields,
  MetricWidgetConfig,
  ResolvedMetric,
  ResolvedText,
  TextWidgetConfig
} from './types'
import { LOCAL_VIEWER_LIMITS } from './types'

function ownField(fields: LocalViewerFields, name: string): LocalFieldValue | undefined {
  return Object.prototype.hasOwnProperty.call(fields, name) ? fields[name] : undefined
}

function scalar(value: LocalFieldValue | undefined): string | number | boolean | null | undefined {
  if (Array.isArray(value)) return undefined
  if (typeof value === 'string' && value.length > LOCAL_VIEWER_LIMITS.stringLength) return undefined
  if (typeof value === 'number' && !Number.isFinite(value)) return undefined
  return value as string | number | boolean | null | undefined
}

export function resolveText(config: TextWidgetConfig, fields: LocalViewerFields): ResolvedText {
  if (!config.field) return { available: true, text: config.text }
  const value = scalar(ownField(fields, config.field))
  if (value === undefined || value === null) {
    return { available: false, text: config.fallback ?? 'Unavailable' }
  }
  return { available: true, text: config.text.replaceAll('{{value}}', String(value)) }
}

export function resolveMetric(config: MetricWidgetConfig, fields: LocalViewerFields): ResolvedMetric {
  const value = scalar(ownField(fields, config.field))
  if (value === undefined || value === null || (typeof value === 'number' && !Number.isFinite(value))) {
    return { available: false, label: config.label, value: config.fallback ?? 'Unavailable', unit: '' }
  }
  const rendered = typeof value === 'number' && config.decimals !== undefined ? value.toFixed(config.decimals) : String(value)
  return { available: true, label: config.label, value: rendered, unit: config.unit ?? '' }
}

function chartData(config: ChartWidgetConfig, fields: LocalViewerFields): { categories: string[]; values: number[] } | null {
  if (config.categories && config.values) {
    return { categories: [...config.categories], values: [...config.values] }
  }
  if (!config.categoryField || !config.valueField) return null
  const categories = ownField(fields, config.categoryField)
  const values = ownField(fields, config.valueField)
  if (!Array.isArray(categories) || !Array.isArray(values) || categories.length !== values.length) return null
  if (categories.length > LOCAL_VIEWER_LIMITS.dataPoints) return null
  if (!categories.every(item => typeof item === 'string' || typeof item === 'number')) return null
  if (!values.every(item => typeof item === 'number' && Number.isFinite(item))) return null
  return { categories: categories.map(String), values: values as number[] }
}

export function buildChartOption(
  type: 'line-chart' | 'bar-chart',
  config: ChartWidgetConfig,
  fields: LocalViewerFields
): BuiltChart {
  const data = chartData(config, fields)
  const emptyOption = {
    title: { text: config.title ?? '', left: 'center' },
    xAxis: { type: 'category' as const, data: [] },
    yAxis: { type: 'value' as const },
    series: []
  }
  if (!data) return { available: false, option: emptyOption }

  return {
    available: true,
    option: {
      title: { text: config.title ?? '', left: 'center' },
      tooltip: { trigger: 'axis' },
      grid: { left: 40, right: 20, top: config.title ? 48 : 20, bottom: 32, containLabel: true },
      xAxis: { type: 'category', data: data.categories },
      yAxis: { type: 'value' },
      series: [
        {
          name: config.seriesName ?? '',
          type: type === 'line-chart' ? 'line' : 'bar',
          data: data.values,
          ...(type === 'line-chart' ? { smooth: false, showSymbol: data.values.length <= 100 } : {})
        }
      ]
    }
  }
}
