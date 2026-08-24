/**
 * 文件用途: 简化数据桥接器。
 * 核心逻辑: 协调组件数据需求、配置转换和多层执行器链，产出组件可消费的数据。
 * 关键注意事项: 这是旧执行器路径的替代入口，配置结构和执行结果字段必须保持兼容。
 * 重构建议: 继续拆分配置转换、执行协调和错误归一，降低桥接器职责集中度。
 */

import {
  type IMultiLayerExecutorChain,
  MultiLayerExecutorChain,
  type DataSourceConfiguration,
  type ExecutionResult
} from './executors/MultiLayerExecutorChain'

import type { DataItem } from './executors/DataItemFetcher'

import { dataWarehouse, type EnhancedDataWarehouse } from '@/core/data-architecture/DataWarehouse'

type ConfigurationSnapshot = { config: any; timestamp: number }
type ConfigurationSnapshotReader = (componentId: string) => any | Promise<any>
type ConfigurationSnapshotStatus =
  | { status: 'not-configured' | 'unavailable' }
  | { status: 'available'; snapshot: ConfigurationSnapshot }
  | { status: 'degraded'; errorCode: 'CONFIGURATION_SNAPSHOT_UNAVAILABLE' }
type SimpleDataBridgeExecutor = Pick<IMultiLayerExecutorChain, 'executeDataProcessingChain'>

export interface SimpleDataBridgeOptions {
  warehouse?: EnhancedDataWarehouse
  /** 仅在仓库由该桥接器独占时启用；默认不销毁共享或外部注入的仓库。 */
  destroyWarehouseOnDispose?: boolean
  executorChain?: SimpleDataBridgeExecutor
  /** 可选宿主快照适配器；运行时核心不反向依赖 visual-editor。 */
  snapshotReader?: ConfigurationSnapshotReader
  now?: () => number
  random?: () => number
  logError?: (...args: unknown[]) => void
}

/**
 * 简化的数据源配置
 */
export interface SimpleDataSourceConfig {
  /** 数据源ID */
  id: string
  /** 数据源类型 */
  type: 'static' | 'http' | 'json' | 'websocket' | 'file' | 'data-source-bindings'
  /** 配置选项 */
  config: {
    // 静态数据
    data?: any
    // HTTP配置
    url?: string
    method?: 'GET' | 'POST'
    headers?: Record<string, string>
    timeout?: number
    [key: string]: any
  }
  /** 过滤路径（JSONPath 语法） */
  filterPath?: string
  /** 自定义处理脚本 */
  processScript?: string
}

/**
 * 数据执行结果
 */
export interface DataResult {
  /** 是否成功 */
  success: boolean
  /** 数据内容 */
  data?: any
  /** 错误信息 */
  error?: string
  /** 稳定错误代码 */
  errorCode?: string
  /** 执行时间戳 */
  timestamp: number
  /** 可选执行元数据；兼容旧调用方，并显式暴露快照适配器降级。 */
  metadata?: {
    configurationSnapshot?: {
      status: 'degraded'
      errorCode: 'CONFIGURATION_SNAPSHOT_UNAVAILABLE'
    }
  }
}

/**
 * 组件数据需求
 */
export interface ComponentDataRequirement {
  /** 组件ID */
  componentId: string
  /** 数据源配置列表 */
  dataSources: SimpleDataSourceConfig[]
}

/**
 * 数据更新回调类型
 */
export type DataUpdateCallback = (componentId: string, data: Record<string, any>) => void

/**
 * 简化数据桥接器类
 * 只提供最基本的配置→数据转换功能
 */
export class SimpleDataBridge {
  /** 数据更新回调列表 */
  private callbacks = new Set<DataUpdateCallback>()

  /** 数据仓库实例 */
  private warehouse: EnhancedDataWarehouse

  /** 桥接器销毁时是否同时销毁其独占仓库。 */
  private destroyWarehouseOnDispose: boolean

  /** 多层执行器链实例（符合需求文档架构） */
  private executorChain: SimpleDataBridgeExecutor

