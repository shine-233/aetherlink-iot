/**
 * 文件用途: 集中管理属性到 HTTP 参数的绑定规则和数据源触发规则。
 * 核心逻辑: 保留可移除、可替换的 v1-v2 默认兼容表，并允许通过公开 API 注册基础、
 * 自定义及组件级附加规则。
 * 关键注意事项: 默认规则和 debug 信息属于 visual-editor 数据源兼容契约，字段名变更会影响已保存配置。
 */

/**
 * 参数绑定规则接口
 */
export interface BindingRule {
  /** 属性路径，如 'base.deviceId' */
  propertyPath: string
  /** HTTP参数名 */
  paramName: string
  /** 数据转换函数（可选） */
  transform?: (value: unknown) => unknown
  /** 兼容必填元数据；当前参数构建不会据此强制校验缺失值 */
  required?: boolean
  /** 参数说明 */
  description?: string
}

/**
 * 触发规则接口
 */
export interface TriggerRule {
  /** 属性路径 */
  propertyPath: string
  /** 是否启用 */
  enabled: boolean
  /** 防抖时间（毫秒），默认使用全局配置 */
  debounceMs?: number
  /** 规则说明 */
  description?: string
}

/**
 * 组件特定配置接口
 */
export interface ComponentBindingConfig {
  /** 组件类型 */
  componentType: string
  /** 额外的绑定规则 */
  additionalBindings?: BindingRule[]
  /** 额外的触发规则 */
  additionalTriggers?: TriggerRule[]
  /** 是否启用自动绑定 */
  autoBindEnabled?: boolean
}

/**
 * 自动绑定配置接口
 * 用于简化数据源配置，提供autoBind选项
 */
export interface AutoBindConfig {
  /** 是否启用自动绑定 */
  enabled: boolean
  /** 绑定模式 */
  mode: 'strict' | 'loose' | 'custom'
  /** 自定义绑定规则 */
  customRules?: BindingRule[]
  /** 排除的属性列表 */
  excludeProperties?: string[]
  /** 包含的属性列表（仅在strict模式下生效） */
  includeProperties?: string[]
}

/**
 * 数据源绑定配置中心。
 * 默认基础规则是保留的 v1-v2 运行时兼容契约；调用方可通过注册、移除、
 * 自定义和组件级 API 逐步迁移到新的配置结构。
 */
export class DataSourceBindingConfig {
  private bindingRules: Map<string, BindingRule> = new Map()
  private triggerRules: Map<string, TriggerRule> = new Map()

  constructor() {
    this.initializeDefaultRules()
  }

  /**
   * 注册可通过公开规则 API 替换或删除的 v1-v2 默认兼容表。
   */
  private initializeDefaultRules(): void {
    // 注册默认绑定规则
    this.registerBindingRule({
      propertyPath: 'base.deviceId',
      paramName: 'deviceId',
      required: true,
      description: '设备ID - 默认规则，可修改或删除'
    })

    this.registerBindingRule({
      propertyPath: 'base.metricsList',
      paramName: 'metrics',
      transform: (value: unknown) => (Array.isArray(value) ? value.join(',') : value),
      description: '指标列表 - 默认规则，可修改或删除'
    })

    this.registerBindingRule({
      propertyPath: 'component.startTime',
      paramName: 'startTime',
      transform: (value: unknown) => (value instanceof Date ? value.toISOString() : value),
      description: '开始时间 - 默认规则，可修改或删除'
    })

    this.registerBindingRule({
      propertyPath: 'component.endTime',
      paramName: 'endTime',
      transform: (value: unknown) => (value instanceof Date ? value.toISOString() : value),
      description: '结束时间 - 默认规则，可修改或删除'
    })

    this.registerBindingRule({
      propertyPath: 'component.dataType',
      paramName: 'dataType',
      description: '数据类型 - 默认规则，可修改或删除'
    })

    this.registerBindingRule({
      propertyPath: 'component.refreshInterval',
      paramName: 'refreshInterval',
      transform: (value: unknown) => parseInt(String(value)) || 30,
      description: '刷新间隔 - 默认规则，可修改或删除'
    })

    this.registerBindingRule({
      propertyPath: 'component.filterCondition',
      paramName: 'filter',
      description: '过滤条件 - 默认规则，可修改或删除'
    })

    // 注册默认触发规则
    this.registerTriggerRule({
      propertyPath: 'base.deviceId',
      enabled: true,
      debounceMs: 100,
      description: '设备ID触发 - 默认规则，可修改或删除'
    })

    this.registerTriggerRule({
      propertyPath: 'base.metricsList',
      enabled: true,
      debounceMs: 200,
      description: '指标列表触发 - 默认规则，可修改或删除'
    })

    this.registerTriggerRule({
      propertyPath: 'component.startTime',
      enabled: true,
      debounceMs: 300,
      description: '开始时间触发 - 默认规则，可修改或删除'
    })

    this.registerTriggerRule({
      propertyPath: 'component.endTime',
      enabled: true,
      debounceMs: 300,
      description: '结束时间触发 - 默认规则，可修改或删除'
    })

    this.registerTriggerRule({
      propertyPath: 'component.dataType',
      enabled: true,
      debounceMs: 150,
      description: '数据类型触发 - 默认规则，可修改或删除'
    })

    this.registerTriggerRule({
      propertyPath: 'component.refreshInterval',
      enabled: false,
      description: '刷新间隔触发 - 默认规则，可修改或删除'
    })

    this.registerTriggerRule({
      propertyPath: 'component.filterCondition',
      enabled: true,
      debounceMs: 250,
      description: '过滤条件触发 - 默认规则，可修改或删除'
    })
  }

