/**
 * 文件用途: RDI composable useRdiTelemetry 的单元测试。
 * 核心逻辑: 通过 mock API、store 或时间行为验证 composable 的状态输出、动作和异常分支。
 * 关键注意事项: 测试应聚焦 composable 契约，避免依赖 RDI 操作视图 DOM 细节。
 * 重构建议: 继续补成功、失败、空数据和清理生命周期用例，提升组合函数边界可信度。
 */
import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockTelemetryDataCurrentKeys, mockGetDeviceOnlineStatus } = vi.hoisted(() => ({
  mockTelemetryDataCurrentKeys: vi.fn(),
  mockGetDeviceOnlineStatus: vi.fn()
}))

vi.mock('@/service/api', () => ({
  telemetryDataCurrentKeys: (...args: unknown[]) => mockTelemetryDataCurrentKeys(...args),
  getDeviceOnlineStatus: (...args: unknown[]) => mockGetDeviceOnlineStatus(...args)
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({ locale: 'en-US' })
}))

import { useRdiTelemetry } from '../useRdiTelemetry'

const Harness = defineComponent({
  props: {
    id: { type: String, required: true },
    online: { type: Number, default: undefined },
    deviceData: { type: Object, default: () => ({}) }
  },
  setup(props, { expose }) {
    const state = useRdiTelemetry(
      () => props.id,
      () => props.online,
      () => props.deviceData as Record<string, any>,
      (key) => key
    )

    expose(state)
    return () => null
  }
})

// 挂载组件暴露的 setup 返回值的受控视图：只声明测试实际触达的成员。
interface RdiTelemetryHarnessVm {
  temperatureUnit: string
  formatTemperatureValue: (value?: unknown) => string
  loadRealtimeState: () => Promise<void>
  loadTelemetry: () => Promise<void>
  telemetry: unknown
  liveOnlineStatus: unknown
  deviceOnlineText: unknown
  deviceDescriptionText: unknown
  telemetryRows: unknown
  formatSwitch: (value?: unknown) => string
  formatLedStatus: (value?: unknown) => string
  formatValue: (value?: unknown) => string
  toAxisValue: (value?: unknown) => unknown
  startTelemetryRefresh: () => void
  stopTelemetryRefresh: () => void
}

const harnessVm = (wrapper: { vm: unknown }) => wrapper.vm as RdiTelemetryHarnessVm

function mountHarness(props?: { id?: string; online?: number; deviceData?: Record<string, unknown> }) {
  return mount(Harness, {
    props: {
      id: props?.id ?? 'dev-1',
      online: props?.online,
      deviceData: props?.deviceData ?? {}
    }
  })
}

