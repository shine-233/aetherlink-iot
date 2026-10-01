/**
 * 文件用途: useDeviceDebugConsole 测试。
 * 覆盖: 自适应轮询节奏、失败指数退避、页面隐藏暂停/恢复补拉、flat `{ error }` 失败识别、
 *       切换设备丢弃旧请求、开关失败回滚、作用域销毁释放定时器。
 */
import { effectScope, nextTick, ref } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getDeviceDebugStatus: vi.fn(),
  setDeviceDebugStatus: vi.fn(),
  getDeviceDebugLogs: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceDiagnostics: vi.fn(),
  getDeviceDebugStatus: hoisted.getDeviceDebugStatus,
  setDeviceDebugStatus: hoisted.setDeviceDebugStatus,
  getDeviceDebugLogs: hoisted.getDeviceDebugLogs
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

import {
  computeDebugLogPollDelay,
  debugLogsSignature,
  mapDebugLogsForConsole,
  useDeviceDebugConsole
} from '../useDeviceDebugConsole'

const scopes: Array<ReturnType<typeof effectScope>> = []

const setup = (id: string | ReturnType<typeof ref<string>> = 'device-1', options = {}) => {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(() => useDeviceDebugConsole(id as any, options))!
}

const setVisibility = (state: 'hidden' | 'visible') => {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state })
  document.dispatchEvent(new Event('visibilitychange'))
}

describe('computeDebugLogPollDelay', () => {
  it('uses 3s when enabled and 15s when disabled', () => {
    expect(computeDebugLogPollDelay({ enabled: true, consecutiveFailures: 0 })).toBe(3000)
    expect(computeDebugLogPollDelay({ enabled: false, consecutiveFailures: 0 })).toBe(15000)
  })

  it('backs off exponentially and caps at 60s', () => {
    expect(computeDebugLogPollDelay({ enabled: true, consecutiveFailures: 1 })).toBe(6000)
    expect(computeDebugLogPollDelay({ enabled: true, consecutiveFailures: 3 })).toBe(24000)
    expect(computeDebugLogPollDelay({ enabled: true, consecutiveFailures: 10 })).toBe(60000)
    expect(computeDebugLogPollDelay({ enabled: false, consecutiveFailures: 50 })).toBe(60000)
  })
})

describe('debug log formatting', () => {
  it('reverses to chronological order and masks secrets', () => {
    const lines = mapDebugLogsForConsole([
      { ts: '2024-01-01T00:00:02Z', event: 'b', password: 'p1' },
      { ts: '2024-01-01T00:00:01Z', message: 'token=abc123' }
    ])
    expect(lines).toHaveLength(2)
    expect(lines[0]).toContain('token=***')
    expect(lines[1]).toContain('"password":"***"')
    expect(lines.join('\n')).not.toContain('p1')
    expect(lines.join('\n')).not.toContain('abc123')
  })

  it('signature only changes when edges or count change', () => {
    const a = [{ ts: 1 }, { ts: 2 }]
    expect(debugLogsSignature(a)).toBe(debugLogsSignature([{ ts: 1 }, { ts: 2 }]))
    expect(debugLogsSignature(a)).not.toBe(debugLogsSignature([{ ts: 0 }, { ts: 1 }, { ts: 2 }]))
    expect(debugLogsSignature([])).toBe('0')
  })
})

