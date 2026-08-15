import type { BuiltInTemplateDefinition } from './definition-types'

/**
 * 数据获取类模板：用于生成设备数据、接口数据和其他输入片段。
 */
export const DATA_FETCHER_TEMPLATES: BuiltInTemplateDefinition[] = [
  {
    name: '模拟设备数据',
    description: '生成模拟的IoT设备数据，包含温度、湿度、状态等信息',
    category: 'data-generation',
    code: `// 生成模拟设备数据
const deviceId = context.deviceId || 'replace_with_device_id'
const timestamp = Date.now()

return {
  deviceId: deviceId,
  timestamp: timestamp,
  data: {
    temperature: Math.round((Math.random() * 40 + 10) * 100) / 100, // 10-50°C
    humidity: Math.round((Math.random() * 60 + 30) * 100) / 100,    // 30-90%
    pressure: Math.round((Math.random() * 200 + 900) * 100) / 100,  // 900-1100 hPa
    battery: Math.round(Math.random() * 100),                       // 0-100%
    status: Math.random() > 0.1 ? 'online' : 'offline',
    location: {
      lat: 39.9042 + (Math.random() - 0.5) * 0.1,
      lng: 116.4074 + (Math.random() - 0.5) * 0.1
    }
  },
  quality: Math.random() > 0.05 ? 'good' : 'poor'
}`,
    parameters: [
      {
        name: 'deviceId',
        type: 'string',
        description: '设备ID',
        required: false,
        defaultValue: 'replace_with_device_id'
      }
    ],
    usageSnippet: '// context = { deviceId: "replace_with_device_id" }',
    isSystem: true
  },

  {
    name: '遥测趋势数据',
    description: '生成设备遥测趋势数组，适用于温湿度、信号强度和在线状态图表展示',
    category: 'data-generation',
    code: `// 生成时序数据
const points = context.points || 24
const startTime = context.startTime || (Date.now() - 24 * 60 * 60 * 1000)
const interval = context.interval || (60 * 60 * 1000) // 1小时间隔

const data = []
let baseValue = context.baseValue || 20
const variationRange = context.variation ?? 5
let currentTime = startTime

for (let i = 0; i < points; i++) {
  // 添加随机波动
  const variation = (Math.random() - 0.5) * variationRange
  const value = Math.max(0, baseValue + variation + Math.sin(i / points * Math.PI * 2) * 10)

  data.push({
    deviceId: context.deviceId || 'replace_with_device_id',
    metric: context.metric || 'temperature',
    timestamp: currentTime,
    value: Math.round(value * 100) / 100,
    label: new Date(currentTime).toLocaleTimeString()
  })

  currentTime += interval
  baseValue += (Math.random() - 0.5) * 2 // 趋势变化
}

return data`,
    parameters: [
      {
        name: 'points',
        type: 'number',
        description: '数据点数量',
        required: false,
        defaultValue: 24
      },
      {
        name: 'baseValue',
        type: 'number',
        description: '基础数值',
        required: false,
        defaultValue: 20
      },
      {
        name: 'variation',
        type: 'number',
        description: '波动范围',
        required: false,
        defaultValue: 5
      }
    ],
    usageSnippet: '// context = { deviceId: "replace_with_device_id", points: 48, baseValue: 25, variation: 8, metric: "temperature" }',
    isSystem: true
  },

  {
    name: 'HTTP API 数据获取',
    description: '通过宿主审计适配器获取 HTTP 数据；本地执行默认返回 SCRIPT_NETWORK_EXTERNAL_BLOCKED',
    category: 'api-integration',
    code: `// HTTP API 数据获取（需要宿主提供可取消、可审计的网络适配器）
const url = context.url || '/api/v1/telemetry/datas/current/replace_with_device_id'
const method = (context.method || 'GET').toUpperCase()
const headers = context.headers || { 'Content-Type': 'application/json' }
const network = _utils.networkUtils

try {
  let data
  if (method === 'GET') {
    data = await network.httpGet(url, { headers })
  } else if (method === 'POST') {
    data = await network.httpPost(url, context.body, { headers })
  } else if (method === 'PUT') {
    data = await network.httpPut(url, context.body, { headers })
  } else if (method === 'DELETE') {
    data = await network.httpDelete(url, { headers })
  } else {
    throw new Error(\`不支持的 HTTP 方法: \${method}\`)
  }

  return {
    success: true,
    data: data,
    timestamp: Date.now(),
    source: url
  }
} catch (error) {
  console.error('API调用失败:', error)
  return {
    success: false,
    error: error.message,
    timestamp: Date.now(),
    source: url
  }
}`,
    parameters: [
      {
        name: 'url',
        type: 'string',
        description: 'API地址（需要宿主审计网络适配器）',
        required: true
      },
      {
        name: 'method',
        type: 'string',
        description: 'HTTP方法：GET、POST、PUT 或 DELETE',
        required: false,
        defaultValue: 'GET'
      }
    ],
    usageSnippet:
      '// 当前本地执行默认返回 SCRIPT_NETWORK_EXTERNAL_BLOCKED；接入宿主审计适配器后使用 context = { url: "/api/v1/telemetry/datas/current/{deviceId}", method: "GET" }',
    isSystem: true
  }
]
