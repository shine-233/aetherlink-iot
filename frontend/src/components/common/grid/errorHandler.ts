/**
 * 文件用途：提供网格模块统一错误类型、默认错误处理器和安全执行包装。
 * 核心逻辑：收集最近错误/警告，按可恢复标记决定是否抛出，并在测试环境抑制未 mock 的 console 噪声。
 * 关键注意事项：不可恢复错误会直接抛出，调用方需要避免把用户可修复输入误标为 fatal。
 * 重构建议：后续可引入错误上报适配器，把 UI 提示、日志采集和测试抑制策略解耦。
 */
import { shouldSuppressUnmockedTestConsole } from '@/utils/test-console'
import type { GridConfig, GridItem } from './types'

export enum GridErrorType {
  VALIDATION_ERROR = 'VALIDATION_ERROR',
  RENDER_ERROR = 'RENDER_ERROR',
  LAYOUT_ERROR = 'LAYOUT_ERROR',
  PERFORMANCE_ERROR = 'PERFORMANCE_ERROR',
  UNKNOWN_ERROR = 'UNKNOWN_ERROR'
}

export interface GridOperationResult<T = unknown> {
  success: boolean
  data?: T
  error?: Error
  message?: string
}

export class GridError extends Error {
  constructor(
    message: string,
    public type: GridErrorType = GridErrorType.UNKNOWN_ERROR,
    public context?: unknown,
    public recoverable = true
  ) {
    super(message)
    this.name = 'GridError'
  }
}

export class DefaultErrorHandler {
  private errors: GridError[] = []
  private warnings: string[] = []

  onError(error: GridError) {
    if (!shouldSuppressUnmockedTestConsole(console.error)) {
      console.error(`[Grid Error] ${error.message}`, error.context)
    }
    this.errors.push(error)
    if (this.errors.length > 100) this.errors.splice(0, this.errors.length - 100)
    if (!error.recoverable) throw error
  }

  onWarning(message: string, context?: unknown) {
    if (!shouldSuppressUnmockedTestConsole(console.warn)) {
      console.warn(`[Grid Warning] ${message}`, context)
    }
    this.warnings.push(message)
    if (this.warnings.length > 100) this.warnings.splice(0, this.warnings.length - 100)
  }

  getErrors() {
    return this.errors
  }

  getWarnings() {
    return this.warnings
  }

  clearErrors() {
    this.errors = []
    this.warnings = []
  }
}

export const gridErrorHandler = new DefaultErrorHandler()

export function safeExecute<T>(fn: () => T, message: string): GridOperationResult<T> {
  try {
    return { success: true, data: fn() }
  } catch (error) {
    return {
      success: false,
      error: error instanceof GridError ? error : new GridError(error instanceof Error ? error.message : String(error)),
      message
    }
  }
}

export async function safeExecuteAsync<T>(fn: () => Promise<T>, message: string): Promise<GridOperationResult<T>> {
  try {
    return { success: true, data: await fn() }
  } catch (error) {
    return {
      success: false,
      error: error instanceof GridError ? error : new GridError(error instanceof Error ? error.message : String(error)),
      message
    }
  }
}

export function validateGridConfig(config: Partial<GridConfig>): GridOperationResult<boolean> {
  const columns = Number(config.columns ?? config.colNum ?? 0)
  const rowHeight = Number(config.rowHeight ?? 0)
  const gap = Number(config.gap ?? 0)
  const minRows = Number(config.minRows ?? 0)
  const maxRows = config.maxRows === undefined ? undefined : Number(config.maxRows)

  if (columns <= 0 || rowHeight <= 0 || gap < 0 || (maxRows !== undefined && minRows > maxRows)) {
    return {
      success: false,
      error: new GridError('Invalid grid config', GridErrorType.VALIDATION_ERROR),
      message: '配置验证失败'
    }
  }

  return { success: true, data: true }
}

export function validateGridItems(items: GridItem[]): GridOperationResult<boolean> {
  const ids = new Set<string>()

  for (const item of items) {
    const id = item.id ?? item.i
    if (!id || ids.has(id)) {
      return {
        success: false,
        error: new GridError('Invalid grid item id', GridErrorType.VALIDATION_ERROR),
        message: 'ID验证失败'
      }
    }
    ids.add(id)

    const col = item.gridCol ?? (item.x ?? 0) + 1
    const row = item.gridRow ?? (item.y ?? 0) + 1
    const colSpan = item.gridColSpan ?? item.w ?? 1
    const rowSpan = item.gridRowSpan ?? item.h ?? 1

    if (col < 1 || row < 1 || colSpan < 1 || rowSpan < 1) {
      return {
        success: false,
        error: new GridError('Invalid grid item position', GridErrorType.VALIDATION_ERROR),
        message: '位置验证失败'
      }
    }

    if (
      (item.minColSpan !== undefined && colSpan < item.minColSpan) ||
      (item.maxColSpan !== undefined && colSpan > item.maxColSpan) ||
      (item.minRowSpan !== undefined && rowSpan < item.minRowSpan) ||
      (item.maxRowSpan !== undefined && rowSpan > item.maxRowSpan)
    ) {
      return {
        success: false,
        error: new GridError('Invalid grid item constraints', GridErrorType.VALIDATION_ERROR),
        message: '约束验证失败'
      }
    }
  }

  return { success: true, data: true }
}

export function withPerformanceMonitor<T extends (...args: any[]) => any>(
  fn: T,
  label = 'grid-operation',
  threshold = 100
): T {
  return ((...args: Parameters<T>) => {
    const start = globalThis.performance?.now ? globalThis.performance.now() : Date.now()
    const finish = () => {
      const end = globalThis.performance?.now ? globalThis.performance.now() : Date.now()
      const duration = end - start
      if (duration > threshold) {
        gridErrorHandler.onWarning(`${label} took ${duration.toFixed(2)}ms`, { duration, threshold, label })
      }
    }

    try {
      const result = fn(...args)

      if (result && typeof result.then === 'function') {
        return result
          .then((value: unknown) => {
            finish()
            return value
          })
          .catch((error: unknown) => {
            gridErrorHandler.onError(
              new GridError(error instanceof Error ? error.message : String(error), GridErrorType.PERFORMANCE_ERROR, {
                label
              })
            )
            throw error
          })
      }

      finish()
      return result
    } catch (error) {
      gridErrorHandler.onError(
        new GridError(error instanceof Error ? error.message : String(error), GridErrorType.PERFORMANCE_ERROR, {
          label
        })
      )
      throw error
    }
  }) as T
}
