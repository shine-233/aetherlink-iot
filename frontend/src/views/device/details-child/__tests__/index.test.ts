/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceDetail: vi.fn(),
  deviceUpdate: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceDetail: hoisted.deviceDetail,
  deviceUpdate: hoisted.deviceUpdate
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { d_id: 'dev-1' } })
}))

vi.mock('@aetherlink/hooks', () => {
  // Vitest hoists this mock before ESM imports, so use a local require here.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { ref } = require('vue')
  return {
  useLoading: (init = false) => {
    const loading = ref(init)
    return { loading, startLoading: vi.fn(() => { loading.value = true }), endLoading: vi.fn(() => { loading.value = false }) }
  }
  }
})

vi.mock('@/store/modules/device', () => ({
  useDeviceDataStore: () => ({
    deviceData: { name: 'Device 1', device_number: '123' },
    fetchData: vi.fn()
  })
}))

vi.mock('@/store/modules/app', () => ({
  useAppStore: () => ({})
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: vi.fn() })
}))

vi.mock('@/utils/common/discrete', () => ({
  message: { info: vi.fn(), error: vi.fn(), success: vi.fn() }
}))

vi.mock('@/views/device/details/modules/telemetry/telemetry.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/join.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/device-analysis.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/message.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/stats.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/event-report.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/command-delivery.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/automate.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/give-an-alarm.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))
vi.mock('@/views/device/details/modules/settings.vue', () => ({ default: defineComponent({ setup: () => () => h('div') }) }))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTabs: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTabPane: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpin: defineComponent({ setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details-child/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceDetail.mockResolvedValue({ data: { id: 'dev-1', name: 'Device 1', device_number: '123' }, error: null })
    hoisted.deviceUpdate.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads child device details and removes subdevice tab for non-gateway templates', async () => {
    hoisted.deviceDetail.mockResolvedValue({
      data: {
        id: 'dev-1',
        name: 'Device 1',
        device_number: '123',
        is_online: 1,
        device_config: { device_type: '1' }
      },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceDetail).toHaveBeenCalledWith('dev-1')
    expect(state.device_number).toBe('123')
    expect(state.device_color).toBe('rgb(2,153,52)')
    expect(state.icon_type).toBe('rgb(2,153,52)')
    expect(state.components.map((item: { key: string }) => item.key)).toEqual([
      'telemetry',
      'join',
      'message',
      'stats',
      'event-report',
      'command-delivery',
      'automate',
      'give-an-alarm',
      'settings'
    ])
  })

  it('has components list with tabs', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.components.map((item: { key: string }) => item.key)).toEqual([
      'telemetry',
      'join',
      'device-analysis',
      'message',
      'stats',
      'event-report',
      'command-delivery',
      'automate',
      'give-an-alarm',
      'settings'
    ])
  })

  it('changeTabs sets tabValue', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.changeTabs('message')
    expect(state.tabValue).toBe('message')
  })

  it('editConfig opens dialog', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.editConfig()
    expect(state.showDialog).toBe(true)
  })
})
