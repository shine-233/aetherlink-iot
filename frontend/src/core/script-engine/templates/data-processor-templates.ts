import type { BuiltInTemplateDefinition } from './definition-types'

/**
 * 数据处理类模板：用于清洗、转换、聚合和格式化输入数据。
 */
export const DATA_PROCESSOR_TEMPLATES: BuiltInTemplateDefinition[] = [
  {
    name: '遥测数值摘要',
    description: '对设备遥测数值进行平均值、最大值、最小值和健康等级计算',
    category: 'data-processing',
    code: `// 遥测数值摘要
if (!data || typeof data !== 'object') {
  return { error: '数据格式不正确' }
}

// 提取数值字段
const numericFields = Object.keys(data).filter(key =>
  typeof data[key] === 'number' && !isNaN(data[key])
)

if (numericFields.length === 0) {
  return { error: '未找到数值字段' }
}

const result = {
  original: data,
  processed: {
    sum: numericFields.reduce((sum, key) => sum + data[key], 0),
    average: numericFields.reduce((sum, key) => sum + data[key], 0) / numericFields.length,
    max: Math.max(...numericFields.map(key => data[key])),
    min: Math.min(...numericFields.map(key => data[key])),
    count: numericFields.length
  },
  fields: numericFields,
  timestamp: Date.now()
}

// 添加计算标志
result.processed.isValid = result.processed.average > 0
result.processed.level = result.processed.average > 50 ? 'high' :
                        result.processed.average > 20 ? 'medium' : 'low'

return result`,
    parameters: [],
    usageSnippet: '// data = { temperature: 25.5, humidity: 60, pressure: 1013.2 }',
    isSystem: true
  },

  {
    name: '异常遥测过滤',
    description: '对设备遥测数组进行阈值过滤、排序和分组处理',
    category: 'data-processing',
    code: `// 异常遥测过滤处理
if (!Array.isArray(data)) {
  return { error: '输入数据不是数组' }
}

const filterValue = context?.filterValue || 35
const sortField = context?.sortField || 'temperature'
const groupField = context?.groupField || 'device_type'

// 过滤数据
const filtered = data.filter(item => {
  if (typeof item === 'number') return item > filterValue
  if (typeof item === 'object' && item[sortField] !== undefined) {
    return item[sortField] > filterValue
  }
  return true
})

// 排序数据
const sorted = filtered.sort((a, b) => {
  const aVal = typeof a === 'object' ? a[sortField] : a
  const bVal = typeof b === 'object' ? b[sortField] : b
  return (aVal || 0) - (bVal || 0)
})

// 分组数据
const grouped = {}
sorted.forEach(item => {
  const groupKey = typeof item === 'object' ?
    (item[groupField] || 'default') : 'values'

  if (!grouped[groupKey]) {
    grouped[groupKey] = []
  }
  grouped[groupKey].push(item)
})

return {
  original: { count: data.length },
  filtered: { count: filtered.length, data: filtered },
  sorted: { count: sorted.length, data: sorted },
  grouped: grouped,
  summary: {
    totalItems: data.length,
    filteredItems: filtered.length,
    groups: Object.keys(grouped).length,
    filterCriteria: \`\${sortField} > \${filterValue}\`,
    sortBy: sortField
  }
}`,
    parameters: [
      {
        name: 'filterValue',
        type: 'number',
        description: '过滤阈值',
        required: false,
        defaultValue: 35
      },
      {
        name: 'sortField',
        type: 'string',
        description: '排序字段',
        required: false,
        defaultValue: 'temperature'
      },
      {
        name: 'groupField',
        type: 'string',
        description: '分组字段',
        required: false,
        defaultValue: 'device_type'
      }
    ],
    usageSnippet: '// data = [{ device_id: "replace_with_device_id", temperature: 38, device_type: "gateway" }]',
    isSystem: true
  },

  {
    name: '时间数据格式化',
    description: '对包含时间戳的数据进行格式化和时间计算',
    category: 'transformation',
    code: `// 时间数据格式化处理
const now = Date.now()
const timezone = context?.timezone || 'Asia/Shanghai'

// 处理时间戳字段
function formatTimestamp(timestamp) {
  const date = new Date(timestamp)
  return {
    timestamp: timestamp,
    iso: date.toISOString(),
    local: date.toLocaleString('zh-CN', { timeZone: timezone }),
    date: date.toLocaleDateString('zh-CN'),
    time: date.toLocaleTimeString('zh-CN'),
    age: now - timestamp, // 数据年龄（毫秒）
    ageText: getAgeText(now - timestamp)
  }
}

function getAgeText(age) {
  const seconds = Math.floor(age / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) return \`\${days}天前\`
  if (hours > 0) return \`\${hours}小时前\`
  if (minutes > 0) return \`\${minutes}分钟前\`
  return \`\${seconds}秒前\`
}

// 处理输入数据
if (typeof data === 'number') {
  // 单个时间戳
  return formatTimestamp(data)
} else if (Array.isArray(data)) {
  // 时间戳数组
  return data.map(item => {
    if (typeof item === 'number') {
      return formatTimestamp(item)
    } else if (typeof item === 'object' && item.timestamp) {
      return { ...item, timeInfo: formatTimestamp(item.timestamp) }
    }
    return item
  })
} else if (typeof data === 'object' && data.timestamp) {
  // 包含时间戳的对象
  return {
    ...data,
    timeInfo: formatTimestamp(data.timestamp)
  }
}

// 添加当前时间信息
return {
  originalData: data,
  currentTime: formatTimestamp(now),
  processed: true
}`,
    parameters: [
      {
        name: 'timezone',
        type: 'string',
        description: '时区',
        required: false,
        defaultValue: 'Asia/Shanghai'
      }
    ],
    usageSnippet: '// data = { deviceId: "replace_with_device_id", value: 25, timestamp: Date.now() }',
    isSystem: true
  }
]
