/**
 * 文件用途: Visual Editor 统一配置服务，管理组件基础配置、组件配置、数据源配置和交互配置。
 * 核心逻辑: 读写 unified-editor 的分层配置，执行配置验证、默认值补齐、迁移和 SimpleDataBridge 同步。
 * 关键注意事项: 配置变更会影响运行时数据绑定，旧配置迁移和默认值必须与组件定义保持兼容。
 * 重构建议: 将配置校验和迁移规则提取为纯函数，并补充缺字段、旧版本配置和数据源同步测试。
 */

import { useUnifiedEditorStore } from '@/store/modules/visual-editor/unified-editor'
import { simpleDataBridge } from '@/core/data-architecture/SimpleDataBridge'
import type {
  WidgetConfiguration,
  BaseConfiguration,
  ComponentConfiguration,
  DataSourceConfiguration,
  InteractionConfiguration
} from './unified-editor'
import type { ComponentDataRequirement, SimpleDataSourceConfig } from '@/core/data-architecture/SimpleDataBridge'

/**
 * 配置变更事件类型
 */
export interface ConfigurationChangeEvent {
  widgetId: string
  section: keyof WidgetConfiguration
  oldValue: any
  newValue: any
  timestamp: Date
}

/**
 * 配置验证结果
 */
export interface ConfigurationValidationResult {
  valid: boolean
  errors: string[]
  warnings: string[]
}

/**
 * 配置迁移信息
 */
export interface ConfigurationMigration {
  fromVersion: string
  toVersion: string
  migrate: (config: any) => any
}

/**
 * 统一配置服务类
 * 🔥 这是配置管理的唯一入口，替代所有分散的配置管理逻辑
 */
export class ConfigurationService {
  private store = useUnifiedEditorStore()
  private eventBus = new EventTarget()
  private migrations: ConfigurationMigration[] = []

  // ==================== 核心配置操作 ====================

  /**
   * 获取完整的组件配置
   * 🔥 唯一的配置获取入口
   */
  getConfiguration(widgetId: string): WidgetConfiguration {
    return this.store.getFullConfiguration(widgetId)
  }

  initializeConfiguration(widgetId: string, config: WidgetConfiguration = {}): WidgetConfiguration {
    const currentConfig = this.getConfiguration(widgetId)
    const nextConfig: WidgetConfiguration = {
      ...currentConfig,
      ...config,
      base: { ...(currentConfig.base || {}), ...(config.base || {}) },
      component: { ...(currentConfig.component || {}), ...(config.component || {}) },
      dataSource: config.dataSource !== undefined ? config.dataSource : currentConfig.dataSource,
      interaction: { ...(currentConfig.interaction || {}), ...(config.interaction || {}) },
      metadata: { ...(currentConfig.metadata || {}), ...(config.metadata || {}) }
    }

    this.setConfiguration(widgetId, nextConfig)
    return this.getConfiguration(widgetId)
  }

  /**
   * 获取特定部分的配置
   */
  getConfigurationSection<T extends keyof WidgetConfiguration>(widgetId: string, section: T): WidgetConfiguration[T] {
    const fullConfig = this.getConfiguration(widgetId)
    return fullConfig[section]
  }

  /**
   * 设置完整的组件配置
   */
  setConfiguration(widgetId: string, configuration: WidgetConfiguration): void {
    // 验证配置
    const validation = this.validateConfiguration(configuration)
    if (!validation.valid) {
      throw new Error(`配置验证失败: ${validation.errors.join(', ')}`)
    }

    // 获取旧配置用于事件
    const oldConfig = this.getConfiguration(widgetId)

    // 分别设置各个部分
    if (configuration.base) {
      this.store.setBaseConfiguration(widgetId, configuration.base)
    }
    if (configuration.component) {
      this.store.setComponentConfiguration(widgetId, configuration.component)
    }
    if (configuration.dataSource) {
      this.store.setDataSourceConfiguration(widgetId, configuration.dataSource)
    }
    if (configuration.interaction) {
      this.store.setInteractionConfiguration(widgetId, configuration.interaction)
    }

    // 触发全局配置变更事件
    this.emitConfigurationChange(widgetId, 'full', oldConfig, configuration)

    if (configuration.dataSource) {
      void this.handleDataSourceSideEffects(widgetId, configuration.dataSource)
    }
  }

