/**
 * 文件用途: Visual Editor 数据流管理器，统一处理用户操作到状态更新、配置同步和视图刷新的链路。
 * 核心逻辑: 验证 UserAction 后更新 unified-editor 状态，触发配置服务、运行时数据和副作用处理器。
 * 关键注意事项: 操作顺序会影响错误恢复和视图刷新，批量操作需要避免中间状态泄漏给 UI。
 * 重构建议: 将 action 校验、状态 reducer 和副作用分发拆成独立 helper，并补充失败恢复与批量事务测试。
 */

import { useUnifiedEditorStore } from '@/store/modules/visual-editor/unified-editor'
import { useConfigurationService } from '@/store/modules/visual-editor/configuration-service'
import type { GraphData, WidgetConfiguration } from '@/store/modules/visual-editor/unified-editor'
import { createStateUpdateHandler } from '@/store/modules/visual-editor/data-flow-state-handlers'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DataFlowManager')

/**
 * 用户操作类型定义
 */
export interface UserAction {
  type: ActionType
  targetId?: string
  data?: any
  metadata?: Record<string, any>
}

export type ActionType =
  | 'ADD_NODE'
  | 'UPDATE_NODE'
  | 'REMOVE_NODE'
  | 'SELECT_NODES'
  | 'UPDATE_CONFIGURATION'
  | 'SET_RUNTIME_DATA'
  | 'BATCH_UPDATE'

/**
 * 操作验证结果
 */
export interface ActionValidationResult {
  valid: boolean
  error?: string
  warnings?: string[]
}

/**
 * 副作用处理器接口
 */
export interface SideEffectHandler {
  name: string
  condition: (action: UserAction, context?: DataFlowContext) => boolean
  execute: (action: UserAction, context: DataFlowContext) => Promise<void> | void
}

/**
 * 数据流上下文
 */
export interface DataFlowContext {
  store: ReturnType<typeof useUnifiedEditorStore>
  configService: ReturnType<typeof useConfigurationService>
  action: UserAction
  timestamp: Date
}

interface ActionProcessingContext {
  action: UserAction
  previousRuntimeData: any
}

/**
 * 数据流管理器
 * 🔥 统一的数据流控制中心，解决数据流混乱问题
 */
export class DataFlowManager {
  private store = useUnifiedEditorStore()
  private configService = useConfigurationService()
  private eventBus = new EventTarget()
  private sideEffectHandlers: SideEffectHandler[] = []
  private isProcessing = false

  constructor() {
    this.registerDefaultSideEffects()
  }

  // ==================== 核心数据流处理 ====================

  /**
   * 处理用户操作
   * 🔥 所有用户操作的统一入口
   */
  async handleUserAction(action: UserAction): Promise<void> {
    if (this.isProcessing) {
      return
    }

    const processingContext = this.startActionProcessing(action)

    try {
      await this.processActionPipeline(action)
    } catch (error) {
      // 触发错误恢复
      await this.handleActionProcessingError(processingContext, error as Error)

      throw error
    } finally {
      this.finishActionProcessing()
    }
  }

  /**
   * 批量处理用户操作
   */
  async handleBatchActions(actions: UserAction[]): Promise<void> {
    // 批量操作使用事务模式
    this.store.setLoading(true)

    try {
      for (const action of actions) {
        await this.handleUserAction(action)
      }
    } finally {
      this.store.setLoading(false)
    }
  }

  // ==================== 状态更新逻辑 ====================

  /**
   * 根据操作类型更新状态
   */
  private async updateState(action: UserAction): Promise<void> {
    const handler = createStateUpdateHandler(action.type, {
      store: this.store,
      configService: this.configService
    })

    if (!handler) {
      return
    }

    await handler(action)
  }

  private startActionProcessing(action: UserAction): ActionProcessingContext {
    this.isProcessing = true

    return {
      action,
      previousRuntimeData: action.targetId ? this.configService.getRuntimeData(action.targetId) : undefined
    }
  }

  private finishActionProcessing(): void {
    this.isProcessing = false
  }

  private async processActionPipeline(action: UserAction): Promise<void> {
    this.validateActionOrThrow(action)
    await this.updateState(action)
    await this.triggerSideEffects(action)
    this.notifyViewUpdate(action)
  }

