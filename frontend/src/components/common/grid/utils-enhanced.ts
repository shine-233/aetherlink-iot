/**
 * 文件用途：提供增强版网格工具的聚合与兼容入口。
 * 核心逻辑：组合基础工具、布局算法、响应式和性能函数，供旧路径或高级场景复用。
 * 关键注意事项：该文件容易形成重复导出，新增能力时需避免与 utils/index.ts 语义冲突。
 * 重构建议：可逐步收敛到按领域拆分的 utils 子模块，并保留薄兼容层。
 */
import {
  calculateGridUtilization as calculatePlusGridUtilization,
  calculateTotalRows as calculatePlusTotalRows,
  generateId as generatePlusId,
  performanceMonitor,
  CacheManager
} from './utils'
import { findOptimalPosition as findPlusOptimalPosition } from './utils/layout-algorithm'
import type { GridConfig, GridItem, GridPosition, GridSize } from './types'

export { performanceMonitor }

export const cacheManager = new CacheManager<string, unknown>()

export function generateId(prefix = 'grid-item'): string {
  return generatePlusId(prefix)
}

function toPlusItem(item: GridItem): GridItem {
  return {
    ...item,
    i: String(item.i ?? item.id ?? ''),
    x: Number(item.x ?? Math.max(0, (item.gridCol ?? 1) - 1)),
    y: Number(item.y ?? Math.max(0, (item.gridRow ?? 1) - 1)),
    w: Number(item.w ?? item.gridColSpan ?? 1),
    h: Number(item.h ?? item.gridRowSpan ?? 1)
  }
}

function getColumns(config: GridConfig) {
  return Number(config.columns ?? config.colNum ?? 12)
}

export function validateGridItem(item: GridItem) {
  const id = item.id ?? item.i
  const col = item.gridCol ?? (item.x ?? 0) + 1
  const row = item.gridRow ?? (item.y ?? 0) + 1
  const colSpan = item.gridColSpan ?? item.w ?? 0
  const rowSpan = item.gridRowSpan ?? item.h ?? 0

  if (!id) return { success: false, data: false, message: 'ID is required' }
  if (col < 1 || row < 1) return { success: false, data: false, message: '位置无效' }
  if (colSpan < 1 || rowSpan < 1) return { success: false, data: false, message: '跨度无效' }
  if (item.minColSpan && colSpan < item.minColSpan) return { success: false, data: false, message: '最小值约束未满足' }
  if (item.maxColSpan && colSpan > item.maxColSpan) return { success: false, data: false, message: '最大值约束未满足' }

  return { success: true, data: true }
}

export function getOverlapArea(item1: GridItem, item2: GridItem) {
  const a = toPlusItem(item1)
  const b = toPlusItem(item2)
  const left = Math.max(a.x, b.x)
  const right = Math.min(a.x + a.w, b.x + b.w)
  const top = Math.max(a.y, b.y)
  const bottom = Math.min(a.y + a.h, b.y + b.h)

  if (right <= left || bottom <= top) return null

  return {
    colStart: left + 1,
    colEnd: right + 1,
    rowStart: top + 1,
    rowEnd: bottom + 1
  }
}

export function isItemsOverlapping(item1: GridItem, item2: GridItem, tolerance = 0) {
  const overlap = getOverlapArea(item1, item2)
  if (!overlap) {
    const a = toPlusItem(item1)
    const b = toPlusItem(item2)
    const touchesHorizontally = a.x + a.w === b.x || b.x + b.w === a.x
    const overlapsVertically = a.y < b.y + b.h && b.y < a.y + a.h
    const touchesVertically = a.y + a.h === b.y || b.y + b.h === a.y
    const overlapsHorizontally = a.x < b.x + b.w && b.x < a.x + a.w
    const hasTemporaryItem = Boolean(item1.temporary || item2.temporary)

    return (
      hasTemporaryItem &&
      tolerance >= 0.5 &&
      ((touchesHorizontally && overlapsVertically) || (touchesVertically && overlapsHorizontally))
    )
  }

  const area = Math.max(0, overlap.colEnd - overlap.colStart) * Math.max(0, overlap.rowEnd - overlap.rowStart)
  const firstArea = (item1.gridColSpan ?? item1.w ?? 1) * (item1.gridRowSpan ?? item1.h ?? 1)
  const secondArea = (item2.gridColSpan ?? item2.w ?? 1) * (item2.gridRowSpan ?? item2.h ?? 1)
  const minArea = Math.min(firstArea, secondArea)

  return minArea === 0 ? false : area / minArea >= tolerance
}

export function checkCollisions(item: GridItem, items: GridItem[], ignoreTemporary = false) {
  const collisions = items.filter((candidate) => {
    if ((candidate.id ?? candidate.i) === (item.id ?? item.i)) return false
    if (ignoreTemporary && candidate.temporary) return false
    return isItemsOverlapping(item, candidate)
  })

  return {
    hasCollision: collisions.length > 0,
    collisions,
    suggestedPosition: collisions.length
      ? findOptimalPosition(items, item.gridColSpan ?? item.w ?? 1, item.gridRowSpan ?? item.h ?? 1, 12)
      : undefined
  }
}

