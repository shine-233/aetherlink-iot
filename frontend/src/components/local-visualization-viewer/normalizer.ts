import {
  LOCAL_VIEWER_LIMITS,
  type ChartWidgetConfig,
  type HtmlWidgetConfig,
  type LocalFieldValue,
  type LocalViewerFields,
  type LocalWidgetConfig,
  type LocalWidgetType,
  type NormalizeDashboardResult,
  type NormalizeFieldsResult,
  type NormalizedLocalWidget
} from './types'
import type { TimewindowConfig } from './timewindow/types'
import type { EntityRelationAggregation, EntityRelationSourceConfig } from './entity-relation/types'
import { generateRelationFieldKey } from './entity-relation/resolver'

const FORBIDDEN_KEY = /(?:script|formatter|function|url|uri|endpoint|api|websocket|datasource|device)/i
const REMOTE_VALUE = /(?:https?:\/\/|wss?:\/\/|data:|javascript:)/i
const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/
const FIELD_PATTERN = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/
const TYPE_ALIASES: Readonly<Record<string, LocalWidgetType>> = {
  text: 'text',
  metric: 'metric',
  line: 'line-chart',
  'line-chart': 'line-chart',
  bar: 'bar-chart',
  'bar-chart': 'bar-chart',
  html: 'html',
  'html-container': 'html',
  'html-card': 'html'
}

class InvalidDashboard extends Error {}

type PlainRecord = Record<string, unknown>

function fail(message: string): never {
  throw new InvalidDashboard(message)
}

function isPlainRecord(value: unknown): value is PlainRecord {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) return false
  return Object.getPrototypeOf(value) === Object.prototype
}

function assertSafeTree(value: unknown, path = 'dashboard', depth = 0): void {
  if (depth > 12) fail(`${path} exceeds maximum depth`)
  if (typeof value === 'function' || typeof value === 'symbol' || typeof value === 'bigint') {
    fail(`${path} contains an unsupported value`)
  }
  if (typeof value === 'string') {
    const isHtmlOrCss = path.endsWith('.html') || path.endsWith('.css')
    const maxLength = isHtmlOrCss ? LOCAL_VIEWER_LIMITS.htmlLength : LOCAL_VIEWER_LIMITS.stringLength
    if (value.length > maxLength) fail(`${path} is too long`)
    if (isHtmlOrCss) {
      if (/(?:javascript:|vbscript:|data:text\/html)/i.test(value)) {
        fail(`${path} contains a remote or executable URL`)
      }
    } else {
      if (REMOTE_VALUE.test(value)) fail(`${path} contains a remote or executable URL`)
    }
    return
  }
  if (Array.isArray(value)) {
    if (value.length > LOCAL_VIEWER_LIMITS.dataPoints) fail(`${path} contains too many entries`)
    value.forEach((item, index) => assertSafeTree(item, `${path}[${index}]`, depth + 1))
    return
  }
  if (value !== null && typeof value === 'object') {
    if (!isPlainRecord(value)) fail(`${path} must be a plain object`)
    for (const key of Object.keys(value)) {
      if (key === '__proto__' || key === 'prototype' || key === 'constructor' || FORBIDDEN_KEY.test(key)) {
        fail(`${path}.${key} is forbidden`)
      }
      const descriptor = Object.getOwnPropertyDescriptor(value, key)
      if (!descriptor || !('value' in descriptor)) fail(`${path}.${key} must be a data property`)
      assertSafeTree(descriptor.value, `${path}.${key}`, depth + 1)
    }
  }
}

function assertKeys(record: PlainRecord, allowed: readonly string[], path: string): void {
  const allowedSet = new Set(allowed)
  for (const key of Object.keys(record)) {
    if (!allowedSet.has(key)) fail(`${path}.${key} is not supported`)
  }
}

function integer(value: unknown, name: string, min: number, max: number): number {
  if (!Number.isInteger(value) || (value as number) < min || (value as number) > max) {
    fail(`${name} must be an integer between ${min} and ${max}`)
  }
  return value as number
}

function shortString(value: unknown, name: string, required = true): string | undefined {
  if (value === undefined && !required) return undefined
  if (typeof value !== 'string' || (required && value.length === 0) || value.length > LOCAL_VIEWER_LIMITS.stringLength) {
    fail(`${name} must be a valid string`)
  }
  return value
}