  /**
   * 更新特定部分的配置
   * 🔥 类型安全的配置更新
   */
  updateConfigurationSection<T extends keyof WidgetConfiguration>(
    widgetId: string,
    section: T,
    data: WidgetConfiguration[T]
  ): void {
    // 获取旧值用于事件
    const oldValue = this.getConfigurationSection(widgetId, section)

    // 根据section类型分别处理
    switch (section) {
      case 'base':
        this.store.setBaseConfiguration(widgetId, data as BaseConfiguration)
        break
      case 'component':
        this.store.setComponentConfiguration(widgetId, data as ComponentConfiguration)
        break
      case 'dataSource':
        this.store.setDataSourceConfiguration(widgetId, data as DataSourceConfiguration)
        break
      case 'interaction':
        this.store.setInteractionConfiguration(widgetId, data as InteractionConfiguration)
        break
      default:
        return
    }

    // 触发配置变更事件
    this.emitConfigurationChange(widgetId, section, oldValue, data)

    if (section === 'dataSource') {
      void this.handleDataSourceSideEffects(widgetId, data as DataSourceConfiguration)
    }
  }

  /**
   * 批量更新配置
   */
  batchUpdateConfiguration(
    updates: Array<{
      widgetId: string
      section: keyof WidgetConfiguration
      data: any
    }>
  ): void {
    updates.forEach(update => {
      this.updateConfigurationSection(update.widgetId, update.section, update.data)
    })
  }

  // ==================== 数据源管理 ====================

  /**
   * 专门的数据源配置管理
   * 🔥 解决数据源配置混乱问题
   */
  setDataSourceConfig(widgetId: string, config: DataSourceConfiguration): void {
    // 验证数据源配置
    const validation = this.validateDataSourceConfig(config)
    if (!validation.valid) {
      throw new Error(`数据源配置验证失败: ${validation.errors.join(', ')}`)
    }

    // 更新配置
    this.updateConfigurationSection(widgetId, 'dataSource', config)

    // 数据源副作用由 updateConfigurationSection 统一触发，避免重复执行。
  }

  /**
   * 更新数据源绑定
   */
  updateDataSourceBindings(widgetId: string, bindings: Record<string, any>): void {
    const currentConfig = this.getConfigurationSection(widgetId, 'dataSource')
    if (!currentConfig) {
      throw new Error(`组件 ${widgetId} 没有数据源配置`)
    }

    const updatedConfig: DataSourceConfiguration = {
      ...currentConfig,
      bindings: { ...currentConfig.bindings, ...bindings }
    }

    this.setDataSourceConfig(widgetId, updatedConfig)
  }

  /**
   * 设置运行时数据
   */
  setRuntimeData(widgetId: string, data: any): void {
    this.store.setRuntimeData(widgetId, data)

    // 触发运行时数据变更事件
    this.emitRuntimeDataChange(widgetId, data)
  }

  /**
   * 获取运行时数据
   */
  getRuntimeData(widgetId: string): any {
    return this.store.getRuntimeData(widgetId)
  }

  // ==================== 配置持久化 ====================

  /**
   * 保存配置到本地存储
   */
  async saveConfiguration(widgetId: string): Promise<void> {
    const config = this.getConfiguration(widgetId)

    // 保存到localStorage（后续可以扩展到服务器）
    const storageKey = `widget_config_${widgetId}`
    localStorage.setItem(storageKey, JSON.stringify(config))
  }

