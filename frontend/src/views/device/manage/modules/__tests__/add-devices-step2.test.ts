/**
 * 文件用途: 覆盖Add Devices Step2在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getDeviceConnectInfo: vi.fn(),
  updateDeviceVoucher: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  getDeviceConnectInfo: hoisted.getDeviceConnectInfo,
  updateDeviceVoucher: hoisted.updateDeviceVoucher
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/clipboard', () => ({
  writeClipboardText: vi.fn().mockResolvedValue(true)
}))

import Component from '../add-devices-step2.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      formElements: [],
      nextCallback: vi.fn(),
      device_id: 'device-1',
      formData: {},
      setIsSuccess: vi.fn(),
      ...props
    },
    global: {
      stubs: {
        NForm: defineComponent({ props: ['rules', 'model'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'placeholder'], emits: ['update:value', 'click'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options'], emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ props: ['type'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NScrollbar: defineComponent({ props: ['style'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NCard: defineComponent({ props: ['title'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDescriptions: defineComponent({ props: ['column'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NDescriptionsItem: defineComponent({ props: ['label'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        DeviceAccessGuide: defineComponent({
          props: ['accessGuide', 'connectInfo'],
          emits: ['copy'],
          setup(_, { slots }) {
            return () => h('section', slots['credential-form']?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/manage/modules/add-devices-step2.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getDeviceConnectInfo.mockResolvedValue({ data: { host: 'localhost', port: '1883' } })
    hoisted.updateDeviceVoucher.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes dynamic voucher fields and connection info for the created device', async () => {
    const formElements = [
      {
        type: 'input',
        dataKey: 'username',
        label: 'Username',
        placeholder: 'Enter username',
        validate: { required: true, message: 'Username required' }
      },
      {
        type: 'table',
        dataKey: 'mqtt',
        label: 'MQTT',
        array: [
          {
            type: 'input',
            dataKey: 'clientId',
            label: 'Client ID',
            validate: { required: true, message: 'Client ID required' }
          }
        ]
      }
    ]
    const wrapper = mountComponent({
      formElements,
      formData: {
        username: 'device-user',
        clientId: 'client-1'
      }
    })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(hoisted.getDeviceConnectInfo).toHaveBeenCalledWith({ device_id: 'device-1' })
    expect(state.connectInfo).toEqual({ host: 'localhost', port: '1883' })
    expect(state.formRules).toMatchObject({
      username: { required: true, message: 'Username required' },
      clientId: { required: true, message: 'Client ID required' }
    })
    expect(state.editableFormData).toMatchObject({
      username: 'device-user',
      clientId: 'client-1'
    })
  })

  it('fetches connect info on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getDeviceConnectInfo).toHaveBeenCalledWith({ device_id: 'device-1' })
  })

  it('handleSubmit calls updateDeviceVoucher', async () => {
    const setIsSuccess = vi.fn()
    const nextCallback = vi.fn()
    const wrapper = mountComponent({ setIsSuccess, nextCallback })
    await flushPromises()
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    expect(hoisted.updateDeviceVoucher).toHaveBeenCalledWith({
      device_id: 'device-1',
      voucher: expect.any(String)
    })
  })

  it('handleSubmit calls setIsSuccess true on success', async () => {
    const setIsSuccess = vi.fn()
    const nextCallback = vi.fn()
    hoisted.updateDeviceVoucher.mockResolvedValue({ error: null })
    const wrapper = mountComponent({ setIsSuccess, nextCallback })
    await flushPromises()
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    expect(setIsSuccess).toHaveBeenCalledWith(true)
  })

  it('handleSubmit calls setIsSuccess false on error', async () => {
    const setIsSuccess = vi.fn()
    const nextCallback = vi.fn()
    hoisted.updateDeviceVoucher.mockResolvedValue({ error: 'fail' })
    const wrapper = mountComponent({ setIsSuccess, nextCallback })
    await flushPromises()
    const state = getSetupState(wrapper)
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    await state.handleSubmit()
    expect(setIsSuccess).toHaveBeenCalledWith(false)
  })

  it('populates connectInfo on fetch', async () => {
    hoisted.getDeviceConnectInfo.mockResolvedValue({ data: { host: 'broker.local', port: '8883' } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.connectInfo).toEqual({ host: 'broker.local', port: '8883' })
  })

  it('processes formElements with input type', async () => {
    const formElements = [
      { type: 'input', dataKey: 'username', label: 'Username', placeholder: 'Enter username', validate: {} }
    ]
    const wrapper = mountComponent({ formElements, formData: { username: 'test' } })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.editableFormData.username).toBe('test')
  })

  it('processes formElements with table type', async () => {
    const formElements = [
      { type: 'table', dataKey: 'table1', label: 'Table', array: [
        { type: 'input', dataKey: 'field1', label: 'Field1', validate: {} }
      ]}
    ]
    const wrapper = mountComponent({ formElements, formData: { field1: 'value1' } })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.editableFormData.field1).toBe('value1')
  })
})