function htmlString(value: unknown, name: string, required = true): string | undefined {
  if (value === undefined && !required) return undefined
  if (typeof value !== 'string' || (required && value.length === 0) || value.length > LOCAL_VIEWER_LIMITS.htmlLength) {
    fail(`${name} must be a valid string`)
  }
  return value
}

function fieldName(value: unknown, name: string, required = false): string | undefined {
  const field = shortString(value, name, required)
  if (field !== undefined && !FIELD_PATTERN.test(field)) fail(`${name} is invalid`)
  return field
}

export function normalizeLocalViewerFields(input: unknown): NormalizeFieldsResult {
  try {
    if (!isPlainRecord(input)) fail('fields must be a plain object')
    const entries = Object.keys(input)
    if (entries.length > LOCAL_VIEWER_LIMITS.fields) fail('fields contains too many entries')

    const fields: Record<string, LocalFieldValue> = {}
    for (const key of entries) {
      if (!FIELD_PATTERN.test(key)) fail(`fields.${key} has an invalid name`)
      const descriptor = Object.getOwnPropertyDescriptor(input, key)
      if (!descriptor || !('value' in descriptor)) fail(`fields.${key} must be a data property`)
      const value = descriptor.value
      const values = Array.isArray(value) ? value : [value]
      if (values.length > LOCAL_VIEWER_LIMITS.dataPoints) fail(`fields.${key} contains too many entries`)
      for (const item of values) {
        if (item !== null && typeof item !== 'string' && typeof item !== 'number' && typeof item !== 'boolean') {
          fail(`fields.${key} contains an unsupported value`)
        }
        if (typeof item === 'string' && item.length > LOCAL_VIEWER_LIMITS.stringLength) fail(`fields.${key} is too long`)
        if (typeof item === 'number' && !Number.isFinite(item)) fail(`fields.${key} must contain finite numbers`)
      }
      fields[key] = Array.isArray(value) ? Object.freeze([...values]) : (value as LocalFieldValue)
    }
    return { ok: true, fields: Object.freeze(fields) }
  } catch (error) {
    return { ok: false, error: error instanceof InvalidDashboard ? error.message : 'fields are invalid' }
  }
}

function numberArray(value: unknown, name: string): readonly number[] | undefined {
  if (value === undefined) return undefined
  if (!Array.isArray(value) || value.length > LOCAL_VIEWER_LIMITS.dataPoints) fail(`${name} must be a bounded array`)
  const result = value.map((item, index) => {
    if (typeof item !== 'number' || !Number.isFinite(item)) fail(`${name}[${index}] must be a finite number`)
    return item
  })
  return Object.freeze(result)
}

function stringArray(value: unknown, name: string): readonly string[] | undefined {
  if (value === undefined) return undefined
  if (!Array.isArray(value) || value.length > LOCAL_VIEWER_LIMITS.dataPoints) fail(`${name} must be a bounded array`)
  return Object.freeze(value.map((item, index) => shortString(item, `${name}[${index}]`) as string))
}

const VALID_ENTITY_TYPES = new Set(['device', 'asset', 'gateway', 'customer', 'rule_chain', 'dashboard'])
const VALID_DIRECTIONS = new Set(['from', 'to'])
const VALID_AGGREGATIONS = new Set(['latest', 'avg', 'sum', 'max', 'min', 'count'])

