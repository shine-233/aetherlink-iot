/**
 * 文件用途：增强版配置、版本与适配器元信息类型。
 * 关键注意事项：由 enhanced-types.ts 拆分而来，enhanced/index.ts 统一重导出，外部引用路径保持不变。
 */
import type { ComponentMappingConfig } from './component-mapping'
import type { DynamicParam } from './placeholder'
import type { HttpBody } from './data-item'
import type { HttpHeader } from './data-item'
import type { HttpParam } from './data-item'
import type { PlaceholderConfig } from './placeholder'
import type { PlaceholderDependencyAnalysis } from './placeholder'
import type { DataSourceConfiguration as BaseDataSourceConfiguration } from '../../executors/MultiLayerExecutorChain'

// ==================== 增强版配置系统 ====================

/**
 * EnhancedHttpConfig接口
 * 用于UnifiedDataConfig的HTTP配置部分，基于SUBTASK-008设计
 */
export interface EnhancedHttpConfig {
  /** 请求URL（支持{{占位符}}） */
  url: string

  /** HTTP请求方法 */
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH'

  /** 请求超时时间（毫秒，默认5000） */
  timeout?: number

  /** 请求头配置数组 */
  headers?: HttpHeader[]

  /** URL参数配置数组 */
  params?: HttpParam[]

  /** 请求体配置 */
  body?: HttpBody

  /** 请求前脚本（调整配置） */
  preRequestScript?: string

  /** 响应后脚本（修改响应结果） */
  postResponseScript?: string

  /** 重试配置 */
  retry?: {
    /** 最大重试次数 */
    maxRetries: number
    /** 重试间隔（毫秒） */
    retryDelay: number
  }

  /** 组件映射配置 */
  componentMappings?: ComponentMappingConfig[]
}

/**
 * ConfigurationManager扩展接口
 * 支持占位符解析和模板管理
 */
export interface EnhancedConfigurationManager {
  /**
   * 分析配置中的占位符依赖
   * @param config 配置对象
   * @returns 依赖分析结果
   */
  analyzePlaceholderDependencies(config: unknown): PlaceholderDependencyAnalysis

  /**
   * 提取配置中的所有占位符
   * @param config 配置对象
   * @returns 占位符名称列表
   */
  extractPlaceholders(config: unknown): string[]

  /**
   * 替换配置中的占位符值
   * @param config 配置对象
   * @param placeholderValues 占位符值映射
   * @returns 替换后的配置
   */
  replacePlaceholders(config: unknown, placeholderValues: Map<string, unknown>): unknown

  /**
   * 验证占位符配置
   * @param config 配置对象
   * @param placeholderConfigs 占位符配置映射
   * @returns 验证结果
   */
  validatePlaceholders(config: unknown, placeholderConfigs: Map<string, PlaceholderConfig>): PlaceholderValidationResult

  /**
   * 检测循环依赖
   * @param dependencies 依赖关系映射
   * @returns 循环依赖检测结果
   */
  detectCircularDependencies(dependencies: Map<string, string[]>): CircularDependencyResult
}

/**
 * 占位符验证结果
 */
export interface PlaceholderValidationResult {
  /** 验证是否通过 */
  isValid: boolean

  /** 验证错误列表 */
  errors: PlaceholderValidationError[]

  /** 验证警告列表 */
  warnings: PlaceholderValidationWarning[]

  /** 缺失的必填占位符 */
  missingRequired: string[]

  /** 未定义的占位符 */
  undefined: string[]
}

/**
 * 占位符验证错误
 */
export interface PlaceholderValidationError {
  /** 占位符名称 */
  placeholder: string

  /** 错误类型 */
  type: 'missing' | 'invalid_type' | 'validation_failed' | 'circular_dependency'

  /** 错误信息 */
  message: string

  /** 错误位置路径 */
  path?: string
}

/**
 * 占位符验证警告
 */
export interface PlaceholderValidationWarning {
  /** 占位符名称 */
  placeholder: string