export function validateGridPosition(position: GridPosition, size: GridSize, config: GridConfig) {
  const col = position.col ?? (position.x ?? 0) + 1
  const row = position.row ?? (position.y ?? 0) + 1
  const colSpan = size.colSpan ?? size.w ?? 1
  const rowSpan = size.rowSpan ?? size.h ?? 1

  if (col < 1 || row < 1 || col + colSpan - 1 > getColumns(config)) return false
  if (config.maxRows && row + rowSpan - 1 > config.maxRows) return false
  return true
}

export function calculateGridUtilization(items: GridItem[], config: GridConfig) {
  return calculatePlusGridUtilization(items.map(toPlusItem), getColumns(config), config.minRows) / 100
}

export function calculateTotalRows(items: GridItem[], minRows = 0) {
  return calculatePlusTotalRows(items.map(toPlusItem), minRows)
}

export function getGridStatistics(items: GridItem[], config: GridConfig) {
  const columns = getColumns(config)
  const totalItems = items.length
  const totalRows = calculateStatisticsRows(items)
  const totalCells = columns * totalRows
  const usedCells = calculateStatisticsUsedCells(items)
  const largestItem = findLargestGridItem(items)
  const overlappingItems = countOverlappingGridItems(items)

  return {
    totalItems,
    totalRows,
    totalCells,
    usedCells,
    utilization: totalCells > 0 ? (usedCells / totalCells) * 100 : 0,
    overlappingItems,
    largestItem,
    averageSize: totalItems > 0 ? usedCells / totalItems : 0
  }
}

function calculateStatisticsRows(items: GridItem[]) {
  if (items.length === 0) return 0
  const maxRow = Math.max(...items.map((item) => item.gridRow ?? (item.y ?? 0) + 1))
  const maxRowSpan = Math.max(...items.map((item) => item.gridRowSpan ?? item.h ?? 1))
  return maxRow + maxRowSpan - 1
}

function gridItemArea(item: GridItem) {
  return (item.gridColSpan ?? item.w ?? 1) * (item.gridRowSpan ?? item.h ?? 1)
}

function calculateStatisticsUsedCells(items: GridItem[]) {
  return items.reduce((sum, item) => sum + gridItemArea(item), 0)
}

function findLargestGridItem(items: GridItem[]) {
  return items.reduce<GridItem | null>((largest, item) => {
    const largestArea = largest ? gridItemArea(largest) : -1
    return gridItemArea(item) > largestArea ? item : largest
  }, null)
}

function countOverlappingGridItems(items: GridItem[]) {
  return items.filter((item, index) =>
    items.some((candidate, candidateIndex) => candidateIndex > index && isItemsOverlapping(item, candidate))
  ).length
}

export function cloneGridItem(item: GridItem): GridItem {
  try {
    return JSON.parse(JSON.stringify(item))
  } catch {
    return { ...item }
  }
}

export function itemToGridArea(item: GridItem) {
  const col = item.gridCol ?? (item.x ?? 0) + 1
  const row = item.gridRow ?? (item.y ?? 0) + 1
  const colSpan = item.gridColSpan ?? item.w ?? 1
  const rowSpan = item.gridRowSpan ?? item.h ?? 1

  return {
    rowStart: row,
    colStart: col,
    rowEnd: row + rowSpan,
    colEnd: col + colSpan
  }
}

export function findOptimalPosition(
  itemOrItems: GridItem | GridItem[],
  itemsOrWidth: GridItem[] | number,
  configOrHeight: GridConfig | number,
  colCount = 12
) {
  if (Array.isArray(itemOrItems)) {
    const result = findPlusOptimalPosition(
      itemOrItems.map(toPlusItem),
      Number(itemsOrWidth),
      Number(configOrHeight),
      colCount
    )
    return { col: result.x + 1, row: result.y + 1, score: result.score }
  }

  const newItem = itemOrItems
  const items = Array.isArray(itemsOrWidth) ? itemsOrWidth : []
  const config = typeof configOrHeight === 'object' ? configOrHeight : ({ columns: colCount } as GridConfig)
  const width = newItem.gridColSpan ?? newItem.w ?? 1
  const height = newItem.gridRowSpan ?? newItem.h ?? 1
  const cacheKey = JSON.stringify({
    id: newItem.id ?? newItem.i,
    width,
    height,
    columns: getColumns(config),
    items: items.map((item) => [
      item.id ?? item.i,
      item.gridCol ?? item.x,
      item.gridRow ?? item.y,
      item.gridColSpan ?? item.w,
      item.gridRowSpan ?? item.h
    ])
  })
  const cached = cacheManager.get(cacheKey) as { col: number; row: number; score: number } | null
  if (cached) return cached

  const result = findPlusOptimalPosition(items.map(toPlusItem), width, height, getColumns(config))
  const position = { col: result.x + 1, row: result.y + 1, score: result.score }
  cacheManager.set(cacheKey, position)
  return position
}
