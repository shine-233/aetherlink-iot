/**
 * 文件用途：覆盖 form 在 接入插件管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import Component from '../components/form.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = mount(Component, {
    props: {
      protocolConfig: {},
      formElements: [],
      ...props
    },
    global: {
      stubs: {
        NForm: defineComponent({
          props: { model: Object, rules: Object, labelPlacement: String },
          setup(_, { slots }) { return () => h('form', { class: 'n-form' }, slots.default ? slots.default() : []) }
        }),
        NFormItem: defineComponent({
          props: { label: String, path: String },
          setup(_, { slots }) { return () => h('div', { class: 'n-form-item' }, slots.default ? slots.default() : []) }
        }),
        NInput: defineComponent({ props: { value: { default: '' }, placeholder: String }, emits: ['update:value'], setup() { return () => h('input') } }),
        NInputNumber: defineComponent({ props: { value: { default: null }, placeholder: String }, emits: ['update:value'], setup() { return () => h('input', { type: 'number' }) } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: Array }, emits: ['update:value'], setup() { return () => h('select') } }),
        NEllipsis: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
        NDynamicInput: defineComponent({
          props: { value: { type: Array, default: () => [] }, onCreate: Function },
          emits: ['update:value'],
          setup(_, { slots }) {
            return () => h('div', { class: 'n-dynamic-input' }, slots.default ? slots.default({ index: 0 }) : [])
          }
        }),
        NEmpty: defineComponent({
          props: { description: String },
          setup() { return () => h('div', { class: 'n-empty' }) }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof mount>) => wrapper.vm.$.setupState as Record<string, any>

describe('apply/plugin/components/form.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('starts with the provided protocol config model and no generated rules when schema is empty', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)

    expect(state.protocol_config).toEqual({})
    expect(state.rules).toEqual({})
  })

  it('renders the empty state instead of a blank form when schema is missing', () => {
    const wrapper = mountComponent()
    expect(wrapper.find('.n-empty').exists()).toBe(true)
    expect(wrapper.find('.n-form').exists()).toBe(false)
  })

  it('renders NForm component when schema has elements', () => {
    const wrapper = mountComponent({
      formElements: [
        { type: 'input', dataKey: 'host', label: 'Host', placeholder: 'Enter host', validate: {} }
      ]
    })
    const form = wrapper.find('.n-form')
    expect(form.attributes('class')).toContain('n-form')
    expect(wrapper.find('.n-empty').exists()).toBe(false)
  })

  it('initializes with empty rules', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.rules).toEqual({})
  })

  it('processes input type formElements', async () => {
    const formElements = [
      { type: 'input', dataKey: 'host', label: 'Host', placeholder: 'Enter host', validate: { required: true, type: 'string' } }
    ]
    const wrapper = mountComponent({ formElements, protocolConfig: {} })
    await wrapper.vm.$nextTick()
    const state = getSetupState(wrapper)
    expect(state.protocol_config.host).toBe('')
    expect(state.rules.host).toEqual({ required: true, type: 'string' })
  })

  it('processes select type formElements', async () => {
    const formElements = [
      { type: 'select', dataKey: 'mode', label: 'Mode', placeholder: 'Select mode', options: [{ label: 'A', value: 'a' }], validate: { required: true } }
    ]
    const wrapper = mountComponent({ formElements, protocolConfig: {} })
    await wrapper.vm.$nextTick()
    const state = getSetupState(wrapper)
    expect(state.protocol_config.mode).toBe('')
    expect(state.rules.mode).toEqual({ required: true })
  })

  it('processes table type formElements with array default', async () => {
    const formElements = [
      { type: 'table', dataKey: 'ports', label: 'Ports', validate: {}, array: [] }
    ]
    const wrapper = mountComponent({ formElements, protocolConfig: {} })
    await wrapper.vm.$nextTick()
    const state = getSetupState(wrapper)
    expect(Array.isArray(state.protocol_config.ports)).toBe(true)
  })

  it('onCreate returns empty object', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.onCreate()).toEqual({})
  })

  it('preserves existing protocolConfig values', async () => {
    const formElements = [
      { type: 'input', dataKey: 'host', label: 'Host', placeholder: 'Enter host', validate: {} }
    ]
    const wrapper = mountComponent({ formElements, protocolConfig: { host: 'existing-host' } })
    await wrapper.vm.$nextTick()
    const state = getSetupState(wrapper)
    expect(state.protocol_config.host).toBe('existing-host')
  })
})
