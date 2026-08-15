/**
 * 文件用途: 测试属性字段新增和编辑步骤。
 * 核心逻辑: 模拟属性表单和接口保存，验证属性模型配置行为。
 * 关键注意事项: 属性字段会影响设备详情和配置页面，测试要覆盖类型与默认值。
 * 重构建议: 与遥测字段测试合并通用字段编辑场景。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addAttributes: vi.fn(),
  putAttributes: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  addAttributes: hoisted.addAttributes,
  putAttributes: hoisted.putAttributes
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../../utils', () => ({
  getAdditionalInfo: vi.fn(() => [])
}))

import Component from '../add-edit-attributes.vue'

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
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        EnumInfo: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/template/components/step/add-edit-attributes.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addAttributes.mockResolvedValue({ error: null })
    hoisted.putAttributes.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes attribute form with template id, required rules and type options', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.addFrom).toMatchObject({
      device_template_id: state.deviceTemplateId,
      data_name: '',
      data_identifier: '',
      unit: '',
      description: '',
      additional_info: []
    })
    expect(state.fromRules).toMatchObject({
      data_name: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheAttributeName'
      },
      data_identifier: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheAttributeIdentifier'
      },
      data_type: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheAttributeType'
      },
      read_write_flag: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'generate.enter-read-write'
      }
    })
    expect(state.generalOptions.map((option: any) => option.value)).toEqual(['String', 'Number', 'Boolean', 'Enum'])
    expect(state.readAndWriteOptions.map((option: any) => option.value)).toEqual(['R', 'W', 'RW'])
  })

  it('initializes with empty form', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.addFrom).toMatchObject({
      device_template_id: 'tpl-1',
      data_name: '',
      data_identifier: '',
      unit: '',
      description: '',
      additional_info: []
    })
  })
})