  /** 注册或替换基础绑定规则。 */
  registerBindingRule(rule: BindingRule): void {
    this.bindingRules.set(rule.propertyPath, rule)
  }

  /** 注册或替换基础触发规则。 */
  registerTriggerRule(rule: TriggerRule): void {
    this.triggerRules.set(rule.propertyPath, rule)
  }

  /** 移除基础绑定规则。 */
  removeBindingRule(propertyPath: string): boolean {
    const removed = this.bindingRules.delete(propertyPath)
    return removed
  }

  /** 移除基础触发规则。 */
  removeTriggerRule(propertyPath: string): boolean {
    const removed = this.triggerRules.delete(propertyPath)
    return removed
  }

  /**
   * 清空基础绑定和触发规则；自定义规则及组件配置保持不变。
   */
  clearAllRules(): void {
    this.bindingRules.clear()
    this.triggerRules.clear()
  }

  // 组件特定配置
  private componentConfigs: Map<string, ComponentBindingConfig> = new Map()

  // 用户自定义规则
  private customBindingRules: BindingRule[] = []
  private customTriggerRules: TriggerRule[] = []

  /**
   * 按基础、自定义、组件附加的顺序返回绑定规则。
   * 同一路径查询使用首个匹配项，因此较早的规则优先。
   */
  getAllBindingRules(componentType?: string): BindingRule[] {
    const rules = Array.from(this.bindingRules.values())
    rules.push(...this.customBindingRules)

    if (componentType) {
      const componentConfig = this.componentConfigs.get(componentType)
      if (componentConfig?.additionalBindings) {
        rules.push(...componentConfig.additionalBindings)
      }
    }

    return rules
  }

  /**
   * 按基础、自定义、组件附加的顺序返回已启用的触发规则。
   * 同一路径查询使用首个匹配项，因此较早的规则优先。
   */
  getAllTriggerRules(componentType?: string): TriggerRule[] {
    const rules = Array.from(this.triggerRules.values())
    rules.push(...this.customTriggerRules)

    if (componentType) {
      const componentConfig = this.componentConfigs.get(componentType)
      if (componentConfig?.additionalTriggers) {
        rules.push(...componentConfig.additionalTriggers)
      }
    }

    return rules.filter((rule) => rule.enabled)
  }

  /**
   * 根据属性路径获取绑定规则
   */
  getBindingRule(propertyPath: string, componentType?: string): BindingRule | undefined {
    const allRules = this.getAllBindingRules(componentType)
    return allRules.find((rule) => rule.propertyPath === propertyPath)
  }

  /**
   * 根据属性路径获取触发规则
   */
  getTriggerRule(propertyPath: string, componentType?: string): TriggerRule | undefined {
    const allRules = this.getAllTriggerRules(componentType)
    return allRules.find((rule) => rule.propertyPath === propertyPath)
  }

