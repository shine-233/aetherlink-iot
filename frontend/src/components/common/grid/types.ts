/**
 * 文件用途：提供 grid 模块的类型兼容入口。
 * 核心逻辑：转导增强网格类型，维持旧调用方从 types.ts 引入的路径兼容。
 * 关键注意事项：不要轻易删除该文件，历史 import 可能仍依赖这个转发层。
 * 重构建议：可在完成引用迁移后把兼容入口标注为 deprecated，并统一到 gridLayoutPlusTypes.ts。
 */
export * from './gridLayoutPlusTypes'

import type { GridLayoutPlusConfig, GridLayoutPlusItem } from './gridLayoutPlusTypes'

export interface GridItem extends GridLayoutPlusItem {
  id?: string
  gridCol?: number
  gridRow?: number
  gridColSpan?: number
  gridRowSpan?: number
  minColSpan?: number
  maxColSpan?: number
  minRowSpan?: number
  maxRowSpan?: number
  resizable?: boolean
  draggable?: boolean
  locked?: boolean
  zIndex?: number
  temporary?: boolean
}

export interface GridConfig extends Partial<GridLayoutPlusConfig> {
  columns: number
  rowHeight: number
  gap: number
  minRows: number
  maxRows?: number
  minHeight?: number
  showGrid?: boolean
  readonly?: boolean
  collision?: 'block' | 'push' | 'none'
  bounds?: 'parent' | 'window' | 'none'
}

export interface GridCalculation {
  gridStyle: Record<string, any>
  itemStyle: Record<string, any>
  totalRows: number
  containerHeight: number
}

export interface VirtualizationConfig {
  enabled: boolean
  itemHeight: number
  overscan: number
  threshold: number
}

export interface GridPosition {
  x?: number
  y?: number
  col?: number
  row?: number
}

export interface GridSize {
  w?: number
  h?: number
  colSpan?: number
  rowSpan?: number
}

export interface PixelPosition {
  left: number
  top: number
}

export interface PixelSize {
  width: number
  height: number
}

export interface UseGridLayoutReturn {
  gridConfig: Readonly<GridConfig>
  gridCalculation: Readonly<import('vue').ComputedRef<GridCalculation>>
  gridToPixel: (gridPos: number, isCol?: boolean) => number
  pixelToGrid: (pixel: number, isCol?: boolean) => number
  getGridArea: (item: GridItem) => string
  validatePosition: (position: GridPosition, size: GridSize) => boolean
  getItemPixelPosition: (item: GridItem) => PixelPosition
  getItemPixelSize: (item: GridItem) => PixelSize
}

export const DEFAULT_GRID_CONFIG: GridConfig = {
  columns: 12,
  colNum: 12,
  rowHeight: 100,
  gap: 10,
  minRows: 1,
  minHeight: 0,
  showGrid: false,
  isDraggable: true,
  isResizable: true,
  isMirrored: false,
  autoSize: true,
  verticalCompact: true,
  margin: [10, 10],
  useCssTransforms: true,
  responsive: false,
  breakpoints: { lg: 1200, md: 996, sm: 768, xs: 480, xxs: 0 },
  cols: { lg: 12, md: 10, sm: 6, xs: 4, xxs: 2 },
  preventCollision: false,
  useStyleCursor: true,
  restoreOnDrag: false
}
