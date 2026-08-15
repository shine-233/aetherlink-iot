/**
 * 文件用途：提供核心交互系统的统一导出和初始化状态入口。
 * 核心逻辑：集中导出交互组件、配置注册表，并记录模块初始化后的组件清单。
 * 关键注意事项：公共导出路径会被编辑器和配置视图复用，调整时需要保持兼容。
 * 重构建议：可将初始化状态封装为独立模块，降低入口文件的职责混杂。
 */

import { configRegistry } from '@/core/interaction-system/managers/ConfigRegistry'

// 导出交互配置组件
export { default as InteractionCardWizard } from '@/core/interaction-system/components/InteractionCardWizard.vue'
export { default as InteractionTemplateSelector } from '@/core/interaction-system/components/InteractionTemplateSelector.vue'
export { default as InteractionPreview } from '@/core/interaction-system/components/InteractionPreview.vue'

// 导出配置管理器
export { configRegistry, default as ConfigRegistry } from '@/core/interaction-system/managers/ConfigRegistry'

interface InteractionSystemInitializationStatus {
  initialized: boolean
  initializedAt: number | null
  components: string[]
  registeredConfigComponents: number
}

const INTERACTION_SYSTEM_COMPONENTS = ['InteractionCardWizard', 'InteractionTemplateSelector', 'InteractionPreview']

const initializationStatus: InteractionSystemInitializationStatus = {
  initialized: false,
  initializedAt: null,
  components: [],
  registeredConfigComponents: 0
}

const readInitializationStatus = (): InteractionSystemInitializationStatus => ({
  ...initializationStatus,
  components: [...initializationStatus.components],
  registeredConfigComponents: configRegistry.getAll().length
})

// 向后兼容的导出（为了保持现有代码正常工作）
export const initializeSettings = () => {
  if (!initializationStatus.initialized) {
    initializationStatus.initialized = true
    initializationStatus.initializedAt = Date.now()
    initializationStatus.components = [...INTERACTION_SYSTEM_COMPONENTS]
  }

  return readInitializationStatus()
}

export const getInteractionSystemStatus = readInitializationStatus
