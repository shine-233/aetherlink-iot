/**
 * 文件用途: 测试自定义控制配置步骤。
 * 核心逻辑: 模拟列表、脚本编辑和保存接口，验证控制项管理流程。
 * 关键注意事项: 自定义脚本测试要避免只断言输入框存在，应验证提交载荷。
 * 重构建议: 与自定义命令测试共享脚本编辑器和分页 mock。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceCustomControlAdd: vi.fn(),
  deviceCustomControlDel: vi.fn(),
  deviceCustomControlList: vi.fn(),
  deviceCustomControlPut: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  deviceCustomControlAdd: hoisted.deviceCustomControlAdd,
  deviceCustomControlDel: hoisted.deviceCustomControlDel,
  deviceCustomControlList: hoisted.deviceCustomControlList,
  deviceCustomControlPut: hoisted.deviceCustomControlPut
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/theme', () => ({
  useThemeStore: () => ({ theme: 'light' })
}))

vi.mock('@/utils/common/tool', () => ({
  isJSON: vi.fn(() => true)
}))

vi.mock('vue-codemirror6', () => ({
  default: defineComponent({ setup() { return () => h('div') } })
}))

vi.mock('@codemirror/lang-javascript', () => ({
  javascript: vi.fn()
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } }),
  NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NPagination: defineComponent({ props: { page: { default: 1 } }, emits: ['update:page'], setup() { return () => h('div') } }),
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } })
}))

import Component from '../custom-controls.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { id: 'tpl-1', ...props },
    global: {
      mocks: {
        getPlatform: () => false
      },
      stubs: {
        NFlex: defineComponent({ props: ['justify'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        'n-card': defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        'n-switch': defineComponent({ props: ['value', 'checkedValue', 'uncheckedValue'], emits: ['update:value'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/components/step/custom-controls.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceCustomControlList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceCustomControlAdd.mockResolvedValue({ error: null })
    hoisted.deviceCustomControlPut.mockResolvedValue({ error: null })
    hoisted.deviceCustomControlDel.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes custom telemetry control list query, form defaults and table columns', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.deviceCustomControlList).toHaveBeenCalledWith({
      page: 1,
      page_size: 100,
      device_template_id: 'tpl-1'
    })
    expect(state.commandjson).toMatchObject({
      configForm: false,
      listData: [],
      total: 0,
      queryjson: {
        page: 1,
        page_size: 100
      },
      formjson: {
        name: '',
        description: '',
        content: '{}',
        enable_status: 'disable'
      }
    })
    expect(state.configFormRules).toMatchObject({
      name: {
        required: true,
        message: 'generate.btnname',
        trigger: 'blur'
      }
    })
    expect(state.columns.map((column: any) => column.key)).toEqual([
      'name',
      'content',
      'enable_status',
      'description',
      'actions'
    ])
  })

  it('loads control list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceCustomControlList).toHaveBeenCalledTimes(1)
  })

  it('getControlList fetches data with page', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.getControlList(2)
    await flushPromises()
    expect(hoisted.deviceCustomControlList).toHaveBeenCalledWith(expect.objectContaining({ page: 2, device_template_id: 'tpl-1' }))
  })
})
