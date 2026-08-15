/*
 * 文件用途：创建 Naive UI 离散 API 实例，供非组件上下文触发消息、通知、弹窗和 loadingBar。
 * 核心逻辑：调用 createDiscreteApi 生成全局 message、notification、dialog、loadingBar。
 * 关键注意事项：该文件提供全局副作用入口，使用时应避免在纯工具函数中滥用提示。
 * 重构建议：可按业务入口收敛调用，减少隐式 UI 副作用。
 */
import { createDiscreteApi } from 'naive-ui'

/**
 * Creates a discrete API instance for Naive UI components.
 * This is used to display messages and other UI elements from outside of a component's setup function.
 */
export const { message, notification, dialog, loadingBar } = createDiscreteApi([
  'message',
  'dialog',
  'notification',
  'loadingBar'
])
