/**
 * 文件用途：提供增强版网格布局的组合式状态入口。
 * 核心逻辑：聚合布局校验、位置计算、响应式转换、历史记录、自动保存和性能节流。
 * 关键注意事项：该 hook 是 GridLayoutPlus 的核心状态层，任何返回字段变更都可能影响页面集成。
 * 重构建议：可拆分为 state、commands、history、responsive 四个子 hook，降低单文件复杂度。
 */

import { computed, ref, watch, type ComputedRef, type Ref } from 'vue'
import type {
  GridLayoutPlusItem,
  GridLayoutPlusConfig,
  ResponsiveLayout,
  LayoutOperationResult,
  PerformanceConfig
} from '../gridLayoutPlusTypes'
import {
  validateLayout,
  validateGridItem,
  findAvailablePosition,
  generateId,
  cloneLayout,
  getLayoutBounds,
  compactLayout,
  sortLayout,
  filterLayout,
  searchLayout,
  getLayoutStats,
  createResponsiveLayout,
  optimizeLayoutPerformance,
  debounce,
  throttle
} from '../gridLayoutPlusUtils'
import { DEFAULT_GRID_LAYOUT_PLUS_CONFIG } from '../gridLayoutPlusTypes'

export interface UseGridLayoutPlusOptions {
  /** 初始布局 */
  initialLayout?: GridLayoutPlusItem[]
  /** 网格配置 */
  config?: Partial<GridLayoutPlusConfig>
  /** 性能配置 */
  performance?: Partial<PerformanceConfig>
  /** 是否启用自动保存 */
  autoSave?: boolean
  /** 自动保存延迟 */
  autoSaveDelay?: number
  /** 保存回调 */
  onSave?: (layout: GridLayoutPlusItem[]) => void
  /** 是否启用历史记录 */
  enableHistory?: boolean
  /** 历史记录最大长度 */
  maxHistoryLength?: number
}

type GridLayoutPlusState = ReturnType<typeof createGridLayoutPlusState>
type GridLayoutComputedState = ReturnType<typeof createGridLayoutComputedState>
type SaveToHistory = () => void
type AutoSave = () => void
type LayoutActionContext = {
  state: GridLayoutPlusState
  saveToHistory: SaveToHistory
  autoSave: AutoSave
}

const DEFAULT_LAYOUT_ITEM_SIZE = 2

function createGridLayoutPlusState(options: UseGridLayoutPlusOptions) {
  const config = ref<GridLayoutPlusConfig>({
    ...DEFAULT_GRID_LAYOUT_PLUS_CONFIG,
    ...options.config
  })

  const performanceConfig = ref<PerformanceConfig>({
    enableVirtualization: false,
    virtualizationThreshold: 100,
    debounceDelay: 100,
    throttleDelay: 16,
    enableLazyLoading: false,
    lazyLoadingBuffer: 5,
    ...options.performance
  })

  return {
    config,
    performanceConfig,
    layout: ref<GridLayoutPlusItem[]>(options.initialLayout || []),
    isLoading: ref(false),
    error: ref<Error | null>(null),
    selectedItems: ref<string[]>([]),
    currentBreakpoint: ref<string>('lg'),
    history: ref<GridLayoutPlusItem[][]>([]),
    historyIndex: ref(-1),
    responsiveLayouts: ref<ResponsiveLayout>({})
  }
}

function createGridLayoutComputedState(state: GridLayoutPlusState) {
  const layoutStats = computed(() => getLayoutStats(state.layout.value, state.config.value.colNum))
  const layoutBounds = computed(() => getLayoutBounds(state.layout.value))
  const hasSelectedItems = computed(() => state.selectedItems.value.length > 0)
  const canUndo = computed(() => state.historyIndex.value > 0)
  const canRedo = computed(() => state.historyIndex.value < state.history.value.length - 1)
  const isValidLayout = computed(() => validateLayout(state.layout.value).success)
  const optimizedLayout = computed(() => {
    return optimizeLayoutPerformance(state.layout.value, state.performanceConfig.value)
  })

  return {
    layoutStats,
    layoutBounds,
    hasSelectedItems,
    canUndo,
    canRedo,
    isValidLayout,
    optimizedLayout
  }
}

