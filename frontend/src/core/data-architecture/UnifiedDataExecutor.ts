/**
 * 文件用途: 统一数据执行器。
 * 核心逻辑: 按数据源类型选择执行逻辑，完成数据获取、基础转换和错误归一。
 * 关键注意事项: 执行结果字段、fallback 和扩展数据源类型会影响调用方的统一消费方式。
 * 重构建议: 将各数据源执行策略拆为注册式处理器，并补充端到端配置回放测试。
 */

import { request } from '@/service/request'
import type { HttpParam, HttpHeader } from '@/core/data-architecture/types/enhanced-types'

/**
 * 统一数据源配置
 * 支持多种数据源类型的统一配置接口
 */
export interface UnifiedDataConfig {
  /** 数据源唯一标识 */
  id: string
  /** 数据源类型 */
  type: 'static' | 'http' | 'websocket' | 'json' | 'file' | 'data-source-bindings'
  /** 数据源名称 */
  name?: string
  /** 是否启用 */
  enabled?: boolean
  /** 配置选项 */
  config: {
    // === 静态数据配置 ===
    data?: any

    // === HTTP配置 ===
    /** 请求URL (必填) */
    url?: string
    /** HTTP请求方法 (必填) */
    method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
    /** 请求超时时间 */
    timeout?: number
    /** HTTP请求头配置 */
    headers?: HttpHeader[]
    /** HTTP请求参数配置 */
    params?: HttpParam[]

    // === WebSocket配置 ===
    wsUrl?: string
    protocols?: string[]
    reconnect?: boolean
    heartbeat?: boolean

    // === JSON数据配置 ===
    jsonContent?: string
    jsonPath?: string

    // === 文件配置 ===
    filePath?: string
    fileType?: 'json' | 'csv' | 'xml'
    encoding?: string

    // === 数据转换配置 ===
    transform?: {
      /** JSONPath表达式 */
      path?: string
      /** 数据映射规则 */
      mapping?: Record<string, string>
      /** 数据过滤条件 */
      filter?: any
      /** 自定义转换函数 */
      script?: string
    }

    // === 扩展配置 ===
    [key: string]: any
  }
}

/**
 * 统一执行结果
 */
export interface UnifiedDataResult {
  /** 执行是否成功 */
  success: boolean
  /** 数据内容 */
  data?: any
  /** 错误信息 */
  error?: string
  /** 错误代码 */
  errorCode?: string
  /** 执行时间戳 */
  timestamp: number
  /** 数据源ID */
  sourceId: string
  /** 额外元数据 */
  metadata?: {
    /** 响应时间(ms) */
    responseTime?: number
    /** 数据大小 */
    dataSize?: number
    /** 原始响应(调试用) */
    rawResponse?: any
  }
}

/**
 * 数据源执行器接口
 * 支持插件化扩展不同类型的数据源
 */
export interface DataSourceExecutor {
  /** 执行器类型 */
  type: string
  /** 执行数据获取 */
  execute(config: UnifiedDataConfig): Promise<UnifiedDataResult>
  /** 验证配置 */
  validate?(config: UnifiedDataConfig): boolean
  /** 清理资源 */
  cleanup?(): void
}

type DataTransform = UnifiedDataConfig['config']['transform']

/** 受控脚本适配器尚未接入时使用的明确边界错误。 */
class TransformScriptExternalBlockedError extends Error {
  readonly code = 'TRANSFORM_SCRIPT_EXTERNAL_BLOCKED'

  constructor() {
    super('脚本转换需要受控脚本适配器，当前执行管线未启用该能力')
    this.name = 'TransformScriptExternalBlockedError'
  }
}

/**
 * 应用所有数据源共享的纯本地转换。
 * 点号路径、字段映射和数组过滤无需外部依赖；脚本转换保留配置契约但明确阻断。
 */
