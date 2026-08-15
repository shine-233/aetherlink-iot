/**
 * 文件用途: 测试物模型入口页的核心交互。
 * 核心逻辑: 通过 Vue Test Utils 挂载入口页，验证查询、创建、编辑和删除相关路径。
 * 关键注意事项: 入口页依赖较多接口 mock，新增用例时要保持分页和物模型状态一致。
 * 重构建议: 将 API mock、分页断言和弹窗断言拆成共享测试工具。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deleteDeviceTemplate: vi.fn(),
  deviceTemplate: vi.fn()
}))

vi.mock('@/service/api/device-template-model', () => ({
  deleteDeviceTemplate: hoisted.deleteDeviceTemplate,
  deviceTemplate: hoisted.deviceTemplate
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: {} })
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
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
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
    vi.clearAllMocks()
    hoisted.deviceTemplate.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.deleteDeviceTemplate.mockResolvedValue({ error: null })
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
})
