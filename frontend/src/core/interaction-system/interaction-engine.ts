/**
 * 文件用途：执行交互动作并处理跨组件属性更新。
 * 核心逻辑：根据动作类型解析目标组件、写入配置或触发导航，并通过消息与日志反馈结果。
 * 关键注意事项：DOM 上暴露的 Vue 实例能力有限，访问 exposed 方法前必须保留空值保护。
 * 重构建议：可将动作处理器拆成策略表，减少新增动作时对主流程的修改。
 */

import { useEditorStore } from '@/store/modules/editor'
import { createLogger } from '@/utils/logger'
import { useMessage } from 'naive-ui'

const logger = createLogger('InteractionEngine')

/** Minimal Vue component internals exposed on rendered component DOM nodes. */
interface VueComponentElement extends Element {
  __vueParentComponent?: {
    exposed?: {
      updateConfig?: (section: string, config: Record<string, unknown>) => void
      watchProperty?: (propertyName: string, callback: (newValue: unknown, oldValue: unknown) => void) => () => void
    }
  }
}

type EditorStore = ReturnType<typeof useEditorStore>
type EditorNode = EditorStore['nodes'][number]
type MessageApi = ReturnType<typeof useMessage>

interface InteractionEngineContext {
  editorStore: EditorStore
  message: MessageApi
}

export interface InteractionAction {
  action: string
  targetComponentId?: string
  targetProperty?: string
  updateValue?: any
  jumpConfig?: {
    jumpType: 'external' | 'internal'
    url?: string
    internalPath?: string
    target?: string
  }
  modifyConfig?: {
    targetComponentId: string
    targetProperty: string
    updateValue: any
    updateMode?: 'replace' | 'merge' | 'append'
  }
}

export interface InteractionEvent {
  event: string
  watchedProperty?: string
  condition?: {
    type: 'comparison' | 'range' | 'expression'
    operator?: string
    value?: any
  }
  responses: InteractionAction[]
  enabled: boolean
}

function findComponentElement(componentId: string): VueComponentElement | null {
  return document.querySelector(`[data-component-id="${componentId}"]`) as VueComponentElement | null
}

function executeJumpAction(action: InteractionAction, message: MessageApi) {
  try {
    if (action.jumpConfig) {
      const { jumpType, url, internalPath, target = '_self' } = action.jumpConfig

      if (jumpType === 'external' && url) {
        window.open(url, target)
      } else if (jumpType === 'internal' && internalPath) {
        if (target === '_blank') {
          window.open(`${window.location.origin}${internalPath}`, '_blank')
        } else {
          window.location.href = internalPath
        }
      }
    } else {
      const url = action.updateValue || ''
      const target = action.targetProperty || '_blank'
      window.open(url, target)
    }
  } catch (error) {
    logger.error('[InteractionEngine] 跳转执行失败:', error)
    message.error(`跳转失败: ${error instanceof Error ? error.message : String(error)}`)
  }
}

function buildUpdatedComponentConfig(targetNode: EditorNode, targetProperty: string, updateValue: any) {
  const currentMetadata = (targetNode.metadata || {}) as Record<string, any>
  const currentUnifiedConfig = (currentMetadata.unifiedConfig || {}) as Record<string, any>
  const currentComponent = (currentUnifiedConfig.component || {}) as Record<string, any>
  const updatedComponent = {
    ...currentComponent,
    [targetProperty]: updateValue
  }

  return {
    currentMetadata,
    currentUnifiedConfig,
    updatedComponent
  }
}

function notifyExposedComponentConfig(targetComponentId: string, updatedComponent: Record<string, unknown>) {
  try {
    const targetElement = findComponentElement(targetComponentId)
    if (targetElement?.__vueParentComponent?.exposed?.updateConfig) {
      targetElement.__vueParentComponent.exposed.updateConfig('component', updatedComponent)
    }
  } catch (error) {
    logger.warn('[InteractionEngine] 直接更新组件配置失败:', error)
  }
}

function executeModifyAction(action: InteractionAction, context: InteractionEngineContext) {
  const { editorStore, message } = context

  try {
    const { targetComponentId, targetProperty, updateValue } = action.modifyConfig || action

    if (!targetComponentId || !targetProperty) {
      throw new Error('缺少目标组件ID或属性名')
    }

    const targetNode = editorStore.nodes.find((node) => node.id === targetComponentId)
    if (!targetNode) {
      throw new Error(`目标组件未找到: ${targetComponentId}`)
    }

    const { currentMetadata, currentUnifiedConfig, updatedComponent } = buildUpdatedComponentConfig(
      targetNode,
      targetProperty,
      updateValue
    )

    editorStore.updateNode(targetComponentId, {
      properties: {
        ...targetNode.properties,
        [targetProperty]: updateValue
      },
      metadata: {
        ...currentMetadata,
        unifiedConfig: {
          ...currentUnifiedConfig,
          component: updatedComponent
        },
        lastInteractionUpdate: Date.now()
      }
    })

    notifyExposedComponentConfig(targetComponentId, updatedComponent)
    message.success(`属性已更新: ${targetProperty} = ${updateValue}`)
  } catch (error) {
    logger.error('[InteractionEngine] 属性修改失败:', error)
    message.error(`属性修改失败: ${error instanceof Error ? error.message : String(error)}`)
  }
}