function applyDataTransform(data: any, transform?: DataTransform): any {
  if (!transform) return data
  if (transform.script?.trim()) throw new TransformScriptExternalBlockedError()

  let result = data

  if (transform.path) {
    result = extractByPath(result, transform.path)
  }

  if (transform.mapping) {
    if (result && typeof result === 'object') {
      result = Object.fromEntries(
        Object.entries(transform.mapping).map(([targetKey, sourceKey]) => [
          targetKey,
          extractByPath(result, sourceKey)
        ])
      )
    }
  }

  if (transform.filter && Array.isArray(result)) {
    result = result.filter(item =>
      Object.entries(transform.filter).every(([key, value]) => item?.[key] === value)
    )
  }

  return result
}

/** 受限点号路径读取；路径不存在时返回 null。 */
function extractByPath(data: any, path: string): any {
  let result = data

  for (const key of path.split('.')) {
    if (result !== null && typeof result === 'object' && key in result) {
      result = result[key]
    } else {
      return null
    }
  }

  return result
}

function isTransformScriptBlocked(error: unknown): error is TransformScriptExternalBlockedError {
  return error instanceof TransformScriptExternalBlockedError
}

/**
 * HTTP数据源执行器
 */
class HttpExecutor implements DataSourceExecutor {
  type = 'http'

  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const startTime = Date.now()

    try {
      const { url, method = 'GET', headers, params, body, timeout = 5000 } = config.config

      if (!url) {
        return this.createErrorResult(config.id, 'HTTP_NO_URL', 'URL未配置', startTime)
      }
      const response = await request({
        url,
        method: method.toLowerCase() as any,
        headers,
        params,
        data: body,
        timeout
      })

      const responseTime = Date.now() - startTime

      // HTTP、静态和 JSON 数据统一使用同一套纯本地转换语义。
      const transformedData = applyDataTransform(response.data, config.config.transform)

      return {
        success: true,
        data: transformedData,
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime,
          dataSize: JSON.stringify(response.data).length,
          rawResponse: response
        }
      }
    } catch (error: any) {
      const responseTime = Date.now() - startTime
      const errorCode = isTransformScriptBlocked(error) ? error.code : 'HTTP_REQUEST_FAILED'
      return this.createErrorResult(config.id, errorCode, error.message || '请求失败', startTime, {
        responseTime
      })
    }
  }

  private createErrorResult(
    sourceId: string,
    errorCode: string,
    error: string,
    startTime: number,
    metadata?: any
  ): UnifiedDataResult {
    return {
      success: false,
      error,
      errorCode,
      timestamp: Date.now(),
      sourceId,
      metadata: {
        responseTime: Date.now() - startTime,
        ...metadata
      }
    }
  }
}

/**
 * 静态数据源执行器
 */
class StaticExecutor implements DataSourceExecutor {
  type = 'static'

  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const startTime = Date.now()

    try {
      const { data } = config.config
      const transformedData = applyDataTransform(data, config.config.transform)

      return {
        success: true,
        data: transformedData,
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime,
          dataSize: JSON.stringify(data).length
        }
      }
    } catch (error: any) {
      return {
        success: false,
        error: error.message || '静态数据处理失败',
        errorCode: isTransformScriptBlocked(error) ? error.code : 'STATIC_DATA_ERROR',
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime
        }
      }
    }
  }
}

/**
 * JSON数据源执行器
 */
class JsonExecutor implements DataSourceExecutor {
  type = 'json'

  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const startTime = Date.now()

    try {
      const { jsonContent } = config.config

      if (!jsonContent) {
        return {
          success: false,
          error: 'JSON内容未配置',
          errorCode: 'JSON_NO_CONTENT',
          timestamp: Date.now(),
          sourceId: config.id,
          metadata: {
            responseTime: Date.now() - startTime
          }
        }
      }

      // 解析后使用与 HTTP、静态数据一致的纯本地转换语义。
      const parsedData = JSON.parse(jsonContent)
      const transformedData = applyDataTransform(parsedData, config.config.transform)

      return {
        success: true,
        data: transformedData,
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime,
          dataSize: jsonContent.length
        }
      }
    } catch (error: any) {
      return {
        success: false,
        error: error.message || 'JSON解析失败',
        errorCode: isTransformScriptBlocked(error) ? error.code : 'JSON_PARSE_ERROR',
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime
        }
      }
    }
  }
}