  /**
   * 检查属性是否应该触发数据源执行
   */
  shouldTriggerDataSource(propertyPath: string, componentType?: string): boolean {
    const triggerRule = this.getTriggerRule(propertyPath, componentType)
    return triggerRule?.enabled === true
  }

  private buildParamsFromRules(componentConfig: unknown, bindingRules: BindingRule[]): Record<string, unknown> {
    const httpParams: Record<string, unknown> = {}

    for (const rule of bindingRules) {
      const propertyValue = this.readPropertyValue(componentConfig, rule.propertyPath)

      if (!propertyValue.exists) {
        continue
      }

      httpParams[rule.paramName] = this.transformRuleValue(rule, propertyValue.value)
    }

    return httpParams
  }

  private readPropertyValue(componentConfig: unknown, propertyPath: string): { exists: boolean; value?: unknown } {
    const pathParts = propertyPath.split('.').filter(Boolean)
    let currentValue: unknown = componentConfig

    for (const part of pathParts) {
      if (currentValue == null || !Object.prototype.hasOwnProperty.call(currentValue as object, part)) {
        return { exists: false }
      }

      currentValue = (currentValue as Record<string, unknown>)[part]
    }

    return { exists: true, value: currentValue }
  }

  private transformRuleValue(rule: BindingRule, value: unknown): unknown {
    if (!rule.transform || typeof rule.transform !== 'function') {
      return value
    }

    try {
      return rule.transform(value)
    } catch (error) {
      console.warn(`⚠️ [DataSourceBindingConfig] 参数转换失败:`, {
        propertyPath: rule.propertyPath,
        paramName: rule.paramName,
        originalValue: value,
        error: error instanceof Error ? error.message : error
      })
      return value
    }
  }

  /**
   * 构建HTTP参数对象
   */
  buildHttpParams(componentConfig: unknown, componentType?: string): Record<string, unknown> {
    const bindingRules = this.getAllBindingRules(componentType)
    return this.buildParamsFromRules(componentConfig, bindingRules)
  }

  /**
   * 🚀 新增：使用autoBind配置自动构建HTTP参数
   * @param componentConfig 组件配置
   * @param autoBindConfig 自动绑定配置
   * @param componentType 组件类型
   * @returns 自动绑定的HTTP参数
   */
  buildAutoBindParams(
    componentConfig: unknown,
    autoBindConfig: AutoBindConfig,
    componentType?: string
  ): Record<string, unknown> {
    if (!autoBindConfig.enabled) {
      return this.buildHttpParams(componentConfig, componentType)
    }

    switch (autoBindConfig.mode) {
      case 'strict':
        // 严格模式：仅绑定指定的属性
        return this.buildStrictModeParams(componentConfig, autoBindConfig, componentType)

      case 'loose':
        // 宽松模式：绑定所有可用属性，排除指定属性
        return this.buildLooseModeParams(componentConfig, autoBindConfig, componentType)

      case 'custom':
        // 自定义模式：使用自定义绑定规则
        return this.buildCustomModeParams(componentConfig, autoBindConfig, componentType)

      default:
        return this.buildHttpParams(componentConfig, componentType)
    }
  }

  /**
   * 构建严格模式参数
   */
  private buildStrictModeParams(
    componentConfig: unknown,
    autoBindConfig: AutoBindConfig,
    componentType?: string
  ): Record<string, unknown> {
    const includeProperties = autoBindConfig.includeProperties || []

    // 只处理指定的属性
    const bindingRules = this.getAllBindingRules(componentType).filter((rule) =>
      includeProperties.includes(rule.propertyPath)
    )

    return this.buildParamsFromRules(componentConfig, bindingRules)
  }

  /**
   * 构建宽松模式参数
   */
  private buildLooseModeParams(
    componentConfig: unknown,
    autoBindConfig: AutoBindConfig,
    componentType?: string
  ): Record<string, unknown> {
    const excludeProperties = autoBindConfig.excludeProperties || []

    // 处理所有属性，排除指定属性
    const bindingRules = this.getAllBindingRules(componentType).filter(
      (rule) => !excludeProperties.includes(rule.propertyPath)
    )

    return this.buildParamsFromRules(componentConfig, bindingRules)
  }

