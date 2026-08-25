/**
 * 文件用途: 维护 HTTP 数据源的预设模板与默认请求配置。
 * 核心逻辑: 将常见内部 API 场景映射为可复用的 HttpConfig 配置对象。
 * 关键注意事项: 模板字段会被导入导出和动态参数编辑器复用，修改路径或默认值需要同步验证兼容性。
 * 重构建议: 将模板元数据、请求默认值和展示文案拆分，便于按业务域扩展模板库。
 */
/**
 * HTTP配置模板
 * 专门维护HTTP数据源的预设配置模板
 */

import type { HttpConfig } from '@/core/data-architecture/types/http-config'
import { smartDeepClone } from '@/utils/deep-clone'

/**
 * HTTP配置模板定义
 */
const httpConfigTemplateRegistry: Array<{
  name: string
  config: HttpConfig
}> = [
  {
    name: '设备列表查询',
    config: {
      url: '/api/v1/device',
      method: 'GET',
      timeout: 5000,
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [],
      body: '',
      preRequestScript: '',
      postResponseScript: 'return response.data || response'
    }
  },
  {
    name: '浏览器发送遥测',
    config: {
      url: '/api/v1/telemetry/datas/pub',
      method: 'POST',
      timeout: 10000,
      headers: [
        {
          key: 'Content-Type',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: '内容类型'
        },
        {
          key: 'Authorization',
          value: 'Bearer ${AETHERLINK_API_TOKEN}',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_authorization',
          description: '认证令牌'
        }
      ],
      params: [],
      body: '{"device_id":"replace_with_device_id","temperature":25.5,"humidity":60,"rssi":-52,"online":true}',
      preRequestScript:
        'config.headers = config.headers || {}\nconfig.headers["X-Request-Time"] = Date.now().toString()\nreturn config',
      postResponseScript:
        'return response.data || { sent: true, latest_at: Date.now(), proof_stage: "telemetry-submit-finished" }'
    }
  },
  {
    name: '设备遥测趋势',
    config: {
      url: '/telemetry/datas/statistic',
      method: 'GET',
      timeout: 15000,
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [
        {
          key: 'device_id',
          value: 'replace_with_device_id',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_device_id',
          description: '设备ID'
        },
        {
          key: 'key',
          value: 'temperature',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_key',
          description: '指标键名'
        },
        {
          key: 'start_time',
          value: String(Date.now() - 3600000),
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_start_time',
          description: '开始时间戳（字符串格式）'
        },
        {
          key: 'end_time',
          value: String(Date.now()),
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_end_time',
          description: '结束时间戳（字符串格式）'
        },
        {
          key: 'aggregate_window',
          value: 'no_aggregate',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: '聚合窗口：1h,1d,no_aggregate'
        },
        {
          key: 'time_range',
          value: 'custom',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: '时间范围类型'
        }
      ],
      body: '',
      preRequestScript: `// 动态补齐设备遥测时间范围和参数
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()

// 如果时间参数仍是占位值，则自动更新为最近 1 小时
if (config.params) {
  const startTimeParam = config.params.find(p => p.key === 'start_time')
  const endTimeParam = config.params.find(p => p.key === 'end_time')

  if (startTimeParam && startTimeParam.value === 'replace_with_start_time') {
    startTimeParam.value = (Date.now() - 3600000).toString()
  }
  if (endTimeParam && endTimeParam.value === 'replace_with_end_time') {
    endTimeParam.value = Date.now().toString()
  }
}

// 验证必要参数
const requiredParams = ['device_id', 'key']
const missingParams = []
if (config.params) {
  for (const required of requiredParams) {
    const param = config.params.find(p => p.key === required && p.enabled)
    if (!param || !param.value) {
      missingParams.push(required)
    }
  }
}
if (missingParams.length > 0) {
  logger.error('缺少必要设备遥测参数:', missingParams)
}

return config`,
      postResponseScript: `// 归一化设备遥测趋势响应
if (process.env.NODE_ENV === 'development') {
}

try {
  let data = null

  // 处理响应数据的多种格式
  if (response && typeof response === 'object') {
    // 标准格式: response.data 包含数组
    if (Array.isArray(response.data)) {
      data = response.data
    }
    // 备用格式: response.result
    else if (Array.isArray(response.result)) {
      data = response.result
    }
    // 直接数组格式
    else if (Array.isArray(response)) {
      data = response
    }
    // 列表格式: response.list
    else if (response.list && Array.isArray(response.list)) {
      data = response.list
    }
    // 单条数据格式
    else if (response.data && typeof response.data === 'object') {
      data = [response.data]
    }
  }

  if (process.env.NODE_ENV === 'development') {
  }

  if (data && Array.isArray(data)) {
    const result = data.map(item => {
      if (!item || typeof item !== 'object') return [0, 0]

      // 多种时间字段兼容
      const timeValue = item.x || item.timestamp || item.time || item.ts || Date.now()
      // 多种数值字段兼容
      const dataValue = item.y || item.value || item.val || item.data || 0

      return [timeValue, dataValue]
    }).filter(item => item[0] && item[1] !== undefined)

    if (process.env.NODE_ENV === 'development') {
    }

    if (result.length > 0) {
      return result
    }
  }

  return response

} catch (error) {
  logger.error('[遥测数据] 处理失败:', error)
  return response
}`
    }
  },
  {
    name: '设备当前遥测数据',
    config: {
      url: '/telemetry/datas/current/',
      method: 'GET',
      timeout: 5000,
      pathParameter: {
        value: 'replace_with_device_id',
        isDynamic: true,
        dataType: 'string',
        variableName: 'var_path_param',
        description: '设备ID'
      },
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [],
      body: '',
      preRequestScript: `// 路径参数会自动拼接到URL后
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()
return config`,
      postResponseScript: `// 设备当前遥测数据响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  // 如果是单个设备的遥测数据
  if (response.data && typeof response.data === 'object') {
    return response.data
  }
  // 如果直接是遥测数据
  if (response.telemetry_data) {
    return response.telemetry_data
  }
}

return response`
    }
  },
  {
    name: '设备属性数据',
    config: {
      url: '/attribute/datas/',
      method: 'GET',
      timeout: 5000,
      pathParameter: {
        value: 'replace_with_device_id',
        isDynamic: true,
        dataType: 'string',
        variableName: 'var_path_param',
        description: '设备ID'
      },
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [],
      body: '',
      preRequestScript: `// 路径参数会自动拼接到URL后
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()
return config`,
      postResponseScript: `// 设备属性数据响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  if (response.data) {
    return response.data
  }
}

return response`
    }
  },
  {
    name: '设备命令下发',
    config: {
      url: '/command/datas/pub',
      method: 'POST',
      timeout: 10000,
      headers: [
        {
          key: 'Content-Type',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: '内容类型'
        },
        {
          key: 'Authorization',
          value: 'Bearer ${AETHERLINK_API_TOKEN}',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_authorization',
          description: '认证令牌'
        }
      ],
      params: [],
      pathParams: [],
      body: JSON.stringify(
        {
          device_id: 'replace_with_device_id',
          command_identifier: 'reboot',
          params: {}
        },
        null,
        2
      ),
      preRequestScript: `// 命令下发前处理
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()

// 验证命令数据格式
let commandData
try {
  commandData = JSON.parse(config.body)
  if (!commandData.device_id || !commandData.command_identifier) {
    logger.error('缺少必要的命令参数: device_id, command_identifier')
  }
} catch (e) {
  logger.error('命令数据格式错误:', e)
}

return config`,
      postResponseScript: `// 命令下发响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  if (response.success !== undefined) {
    return {
      success: response.success,
      message: response.message || '命令已发送',
      timestamp: Date.now()
    }
  }
}

return response`
    }
  },
  {
    name: '设备告警历史',
    config: {
      url: '/alarm/info/history',
      method: 'GET',
      timeout: 10000,
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [
        {
          key: 'device_id',
          value: 'replace_with_device_id',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_device_id',
          description: '设备ID'
        },
        {
          key: 'page',
          value: 1,
          enabled: true,
          isDynamic: false,
          dataType: 'number',
          variableName: '',
          description: '页码'
        },
        {
          key: 'page_size',
          value: 20,
          enabled: true,
          isDynamic: false,
          dataType: 'number',
          variableName: '',
          description: '每页数量'
        }
      ],
      pathParams: [],
      body: '',
      preRequestScript: `// 告警历史查询前处理
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()
return config`,
      postResponseScript: `// 告警历史响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  if (response.data && Array.isArray(response.data)) {
    return response.data.map(alarm => ({
      id: alarm.id,
      device_id: alarm.device_id,
      alarm_type: alarm.alarm_type,
      alarm_message: alarm.alarm_message,
      created_at: alarm.created_at,
      status: alarm.status
    }))
  }

  if (Array.isArray(response)) {
    return response
  }
}

return response`
    }
  },
  {
    name: '设备在线状态',
    config: {
      url: '/device',
      method: 'GET',
      timeout: 5000,
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [
        {
          key: 'device_id',
          value: 'replace_with_device_id',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_device_id',
          description: '设备ID'
        },
        {
          key: 'online_status',
          value: '1',
          enabled: false,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: '在线状态筛选'
        }
      ],
      pathParams: [],
      body: '',
      preRequestScript: `// 设备状态查询前处理
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()
return config`,
      postResponseScript: `// 设备状态响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  if (response.data && Array.isArray(response.data)) {
    return response.data.map(device => ({
      device_id: device.id,
      device_name: device.name,
      is_online: device.is_online,
      last_push_time: device.last_push_time,
      status_text: device.is_online ? '在线' : '离线'
    }))
  }

  // 单个设备详情
  if (response.data && response.data.id) {
    const device = response.data
    return {
      device_id: device.id,
      device_name: device.name,
      is_online: device.is_online,
      last_push_time: device.last_push_time,
      status_text: device.is_online ? '在线' : '离线'
    }
  }
}

return response`
    }
  },
  {
    name: '设备列表查询',
    config: {
      url: '/device',
      method: 'GET',
      timeout: 5000,
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [
        {
          key: 'page',
          value: 1,
          enabled: true,
          isDynamic: true,
          dataType: 'number',
          variableName: 'var_page',
          description: '页码'
        },
        {
          key: 'page_size',
          value: 20,
          enabled: true,
          isDynamic: true,
          dataType: 'number',
          variableName: 'var_page_size',
          description: '每页数量'
        },
        {
          key: 'name',
          value: '',
          enabled: false,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_device_name',
          description: '设备名称搜索'
        }
      ],
      pathParams: [],
      body: '',
      preRequestScript: `// 设备列表查询前处理
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()

// 清理空参数
if (config.params) {
  config.params = config.params.filter(param =>
    param.enabled && param.value !== '' && param.value != null
  )
}

return config`,
      postResponseScript: `// 设备列表响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  if (response.data && Array.isArray(response.data)) {
    return {
      devices: response.data,
      total: response.total || response.data.length,
      page: response.page || 1,
      page_size: response.page_size || 20
    }
  }

  if (Array.isArray(response)) {
    return {
      devices: response,
      total: response.length,
      page: 1,
      page_size: response.length
    }
  }
}

return response`
    }
  },
  {
    name: '事件数据查询',
    config: {
      url: '/event/datas',
      method: 'GET',
      timeout: 5000,
      headers: [
        {
          key: 'Accept',
          value: 'application/json',
          enabled: true,
          isDynamic: false,
          dataType: 'string',
          variableName: '',
          description: 'HTTP Accept头'
        }
      ],
      params: [
        {
          key: 'device_id',
          value: 'replace_with_device_id',
          enabled: true,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_device_id',
          description: '设备ID'
        },
        {
          key: 'event_type',
          value: '',
          enabled: false,
          isDynamic: true,
          dataType: 'string',
          variableName: 'var_event_type',
          description: '事件类型'
        },
        {
          key: 'start_time',
          value: Date.now() - 86400000, // 24小时前
          enabled: true,
          isDynamic: true,
          dataType: 'number',
          variableName: 'var_start_time',
          description: '开始时间戳'
        },
        {
          key: 'end_time',
          value: Date.now(),
          enabled: true,
          isDynamic: true,
          dataType: 'number',
          variableName: 'var_end_time',
          description: '结束时间戳'
        }
      ],
      pathParams: [],
      body: '',
      preRequestScript: `// 事件数据查询前处理
config.headers = config.headers || {}
config.headers['X-Request-Time'] = Date.now().toString()
return config`,
      postResponseScript: `// 事件数据响应处理
if (process.env.NODE_ENV === 'development') {
}

if (response && typeof response === 'object') {
  if (response.data && Array.isArray(response.data)) {
    return response.data.map(event => ({
      event_id: event.id,
      device_id: event.device_id,
      event_type: event.event_type,
      event_data: event.event_data,
      timestamp: event.timestamp,
      created_at: event.created_at
    }))
  }

  if (Array.isArray(response)) {
    return response
  }
}

return response`
    }
  }
]

/**
 * 兼容现有直接导入方式的模板快照；修改该数组不会污染内部注册表。
 */
export const HTTP_CONFIG_TEMPLATES = smartDeepClone(httpConfigTemplateRegistry)

/**
 * 获取全新的模板快照，供需要可编辑配置的调用方使用。
 */
export function getHttpConfigTemplates(): Array<{ name: string; config: HttpConfig }> {
  return smartDeepClone(httpConfigTemplateRegistry)
}
