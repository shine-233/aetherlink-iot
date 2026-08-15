/**
 * 文件用途: 动态绑定 API facade，暴露运行时配置 binding/trigger 规则的高层入口。
 * 核心逻辑: 代理 DataSourceBindingConfig 的基础规则、组件附加规则和自动绑定启用状态。
 * 关键注意事项: 清空操作只移除基础规则，自定义扩展和组件配置继续保留。
 */

import {
  dataSourceBindingConfig,
  type BindingRule,
  type TriggerRule,
  type AutoBindConfig
} from './DataSourceBindingConfig'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DynamicBindingAPI')

/**
 * 运行时绑定规则配置入口；默认兼容表可替换或移除，扩展规则独立保留。
 */
export class DynamicBindingAPI {
  /**
   * 清空基础默认规则；自定义扩展和组件配置保持不变。
   */
  static clearAllDefaultRules(): void {
    dataSourceBindingConfig.clearAllRules()
    logger.debug('[DynamicBindingAPI] 已清空基础默认规则，扩展规则与组件配置保持不变')
  }

  /** 注册或替换指定属性路径的基础绑定规则。 */
  static addCustomBinding(config: {
    propertyPath: string
    paramName: string
    transform?: (value: any) => any
    required?: boolean
    description?: string
  }): void {
    dataSourceBindingConfig.registerBindingRule({
      propertyPath: config.propertyPath,
      paramName: config.paramName,
      transform: config.transform,
      required: config.required || false,
      description: config.description || `自定义绑定: ${config.propertyPath} → ${config.paramName}`
    })
  }

  /** 注册或替换指定属性路径的基础触发规则。 */
  static addCustomTrigger(config: {
    propertyPath: string
    enabled?: boolean
    debounceMs?: number
    description?: string
  }): void {
    dataSourceBindingConfig.registerTriggerRule({
      propertyPath: config.propertyPath,
      enabled: config.enabled !== false,
      debounceMs: config.debounceMs ?? 100,
      description: config.description || `自定义触发: ${config.propertyPath}`
    })
  }

  /** 移除指定属性路径的基础绑定规则。 */
  static removeBinding(propertyPath: string): boolean {
    return dataSourceBindingConfig.removeBindingRule(propertyPath)
  }

  /** 移除指定属性路径的基础触发规则。 */
  static removeTrigger(propertyPath: string): boolean {
    return dataSourceBindingConfig.removeTriggerRule(propertyPath)
  }

  /**
   * 配置组件附加绑定/触发规则和自动绑定开关；同路径基础规则仍优先。
   */
  static configureCustomComponent(
    componentType: string,
    config: {
      bindings: Array<{
        propertyPath: string
        paramName: string
        transform?: (value: any) => any
        required?: boolean
      }>
      triggers: Array<{
        propertyPath: string
        enabled?: boolean
        debounceMs?: number
      }>
      autoBind?: AutoBindConfig
    }
  ): void {
    dataSourceBindingConfig.setComponentConfig(componentType, {
      componentType,
      additionalBindings: config.bindings.map(b => ({
        propertyPath: b.propertyPath,
        paramName: b.paramName,
        transform: b.transform,
        required: b.required || false,
        description: `${componentType}组件专用绑定: ${b.propertyPath}`
      })),
      additionalTriggers: config.triggers.map(t => ({
        propertyPath: t.propertyPath,
        enabled: t.enabled !== false,
        debounceMs: t.debounceMs ?? 100,
        description: `${componentType}组件专用触发: ${t.propertyPath}`
      })),
      autoBindEnabled: config.autoBind?.enabled || false
    })

    logger.debug(`[DynamicBindingAPI] 已配置自定义组件 ${componentType}:`, {
      bindingCount: config.bindings.length,
      triggerCount: config.triggers.length,
      autoBindEnabled: config.autoBind?.enabled
    })
  }

  /** 返回基础、自定义和组件附加绑定规则的聚合结果。 */
  static getCurrentBindingRules(componentType?: string): BindingRule[] {
    return dataSourceBindingConfig.getAllBindingRules(componentType)
  }

  /** 返回对应聚合范围内已启用的触发规则。 */
  static getCurrentTriggerRules(componentType?: string): TriggerRule[] {
    return dataSourceBindingConfig.getAllTriggerRules(componentType)
  }

  /** 应用可通过公开 API 移除或替换的兼容规则模板。 */
  static applyTemplate(template: 'iot-device' | 'data-analytics' | 'user-interface' | 'custom'): void {
    switch (template) {
      case 'iot-device':
        this.applyIoTDeviceTemplate()
        break
      case 'data-analytics':
        this.applyDataAnalyticsTemplate()
        break
      case 'user-interface':
        this.applyUITemplate()
        break
      case 'custom':
        this.clearAllDefaultRules()
        break
    }
  }

