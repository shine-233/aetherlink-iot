/**
 * 文件用途: data-architecture 运行时数据仓库。
 * 核心逻辑: 按组件和数据源隔离缓存运行时数据，维护过期时间、访问统计、版本和内存使用信息。
 * 关键注意事项: 动态参数由 DynamicParameterStore 独立管理；缓存键、过期策略和清理时机会影响数据刷新和内存占用。
 */
import { ref, type Ref } from 'vue'
import { ComponentCacheStore } from './ComponentCacheStore'
import { DynamicParameterStore, type DynamicParameterStorage } from './DynamicParameterStore'

export type { DynamicParameterStorage } from './DynamicParameterStore'

/**
 * 数据存储项接口
 */
export interface DataStorageItem {
  /** 数据内容 */
  data: any
  /** 存储时间戳 */
  timestamp: number
  /** 过期时间戳 */
  expiresAt?: number
  /** 数据来源信息 */
  source: {
    /** 数据源ID */
    sourceId: string
    /** 数据源类型 */
    sourceType: string
    /** 组件ID */
    componentId: string
  }
  /** 数据大小（字节） */
  size: number
  /** 缓存读取次数 */
  accessCount: number
  /** 最后访问时间 */
  lastAccessed: number
  /** 数据版本号，用于丢弃过期写入。 */
  dataVersion?: string
  /** 执行 ID，用于定位一次写入链路。 */
  executionId?: string
}

/**
 * 组件数据存储结构
 */
export interface ComponentDataStorage {
  /** 组件ID */
  componentId: string
  /** 数据源数据映射 */
  dataSources: Map<string, DataStorageItem>
  /** 合并后的数据（缓存） */
  mergedData?: DataStorageItem
  /** 组件创建时间 */
  createdAt: number
  /** 最后更新时间 */
  updatedAt: number
}

/**
 * 仓库配置选项
 */
export interface DataWarehouseConfig {
  /** 默认缓存过期时间（毫秒） */
  defaultCacheExpiry: number
  /** 最大内存使用量（MB） */
  maxMemoryUsage: number
  /** 清理检查间隔（毫秒） */
  cleanupInterval: number
  /** 最大存储项数量 */
  maxStorageItems: number
  /** 启用性能监控 */
  enablePerformanceMonitoring: boolean
  /** 是否在构造时启动后台维护；关闭后仍可显式调用 performMaintenance()。 */
  enableBackgroundMaintenance: boolean
}

/** 写入结果；旧调用方可继续忽略返回值，需要诊断时可检查拒绝原因。 */
export type DataWarehouseWriteResult =
  | { success: true; dataVersion: string; executionId: string }
  | {
      success: false
      reason: 'stale-version' | 'capacity-exceeded'
      dataVersion: string
      executionId: string
    }

/**
 * 性能监控数据
 */
export interface PerformanceMetrics {
  /** 总内存使用（MB） */
  memoryUsage: number
  /** 存储项数量 */
  itemCount: number
  /** 组件数量 */
  componentCount: number
  /** 平均响应时间（ms） */
  averageResponseTime: number
  /** 缓存命中率 */
  cacheHitRate: number
  /** get 请求总数。 */
  totalRequests: number
  /** 缓存命中次数。 */
  cacheHits: number
  /** 缓存未命中次数。 */
  cacheMisses: number
  /** 最后清理时间 */
  lastCleanupTime: number
}

/**
 * 增强数据仓库类
 * 提供多数据源隔离存储和性能优化功能
 */
export class EnhancedDataWarehouse {
  /** 组件数据存储 */
  private componentStorage = new Map<string, ComponentDataStorage>()

  private componentCache = new ComponentCacheStore(this.componentStorage, {
    getDefaultCacheExpiry: () => this.config.defaultCacheExpiry,
    calculateDataSize: (data) => this.calculateDataSize(data),
    isExpired: (item) => this.isExpired(item)
  })

