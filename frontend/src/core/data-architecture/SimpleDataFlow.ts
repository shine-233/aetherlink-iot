/**
 * 文件用途: 简化数据流管理器。
 * 核心逻辑: 通过显式配置更新按绑定规则触发数据源刷新，连接编辑器属性和运行时数据。
 * 关键注意事项: 白名单、绑定路径和触发规则会直接影响组件数据刷新频率和正确性。
 * 重构建议: 把属性监听、触发判定和数据执行拆成独立服务，进一步缩小单例状态范围。
 */

import { simpleDataBridge } from './SimpleDataBridge'
import { dataSourceBindingConfig, type ComponentBindingConfig } from './DataSourceBindingConfig'
import { createLogger } from '@/utils/logger'

const logger = createLogger('SimpleDataFlow')

/**
 * 属性变更事件接口
 */
export interface PropertyChangeEvent {
  componentId: string
  propertyPath: string // 如 'base.deviceId' 或 'component.startTime'
  oldValue: any
  newValue: any
  timestamp: number
}

/**
 * 数据源执行配置接口
 */
export interface DataSourceExecutionConfig {
  componentId: string
  componentType: string
  httpParams: Record<string, any>
  forceRefresh?: boolean
}

/**
 * 简化数据流管理器类
 */
export class SimpleDataFlow {
  private static instance: SimpleDataFlow | null = null

  // 组件配置仅通过显式 API 更新，不需要 Vue 响应式代理。
  private componentConfigs = new Map<string, any>()

  // 属性变更监听器
  private propertyWatchers = new Map<string, Set<(event: PropertyChangeEvent) => void>>()

  // 防抖控制
  private debounceTimers = new Map<string, ReturnType<typeof setTimeout>>()
  private readonly DEBOUNCE_TIME = 100 // 100ms 防抖

  // 执行中的组件集合（防止重复执行）
  private executingComponents = new Set<string>()

  private constructor() {}

  /**
   * 获取单例实例
   */
  static getInstance(): SimpleDataFlow {
    if (!SimpleDataFlow.instance) {
      SimpleDataFlow.instance = new SimpleDataFlow()
    }
    return SimpleDataFlow.instance
  }

  /**
   * 注册组件配置
   * @param componentId 组件ID
   * @param config 组件完整配置
   */
  registerComponent(componentId: string, config: any): void {
    logger.debug(`[SimpleDataFlow] 注册组件:`, { componentId, hasConfig: !!config })
    this.componentConfigs.set(componentId, config)
  }

  /**
   * 更新组件配置的某个部分
   * @param componentId 组件ID
   * @param section 配置节 (base, component, dataSource, interaction)
   * @param newConfig 新配置内容
   */
  updateComponentConfig(componentId: string, section: string, newConfig: any): void {
    const currentConfig = this.componentConfigs.get(componentId) || {}
    const oldSectionConfig = currentConfig[section] || {}

    // 更新配置
    currentConfig[section] = { ...oldSectionConfig, ...newConfig }
    this.componentConfigs.set(componentId, currentConfig)

    logger.debug(`[SimpleDataFlow] 组件配置更新:`, {
      componentId,
      section,
      hasChanges: true,
      newConfigKeys: Object.keys(newConfig)
    })

    // 检查是否有属性变更需要触发数据源
    this.checkAndTriggerDataSource(componentId, section, oldSectionConfig, newConfig)
  }

  /**
   * 检查属性变更并触发数据源（如果需要）
   */
  private checkAndTriggerDataSource(componentId: string, section: string, oldConfig: any, newConfig: any): void {
    const changedProperties: PropertyChangeEvent[] = []

    // 检测具体的属性变更
    for (const [key, newValue] of Object.entries(newConfig)) {
      const oldValue = oldConfig[key]
      if (oldValue !== newValue) {
        const propertyPath = `${section}.${key}`

        changedProperties.push({
          componentId,
          propertyPath,
          oldValue,
          newValue,
          timestamp: Date.now()
        })
      }
    }

    if (changedProperties.length === 0) {
      return
    }

    logger.debug(`[SimpleDataFlow] 检测到属性变更:`, {
      componentId,
      section,
      changedProperties: changedProperties.map(p => ({
        property: p.propertyPath,
        oldValue: p.oldValue,
        newValue: p.newValue
      }))
    })

    // 检查是否有任何变更的属性在触发白名单中
    const config = this.componentConfigs.get(componentId)
    const componentType = config?.componentType

    const triggerProperties = changedProperties.filter(change =>
      this.shouldTriggerDataSource(change.propertyPath, componentType)
    )

    if (triggerProperties.length > 0) {
      logger.debug(`[SimpleDataFlow] 触发数据源更新:`, {
        componentId,
        triggerProperties: triggerProperties.map(p => p.propertyPath)
      })

      // 防抖执行数据源更新
      this.debounceDataSourceExecution(componentId, triggerProperties)
    }

    // 触发属性变更监听器
    changedProperties.forEach(change => {
      this.notifyPropertyWatchers(change)
    })
  }