function normalizeEntityRelation(value: unknown, path: string): EntityRelationSourceConfig | undefined {
  if (value === undefined) return undefined
  if (!isPlainRecord(value)) fail(`${path} must be a plain object`)
  assertKeys(value, ['enabled', 'rootType', 'rootId', 'direction', 'relationType', 'targetType', 'targetKey', 'aggregation'], path)
  if (typeof value.enabled !== 'boolean') fail(`${path}.enabled must be a boolean`)
  if (!value.enabled) {
    return Object.freeze({
      enabled: false,
      rootType: typeof value.rootType === 'string' && VALID_ENTITY_TYPES.has(value.rootType) ? (value.rootType as any) : 'device',
      rootId: typeof value.rootId === 'string' ? value.rootId : '',
      direction: value.direction === 'to' ? 'to' : 'from',
      relationType: typeof value.relationType === 'string' ? value.relationType : '',
      targetType: typeof value.targetType === 'string' && VALID_ENTITY_TYPES.has(value.targetType) ? (value.targetType as any) : 'device',
      targetKey: typeof value.targetKey === 'string' ? value.targetKey : '',
      ...(typeof value.aggregation === 'string' && VALID_AGGREGATIONS.has(value.aggregation) ? { aggregation: value.aggregation as EntityRelationAggregation } : {})
    })
  }

  if (typeof value.rootType !== 'string' || !VALID_ENTITY_TYPES.has(value.rootType)) {
    fail(`${path}.rootType is invalid`)
  }
  const rootId = shortString(value.rootId, `${path}.rootId`, true) as string
  if (value.direction !== 'from' && value.direction !== 'to') {
    fail(`${path}.direction must be one of: from, to`)
  }
  const relationType = shortString(value.relationType, `${path}.relationType`, true) as string
  if (typeof value.targetType !== 'string' || !VALID_ENTITY_TYPES.has(value.targetType)) {
    fail(`${path}.targetType is invalid`)
  }
  const targetKey = fieldName(value.targetKey, `${path}.targetKey`, true) as string
  let aggregation: EntityRelationAggregation | undefined
  if (value.aggregation !== undefined) {
    if (typeof value.aggregation !== 'string' || !VALID_AGGREGATIONS.has(value.aggregation)) {
      fail(`${path}.aggregation must be one of: latest, avg, sum, max, min, count`)
    }
    aggregation = value.aggregation as EntityRelationAggregation
  }

  return Object.freeze({
    enabled: true,
    rootType: value.rootType as any,
    rootId,
    direction: value.direction as 'from' | 'to',
    relationType,
    targetType: value.targetType as any,
    targetKey,
    ...(aggregation ? { aggregation } : {})
  })
}

