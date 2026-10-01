/**
 * 文件用途：提供"页面可见时才轮询"的通用定时器。
 * 核心逻辑：document.hidden 时清掉 setInterval，不再发请求；
 *   页面重新可见时立即补拉一次再恢复定时，避免后台标签页持续打后端。
 * 关键注意事项：只在 start() 之后才挂 visibilitychange 监听；stop() 与作用域销毁都会解绑。
 */
import { getCurrentScope, onScopeDispose } from 'vue'

export interface VisibleIntervalOptions {
  /** 页面恢复可见时是否立即执行一次（默认 true，补齐后台期间错过的数据）。 */
  runOnVisible?: boolean
}

export interface VisibleIntervalHandle {
  start: () => void
  stop: () => void
  isActive: () => boolean
}

function isDocumentHidden() {
  return typeof document !== 'undefined' && document.hidden
}

export function createVisibleInterval(
  task: () => void,
  intervalMs: number,
  options: VisibleIntervalOptions = {}
): VisibleIntervalHandle {
  const runOnVisible = options.runOnVisible ?? true
  let timer: ReturnType<typeof setInterval> | null = null
  let active = false

  const clearTimer = () => {
    if (timer !== null) {
      clearInterval(timer)
      timer = null
    }
  }

  const armTimer = () => {
    if (timer !== null || !active || isDocumentHidden()) return
    timer = setInterval(task, intervalMs)
  }

  const handleVisibilityChange = () => {
    if (!active) return
    if (isDocumentHidden()) {
      clearTimer()
      return
    }
    if (runOnVisible) task()
    armTimer()
  }

  const start = () => {
    if (active) return
    active = true
    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', handleVisibilityChange)
    }
    armTimer()
  }

  const stop = () => {
    if (!active) return
    active = false
    clearTimer()
    if (typeof document !== 'undefined') {
      document.removeEventListener('visibilitychange', handleVisibilityChange)
    }
  }

  if (getCurrentScope()) onScopeDispose(stop)

  return { start, stop, isActive: () => active }
}
