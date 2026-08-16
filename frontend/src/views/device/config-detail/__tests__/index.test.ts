/**
 * 文件用途: index 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigInfo: vi.fn(),
  deviceTemplateDetail: vi.fn(),
  routerPushByKey: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigInfo: hoisted.deviceConfigInfo,
  deviceTemplateDetail: hoisted.deviceTemplateDetail
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { id: 'cfg-1' } })
}))

  vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  createDiscreteApi: () => ({
    message: {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    },
    notification: {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    },
    dialog: {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    },
    loadingBar: {
      start: vi.fn(),
      finish: vi.fn(),
      error: vi.fn()
    }
  })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTabs: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTabPane: defineComponent({ props: { name: { default: '' } }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        AssociatedDevices: true,
        ExtendInfo: true,
        AttributeInfo: true,
        ConnectionInfo: true,
        AlarmInfo: true,
        Automate: true,
        SettingInfo: true,
        DataHandle: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfigInfo.mockResolvedValue({ data: { id: 'cfg-1', name: 'cfg', device_type: '1', device_template_id: 'tpl-1' } })
    hoisted.deviceTemplateDetail.mockResolvedValue({ data: { name: 'Telemetry Model 1' } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes config detail from route id and loads template name', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.configId).toBe('cfg-1')
    expect(state.activeName).toBe(state.tabKeys.associatedDevices)
    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
    expect(hoisted.deviceTemplateDetail).toHaveBeenCalledWith({ id: 'tpl-1' })
    expect(state.configForm).toMatchObject({
      id: 'cfg-1',
      name: 'cfg',
      device_type: '1',
      device_template_id: 'tpl-1',
      device_template_name: 'Telemetry Model 1'
    })
    expect(Object.values(state.tabKeys)).toEqual([
      'associatedDevices',
      'thingModel',
      'protocolConfig',
      'dataProces',
      'automate',
      'alarm',
      'extensionInfo',
      'devicesSetting'
    ])
  })

  it('loads config on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
  })

  it('editConfig navigates to edit page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.editConfig()
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('device_config-edit', { query: { id: 'cfg-1' } })
  })

  it('clickConfig navigates to template page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.clickConfig()
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('device_template', { query: { id: 'tpl-1' } })
  })
})
