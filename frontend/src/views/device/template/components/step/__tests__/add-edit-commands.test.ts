/**
 * 文件用途: 测试命令字段新增和编辑步骤。
 * 核心逻辑: 模拟命令表单、输入输出参数和接口提交，验证命令模型配置。
 * 关键注意事项: 命令参数会影响下行控制，测试要确认提交载荷包含完整参数。
 * 重构建议: 与事件和属性编辑测试共享字段表单 mock。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addCommands: vi.fn(),
  putCommands: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  addCommands: hoisted.addCommands,
  putCommands: hoisted.putCommands
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../add-edit-commands.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { addAndEditModalVisible: false, deviceTemplateId: 'tpl-1', objItem: {}, ...props },
    global: {
      stubs: {
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/template/components/step/add-edit-commands.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addCommands.mockResolvedValue({ error: null })
    hoisted.putCommands.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes command form, parameter rules and enum-capable parameter contract', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.addFrom).toMatchObject({
      device_template_id: state.deviceTemplateId,
      data_name: '',
      data_identifier: '',
      description: '',
      params: ''
    })
    expect(state.fromRules).toMatchObject({
      data_name: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheCommandName'
      },
      data_identifier: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheCommandIdentifier'
      }
    })
    expect(state.addParameterFrom).toMatchObject({
      data_name: '',
      data_identifier: '',
      param_type: 'string',
      description: '',
      data_type: 'string',
      enum_config: []
    })
    expect(state.addParameterRules).toMatchObject({
      data_name: { required: true },
      data_identifier: { required: true },
      param_type: { required: true }
    })
    expect(state.generalOptions.map((option: any) => option.value)).toEqual(['String', 'Number', 'Boolean', 'Enum'])
    expect(state.col.map((column: any) => column.key)).toEqual([
      'data_name',
      'data_identifier',
      'param_type',
      'description',
      'actions'
    ])
  })

  it('initializes with generalOptions including Enum', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.generalOptions).toHaveLength(4)
  })
})
