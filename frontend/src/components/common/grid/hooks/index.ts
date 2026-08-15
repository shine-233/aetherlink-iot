/**
 * 文件用途：汇总导出 grid/hooks 下的组合式逻辑。
 * 核心逻辑：集中暴露布局、响应式、性能、历史、虚拟网格和增强版 hook。
 * 关键注意事项：导出顺序和命名应保持稳定，避免调用方绕过统一入口。
 * 重构建议：可增加分组注释或命名空间导出，区分基础 hook 与增强 hook。
 */
export { useGridLayoutPlus } from './useGridLayoutPlus'
export { useGridCore } from './useGridCore'
export { useGridHistory } from './useGridHistory'
export { useGridPerformance } from './useGridPerformance'
export { useGridResponsive } from './useGridResponsive'

export type { UseGridLayoutReturn } from '../types'
export type { UseGridCoreOptions } from './useGridCore'
export type { UseGridHistoryOptions } from './useGridHistory'
export type { UseGridPerformanceOptions, PerformanceMetrics } from './useGridPerformance'
export type { UseGridResponsiveOptions } from './useGridResponsive'