function normalizeConfig(type: LocalWidgetType, value: unknown, path: string): LocalWidgetConfig {
  if (!isPlainRecord(value)) fail(`${path} must be a plain object`)

  if (type === 'text') {
    assertKeys(value, ['text', 'field', 'fallback', 'entityRelation'], path)
    const entityRelation = normalizeEntityRelation(value.entityRelation, `${path}.entityRelation`)
    const rawField = fieldName(value.field, `${path}.field`)
    const field = rawField ?? (entityRelation?.enabled ? generateRelationFieldKey(entityRelation) : undefined)
    return Object.freeze({
      text: shortString(value.text, `${path}.text`) as string,
      ...(field !== undefined ? { field } : {}),
      fallback: shortString(value.fallback, `${path}.fallback`, false),
      ...(entityRelation ? { entityRelation } : {})
    })
  }
  if (type === 'metric') {
    assertKeys(value, ['label', 'field', 'unit', 'decimals', 'fallback', 'entityRelation'], path)
    const entityRelation = normalizeEntityRelation(value.entityRelation, `${path}.entityRelation`)
    const rawField = fieldName(value.field, `${path}.field`, !entityRelation?.enabled)
    const field = rawField ?? (entityRelation?.enabled ? generateRelationFieldKey(entityRelation) : undefined)
    if (!field) fail(`${path}.field must be a valid string`)
    return Object.freeze({
      label: shortString(value.label, `${path}.label`) as string,
      field,
      unit: shortString(value.unit, `${path}.unit`, false),
      decimals: value.decimals === undefined ? undefined : integer(value.decimals, `${path}.decimals`, 0, 6),
      fallback: shortString(value.fallback, `${path}.fallback`, false),
      ...(entityRelation ? { entityRelation } : {})
    })
  }

  if (type === 'html') {
    assertKeys(value, ['html', 'css', 'field', 'fields', 'fallback', 'entityRelation'], path)
    const entityRelation = normalizeEntityRelation(value.entityRelation, `${path}.entityRelation`)
    const rawField = fieldName(value.field, `${path}.field`, false)
    const field = rawField ?? (entityRelation?.enabled ? generateRelationFieldKey(entityRelation) : undefined)
    const fields = stringArray(value.fields, `${path}.fields`)
    const html = htmlString(value.html, `${path}.html`, false) ?? ''
    const css = htmlString(value.css, `${path}.css`, false)
    const fallback = shortString(value.fallback, `${path}.fallback`, false)

    return Object.freeze({
      html,
      ...(css !== undefined ? { css } : {}),
      ...(field !== undefined ? { field } : {}),
      ...(fields !== undefined ? { fields } : {}),
      ...(fallback !== undefined ? { fallback } : {}),
      ...(entityRelation ? { entityRelation } : {})
    })
  }

  assertKeys(value, ['title', 'categoryField', 'valueField', 'categories', 'values', 'seriesName', 'chartStyle', 'colorTheme', 'yMin', 'yMax', 'threshold', 'timewindow', 'entityRelation'], path)
  let chartStyle: 'line' | 'smooth' | 'area' | 'bar' | undefined
  if (value.chartStyle !== undefined) {
    if (value.chartStyle !== 'line' && value.chartStyle !== 'smooth' && value.chartStyle !== 'area' && value.chartStyle !== 'bar') {
      fail(`${path}.chartStyle must be one of: line, smooth, area, bar`)
    }
    chartStyle = value.chartStyle
  }

  const colorTheme = shortString(value.colorTheme, `${path}.colorTheme`, false)

  let yMin: number | undefined
  if (value.yMin !== undefined) {
    if (typeof value.yMin !== 'number' || !Number.isFinite(value.yMin)) fail(`${path}.yMin must be a finite number`)
    yMin = value.yMin
  }

  let yMax: number | undefined
  if (value.yMax !== undefined) {
    if (typeof value.yMax !== 'number' || !Number.isFinite(value.yMax)) fail(`${path}.yMax must be a finite number`)
    yMax = value.yMax
  }

  let threshold: ChartWidgetConfig['threshold'] | undefined
  if (value.threshold !== undefined) {
    if (!isPlainRecord(value.threshold)) fail(`${path}.threshold must be a plain object`)
    assertKeys(value.threshold, ['enabled', 'value', 'label', 'color'], `${path}.threshold`)
    if (typeof value.threshold.enabled !== 'boolean') fail(`${path}.threshold.enabled must be a boolean`)
    if (typeof value.threshold.value !== 'number' || !Number.isFinite(value.threshold.value)) fail(`${path}.threshold.value must be a finite number`)
    threshold = Object.freeze({
      enabled: value.threshold.enabled,
      value: value.threshold.value,
      label: shortString(value.threshold.label, `${path}.threshold.label`, false),
      color: shortString(value.threshold.color, `${path}.threshold.color`, false)
    })
  }

  let timewindow: TimewindowConfig | undefined
  if (value.timewindow !== undefined) {
    if (!isPlainRecord(value.timewindow)) fail(`${path}.timewindow must be a plain object`)
    timewindow = value.timewindow as unknown as TimewindowConfig
  }

  const entityRelation = normalizeEntityRelation(value.entityRelation, `${path}.entityRelation`)
  let categoryField = fieldName(value.categoryField, `${path}.categoryField`)
  let valueField = fieldName(value.valueField, `${path}.valueField`)
  if (entityRelation?.enabled) {
    if (!valueField) valueField = generateRelationFieldKey(entityRelation)
    if (!categoryField) categoryField = `${generateRelationFieldKey(entityRelation)}_cats`
  }

  const config: ChartWidgetConfig = {
    title: shortString(value.title, `${path}.title`, false),
    categoryField,
    valueField,
    categories: stringArray(value.categories, `${path}.categories`),
    values: numberArray(value.values, `${path}.values`),
    seriesName: shortString(value.seriesName, `${path}.seriesName`, false),
    ...(chartStyle ? { chartStyle } : {}),
    ...(colorTheme ? { colorTheme } : {}),
    ...(yMin !== undefined ? { yMin } : {}),
    ...(yMax !== undefined ? { yMax } : {}),
    ...(threshold ? { threshold } : {}),
    ...(timewindow ? { timewindow } : {}),
    ...(entityRelation ? { entityRelation } : {})
  }
  const hasStatic = config.categories !== undefined || config.values !== undefined
  const hasFields = config.categoryField !== undefined || config.valueField !== undefined
  if (hasStatic && hasFields) fail(`${path} cannot mix static data and field bindings`)
  if (hasStatic && (!config.categories || !config.values || config.categories.length !== config.values.length)) {
    fail(`${path} static chart data must have equal category and value lengths`)
  }
  if (hasFields && (!config.categoryField || !config.valueField)) fail(`${path} requires both chart fields`)
  if (!hasStatic && !hasFields) fail(`${path} requires static data or field bindings`)
  return Object.freeze(config)
}

