/**
 * 文件用途: 覆盖Device Select List在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceGroupRelation: vi.fn(),
  deviceList: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceGroupRelation: hoisted.deviceGroupRelation,
  deviceList: hoisted.deviceList
}))

vi.mock('@/views/device/modules/all-columns', () => ({
  createDeviceColumns: vi.fn(() => [])
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } })
}))

import Component from '../device-select-list.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { groupId: 'grp-1', ...props },
    global: {}
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/grouping-details/modules/device-select-list.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceGroupRelation.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads selectable devices and filters out devices already in the current group', async () => {
    hoisted.deviceList.mockResolvedValue({
      data: {
        list: [
          { id: 'dev-1', name: 'Bound Device', group_id: 'grp-1' },
          { id: 'dev-2', name: 'Free Device', group_id: 'other-group' }
        ],
        total: 11
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.deviceList).toHaveBeenCalledWith({ page: 1, page_size: 5, search: undefined })
    expect(state.data).toEqual([{ id: 'dev-2', name: 'Free Device', group_id: 'other-group' }])
    expect(state.pagination.pageCount).toBe(3)
    expect(state.rowKey({ id: 'dev-2' })).toBe('dev-2')
  })

  it('loads device list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceList).toHaveBeenCalledTimes(1)
  })

  it('handleSearch resets page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleSearch()
    await flushPromises()
    expect(state.pagination.page).toBe(1)
  })

  it('handleReset clears keyword and searches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.searchKeyword = 'test'
    state.handleReset()
    await flushPromises()
    expect(state.searchKeyword).toBe('')
  })

  it('closeModal emits closedModal', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.closeModal()
    expect(wrapper.emitted('closedModal')).toEqual([[false]])
  })

  it('reload emits refreshData', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.reload()
    expect(wrapper.emitted('refreshData')).toEqual([[true]])
  })
})
