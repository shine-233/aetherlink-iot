/**
 * 文件用途：覆盖 service-index 在 协议服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchProtocolPluginList: vi.fn(),
  delProtocolPlugin: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchProtocolPluginList: hoisted.fetchProtocolPluginList,
  delProtocolPlugin: hoisted.delProtocolPlugin
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/constants/business', () => ({
  serviceManagementDeviceTypeLabels: { 1: '直连设备', 2: '网关设备' }
}))

vi.mock('../components/table-action-modal.vue', () => ({
  default: defineComponent({
    props: ['visible', 'type', 'editData'],
    emits: ['update:visible', 'success'],
    setup() { return () => h('div', { 'data-testid': 'table-action-modal' }) }
  })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        NCard: defineComponent({
          props: { title: String, bordered: Boolean, class: String },
          setup(_, { slots }) {
            return () => h('div', { class: 'n-card' }, [
              slots['header-extra'] ? h('div', { class: 'header-extra' }, slots['header-extra']()) : null,
              slots.default ? slots.default() : null
            ])
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NDataTable: defineComponent({
          props: { remote: Boolean, columns: Array, data: Array, loading: Boolean, pagination: Object },
          setup() { return () => h('div', { class: 'n-data-table' }) }
        }),
        TableActionModal: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('apply/service/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchProtocolPluginList.mockResolvedValue({
      data: { list: [], total: 0 }
    })
    hoisted.delProtocolPlugin.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('starts with service plugin query, pagination and table columns contract', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchProtocolPluginList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    expect(state.queryParams).toEqual({ page: 1, page_size: 10 })
    expect(state.pagination).toMatchObject({
      page: 1,
      pageSize: 10,
      showSizePicker: true,
      pageSizes: [10, 15, 20, 25, 30]
    })
    expect(state.columns.map((column: any) => column.key)).toEqual([
      'name',
      'device_type',
      'protocol_type',
      'access_address',
      'http_address',
      'sub_topic_prefix',
      'description',
      'actions'
    ])
  })

  it('loads table data on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchProtocolPluginList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchProtocolPluginList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
  })

  it('initializes with empty table data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toEqual([])
  })

  it('sets table data after fetch', async () => {
    const mockList = [
      { id: '1', name: 'Service 1', device_type: 1, protocol_type: 'MQTT' }
    ]
    hoisted.fetchProtocolPluginList.mockResolvedValue({
      data: { list: mockList, total: 1 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toEqual(mockList)
  })

  it('handleAddTable opens modal in add mode', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddTable()
    expect(state.modalType).toBe('add')
    expect(state.visible).toBe(true)
  })

  it('handleEditTable sets edit data and opens modal', async () => {
    const mockList = [
      { id: 'svc-1', name: 'Service 1', device_type: 1, protocol_type: 'MQTT' }
    ]
    hoisted.fetchProtocolPluginList.mockResolvedValue({
      data: { list: mockList, total: 1 }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditTable('svc-1')
    expect(state.modalType).toBe('edit')
    expect(state.editData).toEqual(mockList[0])
  })

  it('handleDeleteTable calls delProtocolPlugin', async () => {
    hoisted.delProtocolPlugin.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('svc-1')
    expect(hoisted.delProtocolPlugin).toHaveBeenCalledWith('svc-1')
  })

  it('pagination has correct initial values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.pagination.page).toBe(1)
    expect(state.pagination.pageSize).toBe(10)
  })
})
