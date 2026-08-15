/**
 * 文件用途：覆盖 index 在 协议服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
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

vi.mock('@aetherlink/hooks', () => ({
  useBoolean: (init = false) => {
    const bool = ref(init)
    return {
      bool,
      setTrue: vi.fn(() => {
        bool.value = true
      }),
      setFalse: vi.fn(() => {
        bool.value = false
      })
    }
  },
  useLoading: (init = false) => {
    const loading = ref(init)
    return {
      loading,
      startLoading: vi.fn(() => {
        loading.value = true
      }),
      endLoading: vi.fn(() => {
        loading.value = false
      })
    }
  }
}))

vi.mock('../components/table-action-modal.vue', () => ({
  default: defineComponent({
    name: 'TableActionModalStub',
    setup() {
      return () => h('div', { class: 'table-action-modal-stub' })
    }
  })
}))

import ApplyServicePage from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(ApplyServicePage, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const mockService = (overrides: Record<string, any> = {}) => ({
  id: 'service-1',
  name: 'Modbus service',
  device_type: '1',
  protocol_type: 'MODBUS',
  access_address: 'tcp://127.0.0.1:502',
  http_address: 'http://127.0.0.1:8080',
  sub_topic_prefix: 'modbus',
  description: 'service description',
  ...overrides
})

describe('apply/service/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchProtocolPluginList.mockResolvedValue({
      data: {
        list: [mockService()],
        total: 1
      }
    })
    hoisted.delProtocolPlugin.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads protocol plugin services on init and updates pagination total', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(hoisted.fetchProtocolPluginList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10
    })
    expect(state.tableData).toHaveLength(1)
    expect(state.tableData[0].id).toBe('service-1')
    expect(state.pagination.itemCount).toBe(1)
    expect(state.loading).toBe(false)
  })

  it('opens add modal and clears modal type to add', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.handleAddTable()

    expect(state.visible).toBe(true)
    expect(state.modalType).toBe('add')
  })

  it('opens edit modal with the selected row data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.handleEditTable('service-1')

    expect(state.visible).toBe(true)
    expect(state.modalType).toBe('edit')
    expect(state.editData).toEqual(expect.objectContaining({ id: 'service-1' }))
  })

  it('opens edit modal without stale edit data when the row is missing', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.handleEditTable('missing-service')

    expect(state.visible).toBe(true)
    expect(state.modalType).toBe('edit')
    expect(state.editData).toBeNull()
  })

  it('deletes a service and refreshes the list on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.delProtocolPlugin.mockResolvedValue({ error: null })
    hoisted.fetchProtocolPluginList.mockResolvedValue({
      data: {
        list: [],
        total: 0
      }
    })

    const state = getSetupState(wrapper)
    await state.handleDeleteTable('service-1')
    await flushPromises()

    expect(hoisted.delProtocolPlugin).toHaveBeenCalledWith('service-1')
    expect(hoisted.fetchProtocolPluginList).toHaveBeenCalledTimes(1)
    expect(state.tableData).toHaveLength(0)
    expect(state.pagination.itemCount).toBe(0)
  })

  it('does not refresh the list when delete returns an error', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.delProtocolPlugin.mockResolvedValue({ error: 'delete failed' })

    const state = getSetupState(wrapper)
    await state.handleDeleteTable('service-1')

    expect(hoisted.delProtocolPlugin).toHaveBeenCalledWith('service-1')
    expect(hoisted.fetchProtocolPluginList).toHaveBeenCalledTimes(0)
  })

  it('updates pagination page and page size without mutating query params until fetch', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.pagination.onChange(3)
    state.pagination.onUpdatePageSize(20)

    expect(state.pagination.page).toBe(1)
    expect(state.pagination.pageSize).toBe(20)
    expect(state.queryParams).toEqual({
      page: 1,
      page_size: 10
    })
  })
})
