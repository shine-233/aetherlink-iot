/**
 * 文件用途：为网格布局提供撤销、重做、跳转和自动保存历史能力。
 * 核心逻辑：记录克隆后的布局快照，裁剪重做分支和容量上限，并按游标恢复快照。
 * 关键注意事项：历史快照是业务状态检查点，不能共享可变布局对象。
 * 重构建议：拖拽或缩放流程需要合并多次更新时，可增加事务式批处理能力。
 */

import { computed, ref, type Ref } from 'vue'
import type { GridLayoutPlusItem } from '../gridLayoutPlusTypes'
import { cloneLayout } from '../utils/common'
import { createLogger } from '../../../../utils/logger'

const gridHistoryLogger = createLogger('GridHistory')

type GridLayoutSnapshot = GridLayoutPlusItem[]
type LayoutRef = { value: GridLayoutPlusItem[] }
type AutoSaveTimer = ReturnType<typeof setInterval> | null

export interface UseGridHistoryOptions {
  /** Whether history recording is enabled. */
  enabled?: boolean
  /** Maximum number of stored history snapshots. */
  maxLength?: number
  /** Auto-save interval in milliseconds; 0 disables auto-save. */
  autoSaveInterval?: number
}

interface GridHistoryState {
  history: Ref<GridLayoutSnapshot[]>
  historyIndex: Ref<number>
  isRecording: Ref<boolean>
}

interface NormalizedGridHistoryOptions {
  enabled: boolean
  maxLength: number
  autoSaveInterval: number
}

interface HistoryCursorRestoreOptions {
  failureMessage: string
  debugMessage: (index: number) => string
}

function normalizeHistoryOptions(options: UseGridHistoryOptions): NormalizedGridHistoryOptions {
  return {
    enabled: options.enabled ?? true,
    maxLength: options.maxLength ?? 50,
    autoSaveInterval: options.autoSaveInterval ?? 0
  }
}

function createGridHistoryState(): GridHistoryState {
  return {
    history: ref<GridLayoutSnapshot[]>([]),
    historyIndex: ref(-1),
    isRecording: ref(true)
  }
}

function createSnapshot(layout: GridLayoutPlusItem[]): GridLayoutSnapshot {
  return cloneLayout(layout)
}

function serializeSnapshot(snapshot: GridLayoutSnapshot): string {
  return JSON.stringify(snapshot)
}

function hasSameSnapshot(left: GridLayoutSnapshot, right: GridLayoutSnapshot): boolean {
  return serializeSnapshot(left) === serializeSnapshot(right)
}

function shouldRecordSnapshot(
  state: GridHistoryState,
  options: NormalizedGridHistoryOptions,
  layout: GridLayoutPlusItem[]
): boolean {
  return options.enabled && state.isRecording.value && layout.length > 0
}

function isDuplicateCurrentSnapshot(state: GridHistoryState, snapshot: GridLayoutSnapshot): boolean {
  const currentSnapshot = state.history.value[state.historyIndex.value]

  return Boolean(currentSnapshot && hasSameSnapshot(currentSnapshot, snapshot))
}

function discardRedoBranch(state: GridHistoryState): void {
  if (state.historyIndex.value < state.history.value.length - 1) {
    state.history.value = state.history.value.slice(0, state.historyIndex.value + 1)
  }
}

function trimHistoryToCap(state: GridHistoryState, maxLength: number): void {
  if (maxLength <= 0) {
    state.history.value = []
    state.historyIndex.value = -1
    return
  }

  const excessSnapshotCount = state.history.value.length - maxLength
  if (excessSnapshotCount > 0) {
    state.history.value = state.history.value.slice(excessSnapshotCount)
  }

  state.historyIndex.value = state.history.value.length - 1
}

function pushHistorySnapshot(state: GridHistoryState, snapshot: GridLayoutSnapshot, maxLength: number): void {
  discardRedoBranch(state)

  state.history.value.push(snapshot)
  state.historyIndex.value = state.history.value.length - 1
  trimHistoryToCap(state, maxLength)
}

function restoreSnapshot(state: GridHistoryState, index: number): GridLayoutPlusItem[] {
  state.historyIndex.value = index
  return cloneLayout(state.history.value[index])
}

function restoreHistoryIndex(
  state: GridHistoryState,
  index: number,
  options: HistoryCursorRestoreOptions
): GridLayoutPlusItem[] | null {
  try {
    const layout = restoreSnapshot(state, index)
    gridHistoryLogger.debug(options.debugMessage(index))
    return layout
  } catch (err) {
    console.error(options.failureMessage, err)
    return null
  }
}

