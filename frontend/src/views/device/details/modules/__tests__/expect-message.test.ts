/**
 * 文件用途: expect-message 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  expectMessageList: vi.fn(),
  expectMessageDelete: vi.fn()
}))

vi.mock('@/service/api', () => ({
  expectMessageList: hoisted.expectMessageList,
  expectMessageDelete: hoisted.expectMessageDelete
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 12:00:00') }))
}))

import Component from '../expect-message.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'device-1', ...props },
    global: {
      stubs: {
        NForm: defineComponent({ props: ['inline', 'labelPlacement', 'labelAlign', 'labelWidth'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NSelect: defineComponent({ props: ['value', 'options', 'clearable', 'placeholder'], emits: ['update:value'], setup() { return () => h('div') } }),
        NInput: defineComponent({ props: ['value', 'placeholder'], emits: ['update:value'], setup() { return () => h('input') } }),
        NButton: defineComponent({ props: ['type'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NDataTable: defineComponent({ props: ['columns', 'data', 'pagination', 'remote'], setup() { return () => h('table') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/expect-message.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.expectMessageList.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.expectMessageDelete.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes pending expected-message query and option contracts', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.query).toMatchObject({
      status: 'pending',
      type: null,
      label: null,
      page: 1,
      page_size: 10
    })
    expect(state.statusOptions.map((option: any) => option.value)).toEqual(['pending', 'sent', 'expired'])
    expect(state.typeOptions.map((option: any) => option.value)).toEqual(['telemetry', 'attribute', 'command'])
    expect(hoisted.expectMessageList).toHaveBeenCalledWith({
      device_id: 'device-1',
      send_type: null,
      status: 'pending',
      type: null,
      label: null,
      page: 1,
      page_size: 10
    })
  })

  it('fetches table data on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.expectMessageList).toHaveBeenCalledTimes(1)
  })

  it('passes device_id to API call', async () => {
    mountComponent({ id: 'dev-123' })
    await flushPromises()
    expect(hoisted.expectMessageList).toHaveBeenCalledWith(expect.objectContaining({ device_id: 'dev-123' }))
  })

  it('populates tableData on successful fetch', async () => {
    hoisted.expectMessageList.mockResolvedValue({
      data: { list: [{ id: '1', status: 'pending' }], total: 1 },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
  })

  it('handleSearch resets page and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.handleSearch()
    expect(state.pagination.page).toBe(1)
    expect(hoisted.expectMessageList).toHaveBeenCalledTimes(1)
  })

  it('handleReset resets query params', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.query.status = 'sent'
    state.query.type = 'command'
    state.query.label = 'test'
    state.handleReset()
    expect(state.query.status).toBe('pending')
    expect(state.query.type).toBeNull()
    expect(state.query.label).toBeNull()
  })

  it('handleDeleteTable calls delete API and refreshes', async () => {
    hoisted.expectMessageDelete.mockResolvedValue({ error: null })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    await state.handleDeleteTable('msg-1')
    expect(hoisted.expectMessageDelete).toHaveBeenCalledWith('msg-1')
    expect(hoisted.expectMessageList).toHaveBeenCalledTimes(1)
  })

  it('pagination onChange updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.pagination.onChange(2)
    expect(state.pagination.page).toBe(2)
    expect(hoisted.expectMessageList).toHaveBeenCalledTimes(1)
  })

  it('pagination onUpdatePageSize resets page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.pagination.onUpdatePageSize(20)
    expect(state.pagination.pageSize).toBe(20)
    expect(state.pagination.page).toBe(1)
    expect(hoisted.expectMessageList).toHaveBeenCalledTimes(1)
  })
})
