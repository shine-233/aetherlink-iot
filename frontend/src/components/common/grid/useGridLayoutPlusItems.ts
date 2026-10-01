import type { ComputedRef, Ref } from 'vue'

import type { GridLayoutPlusConfig, GridLayoutPlusItem } from './gridLayoutPlusTypes'
import { optimizeItemForLargeGrid } from './utils/validation'
import { applyIdKey, withIdKey } from './gridLayoutPlusIdKey'

/**
 * 文件用途：网格项的增删改查、位置计算与大网格布局优化。
 * 核心逻辑：所有操作都作用于 GridCore 暴露的 internalLayout，并通过回调向上层派发事件。
 * 关键注意事项：与拆分前保持一致——操作失败返回 null，布局变更后同时触发 layout-change 语义。
 */

export interface UseGridLayoutPlusItemsOptions<T> {
  gridCoreRef: Ref<T | null>
  config: ComputedRef<GridLayoutPlusConfig>
  /** 当前主键字段名（内部统一为 i，对外补别名） */
  idKey: () => string
  onItemAdd: (item: GridLayoutPlusItem) => void
  onItemDelete: (itemId: string) => void
  onItemUpdate: (itemId: string, updates: Partial<GridLayoutPlusItem>) => void
  onLayoutChange: (layout: GridLayoutPlusItem[]) => void
}

function generateId(): string {
  return `item-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`
}

export function useGridLayoutPlusItems<T extends { internalLayout: GridLayoutPlusItem[] }>(
  options: UseGridLayoutPlusItemsOptions<T>
) {
  const { gridCoreRef, config, idKey, onItemAdd, onItemDelete, onItemUpdate, onLayoutChange } = options

  const findAvailablePosition = (w: number, h: number): { x: number; y: number } => {
    const colNum = config.value.colNum
    const layout = gridCoreRef.value?.internalLayout || []

    for (let y = 0; y < 100; y++) {
      for (let x = 0; x <= colNum - w; x++) {
        const proposed = { x, y, w, h }

        const hasCollision = layout.some((item) => {
          return !(
            proposed.x + proposed.w <= item.x ||
            proposed.x >= item.x + item.w ||
            proposed.y + proposed.h <= item.y ||
            proposed.y >= item.y + item.h
          )
        })

        if (!hasCollision) {
          return { x, y }
        }
      }
    }

    return { x: 0, y: 0 }
  }

  const addItem = (type: string, itemOptions?: Partial<GridLayoutPlusItem>) => {
    const coreLayout = gridCoreRef.value?.internalLayout
    if (!coreLayout) return null

    const newItem: GridLayoutPlusItem = {
      i: generateId(),
      x: 0,
      y: 0,
      w: 2,
      h: 2,
      type,
      ...itemOptions
    }

    applyIdKey(newItem, idKey())

    const position = findAvailablePosition(newItem.w, newItem.h)
    newItem.x = position.x
    newItem.y = position.y

    coreLayout.push(newItem)
    onItemAdd(withIdKey([newItem], idKey())[0])
    return newItem
  }

  const removeItem = (itemId: string) => {
    const coreLayout = gridCoreRef.value?.internalLayout
    if (!coreLayout) return null

    const index = coreLayout.findIndex((item) => item.i === itemId)
    if (index > -1) {
      const removedItem = coreLayout.splice(index, 1)[0]
      onItemDelete(itemId)
      return removedItem
    }
    return null
  }

  const updateItem = (itemId: string, updates: Partial<GridLayoutPlusItem>) => {
    const coreLayout = gridCoreRef.value?.internalLayout
    if (!coreLayout) return null

    const item = coreLayout.find((i) => i.i === itemId)
    if (item) {
      Object.assign(item, updates)
      onItemUpdate(itemId, updates)
      return item
    }
    return null
  }

  const clearLayout = () => {
    const coreLayout = gridCoreRef.value?.internalLayout
    if (!coreLayout) return

    coreLayout.splice(0)
    onLayoutChange(withIdKey([...coreLayout], idKey()))
  }

  const getItem = (itemId: string) => {
    return gridCoreRef.value?.internalLayout?.find((item) => item.i === itemId) || null
  }

  const getAllItems = () => {
    return gridCoreRef.value?.internalLayout ? [...gridCoreRef.value.internalLayout] : []
  }

  const getLayout = () => {
    return gridCoreRef.value?.internalLayout || []
  }

  const optimizeLayoutForGridSize = (targetCols?: number, sourceCols?: number) => {
    const coreLayout = gridCoreRef.value?.internalLayout
    if (!coreLayout) return

    const targetColumns = targetCols || config.value.colNum
    const sourceColumns = sourceCols || 12

    coreLayout.forEach((item) => {
      Object.assign(item, optimizeItemForLargeGrid(item, targetColumns, sourceColumns))
    })

    onLayoutChange(withIdKey([...coreLayout], idKey()))
  }

  return {
    addItem,
    removeItem,
    updateItem,
    clearLayout,
    getItem,
    getAllItems,
    getLayout,
    optimizeLayoutForGridSize
  }
}
