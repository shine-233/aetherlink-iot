/**
 * 文件用途: Visual Editor 统一数据管理模块入口，集中导出 store、服务、数据流和 Card 2.1 适配能力。
 * 核心逻辑: 组合 unified-editor、configuration-service、data-flow-manager 和 card2-adapter，提供系统级初始化门面。
 * 关键注意事项: 导入导出顺序会影响单例服务复位和测试隔离，初始化流程需要先准备服务再注册监听。
 * 重构建议: 将门面初始化步骤显式拆成可注入依赖，并补充重复初始化、reset 和迁移兼容测试。
 */

// 导入核心状态管理
import {
  useUnifiedEditorStore,
  type UnifiedEditorState,
  type BaseConfiguration,
  type ComponentConfiguration,
  type DataSourceConfiguration,
  type InteractionConfiguration
} from './unified-editor'

// 重新导出给外部使用
export {
  useUnifiedEditorStore,
  type UnifiedEditorState,
  type BaseConfiguration,
  type ComponentConfiguration,
  type DataSourceConfiguration,
  type InteractionConfiguration
}

// 导入配置服务
import {
  useConfigurationService,
  resetConfigurationService,
  type ConfigurationChangeEvent,
  type ConfigurationValidationResult,
  type ConfigurationMigration
} from './configuration-service'

// 导入数据流管理
import {
  useDataFlowManager,
  resetDataFlowManager,
  createAddNodeAction,
  createUpdateConfigAction,
  createSetRuntimeDataAction,
  type UserAction,
  type ActionType,
  type SideEffectHandler,
  type DataFlowContext
} from './data-flow-manager'

// 导入Card 2.1 集成适配器
import {
  useCard2Adapter,
  resetCard2Adapter,
  type ComponentDefinition,
  type DataSourceDefinition,
  type FieldMapping,
  type ComponentConfig,
  type ReactiveDataBinding,
  type ComponentRequirement
} from './card2-adapter'

// 重新导出配置服务
export {
  useConfigurationService,
  resetConfigurationService,
  type ConfigurationChangeEvent,
  type ConfigurationValidationResult,
  type ConfigurationMigration
}

// 重新导出数据流管理
export {
  useDataFlowManager,
  resetDataFlowManager,
  createAddNodeAction,
  createUpdateConfigAction,
  createSetRuntimeDataAction,
  type UserAction,
  type ActionType,
  type SideEffectHandler,
  type DataFlowContext
}

// 重新导出Card 2.1 集成适配器
export {
  useCard2Adapter,
  resetCard2Adapter,
  type ComponentDefinition,
  type DataSourceDefinition,
  type FieldMapping,
  type ComponentConfig,
  type ReactiveDataBinding,
  type ComponentRequirement
}

/**
 * 统一的 Visual Editor 系统类
 * 🔥 这是新架构的核心协调器，替代原有的分散管理
 */
export class UnifiedVisualEditorSystem {
  public store: ReturnType<typeof useUnifiedEditorStore> | null = null
  public configService: ReturnType<typeof useConfigurationService> | null = null
  public dataFlowManager: ReturnType<typeof useDataFlowManager> | null = null
  public card2Adapter: ReturnType<typeof useCard2Adapter> | null = null

  private initialized = false
  private servicesInitialized = false

  constructor() {}

  /**
   * 延迟初始化各个服务
   */
  private initializeServices(): void {
    if (this.servicesInitialized) return

    this.store = useUnifiedEditorStore()
    this.configService = useConfigurationService()
    this.dataFlowManager = useDataFlowManager()
    this.card2Adapter = useCard2Adapter()

    this.servicesInitialized = true
  }

  /**
   * 初始化系统
   */
  async initialize(): Promise<void> {
    if (this.initialized && this.store && this.configService && this.dataFlowManager && this.card2Adapter) {
      return
    }

    if (this.initialized && !this.store) {
      this.initialized = false
    }
    // 0. 先初始化各个服务
    this.initializeServices()

    // 1. 初始化配置服务
    await this.initializeConfigurationService()

    // 2. 初始化数据流管理
    this.initializeDataFlowManager()

    // 3. 初始化Card2.1适配器
    await this.initializeCard2Adapter()

    // 4. 设置系统事件监听
    this.setupSystemEventListeners()

    // 5. 验证所有服务都已正确初始化
    if (!this.store || !this.configService || !this.dataFlowManager || !this.card2Adapter) {
      throw new Error('服务初始化验证失败：某些服务为null')
    }

    this.initialized = true
  }

