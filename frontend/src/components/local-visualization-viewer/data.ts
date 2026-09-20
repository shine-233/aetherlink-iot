import type {
  BuiltChart,
  ChartWidgetConfig,
  HtmlWidgetConfig,
  LocalFieldValue,
  LocalViewerFields,
  MapWidgetConfig,
  MetricWidgetConfig,
  ResolvedHtml,
  ResolvedMapData,
  ResolvedMetric,
  ResolvedText,
  TextWidgetConfig
} from './types'
import { LOCAL_VIEWER_LIMITS } from './types'
import { generateRelationFieldKey } from './entity-relation/resolver'
import { convertSeries, convertUnit, convertUnitToSystem } from './units/converter'
import { sanitizeCss, sanitizeHtml } from './sanitizer'

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
  const targetField =
    config.field || (config.entityRelation?.enabled ? generateRelationFieldKey(config.entityRelation) : undefined)
  if (!targetField) return { available: true, text: config.text }
  const value = scalar(ownField(fields, targetField))
  if (value === undefined || value === null) {
    return { available: false, text: config.fallback ?? 'Unavailable' }
  }
  return { available: true, text: config.text.replaceAll('{{value}}', String(value)) }
}

export function resolveMetric(config: MetricWidgetConfig, fields: LocalViewerFields): ResolvedMetric {
  const targetField =
    config.field || (config.entityRelation?.enabled ? generateRelationFieldKey(config.entityRelation) : '')
  const value = scalar(ownField(fields, targetField))
  if (value === undefined || value === null || (typeof value === 'number' && !Number.isFinite(value))) {
    return { available: false, label: config.label, value: config.fallback ?? 'Unavailable', unit: '' }
  }

  let finalVal = value
  let finalUnit = config.unit ?? ''

  if (typeof value === 'number' && config.unit && config.unitConversion?.enabled) {
    try {
      const hasLeadingSpace = config.unit.startsWith(' ')
      if (config.unitConversion.targetUnit) {
        finalVal = convertUnit(value, config.unit, config.unitConversion.targetUnit)
        finalUnit = hasLeadingSpace ? ` ${config.unitConversion.targetUnit.trim()}` : config.unitConversion.targetUnit
      } else if (config.unitConversion.unitSystem) {
        const res = convertUnitToSystem(value, config.unit, config.unitConversion.unitSystem)
        finalVal = res.value
        finalUnit = hasLeadingSpace ? ` ${res.unit}` : res.unit
      }
    } catch {
      // Fail closed / keep original value & unit
    }
  }

  const rendered =
    typeof finalVal === 'number' && config.decimals !== undefined ? finalVal.toFixed(config.decimals) : String(finalVal)
  return { available: true, label: config.label, value: rendered, unit: finalUnit }
}

export function resolveHtml(config: HtmlWidgetConfig, fields: LocalViewerFields, widgetId?: string): ResolvedHtml {
  if (!config || typeof config.html !== 'string') {
    return { available: false, html: '' }
  }

  const primaryField =
    config.field || (config.entityRelation?.enabled ? generateRelationFieldKey(config.entityRelation) : undefined)
  let hasPrimary = false
  if (primaryField) {
    const val = ownField(fields, primaryField)
    if (val !== undefined && val !== null) {
      hasPrimary = true
    }
  }

  let interpolated = config.html
  interpolated = interpolated.replace(/(?:\{\{|\$\{)([A-Za-z0-9_.-]+)(?:\}\}|\})/g, (_match, key) => {
    const fieldVal = ownField(fields, key)
    const sc = scalar(fieldVal)
    if (sc !== undefined && sc !== null) {
      return String(sc)
    }
    return ''
  })

  if (primaryField) {
    const primaryVal = scalar(ownField(fields, primaryField))
    if (primaryVal !== undefined && primaryVal !== null) {
      interpolated = interpolated.replaceAll('{{value}}', String(primaryVal))
      interpolated = interpolated.replaceAll('${value}', String(primaryVal))
    }
  }

  const available = primaryField ? hasPrimary : true
  if (!available && config.fallback) {
    interpolated = config.fallback
  }

  const cleanHtml = sanitizeHtml(interpolated)
  const scopeSelector = widgetId ? `[data-widget-id="${widgetId}"]` : undefined
  const scopedCss = config.css ? sanitizeCss(config.css, scopeSelector) : undefined

  return {
    available,
    html: cleanHtml,
    ...(scopedCss ? { scopedCss } : {})
  }
}

