import {
  LOCAL_VIEWER_LIMITS,
  type ChartWidgetConfig,
  type LocalFieldValue,
  type LocalViewerFields,
  type LocalWidgetConfig,
  type LocalWidgetType,
  type NormalizeDashboardResult,
  type NormalizeFieldsResult,
  type NormalizedLocalWidget
} from './types'

const FORBIDDEN_KEY = /(?:script|formatter|function|url|uri|endpoint|api|websocket|datasource|device)/i
const REMOTE_VALUE = /(?:https?:\/\/|wss?:\/\/|data:|javascript:)/i
const ID_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/
const FIELD_PATTERN = /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/
const TYPE_ALIASES: Readonly<Record<string, LocalWidgetType>> = {
  text: 'text',
  metric: 'metric',
  line: 'line-chart',
  'line-chart': 'line-chart',
  bar: 'bar-chart',
  'bar-chart': 'bar-chart'
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
    if (value.length > LOCAL_VIEWER_LIMITS.stringLength) fail(`${path} is too long`)
    if (REMOTE_VALUE.test(value)) fail(`${path} contains a remote or executable URL`)
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

function normalizeConfig(type: LocalWidgetType, value: unknown, path: string): LocalWidgetConfig {
  if (!isPlainRecord(value)) fail(`${path} must be a plain object`)

  if (type === 'text') {
    assertKeys(value, ['text', 'field', 'fallback'], path)
    return Object.freeze({
      text: shortString(value.text, `${path}.text`) as string,
      field: fieldName(value.field, `${path}.field`),
      fallback: shortString(value.fallback, `${path}.fallback`, false)
    })
  }
  if (type === 'metric') {
    assertKeys(value, ['label', 'field', 'unit', 'decimals', 'fallback'], path)
    return Object.freeze({
      label: shortString(value.label, `${path}.label`) as string,
      field: fieldName(value.field, `${path}.field`, true) as string,
      unit: shortString(value.unit, `${path}.unit`, false),
      decimals: value.decimals === undefined ? undefined : integer(value.decimals, `${path}.decimals`, 0, 6),
      fallback: shortString(value.fallback, `${path}.fallback`, false)
    })
  }

  assertKeys(value, ['title', 'categoryField', 'valueField', 'categories', 'values', 'seriesName'], path)
  const config: ChartWidgetConfig = {
    title: shortString(value.title, `${path}.title`, false),
    categoryField: fieldName(value.categoryField, `${path}.categoryField`),
    valueField: fieldName(value.valueField, `${path}.valueField`),
    categories: stringArray(value.categories, `${path}.categories`),
    values: numberArray(value.values, `${path}.values`),
    seriesName: shortString(value.seriesName, `${path}.seriesName`, false)
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
  assertKeys(value, ['id', 'i', 'x', 'y', 'w', 'h', 'type', 'componentType', 'config', 'properties'], path)

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

  return Object.freeze({
    id: normalizedId,
    x,
    y,
    w,
    h,
    type: type ?? 'unsupported',
    originalType,
    config: type ? normalizeConfig(type, value.config ?? value.properties ?? {}, `${path}.config`) : Object.freeze({})
  })
}

export function normalizeLocalDashboard(input: unknown): NormalizeDashboardResult {
  try {
    assertSafeTree(input)
    if (!isPlainRecord(input)) fail('dashboard must be a plain object')
    assertKeys(input, ['version', 'columns', 'rowHeight', 'widgets', 'layout'], 'dashboard')
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
    return { ok: true, dashboard: Object.freeze({ version: 1, columns, rowHeight, widgets: Object.freeze(widgets) }) }
  } catch (error) {
    return { ok: false, error: error instanceof InvalidDashboard ? error.message : 'dashboard is invalid' }
  }
}
