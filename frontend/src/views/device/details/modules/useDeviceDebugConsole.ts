/**
 * 文件用途：设备调试控制台的状态层：调试开关读写 + 调试日志轮询。
 * 核心逻辑：
 * 1. 开关状态以服务端为准（`getDeviceDebugStatus`），切换成功后才落本地，失败回滚；
 * 2. 日志轮询改为 setTimeout 链：上一轮结束才排下一轮，不会叠加慢请求；
 * 3. 节奏自适应：调试开启 3s、关闭 15s；连续失败指数退避（上限 60s）；页面隐藏时暂停，
 *    恢复可见立即补拉一次；作用域销毁（组件卸载）时释放定时器与事件监听。
 * 关键注意事项：
 * 1. `request` 是 flat 实例，HTTP 失败通过 `{ error }` 返回，这里与抛错同等视为失败；
 * 2. 切换设备会递增 generation，丢弃旧设备在途请求的结果，避免旧日志回写；
 * 3. 日志签名未变化时不替换数组，避免每 3 秒全量重渲染 100 行控制台。
 */
import { computed, getCurrentScope, onScopeDispose, ref, shallowRef, toValue, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import dayjs from 'dayjs'

import { getDeviceDebugLogs, getDeviceDebugStatus, setDeviceDebugStatus } from '@/service/api'
import { maskSensitiveDiagnosticText, sanitizeDiagnosticValue } from './useDeviceDiagnosticsStats'
import type { DebugLogEntry } from './useDeviceDiagnosticsStats'

export const DEBUG_LOG_FETCH_LIMIT = 100
export const DEBUG_LOG_POLL_INTERVAL_MS = 3000
export const DEBUG_LOG_IDLE_POLL_INTERVAL_MS = 15000
export const DEBUG_LOG_MAX_BACKOFF_MS = 60000

export interface DebugLogPollDelayInput {
  enabled: boolean
  consecutiveFailures: number
  activeIntervalMs?: number
  idleIntervalMs?: number
  maxBackoffMs?: number
}

/** 下一轮轮询延迟：基础间隔 × 2^连续失败次数，封顶 maxBackoffMs（但不低于基础间隔）。 */
export const computeDebugLogPollDelay = ({
  enabled,
  consecutiveFailures,
  activeIntervalMs = DEBUG_LOG_POLL_INTERVAL_MS,
  idleIntervalMs = DEBUG_LOG_IDLE_POLL_INTERVAL_MS,
  maxBackoffMs = DEBUG_LOG_MAX_BACKOFF_MS
}: DebugLogPollDelayInput) => {
  const base = enabled ? activeIntervalMs : idleIntervalMs
  if (consecutiveFailures <= 0) return base
  const backoff = base * 2 ** Math.min(consecutiveFailures, 16)
  return Math.max(base, Math.min(backoff, maxBackoffMs))
}

// 单条日志对象脱敏后 JSON 化输出到类终端窗口，便于原样排障。
export const formatDebugLogEntry = (item: DebugLogEntry) => {
  const time = item.ts ? dayjs(item.ts).format('YYYY-MM-DD HH:mm:ss.SSS') : ''
  return `[${time}] ${maskSensitiveDiagnosticText(JSON.stringify(sanitizeDiagnosticValue(item)))}`
}

// 接口按时间倒序返回，控制台按正序（最新在底部）展示。
export const mapDebugLogsForConsole = (items: DebugLogEntry[] = []) => [...items].reverse().map(formatDebugLogEntry)

/** 轻量签名：数量 + 首尾条目。日志只追加/滚动，首尾不变即可认为内容未变。 */
export const debugLogsSignature = (items: DebugLogEntry[]) => {
  if (items.length === 0) return '0'
  const edge = (item: DebugLogEntry | undefined) => (item ? JSON.stringify(item) : '')
  return `${items.length}|${edge(items[0])}|${edge(items[items.length - 1])}`
}

type FlatResult<T> = { data?: T | null; error?: unknown } | null | undefined

const unwrapFlat = <T>(res: FlatResult<T>): T | null | undefined => {
  if (res && typeof res === 'object' && 'error' in res && res.error) throw res.error
  return res?.data
}

export interface UseDeviceDebugConsoleOptions {
  /** 默认 true：创建即读取开关状态并开始轮询。 */
  autoStart?: boolean
  fetchLimit?: number
  activeIntervalMs?: number
  idleIntervalMs?: number
  maxBackoffMs?: number
}

export const useDeviceDebugConsole = (
  deviceId: MaybeRefOrGetter<string>,
  options: UseDeviceDebugConsoleOptions = {}
) => {
  const fetchLimit = options.fetchLimit ?? DEBUG_LOG_FETCH_LIMIT

  const logEnabled = ref(false)
  const logSwitching = ref(false)
  const statusError = shallowRef<unknown>(null)
  const debugLogEntries = shallowRef<DebugLogEntry[]>([])
  const debugLogs = shallowRef<string[]>([])
  const logsError = shallowRef<unknown>(null)
  const consecutiveFailures = ref(0)
  const polling = ref(false)
  const documentHidden = ref(typeof document !== 'undefined' && document.visibilityState === 'hidden')

  // 每次切换设备递增；在途请求返回时比对，不一致即丢弃。
  let generation = 0
  let timer: ReturnType<typeof setTimeout> | null = null
  let inFlight: Promise<void> | null = null
  let lastSignature = ''

  const nextPollDelay = computed(() =>
    computeDebugLogPollDelay({
      enabled: logEnabled.value,
      consecutiveFailures: consecutiveFailures.value,
      activeIntervalMs: options.activeIntervalMs,
      idleIntervalMs: options.idleIntervalMs,
      maxBackoffMs: options.maxBackoffMs
    })
  )

  const clearTimer = () => {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  }

  const applyLogs = (items: DebugLogEntry[]) => {
    const signature = debugLogsSignature(items)
    if (signature === lastSignature) return
    lastSignature = signature
    debugLogEntries.value = items
    debugLogs.value = mapDebugLogsForConsole(items)
  }

  /** 拉取一次日志；并发调用复用同一个在途请求。失败只计数并记录错误，不抛出。 */
  const fetchLogs = (): Promise<void> => {
    if (inFlight) return inFlight
    const id = toValue(deviceId)
    if (!id) return Promise.resolve()
    const gen = generation
    const run = (async () => {
      try {
        const data = unwrapFlat(await getDeviceDebugLogs(id, { limit: fetchLimit }))
        if (gen !== generation) return
        if (Array.isArray(data?.list)) applyLogs(data.list as DebugLogEntry[])
        logsError.value = null
        consecutiveFailures.value = 0
      } catch (err) {
        if (gen !== generation) return
        logsError.value = err
        consecutiveFailures.value += 1
      }
    })()
    const tracked = run.finally(() => {
      if (inFlight === tracked) inFlight = null
    })
    inFlight = tracked
    return tracked
  }

  const schedule = () => {
    clearTimer()
    if (!polling.value || documentHidden.value) return
    timer = setTimeout(tick, nextPollDelay.value)
  }

  async function tick() {
    clearTimer()
    await fetchLogs()
    schedule()
  }

  const handleVisibilityChange = () => {
    documentHidden.value = document.visibilityState === 'hidden'
    if (!polling.value) return
    if (documentHidden.value) clearTimer()
    else void tick() // 恢复可见立即补拉，之后回到正常节奏
  }

  const startPolling = () => {
    if (polling.value) return
    polling.value = true
    if (typeof document !== 'undefined') document.addEventListener('visibilitychange', handleVisibilityChange)
    if (documentHidden.value) return
    void tick()
  }

  const stopPolling = () => {
    polling.value = false
    clearTimer()
    if (typeof document !== 'undefined') document.removeEventListener('visibilitychange', handleVisibilityChange)
  }

  // 开关状态由后端真实配置决定，避免页面本地状态和设备实际调试模式漂移。
  const refreshStatus = async () => {
    const id = toValue(deviceId)
    if (!id) return
    const gen = generation
    try {
      const data = unwrapFlat(await getDeviceDebugStatus(id))
      if (gen !== generation) return
      if (data) logEnabled.value = data.enabled ?? false
      statusError.value = null
    } catch (err) {
      if (gen !== generation) return
      statusError.value = err
    }
  }

  /** 切换调试模式：成功后才更新本地状态；失败回滚到切换前的值并记录错误。 */
  const handleLogSwitch = async (value: boolean) => {
    const id = toValue(deviceId)
    if (!id || logSwitching.value) return
    const previous = logEnabled.value
    logSwitching.value = true
    try {
      unwrapFlat(await setDeviceDebugStatus(id, { enabled: value }))
      logEnabled.value = value
      statusError.value = null
    } catch (err) {
      logEnabled.value = previous
      statusError.value = err
    } finally {
      logSwitching.value = false
    }
  }

  // 开启调试后立即拉一次，让新报文尽快出现；关闭后按空闲节奏重新排程。
  watch(logEnabled, (enabled) => {
    if (!polling.value) return
    if (enabled) void tick()
    else schedule()
  })

  const resetForDevice = () => {
    generation += 1
    clearTimer()
    inFlight = null
    lastSignature = ''
    logEnabled.value = false
    statusError.value = null
    logsError.value = null
    consecutiveFailures.value = 0
    debugLogEntries.value = []
    debugLogs.value = []
  }

  watch(
    () => toValue(deviceId),
    (next, prev) => {
      if (prev === undefined || next === prev) return
      resetForDevice()
      void refreshStatus()
      if (polling.value) void tick()
    }
  )

  if (options.autoStart !== false) {
    void refreshStatus()
    startPolling()
  }

  if (getCurrentScope()) onScopeDispose(stopPolling)

  return {
    logEnabled,
    logSwitching,
    statusError,
    debugLogEntries,
    debugLogs,
    logsError,
    consecutiveFailures,
    polling,
    documentHidden,
    nextPollDelay,
    fetchLogs,
    refreshStatus,
    handleLogSwitch,
    startPolling,
    stopPolling
  }
}
