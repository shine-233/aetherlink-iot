/**
 * 文件用途: form 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('lodash-es', () => ({
  find: vi.fn((arr: any[], query: any) => arr.find(item => item.value === query.value))
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import Component from '../form.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const defaultFormElements = [
  {
    type: 'input',
    dataKey: 'key1',
    label: 'Label 1',
    placeholder: 'Enter',
    validate: { type: 'string', required: true, message: 'Key1 required' }
  },
  { type: 'select', dataKey: 'key2', label: 'Label 2', options: [{ label: 'A', value: 'a' }] },
  {
    type: 'table',
    dataKey: 'key3',
    label: 'Label 3',
    array: [{ type: 'input', dataKey: 'child', label: 'Child', validate: { type: 'string' } }]
  }
]

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      formElements: defaultFormElements,
      protocolConfig: {},
      ...props
    },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NInputNumber: defineComponent({ props: { value: { default: 0 } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NTooltip: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NEllipsis: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDynamicInput: defineComponent({
          props: { value: { default: () => [] }, onCreate: { type: Function, default: undefined } },
          emits: ['update:value'],
          setup(props, { slots }) {
            return () => {
              return h('div', props.value.map((_, index) => slots.default?.({ index })))
            }
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/config-detail/modules/form.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('initializes protocol config model and rules from plugin form schema', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(wrapper.props('formElements')).toEqual(defaultFormElements)
    expect(state.protocol_config).toEqual({
      key1: '',
      key2: '',
      key3: []
    })
    expect(state.rules).toMatchObject({
      key1: { type: 'string', required: true, message: 'Key1 required' },
      key2: {}
    })
    expect(state.onCreate()).toEqual({})
  })

  it('renders form elements', () => {
    const wrapper = mountComponent()
    expect(wrapper.html()).toContain('Label 1')
    expect(wrapper.html()).toContain('Label 2')
  })
})