describe('useRdiTelemetry', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useRealTimers()
    window.localStorage.clear()
    mockTelemetryDataCurrentKeys.mockResolvedValue({ error: null, data: {} })
    mockGetDeviceOnlineStatus.mockResolvedValue({ error: null, data: { device_status: 0 } })
  })

  it('loads the stored temperature unit and persists changes', async () => {
    window.localStorage.setItem('rdi-temperature-unit', 'F')

    const wrapper = mountHarness()
    const vm = harnessVm(wrapper)

    expect(vm.temperatureUnit).toBe('F')
    expect(vm.formatTemperatureValue(0)).toBe('32.00')

    vm.temperatureUnit = 'C'
    await nextTick()

    expect(window.localStorage.getItem('rdi-temperature-unit')).toBe('C')
    wrapper.unmount()
  })

  it('normalizes realtime telemetry data and online status responses', async () => {
    mockTelemetryDataCurrentKeys.mockResolvedValue({
      error: null,
      data: [
        { key: 'temperature_1', value: 12.34 },
        { key: 'switch_1', value: 1 },
        { key: 'dry_contact_output', value: '0' },
        { key: 'electricity_consumption', value: 5.678 },
        { key: 'led_status', value: 'slow_blink' }
      ]
    })
    mockGetDeviceOnlineStatus.mockResolvedValue({
      error: null,
      data: { device_status: '1' }
    })

    const wrapper = mountHarness({
      deviceData: { description: 'Cold room' }
    })
    const vm = harnessVm(wrapper)

    await vm.loadRealtimeState()

    expect(mockTelemetryDataCurrentKeys).toHaveBeenCalledWith({
      device_id: 'dev-1',
      keys: [
        'temperature_1',
        'temperature_2',
        'switch_1',
        'switch_2',
        'dry_contact_output',
        'electricity_consumption',
        'led_status'
      ]
    })
    expect(mockGetDeviceOnlineStatus).toHaveBeenCalledWith('dev-1')
    expect(vm.telemetry.temperature_1).toBe(12.34)
    expect(vm.liveOnlineStatus).toBe(1)
    expect(vm.deviceOnlineText).toBe('online')
    expect(vm.deviceDescriptionText).toBe('Cold room')
    expect(vm.telemetryRows).toEqual([
      { label: 'T1', value: '12.34', unit: 'C' },
      { label: 'T2', value: '--', unit: 'C' },
      { label: 'switch1', value: 'high', unit: '' },
      { label: 'switch2', value: '--', unit: '' },
      { label: 'dryContact', value: 'low', unit: '' },
      { label: 'kWh', value: '5.68', unit: 'kWh' },
      { label: 'LED1', value: 'ledSlowBlink', unit: '' }
    ])

    wrapper.unmount()
  })

  it('uses the parent device online state instead of refetching status', async () => {
    mockTelemetryDataCurrentKeys.mockResolvedValue({
      error: null,
      data: [{ key: 'temperature_1', value: 18 }]
    })

    const wrapper = mountHarness({
      online: 1,
      deviceData: { description: 'RDI cabinet' }
    })
    const vm = harnessVm(wrapper)

    await vm.loadRealtimeState()

    expect(mockTelemetryDataCurrentKeys).toHaveBeenCalledTimes(1)
    expect(mockGetDeviceOnlineStatus).not.toHaveBeenCalled()
    expect(vm.liveOnlineStatus).toBe(null)
    expect(vm.deviceOnlineText).toBe('online')
    expect(vm.deviceDescriptionText).toBe('RDI cabinet')

    wrapper.unmount()
  })

  it('ignores a stale telemetry response after the device id changes', async () => {
    let resolveFirstRequest: ((value: { error: null; data: Array<{ key: string; value: number }> }) => void) | undefined
    mockTelemetryDataCurrentKeys
      .mockImplementationOnce(
        () =>
          new Promise(resolve => {
            resolveFirstRequest = resolve
          })
      )
      .mockResolvedValueOnce({
        error: null,
        data: [{ key: 'temperature_1', value: 22 }]
      })

    const wrapper = mountHarness({ online: 1 })
    const vm = harnessVm(wrapper)
    const firstLoad = vm.loadTelemetry()

    await wrapper.setProps({ id: 'dev-2' })
    await vm.loadTelemetry()
    expect(vm.telemetry.temperature_1).toBe(22)

    if (!resolveFirstRequest) {
      throw new Error('first telemetry request did not start')
    }
    resolveFirstRequest({
      error: null,
      data: [{ key: 'temperature_1', value: 11 }]
    })
    await firstLoad

    expect(vm.telemetry.temperature_1).toBe(22)
    expect(mockTelemetryDataCurrentKeys).toHaveBeenNthCalledWith(1, expect.objectContaining({ device_id: 'dev-1' }))
    expect(mockTelemetryDataCurrentKeys).toHaveBeenNthCalledWith(2, expect.objectContaining({ device_id: 'dev-2' }))

    wrapper.unmount()
  })

  it('falls back to placeholder formatting for unknown values and empty descriptions', () => {
    const wrapper = mountHarness({
      deviceData: { Description: '   ' }
    })
    const vm = harnessVm(wrapper)

    expect(vm.formatSwitch('unexpected')).toBe('unexpected')
    expect(vm.formatLedStatus('mystery')).toBe('mystery')
    expect(vm.formatValue(undefined)).toBe('--')
    expect(vm.toAxisValue({ nested: true })).toBe('[object Object]')
    expect(vm.deviceDescriptionText).toBe('--')

    wrapper.unmount()
  })

  it('starts and stops the refresh timer around realtime loading', async () => {
    vi.useFakeTimers()
    mockTelemetryDataCurrentKeys.mockResolvedValue({
      error: null,
      data: [{ key: 'temperature_1', value: 21 }]
    })
    mockGetDeviceOnlineStatus.mockResolvedValue({
      error: null,
      data: { is_online: 1 }
    })

    const wrapper = mountHarness()
    const vm = harnessVm(wrapper)

    vm.startTelemetryRefresh()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(mockTelemetryDataCurrentKeys).toHaveBeenCalledTimes(1)
    expect(mockGetDeviceOnlineStatus).toHaveBeenCalledTimes(1)

    vm.stopTelemetryRefresh()
    await vi.advanceTimersByTimeAsync(10_000)

    expect(mockTelemetryDataCurrentKeys).toHaveBeenCalledTimes(1)
    expect(mockGetDeviceOnlineStatus).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })
})
