/**
 * 文件用途: Visual Editor 数据流状态更新处理器。
 * 核心逻辑: 将 UserAction 分发到 store/configService 的具体状态更新。
 * 关键注意事项: 这里只处理状态写入，不触发 DataFlowManager 的校验、错误恢复或副作用流水线。
 */

import type { GraphData, WidgetConfiguration } from '@/store/modules/visual-editor/unified-editor'
import type { useUnifiedEditorStore } from '@/store/modules/visual-editor/unified-editor'
import type { useConfigurationService } from '@/store/modules/visual-editor/configuration-service'
import type { ActionType, UserAction } from '@/store/modules/visual-editor/data-flow-manager'
import { createLogger } from '@/utils/logger'

const logger = createLogger('DataFlowStateHandlers')

type UnifiedEditorStore = ReturnType<typeof useUnifiedEditorStore>
type ConfigurationService = ReturnType<typeof useConfigurationService>

interface StateHandlerContext {
  store: UnifiedEditorStore
  configService: ConfigurationService
}

export type StateUpdateHandler = (action: UserAction) => Promise<void> | void

export function createStateUpdateHandler(
  actionType: ActionType,
  context: StateHandlerContext
): StateUpdateHandler | undefined {
  const handlers: Partial<Record<ActionType, StateUpdateHandler>> = {
    ADD_NODE: (action) => handleAddNode(action, context),
    UPDATE_NODE: (action) => handleUpdateNode(action, context),
    REMOVE_NODE: (action) => handleRemoveNode(action, context),
    SELECT_NODES: (action) => handleSelectNodes(action, context),
    UPDATE_CONFIGURATION: (action) => handleUpdateConfiguration(action, context),
    SET_RUNTIME_DATA: (action) => handleSetRuntimeData(action, context),
    BATCH_UPDATE: (action) => handleBatchUpdate(action, context)
  }

  return handlers[actionType]
}

function handleAddNode(action: UserAction, { store }: StateHandlerContext): void {
  const node = action.data as GraphData
  store.addNode(node)
}

async function handleUpdateNode(action: UserAction, context: StateHandlerContext): Promise<void> {
  if (!action.targetId) {
    throw new Error('更新节点操作需要targetId')
  }

  context.store.updateNode(action.targetId, action.data)

  if (action.data && action.data.properties) {
    try {
      const updatedNode = context.store.nodes.find((node) => node.id === action.targetId)
      if (updatedNode) {
        await syncNodePropertiesToConfiguration(action.targetId, updatedNode.properties, context)
      }
    } catch (error) {
      logger.error(`[DataFlowStateHandlers] 配置系统同步失败`, {
        componentId: action.targetId,
        error: error instanceof Error ? error.message : error
      })
    }
  }
}

function handleRemoveNode(action: UserAction, { store }: StateHandlerContext): void {
  if (!action.targetId) {
    throw new Error('删除节点操作需要targetId')
  }

  store.removeNode(action.targetId)
}

function handleSelectNodes(action: UserAction, { store }: StateHandlerContext): void {
  const nodeIds = action.data as string[]
  store.selectNodes(nodeIds)
}

function handleUpdateConfiguration(action: UserAction, { configService }: StateHandlerContext): void {
  if (!action.targetId) {
    throw new Error('更新配置操作需要targetId')
  }

  const { section, config } = action.data as {
    section: keyof WidgetConfiguration
    config: any
  }

  configService.updateConfigurationSection(action.targetId, section, config)
}

function handleSetRuntimeData(action: UserAction, { configService }: StateHandlerContext): void {
  if (!action.targetId) {
    throw new Error('设置运行时数据操作需要targetId')
  }

  configService.setRuntimeData(action.targetId, action.data)
}

function handleBatchUpdate(action: UserAction, { configService }: StateHandlerContext): void {
  const updates = action.data as Array<{
    widgetId: string
    section: keyof WidgetConfiguration
    data: any
  }>

  configService.batchUpdateConfiguration(updates)
}

async function syncNodePropertiesToConfiguration(
  componentId: string,
  properties: Record<string, any>,
  { configService }: StateHandlerContext
): Promise<void> {
  try {
    const currentConfig = configService.getConfiguration(componentId)

    if (!currentConfig) {
      configService.initializeConfiguration(componentId)
    }

    configService.updateConfigurationSection(componentId, 'component', {
      ...properties
    })
  } catch (error) {
    logger.error(`[DataFlowStateHandlers] syncNodePropertiesToConfiguration 失败`, {
      componentId,
      error: error instanceof Error ? error.message : error
    })
    throw error
  }
}