function executeActionWithContext(action: InteractionAction, context: InteractionEngineContext) {
  switch (action.action) {
    case 'jump':
    case 'navigateToUrl':
      executeJumpAction(action, context.message)
      break

    case 'modify':
    case 'updateComponentData':
      executeModifyAction(action, context)
      break

    default:
      logger.warn(`[InteractionEngine] 未知的交互动作类型: ${action.action}`)
      context.message.warning(`未知的交互动作: ${action.action}`)
  }
}

function checkComparisonCondition(operator: string | undefined, actualValue: any, expectedValue: any): boolean {
  switch (operator) {
    case 'equals':
      return actualValue == expectedValue
    case 'notEquals':
      return actualValue != expectedValue
    case 'greaterThan':
      return Number(actualValue) > Number(expectedValue)
    case 'greaterThanOrEqual':
      return Number(actualValue) >= Number(expectedValue)
    case 'lessThan':
      return Number(actualValue) < Number(expectedValue)
    case 'lessThanOrEqual':
      return Number(actualValue) <= Number(expectedValue)
    case 'contains':
      return String(actualValue).includes(String(expectedValue))
    case 'startsWith':
      return String(actualValue).startsWith(String(expectedValue))
    case 'endsWith':
      return String(actualValue).endsWith(String(expectedValue))
    default:
      logger.warn(`[InteractionEngine] 未知的比较操作符: ${operator}`)
      return false
  }
}

function checkRangeCondition(value: any, rangeValue: string): boolean {
  try {
    const numValue = Number(value)

    if (rangeValue.includes('-')) {
      const [min, max] = rangeValue.split('-').map(Number)
      return numValue >= min && numValue <= max
    } else if (rangeValue.startsWith('>')) {
      const min = Number(rangeValue.substring(1))
      return numValue > min
    } else if (rangeValue.startsWith('<')) {
      const max = Number(rangeValue.substring(1))
      return numValue < max
    }

    return false
  } catch (error) {
    logger.error('[InteractionEngine] 范围条件解析失败:', error)
    return false
  }
}

function checkExpressionCondition(value: any, expression: string): boolean {
  try {
    const safeExpression = expression.replace(/value/g, String(value))

    if (/^[\d\s+\-*/.()><=!&|]+$/.test(safeExpression)) {
      return Function(`"use strict"; return (${safeExpression})`)()
    }

    return false
  } catch (error) {
    logger.error('[InteractionEngine] 表达式条件评估失败:', error)
    return false
  }
}

function checkCondition(condition: InteractionEvent['condition'], value: any): boolean {
  if (!condition) return true

  try {
    switch (condition.type) {
      case 'comparison':
        return checkComparisonCondition(condition.operator, value, condition.value)

      case 'range':
        return checkRangeCondition(value, condition.value)

      case 'expression':
        return checkExpressionCondition(value, condition.value)

      default:
        logger.warn(`[InteractionEngine] 未知的条件类型: ${condition.type}`)
        return true
    }
  } catch (error) {
    logger.error('[InteractionEngine] 条件检查失败:', error)
    return false
  }
}

function executeInteractionWithContext(
  interaction: InteractionEvent,
  triggerData: any,
  context: InteractionEngineContext
) {
  if (!interaction.enabled) {
    return
  }

  if (interaction.event === 'dataChange' && interaction.condition && triggerData !== undefined) {
    if (!checkCondition(interaction.condition, triggerData)) {
      return
    }
  }

  interaction.responses.forEach((action) => {
    executeActionWithContext(action, context)
  })
}

function registerPropertyWatcherWithContext(
  componentId: string,
  propertyName: string,
  interactions: InteractionEvent[],
  context: InteractionEngineContext
) {
  const targetNode = context.editorStore.nodes.find((node) => node.id === componentId)
  if (!targetNode) {
    logger.warn(`[InteractionEngine] 注册属性监听失败，组件未找到: ${componentId}`)
    return
  }

  try {
    const targetElement = findComponentElement(componentId)
    if (targetElement?.__vueParentComponent?.exposed?.watchProperty) {
      const unwatch = targetElement.__vueParentComponent.exposed.watchProperty(
        propertyName,
        (newValue: unknown, _oldValue: unknown) => {
          interactions.forEach((interaction) => {
            if (interaction.event === 'dataChange' && interaction.watchedProperty === propertyName) {
              executeInteractionWithContext(interaction, newValue, context)
            }
          })
        }
      )

      return unwatch
    }
  } catch (error) {
    logger.error('[InteractionEngine] 属性监听器注册失败:', error)
  }

  return null
}

/**
 * Create the interaction execution engine.
 */
export function createInteractionEngine() {
  const editorStore = useEditorStore()
  const message = useMessage()
  const context: InteractionEngineContext = { editorStore, message }

  return {
    executeAction: (action: InteractionAction) => executeActionWithContext(action, context),
    executeInteraction: (interaction: InteractionEvent, triggerData?: any) =>
      executeInteractionWithContext(interaction, triggerData, context),
    registerPropertyWatcher: (componentId: string, propertyName: string, interactions: InteractionEvent[]) =>
      registerPropertyWatcherWithContext(componentId, propertyName, interactions, context),
    checkCondition
  }
}

/**
 * Global interaction engine instance.
 */
export const interactionEngine = createInteractionEngine()
