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
  routeQuery: {} as Record<string, unknown>
}))

vi.mock('@/service/api/system-management-user', () => ({
  getSystemLogList: hoisted.getSystemLogList
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: (v: string) => v
}))

vi.mock('../components/detail-modal.vue', () => ({
  default: defineComponent({
    setup() {
      return () => h('div')
    }
  })
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
        NCard: defineComponent({
          props: ['title'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NForm: defineComponent({
          props: ['model', 'inline', 'labelPlacement'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NFormItem: defineComponent({
          props: ['label', 'path'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NInput: defineComponent({
          props: { value: { default: '' } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: null }, options: { default: () => [] } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NDataTable: defineComponent({
          props: ['data', 'loading', 'columns'],
          setup() {
            return () => h('div')
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NDatePicker: defineComponent({
          props: { value: { default: null }, type: String },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NPagination: defineComponent({
          name: 'NPagination',
          props: ['page', 'pageSize', 'itemCount', 'showSizePicker', 'pageSizes'],
          emits: ['update:page', 'update:page-size'],
          setup() {
            return () => h('div')
          }
        })
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
    for (const key of Object.keys(hoisted.routeQuery)) delete hoisted.routeQuery[key]
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
    delete (globalThis as any).React
  })

  it('should mount and fetch table data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenCalledTimes(1)
    // 输入框默认显示最近一个月，首屏请求也必须带上同一范围（旧实现发空串，显示与查询不一致）。
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      username: '',
      start_time: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/),
      end_time: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/),
      method: '',
      path: '',
      ip: '',
      action: '',
      entity_type: '',
      entity_id: ''
    })
    const sent = hoisted.getSystemLogList.mock.calls[0][0]
    expect(sent).not.toHaveProperty('range')
    expect(getState(wrapper).tableData).toEqual([])
  })

  it('should populate table data on successful fetch', async () => {
    const mockData = [
      {
        id: '1',
        created_at: '2024-01-01',
        ip: '127.0.0.1',
        path: '/api/test',
        name: 'POST',
        latency: 100,
        username: 'admin'
      }
    ]
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
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        username: 'admin',
        method: 'POST',
        ip: '127.0.0.1'
      })
    )
  })

  it('should handle reset', async () => {
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: [], total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.queryParams.username = 'admin'
    state.queryParams.ip = '127.0.0.1'
    state.queryParams.method = 'POST'
    state.queryParams.action = 'create'
    state.queryParams.entity_type = 'customer'
    state.queryParams.entity_id = '6ba7b810-9dad-11d1-80b4-00c04fd430c8'
    state.handleReset()
    await flushPromises()
    expect(state.queryParams.username).toBe('')
    expect(state.queryParams.ip).toBe('')
    expect(state.queryParams.method).toBe('')
    // TB-10 实体级筛选（127.sql）重置后必须一并清空
    expect(state.queryParams.action).toBe('')
    expect(state.queryParams.entity_type).toBe('')
    expect(state.queryParams.entity_id).toBe('')
  })

  it('should handle pickerChange with valid range', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    const start = new Date(2024, 0, 1, 8, 0, 0).valueOf()
    const end = new Date(2024, 1, 1, 0, 0, 0).valueOf()
    state.pickerChange([start, end])
    // 只选日期：结束时间推到当天 23:59:59.999，输入框绑定的 range 同步更新
    expect(state.queryParams.range).toEqual([start, new Date(2024, 1, 1, 23, 59, 59, 999).valueOf()])
    hoisted.getSystemLogList.mockClear()
    state.handleQuery()
    await flushPromises()
    const sent = hoisted.getSystemLogList.mock.calls[0][0]
    expect(sent.start_time).toMatch(/^2024-01-01T08:00:00/)
    expect(sent.end_time).toMatch(/^2024-02-01T23:59:59/)
  })

  it('should handle pickerChange with null range', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.pickerChange(null)
    expect(state.queryParams.range).toBeNull()
    hoisted.getSystemLogList.mockClear()
    state.handleQuery()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith(expect.objectContaining({ start_time: '', end_time: '' }))
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
  it('should reset to page 1 when searching from a later page', async () => {
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: [], total: 100 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.pagination.onUpdatePage(3)
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3 }))
    state.handleQuery()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }))
  })

  it('should bind the size picker and refetch from page 1 on page-size change', async () => {
    hoisted.getSystemLogList.mockResolvedValue({ data: { list: [], total: 100 } })
    const wrapper = mountComponent()
    await flushPromises()
    const pager = wrapper.findComponent({ name: 'NPagination' })
    expect(pager.props('showSizePicker')).toBe(true)
    expect(pager.props('pageSizes')).toEqual([10, 15, 20, 25, 30])
    expect(pager.props('itemCount')).toBe(100)

    pager.vm.$emit('update:page', 4)
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 4, page_size: 10 }))

    pager.vm.$emit('update:page-size', 25)
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, page_size: 25 }))
    expect(pager.props('pageSize')).toBe(25)
    expect(pager.props('page')).toBe(1)
  })

  it('should drop a stale page-1 response that resolves after page 2', async () => {
    const page1 = [{ id: 'p1' }]
    const page2 = [{ id: 'p2' }]
    let resolvePage1: (value: unknown) => void = () => {}
    hoisted.getSystemLogList
      .mockImplementationOnce(() => new Promise((resolve) => (resolvePage1 = resolve)))
      .mockResolvedValueOnce({ data: { list: page2, total: 20 } })
    const wrapper = mountComponent()
    const state = getState(wrapper)
    state.pagination.onUpdatePage(2)
    await flushPromises()
    expect(state.tableData).toEqual(page2)

    resolvePage1({ data: { list: page1, total: 20 } })
    await flushPromises()
    expect(state.tableData).toEqual(page2)
    expect(state.loading).toBe(false)
  })

  it('should seed filters from a ready-check deep link and reset back to plain defaults', async () => {
    Object.assign(hoisted.routeQuery, {
      source: 'ready-check',
      method: 'post',
      path: '/api/v1/device',
      start_time: '2024-03-01T00:00:00Z',
      end_time: '2024-03-02T00:00:00Z'
    })
    const wrapper = mountComponent()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'POST',
        path: '/api/v1/device',
        start_time: expect.stringMatching(/^2024-03-0[12]T/),
        end_time: expect.stringMatching(/^2024-03-0[23]T/)
      })
    )
    const state = getState(wrapper)
    expect(state.isReadyCheckAuditSearch).toBe(true)

    state.handleReset()
    await flushPromises()
    expect(state.queryParams.method).toBe('')
    expect(state.queryParams.path).toBe('')
    expect(hoisted.getSystemLogList).toHaveBeenLastCalledWith(
      expect.objectContaining({ page: 1, method: '', path: '', start_time: expect.any(String) })
    )
  })

  it('should ignore a deep-link method outside the whitelist', async () => {
    Object.assign(hoisted.routeQuery, { method: 'PATCH' })
    mountComponent()
    await flushPromises()
    expect(hoisted.getSystemLogList).toHaveBeenCalledWith(expect.objectContaining({ method: '' }))
  })
})
