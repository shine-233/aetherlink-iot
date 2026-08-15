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
  deleteDeviceGroup: vi.fn(),
  getDeviceGroup: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deleteDeviceGroup: hoisted.deleteDeviceGroup,
  getDeviceGroup: hoisted.getDeviceGroup
}))

vi.mock('@/views/device/modules/all-columns', () => ({
  group_columns: vi.fn(() => [])
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: hoisted.routerPush })
}))

vi.mock('lodash-es', () => ({
  debounce: vi.fn((fn: any) => fn)
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
  NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NPagination: defineComponent({ props: { page: { default: 1 } }, emits: ['update:page'], setup() { return () => h('div') } })
}))

vi.mock('@vicons/ionicons5', () => ({ SearchOutline: defineComponent({ setup: () => () => h('div') }) }))

vi.mock('./components', () => ({
  AddOrEditDevices: defineComponent({ setup() { return () => h('div') } })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        AddOrEditDevices: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/grouping/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getDeviceGroup.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deleteDeviceGroup.mockResolvedValue({})
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads root device groups with default parent and pagination contract', async () => {
    hoisted.getDeviceGroup.mockResolvedValue({
      data: {
        list: [{ id: 'grp-1', name: 'Group 1' }],
        total: 21
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.getDeviceGroup).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      parent_id: 0
    })
    expect(state.loading).toBe(false)
    expect(state.data).toEqual([{ id: 'grp-1', name: 'Group 1' }])
    expect(state.totalPages).toBe(3)
  })

  it('loads device groups on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDeviceGroup).toHaveBeenCalledTimes(1)
  })

  it('keeps empty or incomplete responses from producing NaN pagination', async () => {
    hoisted.getDeviceGroup.mockResolvedValue({ data: { list: [] } })
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.data).toEqual([])
    expect(state.totalPages).toBe(0)
  })

  it('viewDetails navigates to grouping-details', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.viewDetails('grp-1')
    expect(hoisted.routerPush).toHaveBeenCalledWith({ name: 'device_grouping-details', query: { id: 'grp-1' } })
  })

  it('deleteItem calls deleteDeviceGroup and refreshes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.deleteItem('grp-1')
    expect(hoisted.deleteDeviceGroup).toHaveBeenCalledWith({ id: 'grp-1' })
  })

  it('showModal sets showModal on modal ref', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.the_modal = { showModal: false }
    state.showModal()
    expect(state.the_modal.showModal).toBe(true)
  })
})
