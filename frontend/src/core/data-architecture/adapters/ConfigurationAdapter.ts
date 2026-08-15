/**
 * 文件用途: 配置版本适配器实现。
 * 核心逻辑: 检测配置版本并在 v1/v2 与标准数据项格式之间做升级、降级和兼容转换。
 * 关键注意事项: 转换必须尽量无损，版本字段和数据项结构变更需要历史配置样本验证。
 * 重构建议: 把版本检测、升级、降级和差异报告拆成独立适配阶段。
 */

import type {
  // 现有类型
  DataSourceConfiguration as V1DataSourceConfiguration,
  ExecutionResult
} from '../executors/MultiLayerExecutorChain'

import type {
  DataItem as V1DataItem,
  JsonDataItemConfig as V1JsonDataItemConfig,
  HttpDataItemConfig as V1HttpDataItemConfig
} from '../executors/DataItemFetcher'

import type {
  // 增强类型
  EnhancedDataSourceConfiguration,
  DataItemConfig,
  EnhancedJsonDataItemConfig,
  EnhancedHttpDataItemConfig,
  HttpHeader,
  HttpParam,
  ConfigurationAdapter as IConfigurationAdapter
} from '../types/enhanced-types'

import { DEFAULT_ENHANCED_FEATURES } from '@/core/data-architecture/types/enhanced-types'
import { smartDeepClone } from '@/utils/deep-clone'

const FORBIDDEN_HEADER_KEYS = new Set(['__proto__', 'prototype', 'constructor'])

function isSafeHeaderKey(key: unknown): key is string {
  return typeof key === 'string' && key.length > 0 && !FORBIDDEN_HEADER_KEYS.has(key)
}

/**
 * 配置转换结果
 */
export interface ConversionResult<T = any> {
  /** 转换是否成功 */
  success: boolean

  /** 转换后的数据 */
  data?: T

  /** 转换过程中的警告 */
  warnings: string[]

  /** 转换过程中的错误 */
  errors: string[]

  /** 转换元信息 */
  metadata: {
    /** 源版本 */
    sourceVersion: string
    /** 目标版本 */
    targetVersion: string
    /** 转换时间 */
    convertedAt: number
  }
}

/**
 * 配置版本适配器实现类
 */
export class ConfigurationAdapter implements IConfigurationAdapter {
  /**
   * 归一化版本字符串，兼容 `2.0.0`、`v2.0`、`V2.1` 等历史写法。
   */
  private normalizeVersionString(version: string): 'v1.0' | 'v2.0' | null {
    const normalizedVersion = version.trim().toLowerCase()

    if (/^v?2(?:[._-]|$)/.test(normalizedVersion)) {
      return 'v2.0'
    }

    if (/^v?1(?:[._-]|$)/.test(normalizedVersion)) {
      return 'v1.0'
    }

    return null
  }

  /**
   * 检测配置版本
   */
  detectVersion(config: any): 'v1.0' | 'v2.0' {
    // 检查版本字段
    if (config && typeof config.version === 'string') {
      const normalizedVersion = this.normalizeVersionString(config.version)
      if (normalizedVersion) {
        return normalizedVersion
      }
    }

    // 检查增强特性字段
    if (config && (config.dynamicParams || config.enhancedFeatures)) {
      return 'v2.0'
    }

    // 检查数据项格式特征
    if (config && config.dataSources && Array.isArray(config.dataSources)) {
      const firstDataSource = config.dataSources[0]
      if (firstDataSource && firstDataSource.dataItems && Array.isArray(firstDataSource.dataItems)) {
        const firstDataItem = firstDataSource.dataItems[0]
        if (firstDataItem && firstDataItem.id) {
          return 'v2.0' // 有id字段，是v2格式
        }
      }
    }

    // 默认为v1格式
    return 'v1.0'
  }

  /**
   * 适配配置到指定版本
   */
  adaptToVersion(config: any, targetVersion: 'v1.0' | 'v2.0'): ConversionResult {
    const sourceVersion = this.detectVersion(config)

    try {
      if (sourceVersion === targetVersion) {
        return {
          success: true,
          data: smartDeepClone(config),
          warnings: [],
          errors: [],
          metadata: {
            sourceVersion,
            targetVersion,
            convertedAt: Date.now()
          }
        }
      }

      const convertedData = targetVersion === 'v2.0' ? this.upgradeV1ToV2(config) : this.downgradeV2ToV1(config)

      return {
        success: true,
        data: convertedData,
        warnings: [],
        errors: [],
        metadata: {
          sourceVersion,
          targetVersion,
          convertedAt: Date.now()
        }
      }
    } catch (error) {
      return {
        success: false,
        warnings: [],
        errors: [error instanceof Error ? error.message : String(error)],
        metadata: {
          sourceVersion,
          targetVersion,
          convertedAt: Date.now()
        }
      }
    }
  }

