/**
 * 文件用途：管理网格核心布局状态和基础事件处理。
 * 核心逻辑：维护内部 layout、同步外部变更，并提供创建、更新、移动、缩放等事件回调。
 * 关键注意事项：需要避免 watch 与事件回写互相触发导致循环更新。
 * 重构建议：可把布局同步、事件发射和配置派生拆成独立 composable。
 */
import { computed, ref } from 'vue'
import type { Ref } from 'vue'
import type { GridLayoutPlusItem, GridLayoutPlusConfig, LayoutOperationResult } from '../gridLayoutPlusTypes'
import { validateLayout, validateGridItem, cloneLayout, getLayoutBounds, getLayoutStats } from '../gridLayoutPlusUtils'
import { DEFAULT_GRID_LAYOUT_PLUS_CONFIG } from '../gridLayoutPlusTypes'

export interface UseGridCoreOptions {
  /** 初始布局数据 */
  initialLayout?: GridLayoutPlusItem[]
  /** 网格配置 */
  config?: Partial<GridLayoutPlusConfig>
  /** 是否启用深度验证 */
  enableValidation?: boolean
}

interface GridCoreState {
  layout: Ref<GridLayoutPlusItem[]>
  selectedItems: Ref<string[]>
  isLoading: Ref<boolean>
  error: Ref<Error | null>
  config: Ref<GridLayoutPlusConfig>
}

function createGridCoreState(options: UseGridCoreOptions): GridCoreState {
  return {
    layout: ref<GridLayoutPlusItem[]>(options.initialLayout || []),
    selectedItems: ref<string[]>([]),
    isLoading: ref(false),
    error: ref<Error | null>(null),
    config: ref<GridLayoutPlusConfig>({
      ...DEFAULT_GRID_LAYOUT_PLUS_CONFIG,
      ...options.config
    })
  }
}

function createGridCoreComputed(state: GridCoreState, options: UseGridCoreOptions) {
  const { layout, selectedItems, config, error } = state

  const layoutStats = computed(() => {
    try {
      return getLayoutStats(layout.value, config.value.colNum)
    } catch (err) {
      console.error('Failed to calculate layout stats:', err)
      return {
        totalItems: layout.value.length,
        totalRows: 0,
        utilization: 0,
        density: 0
      }
    }
  })

  const layoutBounds = computed(() => {
    try {
      return getLayoutBounds(layout.value)
    } catch (err) {
      console.error('Failed to calculate layout bounds:', err)
      return { minX: 0, maxX: 0, minY: 0, maxY: 0, width: 0, height: 0 }
    }
  })

  const isValidLayout = computed(() => {
    if (!options.enableValidation) return true

    try {
      const validation = validateLayout(layout.value)
      return validation.success
    } catch (err) {
      error.value = err as Error
      return false
    }
  })

  const hasSelectedItems = computed(() => selectedItems.value.length > 0)

  return {
    layoutStats,
    layoutBounds,
    isValidLayout,
    hasSelectedItems
  }
}

function validationFailure<T>(validation: LayoutOperationResult<boolean>): LayoutOperationResult<T> {
  return {
    success: false,
    error: validation.error,
    message: validation.message
  }
}

function operationFailure<T>(action: string, err: unknown, error: Ref<Error | null>): LayoutOperationResult<T> {
  const errorObj = err as Error
  error.value = errorObj
  return {
    success: false,
    error: errorObj,
    message: `Failed to ${action}: ${errorObj.message}`
  }
}