function chartData(
  config: ChartWidgetConfig,
  fields: LocalViewerFields
): { categories: string[]; values: number[] } | null {
  if (config.categories && config.values) {
    return { categories: [...config.categories], values: [...config.values] }
  }
  const categoryField =
    config.categoryField ||
    (config.entityRelation?.enabled ? `${generateRelationFieldKey(config.entityRelation)}_cats` : undefined)
  const valueField =
    config.valueField || (config.entityRelation?.enabled ? generateRelationFieldKey(config.entityRelation) : undefined)
  if (!categoryField || !valueField) return null
  const categories = ownField(fields, categoryField)
  const values = ownField(fields, valueField)
  if (!Array.isArray(categories) || !Array.isArray(values) || categories.length !== values.length) return null
  if (categories.length > LOCAL_VIEWER_LIMITS.dataPoints) return null
  if (!categories.every((item) => typeof item === 'string' || typeof item === 'number')) return null
  if (!values.every((item) => typeof item === 'number' && Number.isFinite(item))) return null
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

  let values = data.values
  let displayUnit = config.unit ?? ''
  let unitConverted = false
  let targetUnitSymbol = ''

  if (config.unit && config.unitConversion?.enabled) {
    try {
      if (config.unitConversion.targetUnit) {
        targetUnitSymbol = config.unitConversion.targetUnit.trim()
        values = convertSeries(values, config.unit.trim(), targetUnitSymbol)
        displayUnit = config.unitConversion.targetUnit
        unitConverted = true
      } else if (config.unitConversion.unitSystem) {
        const sample = convertUnitToSystem(0, config.unit.trim(), config.unitConversion.unitSystem)
        targetUnitSymbol = sample.unit
        values = convertSeries(values, config.unit.trim(), targetUnitSymbol)
        displayUnit = targetUnitSymbol
        unitConverted = true
      }
    } catch {
      // Gracefully fallback to unconverted values
      values = data.values
      displayUnit = config.unit ?? ''
    }
  }

  const isLine = type === 'line-chart'
  const isSmooth = isLine && config.chartStyle === 'smooth'
  const isArea = isLine && config.chartStyle === 'area'

  let yMin = config.yMin
  let yMax = config.yMax
  if (unitConverted && targetUnitSymbol && config.unit) {
    try {
      if (yMin !== undefined) yMin = convertUnit(yMin, config.unit.trim(), targetUnitSymbol)
      if (yMax !== undefined) yMax = convertUnit(yMax, config.unit.trim(), targetUnitSymbol)
    } catch {
      // ignore
    }
  }

  const yAxisConfig: Record<string, unknown> = { type: 'value' }
  if (displayUnit) yAxisConfig.name = displayUnit
  if (yMin !== undefined) yAxisConfig.min = yMin
  if (yMax !== undefined) yAxisConfig.max = yMax

  let thresholdValue = config.threshold?.value
  if (unitConverted && targetUnitSymbol && config.unit && typeof thresholdValue === 'number') {
    try {
      thresholdValue = convertUnit(thresholdValue, config.unit.trim(), targetUnitSymbol)
    } catch {
      // ignore
    }
  }

  const seriesItem: Record<string, unknown> = {
    name: config.seriesName ?? '',
    type: isLine ? 'line' : 'bar',
    data: values,
    ...(isLine
      ? {
          smooth: isSmooth,
          showSymbol: values.length <= 100,
          ...(isArea ? { areaStyle: { opacity: 0.25 } } : {})
        }
      : {})
  }

  if (config.colorTheme) {
    seriesItem.itemStyle = { color: config.colorTheme }
  }

  if (config.threshold?.enabled && typeof thresholdValue === 'number') {
    seriesItem.markLine = {
      data: [
        {
          yAxis: thresholdValue,
          name: config.threshold.label || '阈值',
          lineStyle: {
            color: config.threshold.color || '#ef4444',
            type: 'dashed'
          }
        }
      ]
    }
  }

  return {
    available: true,
    option: {
      title: { text: config.title ?? '', left: 'center' },
      tooltip: { trigger: 'axis' },
      grid: { left: 40, right: 20, top: config.title ? 48 : displayUnit ? 36 : 20, bottom: 32, containLabel: true },
      xAxis: { type: 'category', data: data.categories },
      yAxis: yAxisConfig,
      series: [seriesItem]
    }
  }
}

export function resolveMapData(config: MapWidgetConfig, fields: LocalViewerFields): ResolvedMapData {
  const latKey = config.latField || 'latitude'
  const lngKey = config.lngField || 'longitude'
  const latRaw = scalar(ownField(fields, latKey))
  const lngRaw = scalar(ownField(fields, lngKey))

  let numLat: number | undefined
  if (typeof latRaw === 'number' && Number.isFinite(latRaw)) {
    numLat = latRaw
  } else if (typeof latRaw === 'string') {
    const parsed = parseFloat(latRaw)
    if (Number.isFinite(parsed)) numLat = parsed
  }

  let numLng: number | undefined
  if (typeof lngRaw === 'number' && Number.isFinite(lngRaw)) {
    numLng = lngRaw
  } else if (typeof lngRaw === 'string') {
    const parsed = parseFloat(lngRaw)
    if (Number.isFinite(parsed)) numLng = parsed
  }

  const hasCoords =
    numLat !== undefined && numLat >= -90 && numLat <= 90 && numLng !== undefined && numLng >= -180 && numLng <= 180

  if (!hasCoords && config.defaultLat === undefined) {
    return {
      available: false,
      title: config.title,
      fallback: config.fallback ?? 'No GPS coordinates available'
    }
  }

  return {
    available: true,
    title: config.title,
    lat: hasCoords ? numLat : config.defaultLat,
    lng: hasCoords ? numLng : config.defaultLng
  }
}
