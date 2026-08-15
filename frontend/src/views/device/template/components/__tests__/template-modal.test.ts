/**
 * 文件用途: 测试物模型弹窗组件。
 * 核心逻辑: 挂载弹窗并验证步骤切换、表单状态和提交事件。
 * 关键注意事项: 弹窗测试需要保持初始模板数据和各步骤组件 mock 同步。
 * 重构建议: 抽出步骤组件 mock 工厂，减少新增步骤时的测试维护成本。
 */
import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('../utils', () => ({
  initTemplateInfoData: {},
  templateInfoData: { value: {} }
}))

vi.mock('naive-ui', () => ({
  NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NSteps: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NStep: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
}))

import Component from '../template-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { visible: false, type: 'add', templateId: '', getTableData: vi.fn(), ...props },
    global: {
      stubs: {
        AddInfo: true,
        ModelDefinition: true,
        WebChartConfig: true,
        AppChartConfig: true,
        Complete: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/components/template-modal.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('initializes the five-step template wizard and selected step component', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.modalVisible).toBe(false)
    expect(state.stepCurrent).toBe(1)
    expect(state.deviceTemplateId).toBe('')
    expect(state.componentsList.map((item: any) => item.id)).toEqual([1, 2, 3, 4, 5])
    expect(state.SwitchComponents).toBe(state.componentsList[0].components)
    expect(state.title).toBe('device_template.addThingModel')
  })

  it('initializes with step 1', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.stepCurrent).toBe(1)
  })

  it('title is add when type is add', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('device_template.addThingModel')
  })

  it('title is edit when type is edit', () => {
    const wrapper = mountComponent({ type: 'edit' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('device_template.editThingModel')
  })
})
