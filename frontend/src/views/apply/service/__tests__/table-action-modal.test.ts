/**
 * 文件用途：覆盖 table-action-modal 在 协议服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addProtocolPlugin: vi.fn(),
  editProtocolPlugin: vi.fn()
}))

vi.mock('@/service/api', () => ({
  addProtocolPlugin: hoisted.addProtocolPlugin,
  editProtocolPlugin: hoisted.editProtocolPlugin
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/tool', () => ({
  deepClone: (obj: any) => JSON.parse(JSON.stringify(obj))
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg })
}))

import Component from '../components/table-action-modal.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountComponent = (props: Record<string, any> = {}) => {
  const wrapper = mount(Component, {
    props: {
      visible: false,
      type: 'add',
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({
          props: { show: Boolean, preset: String, title: String, class: String },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', { class: 'n-modal' }, slots.default ? slots.default() : [])
          }
        }),
        NForm: defineComponent({
          props: { labelPlacement: String, labelWidth: Number, model: Object, rules: Object },
          setup() {
            const validate = () => Promise.resolve()
            return { validate }
          },
          render() {
            return h('form', { class: 'n-form' }, this.$slots.default ? this.$slots.default() : [])
          }
        }),
        NFormItemGridItem: defineComponent({
          props: { span: Number, label: String, path: String },
          setup(_, { slots }) { return () => h('div', { class: 'n-form-item' }, slots.default ? slots.default() : []) }
        }),
        NGrid: defineComponent({ props: { cols: Number, xGap: Number }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ props: { value: { default: '' }, type: String, placeholder: String }, emits: ['update:value'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: Array }, emits: ['update:value'], setup() { return () => h('select') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ props: { size: Number, justify: String, class: String }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof mount>) => wrapper.vm.$.setupState as Record<string, any>

describe('apply/service/components/table-action-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.addProtocolPlugin.mockResolvedValue({ error: null })
    hoisted.editProtocolPlugin.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('has correct component name', () => {
    const wrapper = mountComponent()
    expect(wrapper.vm.$options.name).toBe('TableActionModal')
  })

  it('title computed returns add title for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('common.add')
  })

  it('title computed returns edit title for edit type', () => {
    const wrapper = mountComponent({ type: 'edit' })
    const state = getSetupState(wrapper)
    expect(state.title).toBe('common.edit')
  })

  it('formModel has default values', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.formModel.name).toBe('')
    expect(state.formModel.language_code).toBe('zh')
  })

  it('deviceOptions has 2 options', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.deviceOptions).toHaveLength(2)
  })

  it('closeModal emits update:visible with false', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getSetupState(wrapper)
    state.closeModal()
    const emitted = wrapper.emitted('update:visible')
    expect(emitted).toEqual([[false]])
  })

  it('handleAddAdditionalInfo adds item to list', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.handleAddAdditionalInfo()
    expect(state.formModel.additional_info_list).toHaveLength(1)
    expect(state.formModel.additional_info_list[0]).toEqual({ key: '', value: '' })
  })

  it('emits success on add submit', async () => {
    hoisted.addProtocolPlugin.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ visible: true, type: 'add' })
    const state = getSetupState(wrapper)
    state.formModel.name = 'Test Service'
    state.formModel.device_type = 1
    state.formModel.protocol_type = 'MQTT'
    state.formModel.access_address = 'localhost'
    state.formModel.http_address = 'http://localhost'
    state.formModel.sub_topic_prefix = 'test/'
    state.formModel.description = 'desc'
    state.formModel.additional_info_list = [
      { key: 'tenant', value: 'alpha' },
      { key: '', value: 'ignored' }
    ]
    await state.handleSubmit()
    expect(hoisted.addProtocolPlugin).toHaveBeenCalledTimes(1)
    expect(hoisted.addProtocolPlugin).toHaveBeenCalledWith({
      name: 'Test Service',
      device_type: 1,
      protocol_type: 'MQTT',
      access_address: 'localhost',
      http_address: 'http://localhost',
      sub_topic_prefix: 'test/',
      description: 'desc',
      language_code: 'zh',
      additional_info: '{"tenant":"alpha"}'
    })
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })
})
