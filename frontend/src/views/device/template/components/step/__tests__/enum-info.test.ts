/**
 * 文件用途: 测试枚举编辑组件。
 * 核心逻辑: 验证枚举选项的新增、删除和选择值更新。
 * 关键注意事项: 枚举值测试要覆盖空值、重复值和禁用状态。
 * 重构建议: 将枚举编辑测试场景整理为表格驱动用例。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/constants/business', () => ({
  enumDataTypeOption: []
}))

vi.mock('naive-ui', () => ({
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../enum-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { additionalInfo: [], ...props },
    global: {
      stubs: {
        NDataTable: defineComponent({ props: { data: { type: Array, default: () => [] } }, setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/components/step/enum-info.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('initializes enum table columns and boolean value options from additional info', () => {
    const additionalInfo = [{ value_type: 'Boolean', value: true, description: 'enabled' }]
    const wrapper = mountComponent({ additionalInfo })
    const state = getSetupState(wrapper)
    expect(wrapper.props('additionalInfo')).toEqual(additionalInfo)
    expect(state.booleanOptions).toEqual([
      { label: 'True', value: true },
      { label: 'False', value: false }
    ])
    expect(state.columns.map((column: any) => column.key)).toEqual(['value_type', 'value', 'description', 'actions'])
  })

  it('onAdd adds new row and emits updateAdditionalInfo', () => {
    const wrapper = mountComponent({ additionalInfo: [] })
    const state = getSetupState(wrapper)
    state.onAdd()
    expect(wrapper.emitted('updateAdditionalInfo')).toEqual([
      [[{ value_type: '', value: '', description: '' }]]
    ])
  })

  it('onChange updates field and emits updateAdditionalInfo', () => {
    const wrapper = mountComponent({
      additionalInfo: [
        { value_type: '', value: '', description: 'first row' },
        { value_type: 'Number', value: '2', description: 'second row' }
      ]
    })
    const state = getSetupState(wrapper)
    state.onChange('String', 0, 'value_type')
    expect(wrapper.emitted('updateAdditionalInfo')).toEqual([
      [
        [
          { value_type: 'String', value: '', description: 'first row' },
          { value_type: 'Number', value: '2', description: 'second row' }
        ]
      ]
    ])
  })

  it('onDel removes row and emits updateAdditionalInfo', () => {
    const wrapper = mountComponent({ additionalInfo: [{ value_type: 'String', value: '1', description: 'd' }, { value_type: 'Number', value: '2', description: 'd2' }] })
    const state = getSetupState(wrapper)
    state.onDel(0)
    expect(wrapper.emitted('updateAdditionalInfo')).toEqual([
      [[{ value_type: 'Number', value: '2', description: 'd2' }]]
    ])
  })
})
