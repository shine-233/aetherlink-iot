/**
 * 文件用途：作为 grid 模块根导出入口，汇总组件、类型、hooks 和工具函数。
 * 核心逻辑：对外屏蔽内部目录结构，让调用方通过稳定路径引入网格能力。
 * 关键注意事项：导出项是跨页面共享契约，调整前需检查所有 common/grid 引用。
 * 重构建议：可将旧版与增强版导出分组，逐步淘汰重复或历史兼容入口。
 */

// ==================== Grid Layout Plus (推荐) ====================
// 基于 grid-layout-plus 的现代化解决方案
export * from './gridLayoutPlusIndex'

// ==================== 组件信息 ====================

export const GRID_VERSION = '3.0.0'

/**
 * 获取组件信息
 */
export function getGridInfo() {
  return {
    version: GRID_VERSION,
    currentComponent: 'GridLayoutPlus',
    description: '项目已全面采用基于 grid-layout-plus 的现代化网格布局组件。',
    features: [
      '基于 Grid Layout Plus 库',
      '响应式布局',
      '拖拽和调整大小',
      '主题支持',
      '完整的 TypeScript 支持',
      '性能优化',
      '丰富的 API 和事件系统',
      '历史记录（撤销/重做）',
      '布局导入/导出'
    ],
    migration_notes:
      '所有旧的 DraggableResizableGrid 相关组件和 API 已被移除，请确保所有引用都已更新至 GridLayoutPlus。'
  }
}