  private snapshotReader?: ConfigurationSnapshotReader

  private now: () => number

  private random: () => number

  private logError: (...args: unknown[]) => void

  private componentConfigHashes = new Map<string, string>()

  constructor(options: SimpleDataBridgeOptions = {}) {
    this.warehouse = options.warehouse ?? dataWarehouse
    this.destroyWarehouseOnDispose = options.destroyWarehouseOnDispose ?? false
    this.executorChain = options.executorChain ?? new MultiLayerExecutorChain()
    this.snapshotReader = options.snapshotReader
    this.now = options.now ?? Date.now
    this.random = options.random ?? Math.random
    this.logError = options.logError ?? console.error
  }

  /**
   * 执行组件数据获取
   * 使用 MultiLayerExecutorChain 执行数据处理链。
   * @param requirement 组件数据需求
   * @returns 执行结果
   */
  async executeComponent(requirement: ComponentDataRequirement): Promise<DataResult> {
    return await this.doExecuteComponent(requirement, this.now(), 'direct-call')
  }

  /**
   * 实际的组件执行逻辑（从executeComponent中提取）
   */
  private async doExecuteComponent(
    requirement: ComponentDataRequirement,
    _startTime: number,
    _callerInfo: string
  ): Promise<DataResult> {
    const executionId = `${requirement.componentId}-${this.now()}-${this.random().toString(36).substr(2, 9)}`

    let metadata: DataResult['metadata']

    try {
      const snapshotStatus = await this.captureConfigurationSnapshot(requirement.componentId, executionId)
      if (snapshotStatus.status === 'available') {
        requirement = this.reconstructRequirementFromSnapshot(requirement, snapshotStatus.snapshot)
      } else if (snapshotStatus.status === 'degraded') {
        metadata = {
          configurationSnapshot: {
            status: 'degraded',
            errorCode: snapshotStatus.errorCode
          }
        }
      }

      let dataSourceConfig: DataSourceConfiguration

      const isDataSourceConfigFormat = this.isDataSourceConfiguration(requirement)

      if (isDataSourceConfigFormat) {
        dataSourceConfig = requirement as any
      } else {
        const nestedSource = requirement.dataSources?.[0] as
          | { dataSources?: DataSourceConfiguration['dataSources']; createdAt?: number; updatedAt?: number }
          | undefined
        if (nestedSource?.dataSources) {
          dataSourceConfig = {
            componentId: requirement.componentId,
            dataSources: nestedSource.dataSources,
            createdAt: nestedSource.createdAt || this.now(),
            updatedAt: nestedSource.updatedAt || this.now()
          }
        } else {
          dataSourceConfig = this.convertToDataSourceConfiguration(requirement)
        }
      }

      const configHash = this.calculateConfigHash(dataSourceConfig)
      const cachedData = this.warehouse.getComponentData(requirement.componentId)
      const existingConfigHash = this.componentConfigHashes.get(requirement.componentId)

      if (cachedData && existingConfigHash === configHash) {
        return {
          success: true,
          data: cachedData,
          timestamp: this.now(),
          ...(metadata && { metadata })
        }
      }

      if (existingConfigHash && existingConfigHash !== configHash) {
        this.warehouse.clearComponentCache(requirement.componentId)
      }

      const enhancedDataSourceConfig = {
        ...dataSourceConfig,
        configHash
      }

      const executionResult: ExecutionResult = await this.executorChain.executeDataProcessingChain(
        enhancedDataSourceConfig,
        true
      )

      if (executionResult.success && executionResult.componentData) {
        const componentData = this.normalizeComponentData(executionResult.componentData)

        if (executionResult.componentData && typeof executionResult.componentData === 'object') {
          this.warehouse.clearComponentCache(requirement.componentId)

          Object.entries(componentData).forEach(([sourceId, sourceData]) => {
            this.warehouse.storeComponentData(requirement.componentId, sourceId, sourceData, 'multi-source')
          })

          this.warehouse.storeComponentData(requirement.componentId, 'complete', componentData, 'multi-source')
          this.componentConfigHashes.set(requirement.componentId, configHash)
        }

        this.notifyDataUpdate(requirement.componentId, componentData)
        return {
          success: true,
          data: componentData,
          timestamp: this.now(),
          ...(metadata && { metadata })
        }
      } else {
        return {
          success: false,
          error: executionResult.error || '执行失败',
          errorCode: executionResult.errorCode,
          timestamp: this.now(),
          ...(metadata && { metadata })
        }
      }
    } catch (error) {
      const errorMsg = error instanceof Error ? error.message : String(error)
      return {
        success: false,
        error: errorMsg,
        timestamp: this.now(),
        ...(metadata && { metadata })
      }
    }
  }

