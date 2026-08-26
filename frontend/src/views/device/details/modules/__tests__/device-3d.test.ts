/**
 * 文件用途：设备详情 3D 预览包装组件的行为契约测试。
 * 核心逻辑：钉死温度遥测提取启发式（精确 key 优先、temp 模糊兜底、缺失回退）与错误态渲染。
 * 关键注意事项：Device3DPanel 与遥测组合函数均被替换为桩，聚焦本组件自身的接线逻辑。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { telemetryMock } = vi.hoisted(() => ({
  telemetryMock: {
    telemetryData: [] as Array<{ key: string; value: number }>,
    telemetryLoadStatus: 'ready' as 'idle' | 'loading' | 'ready' | 'empty' | 'error',
    telemetryLoadError: '',
    refreshTelemetry: vi.fn()
  }
}))

vi.mock('../telemetry/useTelemetryRealtimeState', async () => {
  const { computed } = await import('vue')
  return {
    // 用 computed 包装，保证 beforeEach 中替换 telemetryMock 字段后组件读取到最新值。
    useTelemetryRealtimeState: () => ({
      telemetryData: computed(() => telemetryMock.telemetryData),
      telemetryLoadStatus: computed(() => telemetryMock.telemetryLoadStatus),
      telemetryLoadError: computed(() => telemetryMock.telemetryLoadError),
      refreshTelemetry: telemetryMock.refreshTelemetry
    })
  }
})

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

// Device3DPanel 为显式导入，必须以模块级 mock 替换，避免测试拉起真实 three/WebGL 渲染链路。
// vi.mock 会被提升到 import 之前，因此桩组件在工厂内动态引入 vue 构造。
vi.mock('@/components/device3d/Device3DPanel.vue', async () => {
  const { defineComponent, h } = await import('vue')
  return {
    default: defineComponent({
      name: 'Device3DPanel',
      props: { online: Boolean, temperature: { type: Number, default: undefined }, deviceName: String },
      setup() {
        return () => h('div', { class: 'device-3d-panel-stub' })
      }
    })
  }
})

import Component from '../device-3d.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = mount(Component, {
    props: { id: 'device-1', online: true, ...props },
    global: {
      stubs: { NSpace: { template: '<div><slot /></div>' }, NAlert: { template: '<div class="n-alert"><slot /></div>' } }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/details/modules/device-3d.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    telemetryMock.telemetryData = []
    telemetryMock.telemetryLoadStatus = 'ready'
    telemetryMock.telemetryLoadError = ''
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('refreshes telemetry on mount and when device id changes', async () => {
    const wrapper = mountComponent()
    expect(telemetryMock.refreshTelemetry).toHaveBeenCalledTimes(1)
    await wrapper.setProps({ id: 'device-2' })
    expect(telemetryMock.refreshTelemetry).toHaveBeenCalledTimes(2)
  })

  it('prefers the exact temperature key for panel color driving', () => {
    telemetryMock.telemetryData = [
      { key: 'humidity', value: 55 },
      { key: 'temperature', value: 26 }
    ]
    const wrapper = mountComponent()
    expect(wrapper.findComponent({ name: 'Device3DPanel' }).props('temperature')).toBe(26)
  })

  it('falls back to the first numeric temp-prefixed telemetry item', () => {
    telemetryMock.telemetryData = [
      { key: 'humidity', value: 55 },
      { key: 'temp_motor', value: 41 }
    ]
    const wrapper = mountComponent()
    expect(wrapper.findComponent({ name: 'Device3DPanel' }).props('temperature')).toBe(41)
  })

  it('passes undefined temperature when no thermal telemetry exists', () => {
    telemetryMock.telemetryData = [{ key: 'humidity', value: 55 }]
    const wrapper = mountComponent()
    expect(wrapper.findComponent({ name: 'Device3DPanel' }).props('temperature')).toBeUndefined()
  })

  it('forwards online status and resolves a display name from device data', () => {
    const wrapper = mountComponent({
      online: false,
      deviceData: { name: 'Gateway-A' }
    })
    const panel = wrapper.findComponent({ name: 'Device3DPanel' })
    expect(panel.props('online')).toBe(false)
    expect(panel.props('deviceName')).toBe('Gateway-A')
  })

  it('renders a warning alert when the telemetry snapshot fails to load', () => {
    telemetryMock.telemetryLoadStatus = 'error'
    telemetryMock.telemetryLoadError = 'network down'
    const wrapper = mountComponent()
    const alerts = wrapper.findAll('.n-alert')
    expect(alerts.length).toBe(2)
    expect(alerts[1].text()).toContain('network down')
  })
})
