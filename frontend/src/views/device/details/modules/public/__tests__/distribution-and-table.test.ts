/**
 * 文件用途: distribution-and-table 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  commandDataById: vi.fn(),
  commandDataPub: vi.fn(),
  deviceCustomCommandsIdList: vi.fn(),
  getAttributeDataSet: vi.fn(),
  fetchDataApi: vi.fn(),
  submitApi: vi.fn(),
  expectApi: vi.fn()
}))

vi.mock('@/service/api', () => ({
  commandDataById: hoisted.commandDataById,
  commandDataPub: hoisted.commandDataPub,
  deviceCustomCommandsIdList: hoisted.deviceCustomCommandsIdList,
  getAttributeDataSet: hoisted.getAttributeDataSet
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/tool', () => ({
  isJSON: vi.fn((str: string) => { try { JSON.parse(str); return true } catch { return false } })
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({ info: vi.fn(), error: vi.fn(), warn: vi.fn() })
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: (init = false) => {
    const loading = ref(init)
    return { loading, startLoading: vi.fn(() => { loading.value = true }), endLoading: vi.fn(() => { loading.value = false }) }
  }
}))

vi.mock('@vicons/ionicons5', () => ({
  Refresh: defineComponent({ setup: () => () => h('span') })
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01 00:00:00') }))
}))

import { ref } from 'vue'
import Component from '../distribution-and-table.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (propsOverrides = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      id: 'device-1',
      tableColumns: [],
      fetchDataApi: hoisted.fetchDataApi,
      submitApi: hoisted.submitApi,
      expectApi: hoisted.expectApi,
      expect: true,
      ...propsOverrides
    },
    global: {
      mocks: {
        getPlatform: () => false
      },
      stubs: {
        NButton: defineComponent({ props: ['type', 'bordered', 'size', 'disabled'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NIcon: defineComponent({ props: ['size'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NDataTable: defineComponent({ props: ['loading', 'columns', 'data'], setup() { return () => h('table') } }),
        NPagination: defineComponent({ props: ['pageCount', 'page', 'pageSize', 'prefix'], emits: ['update:page'], setup() { return () => h('div') } }),
        Pagination: defineComponent({ props: ['pageCount', 'page', 'pageSize', 'prefix'], emits: ['update:page'], setup() { return () => h('div') } }),
        NModal: defineComponent({ props: ['show'], emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NCard: defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        'n-card': defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['model', 'rules', 'labelPlacement'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path', 'required', 'validationStatus', 'feedback'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'type', 'placeholder', 'disabled'], emits: ['update:value'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options', 'filterable', 'tag', 'clearable', 'placeholder'], emits: ['update:value', 'update:show'], setup() { return () => h('div') } }),
        NTabs: defineComponent({ props: ['value'], emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NTabPane: defineComponent({ props: ['name', 'tab'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFlex: defineComponent({ props: ['justify'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInputNumber: defineComponent({ props: ['value', 'showButton'], emits: ['update:value'], setup() { return () => h('input') } }),
        NSwitch: defineComponent({ props: ['value'], emits: ['update:value'], setup() { return () => h('div') } }),
        NGrid: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGridItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NCheckbox: defineComponent({ props: ['checked', 'indeterminate'], emits: ['update:checked'], setup(_, { slots }) { return () => h('label', slots.default?.()) } }),
        SvgIcon: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/details/modules/public/distribution-and-table.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchDataApi.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
    hoisted.submitApi.mockResolvedValue({ error: null })
    hoisted.expectApi.mockResolvedValue({ error: null })
    hoisted.commandDataById.mockResolvedValue({ data: [] })
    hoisted.commandDataPub.mockResolvedValue({})
    hoisted.deviceCustomCommandsIdList.mockResolvedValue({ data: [] })
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [], error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes paged device distribution table query and default dialog state', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.fetchDataApi).toHaveBeenCalledWith({
      page: 1,
      page_size: 4,
      device_id: 'device-1'
    })
    expect(state.the_page).toBe(1)
    expect(state.page_coune).toBe(0)
    expect(state.tableData).toEqual([])
    expect(state.formModel).toMatchObject({
      commandValue: '',
      textValue: '',
      expected: false,
      time: null,
      timeoutSeconds: 10,
      waitForResponse: false
    })
    expect(state.activeTab).toBe('visual')
  })

  it('calls fetchDataApi on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchDataApi).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchDataApi).toHaveBeenCalledWith({
      page: 1,
      page_size: 4,
      device_id: 'device-1'
    })
  })

  it('attribute dispatch dialog loads attribute set with the device_id payload contract', async () => {
    const wrapper = mountComponent({ isCommand: false })
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [], error: null })

    await state.openDialog()
    await flushPromises()

    // 回归锚点：历史上这里误传裸字符串 id，实际请求 /attribute/datas/undefined。
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledWith({ device_id: 'device-1' })
    expect(state.showDialog).toBe(true)
  })

  it('refresh resets page and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.refresh()
    await flushPromises()
    expect(state.the_page).toBe(1)
    expect(hoisted.fetchDataApi).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchDataApi).toHaveBeenCalledWith({
      page: 1,
      page_size: 4,
      device_id: 'device-1'
    })
  })

  it('updatePage updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    vi.clearAllMocks()
    state.updatePage(3)
    await flushPromises()
    expect(state.the_page).toBe(3)
    expect(hoisted.fetchDataApi).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchDataApi).toHaveBeenCalledWith({
      page: 3,
      page_size: 4,
      device_id: 'device-1'
    })
  })

  it('openDialog sets showDialog to true', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.openDialog()
    expect(state.showDialog).toBe(true)
  })

  it('closeDialog resets form state', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.showDialog = true
    state.formModel.textValue = 'test'
    state.closeDialog()
    expect(state.showDialog).toBe(false)
    expect(state.formModel.textValue).toBe('')
  })

  it('exposes refresh method', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    expect(typeof wrapper.vm.refresh).toBe('function')
  })

  it('fetches command list when isCommand is true', async () => {
    mountComponent({ isCommand: true })
    await flushPromises()
    expect(hoisted.deviceCustomCommandsIdList).toHaveBeenCalledWith('device-1')
  })

  it('does not fetch command list when isCommand is false', async () => {
    mountComponent({ isCommand: false })
    await flushPromises()
    expect(hoisted.deviceCustomCommandsIdList).toHaveBeenCalledTimes(0)
  })

  it('onCommandChange calls commandDataPub', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.onCommandChange({ instruct: 'cmd1', data_identifier: 'id1' })
    await flushPromises()
    expect(hoisted.commandDataPub).toHaveBeenCalledWith({
      device_id: 'device-1',
      value: 'cmd1',
      identify: 'id1'
    })
  })
})
