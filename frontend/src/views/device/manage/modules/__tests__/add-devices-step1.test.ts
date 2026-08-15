/**
 * 文件用途: 覆盖Add Devices Step1在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceAdd: vi.fn(),
  messageError: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceAdd: hoisted.deviceAdd
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ error: hoisted.messageError, success: hoisted.messageSuccess })
}))

import Component from '../add-devices-step1.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      configOptions: [{ name: 'Config1', id: 'cfg-1' }],
      nextCallback: vi.fn(),
      setIdCallback: vi.fn(),
      ...props
    },
    global: {
      stubs: {
        NCard: defineComponent({ props: ['bordered'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NForm: defineComponent({ props: ['labelWidth', 'model', 'rules', 'size'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NFormItem: defineComponent({ props: ['label', 'path'], setup(_, { slots }) { return () => h('div', slots.default?.()) } }),
        NInput: defineComponent({ props: ['value', 'placeholder', 'maxlength'], emits: ['update:value'], setup() { return () => h('input') } }),
        NSelect: defineComponent({ props: ['value', 'options', 'filterable', 'placeholder', 'labelField', 'valueField'], emits: ['update:value'], setup() { return () => h('div') } }),
        NButton: defineComponent({ props: ['type', 'attrType'], emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default?.()) } }),
        NDynamicTags: defineComponent({ props: ['value'], emits: ['update:value'], setup() { return () => h('div') } })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/manage/modules/add-devices-step1.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceAdd.mockResolvedValue({ data: { id: 'dev-1', voucher: {} } })
    hoisted.messageError.mockReset()
    hoisted.messageSuccess.mockReset()
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes add-device form with config options and PID business rule', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.formValue).toEqual({
      name: '',
      pid_number: '',
      label: [],
      device_config_id: ''
    })
    expect(state.rules).toMatchObject({
      name: {
        required: true,
        message: 'custom.devicePage.enterDeviceName',
        trigger: 'blur'
      },
      pid_number: {
        required: true,
        message: 'rdi.device.pidInvalid',
        trigger: ['input', 'blur']
      }
    })
    expect(state.rules.pid_number.pattern).toEqual(/^[A-Za-z0-9]{12}$/)
    expect(wrapper.props('configOptions')).toEqual([{ name: 'Config1', id: 'cfg-1' }])
  })

  it('initializes formValue with defaults', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.formValue.name).toBe('')
    expect(state.formValue.pid_number).toBe('')
    expect(state.formValue.label).toEqual([])
    expect(state.formValue.device_config_id).toBe('')
  })

  it('has validation rules for name and pid_number', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.rules.name).toEqual({
      required: true,
      message: 'custom.devicePage.enterDeviceName',
      trigger: 'blur'
    })
    expect(state.rules.pid_number).toMatchObject({
      required: true,
      message: 'rdi.device.pidInvalid',
      trigger: ['input', 'blur']
    })
    expect(state.rules.pid_number.pattern).toEqual(/^[A-Za-z0-9]{12}$/)
  })

  it('handleValidateClick calls deviceAdd on valid form', async () => {
    const nextCallback = vi.fn()
    const setIdCallback = vi.fn()
    const wrapper = mountComponent({ nextCallback, setIdCallback })
    await flushPromises()
    const state = getSetupState(wrapper)
    state.formValue.name = 'Test Device'
    state.formValue.pid_number = 'abc123456789'
    state.formValue.device_config_id = 'cfg-1'
    state.formValue.label = ['cold', 'warehouse']
    const mockValidate = vi.fn().mockResolvedValue(undefined)
    state.formRef = { validate: mockValidate }

    const event = { preventDefault: vi.fn() } as any
    await state.handleValidateClick(event)
    expect(event.preventDefault).toHaveBeenCalledTimes(1)
    expect(mockValidate).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceAdd).toHaveBeenCalledWith({
      name: 'Test Device',
      pid_number: 'ABC123456789',
      label: 'cold,warehouse',
      device_config_id: 'cfg-1',
      access_way: 'A'
    })
    expect(setIdCallback).toHaveBeenCalledWith('dev-1', 'cfg-1', {}, 'ABC123456789')
    expect(nextCallback).toHaveBeenCalledTimes(1)
    expect(hoisted.messageError).toHaveBeenCalledTimes(0)
  })

  it('shows validationFailed and skips deviceAdd when form validation rejects with field errors', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const mockValidateError = [{ path: ['name'] }, { path: ['pid_number'] }]
    state.formRef = { validate: vi.fn().mockRejectedValue(mockValidateError) }

    const event = { preventDefault: vi.fn() } as any
    await state.handleValidateClick(event)

    expect(event.preventDefault).toHaveBeenCalledTimes(1)
    expect(hoisted.deviceAdd).toHaveBeenCalledTimes(0)
    expect(hoisted.messageError).toHaveBeenCalledWith('custom.devicePage.validationFailed')
  })

  it('shows addFailed when deviceAdd rejects after passing validation', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.formValue.name = 'Test Device'
    state.formValue.pid_number = 'ABC123456789'
    state.formRef = { validate: vi.fn().mockResolvedValue(undefined) }
    hoisted.deviceAdd.mockRejectedValue(new Error('network down'))

    const event = { preventDefault: vi.fn() } as any
    await state.handleValidateClick(event)

    expect(hoisted.messageError).toHaveBeenCalledWith('generate.addFailed')
  })

  it('accepts configOptions prop', async () => {
    const options = [{ name: 'Config1', id: 'cfg-1' }, { name: 'Config2', id: 'cfg-2' }]
    const wrapper = mountComponent({ configOptions: options })
    await flushPromises()
    expect(wrapper.props('configOptions')).toEqual(options)
  })
})