  /**
   * 检查属性是否在触发白名单中
   */
  private shouldTriggerDataSource(propertyPath: string, componentType?: string): boolean {
    return dataSourceBindingConfig.shouldTriggerDataSource(propertyPath, componentType)
  }

  /**
   * 防抖执行数据源更新
   */
  private debounceDataSourceExecution(componentId: string, triggerProperties: PropertyChangeEvent[]): void {
    // 清除之前的定时器
    const existingTimer = this.debounceTimers.get(componentId)
    if (existingTimer) {
      clearTimeout(existingTimer)
    }

    // 设置新的定时器
    const timer = setTimeout(() => {
      this.executeDataSource(componentId, triggerProperties)
      this.debounceTimers.delete(componentId)
    }, this.DEBOUNCE_TIME)

    this.debounceTimers.set(componentId, timer)
  }

  /**
   * 执行数据源更新
   */
  private async executeDataSource(componentId: string, triggerProperties: PropertyChangeEvent[]): Promise<void> {
    // 防止重复执行
    if (this.executingComponents.has(componentId)) {
      logger.debug(`[SimpleDataFlow] 组件正在执行中，跳过:`, { componentId })
      return
    }

    this.executingComponents.add(componentId)

    try {
      logger.debug(`[SimpleDataFlow] 开始执行数据源:`, {
        componentId,
        triggerProperties: triggerProperties.map(p => p.propertyPath)
      })

      // 获取组件配置
      const config = this.componentConfigs.get(componentId)
      if (!config || !config.dataSource) {
        logger.debug(`[SimpleDataFlow] 组件无数据源配置:`, { componentId })
        return
      }

      // 构建HTTP参数
      const httpParams = this.buildHttpParams(componentId, config)

      logger.debug(`[SimpleDataFlow] 构建HTTP参数:`, {
        componentId,
        httpParams
      })

      // 清除缓存，确保获取最新数据
      simpleDataBridge.clearComponentCache(componentId)

      // 使用 VisualEditorBridge 执行数据源
      const { getVisualEditorBridge } = await import('./VisualEditorBridge')
      const visualEditorBridge = getVisualEditorBridge()

      // 构建完整配置对象
      const executionConfig = {
        base: config.base || {},
        dataSource: config.dataSource,
        component: config.component || {},
        interaction: config.interaction || {},
        // 注入构建的HTTP参数
        _httpParams: httpParams
      }

      // 执行数据源
      const result = await visualEditorBridge.updateComponentExecutor(
        componentId,
        config.componentType || 'widget',
        executionConfig
      )

      logger.debug(`[SimpleDataFlow] 数据源执行完成:`, {
        componentId,
        success: !!result
      })
    } catch (error) {
      logger.error(`[SimpleDataFlow] 数据源执行失败:`, {
        componentId,
        error: error instanceof Error ? error.message : error
      })
    } finally {
      this.executingComponents.delete(componentId)
    }
  }

  /**
   * 构建HTTP参数
   * 根据自动绑定规则将组件属性映射到HTTP参数
   */
  private buildHttpParams(componentId: string, config: any): Record<string, any> {
    const componentType = config.componentType

    // 🚀 新增：检查是否有autoBind配置
    const autoBindConfig = this.getAutoBindConfig(config)

    if (autoBindConfig) {
      return dataSourceBindingConfig.buildAutoBindParams(config, autoBindConfig, componentType)
    }

    return dataSourceBindingConfig.buildHttpParams(config, componentType)
  }

  /**
   * 按数据源、组件实例、组件类型的优先级解析 autoBind 配置。
   * 组件类型配置只保存启用状态，启用时使用兼容的 loose 模式。
   */
  private getAutoBindConfig(config: any): import('./DataSourceBindingConfig').AutoBindConfig | null {
    if (config.dataSource?.autoBind) {
      return config.dataSource.autoBind
    }

    if (config.autoBind) {
      return config.autoBind
    }

    const componentConfig = dataSourceBindingConfig.getComponentConfig(config.componentType)
    if (componentConfig?.autoBindEnabled) {
      return {
        enabled: true,
        mode: 'loose'
      }
    }

    return null
  }

  /**
   * 添加属性变更监听器
   */
  addPropertyWatcher(propertyPath: string, callback: (event: PropertyChangeEvent) => void): () => void {
    if (!this.propertyWatchers.has(propertyPath)) {
      this.propertyWatchers.set(propertyPath, new Set())
    }

    this.propertyWatchers.get(propertyPath)!.add(callback)

    // 返回移除监听器的函数
    return () => {
      const watchers = this.propertyWatchers.get(propertyPath)
      if (watchers) {
        watchers.delete(callback)
        if (watchers.size === 0) {
          this.propertyWatchers.delete(propertyPath)
        }
      }
    }
  }

