/**
 * 文件用途：作为脚本引擎模块公共入口，统一导出类型、核心类和默认实例。
 * 核心逻辑：转发执行器、沙箱、模板、上下文和主引擎能力，供业务侧按需引入。
 * 关键注意事项：导出路径属于公共契约，移动或重命名会影响所有调用方。
 * 重构建议：可区分运行时导出和类型导出，减少入口的依赖体积。
 */

export * from '@/core/script-engine/types'
export * from '@/core/script-engine/executor'
export * from '@/core/script-engine/sandbox'
export * from '@/core/script-engine/template-manager'
export * from '@/core/script-engine/context-manager'
export * from '@/core/script-engine/script-engine'

// 导出默认脚本引擎实例
export { defaultScriptEngine } from '@/core/script-engine/script-engine'
