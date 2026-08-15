/**
 * 文件用途：覆盖 table-action-modal 在 协议服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addProtocolPlugin: vi.fn(),
  editProtocolPlugin: vi.fn(),
}))

vi.mock('@/service/api', () => ({
  addProtocolPlugin: hoisted.addProtocolPlugin,
  editProtocolPlugin: hoisted.editProtocolPlugin,
}))

vi.mock('@/utils/common/tool', () => ({
  deepClone: (v: any) => JSON.parse(JSON.stringify(v)),
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg }),
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import TableActionModal from '../table-action-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(TableActionModal, {
    props: {
      visible: false,
      type: 'add',
      editData: null,
      ...props
    },
    global: {
      stubs: {
        NModal: defineComponent({ props: { show: Boolean }, emits: ['update:show'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({
          props: ['model', 'rules', 'labelPlacement', 'labelWidth'],
          setup(_, { slots, expose }) {
            expose({ validate: vi.fn().mockResolvedValue(undefined) })
            return () => h('div', slots.default?.())
          }
        }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: { value: { default: '' } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NSelect: defineComponent({ props: { value: { default: null }, options: { default: () => [] } }, emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NSpace: defineComponent({ props: ['justify', 'size'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NGrid: defineComponent({ props: ['cols', 'xGap'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItemGridItem: defineComponent({ props: ['span', 'label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('TableActionModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    mountedWrappers.forEach(w => w.unmount())
    mountedWrappers.length = 0
  })

  it('starts in add mode with required service plugin fields and device options', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)

    expect(state.title).toBe('common.add')
    expect(state.modalVisible).toBe(false)
    expect(state.formModel).toEqual({
      name: '',
      device_type: '',
      protocol_type: '',
      access_address: null,
      http_address: null,
      sub_topic_prefix: null,
      description: null,
      language_code: 'zh',
      additional_info: '',
      additional_info_list: []
    })
    expect(state.rules).toEqual({
      name: { required: true, message: 'common.pleaseCheckValue' },
      device_type: { required: true, message: 'common.pleaseCheckValue' },
      protocol_type: { required: true, message: 'common.pleaseCheckValue' },
      access_address: { required: true, message: 'common.pleaseCheckValue' },
      http_address: { required: true, message: 'common.pleaseCheckValue' },
      sub_topic_prefix: { required: true, message: 'common.pleaseCheckValue' }
    })
    expect(state.deviceOptions).toEqual([
      { label: 'generate.direct-connected-device', value: 1 },
      { label: 'generate.gatewayDevice', value: 2 }
    ])
  })

  it('should compute title based on type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getState(wrapper)
    expect(state.title).toBe('common.add')
  })

  it('should close modal', () => {
    const wrapper = mountComponent({ visible: true })
    const state = getState(wrapper)
    state.closeModal()
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })

  it('should handle add additional info', () => {
    const wrapper = mountComponent()
    const state = getState(wrapper)
    const initialLength = state.formModel.additional_info_list.length
    state.handleAddAdditionalInfo()
    expect(state.formModel.additional_info_list.length).toBe(initialLength + 1)
  })

  it('should reset form model for add type', () => {
    const wrapper = mountComponent({ type: 'add' })
    const state = getState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('')
  })

  it('should update form model for edit type', () => {
    const wrapper = mountComponent({ type: 'edit', editData: { id: '1', name: 'Test', device_type: 1, protocol_type: 'MQTT', access_address: 'addr', http_address: 'http', sub_topic_prefix: 'topic', description: 'desc', additional_info: '{"key":"val"}', language_code: 'zh' } })
    const state = getState(wrapper)
    state.handleUpdateFormModelByModalType()
    expect(state.formModel.name).toBe('Test')
  })

  it('should handle submit for add', async () => {
    hoisted.addProtocolPlugin.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ visible: true, type: 'add' })
    const state = getState(wrapper)
    Object.assign(state.formModel, {
      name: 'Test',
      device_type: 1,
      protocol_type: 'MQTT',
      access_address: 'addr',
      http_address: 'http',
      sub_topic_prefix: 'topic',
      description: 'desc',
      additional_info: '{}',
      additional_info_list: [{ key: 'region', value: 'east' }],
      language_code: 'zh'
    })
    await state.handleSubmit()
    await flushPromises()
    expect(hoisted.addProtocolPlugin).toHaveBeenCalledTimes(1)
    expect(hoisted.addProtocolPlugin).toHaveBeenCalledWith({
      name: 'Test',
      device_type: 1,
      protocol_type: 'MQTT',
      access_address: 'addr',
      http_address: 'http',
      sub_topic_prefix: 'topic',
      description: 'desc',
      additional_info: '{"region":"east"}',
      language_code: 'zh'
    })
    expect(wrapper.emitted('success')).toEqual([[]])
    expect(wrapper.emitted('update:visible')).toEqual([[false]])
  })
})