/**
 * WebSocket 数据源契约适配器。
 *
 * 历史配置仍可被读取，但当前前端执行管线没有订阅生命周期、重连和销毁语义，
 * 因此不能伪造“正在连接”的成功结果。该外部流式能力明确标记为阻断，
 * 待专用订阅适配器实现完整生命周期后再启用。
 */
class FileExecutor implements DataSourceExecutor {
  type = 'file'

  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const startTime = Date.now()
    const { filePath } = config.config

    if (!filePath?.trim()) {
      return {
        success: false,
        error: '文件路径未配置',
        errorCode: 'FILE_NO_PATH',
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime
        }
      }
    }

    return {
      success: false,
      data: { status: 'external-blocked' },
      error: '文件数据源需要外部文件适配器，当前执行管线未启用该能力',
      errorCode: 'FILE_EXTERNAL_BLOCKED',
      timestamp: Date.now(),
      sourceId: config.id,
      metadata: {
        responseTime: Date.now() - startTime
      }
    }
  }
}

class WebSocketExecutor implements DataSourceExecutor {
  type = 'websocket'

  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const startTime = Date.now()
    const { wsUrl } = config.config

    if (!wsUrl?.trim()) {
      return {
        success: false,
        error: 'WebSocket URL未配置',
        errorCode: 'WS_NO_URL',
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime
        }
      }
    }

    return {
      success: false,
      data: { status: 'external-blocked' },
      error: 'WebSocket数据源需要外部订阅适配器，当前执行管线未启用该能力',
      errorCode: 'WS_EXTERNAL_BLOCKED',
      timestamp: Date.now(),
      sourceId: config.id,
      metadata: {
        responseTime: Date.now() - startTime
      }
    }
  }
}

/**
 * 统一数据执行器类
 * 核心功能：管理不同类型的数据源执行器，提供统一接口
 */
export class UnifiedDataExecutor {
  private executors = new Map<string, DataSourceExecutor>()

  constructor() {
    // 注册内置执行器
    this.registerExecutor(new HttpExecutor())
    this.registerExecutor(new StaticExecutor())
    this.registerExecutor(new JsonExecutor())
    this.registerExecutor(new FileExecutor())
    this.registerExecutor(new WebSocketExecutor())
    this.registerExecutor(new DataSourceBindingsExecutor())
  }

  /**
   * 注册数据源执行器 (支持插件扩展)
   */
  registerExecutor(executor: DataSourceExecutor): void {
    this.executors.set(executor.type, executor)
  }

  /**
   * 执行数据源配置
   * 统一的数据获取入口
   */
  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const { type, enabled = true } = config

    // 检查是否启用
    if (!enabled) {
      return {
        success: false,
        error: '数据源未启用',
        errorCode: 'DATA_SOURCE_DISABLED',
        timestamp: Date.now(),
        sourceId: config.id
      }
    }

    // 获取对应执行器
    const executor = this.executors.get(type)
    if (!executor) {
      return {
        success: false,
        error: `不支持的数据源类型: ${type}`,
        errorCode: 'UNSUPPORTED_DATA_SOURCE',
        timestamp: Date.now(),
        sourceId: config.id
      }
    }

