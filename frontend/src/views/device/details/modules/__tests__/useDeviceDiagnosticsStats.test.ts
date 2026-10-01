/**
 * 文件用途: useDeviceDiagnosticsStats 与诊断纯函数测试。
 * 覆盖: 显式 loading/ready/empty/error 状态、flat `{ error }` 识别、失败时保留旧数据、
 *       并发刷新只采纳最后一次、切换设备重置、时间线/下一步/支持摘要与脱敏、表格列定义。
 */
import { effectScope, nextTick, ref } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({ deviceDiagnostics: vi.fn() }))

vi.mock('@/service/api', () => ({ deviceDiagnostics: hoisted.deviceDiagnostics }))
vi.mock('@/locales', () => ({ $t: (key: string) => key }))

import {
  buildDiagnosticSupportSummary,
  buildDiagnosticTimeline,
  createEmptyStatistics,
  createFailureRecordColumns,
  normalizeDiagnosticsResponse,
  resolveDiagnosticNextSteps,
  useDeviceDiagnosticsStats
} from '../useDeviceDiagnosticsStats'

const readyPayload = {
  data: {
    stats: {
      uplink: { success: 10, total: 12, success_rate: 83.3 },
      downlink: { success: 8, total: 10, success_rate: 80 },
      storage: { success: 15, total: 15, success_rate: 100 }
    },
    recent_failures: [{ timestamp: '2024-01-01T00:00:00Z', direction: 'uplink', stage: 'transport', error: 'timeout' }]
  },
  error: null
}

const scopes: Array<ReturnType<typeof effectScope>> = []
const setup = (id: any = 'device-1', options = {}) => {
  const scope = effectScope()
  scopes.push(scope)
  return scope.run(() => useDeviceDiagnosticsStats(id, options))!
}

describe('normalizeDiagnosticsResponse', () => {
  it('accepts wrapped and bare payloads, returns null without stats, throws on flat error', () => {
    expect(normalizeDiagnosticsResponse(readyPayload as any)?.stats?.uplink?.success).toBe(10)
    expect(normalizeDiagnosticsResponse(readyPayload.data as any)?.stats?.storage?.total).toBe(15)
    expect(normalizeDiagnosticsResponse({ data: {} })).toBeNull()
    expect(normalizeDiagnosticsResponse(undefined)).toBeNull()
    expect(() => normalizeDiagnosticsResponse({ data: null, error: new Error('500') })).toThrow('500')
  })
})

describe('useDeviceDiagnosticsStats', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceDiagnostics.mockResolvedValue(readyPayload)
  })
  afterEach(() => {
    while (scopes.length > 0) scopes.pop()?.stop()
  })

  it('goes loading -> ready and maps stats/failures', async () => {
    const api = setup()
    expect(api.status.value).toBe('loading')
    await flushPromises()
    expect(api.status.value).toBe('ready')
    expect(api.hasData.value).toBe(true)
    expect(api.statistics.value.uplink).toEqual({ success: 10, total: 12, rate: 83.3 })
    expect(api.failureRecords.value).toEqual([
      { timestamp: '2024-01-01T00:00:00Z', direction: 'uplink', stage: 'transport', error: 'timeout' }
    ])
  })

  it('reports empty when stats are missing', async () => {
    hoisted.deviceDiagnostics.mockResolvedValue({ data: {}, error: null })
    const api = setup()
    await flushPromises()
    expect(api.status.value).toBe('empty')
    expect(api.statistics.value).toEqual(createEmptyStatistics())
  })

  it('reports error for flat `{ error }` and rejected requests, keeping last good data', async () => {
    const api = setup()
    await flushPromises()
    hoisted.deviceDiagnostics.mockResolvedValueOnce({ data: null, error: new Error('502') })
    await api.refresh()
    expect(api.status.value).toBe('error')
    expect((api.error.value as Error).message).toBe('502')
    expect(api.statistics.value.uplink.success).toBe(10)

    hoisted.deviceDiagnostics.mockRejectedValueOnce(new Error('network'))
    await api.refresh()
    expect((api.error.value as Error).message).toBe('network')
    expect(api.failureRecords.value).toHaveLength(1)

    await api.refresh()
    expect(api.status.value).toBe('ready')
    expect(api.error.value).toBeNull()
  })

  it('only applies the latest of concurrent refreshes', async () => {
    let resolveSlow: (v: unknown) => void = () => {}
    hoisted.deviceDiagnostics.mockImplementationOnce(() => new Promise((r) => (resolveSlow = r)))
    const api = setup()
    hoisted.deviceDiagnostics.mockResolvedValueOnce({ data: { stats: { uplink: { success: 1 } } }, error: null })
    await api.refresh()
    resolveSlow(readyPayload)
    await flushPromises()
    expect(api.statistics.value.uplink.success).toBe(1)
  })

  it('resets and refetches when the device id changes', async () => {
    const id = ref('device-1')
    const api = setup(id)
    await flushPromises()
    hoisted.deviceDiagnostics.mockResolvedValue({ data: {}, error: null })
    id.value = 'device-2'
    await nextTick()
    await flushPromises()
    expect(hoisted.deviceDiagnostics).toHaveBeenLastCalledWith('device-2')
    expect(api.statistics.value).toEqual(createEmptyStatistics())
    expect(api.failureRecords.value).toEqual([])
    expect(api.hasData.value).toBe(false)
  })

  it('skips the initial fetch when immediate is false', async () => {
    const api = setup('device-1', { immediate: false })
    await flushPromises()
    expect(hoisted.deviceDiagnostics).not.toHaveBeenCalled()
    expect(api.status.value).toBe('idle')
  })
})