  /**
   * v1升级到v2（无损升级）
   */
  upgradeV1ToV2(v1Config: V1DataSourceConfiguration): EnhancedDataSourceConfiguration {
    const enhancedConfig: EnhancedDataSourceConfiguration = {
      // 保留所有原有字段
      ...v1Config,

      // 添加版本标识
      version: '2.0.0',

      // 默认动态参数配置
      dynamicParams: [],

      // 默认增强功能开关
      enhancedFeatures: {
        ...DEFAULT_ENHANCED_FEATURES
      },

      // 添加配置元数据
      metadata: {
        name: `配置_${v1Config.componentId}`,
        description: `从v1.0升级的配置`,
        author: 'system',
        versionHistory: [
          {
            version: '2.0.0',
            timestamp: Date.now(),
            changelog: '从v1.0自动升级到v2.0',
            author: 'ConfigurationAdapter'
          }
        ],
        tags: ['upgraded', 'v2']
      },

      // 升级数据源配置
      dataSources: v1Config.dataSources.map((dataSource) => ({
        ...dataSource,
        dataItems: dataSource.dataItems.map((dataItemWrapper, index) => ({
          ...dataItemWrapper,
          item: this.upgradeDataItemToV2(dataItemWrapper.item, `${dataSource.sourceId}_item_${index}`)
        }))
      }))
    }

    return enhancedConfig
  }

  /**
   * v2降级到v1（兼容降级）
   */
  downgradeV2ToV1(v2Config: EnhancedDataSourceConfiguration): V1DataSourceConfiguration {
    const v1Config: V1DataSourceConfiguration = {
      componentId: v2Config.componentId,
      dataSources: v2Config.dataSources.map((dataSource) => ({
        sourceId: dataSource.sourceId,
        dataItems: dataSource.dataItems.map((dataItemWrapper) => ({
          item: this.downgradeDataItemToV1(dataItemWrapper.item),
          processing: dataItemWrapper.processing
        })),
        mergeStrategy: dataSource.mergeStrategy
      })),
      createdAt: v2Config.createdAt,
      updatedAt: Date.now() // 更新时间戳
    }

    return v1Config
  }

  /**
   * 数据项升级到v2格式
   */
  private upgradeDataItemToV2(v1Item: V1DataItem, itemId: string): DataItemConfig {
    const baseItem: DataItemConfig = {
      type: v1Item.type,
      id: itemId,
      config: v1Item.config,
      metadata: {
        displayName: `${v1Item.type}数据项`,
        description: `从v1.0升级的${v1Item.type}数据项`,
        createdAt: Date.now(),
        lastUpdated: Date.now(),
        enabled: true,
        tags: ['upgraded']
      }
    }

    // 特殊处理不同类型的配置
    switch (v1Item.type) {
      case 'json': {
        const v1JsonConfig = v1Item.config as V1JsonDataItemConfig
        const enhancedJsonConfig: EnhancedJsonDataItemConfig = {
          jsonData: v1JsonConfig.jsonString, // 字段重命名
          validation: {
            enableFormat: true,
            enableStructure: false
          },
          preprocessing: {
            removeComments: false,
            formatOutput: false
          }
        }
        return { ...baseItem, config: enhancedJsonConfig }
      }

      case 'http': {
        const v1HttpConfig = v1Item.config as V1HttpDataItemConfig
        const hasUnsupportedParameterConfiguration =
          v1HttpConfig.pathParameter !== undefined ||
          (v1HttpConfig.pathParams?.length ?? 0) > 0 ||
          (v1HttpConfig.params?.length ?? 0) > 0 ||
          (v1HttpConfig.parameters?.length ?? 0) > 0 ||
          v1HttpConfig.addressType !== undefined ||
          v1HttpConfig.selectedInternalAddress !== undefined ||
          v1HttpConfig.enableParams !== undefined

        if (hasUnsupportedParameterConfiguration) {
          throw new Error('UNSUPPORTED_HTTP_PARAMETER_MIGRATION')
        }

        const enhancedHttpConfig: EnhancedHttpDataItemConfig = {
          url: v1HttpConfig.url,
          method: v1HttpConfig.method,
          headers: this.convertHeadersRecordToArray(v1HttpConfig.headers || {}),
          params: [], // 已确认源配置没有增强类型无法表达的参数字段。
          body:
            v1HttpConfig.body !== null && v1HttpConfig.body !== undefined
              ? {
                  type: 'json',
                  content: v1HttpConfig.body
                }
              : undefined,
          timeout: v1HttpConfig.timeout,
          preRequestScript: v1HttpConfig.preRequestScript,
          responseScript: v1HttpConfig.postResponseScript,
          retry: {
            maxRetries: 3,
            retryDelay: 1000
          }
        }
        return { ...baseItem, config: enhancedHttpConfig }
      }

      default:
        // 其他类型保持原样
        return baseItem
    }
  }