  /** 组件级响应式通知器，避免全局广播触发所有组件重算。 */
  private componentChangeNotifiers = new Map<string, Ref<number>>()

  /** 历史全局通知器已废弃，保留注释仅用于解释为何改用组件级通知器。 */
  // private dataChangeNotifier = ref(0) // 已移除，使用组件级通知器替代

  /** 记录每个组件最近一次接受的数据版本。 */
  private componentLatestVersions = new Map<string, string>()

  /** 独立动态参数存储 */
  private parameterStorage = new DynamicParameterStore()

  /** 仓库配置 */
  private config: DataWarehouseConfig

  /** 性能监控数据 */
  private metrics: PerformanceMetrics

  /** 清理定时器 */
  private cleanupTimer: NodeJS.Timeout | null = null

  /** 性能监控定时器 */
  private metricsTimer: NodeJS.Timeout | null = null

  constructor(config: Partial<DataWarehouseConfig> = {}) {
    // 初始化配置
    this.config = {
      defaultCacheExpiry: 5 * 60 * 1000, // 5分钟
      maxMemoryUsage: 100, // 100MB
      cleanupInterval: 60 * 1000, // 1分钟
      maxStorageItems: 1000,
      enablePerformanceMonitoring: true,
      enableBackgroundMaintenance: true,
      ...config
    }

    // 后台维护可关闭；调用方仍可通过 performMaintenance() 显式完成本地清理和统计。
    this.metrics = this.createInitialPerformanceMetrics()
    if (this.config.enableBackgroundMaintenance) {
      this.startCleanupTimer()
      if (this.config.enablePerformanceMonitoring) {
        this.startMetricsCollection()
      }
    }
  }

  /**
   * 存储组件数据。
   * @returns 可观察的写入结果；旧调用方可继续忽略返回值。
   */
  storeComponentData(
    componentId: string,
    sourceId: string,
    data: any,
    sourceType: string = 'unknown',
    customExpiry?: number
  ): DataWarehouseWriteResult {
    const now = Date.now()
    const startTime = now
    const executionId = `${componentId}-${now}-${Math.random().toString(36).substr(2, 9)}`
    const dataVersion = this.generateDataVersion(componentId, data)

    if (!this.shouldAcceptData(componentId, dataVersion)) {
      return { success: false, reason: 'stale-version', dataVersion, executionId }
    }

    const dataSize = this.calculateDataSize(data)
    if (this.shouldRejectStorage(dataSize)) {
      return { success: false, reason: 'capacity-exceeded', dataVersion, executionId }
    }

    const componentStorage = this.componentCache.getOrCreateComponentStorage(componentId, now)
    const storageItem = this.componentCache.createStorageItem(
      componentId,
      sourceId,
      data,
      sourceType,
      dataSize,
      dataVersion,
      executionId,
      customExpiry,
      now
    )

    this.componentCache.writeDataSourceItem(componentStorage, sourceId, storageItem, now)
    this.updateLatestDataVersion(componentId, dataVersion)
    this.notifyComponentChanged(componentId)
    this.recordTimedOperation(startTime, 'store')
    return { success: true, dataVersion, executionId }
  }

  /**
   * 获取组件数据
   * @param componentId 组件ID
   * @returns 组件完整数据或 null
   */
  getComponentData(componentId: string): Record<string, any> | null {
    const startTime = Date.now()
    this.trackComponentDependency(componentId)

    const componentStorage = this.componentStorage.get(componentId)
    if (!componentStorage) {
      this.recordTimedOperation(startTime, 'get', false)
      return null
    }

    const mergedItem = this.componentCache.getValidMergedStorageItem(componentStorage)
    if (mergedItem) {
      this.recordTimedOperation(startTime, 'get', true)
      return mergedItem.data
    }

    const componentData = this.componentCache.collectValidComponentData(componentStorage)
    if (!componentData.hasData) {
      this.recordTimedOperation(startTime, 'get', false)
      return null
    }

    this.recordTimedOperation(startTime, 'get', true)
    const finalData = this.componentCache.unwrapCompleteComponentData(componentData.data)
    this.componentCache.cacheMergedComponentData(componentStorage, finalData)

    return finalData
  }