  /**
   * 通知属性变更监听器
   */
  private notifyPropertyWatchers(event: PropertyChangeEvent): void {
    const watchers = this.propertyWatchers.get(event.propertyPath)
    if (watchers) {
      watchers.forEach(callback => {
        try {
          callback(event)
        } catch (error) {
          logger.error(`[SimpleDataFlow] 属性监听器执行出错:`, {
            propertyPath: event.propertyPath,
            error: error instanceof Error ? error.message : error
          })
        }
      })
    }
  }

  /**
   * 手动触发组件数据源执行
   * @param componentId 组件ID
   * @param reason 触发原因
   */
  async triggerDataSource(componentId: string, reason: string = 'manual'): Promise<void> {
    logger.debug(`[SimpleDataFlow] 手动触发数据源:`, { componentId, reason })

    const config = this.componentConfigs.get(componentId)
    if (!config) {
      logger.warn(`[SimpleDataFlow] 组件配置不存在:`, { componentId })
      return
    }

    // 创建一个虚拟的属性变更事件来触发执行
    const virtualEvent: PropertyChangeEvent = {
      componentId,
      propertyPath: 'manual.trigger',
      oldValue: null,
      newValue: reason,
      timestamp: Date.now()
    }

    await this.executeDataSource(componentId, [virtualEvent])
  }

  /**
   * 移除组件注册
   */
  unregisterComponent(componentId: string): void {
    logger.debug(`[SimpleDataFlow] 注销组件:`, { componentId })

    this.componentConfigs.delete(componentId)

    // 清除相关的防抖定时器
    const timer = this.debounceTimers.get(componentId)
    if (timer) {
      clearTimeout(timer)
      this.debounceTimers.delete(componentId)
    }

    // 移除执行状态
    this.executingComponents.delete(componentId)
  }

  /**
   * 获取当前触发白名单
   */
  getTriggerWhitelist(componentType?: string): string[] {
    return dataSourceBindingConfig.getAllTriggerRules(componentType).map(rule => rule.propertyPath)
  }

  /**
   * 动态添加触发属性到白名单
   */
  addTriggerProperty(propertyPath: string, enabled: boolean = true, debounceMs?: number): void {
    dataSourceBindingConfig.addCustomTriggerRule({
      propertyPath,
      enabled,
      debounceMs,
      description: `动态添加的触发规则: ${propertyPath}`
    })
  }

  /**
   * 动态添加自动绑定规则
   */
  addBindingRule(propertyPath: string, paramName: string, transform?: (value: any) => any, required?: boolean): void {
    dataSourceBindingConfig.addCustomBindingRule({
      propertyPath,
      paramName,
      transform,
      required,
      description: `动态添加的绑定规则: ${propertyPath} → ${paramName}`
    })
  }

  /**
   * 设置组件特定的绑定配置
   */
  setComponentBindingConfig(componentType: string, config: ComponentBindingConfig): void {
    dataSourceBindingConfig.setComponentConfig(componentType, config)
  }

  /**
   * 获取绑定配置的调试信息
   */
  getBindingDebugInfo(componentType?: string): any {
    return dataSourceBindingConfig.getDebugInfo(componentType)
  }
}

// 共享实例保持现有运行时契约，但模块导入不会写入宿主全局。
export const simpleDataFlow = SimpleDataFlow.getInstance()

const SIMPLE_DATA_FLOW_DEBUG_GLOBAL = '__simpleDataFlow'

/**
 * 按需安装调试实例。返回的清理函数只撤销本次安装，不覆盖宿主后续写入。
 */
export function installSimpleDataFlowDebugGlobal(target: typeof globalThis = globalThis): () => void {
  const debugTarget = target as typeof globalThis & Record<string, unknown>
  const hadOwnValue = Object.prototype.hasOwnProperty.call(debugTarget, SIMPLE_DATA_FLOW_DEBUG_GLOBAL)
  const previousValue = debugTarget[SIMPLE_DATA_FLOW_DEBUG_GLOBAL]

  debugTarget[SIMPLE_DATA_FLOW_DEBUG_GLOBAL] = simpleDataFlow

  return () => {
    if (debugTarget[SIMPLE_DATA_FLOW_DEBUG_GLOBAL] !== simpleDataFlow) {
      return
    }

    if (hadOwnValue) {
      debugTarget[SIMPLE_DATA_FLOW_DEBUG_GLOBAL] = previousValue
    } else {
      delete debugTarget[SIMPLE_DATA_FLOW_DEBUG_GLOBAL]
    }
  }
}
