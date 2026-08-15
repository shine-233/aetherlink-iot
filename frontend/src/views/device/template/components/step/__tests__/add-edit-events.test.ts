/**
 * 文件用途: 测试事件字段新增和编辑步骤。
 * 核心逻辑: 模拟事件表单、参数表格和接口提交，验证事件模型配置。
 * 关键注意事项: 事件参数会被告警和自动化读取，测试要覆盖参数序列化。
 * 重构建议: 将事件参数表格断言抽成可复用工具。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addEvents: vi.fn(),
  putEvents: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  addEvents: hoisted.addEvents,
  putEvents: hoisted.putEvents
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../add-edit-events.vue'

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

describe('device/template/components/step/add-edit-events.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addEvents.mockResolvedValue({ error: null })
    hoisted.putEvents.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes event form, parameter rules and parameter table contract', () => {
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
        message: 'device_template.table_header.PleaseEventName'
      },
      data_identifier: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.PleaseEeventIdentifier'
      }
    })
    expect(state.addParameterFrom).toMatchObject({
      data_name: '',
      data_identifier: '',
      read_write_flag: 'string',
      description: ''
    })
    expect(state.addParameterRules).toMatchObject({
      data_name: { required: true },
      data_identifier: { required: true },
      read_write_flag: { required: true }
    })
    expect(state.col.map((column: any) => column.key)).toEqual([
      'data_name',
      'data_identifier',
      'read_write_flag',
      'description',
      'actions'
    ])
  })

  it('initializes with generalOptions', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.generalOptions).toHaveLength(3)
  })
})