  /**
   * 初始化配置服务
   */
  private async initializeConfigurationService(): Promise<void> {
    if (!this.configService) {
      throw new Error('配置服务未初始化')
    }

    // 注册配置迁移
    this.configService.registerMigration({
      fromVersion: '1.0.0',
      toVersion: '1.1.0',
      migrate: config => {
        // 示例迁移逻辑
        return {
          ...config,
          metadata: {
            ...config.metadata,
            version: '1.1.0'
          }
        }
      }
    })
  }

  /**
   * 初始化数据流管理
   */
  private initializeDataFlowManager(): void {
    if (!this.dataFlowManager) {
      throw new Error('数据流管理器未初始化')
    }

    // 注册自定义副作用处理器
    this.dataFlowManager.registerSideEffect({
      name: 'SystemStateSync',
      condition: () => true, // 监听所有操作
      execute: action => {
        // 系统状态同步逻辑
      }
    })
  }

  /**
   * 初始化Card2.1适配器
   */
  private async initializeCard2Adapter(): Promise<void> {
    // Card2.1适配器会自动初始化
    // 这里可以添加额外的初始化逻辑
  }

  /**
   * 设置系统事件监听
   */
  private setupSystemEventListeners(): void {
    if (!this.configService || !this.dataFlowManager) {
      throw new Error('服务未初始化，无法设置事件监听')
    }

    // 监听配置变更
    this.configService.onConfigurationChange(event => {})

    // 监听数据流更新
    this.dataFlowManager.onDataFlowUpdate(action => {})

    // 监听错误事件
    this.dataFlowManager.onError((action, error) => {})
  }

  /**
   * 获取系统状态
   */
  getSystemStatus(): {
    initialized: boolean
    nodeCount: number
    widgetCount: number
    card2ComponentCount: number
    hasUnsavedChanges: boolean
  } {
    if (!this.store) {
      return {
        initialized: false,
        nodeCount: 0,
        widgetCount: 0,
        card2ComponentCount: 0,
        hasUnsavedChanges: false
      }
    }

    return {
      initialized: this.initialized,
      nodeCount: this.store.nodes.length,
      widgetCount: this.store.allWidgets.length,
      card2ComponentCount: this.store.card2ComponentCount,
      hasUnsavedChanges: this.store.hasUnsavedChanges
    }
  }

  /**
   * 保存所有配置
   */
  async saveAll(): Promise<void> {
    if (!this.configService) {
      throw new Error('配置服务未初始化')
    }

    await this.configService.saveAllConfigurations()
  }

  /**
   * 清理系统资源
   */
  cleanup(): void {
    if (this.store) {
      this.store.clearAll()
    }

    this.initialized = false
    this.servicesInitialized = false
  }
}

// ==================== 单例模式 ====================

let unifiedEditorSystemInstance: UnifiedVisualEditorSystem | null = null

/**
 * 获取统一Visual Editor系统实例（单例）
 * 🔥 这是新架构的主要入口点
 */
export function useUnifiedVisualEditorSystem(): UnifiedVisualEditorSystem {
  if (!unifiedEditorSystemInstance) {
    unifiedEditorSystemInstance = new UnifiedVisualEditorSystem()
  }

  return unifiedEditorSystemInstance
}

/**
 * 重置统一Visual Editor系统实例（测试用）
 */
export function resetUnifiedVisualEditorSystem(): void {
  if (unifiedEditorSystemInstance) {
    unifiedEditorSystemInstance.cleanup()
  }
  unifiedEditorSystemInstance = null
}

// ==================== 便捷 Hook ====================

/**
 * Visual Editor Hook
 * 🔥 提供简化的API给组件使用
 */
