/**
 * 文件用途：Responsive Breakpoints 2.0 响应式断点与自适应网格系统。
 *
 * 对标 ThingsBoard 3.8+ 响应式断点规范：
 * 1. 断点定义：
 *    - lg (桌面端, width >= 1200px)：24 列高精细网格；
 *    - md (平板端, 768px <= width < 1200px)：12 列舒适网格；
 *    - sm (移动端, width < 768px)：6 列紧凑流式网格。
 * 2. 坐标与尺寸自动等比换算，并在收窄时自动避让碰撞、下推折行，防止小部件重叠溢出。
 */

export type ResponsiveBreakpoint = 'lg' | 'md' | 'sm'

export const BREAKPOINT_COLUMNS: Record<ResponsiveBreakpoint, number> = {
  lg: 24,
  md: 12,
  sm: 6
}

export const BREAKPOINT_WIDTHS = {
  lg: 1200,
  md: 768
} as const

export interface BaseLayoutItem {
  id: string
  x: number
  y: number
  w: number
  h: number
}

/**
 * 根据容器实际可用像素宽度确定当前断点
 */
export function getBreakpointForWidth(width: number): ResponsiveBreakpoint {
  if (width >= BREAKPOINT_WIDTHS.lg) return 'lg'
  if (width >= BREAKPOINT_WIDTHS.md) return 'md'
  return 'sm'
}

/**
 * 根据断点获取列数
 */
export function getColumnCountForBreakpoint(bp: ResponsiveBreakpoint): number {
  return BREAKPOINT_COLUMNS[bp]
}

/**
 * 碰撞检测：判断两个小部件是否在网格中重叠
 */
export function collides(a: BaseLayoutItem, b: BaseLayoutItem): boolean {
  if (a.id === b.id) return false
  if (a.x + a.w <= b.x) return false // a 在 b 左边
  if (a.x >= b.x + b.w) return false // a 在 b 右边
  if (a.y + a.h <= b.y) return false // a 在 b 上边
  if (a.y >= b.y + b.h) return false // a 在 b 下边
  return true
}

/**
 * 将单小部件从原始列数等比映射到目标列数，并施加边界安全约束
 */
export function scaleWidgetLayout<T extends BaseLayoutItem>(
  widget: T,
  fromCols: number,
  toCols: number,
  minW = 1
): T {
  const effectiveFrom = Math.max(1, fromCols)
  const effectiveTo = Math.max(1, toCols)

  // 1. 等比换算宽度，但不得小于 minW，也不得超过当前视口最大列数
  let newW = Math.round((widget.w / effectiveFrom) * effectiveTo)
  newW = Math.max(minW, Math.min(effectiveTo, newW))

  // 2. 等比换算水平坐标
  let newX = Math.round((widget.x / effectiveFrom) * effectiveTo)
  if (newX + newW > effectiveTo) {
    newX = Math.max(0, effectiveTo - newW)
  }

  return {
    ...widget,
    x: newX,
    w: newW
  }
}

/**
 * 将整块看板布局从小部件列表等比重排至目标列数，并自动解除碰撞
 */
export function adaptDashboardLayout<T extends BaseLayoutItem>(
  widgets: readonly T[],
  toCols: number,
  fromCols = 24
): T[] {
  if (!widgets || widgets.length === 0) return []
  if (fromCols === toCols) return widgets.map(w => ({ ...w }))

  // 1. 先按原始 y 从小到大、x 从小到大排序，保留视觉阅读流
  const sorted = [...widgets].sort((a, b) => (a.y !== b.y ? a.y - b.y : a.x - b.x))

  // 移动端最小宽度设为 2 列或全宽以保证可读性
  const minW = toCols <= 6 ? Math.min(toCols, 3) : 2

  const adapted: T[] = []

  for (const item of sorted) {
    let scaled = scaleWidgetLayout(item, fromCols, toCols, minW)

    // 2. 碰撞检测与垂直下推下浮（Gravity / Push-down）
    let hasCollision = true
    while (hasCollision) {
      hasCollision = false
      for (const placed of adapted) {
        if (collides(scaled, placed)) {
          hasCollision = true
          scaled = {
            ...scaled,
            y: placed.y + placed.h
          }
          break
        }
      }
    }

    adapted.push(scaled)
  }

  return adapted
}
