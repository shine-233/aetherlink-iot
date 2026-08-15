/**
 * 文件用途: settings 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deleteDeviceGroupRelation: vi.fn(),
  deleteDevice: vi.fn(),
  deviceDetail: vi.fn(),
  deviceGroupRelation: vi.fn(),
  deviceGroupTree: vi.fn(),
  deviceUpdateConfig: vi.fn(),
  getDeviceConfigList: vi.fn(),
  getDeviceGroupRelation: vi.fn(),
  fetchData: vi.fn(),
  removeTab: vi.fn()
}))

vi.mock('@/service/api', () => ({
  deleteDeviceGroupRelation: hoisted.deleteDeviceGroupRelation,
  deleteDevice: hoisted.deleteDevice,
  deviceDetail: hoisted.deviceDetail,
  deviceGroupRelation: hoisted.deviceGroupRelation,
  deviceGroupTree: hoisted.deviceGroupTree,
  deviceUpdateConfig: hoisted.deviceUpdateConfig,
  getDeviceConfigList: hoisted.getDeviceConfigList,
  getDeviceGroupRelation: hoisted.getDeviceGroupRelation
}))

vi.mock('@/store/modules/device', () => ({
  useDeviceDataStore: () => ({
    fetchData: hoisted.fetchData,
    deviceData: { device_config_id: 'cfg-1', current_version: '1.0', access_way: 'A' }
  })
}))

vi.mock('@/store/modules/tab', () => ({
  useTabStore: () => ({
    removeTab: hoisted.removeTab
  })
}))

vi.mock('@/store/modules/tab/shared', () => ({
  getTabIdByRoute: vi.fn(() => 'tab-1')
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { d_id: 'device-1' } })
}))

import Component from '../settings.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', online: '1', ...props },
    global: {
      stubs: {
        NSelect: defineComponent({ props: ['value', 'options', 'filterable'], emits: ['update:value', 'search'], setup() { return () => h('div') } }),
        NButton: defineComponent({ props: ['type', 'size'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NTransfer: defineComponent({ props: ['value', 'options', 'sourceFilterable'], emits: ['update:value'], setup() { return () => h('div') } }),
        NTree: defineComponent({ props: ['data', 'checkedKeys', 'checkable', 'defaultExpandAll'], emits: ['update:checkedKeys'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/settings.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.$message = { success: vi.fn(), error: vi.fn(), warning: vi.fn(), info: vi.fn() }
    window.$dialog = { warning: vi.fn(), success: vi.fn(), error: vi.fn(), info: vi.fn() }
    hoisted.deviceDetail.mockResolvedValue({ data: { device_number: 'DN001', device_config_id: 'cfg-1' } })
    hoisted.deviceGroupTree.mockResolvedValue({ data: [], error: null })
    hoisted.getDeviceGroupRelation.mockResolvedValue({ data: [], error: null })
    hoisted.getDeviceConfigList.mockResolvedValue({ data: { list: [] }, error: null })
    hoisted.fetchData.mockResolvedValue(undefined)
    hoisted.deleteDevice.mockResolvedValue({})
    hoisted.deleteDeviceGroupRelation.mockResolvedValue({})
    hoisted.deviceGroupRelation.mockResolvedValue({})
    hoisted.deviceUpdateConfig.mockResolvedValue({})
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads device settings, group relation and config options on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceDetail).toHaveBeenCalledWith('device-1')
    expect(hoisted.deviceGroupTree).toHaveBeenCalledWith({})
    expect(hoisted.getDeviceGroupRelation).toHaveBeenCalledWith({ device_id: 'device-1' })
    expect(hoisted.getDeviceConfigList).toHaveBeenCalledWith({
      page: 1,
      page_size: 99,
      name: ''
    })
    expect(state.device_coding).toBe('DN001')
    expect(state.selectedValues).toBe('cfg-1')
    expect(state.sOptions).toEqual([{ label: 'generate.unbind', value: '' }])
  })

  it('fetches device detail on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceDetail).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceDetail).toHaveBeenCalledWith('device-1')
  })

  it('fetches device group tree on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceGroupTree).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceGroupTree).toHaveBeenCalledWith({})
  })

  it('fetches device group relation on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDeviceGroupRelation).toHaveBeenCalledWith({ device_id: 'device-1' })
  })

  it('fetches device config list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDeviceConfigList).toHaveBeenCalledTimes(1)
    expect(hoisted.getDeviceConfigList).toHaveBeenCalledWith({
      page: 1,
      page_size: 99,
      name: ''
    })
  })

  it('selectConfig calls deviceUpdateConfig', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.selectConfig('cfg-new')
    expect(hoisted.deviceUpdateConfig).toHaveBeenCalledWith({
      device_id: 'device-1',
      device_config_id: 'cfg-new'
    })
  })

  it('selectConfig emits change event', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.selectConfig('cfg-new')
    expect(wrapper.emitted('change')).toEqual([[]])
  })

  it('handleDeleteDevice shows confirmation dialog', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleDeleteDevice()
    expect(window.$dialog?.warning).toHaveBeenCalledTimes(1)
    expect(window.$dialog?.warning).toHaveBeenCalledWith(
      expect.objectContaining({
        title: expect.any(String),
        onPositiveClick: expect.any(Function)
      })
    )
  })

  it('transformDataToOptions transforms tree data correctly', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const input = [
      { group: { name: 'Group1', id: '1' }, children: [
        { group: { name: 'Child1', id: '2' }, children: [] }
      ]}
    ]
    const result = state.transformDataToOptions(input)
    expect(result).toHaveLength(1)
    expect(result[0].label).toBe('Group1')
    expect(result[0].children).toHaveLength(1)
  })

  it('flattenTree flattens tree correctly', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const tree = [
      { label: 'A', value: '1', children: [
        { label: 'B', value: '2', children: undefined }
      ]}
    ]
    const result = state.flattenTree(tree)
    expect(result).toHaveLength(2)
  })
})