  /**
   * 从本地存储加载配置
   */
  async loadConfiguration(widgetId: string): Promise<WidgetConfiguration | null> {
    try {
      const storageKey = `widget_config_${widgetId}`
      const savedData = localStorage.getItem(storageKey)

      if (!savedData) {
        return null
      }

      const config = JSON.parse(savedData)

      // 配置迁移处理
      const migratedConfig = this.migrateConfiguration(config)

      // 验证加载的配置
      const validation = this.validateConfiguration(migratedConfig)
      if (!validation.valid) {
        return null
      }
      return migratedConfig
    } catch (error) {
      return null
    }
  }

  /**
   * 批量保存所有配置
   */
  async saveAllConfigurations(): Promise<void> {
    const nodeIds = this.store.nodes.map(node => node.id)

    await Promise.all(nodeIds.map(id => this.saveConfiguration(id)))

    this.store.markSaved()
  }

  // ==================== 配置验证 ====================

  /**
   * 验证完整配置
   */
  private validateConfiguration(config: WidgetConfiguration): ConfigurationValidationResult {
    const errors: string[] = []
    const warnings: string[] = []

    // 基础配置验证
    if (config.base) {
      if (typeof config.base.opacity !== 'undefined' && (config.base.opacity < 0 || config.base.opacity > 1)) {
        errors.push('透明度必须在0-1之间')
      }
    }

    // 数据源配置验证
    if (config.dataSource) {
      const dsValidation = this.validateDataSourceConfig(config.dataSource)
      errors.push(...dsValidation.errors)
      warnings.push(...dsValidation.warnings)
    }

    return {
      valid: errors.length === 0,
      errors,
      warnings
    }
  }

  /**
   * 验证数据源配置
   */
  private validateDataSourceConfig(config: DataSourceConfiguration): ConfigurationValidationResult {
    const errors: string[] = []
    const warnings: string[] = []

    // 检查数据源类型
    const validTypes = ['static', 'api', 'websocket', 'device', 'script']
    if (!validTypes.includes(config.type)) {
      errors.push(`无效的数据源类型: ${config.type}`)
    }

    // 类型特定验证
    switch (config.type) {
      case 'api':
        if (!config.config.url) {
          errors.push('API数据源必须提供URL')
        }
        break
      case 'websocket':
        if (!config.config.url) {
          errors.push('WebSocket数据源必须提供URL')
        }
        break
      case 'device':
        if (!config.config.deviceId) {
          errors.push('设备数据源必须提供设备ID')
        }
        break
    }

    return { valid: errors.length === 0, errors, warnings }
  }

  // ==================== 配置迁移 ====================

  /**
   * 注册配置迁移
   */
  registerMigration(migration: ConfigurationMigration): void {
    this.migrations.push(migration)
  }

  /**
   * 执行配置迁移
   */
  private migrateConfiguration(config: any): WidgetConfiguration {
    let migratedConfig = { ...config }

    for (const migration of this.migrations) {
      if (config.metadata?.version === migration.fromVersion) {
        migratedConfig = migration.migrate(migratedConfig)
      }
    }

    return migratedConfig
  }

  // ==================== 事件系统 ====================

  /**
   * 监听配置变更事件
   */
  onConfigurationChange(callback: (event: ConfigurationChangeEvent) => void): () => void {
    const handler = (event: CustomEvent<ConfigurationChangeEvent>) => {
      callback(event.detail)
    }

    this.eventBus.addEventListener('configuration-change', handler as EventListener)

    // 返回取消监听函数
    return () => {
      this.eventBus.removeEventListener('configuration-change', handler as EventListener)
    }
  }

  /**
   * 触发配置变更事件
   */
  private emitConfigurationChange(
    widgetId: string,
    section: keyof WidgetConfiguration | 'full',
    oldValue: any,
    newValue: any
  ): void {
    const event: ConfigurationChangeEvent = {
      widgetId,
      section: section as keyof WidgetConfiguration,
      oldValue,
      newValue,
      timestamp: new Date()
    }

    this.eventBus.dispatchEvent(new CustomEvent('configuration-change', { detail: event }))
  }

  /**
   * 触发运行时数据变更事件
   */
  private emitRuntimeDataChange(widgetId: string, data: any): void {
    this.eventBus.dispatchEvent(
      new CustomEvent('runtime-data-change', {
        detail: { widgetId, data, timestamp: new Date() }
      })
    )
  }

