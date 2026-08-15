/**
 * 文件用途: attribute-info 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigEdit: vi.fn(),
  deviceConfigInfo: vi.fn(),
  deviceConfigMenu: vi.fn(),
  routerPushByKey: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigEdit: hoisted.deviceConfigEdit,
  deviceConfigInfo: hoisted.deviceConfigInfo,
  deviceConfigMenu: hoisted.deviceConfigMenu
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../attribute-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { configInfo: { id: 'cfg-1' }, ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value', 'search'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/attribute-info.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfigMenu.mockResolvedValue({ data: [{ id: 'tpl-1', name: 'Telemetry Model 1' }] })
    hoisted.deviceConfigInfo.mockResolvedValue({ data: { device_template_id: 'tpl-1' } })
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes template binding options with unbind option and selected template', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.deviceConfigMenu).toHaveBeenCalledWith({ name: '' })
    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
    expect(state.selectValue).toBe('tpl-1')
    expect(state.plugList).toEqual([
      { name: 'generate.unbind', id: '' },
      { id: 'tpl-1', name: 'Telemetry Model 1' }
    ])
  })

  it('loads table data and config info on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceConfigMenu).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceConfigInfo).toHaveBeenCalledWith({ id: 'cfg-1' })
  })

  it('choseTemp calls deviceConfigEdit and emits upDateConfig', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.choseTemp('tpl-1')
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith({ device_template_id: 'tpl-1', id: 'cfg-1' })
  })

  it('toTemplate navigates to template page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.toTemplate()
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('device_template')
  })

  it('searchPlug calls getTableData with name', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.searchPlug('test')
    await flushPromises()
    expect(hoisted.deviceConfigMenu).toHaveBeenCalledWith({ name: 'test' })
  })
})
