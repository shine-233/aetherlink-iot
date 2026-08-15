/**
 * 文件用途: visual-editor 运行时配置集成桥。
 * 核心逻辑: 按组件 ID 初始化、读取、设置和局部更新 WidgetConfiguration，供 data-architecture 运行时读取。
 * 关键注意事项: 该桥仅保存当前页面进程内状态；写入成功不代表 dashboard 或 localStorage 已持久化。
 * 重构建议: dashboard 装载链路稳定后，将其收敛为统一配置 store 的兼容适配器。
 */
import type { ConfigurationUpdateResult, WidgetConfiguration } from './types'

interface RuntimeConfigurationUpdateResult extends ConfigurationUpdateResult {
  persisted: false
  scope: 'runtime'
}

class ConfigurationIntegrationBridge {
  private readonly configurations = new Map<string, WidgetConfiguration>()

  async initialize() {
    return true
  }

  initializeConfiguration(componentId: string, initialConfig: WidgetConfiguration = {}): WidgetConfiguration {
    const existing = this.configurations.get(componentId)
    if (existing) {
      return existing
    }

    const initialized = this.createDefaultConfiguration(initialConfig)
    this.configurations.set(componentId, initialized)
    return initialized
  }

  getConfiguration(componentId: string): WidgetConfiguration | null {
    return this.configurations.get(componentId) || null
  }

  setConfiguration(componentId: string, config: WidgetConfiguration): RuntimeConfigurationUpdateResult {
    this.configurations.set(componentId, this.createDefaultConfiguration(config))
    return this.createRuntimeUpdateResult()
  }

  updateConfiguration(
    componentId: string,
    sectionOrConfig: string | WidgetConfiguration,
    value?: Record<string, any>
  ): RuntimeConfigurationUpdateResult {
    const current = this.getConfiguration(componentId) || this.createDefaultConfiguration()
    if (typeof sectionOrConfig === 'string') {
      this.configurations.set(componentId, {
        ...current,
        [sectionOrConfig]: {
          ...(current[sectionOrConfig] || {}),
          ...(value || {})
        }
      })
    } else {
      this.configurations.set(componentId, this.createDefaultConfiguration({ ...current, ...sectionOrConfig }))
    }
    return this.createRuntimeUpdateResult()
  }

  private createRuntimeUpdateResult(): RuntimeConfigurationUpdateResult {
    return { success: true, persisted: false, scope: 'runtime' }
  }

  private createDefaultConfiguration(config: WidgetConfiguration = {}): WidgetConfiguration {
    return {
      base: {},
      component: {},
      dataSource: null,
      interaction: {},
      metadata: {},
      customize: {},
      ...config
    }
  }
}

export const configurationIntegrationBridge = new ConfigurationIntegrationBridge()
export type { ConfigurationIntegrationBridge }