describe('useDeviceDebugConsole', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    setVisibility('visible')
    hoisted.getDeviceDebugStatus.mockResolvedValue({ data: { enabled: false }, error: null })
    hoisted.getDeviceDebugLogs.mockResolvedValue({ data: { list: [] }, error: null })
    hoisted.setDeviceDebugStatus.mockResolvedValue({ data: {}, error: null })
  })

  afterEach(() => {
    while (scopes.length > 0) scopes.pop()?.stop()
    vi.useRealTimers()
    setVisibility('visible')
  })

  it('fetches status and logs immediately, then polls at the idle cadence when debug is off', async () => {
    const api = setup()
    await flushPromises()
    expect(hoisted.getDeviceDebugStatus).toHaveBeenCalledWith('device-1')
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledWith('device-1', { limit: 100 })
    expect(api.polling.value).toBe(true)

    await vi.advanceTimersByTimeAsync(14999)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(2)
  })

  it('polls every 3s while debug mode is enabled', async () => {
    hoisted.getDeviceDebugStatus.mockResolvedValue({ data: { enabled: true }, error: null })
    const api = setup()
    await flushPromises()
    expect(api.logEnabled.value).toBe(true)
    hoisted.getDeviceDebugLogs.mockClear()

    await vi.advanceTimersByTimeAsync(3000)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(3000)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(2)
  })

  it('treats flat `{ error }` responses as failures and backs off exponentially, resetting on success', async () => {
    hoisted.getDeviceDebugLogs.mockResolvedValue({ data: null, error: new Error('502') })
    const api = setup()
    await flushPromises()
    expect(api.consecutiveFailures.value).toBe(1)
    expect((api.logsError.value as Error).message).toBe('502')
    expect(api.nextPollDelay.value).toBe(30000)

    await vi.advanceTimersByTimeAsync(29999)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(2)
    expect(api.consecutiveFailures.value).toBe(2)
    expect(api.nextPollDelay.value).toBe(60000)

    hoisted.getDeviceDebugLogs.mockResolvedValue({ data: { list: [{ ts: 1, event: 'ok' }] }, error: null })
    await vi.advanceTimersByTimeAsync(60000)
    expect(api.consecutiveFailures.value).toBe(0)
    expect(api.logsError.value).toBeNull()
    expect(api.debugLogs.value).toHaveLength(1)
    expect(api.nextPollDelay.value).toBe(15000)
  })

  it('pauses while the tab is hidden and catches up immediately when visible again', async () => {
    setup()
    await flushPromises()
    setVisibility('hidden')
    await vi.advanceTimersByTimeAsync(120000)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)

    setVisibility('visible')
    await flushPromises()
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(2)
  })

  it('stops polling when the effect scope is disposed', async () => {
    const api = setup()
    await flushPromises()
    scopes.pop()?.stop()
    expect(api.polling.value).toBe(false)
    await vi.advanceTimersByTimeAsync(120000)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)
  })

  it('does not replace log arrays when the payload is unchanged', async () => {
    hoisted.getDeviceDebugLogs.mockResolvedValue({ data: { list: [{ ts: 2 }, { ts: 1 }] }, error: null })
    const api = setup()
    await flushPromises()
    const first = api.debugLogs.value
    await vi.advanceTimersByTimeAsync(15000)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(2)
    expect(api.debugLogs.value).toBe(first)
  })

  it('discards in-flight results from the previous device after switching', async () => {
    let resolveOld: (value: unknown) => void = () => {}
    hoisted.getDeviceDebugLogs.mockImplementationOnce(() => new Promise((resolve) => (resolveOld = resolve)))
    const id = ref('device-1')
    const api = setup(id)
    await flushPromises()

    hoisted.getDeviceDebugLogs.mockResolvedValue({ data: { list: [{ ts: 1, event: 'new-device' }] }, error: null })
    id.value = 'device-2'
    await nextTick()
    await flushPromises()
    expect(hoisted.getDeviceDebugLogs).toHaveBeenLastCalledWith('device-2', { limit: 100 })
    expect(hoisted.getDeviceDebugStatus).toHaveBeenLastCalledWith('device-2')

    resolveOld({ data: { list: [{ ts: 9, event: 'old-device' }] }, error: null })
    await flushPromises()
    expect(api.debugLogs.value.join('\n')).toContain('new-device')
    expect(api.debugLogs.value.join('\n')).not.toContain('old-device')
  })

  it('handleLogSwitch applies on success, triggers an immediate fetch, and reverts on flat error', async () => {
    const api = setup()
    await flushPromises()
    hoisted.getDeviceDebugLogs.mockClear()

    await api.handleLogSwitch(true)
    await flushPromises()
    expect(hoisted.setDeviceDebugStatus).toHaveBeenCalledWith('device-1', { enabled: true })
    expect(api.logEnabled.value).toBe(true)
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledTimes(1)

    hoisted.setDeviceDebugStatus.mockResolvedValue({ data: null, error: new Error('forbidden') })
    await api.handleLogSwitch(false)
    expect(api.logEnabled.value).toBe(true)
    expect((api.statusError.value as Error).message).toBe('forbidden')
    expect(api.logSwitching.value).toBe(false)
  })

  it('does not auto-start when autoStart is false', async () => {
    const api = setup('device-1', { autoStart: false })
    await flushPromises()
    expect(api.polling.value).toBe(false)
    expect(hoisted.getDeviceDebugLogs).not.toHaveBeenCalled()
    expect(hoisted.getDeviceDebugStatus).not.toHaveBeenCalled()
  })
})