  /**
   * 数据项降级到v1格式
   */
  private downgradeDataItemToV1(v2Item: DataItemConfig): V1DataItem {
    switch (v2Item.type) {
      case 'json': {
        const enhancedJsonConfig = v2Item.config as EnhancedJsonDataItemConfig
        return {
          type: 'json',
          config: {
            jsonString: enhancedJsonConfig.jsonData // 字段重命名回去
          }
        }
      }

      case 'http': {
        const enhancedHttpConfig = v2Item.config as EnhancedHttpDataItemConfig
        return {
          type: 'http',
          config: {
            url: enhancedHttpConfig.url,
            method: enhancedHttpConfig.method,
            headers: this.convertHeadersArrayToRecord(enhancedHttpConfig.headers),
            body: enhancedHttpConfig.body?.content,
            timeout: enhancedHttpConfig.timeout,
            preRequestScript: enhancedHttpConfig.preRequestScript,
            postResponseScript: enhancedHttpConfig.responseScript
          }
        }
      }

      case 'websocket':
        return {
          type: 'websocket',
          config: v2Item.config // WebSocket配置保持不变
        }

      case 'script':
        return {
          type: 'script',
          config: v2Item.config // Script配置保持不变
        }

      default:
        // v1 联合类型没有未知插件类型的安全表示，必须显式报告阻断。
        throw new Error(`UNSUPPORTED_DATA_ITEM_TYPE:${v2Item.type}`)
    }
  }

  /**
   * 将Record格式的headers转换为Array格式
   */
  private convertHeadersRecordToArray(headers: Record<string, string>): HttpHeader[] {
    return Object.entries(headers)
      .filter(([key]) => isSafeHeaderKey(key))
      .map(([key, value]) => ({
        key,
        value,
        enabled: true,
        isDynamic: false
      }))
  }

  /**
   * 将Array格式的headers转换为Record格式
   */
  private convertHeadersArrayToRecord(headers: HttpHeader[]): Record<string, string> {
    return headers
      .filter((header) => header.enabled && isSafeHeaderKey(header.key))
      .reduce(
        (acc, header) => {
          acc[header.key] = header.value
          return acc
        },
        {} as Record<string, string>
      )
  }

  /**
   * 批量转换配置
   */
  public batchConvert(configs: any[], targetVersion: 'v1.0' | 'v2.0'): ConversionResult[] {
    return configs.map((config) => this.adaptToVersion(config, targetVersion))
  }

  /**
   * 验证配置转换的一致性
   */
  public validateConversion(original: any, converted: any): { valid: boolean; issues: string[] } {
    const issues: string[] = []

    // 检查基本字段
    if (original.componentId !== converted.componentId) {
      issues.push('componentId不匹配')
    }

    if (original.dataSources.length !== converted.dataSources.length) {
      issues.push('dataSources数量不匹配')
    }

    // 检查数据源
    for (let i = 0; i < original.dataSources.length; i++) {
      const origDs = original.dataSources[i]
      const convDs = converted.dataSources[i]

      if (origDs.sourceId !== convDs.sourceId) {
        issues.push(`数据源${i}的sourceId不匹配`)
      }

      if (origDs.dataItems.length !== convDs.dataItems.length) {
        issues.push(`数据源${i}的dataItems数量不匹配`)
      }
    }

    return {
      valid: issues.length === 0,
      issues
    }
  }
}

// ==================== 工厂函数 ====================

/**
 * 创建配置适配器实例
 */
export function createConfigurationAdapter(): ConfigurationAdapter {
  return new ConfigurationAdapter()
}

// ==================== 便捷函数 ====================

/**
 * 快速检测配置版本
 */
export function detectConfigVersion(config: any): 'v1.0' | 'v2.0' {
  return createConfigurationAdapter().detectVersion(config)
}

/**
 * 快速升级配置到v2
 */
export function upgradeToV2(v1Config: V1DataSourceConfiguration): EnhancedDataSourceConfiguration {
  return createConfigurationAdapter().upgradeV1ToV2(v1Config)
}

/**
 * 快速降级配置到v1
 */
export function downgradeToV1(v2Config: EnhancedDataSourceConfiguration): V1DataSourceConfiguration {
  return createConfigurationAdapter().downgradeV2ToV1(v2Config)
}

// ==================== 导出 ====================
export type { ConversionResult }