  /**
   * 检查是否为 DataSourceConfiguration 格式
   * @param data 待检查的数据
   * @returns 是否为 DataSourceConfiguration 格式
   */
  private isDataSourceConfiguration(data: any): boolean {
    return (
      data &&
      typeof data === 'object' &&
      'componentId' in data &&
      'dataSources' in data &&
      Array.isArray(data.dataSources) &&
      data.dataSources.length > 0 &&
      'sourceId' in data.dataSources[0] &&
      'dataItems' in data.dataSources[0] &&
      'mergeStrategy' in data.dataSources[0]
    )
  }

  /**
   * 转换为 DataSourceConfiguration 格式
   * 将 SimpleDataBridge 的配置格式转换为 MultiLayerExecutorChain 所需的格式
   * @param requirement 组件数据需求
   * @returns DataSourceConfiguration 格式的配置
   */
  private convertToDataSourceConfiguration(requirement: ComponentDataRequirement): DataSourceConfiguration {
    const dataSources = requirement.dataSources.map(dataSource => ({
      sourceId: dataSource.id,
      dataItems: [
        {
          // SimpleDataSourceConfig 的 type/config 是宽松的运行时契约，
          // 执行链对未知类型有兜底分支，此处按执行器入口契约收窄。
          item: {
            type: dataSource.type,
            config: dataSource.config
          } as unknown as DataItem,
          processing: {
            filterPath: dataSource.filterPath || '$',
            customScript: dataSource.processScript,
            defaultValue: {}
          }
        }
      ],
      mergeStrategy: { type: 'object' } as const // 默认使用对象合并策略
    }))

    return {
      componentId: requirement.componentId,
      dataSources,
      createdAt: this.now(),
      updatedAt: this.now()
    }
  }

  private normalizeComponentData(data: Record<string, any>): Record<string, any> {
    const normalized: Record<string, any> = {}

    for (const [sourceId, sourceData] of Object.entries(data)) {
      if (
        sourceData &&
        typeof sourceData === 'object' &&
        'data' in sourceData &&
        ('type' in sourceData || 'lastUpdated' in sourceData || 'metadata' in sourceData)
      ) {
        normalized[sourceId] = sourceData.data ?? null
      } else {
        normalized[sourceId] = sourceData
      }
    }

    return normalized
  }

  /**
   * 通知数据更新
   * @param componentId 组件ID
   * @param data 数据
   */
  private notifyDataUpdate(componentId: string, data: Record<string, any>): void {
    this.callbacks.forEach(callback => {
      try {
        callback(componentId, data)
      } catch (_error) {
        // Keep one failing subscriber from blocking the remaining callbacks.
      }
    })
  }

  /**
   * 注册数据更新回调
   * @param callback 回调函数
   * @returns 取消注册的函数
   */
  onDataUpdate(callback: DataUpdateCallback): () => void {
    this.callbacks.add(callback)

    return () => {
      this.callbacks.delete(callback)
    }
  }

  /**
   * 获取组件数据（缓存接口）
   * @param componentId 组件ID
   * @returns 组件数据或null
   */
  getComponentData(componentId: string): Record<string, any> | null {
    return this.warehouse.getComponentData(componentId)
  }

  /**
   * 清除组件缓存
   * @param componentId 组件ID
   */
  clearComponentCache(componentId: string): void {
    this.warehouse.clearComponentCache(componentId)
    this.componentConfigHashes.delete(componentId)
  }