function createHistoryController(
  options: UseGridLayoutPlusOptions,
  state: GridLayoutPlusState,
  canUndo: ComputedRef<boolean>,
  canRedo: ComputedRef<boolean>
) {
  const maxHistoryLength = options.maxHistoryLength || 50

  const saveToHistory = () => {
    if (!options.enableHistory) return

    const currentLayout = cloneLayout(state.layout.value)

    if (state.historyIndex.value < state.history.value.length - 1) {
      state.history.value = state.history.value.slice(0, state.historyIndex.value + 1)
    }

    state.history.value.push(currentLayout)
    state.historyIndex.value = state.history.value.length - 1

    if (state.history.value.length > maxHistoryLength) {
      state.history.value = state.history.value.slice(-maxHistoryLength)
      state.historyIndex.value = state.history.value.length - 1
    }
  }

  const undo = (): LayoutOperationResult<boolean> => {
    if (!canUndo.value) {
      return {
        success: false,
        error: new Error('Nothing to undo'),
        message: '没有可撤销的操作'
      }
    }

    state.historyIndex.value--
    state.layout.value = cloneLayout(state.history.value[state.historyIndex.value])

    return {
      success: true,
      data: true,
      message: '撤销成功'
    }
  }

  const redo = (): LayoutOperationResult<boolean> => {
    if (!canRedo.value) {
      return {
        success: false,
        error: new Error('Nothing to redo'),
        message: '没有可重做的操作'
      }
    }

    state.historyIndex.value++
    state.layout.value = cloneLayout(state.history.value[state.historyIndex.value])

    return {
      success: true,
      data: true,
      message: '重做成功'
    }
  }

  return {
    saveToHistory,
    undo,
    redo
  }
}

function createAutoSave(options: UseGridLayoutPlusOptions, layout: Ref<GridLayoutPlusItem[]>) {
  return debounce(() => {
    if (options.autoSave && options.onSave) {
      options.onSave(cloneLayout(layout.value))
    }
  }, options.autoSaveDelay || 1000)
}

function createLayoutActionContext(
  state: GridLayoutPlusState,
  saveToHistory: SaveToHistory,
  autoSave: AutoSave
): LayoutActionContext {
  return {
    state,
    saveToHistory,
    autoSave
  }
}

function createSuccessResult<T>(data: T, message: string): LayoutOperationResult<T> {
  return {
    success: true,
    data,
    message
  }
}

function createFailureResult<T>(error: Error, message: string): LayoutOperationResult<T> {
  return {
    success: false,
    error,
    message
  }
}

function createItemNotFoundResult<T>(message: string, errorMessage = 'Item not found'): LayoutOperationResult<T> {
  return createFailureResult(new Error(errorMessage), message)
}

function preserveFailedResult<T>(result: LayoutOperationResult): LayoutOperationResult<T> {
  return result as LayoutOperationResult<T>
}

function runLayoutAction<T>(failureMessage: string, action: () => LayoutOperationResult<T>): LayoutOperationResult<T> {
  try {
    return action()
  } catch (error) {
    return createFailureResult(error as Error, failureMessage)
  }
}

function commitLayoutMutation<T>(context: LayoutActionContext, mutation: () => T): T {
  context.saveToHistory()
  const result = mutation()
  context.autoSave()

  return result
}

function findLayoutItem(layout: Ref<GridLayoutPlusItem[]>, itemId: string): GridLayoutPlusItem | undefined {
  return layout.value.find((item) => item.i === itemId)
}

function findLayoutItemIndex(layout: Ref<GridLayoutPlusItem[]>, itemId: string): number {
  return layout.value.findIndex((item) => item.i === itemId)
}

function createNewLayoutItem(
  state: GridLayoutPlusState,
  type: string,
  itemOptions?: Partial<GridLayoutPlusItem>
): GridLayoutPlusItem {
  const w = itemOptions?.w || DEFAULT_LAYOUT_ITEM_SIZE
  const h = itemOptions?.h || DEFAULT_LAYOUT_ITEM_SIZE
  const position = findAvailablePosition(w, h, state.layout.value, state.config.value.colNum)

  return {
    i: generateId(),
    x: position.x,
    y: position.y,
    w,
    h,
    type,
    ...itemOptions
  }
}