  private validateActionOrThrow(action: UserAction): void {
    const validationResult = this.validateAction(action)
    if (!validationResult.valid) {
      throw new Error(validationResult.error)
    }
  }

  private async handleActionProcessingError(context: ActionProcessingContext, error: Error): Promise<void> {
    await this.handleError(context.action, error, context.previousRuntimeData)
  }

  // ==================== 操作验证 ====================

  /**
   * 验证用户操作
   */
  private validateAction(action: UserAction): ActionValidationResult {
    // 基础验证
    if (!action.type) {
      return { valid: false, error: '操作类型不能为空' }
    }

    // 类型特定验证
    switch (action.type) {
      case 'ADD_NODE':
        return this.validateAddNodeAction(action)

      case 'UPDATE_NODE':
      case 'REMOVE_NODE':
        return this.validateNodeTargetAction(action)

      case 'UPDATE_CONFIGURATION':
        return this.validateConfigurationAction(action)

      case 'SET_RUNTIME_DATA':
        return this.validateRuntimeDataAction(action)

      default:
        return { valid: true }
    }
  }

  /**
   * 验证添加节点操作
   */
  private validateAddNodeAction(action: UserAction): ActionValidationResult {
    if (!action.data) {
      return { valid: false, error: '添加节点操作需要节点数据' }
    }

    const node = action.data as GraphData
    if (!node.id) {
      return { valid: false, error: '节点必须有ID' }
    }

    // 检查ID是否已存在
    const existingNode = this.store.nodes.find((n) => n.id === node.id)
    if (existingNode) {
      return { valid: false, error: `节点ID已存在: ${node.id}` }
    }

    return { valid: true }
  }

  /**
   * 验证需要目标ID的节点操作
   */
  private validateNodeTargetAction(action: UserAction): ActionValidationResult {
    if (!action.targetId) {
      return { valid: false, error: '操作需要targetId' }
    }

    // 检查节点是否存在
    const node = this.store.nodes.find((n) => n.id === action.targetId)
    if (!node) {
      return { valid: false, error: `节点不存在: ${action.targetId}` }
    }

    return { valid: true }
  }

  /**
   * 验证配置操作
   */
  private validateConfigurationAction(action: UserAction): ActionValidationResult {
    if (!action.targetId) {
      return { valid: false, error: '配置操作需要targetId' }
    }

    if (!action.data || !action.data.section) {
      return { valid: false, error: '配置操作需要section参数' }
    }

    const validSections = ['base', 'component', 'dataSource', 'interaction']
    if (!validSections.includes(action.data.section)) {
      return { valid: false, error: `无效的配置section: ${action.data.section}` }
    }

    return { valid: true }
  }

  /**
   * 验证运行时数据操作
   */
  private validateRuntimeDataAction(action: UserAction): ActionValidationResult {
    if (!action.targetId) {
      return { valid: false, error: '运行时数据操作需要targetId' }
    }

    return { valid: true }
  }

  // ==================== 副作用处理 ====================

  /**
   * 触发副作用处理
   */
  private async triggerSideEffects(action: UserAction): Promise<void> {
    const context: DataFlowContext = {
      store: this.store,
      configService: this.configService,
      action,
      timestamp: new Date()
    }

    // 并行执行所有匹配的副作用处理器
    const matchingHandlers = this.sideEffectHandlers.filter((handler) => handler.condition(action, context))

    await Promise.all(
      matchingHandlers.map(async (handler) => {
        try {
          await handler.execute(action, context)
        } catch (error) {
          // 记录副作用处理器异常，避免单个处理器失败影响其他处理器
          logger.error(`[DataFlowManager] 副作用处理器执行失败: ${handler.name}`, {
            actionType: action.type,
            targetId: action.targetId,
            error: error instanceof Error ? error.message : error
          })
        }
      })
    )
  }

  /**
   * 注册副作用处理器
   */
  registerSideEffect(handler: SideEffectHandler): void {
    this.sideEffectHandlers.push(handler)
  }

