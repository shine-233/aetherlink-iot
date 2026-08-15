/**
 * 文件用途: 测试遥测字段新增和编辑步骤。
 * 核心逻辑: 模拟遥测表单输入和接口调用，验证新增、编辑和校验行为。
 * 关键注意事项: 遥测字段类型、单位和标识会影响图表绑定，测试应保持真实字段形状。
 * 重构建议: 与属性字段测试共享模型字段编辑 helper。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addTelemetry: vi.fn(),
  putTelemetry: vi.fn()
}))

vi.mock('@/service/api/system-data', () => ({
  addTelemetry: hoisted.addTelemetry,
  putTelemetry: hoisted.putTelemetry
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../../utils', () => ({
  getAdditionalInfo: vi.fn(() => [])
}))

import Component from '../add-edit-test.vue'

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

describe('device/template/components/step/add-edit-test.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addTelemetry.mockResolvedValue({ error: null })
    hoisted.putTelemetry.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes telemetry form with template id, validation rules and enum/read-write options', () => {
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
        message: 'device_template.table_header.pleaseEnterADataName'
      },
      data_identifier: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheDataIdentifier'
      },
      data_type: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'device_template.table_header.pleaseEnterTheDataType'
      },
      read_write_flag: {
        required: true,
        trigger: ['blur', 'input'],
        message: 'generate.enter-read-write'
      }
    })
    expect(state.generalOptions.map((option: any) => option.value)).toEqual(['Number', 'String', 'Boolean', 'Enum'])
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
