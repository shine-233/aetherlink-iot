/**
 * 文件用途：泛型化数据项与具体数据项配置类型（JSON / HTTP）。
 * 关键注意事项：由 enhanced-types.ts 拆分而来，enhanced/index.ts 统一重导出，外部引用路径保持不变。
 */
import type { ProcessingConfig } from '@/core/data-architecture/executors/DataItemProcessor'

/**
 * 泛型化数据项配置基础接口
 * 支持任意类型的配置结构扩展
 */
export interface DataItemConfig<T = unknown> {
  /** 数据项类型标识 */
  type: string

  /** 数据项唯一标识符 */
  id: string

  /** 类型特定的配置参数 */
  config: T

  /** 数据处理配置（复用现有ProcessingConfig） */
  processing?: ProcessingConfig

  /** 数据项元数据 */
  metadata?: DataItemMetadata
}

/**
 * 数据项元数据接口
 * 用于存储额外的配置信息和状态
 */
export interface DataItemMetadata {
  /** 数据项显示名称 */
  displayName?: string

  /** 数据项描述 */
  description?: string

  /** 创建时间 */
  createdAt?: number

  /** 最后更新时间 */
  lastUpdated?: number

  /** 数据项启用状态 */
  enabled?: boolean

  /** 自定义标签 */
  tags?: string[]
}

// ==================== 具体数据项类型实现 ====================

/**
 * JSON数据项配置（增强版）
 * 保持与现有JsonDataItemConfig的兼容性
 */
export interface EnhancedJsonDataItemConfig {
  /** JSON数据内容 */
  jsonData: string

  /** 数据验证选项 */
  validation?: {
    /** 是否启用JSON格式验证 */
    enableFormat: boolean
    /** 是否启用数据结构验证 */
    enableStructure: boolean
    /** JSON Schema（可选） */
    schema?: unknown
  }

  /** 数据预处理选项 */
  preprocessing?: {
    /** 是否去除注释 */
    removeComments: boolean
    /** 是否格式化输出 */
    formatOutput: boolean
  }
}

/**
 * HTTP数据项配置（增强版）
 * 为动态参数系统预留扩展接口
 */
export interface EnhancedHttpDataItemConfig {
  /** 请求URL（支持模板语法 {{paramName}}） */
  url: string

  /** HTTP请求方法 */
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'

  /** 请求头配置（数组格式，支持动态参数） */
  headers: HttpHeader[]

  /** URL参数配置（新增，支持动态参数） */
  params: HttpParam[]

  /** 请求体配置 */
  body?: HttpBody

  /** 请求超时时间（毫秒） */
  timeout?: number

  /** 请求前脚本（预留） */
  preRequestScript?: string

  /** 响应后脚本（预留） */
  responseScript?: string

  /** 重试配置 */
  retry?: {
    /** 最大重试次数 */
    maxRetries: number
    /** 重试间隔（毫秒） */
    retryDelay: number
  }
}

/**
 * HTTP头部配置
 * 支持静态值和动态参数两种模式
 */
export interface HttpHeader {
  /** 头部字段名 */
  key: string

  /** 头部字段值（静态时为实际值，动态时为种子值） */
  value: string

  /** 是否启用此头部 */
  enabled: boolean

  /** 是否为动态参数 */
  isDynamic: boolean

  /** 数据类型，用于类型转换和验证 */
  dataType: 'string' | 'number' | 'boolean' | 'json'

  /** 自动生成的变量名（格式：http_${key}） */
  variableName: string

  /** 参数说明（动态参数必填，静态参数可选） */
  description?: string
}

/**
 * HTTP参数配置
 * 支持静态值和动态参数两种模式
 */
export interface HttpParam {
  /** 参数名 */
  key: string

  /** 参数值（静态时为实际值，动态时为种子值） */
  value: string

  /** 是否启用此参数 */
  enabled: boolean

  /** 是否为动态参数 */
  isDynamic: boolean

  /** 数据类型，用于类型转换和验证 */
  dataType: 'string' | 'number' | 'boolean' | 'json'

  /** 自动生成的变量名（格式：http_${key}） */
  variableName: string

  /** 参数说明（动态参数必填，静态参数可选） */
  description?: string
}

/**
 * HTTP请求体配置
 */
export interface HttpBody {
  /** 请求体类型 */
  type: 'json' | 'form' | 'text' | 'binary'

  /** 请求体内容 */
  content: unknown

  /** 内容类型 */
  contentType?: string
}