  /**
   * 注册默认的副作用处理器
   */
  private registerDefaultSideEffects(): void {
    // 配置自动保存
    this.registerSideEffect({
      name: 'AutoSaveConfiguration',
      condition: (action) => action.type === 'UPDATE_CONFIGURATION',
      execute: async (action, context) => {
        if (action.targetId) {
          await context.configService.saveConfiguration(action.targetId)
        }
      }
    })

    // 数据源变更处理
    this.registerSideEffect({
      name: 'DataSourceChangeHandler',
      condition: (action) => action.type === 'UPDATE_CONFIGURATION' && action.data?.section === 'dataSource',
      execute: async (_action, _context) => {
        // 数据源刷新由 ConfigurationService.updateConfigurationSection 统一触发。
      }
    })

    // Card2.1组件特殊处理
    this.registerSideEffect({
      name: 'Card2ComponentHandler',
      condition: (action, context) => {
        if (!action.targetId || !context?.store) return false
        return context.store.card2Components.has(action.targetId)
      },
      execute: async (action, context) => {
        // Card2.1特殊的数据绑定处理
        if (action.type === 'UPDATE_CONFIGURATION' && action.data?.section === 'dataSource') {
          context.store.updateDataBinding(action.targetId!)
        }
      }
    })
  }

  // ==================== 视图更新通知 ====================

  /**
   * 通知视图更新
   */
  private notifyViewUpdate(action: UserAction): void {
    const event = new CustomEvent('data-flow-update', {
      detail: {
        action,
        timestamp: new Date()
      }
    })

    this.eventBus.dispatchEvent(event)
  }

  /**
   * 监听数据流更新事件
   */
  onDataFlowUpdate(callback: (action: UserAction) => void): () => void {
    const handler = (event: CustomEvent) => {
      callback(event.detail.action)
    }

    this.eventBus.addEventListener('data-flow-update', handler as EventListener)

    return () => {
      this.eventBus.removeEventListener('data-flow-update', handler as EventListener)
    }
  }

  // ==================== 错误处理 ====================

  /**
   * 处理错误和恢复
   */
  private async handleError(action: UserAction, error: Error, previousRuntimeData?: any): Promise<void> {
    // 触发错误事件
    const errorEvent = new CustomEvent('data-flow-error', {
      detail: {
        action,
        error,
        timestamp: new Date()
      }
    })

    this.eventBus.dispatchEvent(errorEvent)

    if (action.targetId && previousRuntimeData !== undefined) {
      this.configService.setRuntimeData(action.targetId, previousRuntimeData)
      return
    }

    if (action.targetId && action.type === 'UPDATE_CONFIGURATION' && action.data?.section === 'dataSource') {
      this.configService.setRuntimeData(action.targetId, {
        __error: true,
        message: error.message,
        actionType: action.type,
        timestamp: Date.now()
      })
    }
  }

  /**
   * 监听错误事件
   */
  onError(callback: (action: UserAction, error: Error) => void): () => void {
    const handler = (event: CustomEvent) => {
      callback(event.detail.action, event.detail.error)
    }

    this.eventBus.addEventListener('data-flow-error', handler as EventListener)

    return () => {
      this.eventBus.removeEventListener('data-flow-error', handler as EventListener)
    }
  }
}

// ==================== 单例模式 ====================

let dataFlowManagerInstance: DataFlowManager | null = null

/**
 * 获取数据流管理器实例（单例）
 */
export function useDataFlowManager(): DataFlowManager {
  if (!dataFlowManagerInstance) {
    dataFlowManagerInstance = new DataFlowManager()
  }

  return dataFlowManagerInstance
}

/**
 * 重置数据流管理器实例（测试用）
 */
export function resetDataFlowManager(): void {
  dataFlowManagerInstance = null
}

// ==================== 便捷操作函数 ====================

/**
 * 创建添加节点操作
 */
export function createAddNodeAction(node: GraphData): UserAction {
  return {
    type: 'ADD_NODE',
    data: node
  }
}

/**
 * 创建更新配置操作
 */
export function createUpdateConfigAction(
  widgetId: string,
  section: keyof WidgetConfiguration,
  config: any
): UserAction {
  return {
    type: 'UPDATE_CONFIGURATION',
    targetId: widgetId,
    data: { section, config }
  }
}

/**
 * 创建设置运行时数据操作
 */
export function createSetRuntimeDataAction(widgetId: string, data: any): UserAction {
  return {
    type: 'SET_RUNTIME_DATA',
    targetId: widgetId,
    data
  }
}
