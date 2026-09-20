import { nanoid } from 'nanoid'
import {
  normalizeLocalDashboard,
  type ChartWidgetConfig,
  type HtmlWidgetConfig,
  type LocalWidgetConfig,
  type LocalWidgetType,
  type MetricWidgetConfig,
  type NormalizedLocalDashboard,
  type NormalizedLocalWidget,
  type TextWidgetConfig
} from '@/components/local-visualization-viewer'
import type { TimewindowConfig } from '@/components/local-visualization-viewer/timewindow/types'
import type { EntityRelationSourceConfig } from '@/components/local-visualization-viewer/entity-relation/types'
import { LOCAL_VIEWER_LIMITS } from '@/components/local-visualization-viewer/types'

const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/

export interface EditorWidget {
  id: string
  x: number
  y: number
  w: number
  h: number
  type: LocalWidgetType
  config: LocalWidgetConfig
  timewindow?: TimewindowConfig
  entityRelation?: EntityRelationSourceConfig
}

export interface EditorDashboard {
  version: 1
  columns: number
  rowHeight: number
  widgets: EditorWidget[]
  timewindow?: TimewindowConfig
  responsive?: boolean
}

export type EditorModelResult = { ok: true; dashboard: EditorDashboard } | { ok: false; error: string }

function fail(error: string): EditorModelResult {
  return { ok: false, error }
}

function cleanOptional<T extends Record<string, unknown>>(value: T): T {
  return Object.fromEntries(Object.entries(value).filter(([, item]) => item !== undefined)) as T
}

function canonicalConfig(widget: NormalizedLocalWidget): LocalWidgetConfig {
  if (widget.type === 'text') {
    const config = widget.config as TextWidgetConfig
    return cleanOptional({
      text: config.text,
      field: config.field,
      fallback: config.fallback,
      entityRelation: config.entityRelation
    })
  }
  if (widget.type === 'metric') {
    const config = widget.config as MetricWidgetConfig
    return cleanOptional({
      label: config.label,
      field: config.field,
      unit: config.unit,
      decimals: config.decimals,
      fallback: config.fallback,
      entityRelation: config.entityRelation
    })
  }
  if (widget.type === 'html') {
    const config = widget.config as HtmlWidgetConfig
    return cleanOptional({
      html: config.html,
      css: config.css,
      field: config.field,
      fields: config.fields ? [...config.fields] : undefined,
      fallback: config.fallback,
      entityRelation: config.entityRelation
    })
  }
  const config = widget.config as ChartWidgetConfig
  return cleanOptional({
    title: config.title,
    categoryField: config.categoryField,
    valueField: config.valueField,
    categories: config.categories ? [...config.categories] : undefined,
    values: config.values ? [...config.values] : undefined,
    seriesName: config.seriesName,
    chartStyle: config.chartStyle,
    colorTheme: config.colorTheme,
    yMin: config.yMin,
    yMax: config.yMax,
    threshold: config.threshold,
    timewindow: config.timewindow,
    entityRelation: config.entityRelation
  })
}

function canonicalDashboard(dashboard: NormalizedLocalDashboard): EditorModelResult {
  const unsupported = dashboard.widgets.find((widget) => widget.type === 'unsupported')
  if (unsupported) return fail(`Widget ${unsupported.id} is not supported`)
  const boundChart = dashboard.widgets.find(
    (widget) =>
      (widget.type === 'line-chart' || widget.type === 'bar-chart') &&
      Boolean((widget.config as ChartWidgetConfig).categoryField || (widget.config as ChartWidgetConfig).valueField) &&
      !(widget.config as ChartWidgetConfig).entityRelation?.enabled
  )
  if (boundChart) return fail(`Chart ${boundChart.id} must use static data`)

  return {
    ok: true,
    dashboard: {
      version: 1,
      columns: dashboard.columns,
      rowHeight: dashboard.rowHeight,
      widgets: dashboard.widgets.map((widget) => ({
        id: widget.id,
        x: widget.x,
        y: widget.y,
        w: widget.w,
        h: widget.h,
        type: widget.type as LocalWidgetType,
        config: canonicalConfig(widget),
        ...(widget.timewindow ? { timewindow: widget.timewindow } : {}),
        ...(widget.entityRelation ? { entityRelation: widget.entityRelation } : {})
      })),
      ...(dashboard.timewindow ? { timewindow: dashboard.timewindow } : {}),
      ...(dashboard.responsive !== undefined ? { responsive: dashboard.responsive } : {})
    }
  }
}

function normalizeCanonicalDashboard(input: unknown): EditorModelResult {
  const normalized = normalizeLocalDashboard(input)
  if (!normalized.ok) return normalized

  const canonical = canonicalDashboard(normalized.dashboard)
  if (!canonical.ok) return canonical

  const canonicalGate = normalizeLocalDashboard(canonical.dashboard)
  if (!canonicalGate.ok) return canonicalGate
  return canonicalDashboard(canonicalGate.dashboard)
}

export function loadEditorDashboard(input: unknown): EditorModelResult {
  let parsed = input
  try {
    if (typeof input === 'string') parsed = JSON.parse(input) as unknown
  } catch {
    return fail('Dashboard config is not valid JSON')
  }
  return normalizeCanonicalDashboard(parsed)
}

export function validateEditorDashboard(input: unknown): EditorModelResult {
  return normalizeCanonicalDashboard(input)
}

