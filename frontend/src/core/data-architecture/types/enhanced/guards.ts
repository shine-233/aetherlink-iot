/**
 * 文件用途：增强类型的运行时类型守卫。
 * 关键注意事项：由 enhanced-types.ts 拆分而来，enhanced/index.ts 统一重导出，外部引用路径保持不变。
 */
import type { ComponentMappingConfig } from './component-mapping'
import type { DataItemConfig } from './data-item'
import type { EnhancedDataSourceConfiguration } from './configuration'
import type { EnhancedHttpConfig } from './configuration'
import type { EnhancedHttpDataItemConfig } from './data-item'
import type { HttpHeader } from './data-item'
import type { HttpParam } from './data-item'
import type { PlaceholderConfig } from './placeholder'

// ==================== 泛型化数据项配置 ====================

/**
 * 判定输入是否为可在其上读取属性的对象（不含 null 与数组语义约束）。
 */
export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

export function isEnhancedConfiguration(config: unknown): config is EnhancedDataSourceConfiguration {
  return isRecord(config) && typeof config.version === 'string' && (config.version as string).startsWith('2.')
}

/**
 * 类型守卫：检查是否为泛型数据项配置
 */
export function isGenericDataItemConfig(item: unknown): item is DataItemConfig {
  return isRecord(item) && typeof item.type === 'string' && typeof item.id === 'string' && Boolean(item.config)
}

/**
 * 类型守卫：检查是否为增强版HTTP配置
 */
export function isEnhancedHttpConfig(config: unknown): config is EnhancedHttpDataItemConfig {
  return isRecord(config) && Array.isArray(config.headers) && Array.isArray(config.params)
}

/**
 * 类型守卫：检查是否为EnhancedHttpConfig（用于UnifiedDataConfig）
 */
export function isUnifiedHttpConfig(config: unknown): config is EnhancedHttpConfig {
  return (
    isRecord(config) &&
    typeof config.url === 'string' &&
    typeof config.method === 'string' &&
    (config.headers === undefined || Array.isArray(config.headers)) &&
    (config.params === undefined || Array.isArray(config.params))
  )
}

/**
 * 类型守卫：检查是否为有效的HttpHeader
 */
export function isValidHttpHeader(header: unknown): header is HttpHeader {
  return (
    isRecord(header) &&
    typeof header.key === 'string' &&
    typeof header.value === 'string' &&
    typeof header.enabled === 'boolean' &&
    typeof header.isDynamic === 'boolean' &&
    ['string', 'number', 'boolean', 'json'].includes(header.dataType as string) &&
    typeof header.variableName === 'string'
  )
}

/**
 * 类型守卫：检查是否为有效的HttpParam
 */
export function isValidHttpParam(param: unknown): param is HttpParam {
  return (
    isRecord(param) &&
    typeof param.key === 'string' &&
    typeof param.value === 'string' &&
    typeof param.enabled === 'boolean' &&
    typeof param.isDynamic === 'boolean' &&
    ['string', 'number', 'boolean', 'json'].includes(param.dataType as string) &&
    typeof param.variableName === 'string'
  )
}

/**
 * 类型守卫：检查是否为有效的PlaceholderConfig
 */
export function isValidPlaceholderConfig(config: unknown): config is PlaceholderConfig {
  return (
    isRecord(config) &&
    typeof config.name === 'string' &&
    config.value !== undefined &&
    ['string', 'number', 'boolean', 'json'].includes(config.dataType as string) &&
    typeof config.required === 'boolean'
  )
}

/**
 * 类型守卫：检查是否为有效的ComponentMappingConfig
 */
export function isValidComponentMappingConfig(config: unknown): config is ComponentMappingConfig {
  return (
    isRecord(config) &&
    typeof config.id === 'string' &&
    typeof config.name === 'string' &&
    Boolean(config.sourceComponent) &&
    Boolean(config.targetHttpConfig) &&
    Array.isArray(config.mappings) &&
    ['active', 'inactive', 'error'].includes(config.status as string)
  )
}

// ==================== 向后兼容性保证 ====================

/**
 * 配置类型版本枚举
 */
