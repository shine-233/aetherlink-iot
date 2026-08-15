/**
 * 文件用途：覆盖 index 在 告警通知记录 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getNotificationHistoryList: vi.fn(),
}))

vi.mock('@/service/api/notification', () => ({
  getNotificationHistoryList: hoisted.getNotificationHistoryList,
}))

vi.mock('@/constants/business', () => ({
  notificationOptions: [
    { label: 'Email', value: 'email' },
    { label: 'SMS', value: 'sms' }
  ]
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: (v: string) => v
}))

vi.mock('~/packages/hooks', () => ({
  useLoading: (initial: boolean) => {
    const loading = vi.fn(() => false) as any
    loading.value = false
    return {
      loading: { value: false },
      startLoading: vi.fn(),
      endLoading: vi.fn()
    }
  }
}))

import NotificationRecord from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(NotificationRecord, {
    props,
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NCard: defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['model'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NDataTable: defineComponent({ props: ['data', 'loading', 'pagination', 'columns', 'remote'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NDatePicker: defineComponent({ props: { value: { default: null }, type: String }, emits: ['update:value'], setup() { return () => h('div') } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('NotificationRecord', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getNotificationHistoryList.mockResolvedValue({ data: { list: [], total: 0 } })
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount and fetch table data with the default one-month time range', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getNotificationHistoryList).toHaveBeenCalledTimes(1)
    expect(hoisted.getNotificationHistoryList).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      page_size: 10,
      notification_type: '',
      send_target: '',
      send_time_start: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/),
      send_time_stop: expect.stringMatching(/^\d{4}-\d{2}-\d{2}T/)
    }))
  })

  it('should populate table data on successful fetch', async () => {
    const mockData = [{ send_time: '2024-01-01', send_content: 'test', send_target: 'user', send_result: 'ok', notification_type: 'email' }]
    hoisted.getNotificationHistoryList.mockResolvedValue({ data: { list: mockData, total: 1 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    expect(state.tableData).toEqual(mockData)
    expect(state.pagination.itemCount).toBe(1)
  })

  it('should handle query with search params', async () => {
    hoisted.getNotificationHistoryList.mockResolvedValue({ data: { list: [], total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    const state = getState(wrapper)
    state.queryParams.notification_type = 'email'
    state.queryParams.send_target = 'ops@example.com'
    state.handleQuery()
    await flushPromises()
    expect(hoisted.getNotificationHistoryList).toHaveBeenCalledWith(expect.objectContaining({
      page: 1,
      page_size: 10,
      notification_type: 'email',
      send_target: 'ops@example.com'
    }))
  })

  it('should reset search params', async () => {
    hoisted.getNotificationHistoryList.mockResolvedValue({ data: { list: [], total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.queryParams.notification_type = 'email'
    state.queryParams.send_target = 'user'
    state.handleReset()
    await flushPromises()
    expect(state.queryParams.notification_type).toBe('')
    expect(state.queryParams.send_target).toBe('')
  })

  it('should update date range on pickerChange', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.range = [1704067200000, 1706745600000]
    state.pickerChange()
    expect(state.queryParams.send_time_start).toMatch(/^2024-01-01T/)
    expect(state.queryParams.send_time_end).toMatch(/^2024-02-01T/)
  })

  it('should clear date range when range is empty', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getState(wrapper)
    state.range = null as any
    state.pickerChange()
    expect(state.queryParams.send_time_start).toBe('')
    expect(state.queryParams.send_time_end).toBe('')
  })
})