function createDuplicatedLayoutItem(state: GridLayoutPlusState, sourceItem: GridLayoutPlusItem): GridLayoutPlusItem {
  const position = findAvailablePosition(sourceItem.w, sourceItem.h, state.layout.value, state.config.value.colNum)

  return {
    ...sourceItem,
    i: generateId(),
    x: position.x,
    y: position.y
  }
}

function removeLayoutItemAtIndex(state: GridLayoutPlusState, index: number): GridLayoutPlusItem {
  const [removedItem] = state.layout.value.splice(index, 1)
  state.selectedItems.value = state.selectedItems.value.filter((id) => id !== removedItem.i)

  return removedItem
}

function createAddItemAction(context: LayoutActionContext) {
  return (type: string, itemOptions?: Partial<GridLayoutPlusItem>): LayoutOperationResult<GridLayoutPlusItem> => {
    return runLayoutAction<GridLayoutPlusItem>('项目添加失败', () => {
      const newItem = createNewLayoutItem(context.state, type, itemOptions)
      const validation = validateGridItem(newItem)

      if (!validation.success) {
        return preserveFailedResult(validation)
      }

      commitLayoutMutation(context, () => context.state.layout.value.push(newItem))

      return createSuccessResult(newItem, '项目添加成功')
    })
  }
}

function createRemoveItemAction(context: LayoutActionContext) {
  return (itemId: string): LayoutOperationResult<GridLayoutPlusItem> => {
    return runLayoutAction('项目删除失败', () => {
      const index = findLayoutItemIndex(context.state.layout, itemId)

      if (index === -1) {
        return createItemNotFoundResult('项目不存在')
      }

      const removedItem = commitLayoutMutation(context, () => removeLayoutItemAtIndex(context.state, index))

      return createSuccessResult(removedItem, '项目删除成功')
    })
  }
}

function createUpdateItemAction(context: LayoutActionContext) {
  return (itemId: string, updates: Partial<GridLayoutPlusItem>): LayoutOperationResult<GridLayoutPlusItem> => {
    return runLayoutAction<GridLayoutPlusItem>('项目更新失败', () => {
      const item = findLayoutItem(context.state.layout, itemId)

      if (!item) {
        return createItemNotFoundResult('项目不存在')
      }

      context.saveToHistory()
      Object.assign(item, updates)

      const validation = validateGridItem(item)

      if (!validation.success) {
        return preserveFailedResult(validation)
      }

      context.autoSave()

      return createSuccessResult(item, '项目更新成功')
    })
  }
}

function createDuplicateItemAction(context: LayoutActionContext) {
  return (itemId: string): LayoutOperationResult<GridLayoutPlusItem> => {
    return runLayoutAction('项目复制失败', () => {
      const sourceItem = findLayoutItem(context.state.layout, itemId)

      if (!sourceItem) {
        return createItemNotFoundResult('源项目不存在', 'Source item not found')
      }

      const duplicatedItem = createDuplicatedLayoutItem(context.state, sourceItem)
      commitLayoutMutation(context, () => context.state.layout.value.push(duplicatedItem))

      return createSuccessResult(duplicatedItem, '项目复制成功')
    })
  }
}

function createClearLayoutAction(context: LayoutActionContext) {
  return (): LayoutOperationResult<boolean> => {
    return runLayoutAction('布局清空失败', () => {
      commitLayoutMutation(context, () => {
        context.state.layout.value = []
        context.state.selectedItems.value = []
      })

      return createSuccessResult(true, '布局清空成功')
    })
  }
}

function createLayoutActions(state: GridLayoutPlusState, saveToHistory: SaveToHistory, autoSave: AutoSave) {
  const context = createLayoutActionContext(state, saveToHistory, autoSave)

  return {
    addItem: createAddItemAction(context),
    removeItem: createRemoveItemAction(context),
    updateItem: createUpdateItemAction(context),
    duplicateItem: createDuplicateItemAction(context),
    clearLayout: createClearLayoutAction(context)
  }
}