function normalizeWidget(value: unknown, index: number, columns: number): NormalizedLocalWidget {
  const path = `dashboard.widgets[${index}]`
  if (!isPlainRecord(value)) fail(`${path} must be a plain object`)
  assertKeys(value, ['id', 'i', 'x', 'y', 'w', 'h', 'type', 'componentType', 'config', 'properties', 'timewindow', 'entityRelation'], path)

  const id = value.id
  const legacyId = value.i
  if (id !== undefined && legacyId !== undefined && id !== legacyId) fail(`${path} has conflicting id and i`)
  const normalizedId = shortString(id ?? legacyId, `${path}.id`) as string
  if (!ID_PATTERN.test(normalizedId)) fail(`${path}.id is invalid`)

  const rawType = value.type ?? value.componentType
  if (value.type !== undefined && value.componentType !== undefined && value.type !== value.componentType) {
    fail(`${path} has conflicting type fields`)
  }
  const originalType = shortString(rawType, `${path}.type`) as string
  const type = TYPE_ALIASES[originalType]

  if (value.config !== undefined && value.properties !== undefined) fail(`${path} has conflicting config fields`)
  const x = integer(value.x, `${path}.x`, 0, columns - 1)
  const y = integer(value.y, `${path}.y`, 0, LOCAL_VIEWER_LIMITS.rows - 1)
  const w = integer(value.w, `${path}.w`, 1, columns)
  const h = integer(value.h, `${path}.h`, 1, LOCAL_VIEWER_LIMITS.rows)
  if (x + w > columns || y + h > LOCAL_VIEWER_LIMITS.rows) fail(`${path} exceeds dashboard bounds`)

  let widgetTimewindow: TimewindowConfig | undefined
  if (value.timewindow !== undefined) {
    if (!isPlainRecord(value.timewindow)) fail(`${path}.timewindow must be a plain object`)
    widgetTimewindow = value.timewindow as unknown as TimewindowConfig
  }

  let widgetEntityRelation: EntityRelationSourceConfig | undefined
  if (value.entityRelation !== undefined) {
    widgetEntityRelation = normalizeEntityRelation(value.entityRelation, `${path}.entityRelation`)
  }

  return Object.freeze({
    id: normalizedId,
    x,
    y,
    w,
    h,
    type: type ?? 'unsupported',
    originalType,
    config: type ? normalizeConfig(type, value.config ?? value.properties ?? {}, `${path}.config`) : Object.freeze({}),
    ...(widgetTimewindow ? { timewindow: widgetTimewindow } : {}),
    ...(widgetEntityRelation ? { entityRelation: widgetEntityRelation } : {})
  })
}

export function normalizeLocalDashboard(input: unknown): NormalizeDashboardResult {
  try {
    assertSafeTree(input)
    if (!isPlainRecord(input)) fail('dashboard must be a plain object')
    assertKeys(input, ['version', 'columns', 'rowHeight', 'widgets', 'layout', 'timewindow', 'responsive'], 'dashboard')
    if (input.version !== 1) fail('dashboard.version must be 1')
    if (input.widgets !== undefined && input.layout !== undefined) fail('dashboard has conflicting widget collections')
    const source = input.widgets ?? input.layout
    if (!Array.isArray(source)) fail('dashboard.widgets must be an array')
    if (source.length > LOCAL_VIEWER_LIMITS.widgets) fail('dashboard has too many widgets')
    const columns = input.columns === undefined ? 24 : integer(input.columns, 'dashboard.columns', 1, LOCAL_VIEWER_LIMITS.columns)
    const rowHeight = input.rowHeight === undefined ? 60 : integer(input.rowHeight, 'dashboard.rowHeight', 20, 200)
    const widgets = source.map((widget, index) => normalizeWidget(widget, index, columns))
    const ids = new Set<string>()
    for (const widget of widgets) {
      if (ids.has(widget.id)) fail(`dashboard contains duplicate widget id ${widget.id}`)
      ids.add(widget.id)
    }

    let timewindow: TimewindowConfig | undefined
    if (input.timewindow !== undefined) {
      if (!isPlainRecord(input.timewindow)) fail('dashboard.timewindow must be a plain object')
      timewindow = input.timewindow as unknown as TimewindowConfig
    }

    let responsive: boolean | undefined
    if (input.responsive !== undefined) {
      if (typeof input.responsive !== 'boolean') fail('dashboard.responsive must be a boolean')
      responsive = input.responsive
    }

    return {
      ok: true,
      dashboard: Object.freeze({
        version: 1,
        columns,
        rowHeight,
        widgets: Object.freeze(widgets),
        ...(timewindow ? { timewindow } : {}),
        ...(responsive !== undefined ? { responsive } : {})
      })
    }
  } catch (error) {
    return { ok: false, error: error instanceof InvalidDashboard ? error.message : 'dashboard is invalid' }
  }
}