  /**
   * IoT物模型 - 设备相关的绑定规则
   */
  private static applyIoTDeviceTemplate(): void {
    this.clearAllDefaultRules()

    // 设备基础属性
    this.addCustomBinding({
      propertyPath: 'base.deviceId',
      paramName: 'device_id',
      required: true,
      description: 'IoT设备ID'
    })

    this.addCustomBinding({
      propertyPath: 'base.deviceType',
      paramName: 'device_type',
      description: 'IoT设备类型'
    })

    this.addCustomBinding({
      propertyPath: 'component.sensorIds',
      paramName: 'sensors',
      transform: (ids: string[]) => ids.join(','),
      description: 'IoT传感器列表'
    })

    // 对应的触发规则
    this.addCustomTrigger({
      propertyPath: 'base.deviceId',
      debounceMs: 50,
      description: 'IoT设备切换触发'
    })

    this.addCustomTrigger({
      propertyPath: 'component.sensorIds',
      debounceMs: 200,
      description: 'IoT传感器变更触发'
    })

    logger.debug('[DynamicBindingAPI] 已应用IoT物模型')
  }

  /**
   * 数据分析模板 - 分析相关的绑定规则
   */
  private static applyDataAnalyticsTemplate(): void {
    this.clearAllDefaultRules()

    // 数据查询属性
    this.addCustomBinding({
      propertyPath: 'component.timeRange',
      paramName: 'time_range',
      transform: (range: { start: Date; end: Date }) => ({
        start: range.start.toISOString(),
        end: range.end.toISOString()
      }),
      description: '数据分析时间范围'
    })

    this.addCustomBinding({
      propertyPath: 'component.aggregationType',
      paramName: 'aggregation',
      description: '数据聚合类型'
    })

    this.addCustomBinding({
      propertyPath: 'component.groupBy',
      paramName: 'group_by',
      transform: (fields: string[]) => fields.join(','),
      description: '数据分组字段'
    })

    // 对应的触发规则
    this.addCustomTrigger({
      propertyPath: 'component.timeRange',
      debounceMs: 500,
      description: '时间范围变更触发'
    })

    this.addCustomTrigger({
      propertyPath: 'component.aggregationType',
      debounceMs: 100,
      description: '聚合类型变更触发'
    })

    logger.debug('[DynamicBindingAPI] 已应用数据分析模板')
  }

  /**
   * UI界面模板 - 界面相关的绑定规则
   */
  private static applyUITemplate(): void {
    this.clearAllDefaultRules()

    // UI状态属性
    this.addCustomBinding({
      propertyPath: 'component.selectedTab',
      paramName: 'active_tab',
      description: 'UI选中标签页'
    })

    this.addCustomBinding({
      propertyPath: 'component.filterText',
      paramName: 'search_query',
      description: 'UI搜索查询'
    })

    this.addCustomBinding({
      propertyPath: 'component.pageSize',
      paramName: 'limit',
      transform: (size: number) => Math.max(1, Math.min(100, size)),
      description: 'UI分页大小'
    })

    // 对应的触发规则
    this.addCustomTrigger({
      propertyPath: 'component.selectedTab',
      debounceMs: 50,
      description: 'UI标签页切换触发'
    })

    this.addCustomTrigger({
      propertyPath: 'component.filterText',
      debounceMs: 300,
      description: 'UI搜索输入触发'
    })

    logger.debug('[DynamicBindingAPI] 已应用UI界面模板')
  }

  /** 返回基础规则、有效聚合规则和组件配置的运行时状态。 */
  static getSystemStatus(): {
    totalBindingRules: number
    totalTriggerRules: number
    customComponentCount: number
    hasDefaultRules: boolean
    isFullyCustomized: boolean
  } {
    const allBindings = this.getCurrentBindingRules()
    const allTriggers = this.getCurrentTriggerRules()
    const debugInfo = dataSourceBindingConfig.getDebugInfo()
    const hasDefaultRules = debugInfo.baseBindingRules > 0 || debugInfo.baseTriggerRules > 0

    return {
      totalBindingRules: allBindings.length,
      totalTriggerRules: allTriggers.length,
      customComponentCount: debugInfo.componentConfigs.length,
      hasDefaultRules,
      isFullyCustomized: !hasDefaultRules
    }
  }
}

/**
 * 显式安装调试全局入口，并返回可逆清理函数。
 * 模块导入本身不会写入 globalThis；宿主调试工具可按需安装。
 */
export function installDynamicBindingDebugGlobal(target: typeof globalThis = globalThis): () => void {
  const property = '__dynamicBindingAPI'
  const hadOwnProperty = Object.prototype.hasOwnProperty.call(target, property)
  const previousValue = (target as any)[property]
  ;(target as any)[property] = DynamicBindingAPI

  return () => {
    if (hadOwnProperty) {
      ;(target as any)[property] = previousValue
    } else {
      delete (target as any)[property]
    }
  }
}
