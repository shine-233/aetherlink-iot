/**
 * 文件用途：汇总导出 grid/components 下的局部渲染组件。
 * 核心逻辑：为上层网格入口提供稳定的组件 import 边界。
 * 关键注意事项：新增组件时应同步 README 和导出，避免内部路径被调用方直接依赖。
 * 重构建议：可按核心渲染、拖放提示和内容包装分组导出，提升可读性。
 */
export { default as GridCore } from './GridCore.vue'
export { default as GridItemContent } from './GridItemContent.vue'
export { default as GridDropZone } from './GridDropZone.vue'

// 组件信息用于文档、调试或未来的注册元数据，当前不参与运行时分发。
export const GRID_COMPONENTS_VERSION = '1.0.0'
export const GRID_COMPONENTS_INFO = {
  version: GRID_COMPONENTS_VERSION,
  description: '模块化的网格布局组件系统',
  components: ['GridCore - 网格核心逻辑组件', 'GridItemContent - 网格项内容渲染组件', 'GridDropZone - 拖拽区域处理组件']
}