function createLayoutActions(state: GridCoreState, options: UseGridCoreOptions) {
  const { layout, selectedItems, error } = state

  const updateLayout = (newLayout: GridLayoutPlusItem[]): LayoutOperationResult<void> => {
    try {
      if (options.enableValidation) {
        const validation = validateLayout(newLayout)
        if (!validation.success) {
          return validationFailure(validation)
        }
      }

      layout.value = cloneLayout(newLayout)
      error.value = null

      return { success: true }
    } catch (err) {
      return operationFailure('update layout', err, error)
    }
  }

  const addItem = (item: GridLayoutPlusItem): LayoutOperationResult<GridLayoutPlusItem> => {
    try {
      if (options.enableValidation) {
        const validation = validateGridItem(item)
        if (!validation.success) {
          return validationFailure(validation)
        }
      }

      if (layout.value.some((existingItem) => existingItem.i === item.i)) {
        return {
          success: false,
          error: new Error('Item ID already exists'),
          message: `项目ID '${item.i}' 已存在`
        }
      }

      layout.value.push({ ...item })
      return { success: true, data: item }
    } catch (err) {
      return operationFailure('add item', err, error)
    }
  }

  const removeItem = (itemId: string): LayoutOperationResult<GridLayoutPlusItem> => {
    try {
      const index = layout.value.findIndex((item) => item.i === itemId)
      if (index === -1) {
        return {
          success: false,
          error: new Error('Item not found'),
          message: `项目 '${itemId}' 不存在`
        }
      }

      const removedItem = layout.value.splice(index, 1)[0]
      const selectedIndex = selectedItems.value.indexOf(itemId)
      if (selectedIndex > -1) {
        selectedItems.value.splice(selectedIndex, 1)
      }

      return { success: true, data: removedItem }
    } catch (err) {
      return operationFailure('remove item', err, error)
    }
  }

  const updateItem = (
    itemId: string,
    updates: Partial<GridLayoutPlusItem>
  ): LayoutOperationResult<GridLayoutPlusItem> => {
    try {
      const item = layout.value.find((item) => item.i === itemId)
      if (!item) {
        return {
          success: false,
          error: new Error('Item not found'),
          message: `项目 '${itemId}' 不存在`
        }
      }

      const updatedItem = { ...item, ...updates }
      if (options.enableValidation) {
        const validation = validateGridItem(updatedItem)
        if (!validation.success) {
          return validationFailure(validation)
        }
      }

      Object.assign(item, updates)
      return { success: true, data: item }
    } catch (err) {
      return operationFailure('update item', err, error)
    }
  }

  const clearLayout = () => {
    layout.value = []
    selectedItems.value = []
    error.value = null
  }

  return {
    updateLayout,
    addItem,
    removeItem,
    updateItem,
    clearLayout
  }
}

function createSelectionActions(state: GridCoreState) {
  const { layout, selectedItems } = state

  const selectItem = (itemId: string) => {
    if (!selectedItems.value.includes(itemId)) {
      selectedItems.value.push(itemId)
    }
  }

  const deselectItem = (itemId: string) => {
    const index = selectedItems.value.indexOf(itemId)
    if (index > -1) {
      selectedItems.value.splice(index, 1)
    }
  }

  const toggleItemSelection = (itemId: string) => {
    const index = selectedItems.value.indexOf(itemId)
    if (index > -1) {
      selectedItems.value.splice(index, 1)
    } else {
      selectedItems.value.push(itemId)
    }
  }

  const selectAll = () => {
    selectedItems.value = layout.value.map((item) => item.i)
  }

  const deselectAll = () => {
    selectedItems.value = []
  }

  return {
    selectItem,
    deselectItem,
    toggleItemSelection,
    selectAll,
    deselectAll
  }
}

function createLayoutQueries(state: GridCoreState) {
  const { layout, selectedItems } = state

  const getItem = (itemId: string) => {
    return layout.value.find((item) => item.i === itemId) || null
  }

  const getSelectedItems = () => {
    return layout.value.filter((item) => selectedItems.value.includes(item.i))
  }

  return {
    getItem,
    getSelectedItems
  }
}

/**
 * 核心网格状态管理Hook
 * 提供基础的布局数据管理和验证功能
 */
export function useGridCore(options: UseGridCoreOptions = {}) {
  const state = createGridCoreState(options)
  const computedState = createGridCoreComputed(state, options)
  const layoutActions = createLayoutActions(state, options)
  const selectionActions = createSelectionActions(state)
  const layoutQueries = createLayoutQueries(state)

  return {
    // 状态
    layout: state.layout,
    selectedItems: state.selectedItems,
    isLoading: state.isLoading,
    error: state.error,
    config: state.config,

    // 计算属性
    layoutStats: computedState.layoutStats,
    layoutBounds: computedState.layoutBounds,
    isValidLayout: computedState.isValidLayout,
    hasSelectedItems: computedState.hasSelectedItems,

    // 布局操作
    updateLayout: layoutActions.updateLayout,
    addItem: layoutActions.addItem,
    removeItem: layoutActions.removeItem,
    updateItem: layoutActions.updateItem,
    clearLayout: layoutActions.clearLayout,

    // 选择管理
    selectItem: selectionActions.selectItem,
    deselectItem: selectionActions.deselectItem,
    toggleItemSelection: selectionActions.toggleItemSelection,
    selectAll: selectionActions.selectAll,
    deselectAll: selectionActions.deselectAll,

    // 查询方法
    getItem: layoutQueries.getItem,
    getSelectedItems: layoutQueries.getSelectedItems
  }
}
