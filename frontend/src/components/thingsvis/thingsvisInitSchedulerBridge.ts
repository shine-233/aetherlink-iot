/**
 * 文件说明：
 * - 封装 ThingsVis iframe 初始化调度状态机，集中处理 tv:ready 防抖、重复签名跳过、初始化并发保护和指数退避重试。
 * - AppFrame 只需要注入“是否可初始化”“当前签名”“执行初始化”三个能力，具体加载 dashboard、postMessage 和 viewer/editor 分流仍留在宿主入口。
 * 维护提示：
 * - `tv:ready` 与 `READY` 可能连续触发，调度器必须避免并发初始化，否则 editor/viewer bootstrap 可能互相打断。
 * - dashboard payload 暂不可用时不能标记完成，必须保留退避重试，直到 iframe、token、id 任一条件失效或初始化成功。
 * 审查建议：
 * - 后续若补单元测试，应直接测试本模块接口：重复 ready、成功去重、失败退避、iframe load reset 和 dispose 清理。
 */
type InitSchedulerOptions = {
  canInit: () => boolean
  getSignature: () => string
  runInit: () => Promise<boolean>
  debounceDelay?: number
  retryBaseDelay?: number
  retryMaxDelay?: number
}

export type ThingsVisInitScheduler = {
  schedule: (delay?: number) => void
  invalidate: () => void
  resetAfterFrameLoad: () => void
  dispose: () => void
}

export function createThingsVisInitScheduler(options: InitSchedulerOptions): ThingsVisInitScheduler {
  const debounceDelay = options.debounceDelay ?? 150
  const retryBaseDelay = options.retryBaseDelay ?? 400
  const retryMaxDelay = options.retryMaxDelay ?? 4000

  let initInProgress = false
  let pendingInitDebounceTimer: ReturnType<typeof setTimeout> | null = null
  let pendingInitRetryTimer: ReturnType<typeof setTimeout> | null = null
  let lastInitCompletedSignature = ''
  let initRetryAttempt = 0

  function clearDebounceTimer() {
    if (!pendingInitDebounceTimer) return
    clearTimeout(pendingInitDebounceTimer)
    pendingInitDebounceTimer = null
  }

  function clearRetryTimer() {
    if (!pendingInitRetryTimer) return
    clearTimeout(pendingInitRetryTimer)
    pendingInitRetryTimer = null
  }

  function markInitCompleted(initSignature: string) {
    lastInitCompletedSignature = initSignature
    initRetryAttempt = 0
    clearRetryTimer()
  }

  function invalidate() {
    clearDebounceTimer()
    clearRetryTimer()
    initInProgress = false
    lastInitCompletedSignature = ''
    initRetryAttempt = 0
  }

  function schedule(delay = debounceDelay) {
    if (!options.canInit()) return

    const scheduledSignature = options.getSignature()
    if (!initInProgress && scheduledSignature === lastInitCompletedSignature) {
      return
    }

    clearDebounceTimer()

    const runScheduledInit = async () => {
      if (initInProgress) return
      if (!options.canInit()) return

      const runSignature = options.getSignature()
      if (runSignature === lastInitCompletedSignature) return

      initInProgress = true
      try {
        const initialized = await options.runInit()
        if (initialized) {
          markInitCompleted(runSignature)
          return
        }

        if (!options.canInit()) return

        const retryDelay = Math.min(retryMaxDelay, retryBaseDelay * 2 ** initRetryAttempt)
        initRetryAttempt += 1
        clearRetryTimer()
        pendingInitRetryTimer = setTimeout(() => {
          pendingInitRetryTimer = null
          schedule(retryDelay)
        }, retryDelay)
      } finally {
        initInProgress = false
      }
    }

    pendingInitDebounceTimer = setTimeout(() => {
      pendingInitDebounceTimer = null
      void runScheduledInit()
    }, delay)
  }

  function resetAfterFrameLoad() {
    invalidate()
  }

  function dispose() {
    invalidate()
  }

  return {
    schedule,
    invalidate,
    resetAfterFrameLoad,
    dispose
  }
}