export function serializeEditorDashboard(
  input: unknown
): { ok: true; config: string; dashboard: EditorDashboard } | { ok: false; error: string } {
  const result = validateEditorDashboard(input)
  if (!result.ok) return result

  const config = JSON.stringify(result.dashboard)
  const saveGate = normalizeCanonicalDashboard(JSON.parse(config) as unknown)
  if (!saveGate.ok) return saveGate
  return { ok: true, config: JSON.stringify(saveGate.dashboard), dashboard: saveGate.dashboard }
}

function isLocalWidgetType(type: unknown): type is LocalWidgetType {
  return type === 'text' || type === 'metric' || type === 'line-chart' || type === 'bar-chart' || type === 'html'
}

function defaultConfig(type: LocalWidgetType): LocalWidgetConfig {
  if (type === 'text') return { text: 'Text' }
  if (type === 'metric') return { label: 'Metric', field: 'value', fallback: '-' }
  if (type === 'html') {
    return {
      html: '<div class="html-card">\n  <h4>HTML 容器</h4>\n  <p>状态: <span class="badge">正常</span></p>\n</div>',
      css: '.html-card { padding: 8px; }\n.badge { background: #10b981; color: #fff; padding: 2px 6px; border-radius: 4px; font-size: 12px; }'
    }
  }
  return { title: type === 'line-chart' ? 'Line chart' : 'Bar chart', categories: ['A'], values: [0] }
}

function createUniqueId(dashboard: EditorDashboard, idFactory: () => string): string | null {
  const ids = new Set(dashboard.widgets.map((widget) => widget.id))
  for (let attempt = 0; attempt < 100; attempt += 1) {
    let candidate: unknown
    try {
      candidate = idFactory()
    } catch {
      return null
    }
    if (typeof candidate !== 'string') return null
    const id = candidate.slice(0, 64)
    if (ID_PATTERN.test(id) && !ids.has(id)) return id
  }
  return null
}

export function addWidget(
  dashboard: EditorDashboard,
  type: LocalWidgetType,
  idFactory: () => string = () => nanoid(12)
): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  if (!isLocalWidgetType(type)) return fail('Widget type is not supported')
  if (current.dashboard.widgets.length >= LOCAL_VIEWER_LIMITS.widgets) return fail('Dashboard has too many widgets')

  const id = createUniqueId(current.dashboard, idFactory)
  if (!id) return fail('Unable to create a unique widget ID')
  const width = Math.min(type === 'text' || type === 'metric' ? 6 : type === 'html' ? 8 : 12, current.dashboard.columns)
  const height = type === 'text' ? 2 : 4
  const nextY = current.dashboard.widgets.reduce((maximum, widget) => Math.max(maximum, widget.y + widget.h), 0)
  if (nextY + height > LOCAL_VIEWER_LIMITS.rows) return fail('Dashboard has no remaining rows')
  return validateEditorDashboard({
    ...current.dashboard,
    widgets: [
      ...current.dashboard.widgets,
      { id, x: 0, y: nextY, w: width, h: height, type, config: defaultConfig(type) }
    ]
  })
}

export function removeWidget(dashboard: EditorDashboard, id: string): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  if (!ID_PATTERN.test(id) || !current.dashboard.widgets.some((widget) => widget.id === id))
    return fail('Widget was not found')
  return validateEditorDashboard({
    ...current.dashboard,
    widgets: current.dashboard.widgets.filter((widget) => widget.id !== id)
  })
}

export function updateWidgetLayout(
  dashboard: EditorDashboard,
  id: string,
  layout: Partial<Pick<EditorWidget, 'x' | 'y' | 'w' | 'h'>>
): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  if (!ID_PATTERN.test(id) || !current.dashboard.widgets.some((widget) => widget.id === id))
    return fail('Widget was not found')
  return validateEditorDashboard({
    ...current.dashboard,
    widgets: current.dashboard.widgets.map((widget) => (widget.id === id ? { ...widget, ...layout } : widget))
  })
}

export function updateWidgetConfig(dashboard: EditorDashboard, id: string, config: unknown): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  if (!ID_PATTERN.test(id) || !current.dashboard.widgets.some((widget) => widget.id === id))
    return fail('Widget was not found')
  return validateEditorDashboard({
    ...current.dashboard,
    widgets: current.dashboard.widgets.map((widget) => (widget.id === id ? { ...widget, config } : widget))
  })
}

export function updateDashboardTimewindow(
  dashboard: EditorDashboard,
  timewindow: TimewindowConfig | undefined
): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  return validateEditorDashboard({
    ...current.dashboard,
    timewindow
  })
}

export function updateDashboardResponsive(dashboard: EditorDashboard, responsive: boolean): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  return validateEditorDashboard({
    ...current.dashboard,
    responsive
  })
}

export function updateWidgetTimewindow(
  dashboard: EditorDashboard,
  id: string,
  timewindow: TimewindowConfig | undefined
): EditorModelResult {
  const current = validateEditorDashboard(dashboard)
  if (!current.ok) return current
  if (!ID_PATTERN.test(id) || !current.dashboard.widgets.some((widget) => widget.id === id))
    return fail('Widget was not found')
  return validateEditorDashboard({
    ...current.dashboard,
    widgets: current.dashboard.widgets.map((widget) => (widget.id === id ? { ...widget, timewindow } : widget))
  })
}