  private trackComponentDependency(componentId: string): void {
    void this.getOrCreateComponentNotifier(componentId).value
  }

  private notifyComponentChanged(componentId: string): void {
    this.getOrCreateComponentNotifier(componentId).value++
  }

  private recordTimedOperation(startTime: number, operation: 'store' | 'get', cacheHit?: boolean): void {
    this.updateMetrics(Date.now() - startTime, operation, cacheHit)
  }

  // ==================== 版本与调试辅助 ====================

  /**
   * 生成数据版本号，供乱序写入比较先后顺序。
   */
  private generateDataVersion(componentId: string, data: any): string {
    const dataHash = this.calculateDataHash(data)
    const timestamp = Date.now()
    return `${componentId}-${timestamp}-${dataHash}`
  }

  /**
   * 检查当前写入是否比已缓存版本更新。
   */
  private shouldAcceptData(componentId: string, dataVersion: string): boolean {
    const latestVersion = this.componentLatestVersions.get(componentId)
    if (!latestVersion) {
      return true // 首次存储，直接接受
    }

    // 提取时间戳进行比较
    const currentTimestamp = this.extractTimestampFromVersion(dataVersion)
    const latestTimestamp = this.extractTimestampFromVersion(latestVersion)

    return currentTimestamp >= latestTimestamp
  }

  /**
   * 更新组件最近一次接受的数据版本。
   */
  private updateLatestDataVersion(componentId: string, dataVersion: string): void {
    this.componentLatestVersions.set(componentId, dataVersion)
  }

  /**
   * 从版本号右侧提取时间戳，兼容包含连字符的组件 ID。
   */
  private extractTimestampFromVersion(version: string): number {
    const hashSeparator = version.lastIndexOf('-')
    if (hashSeparator <= 0) return 0

    const timestampSeparator = version.lastIndexOf('-', hashSeparator - 1)
    if (timestampSeparator < 0) return 0

    const timestamp = Number(version.slice(timestampSeparator + 1, hashSeparator))
    return Number.isFinite(timestamp) ? timestamp : 0
  }

  /**
   * 计算数据哈希值，避免版本号只依赖时间戳。
   * 不可序列化数据使用固定标记，保证版本生成可复现。
   */
  private calculateDataHash(data: any): string {
    try {
      const dataString = JSON.stringify(data)
      let hash = 0
      for (let i = 0; i < dataString.length; i++) {
        const char = dataString.charCodeAt(i)
        hash = (hash << 5) - hash + char
        hash = hash & hash // 转换为32位整数
      }
      return Math.abs(hash).toString(36)
    } catch (_error) {
      return 'unserializable'
    }
  }

  /**
   * 提取数据中的关键数值，用于日志和调试追踪。
   * 智能提取各种数据结构中的核心数值
   */
  private extractDataValue(data: any): any {
    if (!data) return undefined

    // 尝试多种可能的数值字段
    const possibleFields = ['value', 'val', 'data', 'result', 'number', 'count']

    // 直接数值
    if (typeof data === 'number') return data

    // 对象中的数值字段
    if (typeof data === 'object' && data !== null) {
      for (const field of possibleFields) {
        if (data[field] !== undefined) {
          return data[field]
        }
      }

      // 检查嵌套结构
      if (data.deviceData?.data?.value !== undefined) {
        return data.deviceData.data.value
      }

      // 如果是数组，尝试提取第一个元素的值
      if (Array.isArray(data) && data.length > 0) {
        return this.extractDataValue(data[0])
      }
    }

    return data
  }

