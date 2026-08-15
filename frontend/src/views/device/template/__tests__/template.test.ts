/**
 * 文件用途: 测试物模型主流程的页面行为。
 * 核心逻辑: 挂载物模型页面并模拟接口、路由和用户操作，验证列表、弹窗和数据刷新是否按预期工作。
 * 关键注意事项: Mock 字段要贴近真实物模型接口，避免只验证组件能渲染。
 * 重构建议: 将通用物模型 fixture 和挂载逻辑抽成 helper，减少测试之间的重复。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deleteDeviceTemplate: vi.fn(),
  deviceTemplate: vi.fn(),
  messageError: vi.fn(),
  messageSuccess: vi.fn(),
  routeQuery: {} as Record<string, unknown>
}))

vi.mock('@/service/api/device-template-model', () => ({
  deleteDeviceTemplate: hoisted.deleteDeviceTemplate,
  deviceTemplate: hoisted.deviceTemplate
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/discrete', () => ({
  message: { error: hoisted.messageError, success: hoisted.messageSuccess }
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery })
}))

vi.mock('~/packages/hooks/src', () => {
  // Vitest hoists this mock before ESM imports, so use a local require here.
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  const { ref } = require('vue')
  return {
  useBoolean: (init = false) => {
    const bool = ref(init)
    return { bool, setTrue: vi.fn(() => { bool.value = true }), setFalse: vi.fn(() => { bool.value = false }), toggle: vi.fn() }
  },
  useLoading: (init = false) => {
    const loading = ref(init)
    return { loading, startLoading: vi.fn(() => { loading.value = true }), endLoading: vi.fn(() => { loading.value = false }) }
  }
  }
})

vi.mock('@/utils/common/tool', () => ({
  getPlatformApiBaseUrl: () => 'http://localhost/api/v1'
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NIcon: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NPagination: defineComponent({ props: { page: { default: 1 }, itemCount: { default: 0 } }, emits: ['update:page'], setup() { return () => h('div') } }),
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
  NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NEmpty: defineComponent({ setup() { return () => h('div') } }),
  NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NGi: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSpin: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

vi.mock('@vicons/ionicons5', () => ({
  SearchOutline: defineComponent({ setup: () => () => h('div') }),
  ListOutline: defineComponent({ setup: () => () => h('div') }),
  GridOutline: defineComponent({ setup: () => () => h('div') })
}))

import Component from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props,
    global: {
      stubs: {
        AdvancedListLayout: true,
        ItemCard: true,
        SvgIcon: true,
        TemplateModal: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/index.vue', () => {
  beforeEach(() => {
    vi.useRealTimers()
    vi.clearAllMocks()
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.deleteDeviceTemplate.mockResolvedValue({ error: null })
    Object.keys(hoisted.routeQuery).forEach(key => {
      delete hoisted.routeQuery[key]
    })
    ;(window as any).$message = {
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes template list query, view modes and first-page API contract', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.queryParams).toMatchObject({
      page: 1,
      page_size: 10,
      name: ''
    })
    expect(hoisted.deviceTemplate).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      name: ''
    })
    expect(state.deviceTemplateList).toEqual([])
    expect(state.dataTotal).toBe(0)
    expect(state.modalType).toBe('add')
    expect(state.templateId).toBe('')
    expect(state.availableViews.map((view: any) => view.key)).toEqual(['card', 'list'])
  })

  it('loads data on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(1)
  })

  it('opens edit modal from route query id on mount', async () => {
    vi.useFakeTimers()
    hoisted.routeQuery.id = 'route-template-1'

    const wrapper = mountComponent()
    await flushPromises()
    vi.runAllTimers()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.modalType).toBe('edit')
    expect(state.templateId).toBe('route-template-1')
  })

  it('populates deviceTemplateList on successful fetch', async () => {
    hoisted.deviceTemplate.mockResolvedValue({
      data: {
        list: [
          { id: '1', name: 'Template A', description: 'Desc A', label: 'tag1,tag2', created_at: '2024-01-01' }
        ],
        total: 1
      },
      error: null
    })

    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.deviceTemplateList).toHaveLength(1)
    expect(state.dataTotal).toBe(1)
  })

  it('handleQuery resets page and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleQuery()
    expect(state.queryParams.page).toBe(1)
  })

  it('handleReset clears name and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.queryParams.name = 'test'
    await state.handleReset()
    expect(state.queryParams.name).toBe('')
  })

  it('handleAddNew sets modal type to add and opens modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddNew()
    expect(state.modalType).toBe('add')
    expect(state.templateId).toBe('')
  })

  it('handleEdit sets modal type to edit with id', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEdit('tpl-1')
    expect(state.modalType).toBe('edit')
    expect(state.templateId).toBe('tpl-1')
  })

  it('handleRemove calls deleteDeviceTemplate and shows success on success', async () => {
    hoisted.deleteDeviceTemplate.mockResolvedValue({ error: null })

    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    await state.handleRemove('tpl-1')
    await flushPromises()

    expect(hoisted.deleteDeviceTemplate).toHaveBeenCalledWith('tpl-1')
    expect((window as any).$message.success).toHaveBeenCalledWith('common.templateDeleted')
  })

  it('handleRemove shows error message on failure', async () => {
    hoisted.deleteDeviceTemplate.mockResolvedValue({ error: { message: 'fail' } })

    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    await state.handleRemove('tpl-1')
    await flushPromises()

    expect((window as any).$message.error).toHaveBeenCalledWith('common.deleteFailed')
  })

  it('handlePageChange updates page and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    state.handlePageChange(3)
    await flushPromises()

    expect(state.queryParams.page).toBe(3)
    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceTemplate).toHaveBeenCalledWith({ page: 3, page_size: 10, name: '' })
  })

  it('handlePageSizeChange resets page to 1 and updates page size', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    state.handlePageSizeChange(20)
    await flushPromises()

    expect(state.queryParams.page_size).toBe(20)
    expect(state.queryParams.page).toBe(1)
    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceTemplate).toHaveBeenCalledWith({ page: 1, page_size: 20, name: '' })
  })

  it('handleRefresh fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()

    const state = getSetupState(wrapper)
    state.handleRefresh()
    await flushPromises()

    expect(hoisted.deviceTemplate).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceTemplate).toHaveBeenCalledWith({ page: 1, page_size: 10, name: '' })
  })

  it('getTagArray splits label string into tags', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const tags = state.getTagArray('tag1, tag2, tag3')
    expect(tags).toEqual(['tag1', 'tag2', 'tag3'])
  })

  it('getTagArray returns empty array for empty label', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const tags = state.getTagArray('')
    expect(tags).toEqual([])
  })

  it('getDisplayTags returns display tags and more count', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const result = state.getDisplayTags('tag1, tag2, tag3, tag4, tag5')
    expect(result.displayTags).toHaveLength(3)
    expect(result.hasMore).toBe(true)
    expect(result.moreCount).toBe(2)
  })

  it('getDisplayTags returns all tags when count is 3 or less', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const result = state.getDisplayTags('tag1, tag2')
    expect(result.displayTags).toHaveLength(2)
    expect(result.hasMore).toBe(false)
    expect(result.moreCount).toBe(0)
  })

  it('columns include name, description, label, created_at, and actions', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const state = getSetupState(wrapper)
    const columnKeys = state.columns.map((c: any) => c.key)
    expect(columnKeys).toContain('name')
    expect(columnKeys).toContain('description')
    expect(columnKeys).toContain('label')
    expect(columnKeys).toContain('created_at')
    expect(columnKeys).toContain('actions')
  })

  it('discrete message error mock is available', () => {
    mountComponent()
    expect(hoisted.messageError).toHaveBeenCalledTimes(0)
    hoisted.messageError('test error')
    expect(hoisted.messageError).toHaveBeenCalledWith('test error')
  })

  it('discrete message success mock is available', () => {
    mountComponent()
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(0)
    hoisted.messageSuccess('test success')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('test success')
  })
})