  // ==================== 数据源副作用处理 ====================

  /**
   * 处理数据源配置的副作用
   */
  private async handleDataSourceSideEffects(widgetId: string, config: DataSourceConfiguration): Promise<void> {
    // 如果是Card2.1组件，触发数据绑定更新
    if (this.store.card2Components.has(widgetId)) {
      this.store.updateDataBinding(widgetId)
    }

    // 清理旧的运行时数据
    this.setRuntimeData(widgetId, null)
    simpleDataBridge.clearComponentCache(widgetId)

    // 根据数据源类型触发相应的数据获取逻辑
    switch (config.type) {
      case 'static':
        this.handleStaticDataSource(widgetId, config)
        break
      case 'api':
        await this.refreshRuntimeData(widgetId, config)
        break
      default:
        await this.refreshRuntimeData(widgetId, config)
        break
    }
  }

  /**
   * 处理静态数据源
   */
  private handleStaticDataSource(widgetId: string, config: DataSourceConfiguration): void {
    if (config.config.data) {
      this.setRuntimeData(widgetId, config.config.data)
    }
  }

  /**
   * 重新执行数据源并写入运行时数据
   */
  async refreshRuntimeData(widgetId: string, config?: DataSourceConfiguration): Promise<void> {
    const dataSourceConfig = config || this.getConfigurationSection(widgetId, 'dataSource')
    if (!dataSourceConfig) return

    const requirement = this.buildRuntimeRequirement(widgetId, dataSourceConfig)
    if (!requirement) return

    const result = await simpleDataBridge.executeComponent(requirement)
    if (result.success) {
      this.setRuntimeData(widgetId, result.data)
      return
    }

    this.setRuntimeData(widgetId, {
      __error: true,
      message: result.error || '数据源执行失败',
      timestamp: result.timestamp
    })
  }

  private buildRuntimeRequirement(widgetId: string, config: DataSourceConfiguration): ComponentDataRequirement | null {
    const rawConfig = config as any

    if (Array.isArray(rawConfig.dataSources) && rawConfig.dataSources.length > 0) {
      return {
        componentId: rawConfig.componentId || widgetId,
        dataSources: rawConfig.dataSources
      } as unknown as ComponentDataRequirement
    }

    const simpleSource = this.createSimpleDataSourceConfig(config)
    if (!simpleSource) return null

    return {
      componentId: widgetId,
      dataSources: [simpleSource]
    }
  }

  private createSimpleDataSourceConfig(config: DataSourceConfiguration): SimpleDataSourceConfig | null {
    const sourceConfig = config.config || {}

    if (config.type === 'api') {
      if (!sourceConfig.url) {
        return null
      }

      return {
        id: sourceConfig.sourceId || 'api',
        type: 'http',
        config: {
          url: sourceConfig.url,
          method: sourceConfig.method || 'GET',
          headers: sourceConfig.headers,
          timeout: sourceConfig.timeout,
          params: sourceConfig.params || sourceConfig.parameters,
          body: sourceConfig.body || sourceConfig.data
        },
        filterPath: sourceConfig.filterPath,
        processScript: sourceConfig.processScript
      }
    }

    if (config.type === 'websocket' && sourceConfig.url) {
      return {
        id: sourceConfig.sourceId || 'websocket',
        type: 'websocket',
        config: sourceConfig,
        filterPath: sourceConfig.filterPath,
        processScript: sourceConfig.processScript
      }
    }

    return null
  }
}

// ==================== 单例模式 ====================

let configurationServiceInstance: ConfigurationService | null = null

/**
 * 获取配置服务实例（单例）
 */
export function useConfigurationService(): ConfigurationService {
  if (!configurationServiceInstance) {
    configurationServiceInstance = new ConfigurationService()
  }

  return configurationServiceInstance
}

/**
 * 重置配置服务实例（测试用）
 */
export function resetConfigurationService(): void {
  configurationServiceInstance = null
}
