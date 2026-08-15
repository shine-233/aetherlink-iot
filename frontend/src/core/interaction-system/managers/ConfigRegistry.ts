/**
 * 文件用途：管理交互系统中组件自定义配置视图的注册、查询和注销。
 * 核心逻辑：使用 Map 维护组件 ID 到配置组件的映射，并提供单例注册表给外部复用。
 * 关键注意事项：注册键必须与组件 ID 保持一致，避免覆盖其他组件的配置入口。
 * 重构建议：后续可补充注册来源、重复注册告警和批量导入导出能力。
 */

export interface IConfigComponent {
  id?: string
  name?: string
  component?: any
  schema?: Record<string, any>
  [key: string]: any
}

interface ConfigComponentRegistration {
  componentId: string
  configComponent: IConfigComponent
}

class ConfigRegistry {
  private registry = new Map<string, IConfigComponent>()

  /**
   * 注册配置组件
   */
  register(componentId: string, configComponent: IConfigComponent) {
    this.registry.set(componentId, configComponent)
  }

  /**
   * 获取配置组件
   */
  get(componentId: string): IConfigComponent | undefined {
    return this.registry.get(componentId)
  }

  /**
   * 检查是否有自定义配置组件
   */
  has(componentId: string): boolean {
    return this.registry.has(componentId)
  }

  /**
   * 获取所有注册的配置组件
   */
  getAll(): ConfigComponentRegistration[] {
    return Array.from(this.registry.entries()).map(([componentId, configComponent]) => ({
      componentId,
      configComponent
    }))
  }

  /**
   * 清除所有注册
   */
  clear() {
    this.registry.clear()
  }

  /**
   * 移除指定组件的配置
   */
  unregister(componentId: string) {
    return this.registry.delete(componentId)
  }
}

// 导出单例
export const configRegistry = new ConfigRegistry()

export default configRegistry
