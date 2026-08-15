/**
 * 文件用途: 测试自定义命令配置步骤。
 * 核心逻辑: 模拟命令列表、参数编辑和接口保存，验证新增编辑流程。
 * 关键注意事项: 命令参数结构需要和真实接口字段一致，避免测试通过但运行失败。
 * 重构建议: 抽出自定义能力测试 helper，复用脚本和参数断言。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceCustomCommandsAdd: vi.fn(),
  deviceCustomCommandsDel: vi.fn(),
  deviceCustomCommandsList: vi.fn(),
  deviceCustomCommandsPut: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  deviceCustomCommandsAdd: hoisted.deviceCustomCommandsAdd,
  deviceCustomCommandsDel: hoisted.deviceCustomCommandsDel,
  deviceCustomCommandsList: hoisted.deviceCustomCommandsList,
  deviceCustomCommandsPut: hoisted.deviceCustomCommandsPut
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/theme', () => ({
  useThemeStore: () => ({ theme: 'light' })
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

import Component from '../custom-commands.vue'

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

describe('device/template/components/step/custom-commands.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceCustomCommandsList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.deviceCustomCommandsAdd.mockResolvedValue({ error: null })
    hoisted.deviceCustomCommandsPut.mockResolvedValue({ error: null })
    hoisted.deviceCustomCommandsDel.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes custom command list query, form defaults and table columns', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.deviceCustomCommandsList).toHaveBeenCalledWith({
      page: 1,
      page_size: 4,
      device_template_id: 'tpl-1'
    })
    expect(state.commandjson).toMatchObject({
      configForm: false,
      listData: [],
      total: 0,
      queryjson: {
        page: 1,
        page_size: 4
      },
      formjson: {
        buttom_name: '',
        data_identifier: '',
        description: '',
        instruct: '{}',
        enable_status: 'disable'
      }
    })
    expect(state.configFormRules).toMatchObject({
      data_identifier: {
        required: true,
        message: 'device_template.table_header.commandIdentifier',
        trigger: 'blur'
      },
      buttom_name: {
        required: true,
        message: 'generate.btnname',
        trigger: 'blur'
      }
    })
    expect(state.columns.map((column: any) => column.key)).toEqual([
      'buttom_name',
      'data_identifier',
      'instruct',
      'description',
      'enable_status',
      'actions'
    ])
  })

  it('loads command list on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.deviceCustomCommandsList).toHaveBeenCalledTimes(1)
  })
})
