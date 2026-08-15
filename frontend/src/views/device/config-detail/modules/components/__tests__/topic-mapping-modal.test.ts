/**
 * 文件用途: topic-mapping-modal 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
  createI18n: () => ({ global: { t: (key: string) => key, locale: { value: 'en-US' } } })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
  NFormItem: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NSelect: defineComponent({ props: { value: { default: null } }, emits: ['update:value'], setup() { return () => h('div') } }),
  NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
  NPopover: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
  NText: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } })
}))

import Component from '../topic-mapping-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { visible: false, editData: null, ...props },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/components/topic-mapping-modal.vue', () => {
  beforeEach(() => { vi.clearAllMocks() })
  afterEach(() => { while (mountedWrappers.length > 0) { mountedWrappers.pop()?.unmount() } })

  it('initializes create modal form, validation rules and downlink topic defaults', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.modalVisible).toBe(false)
    expect(state.formData).toEqual({
      mapping_name: '',
      direction: 'down',
      original_topic: '',
      target_topic: '',
      data_identifier: '',
      description: '',
      priority: 0,
      enabled: true
    })
    expect(state.rules).toMatchObject({
      mapping_name: [{ required: true, message: 'generate.topicMapping.validation.mappingName', trigger: 'blur' }],
      direction: [{ required: true, message: 'generate.topicMapping.validation.direction', trigger: 'change' }],
      original_topic: [{ required: true, message: 'generate.topicMapping.validation.originalTopic', trigger: 'blur' }],
      target_topic: [{ required: true, message: 'generate.topicMapping.validation.targetTopic', trigger: 'change' }]
    })
    expect(state.dataDirectionOptions.map((option: any) => option.value)).toEqual(['down', 'up'])
    expect(state.targetTopicOptions.map((option: any) => option.value)).toContain('devices/command/{device_number}/+')
  })

  it('initializes form with default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.formData.direction).toBe('down')
    expect(state.formData.enabled).toBe(true)
  })

  it('handleCancel emits update:visible false', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleCancel()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('dataDirectionOptions has up and down options', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.dataDirectionOptions).toEqual([
      { label: 'generate.topicMapping.direction.down', value: 'down' },
      { label: 'generate.topicMapping.direction.up', value: 'up' }
    ])
  })

  it('showDataIdentifier is true when target_topic is devices/command', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formData.target_topic = 'devices/command/{device_number}/+'
    expect(state.showDataIdentifier).toBe(true)
  })

  it('handleSave emits save event when form validates', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined), restoreValidation: vi.fn() }
    await state.handleSave()
    expect(wrapper.emitted('save')).toEqual([
      [{
        mapping_name: '',
        direction: 'down',
        original_topic: '',
        target_topic: '',
        data_identifier: '',
        description: '',
        priority: 0,
        enabled: true
      }]
    ])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })
})