  /**
   * 获取单个数据源数据
   * @param componentId 组件ID
   * @param sourceId 数据源ID
   * @returns 数据源数据或null
   */
  getDataSourceData(componentId: string, sourceId: string): any {
    const componentStorage = this.componentStorage.get(componentId)
    if (!componentStorage) {
      return null
    }

    const item = this.componentCache.getValidDataSourceItem(componentStorage, sourceId)
    if (!item) {
      return null
    }

    this.componentCache.markStorageItemAccessed(item)
    return item.data
  }

  /**
   * 清除组件缓存
   * @param componentId 组件ID
   */
  clearComponentCache(componentId: string): void {
    const componentStorage = this.componentStorage.get(componentId)
    if (componentStorage) {
      this.componentStorage.delete(componentId)
      // 同时清理组件级通知器，避免缓存删除后残留响应式引用。
      this.componentChangeNotifiers.delete(componentId)
    }
  }

  /**
   * 强制清除组件的合并数据缓存，保持响应式依赖。
   * @param componentId 组件ID
   */
  clearComponentMergedCache(componentId: string): void {
    const componentStorage = this.componentStorage.get(componentId)
    if (componentStorage) {
      // 无条件清除合并缓存，避免并发写入后继续复用旧聚合结果。
      componentStorage.mergedData = undefined

      // 无论是否存在旧缓存，都主动通知依赖方重新拉取数据。
      this.getOrCreateComponentNotifier(componentId).value++
    }
  }

  /**
   * 清除数据源缓存
   * @param componentId 组件ID
   * @param sourceId 数据源ID
   */
  clearDataSourceCache(componentId: string, sourceId: string): void {
    const componentStorage = this.componentStorage.get(componentId)
    if (!componentStorage) {
      return
    }

    const removed = componentStorage.dataSources.delete(sourceId)
    if (removed) {
      this.componentCache.invalidateMergedData(componentStorage)
      this.notifyExistingComponentChanged(componentId)
    }
  }

  private notifyExistingComponentChanged(componentId: string): void {
    const componentNotifier = this.componentChangeNotifiers.get(componentId)
    if (componentNotifier) {
      componentNotifier.value++
    }
  }

  /**
   * 清除所有缓存
   */
  clearAllCache(): void {
    this.componentStorage.clear()
    this.parameterStorage.clear()
    // 清理所有组件级通知器，避免整仓销毁后仍保留响应式引用。
    this.componentChangeNotifiers.clear()
  }

  /**
   * 设置缓存过期时间
   * @param milliseconds 过期时间（毫秒）
   */
  setCacheExpiry(milliseconds: number): void {
    this.config.defaultCacheExpiry = milliseconds
  }

  /**
   * 获取性能监控数据
   */
  getPerformanceMetrics(): PerformanceMetrics {
    this.updateCurrentMetrics()
    return { ...this.metrics }
  }

  resetPerformanceMetrics(): void {
    this.metrics = this.createInitialPerformanceMetrics()
    this.updateCurrentMetrics()
  }

  /**
   * 获取存储统计信息
   */
  getStorageStats() {
    let totalItems = 0
    let totalSize = 0
    const componentStats: Record<string, any> = {}

    for (const [componentId, storage] of this.componentStorage) {
      const componentSize = Array.from(storage.dataSources.values()).reduce((sum, item) => sum + item.size, 0)

      componentStats[componentId] = {
        dataSourceCount: storage.dataSources.size,
        totalSize: componentSize,
        createdAt: storage.createdAt,
        updatedAt: storage.updatedAt
      }

      totalItems += storage.dataSources.size
      totalSize += componentSize
    }

    return {
      totalComponents: this.componentStorage.size,
      totalDataSources: totalItems,
      totalSize,
      memoryUsageMB: totalSize / (1024 * 1024),
      componentStats,
      config: this.config
    }
  }

  /** 存储动态参数。 */
  storeDynamicParameter(nameOrScope: string, parameterOrName: DynamicParameterStorage | string, value?: any): void {
    this.parameterStorage.store(nameOrScope, parameterOrName, value)
  }

  /** 获取动态参数。 */
  getDynamicParameter(nameOrScope: string, name?: string): DynamicParameterStorage | any | null {
    return this.parameterStorage.get(nameOrScope, name)
  }