function createAutoSaveControls(
  state: GridHistoryState,
  options: NormalizedGridHistoryOptions,
  saveToHistory: (layout: GridLayoutPlusItem[]) => void
) {
  let autoSaveTimer: AutoSaveTimer = null

  const stopAutoSave = () => {
    if (autoSaveTimer) {
      clearInterval(autoSaveTimer)
      autoSaveTimer = null
      gridHistoryLogger.debug('Auto save stopped')
    }
  }

  const startAutoSave = (layoutRef: LayoutRef) => {
    if (options.autoSaveInterval <= 0 || !options.enabled) return

    stopAutoSave()

    autoSaveTimer = setInterval(() => {
      if (state.isRecording.value && layoutRef.value.length > 0) {
        saveToHistory(layoutRef.value)
      }
    }, options.autoSaveInterval)

    gridHistoryLogger.debug(`Auto save started with ${options.autoSaveInterval}ms interval`)
  }

  return {
    startAutoSave,
    stopAutoSave
  }
}

/**
 * Grid history management hook.
 * Provides undo/redo operations and snapshot lifecycle management.
 */
export function useGridHistory(options: UseGridHistoryOptions = {}) {
  const normalizedOptions = normalizeHistoryOptions(options)
  const state = createGridHistoryState()
  const { history, historyIndex, isRecording } = state

  const canUndo = computed(() => normalizedOptions.enabled && historyIndex.value > 0)
  const canRedo = computed(() => normalizedOptions.enabled && historyIndex.value < history.value.length - 1)
  const historyLength = computed(() => history.value.length)
  const currentHistoryIndex = computed(() => historyIndex.value)

  const saveToHistory = (layout: GridLayoutPlusItem[]) => {
    if (!shouldRecordSnapshot(state, normalizedOptions, layout)) return

    try {
      const snapshot = createSnapshot(layout)

      if (history.value.length > 0 && isDuplicateCurrentSnapshot(state, snapshot)) {
        return
      }

      pushHistorySnapshot(state, snapshot, normalizedOptions.maxLength)

      gridHistoryLogger.debug(`Saved to history. Index: ${historyIndex.value}, Total: ${history.value.length}`)
    } catch (err) {
      console.error('[GridHistory] Failed to save to history:', err)
    }
  }

  const undo = (): GridLayoutPlusItem[] | null => {
    if (!canUndo.value) {
      console.error('[GridHistory] Cannot undo: no previous state available')
      return null
    }

    return restoreHistoryIndex(state, historyIndex.value - 1, {
      failureMessage: '[GridHistory] Failed to undo:',
      debugMessage: (index) => `Undo to index: ${index}`
    })
  }

  const redo = (): GridLayoutPlusItem[] | null => {
    if (!canRedo.value) {
      console.error('[GridHistory] Cannot redo: no next state available')
      return null
    }

    return restoreHistoryIndex(state, historyIndex.value + 1, {
      failureMessage: '[GridHistory] Failed to redo:',
      debugMessage: (index) => `Redo to index: ${index}`
    })
  }

  const jumpToHistory = (index: number): GridLayoutPlusItem[] | null => {
    if (!normalizedOptions.enabled || index < 0 || index >= history.value.length) {
      console.error(`[GridHistory] Invalid history index: ${index}`)
      return null
    }

    return restoreHistoryIndex(state, index, {
      failureMessage: '[GridHistory] Failed to jump to history:',
      debugMessage: (restoredIndex) => `Jump to index: ${restoredIndex}`
    })
  }

  const getHistorySummary = () => {
    return history.value.map((layout, index) => ({
      index,
      timestamp: Date.now(),
      itemCount: layout.length,
      isCurrent: index === historyIndex.value
    }))
  }

  const clearHistory = () => {
    history.value = []
    historyIndex.value = -1
    gridHistoryLogger.debug('History cleared')
  }

  const pauseRecording = () => {
    isRecording.value = false
    gridHistoryLogger.debug('Recording paused')
  }

  const resumeRecording = () => {
    isRecording.value = true
    gridHistoryLogger.debug('Recording resumed')
  }

  const initHistory = (initialLayout: GridLayoutPlusItem[]) => {
    if (!normalizedOptions.enabled || initialLayout.length === 0) return

    history.value = [createSnapshot(initialLayout)]
    historyIndex.value = 0
    gridHistoryLogger.debug('History initialized')
  }

  const { startAutoSave, stopAutoSave } = createAutoSaveControls(state, normalizedOptions, saveToHistory)

  return {
    canUndo,
    canRedo,
    historyLength,
    currentHistoryIndex,
    isRecording,

    saveToHistory,
    undo,
    redo,
    jumpToHistory,
    getHistorySummary,
    clearHistory,
    pauseRecording,
    resumeRecording,
    startAutoSave,
    stopAutoSave,
    initHistory
  }
}
