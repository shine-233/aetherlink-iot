/**
 * 文件用途：覆盖 form 在 接入插件管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import DynamicForm from '../form.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(DynamicForm, {
    props,
    global: {
      stubs: {
        NForm: defineComponent({ props: ['model', 'rules', 'labelPlacement'], setup(_, { slots }) { return () => h('form', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path', 'ignorePathChange', 'showLabel', 'rule'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'placeholder'], emits: ['update:value'], setup(_, { slots }) { return () => h('input', slots.default?.()) } }),
        NInputNumber: defineComponent({ props: ['value', 'placeholder'], emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: ['value', 'options'], emits: ['update:value'], setup() { return () => h('div') } }),
        NEllipsis: defineComponent({ props: ['class'], setup(_, { slots }) { return () => h('span', slots.default?.()) } }),
        NDynamicInput: defineComponent({ props: ['value', 'itemStyle', 'onCreate'], emits: ['update:value'], setup(_, { slots }) { return () => h('div', slots.default?.({ index: 0 })) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('DynamicForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('should mount with default state', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    expect(vm.rules).toEqual({})
  })

  it('should process input formElements', () => {
    const wrapper = mountComponent({
      formElements: [
        { type: 'input', dataKey: 'host', label: 'Host', placeholder: 'Enter host', validate: { required: true } }
      ]
    })
    const vm = wrapper.vm as any
    expect(vm.rules.host).toEqual({ required: true })
  })

  it('should process select formElements', () => {
    const wrapper = mountComponent({
      formElements: [
        { type: 'select', dataKey: 'protocol', label: 'Protocol', options: [{ label: 'MQTT', value: 'mqtt' }], validate: {} }
      ]
    })
    const vm = wrapper.vm as any
    expect(vm.rules.protocol).toEqual({})
  })

  it('should process table formElements and initialize as array', () => {
    const wrapper = mountComponent({
      formElements: [
        { type: 'table', dataKey: 'devices', array: [], validate: {} }
      ]
    })
    const vm = wrapper.vm as any
    expect(vm.protocol_config.devices).toEqual([])
  })

  it('should process input with number validate type', () => {
    const wrapper = mountComponent({
      formElements: [
        { type: 'input', dataKey: 'port', label: 'Port', placeholder: 'Enter port', validate: { required: true, type: 'number' } }
      ]
    })
    const vm = wrapper.vm as any
    expect(vm.rules.port).toEqual({ required: true, type: 'number' })
  })

  it('should handle empty formElements', () => {
    const wrapper = mountComponent({ formElements: [] })
    const vm = wrapper.vm as any
    expect(vm.rules).toEqual({})
  })

  it('should handle undefined formElements', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    expect(vm.rules).toEqual({})
  })

  it('should return empty object from onCreate', () => {
    const wrapper = mountComponent()
    const vm = wrapper.vm as any
    expect(vm.onCreate()).toEqual({})
  })
})