function createSelectionActions(layout: Ref<GridLayoutPlusItem[]>, selectedItems: Ref<string[]>) {
  const selectItem = (itemId: string) => {
    if (!selectedItems.value.includes(itemId)) {
      selectedItems.value.push(itemId)
    }
  }

  const deselectItem = (itemId: string) => {
    selectedItems.value = selectedItems.value.filter((id) => id !== itemId)
  }

  const selectMultipleItems = (itemIds: string[]) => {
    selectedItems.value = [...new Set([...selectedItems.value, ...itemIds])]
  }

  const selectAllItems = () => {
    selectedItems.value = layout.value.map((item) => item.i)
  }

  const clearSelection = () => {
    selectedItems.value = []
  }

  const toggleItemSelection = (itemId: string) => {
    if (selectedItems.value.includes(itemId)) {
      deselectItem(itemId)
    } else {
      selectItem(itemId)
    }
  }

  return {
    selectItem,
    deselectItem,
    selectMultipleItems,
    selectAllItems,
    clearSelection,
    toggleItemSelection
  }
}

function createBatchActions(
  state: GridLayoutPlusState,
  duplicateItem: (itemId: string) => LayoutOperationResult<GridLayoutPlusItem>,
  saveToHistory: SaveToHistory,
  autoSave: AutoSave
) {
  const deleteSelectedItems = (): LayoutOperationResult<string[]> => {
    try {
      if (state.selectedItems.value.length === 0) {
        return {
          success: false,
          error: new Error('No items selected'),
          message: '没有选中的项目'
        }
      }

      saveToHistory()
      const deletedIds = [...state.selectedItems.value]
      state.layout.value = state.layout.value.filter((item) => !state.selectedItems.value.includes(item.i))
      state.selectedItems.value = []
      autoSave()

      return {
        success: true,
        data: deletedIds,
        message: `删除了 ${deletedIds.length} 个项目`
      }
    } catch (error) {
      return {
        success: false,
        error: error as Error,
        message: '批量删除失败'
      }
    }
  }

  const duplicateSelectedItems = (): LayoutOperationResult<GridLayoutPlusItem[]> => {
    try {
      if (state.selectedItems.value.length === 0) {
        return {
          success: false,
          error: new Error('No items selected'),
          message: '没有选中的项目'
        }
      }

      saveToHistory()
      const duplicatedItems: GridLayoutPlusItem[] = []

      for (const itemId of state.selectedItems.value) {
        const result = duplicateItem(itemId)
        if (result.success && result.data) {
          duplicatedItems.push(result.data)
        }
      }

      return {
        success: true,
        data: duplicatedItems,
        message: `复制了 ${duplicatedItems.length} 个项目`
      }
    } catch (error) {
      return {
        success: false,
        error: error as Error,
        message: '批量复制失败'
      }
    }
  }

  return {
    deleteSelectedItems,
    duplicateSelectedItems
  }
}

function createLayoutTools(layout: Ref<GridLayoutPlusItem[]>, saveToHistory: SaveToHistory, autoSave: AutoSave) {
  const compactCurrentLayout = () => {
    saveToHistory()
    layout.value = compactLayout(layout.value)
    autoSave()
  }

  const sortCurrentLayout = (sortBy: 'position' | 'size' | 'id' = 'position') => {
    saveToHistory()
    layout.value = sortLayout(layout.value, sortBy)
    autoSave()
  }

  const searchItems = (query: string) => {
    return searchLayout(layout.value, query)
  }

  const filterItems = (predicate: (item: GridLayoutPlusItem) => boolean) => {
    return filterLayout(layout.value, predicate)
  }

  return {
    compactCurrentLayout,
    sortCurrentLayout,
    searchItems,
    filterItems
  }
}

function createResponsiveActions(state: GridLayoutPlusState) {
  const setBreakpoint = (breakpoint: string) => {
    state.currentBreakpoint.value = breakpoint
  }

  const createResponsiveLayoutForAll = () => {
    state.responsiveLayouts.value = createResponsiveLayout(
      state.layout.value,
      state.config.value.breakpoints,
      state.config.value.cols
    )
  }

  const getLayoutForBreakpoint = (breakpoint: string): GridLayoutPlusItem[] => {
    const breakpointLayout = state.responsiveLayouts.value[breakpoint as keyof ResponsiveLayout]
    return breakpointLayout || state.layout.value
  }

  return {
    setBreakpoint,
    createResponsiveLayoutForAll,
    getLayoutForBreakpoint
  }
}