  /**
   * 构建自定义模式参数
   */
  private buildCustomModeParams(
    componentConfig: unknown,
    autoBindConfig: AutoBindConfig,
    componentType?: string
  ): Record<string, unknown> {
    const customRules = autoBindConfig.customRules || []
    return this.buildParamsFromRules(componentConfig, customRules)
  }

  /**
   * 添加自定义绑定规则
   */
  addCustomBindingRule(rule: BindingRule): void {
    // 检查是否已存在相同的属性路径
    const existingIndex = this.customBindingRules.findIndex((r) => r.propertyPath === rule.propertyPath)
    if (existingIndex >= 0) {
      this.customBindingRules[existingIndex] = rule
    } else {
      this.customBindingRules.push(rule)
    }
  }

  /**
   * 添加自定义触发规则
   */
  addCustomTriggerRule(rule: TriggerRule): void {
    // 检查是否已存在相同的属性路径
    const existingIndex = this.customTriggerRules.findIndex((r) => r.propertyPath === rule.propertyPath)
    if (existingIndex >= 0) {
      this.customTriggerRules[existingIndex] = rule
    } else {
      this.customTriggerRules.push(rule)
    }
  }

  /**
   * 设置组件特定配置
   */
  setComponentConfig(componentType: string, config: ComponentBindingConfig): void {
    this.componentConfigs.set(componentType, config)
  }

  /**
   * 获取组件特定配置
   */
  getComponentConfig(componentType: string): ComponentBindingConfig | undefined {
    return this.componentConfigs.get(componentType)
  }

  /**
   * 移除自定义规则
   */
  removeCustomBindingRule(propertyPath: string): boolean {
    const index = this.customBindingRules.findIndex((r) => r.propertyPath === propertyPath)
    if (index >= 0) {
      this.customBindingRules.splice(index, 1)
      return true
    }
    return false
  }

  /**
   * 移除自定义触发规则
   */
  removeCustomTriggerRule(propertyPath: string): boolean {
    const index = this.customTriggerRules.findIndex((r) => r.propertyPath === propertyPath)
    if (index >= 0) {
      this.customTriggerRules.splice(index, 1)
      return true
    }
    return false
  }

  /**
   * 获取调试信息
   */
  getDebugInfo(componentType?: string) {
    const currentBindingRules = this.getAllBindingRules(componentType)
    const currentTriggerRules = this.getAllTriggerRules(componentType)

    return {
      baseBindingRules: this.bindingRules.size,
      baseTriggerRules: this.triggerRules.size,
      registeredBindingRules: this.bindingRules.size,
      registeredTriggerRules: this.triggerRules.size,
      customBindingRules: this.customBindingRules.length,
      customTriggerRules: this.customTriggerRules.length,
      effectiveBindingRules: currentBindingRules.length,
      effectiveTriggerRules: currentTriggerRules.length,
      componentConfigs: Array.from(this.componentConfigs.keys()),
      currentBindingRules: currentBindingRules.map((r) => ({
        propertyPath: r.propertyPath,
        paramName: r.paramName,
        required: r.required
      })),
      currentTriggerRules: currentTriggerRules.map((r) => ({
        propertyPath: r.propertyPath,
        enabled: r.enabled,
        debounceMs: r.debounceMs
      }))
    }
  }
}

// 共享运行时实例；模块导入本身不修改宿主全局作用域。
export const dataSourceBindingConfig = new DataSourceBindingConfig()

/**
 * 显式安装调试入口，并返回可逆清理函数。
 * 清理时不会覆盖宿主在安装后写入的新值。
 */
export function installDataSourceBindingConfigDebugGlobal(
  target: Record<string, unknown> = globalThis as Record<string, unknown>
): () => void {
  const key = '__dataSourceBindingConfig'
  const hadOwnValue = Object.prototype.hasOwnProperty.call(target, key)
  const previousValue = target[key]
  target[key] = dataSourceBindingConfig

  return () => {
    if (target[key] !== dataSourceBindingConfig) {
      return
    }

    if (hadOwnValue) {
      target[key] = previousValue
    } else {
      delete target[key]
    }
  }
}