    try {
      const result = await executor.execute(config)

      return result
    } catch (error: any) {
      return {
        success: false,
        error: error.message || '执行器异常',
        errorCode: 'EXECUTOR_EXCEPTION',
        timestamp: Date.now(),
        sourceId: config.id
      }
    }
  }

  /**
   * 批量执行多个数据源
   */
  async executeMultiple(configs: UnifiedDataConfig[]): Promise<UnifiedDataResult[]> {
    const results = await Promise.allSettled(configs.map(config => this.execute(config)))

    return results.map((result, index) => {
      if (result.status === 'fulfilled') {
        return result.value
      } else {
        return {
          success: false,
          error: result.reason?.message || '批量执行失败',
          errorCode: 'BATCH_EXECUTION_ERROR',
          timestamp: Date.now(),
          sourceId: configs[index]?.id || 'unknown'
        }
      }
    })
  }

  /**
   * 获取支持的数据源类型
   */
  getSupportedTypes(): string[] {
    return Array.from(this.executors.keys())
  }

  /**
   * 验证数据源配置
   */
  validateConfig(config: UnifiedDataConfig): boolean {
    const executor = this.executors.get(config.type)
    if (!executor) return false

    // 如果执行器提供验证方法，使用它
    if (executor.validate) {
      return executor.validate(config)
    }

    // 基础验证：检查必需字段
    return !!(config.id && config.type)
  }

  /**
   * 清理资源
   */
  cleanup(): void {
    this.executors.forEach(executor => {
      if (executor.cleanup) {
        executor.cleanup()
      }
    })
  }
}

/**
 * 🆕 数据源绑定执行器 - 处理data-source-bindings类型
 * 用于处理复杂的数据源绑定配置
 */
class DataSourceBindingsExecutor implements DataSourceExecutor {
  type = 'data-source-bindings'

  async execute(config: UnifiedDataConfig): Promise<UnifiedDataResult> {
    const startTime = Date.now()

    try {
      // 从config中提取dataSourceBindings配置
      const bindings = config.config?.dataSourceBindings || config.config

      if (!bindings || typeof bindings !== 'object') {
        return {
          success: false,
          error: 'dataSourceBindings配置缺失或格式错误',
          errorCode: 'BINDINGS_CONFIG_ERROR',
          timestamp: Date.now(),
          sourceId: config.id,
          metadata: {
            responseTime: Date.now() - startTime
          }
        }
      }

      // 🔥 关键：处理各种可能的数据格式
      let resultData: any = null

      // 情况1：如果bindings包含rawData字段（来自FinalDataProcessing）
      const bindingKeys = Object.keys(bindings)
      if (bindingKeys.length > 0) {
        const firstBinding = bindings[bindingKeys[0]]

        if (firstBinding?.rawData) {
          // 尝试解析rawData（可能是JSON字符串）
          try {
            resultData =
              typeof firstBinding.rawData === 'string' ? JSON.parse(firstBinding.rawData) : firstBinding.rawData
          } catch (error) {
            // 如果解析失败，直接使用原始数据
            resultData = firstBinding.rawData
          }
        } else if (firstBinding?.finalResult) {
          // 使用finalResult
          resultData = firstBinding.finalResult
        } else {
          // 直接使用整个binding作为数据
          resultData = firstBinding
        }
      } else {
        // 情况2：直接使用config中的数据
        resultData = bindings
      }

      return {
        success: true,
        data: resultData,
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime,
          bindingKeys: bindingKeys,
          dataType: typeof resultData
        }
      }
    } catch (error: any) {
      return {
        success: false,
        error: error.message || '数据源绑定处理失败',
        errorCode: 'BINDINGS_EXECUTION_ERROR',
        timestamp: Date.now(),
        sourceId: config.id,
        metadata: {
          responseTime: Date.now() - startTime
        }
      }
    }
  }
}

// 创建全局统一执行器实例
export const unifiedDataExecutor = new UnifiedDataExecutor()

// 仅在 Vite 开发环境暴露调试入口，生产构建不写入全局作用域。
if (typeof window !== 'undefined' && import.meta.env.DEV) {
  ;(window as any).unifiedDataExecutor = unifiedDataExecutor
}
