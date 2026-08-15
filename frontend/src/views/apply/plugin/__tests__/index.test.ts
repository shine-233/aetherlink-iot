/**
 * 文件用途：覆盖 index 在 接入插件管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getServices: vi.fn(),
  delRegisterService: vi.fn(),
  openServiceModal: vi.fn(),
  openServiceConfigModal: vi.fn()
}))

vi.mock('@/service/api/plugin', () => ({
  getServices: hoisted.getServices,
  delRegisterService: hoisted.delRegisterService
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../components/serviceModal.vue', () => ({
  default: defineComponent({
    name: 'ServiceModalStub',
    emits: ['get-list'],
    setup(_, { expose }) {
      expose({
        openModal: hoisted.openServiceModal
      })
      return () => h('div', { class: 'service-modal-stub' })
    }
  })
}))

vi.mock('../components/serviceConfigModal.vue', () => ({
  default: defineComponent({
    name: 'ServiceConfigModalStub',
    emits: ['get-list'],
    setup(_, { expose }) {
      expose({
        openModal: hoisted.openServiceConfigModal
      })
      return () => h('div', { class: 'service-config-modal-stub' })
    }
  })
}))

import ApplyPluginPage from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountComponent = () => {
  const wrapper = mount(ApplyPluginPage, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: '' }, options: { type: Array, default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof mount>) => wrapper.vm.$.setupState as Record<string, any>

const mockPlugin = (overrides: Record<string, any> = {}) => ({
  id: 'plugin-1',
  name: 'MQTT plugin',
  service_type: 1,
  description: 'plugin description',
  version: '1.0.0',
  service_heartbeat: 1,
  ...overrides
})

describe('apply/plugin/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getServices.mockResolvedValue({
      data: {
        list: [mockPlugin()],
        total: 1
      }
    })
    hoisted.delRegisterService.mockResolvedValue({})
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads plugin services on init and fills table data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(hoisted.getServices).toHaveBeenCalledWith(state.queryInfo)
    expect(state.pageData.tableData).toHaveLength(1)
    expect(state.pageData.tableData[0].id).toBe('plugin-1')
    expect(state.queryInfo.itemCount).toBe(1)
  })

  it('opens the add plugin modal without row data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.addData()

    expect(hoisted.openServiceModal).toHaveBeenCalledWith()
  })

  it('opens edit and config modals with the selected row', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const row = mockPlugin({ id: 'plugin-2' })
    state.edit(row)
    state.config(row)

    expect(hoisted.openServiceModal).toHaveBeenCalledWith(row)
    expect(hoisted.openServiceConfigModal).toHaveBeenCalledWith(row)
  })

  it('deletes a registered service and refreshes the list', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.getServices.mockResolvedValue({
      data: {
        list: [],
        total: 0
      }
    })

    const state = getSetupState(wrapper)
    await state.del('plugin-1')
    await flushPromises()

    expect(hoisted.delRegisterService).toHaveBeenCalledWith('plugin-1')
    expect(hoisted.getServices).toHaveBeenCalledTimes(1)
    expect(state.pageData.tableData).toHaveLength(0)
    expect(state.queryInfo.itemCount).toBe(0)
  })

  it('pagination changes request the next list page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    state.queryInfo.onChange(3)
    await flushPromises()

    expect(state.queryInfo.page).toBe(3)
    expect(hoisted.getServices).toHaveBeenCalledTimes(1)
  })

  it('page size change resets to page one and reloads the list', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    state.queryInfo.page = 5
    state.queryInfo.onUpdatePageSize(20)
    await flushPromises()

    expect(state.queryInfo.page_size).toBe(20)
    expect(state.queryInfo.page).toBe(1)
    expect(hoisted.getServices).toHaveBeenCalledTimes(1)
  })

  it('reloads the list when service type filter changes', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    state.queryInfo.service_type = 2
    await nextTick()
    await flushPromises()

    expect(hoisted.getServices).toHaveBeenCalledTimes(1)
  })
})
