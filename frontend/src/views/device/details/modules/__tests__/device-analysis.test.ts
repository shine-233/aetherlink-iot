/**
 * 文件用途: device-analysis 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  childDeviceTableList: vi.fn(),
  childDeviceSelectList: vi.fn(),
  removeChildDevice: vi.fn(),
  addChildDevice: vi.fn(),
  deviceUpdate: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  childDeviceTableList: hoisted.childDeviceTableList,
  childDeviceSelectList: hoisted.childDeviceSelectList,
  removeChildDevice: hoisted.removeChildDevice,
  addChildDevice: hoisted.addChildDevice,
  deviceUpdate: hoisted.deviceUpdate
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: hoisted.routerPush })
}))

import Component from '../device-analysis.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NButton: defineComponent({ props: ['type', 'size'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NDataTable: defineComponent({ props: ['columns', 'data', 'pagination', 'remote'], setup() { return () => h('table') } }),
        NModal: defineComponent({ props: ['show', 'title'], emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['labelPlacement', 'labelWidth'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'type', 'placeholder'], emits: ['update:value'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options', 'multiple', 'maxTagCount', 'virtualScroll'], emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NPopconfirm: defineComponent({ emits: ['positive-click'], setup(_, { slots }) { return () => h('div', slots.default?.()) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/device-analysis.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.childDeviceTableList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.childDeviceSelectList.mockResolvedValue({ data: [] })
    hoisted.removeChildDevice.mockResolvedValue({ error: null })
    hoisted.addChildDevice.mockResolvedValue({ error: null })
    hoisted.deviceUpdate.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes child device table with scoped paging contract', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.childDeviceTableList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      id: 'device-1'
    })
    expect(state.pagination).toMatchObject({
      page: 1,
      pageSize: 10,
      showSizePicker: true,
      pageSizes: [10, 15, 20, 25, 30],
      itemCount: 0
    })
    expect(state.tableData).toEqual([])
    expect(state.total).toBe(0)
  })

  it('fetches child device table list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.childDeviceTableList).toHaveBeenCalledTimes(1)
    expect(hoisted.childDeviceTableList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      id: 'device-1'
    })
  })

  it('populates tableData on successful fetch', async () => {
    hoisted.childDeviceTableList.mockResolvedValue({
      data: { list: [{ id: '1', name: 'Child1' }], total: 1 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.total).toBe(1)
  })

  it('handleLook navigates to details-child', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleLook('child-1')
    expect(hoisted.routerPush).toHaveBeenCalledWith({ path: 'details-child', query: { d_id: 'child-1' } })
  })

  it('deleteDevice calls removeChildDevice and refreshes', async () => {
    hoisted.removeChildDevice.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    await state.deleteDevice('child-1')
    expect(hoisted.removeChildDevice).toHaveBeenCalledWith({ sub_device_id: 'child-1' })
    expect(hoisted.childDeviceTableList).toHaveBeenCalledTimes(1)
    expect(hoisted.childDeviceTableList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      id: 'device-1'
    })
  })

  it('addDevice opens dialog and fetches device list', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.addDevice()
    expect(state.showAddDialog).toBe(true)
    expect(hoisted.childDeviceSelectList).toHaveBeenCalledTimes(1)
    expect(hoisted.childDeviceSelectList).toHaveBeenCalledWith()
  })

  it('addChildDeviceSure shows error when no child selected', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.selectChild = []
    state.addChildDeviceSure()
    expect(window.$message?.error).toHaveBeenCalledTimes(1)
    expect(window.$message?.error).toHaveBeenCalledWith('generate.selectSubDevices')
    expect(hoisted.addChildDevice).toHaveBeenCalledTimes(0)
  })

  it('addChildDeviceSure calls addChildDevice when child selected', async () => {
    hoisted.addChildDevice.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.selectChild = ['child-1', 'child-2']
    await state.addChildDeviceSure()
    expect(hoisted.addChildDevice).toHaveBeenCalledWith({
      id: 'device-1',
      son_id: 'child-1,child-2'
    })
  })

  it('setDeviceAddress shows error when name is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.deviceSetName = ''
    state.setDeviceAddress()
    expect(window.$message?.error).toHaveBeenCalledTimes(1)
    expect(window.$message?.error).toHaveBeenCalledWith('generate.enter-sub-device-address')
    expect(hoisted.deviceUpdate).toHaveBeenCalledTimes(0)
  })

  it('setDeviceAddress calls deviceUpdate when name provided', async () => {
    hoisted.deviceUpdate.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.deviceSetId = 'child-1'
    state.deviceSetName = 'addr-1'
    await state.setDeviceAddress()
    expect(hoisted.deviceUpdate).toHaveBeenCalledWith({
      id: 'child-1',
      parent_id: 'device-1',
      sub_device_addr: 'addr-1'
    })
  })

  it('pagination onChange updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    // pagination is a reactive object, onChange is a function on it
    const pagination = state.pagination
    expect(pagination).toMatchObject({
      page: 1,
      pageSize: 10,
      showSizePicker: true,
      pageSizes: [10, 15, 20, 25, 30],
      itemCount: 0
    })
    expect(typeof pagination.onChange).toBe('function')
    pagination.onChange(2)
    expect(state.log_page).toBe(2)
    expect(hoisted.childDeviceTableList).toHaveBeenCalledWith({
      page: 2,
      page_size: 10,
      id: 'device-1'
    })
  })
})
