import type { ECOption } from '@/hooks/chart/use-echarts'

export const LOCAL_VIEWER_LIMITS = {
  widgets: 64,
  columns: 24,
  rows: 200,
  stringLength: 1000,
  dataPoints: 500,
  fields: 200
} as const

export type LocalWidgetType = 'text' | 'metric' | 'line-chart' | 'bar-chart'

export type LocalFieldValue = string | number | boolean | null | readonly (string | number | boolean | null)[]
export type LocalViewerFields = Readonly<Record<string, LocalFieldValue>>

export interface TextWidgetConfig {
  text: string
  field?: string
  fallback?: string
}

export interface MetricWidgetConfig {
  label: string
  field: string
  unit?: string
  decimals?: number
  fallback?: string
}

export interface ChartWidgetConfig {
  title?: string
  categoryField?: string
  valueField?: string
  categories?: readonly string[]
  values?: readonly number[]
  seriesName?: string
}

export type LocalWidgetConfig = TextWidgetConfig | MetricWidgetConfig | ChartWidgetConfig

export interface NormalizedLocalWidget {
  id: string
  x: number
  y: number
  w: number
  h: number
  type: LocalWidgetType | 'unsupported'
  originalType: string
  config: LocalWidgetConfig | Readonly<Record<string, never>>
}

export interface NormalizedLocalDashboard {
  version: 1
  columns: number
  rowHeight: number
  widgets: readonly NormalizedLocalWidget[]
}

export type NormalizeDashboardResult =
  | { ok: true; dashboard: NormalizedLocalDashboard }
  | { ok: false; error: string }

export type NormalizeFieldsResult =
  | { ok: true; fields: LocalViewerFields }
  | { ok: false; error: string }

export interface ResolvedText {
  available: boolean
  text: string
}

export interface ResolvedMetric {
  available: boolean
  label: string
  value: string
  unit: string
}

export interface BuiltChart {
  available: boolean
  option: ECOption
}