export function useVisualEditor() {
  const system = useUnifiedVisualEditorSystem()

  return {
    // 状态访问 - 🔥 使用 computed 确保总是返回最新的服务实例
    get store() {
      return system.store
    },
    get configService() {
      return system.configService
    },
    get dataFlowManager() {
      return system.dataFlowManager
    },
    get card2Adapter() {
      return system.card2Adapter
    },

    // 系统操作
    initialize: () => system.initialize(),
    saveAll: () => system.saveAll(),
    getStatus: () => system.getSystemStatus(),
    cleanup: () => system.cleanup(),

    // 快捷操作
    addNode: async (node: any) => {
      if (!system.dataFlowManager) {
        throw new Error('数据流管理器未初始化，请先调用 initialize()')
      }
      return system.dataFlowManager.handleUserAction({
        type: 'ADD_NODE',
        data: node
      })
    },

    updateNode: async (nodeId: string, updates: any) => {
      if (!system.dataFlowManager) {
        throw new Error('数据流管理器未初始化，请先调用 initialize()')
      }
      return system.dataFlowManager.handleUserAction({
        type: 'UPDATE_NODE',
        targetId: nodeId,
        data: updates
      })
    },

    removeNode: async (nodeId: string) => {
      if (!system.dataFlowManager) {
        throw new Error('数据流管理器未初始化，请先调用 initialize()')
      }
      return system.dataFlowManager.handleUserAction({
        type: 'REMOVE_NODE',
        targetId: nodeId
      })
    },

    updateConfiguration: async (widgetId: string, section: any, config: any) => {
      if (!system.dataFlowManager) {
        throw new Error('数据流管理器未初始化，请先调用 initialize()')
      }
      return system.dataFlowManager.handleUserAction({
        type: 'UPDATE_CONFIGURATION',
        targetId: widgetId,
        data: { section, config }
      })
    },

    selectNodes: async (ids: string[]) => {
      if (!system.dataFlowManager) {
        throw new Error('数据流管理器未初始化，请先调用 initialize()')
      }
      return system.dataFlowManager.handleUserAction({
        type: 'SELECT_NODES',
        data: ids
      })
    },

    // 状态查询
    getSelectedNodes: () => {
      if (!system.store) {
        return []
      }
      return system.store.selectedNodes
    },
    getConfiguration: (widgetId: string) => {
      if (!system.configService) {
        throw new Error('配置服务未初始化，请先调用 initialize()')
      }
      return system.configService.getConfiguration(widgetId)
    },
    getRuntimeData: (widgetId: string) => {
      if (!system.configService) {
        throw new Error('配置服务未初始化，请先调用 initialize()')
      }
      return system.configService.getRuntimeData(widgetId)
    }
  }
}

/**
 * 迁移辅助工具
 * 帮助从旧系统迁移到新系统
 *
 * Boundary: old_editor_data is a compatibility persisted Visual Editor key. This
 * helper is a manual migration entrypoint only; callers must initialize the
 * unified editor system before passing the parsed payload here.
 */
export const MigrationHelper = {
  /**
   * 从旧的编辑器存储迁移数据
   */
  migrateFromOldStore(oldStoreData: any): void {
    const system = useUnifiedVisualEditorSystem()

    // 迁移目标依赖系统已完成 initialize；未初始化（测试/极端时序）时无可迁移面。
    if (!system.store || !system.configService) {
      return
    }

    // 迁移节点数据
    if (oldStoreData.nodes) {
      oldStoreData.nodes.forEach((node: any) => {
        system.store!.addNode(node)
      })
    }

    // 迁移选中状态
    if (oldStoreData.selectedIds) {
      system.store!.selectNodes(oldStoreData.selectedIds)
    }

    // 迁移配置数据
    if (oldStoreData.configurations) {
      Object.entries(oldStoreData.configurations).forEach(([widgetId, config]: [string, any]) => {
        system.configService!.setConfiguration(widgetId, config)
      })
    }
  },

  /**
   * 检查是否需要迁移
   */
  needsMigration(): boolean {
    // 检查是否存在旧的存储数据
    return localStorage.getItem('old_editor_data') !== null
  }
}
