/**
 * 文件用途: 覆盖测试在系统管理用户侧场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getSystemLogList: vi.fn(),
}))

vi.mock('@/service/api/system-management-user', () => ({
  getSystemLogList: hoisted.getSystemLogList,
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: (v: string) => v
}))

vi.mock('~/packages/hooks', () => ({
  useLoading: () => ({ loading: { value: false }, startLoading: vi.fn(), endLoading: vi.fn() }),
}))

vi.mock('../components/detail-modal.vue', () => ({
  default: defineComponent({ setup() { return () => h('div') } })
}))

import SystemLogIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(SystemLogIndex, {
    props,
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NCard: defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['model', 'inline', 'labelPlacement'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NDataTable: defineComponent({ props: ['data', 'loading', 'columns'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NDatePicker: defineComponent({ props: { value: { default: null }, type: String }, emits: ['update:value'], setup() { return () => h('div') } }),
        NPagination: defineComponent({ props: ['page', 'itemCount'], emits: ['update:page'], setup() { return () => h('div') } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('SystemLogIndex', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(globalThis as any).React = {
      createElement: (type: any, props: any, ...children: any[]) => h(type, props, children)
    }
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: [], total: 0 } })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
    delete (globalThis as any).React
  })

  it('should mount and fetch table data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenCalledTimes(1)
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      username: '',
      selected_time: null,
      start_time: '',
      end_time: '',
      method: '',
      path: '',
      ip: ''
    })
    expect(getState(wrapper).tableData).toEqual([])
  })

  it('should populate table data on successful fetch', async () => {
    const mockData = [{ id: '1', created_at: '2024-01-01', ip: '127.0.0.1', path: '/api/test', name: 'POST', latency: 100, username: 'admin' }]
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: mockData, total: 1 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.tableData).toEqual(mockData)
  })

  it('should handle query', async () => {
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: [], total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.queryParams.username = 'admin'
    state.queryParams.method = 'POST'
    state.queryParams.ip = '127.0.0.1'
    state.handleQuery()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      page_size: 10,
      username: 'admin',
      method: 'POST',
      ip: '127.0.0.1'
    }))
  })

  it('should handle reset', async () => {
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: [], total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.queryParams.username = 'admin'
    state.queryParams.ip = '127.0.0.1'
    state.queryParams.method = 'POST'
    state.handleReset()
    await flushPromises()
    expect(state.queryParams.username).toBe('')
    expect(state.queryParams.ip).toBe('')
    expect(state.queryParams.method).toBe('')
  })

  it('should handle pickerChange with valid range', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.pickerChange([1704067200000, 1706745600000])
    expect(state.queryParams.start_time).toMatch(/^2024-01-01T/)
    expect(state.queryParams.end_time).toMatch(/^2024-02-01T/)
  })

  it('should handle pickerChange with null range', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.pickerChange(null)
    expect(state.queryParams.start_time).toBe('')
    expect(state.queryParams.end_time).toBe('')
  })

  it('should handle detail modal ref', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const show = vi.fn()
    state.detailModalRef = { show }
    const actionColumn = state.columns.find((column: any) => column.key === '')
    const row = { id: 'log-1', path: '/api/v1/device', name: 'GET' }

    actionColumn.render(row).props.onClick()

    expect(show).toHaveBeenCalledTimes(1)
    expect(show).toHaveBeenCalledWith(row)
  })
})