function createImportExportActions(
  layout: Ref<GridLayoutPlusItem[]>,
  selectedItems: Ref<string[]>,
  saveToHistory: SaveToHistory,
  autoSave: AutoSave
) {
  const exportCurrentLayout = (format: 'json' | 'csv' = 'json'): string => {
    return JSON.stringify(layout.value, null, 2)
  }

  const importLayout = (data: string): LayoutOperationResult<boolean> => {
    try {
      const importedLayout = JSON.parse(data) as GridLayoutPlusItem[]
      const validation = validateLayout(importedLayout)

      if (!validation.success) {
        return validation
      }

      saveToHistory()
      layout.value = importedLayout
      selectedItems.value = []
      autoSave()

      return {
        success: true,
        data: true,
        message: '布局导入成功'
      }
    } catch (error) {
      return {
        success: false,
        error: error as Error,
        message: '布局导入失败'
      }
    }
  }

  return {
    exportCurrentLayout,
    importLayout
  }
}

function createItemQueries(layout: Ref<GridLayoutPlusItem[]>, selectedItems: Ref<string[]>) {
  const getItem = (itemId: string): GridLayoutPlusItem | undefined => {
    return layout.value.find((item) => item.i === itemId)
  }

  const hasItem = (itemId: string): boolean => {
    return layout.value.some((item) => item.i === itemId)
  }

  const getSelectedItems = (): GridLayoutPlusItem[] => {
    return layout.value.filter((item) => selectedItems.value.includes(item.i))
  }

  return {
    getItem,
    hasItem,
    getSelectedItems
  }
}

function createThrottledLayoutUpdate(
  layout: Ref<GridLayoutPlusItem[]>,
  performanceConfig: Ref<PerformanceConfig>,
  autoSave: AutoSave
) {
  return throttle((newLayout: GridLayoutPlusItem[]) => {
    layout.value = newLayout
    autoSave()
  }, performanceConfig.value.throttleDelay)
}

function watchConfigValidation(state: GridLayoutPlusState) {
  watch(
    () => state.config.value,
    () => {
      const validation = validateLayout(state.layout.value)
      state.error.value = validation.success ? null : validation.error || null
    },
    { deep: true }
  )
}

function initializeHistory(
  options: UseGridLayoutPlusOptions,
  layout: Ref<GridLayoutPlusItem[]>,
  saveToHistory: SaveToHistory
) {
  if (options.enableHistory && layout.value.length > 0) {
    saveToHistory()
  }
}

export function useGridLayoutPlus(options: UseGridLayoutPlusOptions = {}) {
  const state = createGridLayoutPlusState(options)
  const computedState = createGridLayoutComputedState(state)
  const historyActions = createHistoryController(options, state, computedState.canUndo, computedState.canRedo)
  const autoSave = createAutoSave(options, state.layout)
  const layoutActions = createLayoutActions(state, historyActions.saveToHistory, autoSave)
  const selectionActions = createSelectionActions(state.layout, state.selectedItems)
  const batchActions = createBatchActions(state, layoutActions.duplicateItem, historyActions.saveToHistory, autoSave)
  const layoutTools = createLayoutTools(state.layout, historyActions.saveToHistory, autoSave)
  const responsiveActions = createResponsiveActions(state)
  const importExportActions = createImportExportActions(
    state.layout,
    state.selectedItems,
    historyActions.saveToHistory,
    autoSave
  )
  const itemQueries = createItemQueries(state.layout, state.selectedItems)
  const throttledLayoutUpdate = createThrottledLayoutUpdate(state.layout, state.performanceConfig, autoSave)

  watchConfigValidation(state)
  initializeHistory(options, state.layout, historyActions.saveToHistory)

  return {
    ...state,
    ...computedState,
    ...layoutActions,
    ...selectionActions,
    ...batchActions,
    ...layoutTools,
    ...historyActions,
    ...responsiveActions,
    ...importExportActions,
    ...itemQueries,
    throttledLayoutUpdate
  }
}
