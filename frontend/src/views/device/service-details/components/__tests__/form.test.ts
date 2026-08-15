/**
 * 文件用途: 覆盖Form在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, nextTick } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import Component from '../form.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      formElements: [
        { type: 'input', dataKey: 'key1', label: 'Label 1', placeholder: 'Enter', validate: { type: 'string' } },
        { type: 'select', dataKey: 'key2', label: 'Label 2', options: [{ label: 'A', value: 'a' }] }
      ],
      protocolConfig: {},
      ...props
    },
    global: {
      stubs: {
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NInputNumber: defineComponent({ props: { value: { default: 0 } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NEllipsis: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NDynamicInput: defineComponent({
          props: { value: { default: () => [] } },
          emits: ['update:value'],
          setup(stubProps, { slots }) {
            return () => h('div', (stubProps.value as any[]).map((_row, index) => slots.default?.({ index })))
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('device/service-details/components/form.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('initializes protocol config keys for each service form field', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>
    expect(state.protocol_config).toEqual({
      key1: '',
      key2: ''
    })
    expect(state.rules).toEqual({
      key1: { type: 'string' },
      key2: {}
    })
  })

  it('renders form elements', () => {
    const wrapper = mountComponent()
    expect(wrapper.html()).toContain('Label 1')
    expect(wrapper.html()).toContain('Label 2')
  })

  it('initializes input, select and table fields in protocol config', async () => {
    const wrapper = mountComponent({
      formElements: [
        { type: 'input', dataKey: 'host', label: 'Host', placeholder: 'Host', validate: { type: 'string' } },
        { type: 'input', dataKey: 'port', label: 'Port', placeholder: 'Port', validate: { type: 'number' } },
        { type: 'select', dataKey: 'mode', label: 'Mode', options: [{ label: 'TCP', value: 'tcp' }], validate: {} },
        {
          type: 'table',
          dataKey: 'topics',
          label: 'Topics',
          validate: {},
          array: [
            { type: 'input', dataKey: 'source', label: 'Source', placeholder: 'Source', validate: { type: 'string' } }
          ]
        }
      ],
      protocolConfig: { host: '127.0.0.1' }
    })
    await nextTick()
    const state = wrapper.vm.$.setupState as Record<string, any>

    expect(state.protocol_config).toEqual({
      host: '127.0.0.1',
      port: '',
      mode: '',
      topics: []
    })
    expect(state.rules.host).toEqual({ type: 'string' })
    expect(state.rules.port).toEqual({ type: 'number' })
    expect(state.rules.mode).toEqual({})
  })

  it('creates empty rows for dynamic table entries', () => {
    const wrapper = mountComponent()
    const state = wrapper.vm.$.setupState as Record<string, any>

    expect(state.onCreate()).toEqual({})
  })
})
