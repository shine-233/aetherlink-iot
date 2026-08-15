/*
 * 文件用途：解析 ThingsVis 物模型图表预设配置。
 * 核心逻辑：从物模型图表配置中提取按属性类型和字段 ID 组织的预设 widget。
 * 关键注意事项：配置结构可能来自后端或第三方模板，需要容错未知字段。
 * 重构建议：建议补充非法 JSON、缺字段和多预设合并测试。
 */
import type { PlatformField } from './types'

export type TemplatePresetPropertyType = 'telemetry' | 'attributes'

type TemplatePresetWidget = Record<string, unknown>

type TemplatePresetEntry = {
  id?: string
  name?: string
  widget?: TemplatePresetWidget
  thumbnail?: string
  [key: string]: unknown
}

type TemplatePresetMap = Record<string, TemplatePresetEntry[]>

type TemplateChartConfig = Record<string, unknown> & {
  device_widget_presets?: TemplatePresetMap
  nodes?: TemplatePresetWidget[]
  canvas?: Record<string, unknown>
  dataSources?: unknown[]
}

const DEFAULT_PRESET_CANVAS = {
  mode: 'grid',
  width: 1920,
  height: 1080,
  gridCols: 24,
  gridRowHeight: 50,
  gridGap: 5
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function parseTemplateChartConfig(rawConfig: unknown): TemplateChartConfig {
  if (typeof rawConfig === 'string') {
    if (!rawConfig.trim()) return {}
    try {
      const parsed = JSON.parse(rawConfig)
      return isRecord(parsed) ? (parsed as TemplateChartConfig) : {}
    } catch (error) {
      console.warn('[template-presets] Failed to parse chart config:', error)
      return {}
    }
  }

  if (isRecord(rawConfig)) {
    return rawConfig as TemplateChartConfig
  }

  return {}
}

export function getTemplatePresetKey(propertyType: TemplatePresetPropertyType, fieldId: string) {
  return `${propertyType}_${fieldId}`
}

export function getTemplatePresetMap(rawConfig: unknown): TemplatePresetMap {
  const config = parseTemplateChartConfig(rawConfig)
  return isRecord(config.device_widget_presets) ? config.device_widget_presets : {}
}

export function getTemplatePresetEntries(
  rawConfig: unknown,
  propertyType: TemplatePresetPropertyType,
  fieldId: string
): TemplatePresetEntry[] {
  const presets = getTemplatePresetMap(rawConfig)
  const entries = presets[getTemplatePresetKey(propertyType, fieldId)]
  return Array.isArray(entries) ? (entries.filter(isRecord) as TemplatePresetEntry[]) : []
}

export function buildPresetEditorConfig(widget?: TemplatePresetWidget | null) {
  if (!isRecord(widget)) return null

  return {
    meta: {
      id: 'template-preset',
      name: '物模型预设'
    },
    canvas: DEFAULT_PRESET_CANVAS,
    nodes: [widget],
    dataSources: []
  }
}

export function extractFirstNodeFromWidgetConfig(payload: unknown): TemplatePresetWidget | null {
  if (!isRecord(payload)) return null

  if (Array.isArray(payload.nodes) && payload.nodes.length > 0 && isRecord(payload.nodes[0])) {
    return payload.nodes[0] as TemplatePresetWidget
  }

  if (isRecord(payload.nodesById)) {
    const firstNode = Object.values(payload.nodesById).find(isRecord)
    if (firstNode) return firstNode as TemplatePresetWidget
  }

  return null
}

function resolvePresetFieldMeta(
  fieldId: string,
  propertyType: TemplatePresetPropertyType,
  fieldMap: Map<string, PlatformField>
) {
  const field = fieldMap.get(fieldId)

  return {
    fieldId,
    fieldName: field?.name || fieldId,
    propertyType,
    dataType: field?.dataType,
    unit: field?.unit
  }
}

export function buildTemplateDevicePresets(rawConfig: unknown, fields: PlatformField[]) {
  const presetMap = getTemplatePresetMap(rawConfig)
  const fieldMap = new Map(fields.map(field => [field.id, field]))

  return Object.entries(presetMap).flatMap(([presetKey, entries]) => {
    const match = /^(telemetry|attributes)_(.+)$/.exec(presetKey)
    if (!match || !Array.isArray(entries)) return []

    const [, propertyType, fieldId] = match as unknown as [string, TemplatePresetPropertyType, string]
    const fieldMeta = resolvePresetFieldMeta(fieldId, propertyType, fieldMap)

    return entries.flatMap((entry, index) => {
      if (!isRecord(entry?.widget)) return []

      return [
        {
          id: String(entry.id || `${presetKey}_${index}`),
          name: String(entry.name || `${fieldMeta.fieldName}卡片预设`),
          widget: entry.widget,
          thumbnail: typeof entry.thumbnail === 'string' ? entry.thumbnail : undefined,
          ...fieldMeta
        }
      ]
    })
  })
}

export function hasThingsVisChartContent(rawConfig: unknown) {
  const config = parseTemplateChartConfig(rawConfig)

  if (Array.isArray(config.nodes) && config.nodes.length > 0) {
    return true
  }

  return isRecord(config.canvas) || Array.isArray(config.dataSources)
}
