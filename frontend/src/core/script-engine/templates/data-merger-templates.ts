import type { BuiltInTemplateDefinition } from './definition-types'

/**
 * 数据合并类模板：用于多源数据合并、去重和选择。
 */
export const DATA_MERGER_TEMPLATES: BuiltInTemplateDefinition[] = [
  {
    name: '设备遥测对象合并',
    description: '合并多路设备遥测对象，处理重复指标和数据类型差异',
    category: 'data-processing',
    code: `// 设备遥测对象合并
if (!Array.isArray(items) || items.length === 0) {
  return {}
}

const result = {}
const metadata = {
  sources: items.length,
  conflicts: [],
  dataTypes: {},
  mergedFields: []
}

items.forEach((item, index) => {
  if (!item || typeof item !== 'object') return

  Object.keys(item).forEach(key => {
    const value = item[key]

    // 记录数据类型
    const valueType = Array.isArray(value) ? 'array' : typeof value
    if (!metadata.dataTypes[key]) {
      metadata.dataTypes[key] = []
    }
    if (!metadata.dataTypes[key].includes(valueType)) {
      metadata.dataTypes[key].push(valueType)
    }

    if (result[key] === undefined) {
      // 首次赋值
      result[key] = value
      metadata.mergedFields.push(key)
    } else {
      // 处理冲突
      if (result[key] !== value) {
        metadata.conflicts.push({
          field: key,
          values: [result[key], value],
          sources: [\`telemetry_\${index-1}\`, \`telemetry_\${index}\`]
        })

        // 合并策略
        if (typeof result[key] === 'number' && typeof value === 'number') {
          // 数值求平均
          result[key] = (result[key] + value) / 2
        } else if (Array.isArray(result[key]) && Array.isArray(value)) {
          // 数组合并去重
          result[key] = [...new Set([...result[key], ...value])]
        } else if (typeof result[key] === 'string' && typeof value === 'string') {
          // 字符串拼接
          result[key] = \`\${result[key]}, \${value}\`
        } else {
          // 保持原值或使用最新值
          result[key] = value
        }
      }
    }
  })
})

return {
  ...result,
  _metadata: metadata,
  _mergeInfo: {
    timestamp: Date.now(),
    strategy: 'intelligent',
    itemsProcessed: items.length,
    fieldsCount: Object.keys(result).length - 2 // 排除元数据
  }
}`,
    parameters: [],
    usageSnippet: '// items = [{ device_id: "replace_with_device_id", temperature: 25 }, { device_id: "replace_with_device_id", humidity: 60 }]',
    isSystem: true
  },

  {
    name: '时序数据合并',
    description: '按时间戳合并多个时序数据数组',
    category: 'time-series',
    code: `// 时序数据合并
if (!Array.isArray(items) || items.length === 0) {
  return []
}

// 收集所有时序数据点
const allPoints = []
const sources = []

items.forEach((item, index) => {
  if (Array.isArray(item)) {
    // 直接是时序数组
    item.forEach(point => {
      if (point && typeof point.timestamp === 'number') {
        allPoints.push({
          ...point,
          sourceIndex: index,
          sourceName: \`telemetry_source_\${index}\`
        })
      }
    })
    sources.push(\`telemetry_array_\${index}\`)
  } else if (item && typeof item === 'object') {
    if (typeof item.timestamp === 'number') {
      // 单个时序点
      allPoints.push({
        ...item,
        sourceIndex: index,
        sourceName: \`telemetry_source_\${index}\`
      })
      sources.push(\`telemetry_point_\${index}\`)
    } else if (item.data && Array.isArray(item.data)) {
      // 包含data字段的对象
      item.data.forEach(point => {
        if (point && typeof point.timestamp === 'number') {
          allPoints.push({
            ...point,
            sourceIndex: index,
            sourceName: item.name || \`telemetry_source_\${index}\`
          })
        }
      })
      sources.push(item.name || \`telemetry_payload_\${index}\`)
    }
  }
})

// 按时间戳排序
allPoints.sort((a, b) => a.timestamp - b.timestamp)

// 合并相同时间戳的数据点
const merged = []
const timeGroups = {}

allPoints.forEach(point => {
  const timeKey = point.timestamp
  if (!timeGroups[timeKey]) {
    timeGroups[timeKey] = []
  }
  timeGroups[timeKey].push(point)
})

// 生成合并后的数据
Object.keys(timeGroups).forEach(timestamp => {
  const points = timeGroups[timestamp]
  const mergedPoint = {
    timestamp: parseInt(timestamp),
    values: {},
    sources: [],
    count: points.length
  }

  points.forEach(point => {
    mergedPoint.sources.push(point.sourceName)

    // 合并数值字段
    Object.keys(point).forEach(key => {
      if (key !== 'timestamp' && key !== 'sourceIndex' && key !== 'sourceName') {
        if (typeof point[key] === 'number') {
          if (!mergedPoint.values[key]) {
            mergedPoint.values[key] = []
          }
          mergedPoint.values[key].push(point[key])
        } else if (point[key] !== undefined) {
          mergedPoint[key] = point[key] // 保留非数值字段
        }
      }
    })
  })

  // 计算数值字段的统计值
  Object.keys(mergedPoint.values).forEach(key => {
    const values = mergedPoint.values[key]
    mergedPoint.values[key] = {
      raw: values,
      avg: values.reduce((sum, v) => sum + v, 0) / values.length,
      min: Math.min(...values),
      max: Math.max(...values),
      sum: values.reduce((sum, v) => sum + v, 0)
    }
  })

  merged.push(mergedPoint)
})

return merged`,
    parameters: [],
    usageSnippet: '// items = [temperatureRows, humidityRows] where rows contain {deviceId, timestamp, value}',
    isSystem: true
  },

  {
    name: '最佳遥测点选择',
    description: '按时间、数值或质量从多路遥测中选择最可信的数据点',
    category: 'data-processing',
    code: `// 最佳遥测点选择
if (!Array.isArray(items) || items.length === 0) {
  return null
}

const criteria = context?.criteria || 'latest' // latest, highest, lowest, quality
const valueField = context?.valueField || 'temperature'
const timestampField = context?.timestampField || 'timestamp'
const qualityField = context?.qualityField || 'quality'

let selected = null
let reason = ''

switch (criteria) {
  case 'latest':
    // 选择时间戳最新的遥测点
    selected = items.reduce((latest, item) => {
      if (!latest) return item
      const itemTime = item[timestampField] || 0
      const latestTime = latest[timestampField] || 0
      return itemTime > latestTime ? item : latest
    }, null)
    reason = '选择最新上报的遥测点'
    break

  case 'highest':
    // 选择数值最高的遥测点
    selected = items.reduce((highest, item) => {
      if (!highest) return item
      const itemValue = item[valueField] || 0
      const highestValue = highest[valueField] || 0
      return itemValue > highestValue ? item : highest
    }, null)
    reason = \`选择\${valueField}指标值最高的遥测点\`
    break

  case 'lowest':
    // 选择数值最低的遥测点
    selected = items.reduce((lowest, item) => {
      if (!lowest) return item
      const itemValue = item[valueField] || Number.MAX_VALUE
      const lowestValue = lowest[valueField] || Number.MAX_VALUE
      return itemValue < lowestValue ? item : lowest
    }, null)
    reason = \`选择\${valueField}指标值最低的遥测点\`
    break

  case 'quality':
    // 选择质量最好的遥测点
    const qualityOrder = ['excellent', 'good', 'fair', 'poor']
    selected = items.reduce((best, item) => {
      if (!best) return item
      const itemQuality = item[qualityField] || 'poor'
      const bestQuality = best[qualityField] || 'poor'
      const itemIndex = qualityOrder.indexOf(itemQuality)
      const bestIndex = qualityOrder.indexOf(bestQuality)
      return (itemIndex !== -1 && (bestIndex === -1 || itemIndex < bestIndex)) ? item : best
    }, null)
    reason = \`选择\${qualityField}质量最好的遥测点\`
    break

  default:
    selected = items[0]
    reason = '使用默认选择（第一条遥测点）'
}

// 添加选择信息
return {
  ...selected,
  _selectionInfo: {
    criteria: criteria,
    reason: reason,
    totalItems: items.length,
    selectedIndex: items.indexOf(selected),
    timestamp: Date.now(),
    alternatives: items.length - 1
  }
}`,
    parameters: [
      {
        name: 'criteria',
        type: 'string',
        description: '选择条件',
        required: false,
        defaultValue: 'latest',
        validation: {
          enum: ['latest', 'highest', 'lowest', 'quality']
        }
      },
      {
        name: 'valueField',
        type: 'string',
        description: '比较的数值字段',
        required: false,
        defaultValue: 'temperature'
      }
    ],
    usageSnippet: '// context = { criteria: "highest", valueField: "temperature" }',
    isSystem: true
  }
]
