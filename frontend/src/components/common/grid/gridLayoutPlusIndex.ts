/**
 * 文件用途：汇总导出 Grid Layout Plus 组件、类型、工具、hook 和版本信息。
 * 核心逻辑：提供统一 import 入口，并暴露组件信息函数供文档、调试或运行时检查使用。
 * 关键注意事项：这里是外部依赖的公共边界，删除或改名导出会影响调用方构建。
 * 重构建议：可与根级 index.ts 明确分工，减少重复导出和版本信息分散。
 */

// 导出主要组件
export { default as GridLayoutPlus } from './GridLayoutPlus.vue'

// 导出类型定义
export type * from './gridLayoutPlusTypes'

// 导出工具函数
export * from './gridLayoutPlusUtils'

// 导出Hook
export { useGridLayoutPlus } from './hooks/useGridLayoutPlus'

// 导出默认配置和主题
export { DEFAULT_GRID_LAYOUT_PLUS_CONFIG, LIGHT_THEME, DARK_THEME } from './gridLayoutPlusTypes'

// 导出常用类型别名
export type {
  GridLayoutPlusItem as GridItem,
  GridLayoutPlusConfig as GridConfig,
  GridLayoutPlusProps as GridProps,
  GridLayoutPlusEmits as GridEmits
} from './gridLayoutPlusTypes'

// 版本信息
export const GRID_LAYOUT_PLUS_VERSION = '1.0.0'

/**
 * 获取组件信息
 */
export function getGridLayoutPlusInfo() {
  return {
    name: 'Grid Layout Plus',
    version: GRID_LAYOUT_PLUS_VERSION,
    description: '基于 grid-layout-plus 的企业级网格布局组件',
    features: [
      '基于 Grid Layout Plus 库',
      '完整的 TypeScript 支持',
      '响应式布局',
      '拖拽和调整大小',
      '主题支持',
      '性能优化',
      '丰富的 API',
      '事件处理',
      '历史记录',
      '导入导出'
    ],
    dependencies: {
      'grid-layout-plus': '^1.0.0',
      vue: '^3.0.0'
    }
  }
}