  /** 警告类型 */
  type: 'unused' | 'compatibility' | 'performance'

  /** 警告信息 */
  message: string
}

/**
 * 循环依赖检测结果
 */
export interface CircularDependencyResult {
  /** 是否存在循环依赖 */
  hasCircularDependency: boolean

  /** 循环依赖路径列表 */
  circularPaths: string[][]

  /** 涉及循环依赖的占位符 */
  affectedPlaceholders: string[]
}

/**
 * 增强版数据源配置
 * 继承现有配置，增加版本管理和动态参数支持
 */
export interface EnhancedDataSourceConfiguration extends BaseDataSourceConfiguration {
  /** 配置版本标识 */
  version: string

  /** 动态参数定义（预留） */
  dynamicParams?: DynamicParam[]

  /** 增强功能开关 */
  enhancedFeatures?: EnhancedFeatureFlags

  /** 配置元数据 */
  metadata?: ConfigurationMetadata

  /** 占位符配置映射 */
  placeholderConfigs?: Map<string, PlaceholderConfig>

  /** 组件映射配置列表 */
  componentMappings?: ComponentMappingConfig[]
}

/**
 * 增强功能开关
 */
export interface EnhancedFeatureFlags {
  /** 启用HTTP数组格式 */
  httpArrayFormat: boolean

  /** 启用动态参数支持 */
  dynamicParameterSupport: boolean

  /** 启用安全脚本执行 */
  secureScriptExecution: boolean

  /** 启用配置验证 */
  configurationValidation: boolean

  /** 启用性能监控 */
  performanceMonitoring: boolean
}

/**
 * 配置元数据
 */
export interface ConfigurationMetadata {
  /** 配置名称 */
  name?: string

  /** 配置描述 */
  description?: string

  /** 配置创建者 */
  author?: string

  /** 配置版本历史 */
  versionHistory?: ConfigurationVersion[]

  /** 配置标签 */
  tags?: string[]
}

/**
 * 配置版本信息
 */
export interface ConfigurationVersion {
  /** 版本号 */
  version: string

  /** 变更时间 */
  timestamp: number

  /** 变更说明 */
  changelog: string

  /** 变更作者 */
  author?: string
}

// ==================== 配置适配器系统 ====================

/**
 * 配置版本适配器
 * 处理新旧配置格式的自动转换
 */
export interface ConfigurationAdapterInfo {
  /**
   * 检测配置版本
   * @param config 配置对象
   * @returns 版本标识
   */
  detectVersion(config: unknown): 'v1.0' | 'v2.0'

  /**
   * 适配配置到指定版本
   * @param config 源配置
   * @param targetVersion 目标版本
   * @returns 适配后的配置
   */
  adaptToVersion(config: unknown, targetVersion: 'v1.0' | 'v2.0'): unknown

  /**
   * v1升级到v2。
   * 已支持字段保持无损；增强类型无法表达的 HTTP 参数配置必须明确阻断。
   * @param v1Config v1格式配置
   * @returns v2格式配置
   */
  upgradeV1ToV2(v1Config: BaseDataSourceConfiguration): EnhancedDataSourceConfiguration

  /**
   * v2降级到v1（功能裁剪）
   * @param v2Config v2格式配置
   * @returns v1格式配置
   */
  downgradeV2ToV1(v2Config: EnhancedDataSourceConfiguration): BaseDataSourceConfiguration
}

// ==================== 类型工具和辅助函数 ====================

/**
 * 数据类型转换器
 * 用于HttpHeader和HttpParam的值转换
 */
export enum ConfigurationVersionEnum {
  V1_0 = 'v1.0',
  V2_0 = 'v2.0'
}

/**
 * 默认的增强功能开关配置
 */
export const DEFAULT_ENHANCED_FEATURES: EnhancedFeatureFlags = {
  httpArrayFormat: true,
  dynamicParameterSupport: true,
  secureScriptExecution: true,
  configurationValidation: true,
  performanceMonitoring: true
}
