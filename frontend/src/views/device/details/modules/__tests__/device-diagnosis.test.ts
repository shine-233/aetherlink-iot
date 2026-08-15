/**
 * 文件用途: device-diagnosis 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceDiagnostics: vi.fn(),
  getDeviceDebugStatus: vi.fn(),
  setDeviceDebugStatus: vi.fn(),
  getDeviceDebugLogs: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deviceDiagnostics: hoisted.deviceDiagnostics,
  getDeviceDebugStatus: hoisted.getDeviceDebugStatus,
  setDeviceDebugStatus: hoisted.setDeviceDebugStatus,
  getDeviceDebugLogs: hoisted.getDeviceDebugLogs
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@vicons/ionicons5', () => ({
  Refresh: defineComponent({ setup: () => () => h('span') }),
  HelpCircleOutline: defineComponent({ setup: () => () => h('span') })
}))

import Component from '../device-diagnosis.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        NButton: defineComponent({ props: ['bordered'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NIcon: defineComponent({ props: ['size'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NFlex: defineComponent({ props: ['gap', 'vertical'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NCard: defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NText: defineComponent({ props: ['type', 'depth'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NNumberAnimation: defineComponent({ props: ['from', 'to', 'precision'], setup() { return () => h('span') } }),
        NDataTable: defineComponent({ props: ['columns', 'data', 'maxHeight', 'remote'], setup() { return () => h('table') } }),
        NSwitch: defineComponent({ props: ['value'], emits: ['update:value'], setup() { return () => h('div') } }),
        NTooltip: defineComponent({ props: ['trigger'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/device-diagnosis.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    hoisted.deviceDiagnostics.mockResolvedValue({
      data: {
        stats: {
          uplink: { success: 10, total: 12, success_rate: 83.3 },
          downlink: { success: 8, total: 10, success_rate: 80 },
          storage: { success: 15, total: 15, success_rate: 100 }
        },
        recent_failures: [
          { timestamp: '2024-01-01T00:00:00Z', direction: 'uplink', stage: 'transport', error: 'timeout' }
        ]
      }
    })
    hoisted.getDeviceDebugStatus.mockResolvedValue({ data: { enabled: false } })
    hoisted.getDeviceDebugLogs.mockResolvedValue({ data: { list: [] } })
    hoisted.setDeviceDebugStatus.mockResolvedValue({})
  })

  afterEach(() => {
    vi.useRealTimers()
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes diagnostics, debug status and log polling for the device', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.deviceDiagnostics).toHaveBeenCalledWith('device-1')
    expect(hoisted.getDeviceDebugStatus).toHaveBeenCalledWith('device-1')
    expect(hoisted.getDeviceDebugLogs).toHaveBeenCalledWith('device-1', { limit: 100 })
    expect(state.statistics).toMatchObject({
      uplink: { success: 10, total: 12, rate: 83.3 },
      downlink: { success: 8, total: 10, rate: 80 },
      storage: { success: 15, total: 15, rate: 100 }
    })
    expect(state.failureRecords).toEqual([
      {
        timestamp: '2024-01-01T00:00:00Z',
        direction: 'uplink',
        stage: 'transport',
        error: 'timeout'
      }
    ])
  })

  it('fetches diagnostics on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceDiagnostics).toHaveBeenCalledWith('device-1')
  })

  it('fetches log status on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDeviceDebugStatus).toHaveBeenCalledWith('device-1')
  })

  it('updates statistics on successful fetch', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.statistics.uplink.success).toBe(10)
    expect(state.statistics.uplink.total).toBe(12)
    expect(state.statistics.uplink.rate).toBe(83.3)
  })

  it('updates failure records on successful fetch', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.failureRecords).toHaveLength(1)
    expect(state.failureRecords[0].direction).toBe('uplink')
  })

  it('sets logEnabled from debug status', async () => {
    hoisted.getDeviceDebugStatus.mockResolvedValue({ data: { enabled: true } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.logEnabled).toBe(true)
  })

  it('handleLogSwitch calls setDeviceDebugStatus', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleLogSwitch(true)
    expect(hoisted.setDeviceDebugStatus).toHaveBeenCalledWith('device-1', { enabled: true })
  })

  it('handleLogSwitch reverts on error', async () => {
    hoisted.setDeviceDebugStatus.mockRejectedValue(new Error('fail'))
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.logEnabled = false
    await state.handleLogSwitch(true)
    expect(state.logEnabled).toBe(false)
  })

  it('refresh calls fetchDiagnostics and getLogStatus', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getSetupState(wrapper)
    state.refresh()
    expect(hoisted.deviceDiagnostics).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceDiagnostics).toHaveBeenCalledWith('device-1')
    expect(hoisted.getDeviceDebugStatus).toHaveBeenCalledTimes(1)
    expect(hoisted.getDeviceDebugStatus).toHaveBeenCalledWith('device-1')
  })

  it('handles empty diagnostics data gracefully', async () => {
    hoisted.deviceDiagnostics.mockResolvedValue({ data: {} })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.statistics.uplink.success).toBe(0)
    expect(state.failureRecords).toHaveLength(0)
  })

  it('handles diagnostics error gracefully', async () => {
    hoisted.deviceDiagnostics.mockRejectedValue(new Error('fail'))
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.statistics).toMatchObject({
      uplink: { success: 0, total: 0, rate: 0 },
      downlink: { success: 0, total: 0, rate: 0 },
      storage: { success: 0, total: 0, rate: 0 }
    })
    expect(state.failureRecords).toEqual([])
    expect(hoisted.getDeviceDebugStatus).toHaveBeenCalledWith('device-1')
  })
})