describe('diagnostic derivations', () => {
  const failures = [
    { timestamp: '2024-01-01T00:00:00Z', direction: 'uplink' as const, stage: 'parse', error: 'token=abc' }
  ]
  const logs = [{ ts: '2024-01-01T00:00:01Z', diagnostic_code: 'mqtt_auth_failed', meta: { password: 'x' } }]

  it('builds a masked timeline with next actions', () => {
    const timeline = buildDiagnosticTimeline(failures, logs)
    expect(timeline).toHaveLength(2)
    expect(timeline[0]).toMatchObject({ type: 'error', title: 'uplink / parse', detail: 'token=***' })
    expect(timeline[0].nextAction).toBe('custom.device_details.nextActionUplink')
    expect(timeline[1]).toMatchObject({ type: 'info', title: 'mqtt_auth_failed' })
    expect(timeline[1].nextAction).toBe('custom.device_details.nextActionAuth')
    expect(timeline[1].detail).not.toContain('"x"')
  })

  it('resolves next steps by priority: error > debug off > no logs > review', () => {
    const steps = (diagnosticsStatus: any, logEnabled: boolean, logCount: number) =>
      resolveDiagnosticNextSteps({ diagnosticsStatus, logEnabled, logCount })[0]
    expect(steps('error', true, 5)).toBe('custom.device_details.nextStepErrorRefresh')
    expect(steps('ready', false, 5)).toBe('custom.device_details.nextStepEnableDebug')
    expect(steps('ready', true, 0)).toBe('custom.device_details.nextStepDebugNoLogs')
    expect(steps('ready', true, 3)).toBe('custom.device_details.nextStepReviewLatest')
  })

  it('builds a support summary with stats, masked failures, timeline and next steps', () => {
    const summary = buildDiagnosticSupportSummary({
      deviceId: 'device-1',
      logEnabled: true,
      statistics: { ...createEmptyStatistics(), uplink: { success: 10, total: 12, rate: 83.33 } },
      failureRecords: failures,
      timeline: buildDiagnosticTimeline(failures, logs),
      nextSteps: ['step-a'],
      generatedAt: '2024-01-02T03:04:05'
    })
    expect(summary).toContain('custom.device_details.summaryDeviceId: device-1')
    expect(summary).toContain('custom.device_details.summaryDebugMode: enabled')
    expect(summary).toContain('uplink: 10/12 (83.3%)')
    expect(summary).toContain('2024-01-02 03:04:05')
    expect(summary).toContain('custom.device_details.summaryCount: 1')
    expect(summary).toContain('- step-a')
    expect(summary).not.toContain('abc')
  })

  it('keeps the failure table column contract', () => {
    const columns = createFailureRecordColumns() as Array<{ key: string; width?: number }>
    expect(columns.map((c) => c.key)).toEqual(['timestamp', 'direction', 'stage', 'error'])
    expect(columns.map((c) => c.width)).toEqual([200, 150, 200, undefined])
  })
})
