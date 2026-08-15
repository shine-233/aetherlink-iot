/**
 * 文件用途：统一导出脚本引擎相关 Vue 组件和常用类型。
 * 核心逻辑：集中暴露完整脚本编辑器、结果视图、简化编辑器及组件 props/emits 类型。
 * 关键注意事项：组件导出名会被业务页面直接引用，调整时需要兼容旧入口。
 * 重构建议：可将组件导出与核心类型转发拆分，避免组件层入口承担过多职责。
 */

export { default as SimpleScriptEditor } from '@/core/script-engine/components/SimpleScriptEditor.vue'

// 重新导出 script-engine 的类型
export type {
  ScriptExecutionResult,
  ScriptTemplate,
  ScriptConfig,
  ScriptExecutionContext,
  TemplateCategory,
  ScriptLog
} from '../types'