  /**
   * 清除所有缓存
   */
  clearAllCache(): void {
    this.warehouse.clearAllCache()
    this.componentConfigHashes.clear()
  }

  /**
   * 设置缓存过期时间
   * @param milliseconds 过期时间（毫秒）
   */
  setCacheExpiry(milliseconds: number): void {
    this.warehouse.setCacheExpiry(milliseconds)
  }

  /**
   * 获取数据仓库性能指标
   */
  getWarehouseMetrics() {
    return this.warehouse.getPerformanceMetrics()
  }

  /**
   * 获取存储统计信息
   */
  getStorageStats() {
    return this.warehouse.getStorageStats()
  }

  /**
   * 获取简单统计信息，包含数据仓库数据
   */
  getStats() {
    const warehouseStats = this.warehouse.getStorageStats()
    return {
      activeCallbacks: this.callbacks.size,
      timestamp: this.now(),
      warehouse: {
        totalComponents: warehouseStats.totalComponents,
        totalDataSources: warehouseStats.totalDataSources,
        memoryUsageMB: warehouseStats.memoryUsageMB
      }
    }
  }

  /**
   * 捕获可选宿主配置快照；适配器失败时返回明确降级状态。
   */
  private async captureConfigurationSnapshot(
    componentId: string,
    executionId: string
  ): Promise<ConfigurationSnapshotStatus> {
    if (!this.snapshotReader) {
      return { status: 'not-configured' }
    }

    try {
      const config = await this.snapshotReader(componentId)
      if (!config) {
        return { status: 'unavailable' }
      }

      return {
        status: 'available',
        snapshot: {
          config: JSON.parse(JSON.stringify(config)),
          timestamp: this.now()
        }
      }
    } catch (error) {
      this.logError(`❌ [SimpleDataBridge] [${executionId}] 配置快照捕获失败:`, error)
      return {
        status: 'degraded',
        errorCode: 'CONFIGURATION_SNAPSHOT_UNAVAILABLE'
      }
    }
  }

  /**
   * 基于配置快照重构数据需求。
   */
  private reconstructRequirementFromSnapshot(
    originalRequirement: ComponentDataRequirement,
    snapshot: ConfigurationSnapshot
  ): ComponentDataRequirement {
    if (snapshot.config.dataSource) {
      return {
        ...originalRequirement,
        dataSources: this.convertSnapshotToDataSources(snapshot.config)
      }
    }
    return originalRequirement
  }

  /**
   * 将配置快照转换为数据源格式。
   */
  private convertSnapshotToDataSources(config: any): any[] {
    if (config.dataSource && config.dataSource.dataSources) {
      return config.dataSource.dataSources
    }
    return []
  }

  /**
   * 计算配置哈希值，用于检测配置变化。
   */
  private calculateConfigHash(config: any): string {
    try {
      const configString = JSON.stringify(config, (key, value) =>
        ['createdAt', 'updatedAt', 'configHash'].includes(key) ? undefined : value
      )
      let hash = 0
      for (let i = 0; i < configString.length; i++) {
        const char = configString.charCodeAt(i)
        hash = (hash << 5) - hash + char
        hash = hash & hash
      }
      return Math.abs(hash).toString(36)
    } catch (_error) {
      return this.now().toString(36)
    }
  }

  /**
   * 清理桥接器自身资源；仅在调用方明确声明独占仓库时联动销毁仓库。
   */
  destroy(): void {
    this.callbacks.clear()
    this.componentConfigHashes.clear()
    if (this.destroyWarehouseOnDispose) {
      this.warehouse.destroy()
    }
  }
}

/**
 * 导出全局单例实例
 */
export const simpleDataBridge = new SimpleDataBridge()

/**
 * 创建新的数据桥接器实例
 */
export function createSimpleDataBridge(options?: SimpleDataBridgeOptions): SimpleDataBridge {
  return new SimpleDataBridge(options)
}

/**
 * 开发环境自动验证
 * 在控制台输出 Phase 2 架构状态信息
 */