  getAllDynamicParameters(scope?: string): Record<string, any> {
    return this.parameterStorage.getAll(scope)
  }

  performMaintenance(): void {
    this.performCleanup()
    this.updateCurrentMetrics()
  }

  /**
   * 销毁数据仓库
   */
  destroy(): void {
    // 停止定时器
    if (this.cleanupTimer) {
      clearInterval(this.cleanupTimer)
      this.cleanupTimer = null
    }

    if (this.metricsTimer) {
      clearInterval(this.metricsTimer)
      this.metricsTimer = null
    }

    // 清除所有数据（已包含组件级响应式通知器清理）
    this.clearAllCache()
  }

  // ==================== 私有方法 ====================

  /**
   * 检查数据项是否过期
   */
  private isExpired(item: DataStorageItem): boolean {
    return item.expiresAt !== undefined && Date.now() > item.expiresAt
  }

  /**
   * 计算数据大小（估算）
   */
  private calculateDataSize(data: any): number {
    try {
      return JSON.stringify(data).length * 2 // 粗略估算UTF-16字节数
    } catch {
      return 1024 // 默认1KB
    }
  }

  /**
   * 检查是否应该拒绝存储（内存限制）
   */
  private shouldRejectStorage(dataSize: number): boolean {
    const currentMemoryMB = this.getCurrentMemoryUsage()
    const newDataMB = dataSize / (1024 * 1024)

    return (
      currentMemoryMB + newDataMB > this.config.maxMemoryUsage ||
      this.getTotalItemCount() >= this.config.maxStorageItems
    )
  }

  /**
   * 获取当前内存使用量（MB）
   */
  private getCurrentMemoryUsage(): number {
    let totalSize = 0
    for (const storage of this.componentStorage.values()) {
      for (const item of storage.dataSources.values()) {
        totalSize += item.size
      }
      if (storage.mergedData) {
        totalSize += storage.mergedData.size
      }
    }
    return totalSize / (1024 * 1024)
  }

  /**
   * 获取总存储项数量
   */
  private getTotalItemCount(): number {
    let count = 0
    for (const storage of this.componentStorage.values()) {
      count += storage.dataSources.size
      if (storage.mergedData) count++
    }
    return count
  }

  /**
   * 启动清理定时器
   */
  private startCleanupTimer(): void {
    this.cleanupTimer = setInterval(() => {
      this.performCleanup()
    }, this.config.cleanupInterval)
    // 浏览器环境下 setInterval 返回 number，Node.js 下返回具有 unref() 的 Timeout 对象
    ;(this.cleanupTimer as { unref?: () => void }).unref?.()
  }

  /**
   * 执行清理操作
   */
  private performCleanup(): void {
    this.removeExpiredComponentData()
    this.pruneMemoryPressure()
    this.metrics.lastCleanupTime = Date.now()
  }

  private removeExpiredComponentData(): void {
    for (const [componentId, storage] of this.componentStorage) {
      this.removeExpiredDataSources(storage)
      this.removeExpiredMergedData(storage)

      if (this.isEmptyComponentStorage(storage)) {
        this.componentStorage.delete(componentId)
      }
    }
  }

  private removeExpiredDataSources(storage: ComponentDataStorage): void {
    for (const [sourceId, item] of storage.dataSources) {
      if (this.isExpired(item)) {
        storage.dataSources.delete(sourceId)
      }
    }
  }

  private removeExpiredMergedData(storage: ComponentDataStorage): void {
    if (storage.mergedData && this.isExpired(storage.mergedData)) {
      this.componentCache.invalidateMergedData(storage)
    }
  }

  private isEmptyComponentStorage(storage: ComponentDataStorage): boolean {
    return storage.dataSources.size === 0 && !storage.mergedData
  }

