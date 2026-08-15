/**
 * 文件用途：汇总导出 grid/utils 下的纯工具模块。
 * 核心逻辑：统一暴露通用、校验、响应式、性能和布局算法工具。
 * 关键注意事项：新增导出要避免与上层 gridLayoutPlusUtils 的同名能力产生歧义。
 * 重构建议：可按领域提供命名空间导出，让调用方更容易定位来源。
 */
export {
  validateGridItem,
  validateLayout,
  validateGridPosition,
  checkItemsOverlap,
  validateNoOverlaps,
  validateResponsiveConfig,
  validateExtendedGridConfig,
  validateLargeGridPerformance,
  optimizeItemForLargeGrid
} from './validation'

export {
  findAvailablePosition,
  findOptimalPosition,
  isPositionAvailable,
  compactLayout,
  sortLayout,
  getLayoutBounds,
  getOverlapArea,
  moveItemWithCollisionHandling
} from './layout-algorithm'

export {
  debounce,
  throttle,
  optimizeLayoutPerformance,
  PerformanceMonitor,
  performanceMonitor,
  getMemoryUsage,
  CacheManager,
  AsyncQueue
} from './performance'

export {
  createResponsiveLayout,
  transformLayoutForBreakpoint,
  mergeResponsiveLayouts,
  validateResponsiveLayout,
  getBreakpointInfo,
  calculateBreakpointTransition,
  adaptItemSizeForBreakpoint,
  ResponsiveMediaQuery
} from './responsive'

export {
  generateId,
  cloneLayout,
  cloneGridItem,
  getLayoutStats,
  filterLayout,
  searchLayout,
  itemToGridArea,
  calculateGridUtilization,
  calculateTotalRows,
  calculateColWidth,
  calculateContainerHeight,
  getGridArea,
  getItemPixelPosition,
  getItemPixelSize,
  gridToPixel,
  pixelToGrid,
  generateGridBackgroundStyle,
  getGridStatistics,
  uniqueArray,
  parseNumber,
  clamp,
  formatFileSize,
  formatDuration
} from './common'

export { validateGridItem as validateItem } from './validation'
export { findAvailablePosition as findPosition } from './layout-algorithm'
export { getLayoutStats as getStats } from './common'
