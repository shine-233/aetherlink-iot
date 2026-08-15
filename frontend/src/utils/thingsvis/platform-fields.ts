/*
 * 文件用途：从 ThingsVis 物模型中提取平台字段定义。
 * 核心逻辑：遍历 telemetry、attributes、commands、events 字段段落，并归一化字段类型为 number/string/boolean/json。
 * 关键注意事项：字段类型映射会影响看板绑定、实时数据展示和后续平台字段请求。
 * 重构建议：建议补充未知类型、空物模型、字符串 JSON 和嵌套字段的单元测试。
 */

import type { PlatformField } from './types'

/**
 * 从物模型中提取平台字段。
 * @param template 物模型数据。
 * @returns 平台字段数组。
 */
export function extractPlatformFields(template: any): PlatformField[] {
  if (!template) return []

  try {
    return platformFieldSections().flatMap(({ key, dataType }) => extractFieldSection(template[key], dataType))
  } catch (error) {
    console.error('Failed to extract platform fields:', error)
    return []
  }
}

export function mergePlatformFieldsById(primary: PlatformField[], fallback: PlatformField[]): PlatformField[] {
  const seen = new Set<string>()
  const merged: PlatformField[] = []

  for (const field of [...primary, ...fallback]) {
    if (!field?.id || seen.has(field.id)) continue
    seen.add(field.id)
    merged.push(field)
  }

  return merged
}

function platformFieldSections(): Array<{ key: string; dataType: PlatformField['dataType'] }> {
  return [
    { key: 'telemetry', dataType: 'telemetry' },
    { key: 'attributes', dataType: 'attribute' },
    { key: 'commands', dataType: 'command' },
    { key: 'events', dataType: 'event' }
  ]
}

function extractFieldSection(section: unknown, dataType: PlatformField['dataType']): PlatformField[] {
  const items = parseFieldSection(section)
  if (!Array.isArray(items)) {
    return []
  }

  return items
    .map((item) => normalizePlatformField(item, dataType))
    .filter((field): field is PlatformField => field !== null)
}

function parseFieldSection(section: unknown): unknown {
  if (typeof section === 'string') {
    return JSON.parse(section)
  }
  return section
}

function normalizePlatformField(item: any, dataType: PlatformField['dataType']): PlatformField | null {
  const id = item?.key || item?.data_identifier || item?.identifier || item?.id
  const name = item?.name || item?.data_name || item?.label || id
  if (!id) return null

  return {
    id,
    name: name || id,
    type: mapDataType(item?.data_type || item?.type),
    dataType,
    unit: item?.unit,
    description: item?.description || item?.define
  }
}

/**
 * 将物模型数据类型映射为 ThingsVis 支持的字段类型。
 */
function mapDataType(type: string): 'number' | 'string' | 'boolean' | 'json' {
  if (!type) return 'string'

  const lowerType = type.toLowerCase()

  if (
    lowerType.includes('int') ||
    lowerType.includes('float') ||
    lowerType.includes('double') ||
    lowerType === 'number'
  ) {
    return 'number'
  }

  if (lowerType.includes('bool')) {
    return 'boolean'
  }

  if (lowerType.includes('json') || lowerType.includes('object') || lowerType.includes('array')) {
    return 'json'
  }

  return 'string'
}