  private pruneMemoryPressure(): void {
    if (!this.isMemoryPressureHigh()) {
      return
    }

    this.getLeastAccessedItems(10).forEach(({ componentId, sourceId }) => {
      this.clearDataSourceCache(componentId, sourceId)
    })
  }

  private isMemoryPressureHigh(): boolean {
    return this.getCurrentMemoryUsage() > this.config.maxMemoryUsage * 0.8
  }

  /**
   * 获取最少访问的数据项
   */
  private getLeastAccessedItems(count: number): Array<{ componentId: string; sourceId: string }> {
    const allItems: Array<{ componentId: string; sourceId: string; accessCount: number; lastAccessed: number }> = []

    for (const [componentId, storage] of this.componentStorage) {
      for (const [sourceId, item] of storage.dataSources) {
        allItems.push({
          componentId,
          sourceId,
          accessCount: item.accessCount,
          lastAccessed: item.lastAccessed
        })
      }
    }

    // 按缓存读取次数和最后访问时间排序
    allItems.sort((a, b) => {
      if (a.accessCount !== b.accessCount) {
        return a.accessCount - b.accessCount
      }
      return a.lastAccessed - b.lastAccessed
    })

    return allItems.slice(0, count)
  }

  /**
   * 启动性能监控
   */
  private startMetricsCollection(): void {
    this.metricsTimer = setInterval(() => {
      this.updateCurrentMetrics()
    }, 30000) // 30秒更新一次
    // 浏览器环境下 setInterval 返回 number，Node.js 下返回具有 unref() 的 Timeout 对象
    ;(this.metricsTimer as { unref?: () => void }).unref?.()
  }

  /**
   * 更新当前监控数据
   */
  private createInitialPerformanceMetrics(): PerformanceMetrics {
    return {
      memoryUsage: 0,
      itemCount: 0,
      componentCount: 0,
      averageResponseTime: 0,
      cacheHitRate: 0,
      totalRequests: 0,
      cacheHits: 0,
      cacheMisses: 0,
      lastCleanupTime: Date.now()
    }
  }

  private getOrCreateComponentNotifier(componentId: string): Ref<number> {
    let componentNotifier = this.componentChangeNotifiers.get(componentId)
    if (!componentNotifier) {
      componentNotifier = ref(0)
      this.componentChangeNotifiers.set(componentId, componentNotifier)
    }
    return componentNotifier
  }

  private updateCurrentMetrics(): void {
    this.metrics.memoryUsage = this.getCurrentMemoryUsage()
    this.metrics.itemCount = this.getTotalItemCount()
    this.metrics.componentCount = this.componentStorage.size
  }

  /**
   * 更新性能监控指标
   */
  private updateMetrics(responseTime: number, operation: 'store' | 'get', cacheHit?: boolean): void {
    const normalizedResponseTime = Math.max(responseTime, 0.001)

    if (operation === 'get' && cacheHit !== undefined) {
      const previousRequests = this.metrics.totalRequests
      this.metrics.totalRequests += 1

      if (cacheHit) {
        this.metrics.cacheHits += 1
      } else {
        this.metrics.cacheMisses += 1
      }

      this.metrics.averageResponseTime =
        (this.metrics.averageResponseTime * previousRequests + normalizedResponseTime) / this.metrics.totalRequests
      this.metrics.cacheHitRate =
        this.metrics.totalRequests > 0 ? this.metrics.cacheHits / this.metrics.totalRequests : 0
      return
    }

    this.metrics.averageResponseTime = (this.metrics.averageResponseTime + normalizedResponseTime) / 2
  }
}

/**
 * 默认数据仓库实例。
 * 模块导入不启动后台定时器；应用可显式调用 performMaintenance() 完成本地维护。
 */
export const dataWarehouse = new EnhancedDataWarehouse({ enableBackgroundMaintenance: false })

/**
 * 创建自定义配置的数据仓库实例
 */
export function createDataWarehouse(config: Partial<DataWarehouseConfig> = {}): EnhancedDataWarehouse {
  return new EnhancedDataWarehouse(config)
}
